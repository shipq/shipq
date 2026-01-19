//go:build property

package portapi

import (
	"math/rand"
	"reflect"
	"testing"
)

var methods = []string{"GET", "POST", "PUT", "DELETE"}
var paths = []string{"/", "/pets", "/pets/{id}", "/users", "/users/{id}/posts", "/files/{path...}"}
var pkgs = []string{"", "example.com/app", "example.com/app/pets", "internal/handlers"}
var names = []string{"", "Handler", "Get", "Create", "Delete", "List"}

func randomEndpoint(r *rand.Rand) Endpoint {
	return Endpoint{
		Method:      methods[r.Intn(len(methods))],
		Path:        paths[r.Intn(len(paths))],
		HandlerPkg:  pkgs[r.Intn(len(pkgs))],
		HandlerName: names[r.Intn(len(names))],
	}
}

func randomEndpoints(r *rand.Rand, n int) []Endpoint {
	eps := make([]Endpoint, n)
	for i := range eps {
		eps[i] = randomEndpoint(r)
	}
	return eps
}

func TestProperty_SortEndpoints_Deterministic(t *testing.T) {
	// Generate random valid endpoints
	// Sort twice with same input
	// Assert outputs are byte-identical
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		n := r.Intn(20) + 1
		endpoints := randomEndpoints(r, n)

		sorted1 := SortEndpoints(endpoints)
		sorted2 := SortEndpoints(endpoints)

		if !reflect.DeepEqual(sorted1, sorted2) {
			t.Errorf("seed=%d: SortEndpoints is not deterministic", seed)
		}
	}
}

func TestProperty_SortEndpoints_Idempotent(t *testing.T) {
	// Sort once, sort again
	// Assert sort(sort(x)) == sort(x)
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		n := r.Intn(20) + 1
		endpoints := randomEndpoints(r, n)

		sorted1 := SortEndpoints(endpoints)
		sorted2 := SortEndpoints(sorted1)

		if !reflect.DeepEqual(sorted1, sorted2) {
			t.Errorf("seed=%d: SortEndpoints is not idempotent", seed)
		}
	}
}

func TestProperty_SortEndpoints_Preserves_Elements(t *testing.T) {
	// Sorting should preserve all elements, just reorder them
	for seed := int64(0); seed < 100; seed++ {
		r := rand.New(rand.NewSource(seed))
		n := r.Intn(20) + 1
		endpoints := randomEndpoints(r, n)

		sorted := SortEndpoints(endpoints)

		if len(sorted) != len(endpoints) {
			t.Errorf("seed=%d: length changed after sort: %d -> %d", seed, len(endpoints), len(sorted))
			continue
		}

		// Count occurrences of each sort key
		origCounts := make(map[string]int)
		sortedCounts := make(map[string]int)
		for _, ep := range endpoints {
			origCounts[ep.SortKey()]++
		}
		for _, ep := range sorted {
			sortedCounts[ep.SortKey()]++
		}

		if !reflect.DeepEqual(origCounts, sortedCounts) {
			t.Errorf("seed=%d: elements changed after sort", seed)
		}
	}
}
