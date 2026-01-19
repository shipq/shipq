//go:build property

package runtime

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestProperty_ConvertString_Roundtrip_Int64(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		original := r.Int63()
		if r.Float32() < 0.5 {
			original = -original
		}

		str := strconv.FormatInt(original, 10)
		v, err := ConvertString(str, reflect.TypeOf(int64(0)))
		if err != nil {
			t.Errorf("seed=%d: unexpected error: %v", seed, err)
			continue
		}

		got := v.Interface().(int64)
		if got != original {
			t.Errorf("seed=%d: roundtrip failed: got %d, want %d", seed, got, original)
		}
	}
}

func TestProperty_ConvertString_Roundtrip_Uint64(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		original := r.Uint64()

		str := strconv.FormatUint(original, 10)
		v, err := ConvertString(str, reflect.TypeOf(uint64(0)))
		if err != nil {
			t.Errorf("seed=%d: unexpected error: %v", seed, err)
			continue
		}

		got := v.Interface().(uint64)
		if got != original {
			t.Errorf("seed=%d: roundtrip failed: got %d, want %d", seed, got, original)
		}
	}
}

func TestProperty_ConvertString_Roundtrip_Float64(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		original := r.Float64() * 1000000
		if r.Float32() < 0.5 {
			original = -original
		}

		str := strconv.FormatFloat(original, 'f', -1, 64)
		v, err := ConvertString(str, reflect.TypeOf(float64(0)))
		if err != nil {
			t.Errorf("seed=%d: unexpected error: %v", seed, err)
			continue
		}

		got := v.Interface().(float64)
		// Allow small epsilon for float comparison
		epsilon := 0.0000001
		if got < original-epsilon || got > original+epsilon {
			t.Errorf("seed=%d: roundtrip failed: got %f, want %f", seed, got, original)
		}
	}
}

func TestProperty_ConvertString_Roundtrip_Bool(t *testing.T) {
	boolStrings := map[bool][]string{
		true:  {"true", "True", "TRUE", "1"},
		false: {"false", "False", "FALSE", "0"},
	}

	for expected, strings := range boolStrings {
		for _, s := range strings {
			v, err := ConvertString(s, reflect.TypeOf(false))
			if err != nil {
				t.Errorf("unexpected error for %q: %v", s, err)
				continue
			}
			got := v.Interface().(bool)
			if got != expected {
				t.Errorf("got %v for %q, want %v", got, s, expected)
			}
		}
	}
}

func TestProperty_ConvertString_NeverPanics(t *testing.T) {
	// Test various edge case strings
	edgeCases := []string{
		"",
		" ",
		"   ",
		"\t",
		"\n",
		"\x00",
		"null",
		"nil",
		"undefined",
		"NaN",
		"Inf",
		"-Inf",
		"+Inf",
		"true",
		"false",
		"TRUE",
		"FALSE",
		"yes",
		"no",
		"on",
		"off",
		"0",
		"1",
		"-1",
		"42",
		"-42",
		"3.14",
		"-3.14",
		"1e10",
		"1e-10",
		"1E10",
		"0x10",
		"0o10",
		"0b10",
		"abc",
		"ABC",
		"abc123",
		"123abc",
		"hello world",
		"hello\nworld",
		"hello\tworld",
		"'hello'",
		"\"hello\"",
		"2024-01-15",
		"2024-01-15T10:30:00Z",
		"2024-01-15T10:30:00+00:00",
		"invalid-date",
		string([]byte{0xFF, 0xFE}),
		"🔥",
		"日本語",
		"مرحبا",
		" leading space",
		"trailing space ",
		" both spaces ",
	}

	// Generate some random strings
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		length := r.Intn(100)
		bytes := make([]byte, length)
		for j := 0; j < length; j++ {
			bytes[j] = byte(r.Intn(256))
		}
		edgeCases = append(edgeCases, string(bytes))
	}

	targetTypes := []reflect.Type{
		reflect.TypeOf(""),
		reflect.TypeOf(int(0)),
		reflect.TypeOf(int8(0)),
		reflect.TypeOf(int16(0)),
		reflect.TypeOf(int32(0)),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(uint(0)),
		reflect.TypeOf(uint8(0)),
		reflect.TypeOf(uint16(0)),
		reflect.TypeOf(uint32(0)),
		reflect.TypeOf(uint64(0)),
		reflect.TypeOf(float32(0)),
		reflect.TypeOf(float64(0)),
		reflect.TypeOf(false),
		reflect.TypeOf(time.Time{}),
	}

	for _, s := range edgeCases {
		for _, targetType := range targetTypes {
			t.Run(fmt.Sprintf("%s_to_%s", truncateString(s, 20), targetType.String()), func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic for input %q to type %s: %v", s, targetType, r)
					}
				}()

				// We don't care about the result, just that it doesn't panic
				_, _ = ConvertString(s, targetType)
			})
		}
	}
}

func TestProperty_ConvertStrings_NeverPanics(t *testing.T) {
	edgeCases := [][]string{
		nil,
		{},
		{""},
		{" "},
		{"a", "b", "c"},
		{"1", "2", "3"},
		{"1", "abc", "3"},
		{"true", "false"},
		{"yes", "no"},
	}

	targetTypes := []reflect.Type{
		reflect.TypeOf(""),
		reflect.TypeOf(int(0)),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(float64(0)),
		reflect.TypeOf(false),
	}

	for i, ss := range edgeCases {
		for _, targetType := range targetTypes {
			t.Run(fmt.Sprintf("case_%d_to_%s", i, targetType.String()), func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic for input %v to type %s: %v", ss, targetType, r)
					}
				}()

				// We don't care about the result, just that it doesn't panic
				_, _ = ConvertStrings(ss, targetType)
			})
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
