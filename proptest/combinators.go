package proptest

// =============================================================================
// Selection Combinators
// =============================================================================

// OneOf returns a random element from the provided values.
// Panics if values is empty.
func OneOf[T any](g *Generator, values ...T) T {
	if len(values) == 0 {
		panic("proptest: OneOf called with no values")
	}
	return values[g.Intn(len(values))]
}

// OneOfFunc calls a random generator function from the provided functions.
// Panics if fns is empty.
func OneOfFunc[T any](g *Generator, fns ...func(*Generator) T) T {
	if len(fns) == 0 {
		panic("proptest: OneOfFunc called with no functions")
	}
	return fns[g.Intn(len(fns))](g)
}

// Weighted returns a random element from values, with selection probability
// proportional to the given weights. Weights don't need to sum to 1.
// Panics if weights and values have different lengths or are empty.
func Weighted[T any](g *Generator, weights []float64, values []T) T {
	if len(weights) != len(values) {
		panic("proptest: Weighted weights and values must have same length")
	}
	if len(values) == 0 {
		panic("proptest: Weighted called with no values")
	}

	// Calculate total weight
	var total float64
	for _, w := range weights {
		total += w
	}

	// Pick a random point
	point := g.Float64() * total

	// Find which bucket it falls into
	var cumulative float64
	for i, w := range weights {
		cumulative += w
		if point < cumulative {
			return values[i]
		}
	}

	// Floating point edge case: return last element
	return values[len(values)-1]
}

// Pick returns a random element from a non-empty slice.
// Panics if slice is empty.
func Pick[T any](g *Generator, slice []T) T {
	if len(slice) == 0 {
		panic("proptest: Pick called with empty slice")
	}
	return slice[g.Intn(len(slice))]
}

// PickN returns n random elements from slice (with replacement).
// Panics if slice is empty.
func PickN[T any](g *Generator, slice []T, n int) []T {
	if len(slice) == 0 {
		panic("proptest: PickN called with empty slice")
	}
	result := make([]T, n)
	for i := 0; i < n; i++ {
		result[i] = slice[g.Intn(len(slice))]
	}
	return result
}

// Shuffle returns a shuffled copy of the slice.
func Shuffle[T any](g *Generator, slice []T) []T {
	result := make([]T, len(slice))
	copy(result, slice)
	g.rng.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})
	return result
}

// Sample returns n unique elements from slice (without replacement).
// Panics if n > len(slice) or slice is empty.
func Sample[T any](g *Generator, slice []T, n int) []T {
	if n > len(slice) {
		panic("proptest: Sample n > len(slice)")
	}
	if len(slice) == 0 {
		panic("proptest: Sample called with empty slice")
	}

	// Fisher-Yates shuffle, but only for first n elements
	indices := make([]int, len(slice))
	for i := range indices {
		indices[i] = i
	}

	result := make([]T, n)
	for i := 0; i < n; i++ {
		j := i + g.Intn(len(indices)-i)
		indices[i], indices[j] = indices[j], indices[i]
		result[i] = slice[indices[i]]
	}

	return result
}

// =============================================================================
// Collection Generators
// =============================================================================

// Slice generates a slice of length [0, maxLen] using the generator function.
func Slice[T any](g *Generator, maxLen int, gen func(*Generator) T) []T {
	if maxLen <= 0 {
		return nil
	}
	length := g.Intn(maxLen + 1)
	return SliceExact(g, length, gen)
}

// SliceN generates a slice of length [minLen, maxLen] using the generator function.
func SliceN[T any](g *Generator, minLen, maxLen int, gen func(*Generator) T) []T {
	if minLen > maxLen {
		panic("proptest: SliceN minLen > maxLen")
	}
	length := g.IntRange(minLen, maxLen)
	return SliceExact(g, length, gen)
}

// SliceExact generates a slice of exactly the given length.
func SliceExact[T any](g *Generator, length int, gen func(*Generator) T) []T {
	result := make([]T, length)
	for i := 0; i < length; i++ {
		result[i] = gen(g)
	}
	return result
}

