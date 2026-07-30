package paymentconfig

import (
	"bytes"
	"strings"
	"testing"
)

func validConfig() ChannelConfig {
	return ChannelConfig{
		APIBaseURL:     "http://bepusdt:8080/",
		PublicBaseURL:  "https://pay.example.com/",
		APIToken:       "a-token-that-is-never-logged",
		TradeType:      "USDT.TRC20",
		Fiat:           "cny",
		TimeoutSeconds: 1200,
	}
}

func TestCipherRoundTripAndChannelBinding(t *testing.T) {
	cipher, err := NewCipher("0123456789abcdef-payment-test-key")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("usdt", validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted, []byte("a-token-that-is-never-logged")) {
		t.Fatal("ciphertext contains the plaintext API token")
	}
	decrypted, err := cipher.Decrypt("usdt", encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted.APIBaseURL != "http://bepusdt:8080" ||
		decrypted.PublicBaseURL != "https://pay.example.com" ||
		decrypted.TradeType != "usdt.trc20" ||
		decrypted.Fiat != "CNY" ||
		decrypted.APIToken != validConfig().APIToken {
		t.Fatalf("unexpected normalized configuration: %#v", decrypted)
	}
	if _, err = cipher.Decrypt("other", encrypted); err == nil {
		t.Fatal("ciphertext decrypted under a different channel key")
	}
}

func TestCipherRejectsTamperingWithoutLeakingToken(t *testing.T) {
	cipher, err := NewCipher("0123456789abcdef-payment-test-key")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("usdt", validConfig())
	if err != nil {
		t.Fatal(err)
	}
	encrypted[len(encrypted)-1] ^= 0xff
	_, err = cipher.Decrypt("usdt", encrypted)
	if err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
	if strings.Contains(err.Error(), validConfig().APIToken) {
		t.Fatal("decryption error leaked the API token")
	}
}

func TestValidateRejectsUnsafeOrIncompleteConfig(t *testing.T) {
	cases := []ChannelConfig{
		{},
		func() ChannelConfig {
			value := validConfig()
			value.APIBaseURL = "file:///tmp/provider"
			return value
		}(),
		func() ChannelConfig {
			value := validConfig()
			value.PublicBaseURL = "https://user:pass@pay.example.com"
			return value
		}(),
		func() ChannelConfig {
			value := validConfig()
			value.APIToken = "short"
			return value
		}(),
		func() ChannelConfig {
			value := validConfig()
			value.TradeType = "bad trade"
			return value
		}(),
		func() ChannelConfig {
			value := validConfig()
			value.Fiat = "USDT"
			return value
		}(),
		func() ChannelConfig {
			value := validConfig()
			value.TimeoutSeconds = 120
			return value
		}(),
	}
	for index, testCase := range cases {
		if err := Validate(testCase); err == nil {
			t.Fatalf("case %d unexpectedly passed", index)
		}
	}
}
