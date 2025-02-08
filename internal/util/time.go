package util

import (
	"fmt"
	"strconv"
)

// ParseExpirationTTL converts a TTL string (like "365d", "12h", or "30m") to seconds.
func ParseExpirationTTL(ttl string) (int64, error) {
	if len(ttl) < 2 {
		return 0, fmt.Errorf("invalid TTL format, expected format like '365d', '12h', or '30m'")
	}

	unit := ttl[len(ttl)-1]
	valueStr := ttl[:len(ttl)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %v", err)
	}

	switch unit {
	case 'd':
		return int64(value * 24 * 60 * 60), nil
	case 'h':
		return int64(value * 60 * 60), nil
	case 'm':
		return int64(value * 60), nil
	case 's':
		return int64(value), nil
	default:
		return 0, fmt.Errorf("invalid TTL unit %c, expected 'd', 'h', or 'm'", unit)
	}
}
