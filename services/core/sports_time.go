package main

import "time"

const sportsTimezoneName = "Asia/Shanghai"

var sportsTimezone = func() *time.Location {
	location, err := time.LoadLocation(sportsTimezoneName)
	if err == nil {
		return location
	}
	return time.FixedZone("CST", 8*60*60)
}()

// normalizeSportsTimestamp keeps all collected and persisted sports times in
// Unix seconds. It also tolerates upstream millisecond/microsecond timestamps.
func normalizeSportsTimestamp(value int64) int64 {
	for value > 99_999_999_999 {
		value /= 1000
	}
	return value
}

func sportsTimestampText(value int64, layout string) string {
	value = normalizeSportsTimestamp(value)
	if value < 1 {
		return ""
	}
	return time.Unix(value, 0).In(sportsTimezone).Format(layout)
}

func sportsTimezoneOffset(value int64) int {
	if value < 1 {
		value = time.Now().Unix()
	}
	_, offset := time.Unix(value, 0).In(sportsTimezone).Zone()
	return offset
}

func sportsStatusAllowsBet(status string) bool {
	return status == "NS" || status == "TBD"
}

func sportsBetWindowOpen(match sportsMatch, now int64) bool {
	return match.BetStatus != 0 &&
		match.SettleStatus == 0 &&
		sportsStatusAllowsBet(match.Status) &&
		match.BetCloseTime > now
}

func effectiveSportsBetStatus(match sportsMatch, now int64) int {
	if match.BetStatus == 0 {
		return 0
	}
	if sportsBetWindowOpen(match, now) {
		return 1
	}
	return 2
}
