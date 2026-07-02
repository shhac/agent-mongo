package shared

import (
	"fmt"
	"strconv"
)

// ParsePositiveInt parses a string-typed numeric flag where empty means
// "unset" (returns 0 so the caller can apply a configured default).
func ParsePositiveInt(value, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("Invalid %s: %q. Must be a positive integer.", name, value)
	}
	return n, nil
}

// ParseNonNegativeInt is ParsePositiveInt but admits zero.
func ParseNonNegativeInt(value, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("Invalid %s: %q. Must be a non-negative integer.", name, value)
	}
	return n, nil
}
