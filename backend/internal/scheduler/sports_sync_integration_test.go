package scheduler

import (
	"context"
	"database/sql"
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
	"github.com/zllyxr/live_claw/backend/internal/idgen"
	"github.com/zllyxr/live_claw/backend/migrations"
)

func sportsSyncTestDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("CLAW_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("CLAW_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func insertTrackedTestMatch(t *testing.T, db *sql.DB, ctx context.Context, source string, fixtureID int64, status string, settled int, kickoff time.Time) {
	t.Helper()
	publicID, err := idgen.New()
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, `
		INSERT INTO sports_matches
		(public_match_id,source,source_match_id,home_name,away_name,kickoff_at,
		 bet_close_at,match_status,settle_status,home_score,away_score)
		VALUES(?,?,?,'Home','Away',?,?,?, ?,0,3)`,
		publicID, source, strconv.FormatInt(fixtureID, 10), kickoff, kickoff.Add(-5*time.Minute), status, settled,
	)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM sports_matches WHERE id=?", id) })
}

func testUpstreamFixture(id int64, status string, home, away int, kickoff time.Time) apiFootballFixture {
	var fixture apiFootballFixture
	fixture.Fixture.ID = id
	fixture.Fixture.Timestamp = kickoff.Unix()
	fixture.Fixture.Status.Short = status
	fixture.Teams.Home.Name = "Home"
	fixture.Teams.Away.Name = "Away"
	fixture.Goals.Home = &home
	fixture.Goals.Away = &away
	return fixture
}

func TestSportsSyncTracksFixturesAfterTheyLeaveLiveFeedIntegration(t *testing.T) {
	db, ctx := sportsSyncTestDB(t)
	now := time.Now().Truncate(time.Second)
	base := time.Now().UnixNano()%1_000_000_000 + 1_000_000_000
	for i, row := range []struct {
		source, status string
		settled        int
		kickoff        time.Time
	}{
		{"api-football", "LIVE", 0, now.Add(-26 * time.Hour)}, // Crosses midnight.
		{"api-football", "NS", 0, now.Add(-3 * time.Hour)},    // Final-only coverage.
		{"api-football", "HT", 0, now.Add(-2 * time.Hour)},
		{"api-football", "LIVE", 0, now.Add(-2 * time.Hour)}, // Temporarily omitted upstream.
		{"api-football", "NS", 0, now.Add(time.Hour)},        // Future match.
		{"api-football", "LIVE", 2, now.Add(-3 * time.Hour)}, // Locked settlement.
		{"manual_admin", "LIVE", 0, now.Add(-3 * time.Hour)},
		{"api-football", "FT", 0, now.Add(-3 * time.Hour)},
	} {
		insertTrackedTestMatch(t, db, ctx, row.source, base+int64(i), row.status, row.settled, row.kickoff)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		query := request.URL.Query()
		if request.URL.Path != "/fixtures" || query.Get("live") != "" || query.Get("timezone") != "Asia/Shanghai" || request.Header.Get("x-apisports-key") != "test-key" {
			t.Errorf("unexpected fixture request: %s", request.URL)
		}
		want := fmt.Sprintf("%d-%d-%d-%d", base, base+1, base+2, base+3)
		response := []apiFootballFixture{
			testUpstreamFixture(base, "FT", 0, 4, now.Add(-26*time.Hour)),
			testUpstreamFixture(base+1, "FT", 1, 2, now.Add(-3*time.Hour)),
			testUpstreamFixture(base+2, "CANC", 0, 0, now.Add(-2*time.Hour)),
			// Neither an unexpected existing fixture nor an unknown fixture may be updated/inserted.
			testUpstreamFixture(base+4, "FT", 9, 9, now.Add(time.Hour)),
			testUpstreamFixture(base+100, "NS", 0, 0, now.Add(time.Hour)),
		}
		if requests == 2 {
			want = strconv.FormatInt(base+3, 10)
			response = []apiFootballFixture{testUpstreamFixture(base+3, "FT", 3, 1, now.Add(-2*time.Hour))}
		}
		if query.Get("ids") != want {
			t.Errorf("tracked IDs = %q, want %q", query.Get("ids"), want)
		}
		_ = json.NewEncoder(w).Encode(apiFootballFixtureResponse{Response: response})
	}))
	defer server.Close()
	newRunner := func() *Runner {
		runner := &Runner{db: db, now: func() time.Time { return now }}
		runner.ConfigureSportsSync(SportsSyncConfig{BaseURL: server.URL, APIKey: "test-key"})
		runner.sports.nextCatalog = now.Add(time.Hour)
		runner.sports.nextOdds = now.Add(time.Hour)
		return runner
	}
	runner := newRunner()
	if err := runner.runSportsSync(ctx); err != nil {
		t.Fatal(err)
	}
	assertMatch := func(offset int64, wantStatus string, wantHome, wantAway, wantSettled int) {
		t.Helper()
		var status string
		var home, away, settled int
		err := db.QueryRowContext(ctx, `SELECT match_status,home_score,away_score,settle_status FROM sports_matches WHERE source_match_id=?`, base+offset).Scan(&status, &home, &away, &settled)
		if err != nil || status != wantStatus || home != wantHome || away != wantAway || settled != wantSettled {
			t.Fatalf("fixture %d: got %s %d:%d settlement=%d err=%v", offset, status, home, away, settled, err)
		}
	}
	assertMatch(0, "FT", 0, 4, 1)
	assertMatch(1, "FT", 1, 2, 1)
	assertMatch(2, "CANCELLED", 0, 0, 1)
	assertMatch(3, "LIVE", 0, 3, 0)
	assertMatch(4, "NS", 0, 3, 0)
	assertMatch(5, "LIVE", 0, 3, 2)
	assertMatch(6, "LIVE", 0, 3, 0)
	assertMatch(7, "FT", 0, 3, 0)
	var inserted int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sports_matches WHERE source_match_id=?", base+100).Scan(&inserted); err != nil || inserted != 0 {
		t.Fatalf("unexpected fixture inserted: %d, %v", inserted, err)
	}
	// A fresh runner recovers the remaining fixture from the database after restart.
	if err := newRunner().runSportsSync(ctx); err != nil {
		t.Fatal(err)
	}
	assertMatch(3, "FT", 3, 1, 1)
	if count, err := newRunner().syncSportsTrackedFixtures(ctx); err != nil || count != 0 || requests != 2 {
		t.Fatalf("finished fixtures were polled again: count=%d requests=%d err=%v", count, requests, err)
	}
}

