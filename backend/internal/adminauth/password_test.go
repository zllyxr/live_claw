package adminauth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(encoded, "correct horse battery staple") {
		t.Fatal("valid password was rejected")
	}
	if VerifyPassword(encoded, "incorrect horse battery staple") {
		t.Fatal("invalid password was accepted")
	}
}

func TestPasswordPolicy(t *testing.T) {
	if _, err := HashPassword("too-short"); err == nil {
		t.Fatal("short password was accepted")
	}
	if _, err := HashPassword(strings.Repeat("密", 12)); err != nil {
		t.Fatalf("12 multibyte characters were rejected: %v", err)
	}
	if _, err := HashPassword(strings.Repeat("🔐", 128)); err != nil {
		t.Fatalf("128 four-byte characters were rejected: %v", err)
	}
	if _, err := HashPassword(strings.Repeat("🔐", 129)); err == nil {
		t.Fatal("129 characters were accepted")
	}
	if _, err := HashPassword(string([]byte{0xff, 0xfe})); err == nil {
		t.Fatal("invalid UTF-8 password was accepted")
	}
	if VerifyPassword("not-a-hash", "anything") {
		t.Fatal("malformed hash was accepted")
	}
}

func TestValidPortalIncludesIsolatedAgentPortal(t *testing.T) {
	for _, portal := range []string{PortalAdmin, PortalAgent, PortalSupport} {
		if !validPortal(portal) {
			t.Errorf("portal %q was rejected", portal)
		}
	}
	for _, portal := range []string{"", "administrator", "agents", "support-console"} {
		if validPortal(portal) {
			t.Errorf("unknown portal %q was accepted", portal)
		}
	}
}
