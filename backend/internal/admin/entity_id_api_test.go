package admin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zllyxr/live_claw/backend/internal/httpx"
)

type entityIDQueryFunc func(string) (driver.Rows, error)

type entityIDTestDriver struct {
	query entityIDQueryFunc
}

func (testDriver *entityIDTestDriver) Open(string) (driver.Conn, error) {
	return &entityIDTestConn{query: testDriver.query}, nil
}

type entityIDTestConn struct {
	query entityIDQueryFunc
}

func (connection *entityIDTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (connection *entityIDTestConn) Close() error {
	return nil
}

func (connection *entityIDTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (connection *entityIDTestConn) QueryContext(
	_ context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	return connection.query(query)
}

type entityIDTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (rows *entityIDTestRows) Columns() []string {
	return rows.columns
}

func (rows *entityIDTestRows) Close() error {
	return nil
}

func (rows *entityIDTestRows) Next(destination []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(destination, rows.values[rows.index])
	rows.index++
	return nil
}

var entityIDDriverSequence atomic.Uint64

func entityIDTestDB(t *testing.T, query entityIDQueryFunc) *sql.DB {
	t.Helper()
	name := "admin_entity_id_" + strconv.FormatUint(entityIDDriverSequence.Add(1), 10)
	sql.Register(name, &entityIDTestDriver{query: query})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func entityIDAPIResponse(t *testing.T, handler http.HandlerFunc, request *http.Request) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	httpx.RequestContext(handler).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected response (%d): %s", recorder.Code, recorder.Body.String())
	}
	decoder := json.NewDecoder(strings.NewReader(recorder.Body.String()))
	decoder.UseNumber()
	var response map[string]any
	if err := decoder.Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["code"] != json.Number("0") {
		t.Fatalf("unexpected response envelope: %#v", response)
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing response data: %#v", response)
	}
	return data
}

func TestListAppReleasesAPIKeepsEntityIDsExact(t *testing.T) {
	const (
		releaseID = int64(1785252579710207004)
		assetID   = int64(1785252579710207005)
	)
	now := time.Unix(1_800_000_000, 0)
	db := entityIDTestDB(t, func(query string) (driver.Rows, error) {
		if strings.Contains(query, "SELECT COUNT(*)") {
			return &entityIDTestRows{
				columns: []string{"COUNT(*)"},
				values:  [][]driver.Value{{int64(1)}},
			}, nil
		}
		if strings.Contains(query, "FROM app_releases app_release") {
			return &entityIDTestRows{
				columns: []string{
					"id", "platform", "release_type", "version_name", "version_code",
					"min_native_code", "force_update", "silent_update", "rollout_percent",
					"package_asset_id", "package_size", "package_sha256", "release_notes",
					"status", "published_at", "created_at",
				},
				values: [][]driver.Value{{
					releaseID, "android", "wgt", "8.1.1", int64(811), int64(800),
					true, true, int64(100), assetID, int64(4096),
					strings.Repeat("a", 64), "precision", int64(1), nil, now,
				}},
			}, nil
		}
		return nil, errors.New("unexpected query")
	})
	handler := &Handler{db: db}
	data := entityIDAPIResponse(
		t,
		handler.listAppReleases,
		httptest.NewRequest(http.MethodGet, "/admin/api/app/releases?page=1&page_size=10", nil),
	)
	items := data["items"].([]any)
	item := items[0].(map[string]any)
	if item["id"] != "1785252579710207004" ||
		item["package_asset_id"] != "1785252579710207005" {
		t.Fatalf("app entity IDs were not exact decimal strings: %#v", item)
	}
	if item["package_size"] != json.Number("4096") ||
		item["status"] != json.Number("1") {
		t.Fatalf("app numeric fields unexpectedly changed type: %#v", item)
	}
}

func TestListSportsMarketsAPIKeepsNestedEntityIDsExact(t *testing.T) {
	const (
		matchID  = int64(1785252579710207004)
		marketID = int64(1785252579710207005)
		optionID = int64(1785252579710207006)
	)
	db := entityIDTestDB(t, func(query string) (driver.Rows, error) {
		if !strings.Contains(query, "FROM sports_markets market") {
			return nil, errors.New("unexpected query")
		}
		return &entityIDTestRows{
			columns: []string{
				"market_id", "market_code", "market_name", "settlement_rule",
				"market_status", "sort_order", "option_id", "option_code",
				"option_name", "odds_scaled", "option_result", "option_status",
			},
			values: [][]driver.Value{{
				marketID, "1x2", "胜平负", "result_option", int64(1), int64(0),
				optionID, "home", "主胜", int64(1_800_000), int64(0), int64(1),
			}},
		}, nil
	})
	handler := &Handler{db: db}
	request := httptest.NewRequest(http.MethodGet, "/admin/api/sports/matches/placeholder/markets", nil)
	request.SetPathValue("id", strconv.FormatInt(matchID, 10))
	data := entityIDAPIResponse(t, handler.listSportsMarkets, request)
	if data["match_id"] != "1785252579710207004" {
		t.Fatalf("match ID was not exact: %#v", data)
	}
	market := data["items"].([]any)[0].(map[string]any)
	option := market["options"].([]any)[0].(map[string]any)
	if market["id"] != "1785252579710207005" ||
		option["id"] != "1785252579710207006" {
		t.Fatalf("nested sports IDs were not exact decimal strings: %#v", market)
	}
	if option["odds_scaled"] != json.Number("1800000") ||
		option["status"] != json.Number("1") {
		t.Fatalf("sports numeric fields unexpectedly changed type: %#v", option)
	}
}
