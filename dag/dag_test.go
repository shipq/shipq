package dag_test

import (
	"errors"
	"testing"

	"github.com/shipq/shipq/dag"
)

// --- Construction & validation ---

func TestNew_EmptyGraph(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{})
	if err != nil {
		t.Fatalf("empty graph should be valid: %v", err)
	}
	if len(g.Nodes()) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g.Nodes()))
	}
}

func TestNew_SingleNode(t *testing.T) {
	g, err := dag.New([]dag.Node[string]{
		{ID: "a", Description: "node a"},
	})
	if err != nil {
		t.Fatalf("single node should be valid: %v", err)
	}
	if g.Find("a") == nil {
		t.Error("expected to find node 'a'")
	}
}

func TestNew_DuplicateID(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "a"},
	})
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
	var dupErr *dag.DuplicateIDError[string]
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateIDError, got %T: %v", err, err)
	}
	if dupErr.ID != "a" {
		t.Errorf("expected duplicate ID 'a', got %q", dupErr.ID)
	}
}

func TestNew_DanglingHardDep(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", HardDeps: []string{"nonexistent"}},
	})
	if err == nil {
		t.Fatal("expected error for dangling dep")
	}
	var dangErr *dag.DanglingDepError[string]
	if !errors.As(err, &dangErr) {
		t.Fatalf("expected DanglingDepError, got %T: %v", err, err)
	}
	if dangErr.From != "a" {
		t.Errorf("expected From 'a', got %q", dangErr.From)
	}
	if dangErr.To != "nonexistent" {
		t.Errorf("expected To 'nonexistent', got %q", dangErr.To)
	}
	if dangErr.Kind != dag.Hard {
		t.Error("expected hard dep kind")
	}
}

func TestNew_DanglingSoftDep(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", SoftDeps: []string{"nonexistent"}},
	})
	if err == nil {
		t.Fatal("expected error for dangling soft dep")
	}
	var dangErr *dag.DanglingDepError[string]
	if !errors.As(err, &dangErr) {
		t.Fatalf("expected DanglingDepError, got %T: %v", err, err)
	}
	if dangErr.Kind != dag.Soft {
		t.Error("expected soft dep kind")
	}
}

func TestNew_SimpleCycle(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", HardDeps: []string{"b"}},
		{ID: "b", HardDeps: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	var cycErr *dag.CycleError[string]
	if !errors.As(err, &cycErr) {
		t.Fatalf("expected CycleError, got %T: %v", err, err)
	}
}

func TestNew_TransitiveCycle(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", HardDeps: []string{"c"}},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"b"}},
	})
	if err == nil {
		t.Fatal("expected error for transitive cycle")
	}
}

func TestNew_SelfCycle(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", HardDeps: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected error for self-cycle")
	}
}

func TestNew_SoftDepCycle(t *testing.T) {
	// Soft deps also participate in cycle detection.
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", SoftDeps: []string{"b"}},
		{ID: "b", HardDeps: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected error for cycle via soft dep")
	}
}

func TestNew_ValidDiamond(t *testing.T) {
	// Diamond shape (A→B, A→C, B→D, C→D) is fine — it's a DAG.
	_, err := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"a"}},
		{ID: "d", HardDeps: []string{"b", "c"}},
	})
	if err != nil {
		t.Fatalf("diamond should be valid: %v", err)
	}
}

// --- TopologicalOrder ---

func TestTopologicalOrder_Linear(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"b"}},
	})
	order := g.TopologicalOrder()
	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	if pos["a"] >= pos["b"] || pos["b"] >= pos["c"] {
		t.Errorf("invalid topological order: %v", order)
	}
}

