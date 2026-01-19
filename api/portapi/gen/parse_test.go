package gen

import (
	"testing"
	"time"
)

func TestParseInt(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"-1", -1, false},
		{"42", 42, false},
		{"123456789", 123456789, false},
		{"-123456789", -123456789, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1.5", 0, true},
		{"1e5", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInt8(t *testing.T) {
	tests := []struct {
		input   string
		want    int8
		wantErr bool
	}{
		{"0", 0, false},
		{"127", 127, false},
		{"-128", -128, false},
		{"128", 0, true},  // overflow
		{"-129", 0, true}, // underflow
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseInt8(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt8(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt8(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInt16(t *testing.T) {
	tests := []struct {
		input   string
		want    int16
		wantErr bool
	}{
		{"0", 0, false},
		{"32767", 32767, false},
		{"-32768", -32768, false},
		{"32768", 0, true},  // overflow
		{"-32769", 0, true}, // underflow
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseInt16(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt16(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt16(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInt32(t *testing.T) {
	tests := []struct {
		input   string
		want    int32
		wantErr bool
	}{
		{"0", 0, false},
		{"2147483647", 2147483647, false},
		{"-2147483648", -2147483648, false},
		{"2147483648", 0, true},  // overflow
		{"-2147483649", 0, true}, // underflow
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseInt32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt32(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"0", 0, false},
		{"9223372036854775807", 9223372036854775807, false},
		{"-9223372036854775808", -9223372036854775808, false},
		{"9223372036854775808", 0, true},  // overflow
		{"-9223372036854775809", 0, true}, // underflow
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseInt64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInt64(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseInt64(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUint(t *testing.T) {
	tests := []struct {
		input   string
		want    uint
		wantErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"42", 42, false},
		{"-1", 0, true}, // negative
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUint(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseUint(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUint8(t *testing.T) {
	tests := []struct {
		input   string
		want    uint8
		wantErr bool
	}{
		{"0", 0, false},
		{"255", 255, false},
		{"256", 0, true}, // overflow
		{"-1", 0, true},  // negative
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUint8(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint8(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseUint8(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUint16(t *testing.T) {
	tests := []struct {
		input   string
		want    uint16
		wantErr bool
	}{
		{"0", 0, false},
		{"65535", 65535, false},
		{"65536", 0, true}, // overflow
		{"-1", 0, true},    // negative
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUint16(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint16(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseUint16(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUint32(t *testing.T) {
	tests := []struct {
		input   string
		want    uint32
		wantErr bool
	}{
		{"0", 0, false},
		{"4294967295", 4294967295, false},
		{"4294967296", 0, true}, // overflow
		{"-1", 0, true},         // negative
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUint32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseUint32(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseUint64(t *testing.T) {
	tests := []struct {
		input   string
		want    uint64
		wantErr bool
	}{
		{"0", 0, false},
		{"18446744073709551615", 18446744073709551615, false},
		{"18446744073709551616", 0, true}, // overflow
		{"-1", 0, true},                   // negative
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseUint64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUint64(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseUint64(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFloat32(t *testing.T) {
	tests := []struct {
		input   string
		want    float32
		wantErr bool
	}{
		{"0", 0, false},
		{"1.5", 1.5, false},
		{"-1.5", -1.5, false},
		{"3.14159", 3.14159, false},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFloat32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloat32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseFloat32(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"0", 0, false},
		{"1.5", 1.5, false},
		{"-1.5", -1.5, false},
		{"3.141592653589793", 3.141592653589793, false},
		{"1e10", 1e10, false},
		{"-1e-10", -1e-10, false},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFloat64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFloat64(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseFloat64(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input   string
		want    bool
		wantErr bool
	}{
		// True values
		{"true", true, false},
		{"True", true, false},
		{"TRUE", true, false},
		{"1", true, false},
		// False values
		{"false", false, false},
		{"False", false, false},
		{"FALSE", false, false},
		{"0", false, false},
		// Invalid values
		{"yes", false, true},
		{"no", false, true},
		{"t", false, true},
		{"f", false, true},
		{"", false, true},
		{"invalid", false, true},
		{"tRuE", false, true}, // mixed case not supported
		{"fAlSe", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBool(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "RFC3339 UTC",
			input:   "2024-01-15T10:30:00Z",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "RFC3339 with offset",
			input:   "2024-01-15T10:30:00+05:00",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", 5*60*60)),
			wantErr: false,
		},
		{
			name:    "RFC3339 with negative offset",
			input:   "2024-01-15T10:30:00-08:00",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", -8*60*60)),
			wantErr: false,
		},
		{
			name:    "RFC3339 with nanoseconds",
			input:   "2024-01-15T10:30:00.123456789Z",
			want:    time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC),
			wantErr: false,
		},
		{
			name:    "date only - invalid",
			input:   "2024-01-15",
			wantErr: true,
		},
		{
			name:    "unix timestamp - invalid",
			input:   "1705315800",
			wantErr: true,
		},
		{
			name:    "empty string - invalid",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "not a date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseTime(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseInts(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []int
		wantErr bool
	}{
		{
			name:    "empty slice",
			input:   []string{},
			want:    []int{},
			wantErr: false,
		},
		{
			name:    "single value",
			input:   []string{"42"},
			want:    []int{42},
			wantErr: false,
		},
		{
			name:    "multiple values",
			input:   []string{"1", "2", "3"},
			want:    []int{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "with negative values",
			input:   []string{"-1", "0", "1"},
			want:    []int{-1, 0, 1},
			wantErr: false,
		},
		{
			name:    "invalid value in middle",
			input:   []string{"1", "abc", "3"},
			wantErr: true,
		},
		{
			name:    "invalid first value",
			input:   []string{"abc", "2", "3"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInts(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInts(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseInts(%v) len = %d, want %d", tt.input, len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseInts(%v)[%d] = %v, want %v", tt.input, i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestParseStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "single value",
			input: []string{"hello"},
			want:  []string{"hello"},
		},
		{
			name:  "multiple values",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with empty strings",
			input: []string{"", "hello", ""},
			want:  []string{"", "hello", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStrings(tt.input)
			if err != nil {
				t.Errorf("parseStrings(%v) unexpected error: %v", tt.input, err)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseStrings(%v) len = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseStrings(%v)[%d] = %v, want %v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseBools(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []bool
		wantErr bool
	}{
		{
			name:    "empty slice",
			input:   []string{},
			want:    []bool{},
			wantErr: false,
		},
		{
			name:    "single true",
			input:   []string{"true"},
			want:    []bool{true},
			wantErr: false,
		},
		{
			name:    "single false",
			input:   []string{"false"},
			want:    []bool{false},
			wantErr: false,
		},
		{
			name:    "multiple values",
			input:   []string{"true", "false", "1", "0"},
			want:    []bool{true, false, true, false},
			wantErr: false,
		},
		{
			name:    "invalid value",
			input:   []string{"true", "invalid", "false"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBools(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBools(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("parseBools(%v) len = %d, want %d", tt.input, len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("parseBools(%v)[%d] = %v, want %v", tt.input, i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}
