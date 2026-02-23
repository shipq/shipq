package proptest

import (
	"math"
)

// Charsets for string generation
const (
	CharsetAlpha      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsetAlphaLower = "abcdefghijklmnopqrstuvwxyz"
	CharsetAlphaUpper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsetDigits     = "0123456789"
	CharsetAlphaNum   = CharsetAlpha + CharsetDigits
	CharsetHex        = "0123456789abcdef"
	CharsetPrintable  = CharsetAlphaNum + " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	CharsetIdentStart = CharsetAlpha + "_"
	CharsetIdentBody  = CharsetAlphaNum + "_"
)

// =============================================================================
// Integer Generators
// =============================================================================

// Int returns a random int (can be negative).
func (g *Generator) Int() int {
	// Generate a full range int by using two int32s
	high := g.rng.Int31()
	if g.Bool() {
		high = -high
	}
	return int(high)
}

// IntRange returns a random int in [min, max].
// Panics if min > max.
func (g *Generator) IntRange(min, max int) int {
	if min > max {
		panic("proptest: IntRange min > max")
	}
	if min == max {
		return min
	}
	return min + g.rng.Intn(max-min+1)
}

// IntPositive returns a random positive int in [1, max].
func (g *Generator) IntPositive(max int) int {
	return g.IntRange(1, max)
}

// Int64 returns a random int64 (can be negative).
func (g *Generator) Int64() int64 {
	n := g.rng.Int63()
	if g.Bool() {
		n = -n
	}
	return n
}

// Int64Range returns a random int64 in [min, max].
// Panics if min > max.
func (g *Generator) Int64Range(min, max int64) int64 {
	if min > max {
		panic("proptest: Int64Range min > max")
	}
	if min == max {
		return min
	}
	diff := max - min + 1
	return min + g.rng.Int63n(diff)
}

// Uint64 returns a random uint64.
func (g *Generator) Uint64() uint64 {
	return uint64(g.rng.Int63())<<1 | uint64(g.rng.Int63n(2))
}

// =============================================================================
// Float Generators
// =============================================================================

// Float64Range returns a random float64 in [min, max).
func (g *Generator) Float64Range(min, max float64) float64 {
	return min + g.rng.Float64()*(max-min)
}

// Float64Normal returns a normally distributed float64 with mean 0 and stddev 1.
func (g *Generator) Float64Normal() float64 {
	return g.rng.NormFloat64()
}

// =============================================================================
// String Generators
// =============================================================================

// String returns a random printable ASCII string of length [0, maxLen].
func (g *Generator) String(maxLen int) string {
	return g.StringFrom(CharsetPrintable, maxLen)
}

// StringN returns a random printable ASCII string of length [minLen, maxLen].
func (g *Generator) StringN(minLen, maxLen int) string {
	return g.StringFromN(CharsetPrintable, minLen, maxLen)
}

// StringAlpha returns a random alphabetic string (a-zA-Z) of length [0, maxLen].
func (g *Generator) StringAlpha(maxLen int) string {
	return g.StringFrom(CharsetAlpha, maxLen)
}

// StringAlphaLower returns a random lowercase alphabetic string of length [0, maxLen].
func (g *Generator) StringAlphaLower(maxLen int) string {
	return g.StringFrom(CharsetAlphaLower, maxLen)
}

// StringAlphaNum returns a random alphanumeric string of length [0, maxLen].
func (g *Generator) StringAlphaNum(maxLen int) string {
	return g.StringFrom(CharsetAlphaNum, maxLen)
}

// StringFrom returns a random string using characters from the given charset,
// with length [0, maxLen].
func (g *Generator) StringFrom(charset string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	length := g.Intn(maxLen + 1)
	return g.stringOfLen(charset, length)
}

// StringFromN returns a random string using characters from the given charset,
// with length [minLen, maxLen].
func (g *Generator) StringFromN(charset string, minLen, maxLen int) string {
	if minLen > maxLen {
		panic("proptest: StringFromN minLen > maxLen")
	}
	length := g.IntRange(minLen, maxLen)
	return g.stringOfLen(charset, length)
}

// stringOfLen returns a string of exactly the given length from charset.
func (g *Generator) stringOfLen(charset string, length int) string {
	if length == 0 {
		return ""
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[g.Intn(len(charset))]
	}
	return string(b)
}

// Identifier returns a valid identifier (starts with letter or underscore,
// followed by alphanumeric or underscore) of length [1, maxLen].
func (g *Generator) Identifier(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 1
	}
	length := g.IntRange(1, maxLen)
	if length == 0 {
		length = 1
	}

	b := make([]byte, length)
	// First character must be letter or underscore
	b[0] = CharsetIdentStart[g.Intn(len(CharsetIdentStart))]
	// Rest can be alphanumeric or underscore
	for i := 1; i < length; i++ {
		b[i] = CharsetIdentBody[g.Intn(len(CharsetIdentBody))]
	}
	return string(b)
}

