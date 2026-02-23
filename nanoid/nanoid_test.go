package nanoid

import (
	"strings"
	"sync"
	"testing"
)

func TestNanoid(t *testing.T) {
	nanoid := New()
	if len(nanoid) != 21 {
		t.Errorf("Nanoid length is not 21: %s", nanoid)
	}
}

func TestNanoidRandomness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		nanoid := New()
		if seen[nanoid] {
			t.Errorf("Nanoid is not random: %s", nanoid)
		}
		seen[nanoid] = true
	}
}

// Test collision resistance with large sample
func TestCollisionResistance(t *testing.T) {
	seen := make(map[string]bool)
	numTests := 100000 // Large sample

	for i := 0; i < numTests; i++ {
		nanoid := New()
		if seen[nanoid] {
			t.Errorf("Collision detected after %d generations: %s", i+1, nanoid)
			return
		}
		seen[nanoid] = true
	}

	t.Logf("Generated %d unique nanoids without collision", numTests)
}

// Test that all characters are URL-safe
func TestURLSafety(t *testing.T) {
	urlUnsafeChars := "+/="

	for i := 0; i < 1000; i++ {
		nanoid := New()
		if strings.ContainsAny(nanoid, urlUnsafeChars) {
			t.Errorf("Nanoid contains URL-unsafe characters: %s", nanoid)
		}
	}
}

// Test that each position uses the full 64-character alphabet
// This catches bugs where bit extraction only produces 4 bits (0-15) instead of 6 bits (0-63)
func TestFullAlphabetAtEachPosition(t *testing.T) {
	const iterations = 100000
	const expectedMinChars = 50 // With 64 chars and 100k samples, we should see at least 50 unique chars per position

	// Track unique characters seen at each position
	charsByPosition := make([]map[byte]bool, 21)
	for i := range charsByPosition {
		charsByPosition[i] = make(map[byte]bool)
	}

	// Generate many nanoids
	for i := 0; i < iterations; i++ {
		nanoid := New()
		for pos := 0; pos < 21; pos++ {
			charsByPosition[pos][nanoid[pos]] = true
		}
	}

	// Check each position has sufficient character diversity
	for pos, chars := range charsByPosition {
		if len(chars) < expectedMinChars {
			t.Errorf("Position %d only has %d unique characters (expected at least %d). This suggests broken bit extraction.",
				pos, len(chars), expectedMinChars)
		}
	}
}

// Test that positions aren't correlated (no bit reuse)
// If bits are reused, certain character combinations will appear more often than expected
func TestPositionIndependence(t *testing.T) {
	const iterations = 50000

	// Track co-occurrence of specific character pairs between adjacent positions
	// If bits are reused, we'll see non-uniform distribution
	pairCounts := make(map[string]int)

	for i := 0; i < iterations; i++ {
		nanoid := New()
		// Check positions 1 and 2 (which should use independent bits from bytes 0-1 and 1-2)
		pair := string([]byte{nanoid[1], nanoid[2]})
		pairCounts[pair]++
	}

	// With 64*64 = 4096 possible pairs and 50k iterations, expected ~12 per pair
	// Check that no pair appears way too often (which would indicate correlation)
	maxExpected := iterations / 100 // No pair should appear more than 1% of the time
	for pair, count := range pairCounts {
		if count > maxExpected {
			t.Errorf("Pair %q appeared %d times (max expected %d). This suggests bit reuse between positions.",
				pair, count, maxExpected)
		}
	}
}

// Benchmark nanoid generator
func BenchmarkNanoid(b *testing.B) {
	for n := 0; n < b.N; n++ {
		_ = New()
	}
}

// Benchmark parallel nanoid generation
func BenchmarkNanoidParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = New()
		}
	})
}

// Test for race conditions under heavy concurrent load
func TestConcurrentSafety(t *testing.T) {
	const goroutines = 100
	const idsPerGoroutine = 10000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id := New()
				// Verify the ID is valid (all chars from alphabet)
				for _, c := range []byte(id) {
					valid := (c >= '0' && c <= '9') ||
						(c >= 'a' && c <= 'z') ||
						(c >= 'A' && c <= 'Z') ||
						c == '-' || c == '_'
					if !valid {
						t.Errorf("Invalid character %q in nanoid %q", c, id)
						return
					}
				}
			}
		}()
	}

	wg.Wait()
}
