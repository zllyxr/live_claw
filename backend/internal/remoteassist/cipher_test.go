package remoteassist

import (
	"bytes"
	"strings"
	"testing"
)

func TestSecretCipherRoundTripAndScopeBinding(t *testing.T) {
	cipher, err := newSecretCipher("test-master-key-that-is-long-enough")
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("temporary-password-value")
	ciphertext, err := cipher.encrypt("credential/request-one", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	decoded, err := cipher.decrypt("credential/request-one", ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatalf("unexpected plaintext: %q", decoded)
	}
	if _, err = cipher.decrypt("credential/request-two", ciphertext); err == nil {
		t.Fatal("ciphertext was accepted in a different scope")
	}
}

func TestRandomPasswordPolicy(t *testing.T) {
	first, err := randomPassword(20)
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomPassword(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 20 || first == second {
		t.Fatalf("unexpected generated passwords: %q %q", first, second)
	}
	for _, value := range first {
		if !strings.ContainsRune(passwordAlphabet, value) {
			t.Fatalf("password contains unsupported character %q", value)
		}
	}
}

func TestEventMetadataRejectsSensitiveContentKeys(t *testing.T) {
	for _, key := range []string{"file_name", "clipboard_text", "audio_content", "password"} {
		if _, err := safeEventMetadata(map[string]any{key: "secret"}); err == nil {
			t.Fatalf("sensitive key %q was accepted", key)
		}
	}
	if _, err := safeEventMetadata(map[string]any{"nested": map[string]any{"clipboard": "secret"}}); err == nil {
		t.Fatal("nested sensitive metadata was accepted")
	}
	if _, err := safeEventMetadata(map[string]any{"direction": "incoming", "transport": "relay"}); err != nil {
		t.Fatalf("safe metadata was rejected: %v", err)
	}
	if _, err := safeEventMetadata(map[string]any{"result": "free-form-content"}); err == nil {
		t.Fatal("free-form event content was accepted")
	}
}

func TestRemoteStateAndAckMetadataAreAllowlisted(t *testing.T) {
	permissions := map[string]struct{}{"notification": {}}
	if _, err := safeBooleanState(map[string]any{"notification": true}, permissions); err != nil {
		t.Fatalf("valid permission state was rejected: %v", err)
	}
	if _, err := safeBooleanState(map[string]any{"password": "secret"}, permissions); err == nil {
		t.Fatal("unknown state metadata was accepted")
	}
	if _, err := safeAckResult(map[string]any{"code": "apply_failed"}); err != nil {
		t.Fatalf("valid ack code was rejected: %v", err)
	}
	if _, err := safeAckResult(map[string]any{"password": "secret"}); err == nil {
		t.Fatal("sensitive ack metadata was accepted")
	}
}

func TestRolloutGateAndIdentifiers(t *testing.T) {
	service, err := New(nil, "test-master-key-that-is-long-enough", Config{
		AllowedUserIDs: []int64{42},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !service.userEnabled(42) || service.userEnabled(43) {
		t.Fatal("staged user allowlist was not enforced")
	}
	for _, id := range []string{"123456789", "device_A-1"} {
		if !validDeviceCode(id) {
			t.Fatalf("valid device code %q was rejected", id)
		}
	}
	for _, id := range []string{"", "bad/id", "bad?query", "bad id"} {
		if validDeviceCode(id) {
			t.Fatalf("invalid device code %q was accepted", id)
		}
	}
}