func TestSportsTrackedSyncBatchesAndRetainsFixturesOnErrorsIntegration(t *testing.T) {
	db, ctx := sportsSyncTestDB(t)
	now := time.Now().Truncate(time.Second)
	base := time.Now().UnixNano()%1_000_000_000 + 1_000_000_000
	for i := 0; i < 41; i++ {
		insertTrackedTestMatch(t, db, ctx, "api-football", base+int64(i), "NS", 0, now.Add(-time.Hour))
	}
	fail := true
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		ids := strings.Split(request.URL.Query().Get("ids"), "-")
		if len(ids) > 20 || len(ids) == 0 {
			t.Errorf("invalid batch size: %d", len(ids))
		}
		if fail && requests == 2 {
			_, _ = w.Write([]byte(`{"errors":{"rateLimit":"try later"},"response":[]}`))
			return
		}
		response := make([]apiFootballFixture, 0, len(ids))
		for _, id := range ids {
			parsed, _ := strconv.ParseInt(id, 10, 64)
			response = append(response, testUpstreamFixture(parsed, "NS", 0, 3, now.Add(-time.Hour)))
		}
		_ = json.NewEncoder(w).Encode(apiFootballFixtureResponse{Response: response})
	}))
	defer server.Close()
	runner := &Runner{db: db, now: func() time.Time { return now }}
	runner.ConfigureSportsSync(SportsSyncConfig{BaseURL: server.URL, APIKey: "test-key"})
	if count, err := runner.syncSportsTrackedFixtures(ctx); count != 20 || err == nil || !strings.Contains(err.Error(), "rateLimit") {
		t.Fatalf("expected upstream error after first batch, count=%d err=%v", count, err)
	}
	fail, requests = false, 0
	if count, err := runner.syncSportsTrackedFixtures(ctx); count != 41 || err != nil || requests != 3 {
		t.Fatalf("retry lost fixtures or exceeded batch limit: count=%d requests=%d err=%v", count, requests, err)
	}
}
