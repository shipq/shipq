package proptest

import (
	"strings"
	"testing"
	"unicode"
)

// =============================================================================
// Generator Core Tests
// =============================================================================

func TestGenerator_Deterministic(t *testing.T) {
	// Same seed should produce same sequence
	seed := int64(12345)

	g1 := New(seed)
	g2 := New(seed)

	for i := 0; i < 100; i++ {
		v1 := g1.Intn(1000)
		v2 := g2.Intn(1000)
		if v1 != v2 {
			t.Errorf("same seed produced different values at iteration %d: %d vs %d", i, v1, v2)
		}
	}
}

func TestGenerator_DifferentSeeds(t *testing.T) {
	// Different seeds should (with high probability) produce different sequences
	g1 := New(12345)
	g2 := New(54321)

	same := 0
	for i := 0; i < 100; i++ {
		if g1.Intn(1000) == g2.Intn(1000) {
			same++
		}
	}

	// Allow some coincidental matches, but not too many
	if same > 20 {
		t.Errorf("different seeds produced too many same values: %d/100", same)
	}
}

func TestGenerator_Seed(t *testing.T) {
	seed := int64(99999)
	g := New(seed)
	if g.Seed() != seed {
		t.Errorf("Seed() = %d, want %d", g.Seed(), seed)
	}
}

func TestGenerator_ZeroSeed_UsesTime(t *testing.T) {
	g := New(0)
	if g.Seed() == 0 {
		t.Error("seed 0 should be replaced with time-based seed")
	}
}

// =============================================================================
// Integer Generator Tests
// =============================================================================

func TestIntRange_Bounds(t *testing.T) {
	g := New(42)
	min, max := 10, 20

	for i := 0; i < 1000; i++ {
		n := g.IntRange(min, max)
		if n < min || n > max {
			t.Errorf("IntRange(%d, %d) = %d, out of bounds", min, max, n)
		}
	}
}

func TestIntRange_SingleValue(t *testing.T) {
	g := New(42)
	for i := 0; i < 100; i++ {
		n := g.IntRange(5, 5)
		if n != 5 {
			t.Errorf("IntRange(5, 5) = %d, want 5", n)
		}
	}
}

func TestIntRange_Coverage(t *testing.T) {
	g := New(42)
	min, max := 0, 10
	seen := make(map[int]bool)

	for i := 0; i < 1000; i++ {
		seen[g.IntRange(min, max)] = true
	}

	// Should see all values in range
	for i := min; i <= max; i++ {
		if !seen[i] {
			t.Errorf("IntRange(%d, %d) never produced %d", min, max, i)
		}
	}
}

func TestInt64Range_Bounds(t *testing.T) {
	g := New(42)
	min, max := int64(1000000000), int64(1000000010)

	for i := 0; i < 1000; i++ {
		n := g.Int64Range(min, max)
		if n < min || n > max {
			t.Errorf("Int64Range(%d, %d) = %d, out of bounds", min, max, n)
		}
	}
}

func TestIntPositive(t *testing.T) {
	g := New(42)
	for i := 0; i < 1000; i++ {
		n := g.IntPositive(100)
		if n < 1 || n > 100 {
			t.Errorf("IntPositive(100) = %d, want [1, 100]", n)
		}
	}
}

// =============================================================================
// Float Generator Tests
// =============================================================================

func TestFloat64_Bounds(t *testing.T) {
	g := New(42)
	for i := 0; i < 1000; i++ {
		f := g.Float64()
		if f < 0.0 || f >= 1.0 {
			t.Errorf("Float64() = %f, want [0.0, 1.0)", f)
		}
	}
}

func TestFloat64Range_Bounds(t *testing.T) {
	g := New(42)
	min, max := 5.0, 10.0

	for i := 0; i < 1000; i++ {
		f := g.Float64Range(min, max)
		if f < min || f >= max {
			t.Errorf("Float64Range(%f, %f) = %f, out of bounds", min, max, f)
		}
	}
}

// =============================================================================
// String Generator Tests
// =============================================================================

func TestString_Length(t *testing.T) {
	g := New(42)
	maxLen := 20

	for i := 0; i < 1000; i++ {
		s := g.String(maxLen)
		if len(s) > maxLen {
			t.Errorf("String(%d) = %q (len %d), exceeds max length", maxLen, s, len(s))
		}
	}
}

