package invite

import (
	"bytes"
	"strings"
	"testing"
)

func TestInviteCodeFormat(t *testing.T) {
	source := bytes.NewReader(bytes.Repeat([]byte{0, 11, 22, 35}, 16))
	team, err := GeneratePart(source, 3)
	if err != nil {
		t.Fatal(err)
	}
	personal, err := GeneratePart(source, 4)
	if err != nil {
		t.Fatal(err)
	}
	full := team + "-" + personal
	if !ValidTeamCode(team) || !ValidCode(full) {
		t.Fatalf("invalid generated code %q", full)
	}
	if full != strings.ToLower(full) || len(full) != 8 {
		t.Fatalf("unexpected generated code %q", full)
	}
}

func TestInviteCodeNormalizesUppercaseInput(t *testing.T) {
	if !ValidCode(" ABC-1234 ") || Normalize(" ABC-1234 ") != "abc-1234" {
		t.Fatal("uppercase user input should be accepted and normalized")
	}
	if !ValidCode("ABC1234") || Normalize("ABC1234") != "abc-1234" {
		t.Fatal("legacy compact input should be accepted and normalized")
	}
}

func TestInviteCodeRejectsInvalidShape(t *testing.T) {
	for _, value := range []string{"abcd-1234", "abc12345", "ab-1234", "abc-123"} {
		if ValidCode(value) {
			t.Fatalf("accepted invalid code %q", value)
		}
	}
}
