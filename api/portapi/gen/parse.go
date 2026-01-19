package gen

import (
	"errors"
	"strconv"
	"time"
)

// parseInt parses a string to int.
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// parseInt8 parses a string to int8.
func parseInt8(s string) (int8, error) {
	v, err := strconv.ParseInt(s, 10, 8)
	if err != nil {
		return 0, err
	}
	return int8(v), nil
}

// parseInt16 parses a string to int16.
func parseInt16(s string) (int16, error) {
	v, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		return 0, err
	}
	return int16(v), nil
}

// parseInt32 parses a string to int32.
func parseInt32(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

// parseInt64 parses a string to int64.
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// parseUint parses a string to uint.
func parseUint(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

// parseUint8 parses a string to uint8.
func parseUint8(s string) (uint8, error) {
	v, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(v), nil
}

// parseUint16 parses a string to uint16.
func parseUint16(s string) (uint16, error) {
	v, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, err
	}
	return uint16(v), nil
}

// parseUint32 parses a string to uint32.
func parseUint32(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

// parseUint64 parses a string to uint64.
func parseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// parseFloat32 parses a string to float32.
func parseFloat32(s string) (float32, error) {
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0, err
	}
	return float32(v), nil
}

// parseFloat64 parses a string to float64.
func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// parseBool parses a string to bool.
// Accepts: "true", "True", "TRUE", "1" -> true
//
//	"false", "False", "FALSE", "0" -> false
func parseBool(s string) (bool, error) {
	switch s {
	case "true", "True", "TRUE", "1":
		return true, nil
	case "false", "False", "FALSE", "0":
		return false, nil
	default:
		return false, errors.New("invalid bool: " + s)
	}
}

// parseTime parses a string to time.Time using RFC3339 format.
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// parseInts parses a slice of strings to []int.
func parseInts(ss []string) ([]int, error) {
	result := make([]int, len(ss))
	for i, s := range ss {
		v, err := parseInt(s)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

// parseStrings is a no-op that returns the input slice.
// Provided for consistency with other parse functions.
func parseStrings(ss []string) ([]string, error) {
	return ss, nil
}

// parseBools parses a slice of strings to []bool.
func parseBools(ss []string) ([]bool, error) {
	result := make([]bool, len(ss))
	for i, s := range ss {
		v, err := parseBool(s)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}
