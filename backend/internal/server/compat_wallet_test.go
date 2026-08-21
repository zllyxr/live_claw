package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/migrations"
)

type compatWalletQueryFunc func(string, []driver.NamedValue) (driver.Rows, error)

type compatWalletTestDriver struct {
	query compatWalletQueryFunc
}

func (testDriver *compatWalletTestDriver) Open(string) (driver.Conn, error) {
	return &compatWalletTestConn{query: testDriver.query}, nil
}

type compatWalletTestConn struct {
	query compatWalletQueryFunc
}

func (connection *compatWalletTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (connection *compatWalletTestConn) Close() error {
	return nil
}

func (connection *compatWalletTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (connection *compatWalletTestConn) QueryContext(
	_ context.Context,
	query string,
	arguments []driver.NamedValue,
) (driver.Rows, error) {
	return connection.query(query, arguments)
}

type compatWalletTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (rows *compatWalletTestRows) Columns() []string {
	return rows.columns
}

func (rows *compatWalletTestRows) Close() error {
	return nil
}

func (rows *compatWalletTestRows) Next(destination []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(destination, rows.values[rows.index])
	rows.index++
	return nil
}

var compatWalletDriverSequence atomic.Uint64

func compatWalletTestDB(t *testing.T, query compatWalletQueryFunc) *sql.DB {
	t.Helper()
	name := "server_compat_wallet_" + strconv.FormatUint(compatWalletDriverSequence.Add(1), 10)
	sql.Register(name, &compatWalletTestDriver{query: query})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestCompatWalletBalanceReturnsSixEnabledProductsWhenPayListIsEmpty(t *testing.T) {
	productRows := make([][]driver.Value, 0, 6)
	for index, coin := range []int64{10, 100, 3000, 9800, 38800, 58800} {
		productRows = append(productRows, []driver.Value{
			int64(index + 1), coin, coin * 100, int64(0), int64(2),
		})
	}
	var productQuery string
	db := compatWalletTestDB(t, func(query string, arguments []driver.NamedValue) (driver.Rows, error) {
		switch {
		case strings.Contains(query, "wallet_accounts"):
			if len(arguments) != 1 || arguments[0].Value != int64(42) {
				return nil, errors.New("unexpected wallet user")
			}
			return &compatWalletTestRows{
				columns: []string{"coin"},
				values:  [][]driver.Value{{int64(125)}},
			}, nil
		case strings.Contains(query, "FROM recharge_products product"):
			productQuery = query
			return &compatWalletTestRows{
				columns: []string{"id", "coin_amount", "amount_minor", "bonus_coin", "currency_scale"},
				values:  productRows,
			}, nil
		case strings.Contains(query, "FROM payment_channels"):
			return &compatWalletTestRows{columns: []string{"channel_key", "name"}}, nil
		default:
			return nil, errors.New("unexpected query")
		}
	})

	balance, err := compatWalletBalance(context.Background(), db, 42)
	if err != nil {
		t.Fatal(err)
	}
	if productQuery == "" || !strings.Contains(productQuery, "WHERE product.status=1") {
		t.Fatalf("enabled-product filter is missing: %q", productQuery)
	}
	if !strings.Contains(productQuery, "NOT EXISTS") ||
		!strings.Contains(productQuery, "enabled_channel.status=1") ||
		!strings.Contains(productQuery, "OR EXISTS") {
		t.Fatalf("empty-channel fallback is missing: %q", productQuery)
	}
	payList := balance["paylist"].([]map[string]any)
	if len(payList) != 0 {
		t.Fatalf("disabled channels leaked into paylist: %#v", payList)
	}
	rules := balance["rules"].([]map[string]any)
	if len(rules) != 6 {
		t.Fatalf("got %d recharge tiers, want 6: %#v", len(rules), rules)
	}
	if rules[0]["coin"] != int64(10) || rules[0]["money"] != "10.00" ||
		rules[5]["coin"] != int64(58800) || rules[5]["money"] != "58800.00" {
		t.Fatalf("unexpected recharge tier payload: %#v", rules)
	}
}

func TestCompatWalletBalanceWithoutEnabledChannelsIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err = tx.ExecContext(ctx, "UPDATE payment_channels SET status=0"); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	enabledIDs := make(map[int64]bool, 6)
	for index := 0; index < 6; index++ {
		result, insertErr := tx.ExecContext(ctx, `
			INSERT INTO recharge_products
				(name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
			VALUES(?, 'TST', 2, ?, ?, 0, 1, ?)`,
			"channel-independent-"+suffix+"-"+strconv.Itoa(index),
			int64(900_000_000+index), int64(9_000_000+index), 10_000+index,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			t.Fatal(idErr)
		}
		enabledIDs[id] = false
	}
	disabledResult, err := tx.ExecContext(ctx, `
		INSERT INTO recharge_products
			(name,fiat_currency,currency_scale,amount_minor,coin_amount,bonus_coin,status,sort_order)
		VALUES(?, 'TST', 2, 999999999, 9999999, 0, 0, 20000)`,
		"disabled-channel-independent-"+suffix,
	)
	if err != nil {
		t.Fatal(err)
	}
	disabledID, err := disabledResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	balance, err := compatWalletBalance(ctx, tx, 9_999_999_999)
	if err != nil {
		t.Fatal(err)
	}
	if payList := balance["paylist"].([]map[string]any); len(payList) != 0 {
		t.Fatalf("paylist must stay empty when all channels are disabled: %#v", payList)
	}
	for _, rule := range balance["rules"].([]map[string]any) {
		id := rule["id"].(int64)
		if id == disabledID {
			t.Fatalf("disabled recharge product was returned: %#v", rule)
		}
		if _, exists := enabledIDs[id]; exists {
			enabledIDs[id] = true
		}
	}
	for id, found := range enabledIDs {
		if !found {
			t.Errorf("enabled product %d was hidden without a payment channel", id)
		}
	}

	channelKey := "wallet-test-" + suffix
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO payment_channels
			(channel_key,name,provider,currency,currency_scale,min_amount_minor,max_amount_minor,status)
		VALUES(?, 'wallet catalog test', 'test', 'TST', 2, 900000002, 900000004, 1)`,
		channelKey,
	); err != nil {
		t.Fatal(err)
	}
	balance, err = compatWalletBalance(ctx, tx, 9_999_999_999)
	if err != nil {
		t.Fatal(err)
	}
	if payList := balance["paylist"].([]map[string]any); len(payList) != 1 || payList[0]["id"] != channelKey {
		t.Fatalf("enabled test channel is missing from paylist: %#v", payList)
	}
	for id := range enabledIDs {
		enabledIDs[id] = false
	}
	for _, rule := range balance["rules"].([]map[string]any) {
		id := rule["id"].(int64)
		if _, exists := enabledIDs[id]; exists {
			enabledIDs[id] = true
		}
	}
	for id, found := range enabledIDs {
		var amountMinor int64
		if err = tx.QueryRowContext(ctx, "SELECT amount_minor FROM recharge_products WHERE id=?", id).Scan(&amountMinor); err != nil {
			t.Fatal(err)
		}
		want := amountMinor >= 900_000_002 && amountMinor <= 900_000_004
		if found != want {
			t.Errorf("channel amount compatibility for product %d: found=%t want=%t", id, found, want)
		}
	}
}
