package slo

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var durationPattern = regexp.MustCompile(`^(\d+)(s|m|h|d)$`)

// ParseDuration parses duration strings like "5m", "1h", "30d"
func ParseDuration(s string) (time.Duration, error) {
	matches := durationPattern.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}

	unit := matches[2]
	switch unit {
	case "s":
		return time.Duration(value) * time.Second, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}
}
