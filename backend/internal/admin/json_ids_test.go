package admin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListUsersJSONKeepsExactNineteenDigitID(t *testing.T) {
	const userID int64 = 1785252579710207004
	payload := struct {
		Items []adminUserListItem `json:"items"`
	}{
		Items: []adminUserListItem{{ID: apiDecimalID(userID), Username: "precision_test"}},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"id":"1785252579710207004"`) {
		t.Fatalf("listUsers encoded an inexact or numeric ID: %s", encoded)
	}

	var decoded struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Items) != 1 || decoded.Items[0].ID != "1785252579710207004" {
		t.Fatalf("listUsers ID changed across JSON: %#v", decoded.Items)
	}
}

func TestDecimalIDInputAcceptsExactStringAndLegacyNumber(t *testing.T) {
	for _, input := range []string{
		`"1785252579710207004"`,
		`1785252579710207004`,
	} {
		var id decimalIDInput
		if err := json.Unmarshal([]byte(input), &id); err != nil {
			t.Fatalf("decode %s: %v", input, err)
		}
		if id.Int64() != 1785252579710207004 {
			t.Fatalf("decode %s changed ID to %d", input, id.Int64())
		}
	}
}

func TestDecimalIDInputsMarshalAsExactStrings(t *testing.T) {
	var ids decimalIDListInput
	if err := json.Unmarshal(
		[]byte(`["1785252579710207004",1785252579710207005]`),
		&ids,
	); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(ids)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `["1785252579710207004","1785252579710207005"]` {
		t.Fatalf("identifier list did not marshal as exact strings: %s", encoded)
	}
}

func TestAuditIdentifierValuesRemainExactStrings(t *testing.T) {
	got := jsonOrNil([]byte(
		`{"user_id":1785252579710207004,` +
			`"package_asset_id":1785252579710207005,` +
			`"winner_option_ids":[1785252579710207006],` +
			`"amount":1785252579710207004}`,
	))
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if !strings.Contains(text, `"user_id":"1785252579710207004"`) {
		t.Fatalf("audit user ID was not stringified exactly: %s", text)
	}
	if !strings.Contains(text, `"package_asset_id":"1785252579710207005"`) ||
		!strings.Contains(text, `"winner_option_ids":["1785252579710207006"]`) {
		t.Fatalf("nested entity IDs were not stringified exactly: %s", text)
	}
	if !strings.Contains(text, `"amount":1785252579710207004`) {
		t.Fatalf("non-ID audit value was unexpectedly converted: %s", text)
	}
}
