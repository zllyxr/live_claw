package adminauth

import "testing"

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
	if VerifyPassword("not-a-hash", "anything") {
		t.Fatal("malformed hash was accepted")
	}
}
