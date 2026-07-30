package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/database"
	"github.com/zllyxr/live_claw/backend/internal/httpx"
	"github.com/zllyxr/live_claw/backend/migrations"
)

type adminListEnvelope struct {
	Code int `json:"code"`
	Data struct {
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int64            `json:"total"`
		HasMore  bool             `json:"has_more"`
		Items    []map[string]any `json:"items"`
	} `json:"data"`
}

func readAdminList(
	t *testing.T,
	handler http.HandlerFunc,
	target string,
) adminListEnvelope {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	httpx.RequestContext(handler).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected list response (%d): %s", recorder.Code, recorder.Body.String())
	}
	var response adminListEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response %q: %v", recorder.Body.String(), err)
	}
	if response.Code != 0 {
		t.Fatalf("unexpected list envelope: %#v", response)
	}
	return response
}

func paginationTestCode(prefix, marker string, sequence int) string {
	value := prefix + marker + strconv.Itoa(sequence)
	if len(value) > 26 {
		value = value[:26]
	}
	return value + strings.Repeat("0", 26-len(value))
}

func TestAdminListPaginationIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	marker := suffix
	if len(marker) > 10 {
		marker = marker[len(marker)-10:]
	}
	userIDs := make([]int64, 0, 3)
	teamIDs := make([]int64, 0, 3)
	rechargeCodes := make([]string, 0, 3)
	withdrawCodes := make([]string, 0, 2)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, orderNo := range withdrawCodes {
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM withdraw_orders WHERE order_no=?", orderNo)
		}
		for _, orderNo := range rechargeCodes {
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM recharge_orders WHERE order_no=?", orderNo)
		}
		for _, userID := range userIDs {
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_ledger_entries WHERE user_id=?", userID)
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM wallet_accounts WHERE user_id=?", userID)
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM team_members WHERE user_id=?", userID)
		}
		for _, teamID := range teamIDs {
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM teams WHERE id=?", teamID)
		}
		for _, userID := range userIDs {
			_, _ = db.ExecContext(cleanupCtx, "DELETE FROM users WHERE id=?", userID)
		}
	})
	userRows := []struct {
		username string
		nickname string
		status   int
	}{
		{"page_" + marker + "_a", "literal%" + marker, 1},
		{"page_" + marker + "_b", "literalX" + marker, 1},
		{"page_" + marker + "_c", "冻结用户" + marker, 2},
	}
	for index, row := range userRows {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO users
				(username,country_code,mobile,email,password_hash,password_algo,nickname,status)
			VALUES(?,'86',?,?,'integration-test-only','argon2id',?,?)`,
			row.username,
			fmt.Sprintf("19%09d", (time.Now().UnixNano()+int64(index))%1000000000),
			fmt.Sprintf("pagination-%s-%d@example.test", marker, index),
			row.nickname,
			row.status,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		userID, _ := result.LastInsertId()
		userIDs = append(userIDs, userID)
	}

	teamCodes := []string{"x" + marker[len(marker)-2:], "y" + marker[len(marker)-2:], "z" + marker[len(marker)-2:]}
	teamRows := []struct {
		name   string
		status int
	}{
		{"Team%" + marker, 1},
		{"TeamX" + marker, 1},
		{"停用团队" + marker, 0},
	}
	for index, row := range teamRows {
		result, insertErr := db.ExecContext(ctx, `
			INSERT INTO teams(code,name,owner_user_id,status,created_by)
			VALUES(?,?,?,?,0)`,
			teamCodes[index], row.name, userIDs[index], row.status,
		)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
		teamID, _ := result.LastInsertId()
		teamIDs = append(teamIDs, teamID)
	}
	for index, userID := range userIDs {
		if _, err = db.ExecContext(ctx, `
			INSERT INTO team_members(user_id,team_id,inviter_user_id,status)
			VALUES(?,?,0,1)`,
			userID, teamIDs[index],
		); err != nil {
			t.Fatal(err)
		}
	}

	accountResult, err := db.ExecContext(ctx, `
		INSERT INTO wallet_accounts(user_id,currency,available,frozen,status)
		VALUES(?,'COIN',100,0,1)`,
		userIDs[0],
	)
	if err != nil {
		t.Fatal(err)
	}
	accountID, _ := accountResult.LastInsertId()
	ledgerDescriptions := []string{"流水%" + marker, "流水X" + marker, "第三条流水" + marker}
	for index, description := range ledgerDescriptions {
		_, err = db.ExecContext(ctx, `
			INSERT INTO wallet_ledger_entries
				(entry_no,account_id,user_id,delta_available,delta_frozen,
				 balance_available,balance_frozen,business_type,business_id,direction,
				 game_code,venue_code,round_no,description)
			VALUES(?,?,?,1,0,?,?,?, ?,1,?,?,?,?)`,
			paginationTestCode("L", marker, index),
			accountID,
			userIDs[0],
			50+index,
			0,
			"pagination_"+marker,
			paginationTestCode("B", marker, index),
			"fish_"+marker,
			"venue_"+marker,
			"round_"+marker,
			description,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	for index, status := range []int{0, 0, 2} {
		orderNo := paginationTestCode("R", marker, index)
		rechargeCodes = append(rechargeCodes, orderNo)
		_, err = db.ExecContext(ctx, `
			INSERT INTO recharge_orders
				(order_no,user_id,product_id,channel_id,fiat_currency,currency_scale,
				 amount_minor,coin_amount,bonus_coin,status)
			VALUES(?,?,0,0,'CNY',2,100,100,0,?)`,
			orderNo, userIDs[0], status,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	for index, status := range []int{0, 3} {
		orderNo := paginationTestCode("W", marker, index)
		withdrawCodes = append(withdrawCodes, orderNo)
		_, err = db.ExecContext(ctx, `
			INSERT INTO withdraw_orders
				(order_no,user_id,account_id,hold_no,coin_amount,fee_coin,payout_currency,
				 currency_scale,payout_amount_minor,account_snapshot_ciphertext,account_masked,status)
			VALUES(?,?,0,?,100,1,'CNY',2,99,?,'***0000',?)`,
			orderNo,
			userIDs[0],
			paginationTestCode("H", marker, index),
			[]byte("integration-ciphertext"),
			status,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	handler := &Handler{db: db}
	t.Run("empty filters keep default pagination", func(t *testing.T) {
		checks := []struct {
			name       string
			endpoint   http.HandlerFunc
			target     string
			minimum    int64
			defaultCap int
		}{
			{name: "users", endpoint: handler.listUsers, target: "/admin/api/users", minimum: 3, defaultCap: 20},
			{name: "ledger", endpoint: handler.listWalletLedger, target: "/admin/api/wallet/ledger", minimum: 3, defaultCap: 20},
			{name: "recharges", endpoint: handler.listRechargeOrders, target: "/admin/api/wallet/recharges", minimum: 3, defaultCap: 20},
			{name: "withdrawals", endpoint: handler.listWithdrawOrders, target: "/admin/api/wallet/withdrawals", minimum: 2, defaultCap: 20},
			{name: "teams", endpoint: handler.listTeams, target: "/admin/api/teams", minimum: 4, defaultCap: 20},
		}
		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
				response := readAdminList(t, check.endpoint, check.target)
				if response.Data.Page != 1 || response.Data.PageSize != check.defaultCap ||
					response.Data.Total < check.minimum || len(response.Data.Items) > check.defaultCap {
					t.Fatalf("unexpected default pagination: %#v", response.Data)
				}
			})
		}
	})
	t.Run("users metadata and literal like escaping", func(t *testing.T) {
		response := readAdminList(
			t,
			handler.listUsers,
			"/admin/api/users?q=page_"+
				marker+"&status=1&page=1&page_size=1",
		)
		if response.Data.Total != 2 || len(response.Data.Items) != 1 || !response.Data.HasMore ||
			response.Data.Page != 1 || response.Data.PageSize != 1 {
			t.Fatalf("unexpected user pagination: %#v", response.Data)
		}
		escaped := readAdminList(
			t,
			handler.listUsers,
			"/admin/api/users?q=literal%25"+marker+"&status=1",
		)
		if escaped.Data.Total != 1 || len(escaped.Data.Items) != 1 || escaped.Data.HasMore {
			t.Fatalf("unexpected escaped user search: %#v", escaped.Data)
		}
	})

	t.Run("wallet ledger metadata and q search", func(t *testing.T) {
		response := readAdminList(
			t,
			handler.listWalletLedger,
			"/admin/api/wallet/ledger?q="+marker+"&user_id="+
				strconv.FormatInt(userIDs[0], 10)+"&page=1&page_size=2",
		)
		if response.Data.Total != 3 || len(response.Data.Items) != 2 || !response.Data.HasMore {
			t.Fatalf("unexpected wallet ledger pagination: %#v", response.Data)
		}
		escaped := readAdminList(
			t,
			handler.listWalletLedger,
			"/admin/api/wallet/ledger?q=%E6%B5%81%E6%B0%B4%25"+marker,
		)
		if escaped.Data.Total != 1 || len(escaped.Data.Items) != 1 {
			t.Fatalf("unexpected escaped ledger search: %#v", escaped.Data)
		}
	})

	t.Run("recharge and withdrawal filters", func(t *testing.T) {
		recharges := readAdminList(
			t,
			handler.listRechargeOrders,
			"/admin/api/wallet/recharges?q="+marker+"&status=0&page=1&page_size=1",
		)
		if recharges.Data.Total != 2 || len(recharges.Data.Items) != 1 || !recharges.Data.HasMore {
			t.Fatalf("unexpected recharge pagination: %#v", recharges.Data)
		}
		rechargesByUser := readAdminList(
			t,
			handler.listRechargeOrders,
			"/admin/api/wallet/recharges?q="+strconv.FormatInt(userIDs[0], 10),
		)
		if rechargesByUser.Data.Total != 3 || len(rechargesByUser.Data.Items) != 3 {
			t.Fatalf("unexpected recharge user search: %#v", rechargesByUser.Data)
		}
		withdrawals := readAdminList(
			t,
			handler.listWithdrawOrders,
			"/admin/api/wallet/withdrawals?q="+marker+"&status=3",
		)
		if withdrawals.Data.Total != 1 || len(withdrawals.Data.Items) != 1 || withdrawals.Data.HasMore {
			t.Fatalf("unexpected withdrawal filters: %#v", withdrawals.Data)
		}
	})

	t.Run("teams metadata and literal like escaping", func(t *testing.T) {
		response := readAdminList(
			t,
			handler.listTeams,
			"/admin/api/teams?q="+marker+"&status=1&page=1&page_size=1",
		)
		if response.Data.Total != 2 || len(response.Data.Items) != 1 || !response.Data.HasMore {
			t.Fatalf("unexpected team pagination: %#v", response.Data)
		}
		escaped := readAdminList(
			t,
			handler.listTeams,
			"/admin/api/teams?q=Team%25"+marker,
		)
		if escaped.Data.Total != 1 || len(escaped.Data.Items) != 1 || escaped.Data.HasMore {
			t.Fatalf("unexpected escaped team search: %#v", escaped.Data)
		}
	})
}
