package payment

import (
	"bytes"
	"crypto/md5" //nolint:gosec // BEpusdt's wire protocol mandates MD5.
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

const maxCallbackBody = 64 << 10

// SignBEpusdtRequest returns the provider-protocol signature for request
// fields. It never mutates fields and never returns or embeds the API token.
// Callers may add the returned value as the "signature" field afterwards.
func SignBEpusdtRequest(fields map[string]any, apiToken string) (string, error) {
	apiToken = strings.TrimSpace(apiToken)
	if len(fields) == 0 || len(apiToken) < 8 || len(apiToken) > 512 {
		return "", ErrInvalidRequest
	}
	copied := make(map[string]any, len(fields))
	for key, value := range fields {
		if key != "signature" {
			copied[key] = value
		}
	}
	signature, err := signFields(copied, apiToken)
	if err != nil {
		return "", ErrInvalidRequest
	}
	return signature, nil
}

func signFields(fields map[string]any, token string) (string, error) {
	canonical, err := canonicalFields(fields)
	if err != nil {
		return "", err
	}
	sum := md5.Sum([]byte(canonical + token)) //nolint:gosec // Required for provider compatibility.
	return hex.EncodeToString(sum[:]), nil
}

func verifyFields(fields map[string]any, token string) (string, string, error) {
	rawSignature, ok := fields["signature"]
	if !ok {
		return "", "", ErrInvalidSignature
	}
	signature, ok := rawSignature.(string)
	signature = strings.ToLower(strings.TrimSpace(signature))
	if !ok || len(signature) != md5.Size*2 {
		return "", "", ErrInvalidSignature
	}
	decoded, err := hex.DecodeString(signature)
	if err != nil || len(decoded) != md5.Size {
		return "", "", ErrInvalidSignature
	}
	expected, err := signFields(fields, token)
	if err != nil {
		return "", "", ErrInvalidCallback
	}
	expectedBytes, _ := hex.DecodeString(expected)
	if subtle.ConstantTimeCompare(decoded, expectedBytes) != 1 {
		return "", "", ErrInvalidSignature
	}
	canonical, err := canonicalFields(fields)
	if err != nil {
		return "", "", ErrInvalidCallback
	}
	hash := sha256.Sum256([]byte(canonical + "&signature=" + signature))
	return signature, hex.EncodeToString(hash[:]), nil
}

func canonicalFields(fields map[string]any) (string, error) {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key != "signature" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value, include, err := providerString(fields[key])
		if err != nil {
			return "", fmt.Errorf("canonicalize payment field %q: %w", key, err)
		}
		if include {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, "&"), nil
}

func providerString(value any) (string, bool, error) {
	switch typed := value.(type) {
	case nil:
		return "", false, nil
	case string:
		if typed == "" {
			return "", false, nil
		}
		return typed, true, nil
	case json.Number:
		number, err := strconv.ParseFloat(typed.String(), 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return "", false, errors.New("invalid JSON number")
		}
		// BEpusdt decodes JSON into map[string]interface{}, which turns every
		// JSON number into float64 before fmt.Sprintf("%v") signs it. Mirror
		// that behavior even though the callback decoder uses UseNumber to
		// retain exact values for the separate amount validation.
		return strconv.FormatFloat(number, 'g', -1, 64), true, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", false, errors.New("invalid floating-point number")
		}
		return strconv.FormatFloat(typed, 'g', -1, 64), true, nil
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return "", false, errors.New("invalid floating-point number")
		}
		return strconv.FormatFloat(float64(typed), 'g', -1, 32), true, nil
	case int:
		return strconv.Itoa(typed), true, nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), true, nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), true, nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), true, nil
	case int64:
		return strconv.FormatInt(typed, 10), true, nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true, nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true, nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true, nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true, nil
	case uint64:
		return strconv.FormatUint(typed, 10), true, nil
	case bool:
		return strconv.FormatBool(typed), true, nil
	default:
		return "", false, errors.New("unsupported payment field type")
	}
}

func decodeCallbackFields(raw []byte) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > maxCallbackBody {
		return nil, ErrInvalidCallback
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, ErrInvalidCallback
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, ErrInvalidCallback
	}
	return fields, nil
}
