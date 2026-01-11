package proptest

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// Config controls property test behavior.
type Config struct {
	// NumTrials is the number of test iterations. Default: 100.
	NumTrials int

	// Seed is the random seed for reproducibility. Default: time-based.
	// Set to 0 for random seed.
	Seed int64

	// Verbose enables additional logging.
	Verbose bool
}

// DefaultConfig returns sensible defaults for property testing.
func DefaultConfig() Config {
	return Config{
		NumTrials: 100,
		Seed:      0, // Will be set from time or environment
	}
}

// getEffectiveSeed returns the seed to use, checking environment first.
func getEffectiveSeed(cfg Config) int64 {
	// Check environment variable for reproducibility
	if envSeed := os.Getenv("PROPTEST_SEED"); envSeed != "" {
		if seed, err := strconv.ParseInt(envSeed, 10, 64); err == nil {
			return seed
		}
	}

	if cfg.Seed != 0 {
		return cfg.Seed
	}

	return time.Now().UnixNano()
}

// Check runs a property multiple times with different random inputs.
// On failure, it logs the seed for reproducibility.
//
// Example:
//
//	proptest.Check(t, "positive ints", proptest.Config{NumTrials: 50}, func(g *proptest.Generator) bool {
//	    n := g.IntRange(1, 100)
//	    return n >= 1 && n <= 100
//	})
func Check(t *testing.T, name string, cfg Config, prop func(g *Generator) bool) {
	t.Helper()

	if cfg.NumTrials <= 0 {
		cfg.NumTrials = 100
	}

	seed := getEffectiveSeed(cfg)
	g := New(seed)

	if cfg.Verbose {
		t.Logf("proptest %q: running %d trials with seed %d", name, cfg.NumTrials, seed)
	}

	for i := 0; i < cfg.NumTrials; i++ {
		if !prop(g) {
			t.Errorf("proptest %q failed on trial %d (seed=%d, use PROPTEST_SEED=%d to reproduce)",
				name, i+1, seed, seed)
			return
		}
	}

	if cfg.Verbose {
		t.Logf("proptest %q: passed %d trials", name, cfg.NumTrials)
	}
}

// QuickCheck runs a property with default configuration (100 trials).
//
// Example:
//
//	proptest.QuickCheck(t, "strings are non-negative length", func(g *proptest.Generator) bool {
//	    s := g.String(100)
//	    return len(s) >= 0
//	})
func QuickCheck(t *testing.T, name string, prop func(g *Generator) bool) {
	t.Helper()
	Check(t, name, DefaultConfig(), prop)
}

// MustCheck is like Check but calls t.Fatal instead of t.Error on failure.
func MustCheck(t *testing.T, name string, cfg Config, prop func(g *Generator) bool) {
	t.Helper()

	if cfg.NumTrials <= 0 {
		cfg.NumTrials = 100
	}

	seed := getEffectiveSeed(cfg)
	g := New(seed)

	for i := 0; i < cfg.NumTrials; i++ {
		if !prop(g) {
			t.Fatalf("proptest %q failed on trial %d (seed=%d, use PROPTEST_SEED=%d to reproduce)",
				name, i+1, seed, seed)
			return
		}
	}
}

// ForAll runs a property that generates a value and returns both the value
// and whether the property holds. On failure, it logs the generated value.
//
// Example:
//
//	proptest.ForAll(t, "positive ints", 100, func(g *proptest.Generator) (int, bool) {
//	    n := g.IntRange(1, 100)
//	    return n, n >= 1 && n <= 100
//	})
func ForAll[T any](t *testing.T, name string, numTrials int, prop func(g *Generator) (T, bool)) {
	t.Helper()

	seed := getEffectiveSeed(Config{})
	g := New(seed)

	for i := 0; i < numTrials; i++ {
		val, ok := prop(g)
		if !ok {
			t.Errorf("proptest %q failed on trial %d with value %+v (seed=%d, use PROPTEST_SEED=%d to reproduce)",
				name, i+1, val, seed, seed)
			return
		}
	}
}

