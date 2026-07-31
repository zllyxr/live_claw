package bankpayment

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	KeyVersion = 1
	maxPayload = 16 << 10
)

var magic = []byte("BANKPAY1")

type AccountSecret struct {
	HolderName string `json:"holder_name"`
	CardNumber string `json:"card_number"`
}

type AccountSnapshot struct {
	DisplayName  string `json:"display_name"`
	BankName     string `json:"bank_name"`
	BranchName   string `json:"branch_name"`
	HolderName   string `json:"holder_name"`
	CardNumber   string `json:"card_number"`
	Instructions string `json:"instructions"`
}

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(masterKey string) (*Cipher, error) {
	if len(strings.TrimSpace(masterKey)) < 16 {
		return nil, errors.New("bank payment encryption key is too short")
	}
	key := sha256.Sum256([]byte(masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("initialize bank payment cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize bank payment AEAD: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) EncryptAccount(accountHash string, value AccountSecret) ([]byte, error) {
	return c.seal("account", accountHash, value)
}

func (c *Cipher) DecryptAccount(accountHash string, ciphertext []byte) (AccountSecret, error) {
	var value AccountSecret
	err := c.open("account", accountHash, ciphertext, &value)
	return value, err
}

func (c *Cipher) EncryptSnapshot(orderNo string, value AccountSnapshot) ([]byte, error) {
	return c.seal("snapshot", strings.ToLower(strings.TrimSpace(orderNo)), value)
}

func (c *Cipher) DecryptSnapshot(orderNo string, ciphertext []byte) (AccountSnapshot, error) {
	var value AccountSnapshot
	err := c.open("snapshot", strings.ToLower(strings.TrimSpace(orderNo)), ciphertext, &value)
	return value, err
}

func (c *Cipher) seal(kind, scope string, value any) ([]byte, error) {
	if c == nil || c.aead == nil || strings.TrimSpace(scope) == "" {
		return nil, errors.New("bank payment cipher is not initialized")
	}
	plaintext, err := json.Marshal(value)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxPayload {
		return nil, errors.New("bank payment payload is invalid")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate bank payment nonce: %w", err)
	}
	result := make([]byte, 0, len(magic)+1+len(nonce)+len(plaintext)+c.aead.Overhead())
	result = append(result, magic...)
	result = append(result, byte(KeyVersion))
	result = append(result, nonce...)
	return c.aead.Seal(result, nonce, plaintext, []byte(kind+":"+scope)), nil
}

func (c *Cipher) open(kind, scope string, ciphertext []byte, destination any) error {
	if c == nil || c.aead == nil || strings.TrimSpace(scope) == "" {
		return errors.New("bank payment cipher is not initialized")
	}
	headerSize := len(magic) + 1
	if len(ciphertext) < headerSize+c.aead.NonceSize()+c.aead.Overhead() ||
		!bytes.Equal(ciphertext[:len(magic)], magic) || ciphertext[len(magic)] != byte(KeyVersion) {
		return errors.New("bank payment ciphertext is invalid")
	}
	nonceEnd := headerSize + c.aead.NonceSize()
	plaintext, err := c.aead.Open(
		nil, ciphertext[headerSize:nonceEnd], ciphertext[nonceEnd:], []byte(kind+":"+scope),
	)
	if err != nil || len(plaintext) > maxPayload {
		return errors.New("bank payment ciphertext authentication failed")
	}
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("bank payment payload is invalid")
	}
	return nil
}

func NormalizeCardNumber(value string) (string, error) {
	var result strings.Builder
	for _, character := range strings.TrimSpace(value) {
		switch {
		case unicode.IsDigit(character) && character <= unicode.MaxASCII:
			result.WriteRune(character)
		case unicode.IsSpace(character), character == '-', character == '_':
		default:
			return "", errors.New("银行卡号只能包含数字、空格或短横线")
		}
	}
	normalized := result.String()
	if len(normalized) < 12 || len(normalized) > 30 {
		return "", errors.New("银行卡号长度必须为 12 到 30 位")
	}
	return normalized, nil
}

func CardHash(cardNumber string) string {
	sum := sha256.Sum256([]byte(cardNumber))
	return hex.EncodeToString(sum[:])
}

func MaskCardNumber(cardNumber string) string {
	if len(cardNumber) <= 4 {
		return "****"
	}
	return "**** **** **** " + cardNumber[len(cardNumber)-4:]
}
