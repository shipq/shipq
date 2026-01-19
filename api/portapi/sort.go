package portapi

import (
	"cmp"
	"slices"
)

// SortKey returns a stable string key for sorting.
func (e Endpoint) SortKey() string {
	return e.Method + " " + e.Path + " " + e.HandlerPkg + "." + e.HandlerName
}

// SortEndpoints returns a new slice sorted deterministically.
// Sorts by method, then path, then handler identity.
// Does not mutate the input. Always returns a non-nil slice.
func SortEndpoints(endpoints []Endpoint) []Endpoint {
	sorted := make([]Endpoint, len(endpoints))
	copy(sorted, endpoints)
	slices.SortFunc(sorted, func(a, b Endpoint) int {
		return cmp.Compare(a.SortKey(), b.SortKey())
	})
	return sorted
}
