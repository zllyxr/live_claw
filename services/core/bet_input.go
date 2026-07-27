package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// betInteger accepts both JSON numbers and base-10 numeric strings. Older
// UniApp builds serialize database IDs as strings, while newer clients send
// them as numbers.
type betInteger int64

func (value *betInteger) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return err
		}
		text = strings.TrimSpace(text)
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer %q: %w", text, err)
	}
	*value = betInteger(parsed)
	return nil
}

func decodeBetItems[T any](raw string) ([]T, error) {
	var items []T
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&items); err != nil {
		return nil, err
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return nil, err
	}
	return items, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