func TestStringN_Length(t *testing.T) {
	g := New(42)
	minLen, maxLen := 5, 15

	for i := 0; i < 1000; i++ {
		s := g.StringN(minLen, maxLen)
		if len(s) < minLen || len(s) > maxLen {
			t.Errorf("StringN(%d, %d) = %q (len %d), out of bounds", minLen, maxLen, s, len(s))
		}
	}
}

func TestStringAlpha_Characters(t *testing.T) {
	g := New(42)
	for i := 0; i < 100; i++ {
		s := g.StringAlpha(50)
		for _, c := range s {
			if !unicode.IsLetter(c) {
				t.Errorf("StringAlpha() produced non-letter: %q in %q", c, s)
			}
		}
	}
}

func TestStringAlphaNum_Characters(t *testing.T) {
	g := New(42)
	for i := 0; i < 100; i++ {
		s := g.StringAlphaNum(50)
		for _, c := range s {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				t.Errorf("StringAlphaNum() produced invalid char: %q in %q", c, s)
			}
		}
	}
}

func TestIdentifier_Valid(t *testing.T) {
	g := New(42)
	for i := 0; i < 1000; i++ {
		s := g.Identifier(20)

		if len(s) == 0 {
			t.Error("Identifier() returned empty string")
			continue
		}

		// First char must be letter or underscore
		first := rune(s[0])
		if !unicode.IsLetter(first) && first != '_' {
			t.Errorf("Identifier() starts with invalid char: %q", s)
		}

		// Rest must be alphanumeric or underscore
		for _, c := range s[1:] {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
				t.Errorf("Identifier() contains invalid char: %q in %q", c, s)
			}
		}
	}
}

func TestIdentifier_MinLength(t *testing.T) {
	g := New(42)
	for i := 0; i < 100; i++ {
		s := g.Identifier(10)
		if len(s) < 1 {
			t.Error("Identifier() returned empty string")
		}
	}
}

// =============================================================================
// Byte Generator Tests
// =============================================================================

func TestBytes_Length(t *testing.T) {
	g := New(42)
	maxLen := 50

	for i := 0; i < 100; i++ {
		b := g.Bytes(maxLen)
		if len(b) > maxLen {
			t.Errorf("Bytes(%d) returned slice of length %d", maxLen, len(b))
		}
	}
}

func TestBytesN_Length(t *testing.T) {
	g := New(42)
	minLen, maxLen := 10, 20

	for i := 0; i < 100; i++ {
		b := g.BytesN(minLen, maxLen)
		if len(b) < minLen || len(b) > maxLen {
			t.Errorf("BytesN(%d, %d) returned slice of length %d", minLen, maxLen, len(b))
		}
	}
}

// =============================================================================
// Combinator Tests
// =============================================================================

func TestOneOf_Coverage(t *testing.T) {
	g := New(42)
	options := []string{"a", "b", "c", "d", "e"}
	seen := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		seen[OneOf(g, options...)] = true
	}

	for _, opt := range options {
		if !seen[opt] {
			t.Errorf("OneOf() never picked %q", opt)
		}
	}
}

func TestOneOfFunc_Works(t *testing.T) {
	g := New(42)

	genInt := func(g *Generator) int { return 1 }
	genInt2 := func(g *Generator) int { return 2 }

	seen1, seen2 := false, false
	for i := 0; i < 100; i++ {
		n := OneOfFunc(g, genInt, genInt2)
		if n == 1 {
			seen1 = true
		}
		if n == 2 {
			seen2 = true
		}
	}

	if !seen1 || !seen2 {
		t.Error("OneOfFunc() didn't pick all options")
	}
}

func TestPick_Works(t *testing.T) {
	g := New(42)
	slice := []int{10, 20, 30, 40, 50}
	seen := make(map[int]bool)

	for i := 0; i < 1000; i++ {
		seen[Pick(g, slice)] = true
	}

	for _, v := range slice {
		if !seen[v] {
			t.Errorf("Pick() never selected %d", v)
		}
	}
}

