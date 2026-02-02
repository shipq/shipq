//go:build property

package cli

import (
	"go/parser"
	"go/token"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// randomManifest generates a random valid manifest for property testing.
func randomManifest(rng *rand.Rand, maxEndpoints int) Manifest {
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	shapes := []string{"ctx_err", "ctx_resp_err", "ctx_req_err", "ctx_req_resp_err"}
	paths := []string{
		"/users",
		"/pets",
		"/items",
		"/orders",
		"/products",
		"/users/{id}",
		"/pets/{id}",
		"/items/{id}",
		"/health",
		"/status",
	}
	handlers := []string{
		"List",
		"Get",
		"Create",
		"Update",
		"Delete",
		"Health",
		"Status",
	}
	pkgs := []string{
		"example.com/app/users",
		"example.com/app/pets",
		"example.com/app/items",
		"example.com/app/orders",
		"example.com/app/health",
	}
	reqTypes := []string{
		"CreateRequest",
		"UpdateRequest",
		"GetRequest",
		"DeleteRequest",
		"ListRequest",
	}
	respTypes := []string{
		"User",
		"[]User",
		"Pet",
		"[]Pet",
		"Item",
		"[]Item",
	}

	numEndpoints := rng.Intn(maxEndpoints) + 1
	endpoints := make([]ManifestEndpoint, numEndpoints)

	for i := 0; i < numEndpoints; i++ {
		shape := shapes[rng.Intn(len(shapes))]
		ep := ManifestEndpoint{
			Method:      methods[rng.Intn(len(methods))],
			Path:        paths[rng.Intn(len(paths))],
			HandlerPkg:  pkgs[rng.Intn(len(pkgs))],
			HandlerName: handlers[rng.Intn(len(handlers))],
			Shape:       shape,
		}

		// Set types based on shape
		switch shape {
		case "ctx_req_resp_err":
			ep.ReqType = reqTypes[rng.Intn(len(reqTypes))]
			ep.RespType = respTypes[rng.Intn(len(respTypes))]
		case "ctx_req_err":
			ep.ReqType = reqTypes[rng.Intn(len(reqTypes))]
		case "ctx_resp_err":
			ep.RespType = respTypes[rng.Intn(len(respTypes))]
		}

		endpoints[i] = ep
	}

	return Manifest{Endpoints: endpoints}
}

func TestProperty_Generate_Deterministic(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("Using seed: %d", seed)

	const iterations = 100

	for i := 0; i < iterations; i++ {
		// Create two rngs with the same seed to generate identical manifests
		rng := rand.New(rand.NewSource(seed + int64(i)))
		m := randomManifest(rng, 10)

		// Generate code twice
		code1, err1 := Generate(m, "testpkg", "")
		code2, err2 := Generate(m, "testpkg", "")

		// Both should succeed or both should fail
		if err1 != nil || err2 != nil {
			// If one failed, both should fail with same error type
			if err1 == nil || err2 == nil {
				t.Errorf("iteration %d: inconsistent errors: err1=%v, err2=%v", i, err1, err2)
			}
			continue
		}

		// Assert byte-identical
		if code1 != code2 {
			t.Errorf("iteration %d: generated code is not deterministic\ncode1:\n%s\ncode2:\n%s", i, code1, code2)
		}
	}
}

func TestProperty_Generate_AlwaysParses(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("Using seed: %d", seed)

	const iterations = 100

	for i := 0; i < iterations; i++ {
		rng := rand.New(rand.NewSource(seed + int64(i)))
		m := randomManifest(rng, 10)

		code, err := Generate(m, "testpkg", "")
		if err != nil {
			// Generation failure is acceptable for some invalid manifests
			continue
		}

		// Assert go/parser.ParseFile succeeds
		fset := token.NewFileSet()
		_, parseErr := parser.ParseFile(fset, "", code, 0)
		if parseErr != nil {
			t.Errorf("iteration %d: generated code does not parse: %v\ncode:\n%s", i, parseErr, code)
		}
	}
}

func TestProperty_Generate_DifferentOrderSameOutput(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("Using seed: %d", seed)

	const iterations = 50

	for i := 0; i < iterations; i++ {
		rng := rand.New(rand.NewSource(seed + int64(i)))
		m := randomManifest(rng, 10)

		// Shuffle the endpoints
		shuffled := make([]ManifestEndpoint, len(m.Endpoints))
		copy(shuffled, m.Endpoints)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		m2 := Manifest{Endpoints: shuffled}

		// Generate code for both orderings
		code1, err1 := Generate(m, "testpkg", "")
		code2, err2 := Generate(m2, "testpkg")

		if err1 != nil || err2 != nil {
			continue
		}

		// Should produce identical output regardless of input order
		if code1 != code2 {
			t.Errorf("iteration %d: different ordering should produce same output", i)
		}
	}
}

func TestProperty_Generate_ValidPackageNames(t *testing.T) {
	pkgNames := []string{
		"api",
		"myapi",
		"handlers",
		"pkg",
		"internal",
		"v1",
		"api_v2",
	}

	for _, pkgName := range pkgNames {
		m := Manifest{Endpoints: []ManifestEndpoint{
			{
				Method:      "GET",
				Path:        "/health",
				HandlerPkg:  "example.com/app/handlers",
				HandlerName: "Health",
				Shape:       "ctx_err",
			},
		}}

		code, err := Generate(m, pkgName)
		if err != nil {
			t.Errorf("should generate for package %q: %v", pkgName, err)
			continue
		}

		// Parse to verify
		fset := token.NewFileSet()
		_, parseErr := parser.ParseFile(fset, "", code, 0)
		if parseErr != nil {
			t.Errorf("generated code should parse for package %q: %v", pkgName, parseErr)
			continue
		}

		// Verify package declaration
		if !strings.Contains(code, "package "+pkgName) {
			t.Errorf("expected code to contain 'package %s'", pkgName)
		}
	}
}
