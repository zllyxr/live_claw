package auth

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

func TestLegacyCMFPasswordVerification(t *testing.T) {
	service := New(nil, Options{LegacyAuthCode: "migration-secret", LegacyTablePrefix: "cmf_"})
	first := md5.Sum([]byte("migration-secret" + "UserPass123")) //nolint:gosec
	second := md5.Sum([]byte(hex.EncodeToString(first[:])))      //nolint:gosec
	encoded := "###" + hex.EncodeToString(second[:])
	if !service.verifyPassword("legacy_cmf", encoded, "UserPass123") {
		t.Fatal("valid legacy password was rejected")
	}
	if service.verifyPassword("legacy_cmf", encoded, "wrong") {
		t.Fatal("invalid legacy password was accepted")
	}
}

func TestLegacyPasswordRequiresConfiguredSecret(t *testing.T) {
	service := New(nil)
	if service.verifyPassword("legacy_cmf", "###anything", "password") {
		t.Fatal("legacy password was accepted without the migration secret")
	}
}