func TestSlice_Length(t *testing.T) {
	g := New(42)
	maxLen := 10

	for i := 0; i < 100; i++ {
		s := Slice(g, maxLen, func(g *Generator) int { return g.Int() })
		if len(s) > maxLen {
			t.Errorf("Slice(maxLen=%d) returned slice of length %d", maxLen, len(s))
		}
	}
}

func TestSliceN_Length(t *testing.T) {
	g := New(42)
	minLen, maxLen := 5, 10

	for i := 0; i < 100; i++ {
		s := SliceN(g, minLen, maxLen, func(g *Generator) int { return g.Int() })
		if len(s) < minLen || len(s) > maxLen {
			t.Errorf("SliceN(%d, %d) returned slice of length %d", minLen, maxLen, len(s))
		}
	}
}

func TestSliceExact_Length(t *testing.T) {
	g := New(42)
	for length := 0; length <= 20; length++ {
		s := SliceExact(g, length, func(g *Generator) int { return g.Int() })
		if len(s) != length {
			t.Errorf("SliceExact(%d) returned slice of length %d", length, len(s))
		}
	}
}

func TestMap_Size(t *testing.T) {
	g := New(42)
	maxSize := 10

	for i := 0; i < 100; i++ {
		m := Map(g, maxSize,
			func(g *Generator) string { return g.Identifier(10) },
			func(g *Generator) int { return g.Int() })
		if len(m) > maxSize {
			t.Errorf("Map(maxSize=%d) returned map of size %d", maxSize, len(m))
		}
	}
}

func TestPointer_NilChance(t *testing.T) {
	g := New(42)
	nilCount := 0
	total := 1000

	for i := 0; i < total; i++ {
		p := Pointer(g, 0.3, func(g *Generator) int { return g.Int() })
		if p == nil {
			nilCount++
		}
	}

	// Should be around 30% nil, allow some variance
	nilRate := float64(nilCount) / float64(total)
	if nilRate < 0.2 || nilRate > 0.4 {
		t.Errorf("Pointer(0.3) produced %.1f%% nils, expected ~30%%", nilRate*100)
	}
}

func TestFilter_Works(t *testing.T) {
	g := New(42)

	// Filter for even numbers
	val, ok := Filter(g, 1000, func(g *Generator) int { return g.IntRange(0, 100) }, func(n int) bool { return n%2 == 0 })

	if !ok {
		t.Error("Filter() failed to find even number")
	}
	if val%2 != 0 {
		t.Errorf("Filter() returned odd number: %d", val)
	}
}

func TestFilter_Fails(t *testing.T) {
	g := New(42)

	// Filter for impossible condition
	_, ok := Filter(g, 100, func(g *Generator) int { return g.IntRange(0, 10) }, func(n int) bool { return n > 100 })

	if ok {
		t.Error("Filter() should have failed for impossible condition")
	}
}

func TestShuffle_PreservesElements(t *testing.T) {
	g := New(42)
	original := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	shuffled := Shuffle(g, original)

	if len(shuffled) != len(original) {
		t.Errorf("Shuffle() changed length: %d -> %d", len(original), len(shuffled))
	}

	// Check all elements are present
	seen := make(map[int]bool)
	for _, v := range shuffled {
		seen[v] = true
	}
	for _, v := range original {
		if !seen[v] {
			t.Errorf("Shuffle() lost element %d", v)
		}
	}
}

func TestSample_UniqueElements(t *testing.T) {
	g := New(42)
	original := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	n := 5

	sample := Sample(g, original, n)

	if len(sample) != n {
		t.Errorf("Sample(%d) returned %d elements", n, len(sample))
	}

	// Check uniqueness
	seen := make(map[int]bool)
	for _, v := range sample {
		if seen[v] {
			t.Errorf("Sample() returned duplicate: %d", v)
		}
		seen[v] = true
	}
}

