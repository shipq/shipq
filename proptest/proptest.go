// Package proptest provides property-based testing utilities with seeded
// random generation for reproducible tests.
//
// Property-based testing generates random inputs and verifies that certain
// invariants (properties) always hold. When a test fails, the seed is logged
// so the failure can be reproduced.
//
// Basic usage:
//
//	func TestMyProperty(t *testing.T) {
//	    proptest.QuickCheck(t, "my property", func(g *proptest.Generator) bool {
//	        n := g.IntRange(1, 100)
//	        return n >= 1 && n <= 100
//	    })
//	}
package proptest

import (
	"math/rand"
	"time"
)

// Generator wraps a seeded random number generator for reproducible
// random value generation. The seed is stored so it can be logged
// on test failure for reproducibility.
type Generator struct {
	rng  *rand.Rand
	seed int64
}

// New creates a new Generator with the given seed.
// If seed is 0, uses the current time as the seed.
func New(seed int64) *Generator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Generator{
		rng:  rand.New(rand.NewSource(seed)),
		seed: seed,
	}
}

// Seed returns the seed used by this generator.
// This is useful for logging on test failure so the failure can be reproduced.
func (g *Generator) Seed() int64 {
	return g.seed
}

// Intn returns a random int in [0, n).
// Panics if n <= 0.
func (g *Generator) Intn(n int) int {
	return g.rng.Intn(n)
}

// Int63n returns a random int64 in [0, n).
// Panics if n <= 0.
func (g *Generator) Int63n(n int64) int64 {
	return g.rng.Int63n(n)
}

// Float64 returns a random float64 in [0.0, 1.0).
func (g *Generator) Float64() float64 {
	return g.rng.Float64()
}

// Bool returns a random boolean with 50% probability for each value.
func (g *Generator) Bool() bool {
	return g.rng.Intn(2) == 1
}

// BoolWithProb returns true with the given probability (0.0 to 1.0).
func (g *Generator) BoolWithProb(prob float64) bool {
	return g.rng.Float64() < prob
}