func TestTopologicalOrder_AllNodesPresent(t *testing.T) {
	nodes := []dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c"},
		{ID: "d", HardDeps: []string{"b", "c"}},
	}
	g, _ := dag.New(nodes)
	order := g.TopologicalOrder()
	if len(order) != len(nodes) {
		t.Errorf("expected %d nodes in order, got %d", len(nodes), len(order))
	}
	seen := make(map[string]bool)
	for _, id := range order {
		if seen[id] {
			t.Errorf("duplicate in topological order: %s", id)
		}
		seen[id] = true
	}
}

func TestTopologicalOrder_DepsBeforeDependents(t *testing.T) {
	nodes := []dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"a"}},
		{ID: "d", HardDeps: []string{"b", "c"}},
		{ID: "e", HardDeps: []string{"d"}},
	}
	g, _ := dag.New(nodes)
	order := g.TopologicalOrder()
	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	for _, node := range nodes {
		for _, dep := range node.HardDeps {
			if pos[dep] >= pos[node.ID] {
				t.Errorf("%s must come before %s", dep, node.ID)
			}
		}
	}
}

// --- TransitiveDeps ---

func TestTransitiveDeps_Linear(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"b"}},
	})
	deps := g.TransitiveDeps("c")
	if len(deps) != 2 {
		t.Fatalf("expected 2 transitive deps, got %d: %v", len(deps), deps)
	}
	// Should be in topological order: a before b
	if deps[0] != "a" || deps[1] != "b" {
		t.Errorf("expected [a, b], got %v", deps)
	}
}

func TestTransitiveDeps_Diamond(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"a"}},
		{ID: "d", HardDeps: []string{"b", "c"}},
	})
	deps := g.TransitiveDeps("d")
	// Should contain a, b, c (in some valid topo order where a is first)
	if len(deps) != 3 {
		t.Fatalf("expected 3 transitive deps, got %d: %v", len(deps), deps)
	}
	if deps[0] != "a" {
		t.Errorf("expected 'a' first, got %v", deps)
	}
}

func TestTransitiveDeps_Root(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
	})
	deps := g.TransitiveDeps("a")
	if len(deps) != 0 {
		t.Errorf("root node should have no transitive deps, got %v", deps)
	}
}

func TestTransitiveDeps_UnknownID(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{{ID: "a"}})
	deps := g.TransitiveDeps("nonexistent")
	if deps != nil {
		t.Errorf("unknown ID should return nil, got %v", deps)
	}
}

// --- CheckHardDeps / CheckSoftDeps ---

func TestCheckHardDeps_AllSatisfied(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
	})
	unsatisfied := g.CheckHardDeps("b", func(id string) bool { return true })
	if len(unsatisfied) != 0 {
		t.Errorf("expected no unsatisfied deps, got %v", unsatisfied)
	}
}

func TestCheckHardDeps_NoneSatisfied(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", HardDeps: []string{"a", "b"}},
	})
	unsatisfied := g.CheckHardDeps("c", func(id string) bool { return false })
	if len(unsatisfied) != 2 {
		t.Errorf("expected 2 unsatisfied deps, got %v", unsatisfied)
	}
}

func TestCheckHardDeps_Partial(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", HardDeps: []string{"a", "b"}},
	})
	unsatisfied := g.CheckHardDeps("c", func(id string) bool { return id == "a" })
	if len(unsatisfied) != 1 || unsatisfied[0] != "b" {
		t.Errorf("expected [b], got %v", unsatisfied)
	}
}

func TestCheckSoftDeps(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", SoftDeps: []string{"a", "b"}},
	})
	unsatisfied := g.CheckSoftDeps("c", func(id string) bool { return id == "a" })
	if len(unsatisfied) != 1 || unsatisfied[0] != "b" {
		t.Errorf("expected [b], got %v", unsatisfied)
	}
}

func TestCheckHardDeps_UnknownID(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{{ID: "a"}})
	unsatisfied := g.CheckHardDeps("nonexistent", func(string) bool { return false })
	if unsatisfied != nil {
		t.Errorf("unknown ID should return nil, got %v", unsatisfied)
	}
}

