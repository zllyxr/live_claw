package adminauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

func HashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	return hashPassword(password)
}

// HashMigratedPassword is only for transparently upgrading an already
// authenticated legacy account. New passwords must use HashPassword and its
// stronger length policy.
func HashMigratedPassword(password string) (string, error) {
	if password == "" || len(password) > 128 {
		return "", errors.New("legacy password length is invalid")
	}
	return hashPassword(password)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(encoded, password string) bool {
	if len(password) > 512 || !utf8.ValidString(password) {
		return false
	}
	memory, iterations, parallelism, salt, expected, err := parsePasswordHash(encoded)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func validatePassword(password string) error {
	if len(password) > 512 || !utf8.ValidString(password) {
		return errors.New("password must be valid UTF-8 and no more than 512 bytes")
	}
	characterCount := utf8.RuneCountInString(password)
	if characterCount < 12 {
		return errors.New("password must contain at least 12 characters")
	}
	if characterCount > 128 {
		return errors.New("password must not exceed 128 characters")
	}
	return nil
}

func parsePasswordHash(encoded string) (uint32, uint32, uint8, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return 0, 0, 0, nil, nil, errors.New("unsupported password hash")
	}
	var memory uint64
	var iterations uint64
	var parallelism uint64
	for _, item := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return 0, 0, 0, nil, nil, errors.New("invalid password parameters")
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return 0, 0, 0, nil, nil, errors.New("invalid password parameters")
		}
		switch key {
		case "m":
			memory = parsed
		case "t":
			iterations = parsed
		case "p":
			parallelism = parsed
		}
	}
	if memory < 8*1024 || memory > 256*1024 ||
		iterations < 1 || iterations > 10 ||
		parallelism < 1 || parallelism > 16 {
		return 0, 0, 0, nil, nil, errors.New("unsafe password parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return 0, 0, 0, nil, nil, errors.New("invalid password salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return 0, 0, 0, nil, nil, errors.New("invalid password key")
	}
	return uint32(memory), uint32(iterations), uint8(parallelism), salt, key, nil
}