// ForAll2 runs a property that generates two values.
func ForAll2[A, B any](t *testing.T, name string, numTrials int, prop func(g *Generator) (A, B, bool)) {
	t.Helper()

	seed := getEffectiveSeed(Config{})
	g := New(seed)

	for i := 0; i < numTrials; i++ {
		a, b, ok := prop(g)
		if !ok {
			t.Errorf("proptest %q failed on trial %d with values (%+v, %+v) (seed=%d, use PROPTEST_SEED=%d to reproduce)",
				name, i+1, a, b, seed, seed)
			return
		}
	}
}

// ForAll3 runs a property that generates three values.
func ForAll3[A, B, C any](t *testing.T, name string, numTrials int, prop func(g *Generator) (A, B, C, bool)) {
	t.Helper()

	seed := getEffectiveSeed(Config{})
	g := New(seed)

	for i := 0; i < numTrials; i++ {
		a, b, c, ok := prop(g)
		if !ok {
			t.Errorf("proptest %q failed on trial %d with values (%+v, %+v, %+v) (seed=%d, use PROPTEST_SEED=%d to reproduce)",
				name, i+1, a, b, c, seed, seed)
			return
		}
	}
}

// CheckWithLabel runs a property and includes a label in failure messages.
// The label function is called with the generator to produce a description
// of the current test case.
func CheckWithLabel(t *testing.T, name string, cfg Config, prop func(g *Generator) (label string, ok bool)) {
	t.Helper()

	if cfg.NumTrials <= 0 {
		cfg.NumTrials = 100
	}

	seed := getEffectiveSeed(cfg)
	g := New(seed)

	for i := 0; i < cfg.NumTrials; i++ {
		label, ok := prop(g)
		if !ok {
			t.Errorf("proptest %q failed on trial %d: %s (seed=%d, use PROPTEST_SEED=%d to reproduce)",
				name, i+1, label, seed, seed)
			return
		}
	}
}

// RunSeeds runs a property with multiple specific seeds.
// Useful for regression testing with known problematic seeds.
func RunSeeds(t *testing.T, name string, seeds []int64, prop func(g *Generator) bool) {
	t.Helper()

	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			g := New(seed)
			if !prop(g) {
				t.Errorf("proptest %q failed with seed %d", name, seed)
			}
		})
	}
}

// Benchmark runs a property repeatedly for benchmarking.
func Benchmark(b *testing.B, prop func(g *Generator)) {
	seed := time.Now().UnixNano()
	g := New(seed)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prop(g)
	}
}

// =============================================================================
// Assertion Helpers
// =============================================================================

// Assert is a helper that returns false if the condition is false,
// allowing use in property functions.
func Assert(condition bool) bool {
	return condition
}

// AssertEqual checks if two values are equal.
func AssertEqual[T comparable](a, b T) bool {
	return a == b
}

// AssertNotEqual checks if two values are not equal.
func AssertNotEqual[T comparable](a, b T) bool {
	return a != b
}

// AssertInRange checks if a value is in the given range [min, max].
func AssertInRange(val, min, max int) bool {
	return val >= min && val <= max
}

// AssertInRangeFloat checks if a float is in the given range.
func AssertInRangeFloat(val, min, max float64) bool {
	return val >= min && val <= max
}

// AssertLenInRange checks if a slice length is in the given range.
func AssertLenInRange[T any](slice []T, min, max int) bool {
	return len(slice) >= min && len(slice) <= max
}

// AssertNonEmpty checks if a slice is non-empty.
func AssertNonEmpty[T any](slice []T) bool {
	return len(slice) > 0
}

// AssertAllSatisfy checks if all elements satisfy a predicate.
func AssertAllSatisfy[T any](slice []T, pred func(T) bool) bool {
	for _, v := range slice {
		if !pred(v) {
			return false
		}
	}
	return true
}

// AssertAnySatisfy checks if any element satisfies a predicate.
func AssertAnySatisfy[T any](slice []T, pred func(T) bool) bool {
	for _, v := range slice {
		if pred(v) {
			return true
		}
	}
	return false
}