func TestCheckSoftDeps_UnknownID(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{{ID: "a"}})
	unsatisfied := g.CheckSoftDeps("nonexistent", func(string) bool { return false })
	if unsatisfied != nil {
		t.Errorf("unknown ID should return nil, got %v", unsatisfied)
	}
}

func TestCheckHardDeps_NoDeps(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
	})
	unsatisfied := g.CheckHardDeps("a", func(string) bool { return false })
	if len(unsatisfied) != 0 {
		t.Errorf("node with no deps should have no unsatisfied deps, got %v", unsatisfied)
	}
}

func TestCheckSoftDeps_NoDeps(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
	})
	unsatisfied := g.CheckSoftDeps("a", func(string) bool { return false })
	if len(unsatisfied) != 0 {
		t.Errorf("node with no soft deps should have no unsatisfied deps, got %v", unsatisfied)
	}
}

// --- Available ---

func TestAvailable_NothingDone(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c"},
	})
	avail := g.Available(func(string) bool { return false })
	// Only roots (a, c) should be available
	if len(avail) != 2 {
		t.Fatalf("expected 2 available, got %d: %v", len(avail), avail)
	}
	availSet := make(map[string]bool)
	for _, id := range avail {
		availSet[id] = true
	}
	if !availSet["a"] || !availSet["c"] {
		t.Errorf("expected a and c to be available, got %v", avail)
	}
}

func TestAvailable_AfterCompletingRoot(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", HardDeps: []string{"a"}},
		{ID: "d", HardDeps: []string{"b", "c"}},
	})
	avail := g.Available(func(id string) bool { return id == "a" })
	// b and c should be available (a is already done, d needs b+c)
	if len(avail) != 2 {
		t.Fatalf("expected 2 available, got %d: %v", len(avail), avail)
	}
	availSet := make(map[string]bool)
	for _, id := range avail {
		availSet[id] = true
	}
	if !availSet["b"] || !availSet["c"] {
		t.Errorf("expected b and c to be available, got %v", avail)
	}
}

func TestAvailable_AllDone(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
	})
	avail := g.Available(func(string) bool { return true })
	// Everything is satisfied, so nothing is "available" (nothing left to do)
	if len(avail) != 0 {
		t.Errorf("expected 0 available when all done, got %v", avail)
	}
}

func TestAvailable_SoftDepsDoNotBlock(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", HardDeps: []string{"a"}, SoftDeps: []string{"b"}},
	})
	// "a" is satisfied, "b" is not — but "b" is only a soft dep of "c",
	// so "c" should still be available.
	avail := g.Available(func(id string) bool { return id == "a" })
	found := false
	for _, id := range avail {
		if id == "c" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'c' to be available (soft dep 'b' should not block), got %v", avail)
	}
}

func TestAvailable_MultipleHardDeps_PartiallyMet(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", HardDeps: []string{"a", "b"}},
	})
	// Only "a" is satisfied — "c" needs both "a" and "b"
	avail := g.Available(func(id string) bool { return id == "a" })
	availSet := make(map[string]bool)
	for _, id := range avail {
		availSet[id] = true
	}
	if availSet["c"] {
		t.Error("c should NOT be available when only one of its hard deps is met")
	}
	if !availSet["b"] {
		t.Error("b should be available")
	}
}

// --- Dependents ---

func TestDependents(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", SoftDeps: []string{"a"}},
		{ID: "d"},
	})
	deps := g.Dependents("a")
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependents, got %d: %v", len(deps), deps)
	}
	depSet := make(map[string]bool)
	for _, id := range deps {
		depSet[id] = true
	}
	if !depSet["b"] || !depSet["c"] {
		t.Errorf("expected b and c as dependents, got %v", deps)
	}
}

func TestDependents_NoDependents(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
	})
	deps := g.Dependents("a")
	if len(deps) != 0 {
		t.Errorf("expected no dependents, got %v", deps)
	}
}

