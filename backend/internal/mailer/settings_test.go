package mailer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSettingsEncryptionValidationAndRedaction(t *testing.T) {
	store, err := NewStore(nil, "email-test-key-at-least-sixteen", Config{})
	if err != nil {
		t.Fatal(err)
	}
	settings := Settings{Enabled: true, Config: Config{Host: "smtp.example.com", Port: "587", Username: "sender@example.com", Password: "secret-never-returned", From: "sender@example.com", TLSMode: "starttls"}}
	raw, err := store.encode(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), settings.Password) || strings.Contains(string(raw), settings.Username) {
		t.Fatal("plaintext configuration stored")
	}
	read, err := store.decode(raw)
	if err != nil || read != settings {
		t.Fatal("encrypted settings round trip failed")
	}
	public, _ := json.Marshal((Snapshot{Settings: settings, Version: 1}).Public())
	if strings.Contains(string(public), settings.Password) || strings.Contains(string(public), `"password":`) {
		t.Fatal("public response leaks password")
	}
	wrong, _ := NewStore(nil, "different-email-encryption-key", Config{})
	if _, err = wrong.decode(raw); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err = store.decode([]byte(`{"ciphertext":"invalid"}`)); err == nil {
		t.Fatal("corrupt settings accepted")
	}
	for _, change := range []func(*Settings){
		func(s *Settings) { s.Port = "70000" }, func(s *Settings) { s.Port = "abc" },
		func(s *Settings) { s.Host = "https://smtp.example.com" }, func(s *Settings) { s.Host = "smtp.example.com:25" },
		func(s *Settings) { s.TLSMode = "none" }, func(s *Settings) { s.Password = "" },
		func(s *Settings) { s.From = "invalid" }, func(s *Settings) { s.Username = "bad\r\nvalue" },
	} {
		candidate := settings
		change(&candidate)
		if _, err = normalizeSettings(candidate); err == nil {
			t.Fatal("invalid SMTP configuration accepted")
		}
	}
}