// Map generates a map with [0, maxSize] entries using the key and value generators.
// Note: actual size may be less if duplicate keys are generated.
func Map[K comparable, V any](g *Generator, maxSize int, key func(*Generator) K, val func(*Generator) V) map[K]V {
	if maxSize <= 0 {
		return nil
	}
	size := g.Intn(maxSize + 1)
	result := make(map[K]V, size)
	for i := 0; i < size; i++ {
		result[key(g)] = val(g)
	}
	return result
}

// MapN generates a map with [minSize, maxSize] entries.
// Note: actual size may be less if duplicate keys are generated.
func MapN[K comparable, V any](g *Generator, minSize, maxSize int, key func(*Generator) K, val func(*Generator) V) map[K]V {
	if minSize > maxSize {
		panic("proptest: MapN minSize > maxSize")
	}
	size := g.IntRange(minSize, maxSize)
	result := make(map[K]V, size)
	for i := 0; i < size; i++ {
		result[key(g)] = val(g)
	}
	return result
}

// =============================================================================
// Optional/Nullable Generators
// =============================================================================

// Pointer returns nil with given probability, otherwise generates a value.
// nilChance should be in [0.0, 1.0].
func Pointer[T any](g *Generator, nilChance float64, gen func(*Generator) T) *T {
	if g.Float64() < nilChance {
		return nil
	}
	val := gen(g)
	return &val
}

// Optional returns (value, true) or (zero, false) with given probability.
func Optional[T any](g *Generator, nilChance float64, gen func(*Generator) T) (T, bool) {
	if g.Float64() < nilChance {
		var zero T
		return zero, false
	}
	return gen(g), true
}

// =============================================================================
// Transformation Combinators
// =============================================================================

// Transform applies a transformation function to a generated value.
func Transform[T, U any](g *Generator, gen func(*Generator) T, fn func(T) U) U {
	return fn(gen(g))
}

// Filter generates values until the predicate passes or maxRetries is exceeded.
// Returns (value, true) if a matching value was found, (zero, false) otherwise.
func Filter[T any](g *Generator, maxRetries int, gen func(*Generator) T, pred func(T) bool) (T, bool) {
	for i := 0; i < maxRetries; i++ {
		val := gen(g)
		if pred(val) {
			return val, true
		}
	}
	var zero T
	return zero, false
}

// FilterMust is like Filter but panics if no value passes the predicate.
func FilterMust[T any](g *Generator, maxRetries int, gen func(*Generator) T, pred func(T) bool) T {
	val, ok := Filter(g, maxRetries, gen, pred)
	if !ok {
		panic("proptest: FilterMust failed to find matching value")
	}
	return val
}

// =============================================================================
// Struct/Tuple Helpers
// =============================================================================

// Pair generates a pair of values.
func Pair[A, B any](g *Generator, genA func(*Generator) A, genB func(*Generator) B) (A, B) {
	return genA(g), genB(g)
}

// Triple generates three values.
func Triple[A, B, C any](g *Generator, genA func(*Generator) A, genB func(*Generator) B, genC func(*Generator) C) (A, B, C) {
	return genA(g), genB(g), genC(g)
}

// =============================================================================
// Range Helpers
// =============================================================================

// IntSlice generates a slice of random ints.
func (g *Generator) IntSlice(maxLen, minVal, maxVal int) []int {
	return Slice(g, maxLen, func(g *Generator) int {
		return g.IntRange(minVal, maxVal)
	})
}

// StringSlice generates a slice of random strings.
func (g *Generator) StringSlice(maxSliceLen, maxStrLen int) []string {
	return Slice(g, maxSliceLen, func(g *Generator) string {
		return g.String(maxStrLen)
	})
}

// UniqueStrings generates n unique strings.
func (g *Generator) UniqueStrings(n, maxLen int) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, n)

	maxAttempts := n * 10
	for i := 0; i < maxAttempts && len(result) < n; i++ {
		s := g.String(maxLen)
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// UniqueIdentifiers generates n unique identifiers.
func (g *Generator) UniqueIdentifiers(n, maxLen int) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, n)

	maxAttempts := n * 10
	for i := 0; i < maxAttempts && len(result) < n; i++ {
		s := g.IdentifierLower(maxLen)
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