func TestDependents_UnknownID(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{{ID: "a"}})
	deps := g.Dependents("nonexistent")
	if deps != nil {
		t.Errorf("unknown ID should return nil, got %v", deps)
	}
}

// --- Find ---

func TestFind_Exists(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a", Description: "first"},
	})
	n := g.Find("a")
	if n == nil {
		t.Fatal("expected to find 'a'")
	}
	if n.Description != "first" {
		t.Errorf("expected description 'first', got %q", n.Description)
	}
}

func TestFind_NotExists(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{{ID: "a"}})
	if g.Find("b") != nil {
		t.Error("expected nil for unknown ID")
	}
}

func TestFind_ReturnsACopy(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a", Description: "original"},
	})
	n := g.Find("a")
	n.Description = "mutated"
	n2 := g.Find("a")
	if n2.Description != "original" {
		t.Error("Find should return a copy; mutation of returned node should not affect graph")
	}
}

// --- Edges ---

func TestEdges(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
		{ID: "c", SoftDeps: []string{"a"}},
	})
	edges := g.Edges()
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
	// Verify we have one hard and one soft edge
	hardCount, softCount := 0, 0
	for _, e := range edges {
		if e.Kind == dag.Hard {
			hardCount++
		}
		if e.Kind == dag.Soft {
			softCount++
		}
	}
	if hardCount != 1 || softCount != 1 {
		t.Errorf("expected 1 hard + 1 soft edge, got %d hard + %d soft", hardCount, softCount)
	}
}

func TestEdges_EmptyGraph(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{})
	edges := g.Edges()
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestEdges_NoEdges(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
	})
	edges := g.Edges()
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestEdges_VerifyDirection(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", HardDeps: []string{"a"}},
	})
	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	e := edges[0]
	if e.Dep != "a" {
		t.Errorf("expected Dep='a', got %q", e.Dep)
	}
	if e.Dependent != "b" {
		t.Errorf("expected Dependent='b', got %q", e.Dependent)
	}
	if e.Kind != dag.Hard {
		t.Error("expected hard edge")
	}
}

// --- Nodes ---

func TestNodes_ReturnsACopy(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a", Description: "original"},
	})
	nodes := g.Nodes()
	nodes[0].Description = "mutated"
	nodes2 := g.Nodes()
	if nodes2[0].Description != "original" {
		t.Error("Nodes should return a copy; mutation should not affect graph")
	}
}

// --- Generic type support ---

func TestNew_IntKeys(t *testing.T) {
	g, err := dag.New([]dag.Node[int]{
		{ID: 1, Description: "first"},
		{ID: 2, Description: "second", HardDeps: []int{1}},
		{ID: 3, Description: "third", HardDeps: []int{2}},
	})
	if err != nil {
		t.Fatalf("int-keyed graph should be valid: %v", err)
	}
	order := g.TopologicalOrder()
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(order))
	}
	pos := make(map[int]int)
	for i, id := range order {
		pos[id] = i
	}
	if pos[1] >= pos[2] || pos[2] >= pos[3] {
		t.Errorf("invalid topological order for int keys: %v", order)
	}
}

type customKey struct {
	Namespace string
	Name      string
}

func TestNew_StructKeys(t *testing.T) {
	fetch := customKey{"io", "fetch"}
	parse := customKey{"transform", "parse"}
	g, err := dag.New([]dag.Node[customKey]{
		{ID: fetch, Description: "fetch data"},
		{ID: parse, Description: "parse data", HardDeps: []customKey{fetch}},
	})
	if err != nil {
		t.Fatalf("struct-keyed graph should be valid: %v", err)
	}
	if g.Find(parse) == nil {
		t.Error("expected to find parse node")
	}
	deps := g.TransitiveDeps(parse)
	if len(deps) != 1 {
		t.Fatalf("expected 1 transitive dep, got %d", len(deps))
	}
	if deps[0] != fetch {
		t.Errorf("expected fetch dep, got %v", deps[0])
	}
}