// IdentifierLower returns a valid lowercase identifier of length [1, maxLen].
func (g *Generator) IdentifierLower(maxLen int) string {
	if maxLen <= 0 {
		maxLen = 1
	}
	length := g.IntRange(1, maxLen)
	if length == 0 {
		length = 1
	}

	const startChars = CharsetAlphaLower + "_"
	const bodyChars = CharsetAlphaLower + CharsetDigits + "_"

	b := make([]byte, length)
	b[0] = startChars[g.Intn(len(startChars))]
	for i := 1; i < length; i++ {
		b[i] = bodyChars[g.Intn(len(bodyChars))]
	}
	return string(b)
}

// =============================================================================
// Byte Generators
// =============================================================================

// Bytes returns a random byte slice of length [0, maxLen].
func (g *Generator) Bytes(maxLen int) []byte {
	if maxLen <= 0 {
		return nil
	}
	length := g.Intn(maxLen + 1)
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(g.Intn(256))
	}
	return b
}

// BytesN returns a random byte slice of length [minLen, maxLen].
func (g *Generator) BytesN(minLen, maxLen int) []byte {
	if minLen > maxLen {
		panic("proptest: BytesN minLen > maxLen")
	}
	length := g.IntRange(minLen, maxLen)
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(g.Intn(256))
	}
	return b
}

// =============================================================================
// Special Value Generators
// =============================================================================

// Rune returns a random rune from the given string.
func (g *Generator) Rune(from string) rune {
	runes := []rune(from)
	return runes[g.Intn(len(runes))]
}

// Duration returns a random duration in [0, max).
func (g *Generator) Duration(max int64) int64 {
	if max <= 0 {
		return 0
	}
	return g.Int63n(max)
}

// Percentage returns a random float64 in [0.0, 1.0].
func (g *Generator) Percentage() float64 {
	return g.Float64()
}

// =============================================================================
// Edge Case Generators (for fuzzing)
// =============================================================================

// EdgeCaseInt returns an int that's likely to trigger edge cases.
func (g *Generator) EdgeCaseInt() int {
	edgeCases := []int{
		0,
		1,
		-1,
		math.MaxInt32,
		math.MinInt32,
		math.MaxInt,
		math.MinInt,
		127,
		-128,
		255,
		256,
		65535,
		65536,
	}
	// 50% chance of edge case, 50% chance of random
	if g.Bool() {
		return edgeCases[g.Intn(len(edgeCases))]
	}
	return g.Int()
}

// EdgeCaseString returns a string that's likely to trigger edge cases.
func (g *Generator) EdgeCaseString() string {
	edgeCases := []string{
		"",                      // empty
		" ",                     // single space
		"  ",                    // multiple spaces
		"\t",                    // tab
		"\n",                    // newline
		"\r\n",                  // CRLF
		"'",                     // single quote
		"''",                    // escaped single quote
		`"`,                     // double quote
		`""`,                    // escaped double quote
		`\`,                     // backslash
		`\\`,                    // escaped backslash
		"it's",                  // apostrophe
		`say "hello"`,           // embedded quotes
		"line1\nline2",          // multiline
		"col1\tcol2",            // tabs
		"NULL",                  // SQL keyword
		"null",                  // lowercase null
		"true",                  // boolean keyword
		"false",                 // boolean keyword
		"0",                     // numeric string
		"-1",                    // negative numeric
		"123.456",               // decimal string
		"日本語",                   // Japanese
		"中文",                    // Chinese
		"🎉",                     // emoji
		"hello🎉world",           // mixed with emoji
		"<script>",              // HTML-like
		"--",                    // SQL comment
		"/**/",                  // SQL block comment
		"; DROP TABLE users;",   // SQL injection
		"SELECT * FROM",         // SQL keywords
		string(make([]byte, 0)), // truly empty
	}
	// 70% chance of edge case, 30% chance of random
	if g.Float64() < 0.7 {
		return edgeCases[g.Intn(len(edgeCases))]
	}
	return g.String(50)
}

// EdgeCaseIdentifier returns an identifier that might be a reserved word or edge case.
func (g *Generator) EdgeCaseIdentifier() string {
	reservedWords := []string{
		"select", "from", "where", "table", "index",
		"create", "drop", "alter", "insert", "update",
		"delete", "and", "or", "not", "null", "true",
		"false", "is", "in", "like", "between", "join",
		"left", "right", "inner", "outer", "on", "as",
		"order", "by", "group", "having", "limit",
		"offset", "union", "all", "distinct", "case",
		"when", "then", "else", "end", "cast", "user",
		"key", "primary", "foreign", "references",
		"constraint", "unique", "check", "default",
	}
	// 50% reserved word, 50% random identifier
	if g.Bool() {
		return reservedWords[g.Intn(len(reservedWords))]
	}
	return g.Identifier(20)
}
