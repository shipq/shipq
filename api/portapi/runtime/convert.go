package runtime

import (
	"errors"
	"reflect"
	"strconv"
	"time"
)

// ErrConversion indicates a type conversion failure.
var ErrConversion = errors.New("conversion error")

// ConvertString converts a string value to the specified target type.
// Supported types: string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
// float32, float64, bool, and time.Time (RFC3339 format).
func ConvertString(s string, targetType reflect.Type) (reflect.Value, error) {
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(s), nil

	case reflect.Int:
		v, err := strconv.Atoi(s)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil

	case reflect.Int8:
		v, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int8(v)), nil

	case reflect.Int16:
		v, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int16(v)), nil

	case reflect.Int32:
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(int32(v)), nil

	case reflect.Int64:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil

	case reflect.Uint:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint(v)), nil

	case reflect.Uint8:
		v, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint8(v)), nil

	case reflect.Uint16:
		v, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint16(v)), nil

	case reflect.Uint32:
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(uint32(v)), nil

	case reflect.Uint64:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil

	case reflect.Float32:
		v, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(float32(v)), nil

	case reflect.Float64:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(v), nil

	case reflect.Bool:
		switch s {
		case "true", "True", "TRUE", "1":
			return reflect.ValueOf(true), nil
		case "false", "False", "FALSE", "0":
			return reflect.ValueOf(false), nil
		default:
			return reflect.Value{}, errors.New("invalid bool: " + s)
		}

	case reflect.Struct:
		if targetType == reflect.TypeOf(time.Time{}) {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(t), nil
		}
		fallthrough

	default:
		return reflect.Value{}, errors.New("unsupported type: " + targetType.String())
	}
}

// ConvertStrings converts a slice of strings to a slice of the specified element type.
// This is useful for multi-value query parameters.
func ConvertStrings(ss []string, elemType reflect.Type) (reflect.Value, error) {
	slice := reflect.MakeSlice(reflect.SliceOf(elemType), 0, len(ss))
	for _, s := range ss {
		v, err := ConvertString(s, elemType)
		if err != nil {
			return reflect.Value{}, err
		}
		slice = reflect.Append(slice, v)
	}
	return slice, nil
}