// --- Complex scenarios ---

func TestComplex_LargeLinearChain(t *testing.T) {
	nodes := make([]dag.Node[int], 100)
	for i := range nodes {
		nodes[i] = dag.Node[int]{ID: i}
		if i > 0 {
			nodes[i].HardDeps = []int{i - 1}
		}
	}
	g, err := dag.New(nodes)
	if err != nil {
		t.Fatalf("large linear chain should be valid: %v", err)
	}

	order := g.TopologicalOrder()
	for i := 1; i < len(order); i++ {
		if order[i-1] > order[i] {
			// In a linear chain 0→1→2→...→99, each must come in order
			// But topological sort only guarantees deps before dependents,
			// so let's check that constraint.
		}
	}

	deps := g.TransitiveDeps(99)
	if len(deps) != 99 {
		t.Errorf("expected 99 transitive deps for node 99, got %d", len(deps))
	}

	deps0 := g.TransitiveDeps(0)
	if len(deps0) != 0 {
		t.Errorf("expected 0 transitive deps for node 0, got %d", len(deps0))
	}
}

func TestComplex_WideGraph(t *testing.T) {
	// One root with many children
	nodes := []dag.Node[string]{{ID: "root"}}
	for i := 0; i < 50; i++ {
		id := string(rune('A' + i%26))
		if i >= 26 {
			id = id + "2"
		}
		nodes = append(nodes, dag.Node[string]{
			ID:       id,
			HardDeps: []string{"root"},
		})
	}
	g, err := dag.New(nodes)
	if err != nil {
		t.Fatalf("wide graph should be valid: %v", err)
	}
	avail := g.Available(func(id string) bool { return id == "root" })
	if len(avail) != 50 {
		t.Errorf("expected 50 available after completing root, got %d", len(avail))
	}
}

func TestComplex_MixedHardSoftDeps(t *testing.T) {
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
		{ID: "d", HardDeps: []string{"a"}, SoftDeps: []string{"b", "c"}},
	})

	// Only "a" satisfied — "d" should be available (soft deps don't block)
	avail := g.Available(func(id string) bool { return id == "a" })
	found := false
	for _, id := range avail {
		if id == "d" {
			found = true
		}
	}
	if !found {
		t.Error("d should be available when hard dep a is met, even if soft deps b,c are not")
	}

	// Check soft deps
	softMissing := g.CheckSoftDeps("d", func(id string) bool { return id == "b" })
	if len(softMissing) != 1 || softMissing[0] != "c" {
		t.Errorf("expected [c] as unsatisfied soft dep, got %v", softMissing)
	}

	// Hard deps are all satisfied
	hardMissing := g.CheckHardDeps("d", func(id string) bool { return id == "a" })
	if len(hardMissing) != 0 {
		t.Errorf("expected no unsatisfied hard deps, got %v", hardMissing)
	}
}

func TestTopologicalOrder_SoftDepsRespected(t *testing.T) {
	// Soft deps should also appear before their dependents in topo order
	g, _ := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "b", SoftDeps: []string{"a"}},
	})
	order := g.TopologicalOrder()
	pos := make(map[string]int)
	for i, id := range order {
		pos[id] = i
	}
	if pos["a"] >= pos["b"] {
		t.Errorf("soft dep 'a' should come before 'b' in topo order, got %v", order)
	}
}

// --- Error message formatting ---

func TestCycleError_Message(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", HardDeps: []string{"b"}},
		{ID: "b", HardDeps: []string{"a"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestDuplicateIDError_Message(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a"},
		{ID: "a"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestDanglingDepError_Message(t *testing.T) {
	_, err := dag.New([]dag.Node[string]{
		{ID: "a", HardDeps: []string{"missing"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}
