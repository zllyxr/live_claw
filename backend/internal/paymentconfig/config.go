package paymentconfig

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

const (
	configVersion = 1
	maxConfigSize = 16 << 10
)

var (
	configMagic   = []byte("PAYCFG1")
	tradeTypeExpr = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,39}$`)
)

// ChannelConfig contains the provider settings stored in
// payment_channels.config_ciphertext. APIToken must never be returned by a
// public API, written to an audit log, or included in an error message.
type ChannelConfig struct {
	APIBaseURL     string `json:"api_base_url"`
	PublicBaseURL  string `json:"public_base_url"`
	APIToken       string `json:"api_token"`
	TradeType      string `json:"trade_type"`
	Fiat           string `json:"fiat"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// Cipher encrypts channel configuration with AES-256-GCM. The channel key is
// authenticated as additional data, preventing ciphertext from being copied
// from one payment channel to another.
type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(masterKey string) (*Cipher, error) {
	if len(strings.TrimSpace(masterKey)) < 16 {
		return nil, errors.New("payment configuration master key is too short")
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("initialize payment configuration cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize payment configuration AEAD: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(channelKey string, config ChannelConfig) ([]byte, error) {
	if c == nil || c.aead == nil {
		return nil, errors.New("payment configuration cipher is not initialized")
	}
	channelKey, err := validateChannelKey(channelKey)
	if err != nil {
		return nil, err
	}
	config, err = Normalize(config)
	if err != nil {
		return nil, err
	}
	plaintext, err := json.Marshal(config)
	if err != nil {
		return nil, errors.New("encode payment configuration")
	}
	if len(plaintext) > maxConfigSize {
		return nil, errors.New("payment configuration is too large")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate payment configuration nonce: %w", err)
	}
	result := make([]byte, 0, len(configMagic)+1+len(nonce)+len(plaintext)+c.aead.Overhead())
	result = append(result, configMagic...)
	result = append(result, byte(configVersion))
	result = append(result, nonce...)
	result = c.aead.Seal(result, nonce, plaintext, configAAD(channelKey))
	return result, nil
}

func (c *Cipher) Decrypt(channelKey string, ciphertext []byte) (ChannelConfig, error) {
	if c == nil || c.aead == nil {
		return ChannelConfig{}, errors.New("payment configuration cipher is not initialized")
	}
	channelKey, err := validateChannelKey(channelKey)
	if err != nil {
		return ChannelConfig{}, err
	}
	headerSize := len(configMagic) + 1
	if len(ciphertext) < headerSize+c.aead.NonceSize()+c.aead.Overhead() ||
		!bytes.Equal(ciphertext[:len(configMagic)], configMagic) ||
		int(ciphertext[len(configMagic)]) != configVersion {
		return ChannelConfig{}, errors.New("payment configuration ciphertext is invalid")
	}
	nonceStart := headerSize
	nonceEnd := nonceStart + c.aead.NonceSize()
	plaintext, err := c.aead.Open(
		nil,
		ciphertext[nonceStart:nonceEnd],
		ciphertext[nonceEnd:],
		configAAD(channelKey),
	)
	if err != nil {
		return ChannelConfig{}, errors.New("payment configuration authentication failed")
	}
	if len(plaintext) > maxConfigSize {
		return ChannelConfig{}, errors.New("payment configuration is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	var config ChannelConfig
	if err = decoder.Decode(&config); err != nil {
		return ChannelConfig{}, errors.New("payment configuration payload is invalid")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return ChannelConfig{}, errors.New("payment configuration payload has trailing data")
	}
	return Normalize(config)
}

func Validate(config ChannelConfig) error {
	_, err := Normalize(config)
	return err
}

// Normalize returns a validated canonical configuration. It intentionally does
// not expose or mask APIToken; callers serving administrator APIs must replace
// the token with a configured boolean instead of serializing this value.
func Normalize(config ChannelConfig) (ChannelConfig, error) {
	config.APIBaseURL = strings.TrimRight(strings.TrimSpace(config.APIBaseURL), "/")
	config.PublicBaseURL = strings.TrimRight(strings.TrimSpace(config.PublicBaseURL), "/")
	config.APIToken = strings.TrimSpace(config.APIToken)
	config.TradeType = strings.ToLower(strings.TrimSpace(config.TradeType))
	config.Fiat = strings.ToUpper(strings.TrimSpace(config.Fiat))

	if err := validateBaseURL("API", config.APIBaseURL); err != nil {
		return ChannelConfig{}, err
	}
	if err := validateBaseURL("public", config.PublicBaseURL); err != nil {
		return ChannelConfig{}, err
	}
	if len(config.APIToken) < 8 || len(config.APIToken) > 512 {
		return ChannelConfig{}, errors.New("payment API token length is invalid")
	}
	if !tradeTypeExpr.MatchString(config.TradeType) {
		return ChannelConfig{}, errors.New("payment trade type is invalid")
	}
	switch config.Fiat {
	case "CNY", "USD", "EUR", "GBP", "JPY":
	default:
		return ChannelConfig{}, errors.New("payment fiat currency is unsupported")
	}
	if config.TimeoutSeconds < 180 || config.TimeoutSeconds > 3600 {
		return ChannelConfig{}, errors.New("payment timeout must be between 180 and 3600 seconds")
	}
	return config, nil
}

func validateChannelKey(channelKey string) (string, error) {
	channelKey = strings.TrimSpace(channelKey)
	if channelKey == "" || len(channelKey) > 40 {
		return "", errors.New("payment channel key is invalid")
	}
	for _, value := range channelKey {
		if (value < 'a' || value > 'z') && (value < '0' || value > '9') &&
			value != '_' && value != '-' {
			return "", errors.New("payment channel key is invalid")
		}
	}
	return channelKey, nil
}

func validateBaseURL(label, raw string) error {
	if len(raw) > 1000 {
		return fmt.Errorf("payment %s base URL is too long", label)
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("payment %s base URL is invalid", label)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return fmt.Errorf("payment %s base URL must not contain a path", label)
	}
	return nil
}

func configAAD(channelKey string) []byte {
	return []byte("claw/payment-config/v1/" + channelKey)
}
