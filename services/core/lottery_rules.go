package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type drawConfig struct {
	GameID      int64
	Template    string
	Count       int
	Min         int
	Max         int
	Unique      bool
	Pad         int
	SumBig      int
	ConfigState int
}

func generateOpenCode(config drawConfig) (string, error) {
	if config.Count < 1 || config.Max < config.Min {
		return "", fmt.Errorf("invalid draw config")
	}
	rangeSize := config.Max - config.Min + 1
	if config.Unique && config.Count > rangeSize {
		return "", fmt.Errorf("draw count exceeds unique range")
	}
	pool := make([]int, rangeSize)
	for index := range pool {
		pool[index] = config.Min + index
	}
	values := make([]string, 0, config.Count)
	for index := 0; index < config.Count; index++ {
		upper := rangeSize
		if config.Unique {
			upper = len(pool)
		}
		position, err := rand.Int(rand.Reader, big.NewInt(int64(upper)))
		if err != nil {
			return "", err
		}
		value := config.Min + int(position.Int64())
		if config.Unique {
			value = pool[position.Int64()]
			pool = append(pool[:position.Int64()], pool[position.Int64()+1:]...)
		}
		if config.Pad > 0 {
			values = append(values, fmt.Sprintf("%0*d", config.Pad, value))
		} else {
			values = append(values, strconv.Itoa(value))
		}
	}
	return strings.Join(values, ","), nil
}

func parseOpenCode(openCode string) []int {
	parts := strings.FieldsFunc(strings.TrimSpace(openCode), func(r rune) bool {
		return r == ',' || r == '，' || r == ' ' || r == '\t' || r == '\n'
	})
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err == nil {
			values = append(values, value)
		}
	}
	return values
}

var trailingNumber = regexp.MustCompile(`(\d+)$`)

func optionNumber(code string) (int, bool) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return 0, false
	}
	if value, err := strconv.Atoi(code); err == nil {
		return value, true
	}
	match := trailingNumber.FindStringSubmatch(code)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	return value, err == nil
}

func isLotteryWin(openCode, playCode, optionCode, resultRule string) bool {
	numbers := parseOpenCode(openCode)
	if len(numbers) == 0 {
		return false
	}
	optionCode = strings.ToUpper(strings.TrimSpace(optionCode))
	parts := strings.Split(strings.ToLower(strings.TrimSpace(resultRule)), ":")
	base := parts[0]
	sum := 0
	for _, value := range numbers {
		sum += value
	}

	switch {
	case base == "dragon_tiger":
		left, right := 0, len(numbers)-1
		if len(parts) > 1 {
			left, _ = strconv.Atoi(parts[1])
			left--
		}
		if len(parts) > 2 {
			right, _ = strconv.Atoi(parts[2])
			right--
		}
		if left < 0 || right < 0 || left >= len(numbers) || right >= len(numbers) {
			return false
		}
		if numbers[left] > numbers[right] {
			return optionCode == "DRAGON"
		}
		if numbers[left] < numbers[right] {
			return optionCode == "TIGER"
		}
		return optionCode == "TIE"
	case base == "sum_size":
		minimum, maximum := 0, 9
		if len(parts) > 1 {
			minimum, _ = strconv.Atoi(parts[1])
		}
		if len(parts) > 2 {
			maximum, _ = strconv.Atoi(parts[2])
		}
		if maximum < minimum {
			minimum, maximum = maximum, minimum
		}
		threshold := len(numbers) * (minimum + maximum) / 2
		return optionCode == "BIG" && sum > threshold || optionCode == "SMALL" && sum <= threshold
	case base == "sum_size_threshold":
		threshold := 0
		if len(parts) > 1 {
			threshold, _ = strconv.Atoi(parts[1])
		}
		return optionCode == "BIG" && sum > threshold || optionCode == "SMALL" && sum <= threshold
	case base == "sum_odd_even":
		return optionCode == "ODD" && sum%2 == 1 || optionCode == "EVEN" && sum%2 == 0
	case base == "exact_sum":
		target, ok := optionNumber(optionCode)
		return ok && sum == target
	case strings.HasPrefix(base, "position_"):
		position, _ := strconv.Atoi(strings.TrimPrefix(base, "position_"))
		target, ok := optionNumber(optionCode)
		return ok && position > 0 && position <= len(numbers) && numbers[position-1] == target
	case base == "contains_number":
		target, ok := optionNumber(optionCode)
		return ok && slices.Contains(numbers, target)
	case base == "k3_triple_any":
		return len(numbers) >= 3 && numbers[0] == numbers[1] && numbers[1] == numbers[2]
	case base == "k3_triple_exact":
		target, ok := optionNumber(optionCode)
		digit := firstDigit(target)
		return ok && len(numbers) >= 3 && numbers[0] == digit && numbers[1] == digit && numbers[2] == digit
	case base == "k3_pair_exact":
		target, ok := optionNumber(optionCode)
		if !ok {
			return false
		}
		digit, hits := firstDigit(target), 0
		for _, value := range numbers {
			if value == digit {
				hits++
			}
		}
		return hits >= 2
	case base == "pc28_extreme":
		return optionCode == "EXTREME_BIG" && sum >= 22 || optionCode == "EXTREME_SMALL" && sum <= 5
	case strings.HasPrefix(base, "lhc_special_"):
		special := numbers[len(numbers)-1]
		switch base {
		case "lhc_special_number":
			target, ok := optionNumber(optionCode)
			return ok && special == target
		case "lhc_special_size":
			return optionCode == "BIG" && special >= 25 || optionCode == "SMALL" && special <= 24
		case "lhc_special_odd_even":
			return optionCode == "ODD" && special%2 == 1 || optionCode == "EVEN" && special%2 == 0
		case "lhc_special_color":
			return optionCode == lhcColor(special)
		case "lhc_special_zodiac":
			return optionCode == lhcZodiac(special, time.Now().Year())
		}
	}
	_ = playCode
	return false
}

func firstDigit(value int) int {
	for value >= 10 {
		value /= 10
	}
	return value
}

func lhcColor(number int) string {
	red := []int{1, 2, 7, 8, 12, 13, 18, 19, 23, 24, 29, 30, 34, 35, 40, 45, 46}
	blue := []int{3, 4, 9, 10, 14, 15, 20, 25, 26, 31, 36, 37, 41, 42, 47, 48}
	if slices.Contains(red, number) {
		return "RED"
	}
	if slices.Contains(blue, number) {
		return "BLUE"
	}
	return "GREEN"
}

func lhcZodiac(number, year int) string {
	animals := []string{"RAT", "OX", "TIGER", "RABBIT", "DRAGON", "SNAKE", "HORSE", "GOAT", "MONKEY", "ROOSTER", "DOG", "PIG"}
	yearIndex := ((year-2020)%12 + 12) % 12
	index := (yearIndex - (number - 1)) % 12
	if index < 0 {
		index += 12
	}
	return animals[index]
}

func parseOddsScaled(value string) (int64, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, fmt.Errorf("invalid odds %q", value)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 4 {
		fraction = fraction[:4]
	}
	fraction += strings.Repeat("0", 4-len(fraction))
	fractionValue := int64(0)
	if fraction != "" {
		fractionValue, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return whole*10000 + fractionValue, nil
}
