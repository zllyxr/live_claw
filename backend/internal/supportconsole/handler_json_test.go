package supportconsole

import (
	"encoding/json"
	"testing"
)

func TestExactInt64AcceptsStringAndLegacyNumberWithoutRounding(t *testing.T) {
	const expected int64 = 1785252579710207004

	for _, body := range []string{
		`{"target_agent_id":"1785252579710207004"}`,
		`{"target_agent_id":1785252579710207004}`,
	} {
		var request struct {
			TargetAgentID exactInt64 `json:"target_agent_id"`
		}
		if err := json.Unmarshal([]byte(body), &request); err != nil {
			t.Fatalf("decode %s: %v", body, err)
		}
		if int64(request.TargetAgentID) != expected {
			t.Fatalf(
				"target agent ID was rounded: got %d, want %d",
				request.TargetAgentID,
				expected,
			)
		}
	}
}

func TestExactInt64RejectsFloatingPointTransferID(t *testing.T) {
	var request struct {
		TargetAgentID exactInt64 `json:"target_agent_id"`
	}
	if err := json.Unmarshal(
		[]byte(`{"target_agent_id":1785252579710207004.0}`),
		&request,
	); err == nil {
		t.Fatal("expected floating-point target_agent_id to be rejected")
	}
}

func TestExactInt64AcceptsStringAndLegacyNumberForAssetID(t *testing.T) {
	const expected int64 = 1785252579710207004

	for _, body := range []string{
		`{"asset_id":"1785252579710207004"}`,
		`{"asset_id":1785252579710207004}`,
	} {
		var request struct {
			AssetID exactInt64 `json:"asset_id"`
		}
		if err := json.Unmarshal([]byte(body), &request); err != nil {
			t.Fatalf("decode %s: %v", body, err)
		}
		if int64(request.AssetID) != expected {
			t.Fatalf("asset ID was rounded: got %d, want %d", request.AssetID, expected)
		}
	}
}
