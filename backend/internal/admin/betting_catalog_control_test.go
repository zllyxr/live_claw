package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseControlIntFilter(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		wantValue int64
		wantSet   bool
		wantError bool
	}{
		{name: "absent", target: "/admin/api/bets/sports"},
		{name: "valid", target: "/admin/api/bets/sports?status=3", wantValue: 3, wantSet: true},
		{name: "zero", target: "/admin/api/bets/sports?status=0", wantValue: 0, wantSet: true},
		{name: "negative", target: "/admin/api/bets/sports?status=-1", wantError: true},
		{name: "above maximum", target: "/admin/api/bets/sports?status=5", wantError: true},
		{name: "not numeric", target: "/admin/api/bets/sports?status=won", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			value, set, err := parseControlIntFilter(request, "status", 0, 4)
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != test.wantValue || set != test.wantSet {
				t.Fatalf("unexpected result: value=%d set=%t", value, set)
			}
		})
	}
}

func TestSportsMarketUpdateRequestNormalize(t *testing.T) {
	tests := []struct {
		name      string
		request   sportsMarketUpdateRequest
		wantError bool
	}{
		{
			name: "valid and normalized",
			request: sportsMarketUpdateRequest{
				MarketCode: "  MATCH_RESULT ", Name: "  独赢  ",
				SettlementRule: "  match_result  ", Status: 0, SortOrder: 20,
			},
		},
		{
			name: "restored",
			request: sportsMarketUpdateRequest{
				MarketCode: "total_goals", Name: "大小球",
				SettlementRule: "total_goals", Status: 1,
			},
		},
		{
			name: "invalid code",
			request: sportsMarketUpdateRequest{
				MarketCode: "match result", Name: "独赢",
				SettlementRule: "match_result", Status: 1,
			},
			wantError: true,
		},
		{
			name: "invalid status",
			request: sportsMarketUpdateRequest{
				MarketCode: "match_result", Name: "独赢",
				SettlementRule: "match_result", Status: 2,
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.normalize()
			if (err != nil) != test.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !test.wantError && test.name == "valid and normalized" {
				if test.request.MarketCode != "match_result" ||
					test.request.Name != "独赢" ||
					test.request.SettlementRule != "match_result" {
					t.Fatalf("request was not normalized: %#v", test.request)
				}
			}
		})
	}
}

func TestControlListRejectsInvalidFiltersBeforeDatabase(t *testing.T) {
	handler := &Handler{}
	tests := []struct {
		name     string
		target   string
		endpoint http.HandlerFunc
	}{
		{
			name:     "lottery issue status",
			target:   "/admin/api/lottery/issues?status=6",
			endpoint: handler.listLotteryIssues,
		},
		{
			name:     "lottery bet user",
			target:   "/admin/api/bets/lottery?user_id=invalid",
			endpoint: handler.listBetLotteryOrders,
		},
		{
			name:     "sports bet status",
			target:   "/admin/api/bets/sports?status=5",
			endpoint: handler.listBetSportsOrders,
		},
		{
			name:     "game bet status",
			target:   "/admin/api/bets/game?status=4",
			endpoint: handler.listBetGameOrders,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			recorder := httptest.NewRecorder()
			test.endpoint.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