func TestWeighted_Biased(t *testing.T) {
	g := New(42)
	weights := []float64{1.0, 0.0} // Always pick first
	values := []string{"a", "b"}

	for i := 0; i < 100; i++ {
		result := Weighted(g, weights, values)
		if result != "a" {
			t.Errorf("Weighted with [1,0] should always return 'a', got %q", result)
		}
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestEdgeCaseString_Coverage(t *testing.T) {
	g := New(42)

	// Just verify it doesn't panic and produces varied output
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		s := g.EdgeCaseString()
		seen[s] = true
	}

	// Should produce multiple different values
	if len(seen) < 10 {
		t.Errorf("EdgeCaseString() only produced %d unique values", len(seen))
	}
}

func TestEdgeCaseIdentifier_IncludesReserved(t *testing.T) {
	g := New(42)

	foundReserved := false
	for i := 0; i < 1000; i++ {
		s := g.EdgeCaseIdentifier()
		// Check if it's a common reserved word
		lower := strings.ToLower(s)
		if lower == "select" || lower == "table" || lower == "from" || lower == "where" {
			foundReserved = true
			break
		}
	}

	if !foundReserved {
		t.Error("EdgeCaseIdentifier() never produced a reserved word")
	}
}

// =============================================================================
// Property Runner Tests
// =============================================================================

func TestQuickCheck_Passes(t *testing.T) {
	QuickCheck(t, "always true", func(g *Generator) bool {
		return true
	})
}

func TestForAll_Passes(t *testing.T) {
	ForAll(t, "positive in range", 100, func(g *Generator) (int, bool) {
		n := g.IntRange(1, 100)
		return n, n >= 1 && n <= 100
	})
}

func TestForAll2_Passes(t *testing.T) {
	ForAll2(t, "sum is commutative", 100, func(g *Generator) (int, int, bool) {
		a := g.IntRange(0, 100)
		b := g.IntRange(0, 100)
		return a, b, a+b == b+a
	})
}

// =============================================================================
// Assertion Helper Tests
// =============================================================================

func TestAssertInRange(t *testing.T) {
	if !AssertInRange(5, 0, 10) {
		t.Error("5 should be in [0, 10]")
	}
	if AssertInRange(15, 0, 10) {
		t.Error("15 should not be in [0, 10]")
	}
}

func TestAssertLenInRange(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	if !AssertLenInRange(slice, 3, 7) {
		t.Error("slice of len 5 should be in [3, 7]")
	}
	if AssertLenInRange(slice, 10, 20) {
		t.Error("slice of len 5 should not be in [10, 20]")
	}
}

func TestAssertAllSatisfy(t *testing.T) {
	positive := []int{1, 2, 3, 4, 5}
	mixed := []int{1, -2, 3, -4, 5}

	if !AssertAllSatisfy(positive, func(n int) bool { return n > 0 }) {
		t.Error("all positive should satisfy > 0")
	}
	if AssertAllSatisfy(mixed, func(n int) bool { return n > 0 }) {
		t.Error("mixed should not satisfy all > 0")
	}
}

// =============================================================================
// Integration Tests (Property of Properties)
// =============================================================================

func TestProperty_IntRangeAlwaysInBounds(t *testing.T) {
	QuickCheck(t, "IntRange always in bounds", func(g *Generator) bool {
		min := g.IntRange(-1000, 1000)
		max := g.IntRange(min, min+1000) // Ensure max >= min

		val := g.IntRange(min, max)
		return val >= min && val <= max
	})
}

func TestProperty_StringLengthAlwaysValid(t *testing.T) {
	QuickCheck(t, "String length always valid", func(g *Generator) bool {
		maxLen := g.IntRange(0, 100)
		s := g.String(maxLen)
		return len(s) <= maxLen
	})
}

func TestProperty_SliceLengthAlwaysValid(t *testing.T) {
	QuickCheck(t, "Slice length always valid", func(g *Generator) bool {
		minLen := g.IntRange(0, 50)
		maxLen := g.IntRange(minLen, minLen+50)

		slice := SliceN(g, minLen, maxLen, func(g *Generator) int { return g.Int() })
		return len(slice) >= minLen && len(slice) <= maxLen
	})
}

func TestProperty_IdentifierAlwaysValid(t *testing.T) {
	QuickCheck(t, "Identifier always valid", func(g *Generator) bool {
		maxLen := g.IntRange(1, 50)
		id := g.Identifier(maxLen)

		if len(id) == 0 || len(id) > maxLen {
			return false
		}

		// First char must be letter or underscore
		first := rune(id[0])
		if !unicode.IsLetter(first) && first != '_' {
			return false
		}

		// Rest must be alphanumeric or underscore
		for _, c := range id[1:] {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
				return false
			}
		}

		return true
	})
}
