package runtime

import (
	"reflect"
	"testing"
	"time"
)

func TestConvertString(t *testing.T) {
	t.Run("string to string", func(t *testing.T) {
		v, err := ConvertString("hello", reflect.TypeOf(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != "hello" {
			t.Errorf("got %v, want %v", v.Interface(), "hello")
		}
	})

	t.Run("string to int", func(t *testing.T) {
		v, err := ConvertString("42", reflect.TypeOf(0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != 42 {
			t.Errorf("got %v, want %v", v.Interface(), 42)
		}
	})

	t.Run("string to int8", func(t *testing.T) {
		v, err := ConvertString("127", reflect.TypeOf(int8(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != int8(127) {
			t.Errorf("got %v, want %v", v.Interface(), int8(127))
		}
	})

	t.Run("string to int16", func(t *testing.T) {
		v, err := ConvertString("32767", reflect.TypeOf(int16(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != int16(32767) {
			t.Errorf("got %v, want %v", v.Interface(), int16(32767))
		}
	})

	t.Run("string to int32", func(t *testing.T) {
		v, err := ConvertString("2147483647", reflect.TypeOf(int32(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != int32(2147483647) {
			t.Errorf("got %v, want %v", v.Interface(), int32(2147483647))
		}
	})

	t.Run("string to int64", func(t *testing.T) {
		v, err := ConvertString("9223372036854775807", reflect.TypeOf(int64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != int64(9223372036854775807) {
			t.Errorf("got %v, want %v", v.Interface(), int64(9223372036854775807))
		}
	})

	t.Run("string to uint", func(t *testing.T) {
		v, err := ConvertString("42", reflect.TypeOf(uint(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != uint(42) {
			t.Errorf("got %v, want %v", v.Interface(), uint(42))
		}
	})

	t.Run("string to uint8", func(t *testing.T) {
		v, err := ConvertString("255", reflect.TypeOf(uint8(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != uint8(255) {
			t.Errorf("got %v, want %v", v.Interface(), uint8(255))
		}
	})

	t.Run("string to uint16", func(t *testing.T) {
		v, err := ConvertString("65535", reflect.TypeOf(uint16(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != uint16(65535) {
			t.Errorf("got %v, want %v", v.Interface(), uint16(65535))
		}
	})

	t.Run("string to uint32", func(t *testing.T) {
		v, err := ConvertString("4294967295", reflect.TypeOf(uint32(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != uint32(4294967295) {
			t.Errorf("got %v, want %v", v.Interface(), uint32(4294967295))
		}
	})

	t.Run("string to uint64", func(t *testing.T) {
		v, err := ConvertString("18446744073709551615", reflect.TypeOf(uint64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != uint64(18446744073709551615) {
			t.Errorf("got %v, want %v", v.Interface(), uint64(18446744073709551615))
		}
	})

	t.Run("string to bool true", func(t *testing.T) {
		for _, s := range []string{"true", "True", "TRUE", "1"} {
			v, err := ConvertString(s, reflect.TypeOf(false))
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", s, err)
			}
			if v.Interface() != true {
				t.Errorf("got %v for %q, want true", v.Interface(), s)
			}
		}
	})

	t.Run("string to bool false", func(t *testing.T) {
		for _, s := range []string{"false", "False", "FALSE", "0"} {
			v, err := ConvertString(s, reflect.TypeOf(false))
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", s, err)
			}
			if v.Interface() != false {
				t.Errorf("got %v for %q, want false", v.Interface(), s)
			}
		}
	})

	t.Run("string to float32", func(t *testing.T) {
		v, err := ConvertString("3.14", reflect.TypeOf(float32(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().(float32)
		if got < 3.13 || got > 3.15 {
			t.Errorf("got %v, want approximately 3.14", got)
		}
	})

	t.Run("string to float64", func(t *testing.T) {
		v, err := ConvertString("3.14159", reflect.TypeOf(float64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().(float64)
		if got < 3.14158 || got > 3.14160 {
			t.Errorf("got %v, want approximately 3.14159", got)
		}
	})

	t.Run("string to time.Time RFC3339", func(t *testing.T) {
		v, err := ConvertString("2024-01-15T10:30:00Z", reflect.TypeOf(time.Time{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
		if !v.Interface().(time.Time).Equal(expected) {
			t.Errorf("got %v, want %v", v.Interface(), expected)
		}
	})

	t.Run("string to time.Time with timezone", func(t *testing.T) {
		v, err := ConvertString("2024-01-15T10:30:00-05:00", reflect.TypeOf(time.Time{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00-05:00")
		if !v.Interface().(time.Time).Equal(expected) {
			t.Errorf("got %v, want %v", v.Interface(), expected)
		}
	})

	t.Run("negative int", func(t *testing.T) {
		v, err := ConvertString("-42", reflect.TypeOf(0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v.Interface() != -42 {
			t.Errorf("got %v, want %v", v.Interface(), -42)
		}
	})

	t.Run("negative float", func(t *testing.T) {
		v, err := ConvertString("-3.14", reflect.TypeOf(float64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().(float64)
		if got < -3.15 || got > -3.13 {
			t.Errorf("got %v, want approximately -3.14", got)
		}
	})
}

func TestConvertString_Errors(t *testing.T) {
	t.Run("empty string to int", func(t *testing.T) {
		_, err := ConvertString("", reflect.TypeOf(0))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("non-numeric to int", func(t *testing.T) {
		_, err := ConvertString("abc", reflect.TypeOf(0))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("overflow int8", func(t *testing.T) {
		_, err := ConvertString("128", reflect.TypeOf(int8(0)))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("overflow int", func(t *testing.T) {
		_, err := ConvertString("99999999999999999999", reflect.TypeOf(0))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("negative to uint", func(t *testing.T) {
		_, err := ConvertString("-1", reflect.TypeOf(uint(0)))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid bool", func(t *testing.T) {
		_, err := ConvertString("yes", reflect.TypeOf(false))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid bool - maybe", func(t *testing.T) {
		_, err := ConvertString("maybe", reflect.TypeOf(false))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid time format - date only", func(t *testing.T) {
		_, err := ConvertString("2024-01-15", reflect.TypeOf(time.Time{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid time format - unix timestamp", func(t *testing.T) {
		_, err := ConvertString("1705311000", reflect.TypeOf(time.Time{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("unsupported type - byte slice", func(t *testing.T) {
		_, err := ConvertString("data", reflect.TypeOf([]byte{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("unsupported type - struct", func(t *testing.T) {
		type custom struct{ X int }
		_, err := ConvertString("data", reflect.TypeOf(custom{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("unsupported type - map", func(t *testing.T) {
		_, err := ConvertString("data", reflect.TypeOf(map[string]string{}))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("invalid float", func(t *testing.T) {
		_, err := ConvertString("not-a-number", reflect.TypeOf(float64(0)))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("float with letters", func(t *testing.T) {
		_, err := ConvertString("3.14abc", reflect.TypeOf(float64(0)))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestConvertStrings(t *testing.T) {
	t.Run("converts slice of strings to strings", func(t *testing.T) {
		v, err := ConvertStrings([]string{"a", "b", "c"}, reflect.TypeOf(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().([]string)
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Errorf("got %v, want [a b c]", got)
		}
	})

	t.Run("converts slice of strings to ints", func(t *testing.T) {
		v, err := ConvertStrings([]string{"1", "2", "3"}, reflect.TypeOf(0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().([]int)
		if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
			t.Errorf("got %v, want [1 2 3]", got)
		}
	})

	t.Run("converts slice of strings to int64s", func(t *testing.T) {
		v, err := ConvertStrings([]string{"100", "200"}, reflect.TypeOf(int64(0)))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().([]int64)
		if len(got) != 2 || got[0] != 100 || got[1] != 200 {
			t.Errorf("got %v, want [100 200]", got)
		}
	})

	t.Run("converts slice of strings to bools", func(t *testing.T) {
		v, err := ConvertStrings([]string{"true", "false", "1"}, reflect.TypeOf(false))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().([]bool)
		if len(got) != 3 || got[0] != true || got[1] != false || got[2] != true {
			t.Errorf("got %v, want [true false true]", got)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		v, err := ConvertStrings([]string{}, reflect.TypeOf(0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := v.Interface().([]int)
		if len(got) != 0 {
			t.Errorf("got %v, want empty slice", got)
		}
	})

	t.Run("error on invalid element", func(t *testing.T) {
		_, err := ConvertStrings([]string{"1", "abc", "3"}, reflect.TypeOf(0))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
