package payment

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const maxDecimalDigits = 64

// AmountMatchesMinor validates a provider decimal against an exact fixed-scale
// local amount without passing through binary floating point.
func AmountMatchesMinor(raw string, amount int64, scale int) bool {
	parsed, err := decimalToMinor(raw, scale)
	return err == nil && parsed == amount
}

// decimalToMinor converts a non-negative decimal to a fixed-scale integer
// without passing through binary floating point.
func decimalToMinor(raw string, scale int) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || scale < 0 || scale > 18 || len(raw) > maxDecimalDigits {
		return 0, errors.New("invalid decimal amount")
	}
	if raw[0] == '+' {
		raw = raw[1:]
	} else if raw[0] == '-' {
		return 0, errors.New("decimal amount must not be negative")
	}
	exponent := 0
	if index := strings.IndexAny(raw, "eE"); index >= 0 {
		if strings.IndexAny(raw[index+1:], "eE") >= 0 {
			return 0, errors.New("invalid decimal exponent")
		}
		parsed, err := strconv.Atoi(raw[index+1:])
		if err != nil || parsed < -18 || parsed > 18 {
			return 0, errors.New("invalid decimal exponent")
		}
		exponent = parsed
		raw = raw[:index]
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid decimal amount")
	}
	whole := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if whole == "" {
		whole = "0"
	}
	if fraction == "" && len(parts) == 2 {
		return 0, errors.New("invalid decimal amount")
	}
	if !asciiDigits(whole) || !asciiDigits(fraction) {
		return 0, errors.New("invalid decimal amount")
	}
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	fractionDigits := len(fraction) - exponent
	switch {
	case fractionDigits < scale:
		padding := scale - fractionDigits
		if len(digits)+padding > maxDecimalDigits {
			return 0, errors.New("decimal amount is too large")
		}
		digits += strings.Repeat("0", padding)
	case fractionDigits > scale:
		remove := fractionDigits - scale
		if remove >= len(digits) {
			if strings.Trim(digits, "0") != "" {
				return 0, errors.New("decimal amount has excessive precision")
			}
			digits = "0"
		} else {
			discarded := digits[len(digits)-remove:]
			if strings.Trim(discarded, "0") != "" {
				return 0, errors.New("decimal amount has excessive precision")
			}
			digits = digits[:len(digits)-remove]
		}
	}
	amount, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, errors.New("decimal amount is out of range")
	}
	return amount, nil
}

func formatMinorCanonical(amount int64, scale int) (string, error) {
	if amount < 0 || scale < 0 || scale > 18 {
		return "", errors.New("invalid minor amount")
	}
	raw := strconv.FormatInt(amount, 10)
	if scale == 0 {
		return raw, nil
	}
	if len(raw) <= scale {
		raw = strings.Repeat("0", scale-len(raw)+1) + raw
	}
	split := len(raw) - scale
	result := raw[:split] + "." + raw[split:]
	result = strings.TrimRight(result, "0")
	result = strings.TrimRight(result, ".")
	if result == "" {
		return "0", nil
	}
	return result, nil
}

func normalizePositiveDecimal(raw string, maximum int) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maximum || strings.ContainsAny(raw, "eE+-") {
		return "", errors.New("invalid provider amount")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || !asciiDigits(parts[0]) ||
		(len(parts) == 2 && (parts[1] == "" || !asciiDigits(parts[1]))) {
		return "", errors.New("invalid provider amount")
	}
	whole := strings.TrimLeft(parts[0], "0")
	if whole == "" {
		whole = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if whole == "0" && fraction == "" {
		return "", errors.New("provider amount must be positive")
	}
	if fraction == "" {
		return whole, nil
	}
	return fmt.Sprintf("%s.%s", whole, fraction), nil
}

func asciiDigits(value string) bool {
	if value == "" {
		return true
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
