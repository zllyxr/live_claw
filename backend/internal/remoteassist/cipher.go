package remoteassist

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
)

var cipherMagic = []byte("REMOTE1")

type secretCipher struct {
	aead cipher.AEAD
}

func newSecretCipher(masterKey string) (*secretCipher, error) {
	if len(strings.TrimSpace(masterKey)) < 16 {
		return nil, errors.New("remote assistance master key is too short")
	}
	key := sha256.Sum256([]byte("claw/remote-assistance/aes-256-gcm/v1\x00" + masterKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("initialize remote assistance cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize remote assistance AEAD: %w", err)
	}
	return &secretCipher{aead: aead}, nil
}

func (c *secretCipher) encrypt(scope string, plaintext []byte) ([]byte, error) {
	if c == nil || c.aead == nil || scope == "" || len(plaintext) == 0 || len(plaintext) > 4096 {
		return nil, errors.New("remote assistance secret is invalid")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("generate remote assistance nonce")
	}
	result := append(append([]byte{}, cipherMagic...), nonce...)
	return c.aead.Seal(result, nonce, plaintext, []byte("claw/remote-assistance/v1/"+scope)), nil
}

func (c *secretCipher) decrypt(scope string, ciphertext []byte) ([]byte, error) {
	if c == nil || c.aead == nil || scope == "" ||
		len(ciphertext) < len(cipherMagic)+c.aead.NonceSize()+c.aead.Overhead() ||
		!bytes.Equal(ciphertext[:len(cipherMagic)], cipherMagic) {
		return nil, errors.New("remote assistance ciphertext is invalid")
	}
	nonceStart := len(cipherMagic)
	nonceEnd := nonceStart + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, ciphertext[nonceStart:nonceEnd], ciphertext[nonceEnd:], []byte("claw/remote-assistance/v1/"+scope))
	if err != nil {
		return nil, errors.New("remote assistance secret authentication failed")
	}
	return plaintext, nil
}
