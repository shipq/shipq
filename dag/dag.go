package dag

import "fmt"

// DepKind distinguishes hard dependencies (must be met) from soft
// dependencies (recommended but not fatal if missing).
type DepKind int

const (
	Hard DepKind = iota // Must be satisfied before the dependent can run.
	Soft                // Recommended; produces a warning if unsatisfied.
)

// Edge represents a dependency edge from one node to another, along with
// its kind.
type Edge[K comparable] struct {
	Dep       K // The prerequisite node.
	Dependent K // The node that requires the prerequisite.
}

// Node represents a single node in the dependency graph.
type Node[K comparable] struct {
	// ID uniquely identifies this node within the graph.
	ID K

	// Description is a human-readable summary of what this node represents.
	Description string

	// HardDeps are nodes that MUST be completed before this one.
	HardDeps []K

	// SoftDeps are nodes that SHOULD be completed but are not fatal.
	SoftDeps []K
}

// Graph is a validated, immutable dependency graph over nodes of key type K.
//
// Construct a Graph by calling [New], which validates the structure (no
// cycles, no dangling references, no duplicate IDs) at construction time.
// After construction, all query methods are safe to call without error
// checks for structural problems.
type Graph[K comparable] struct {
	nodes []Node[K]
	index map[K]int // ID → position in nodes slice
}

// CycleError reports a cycle detected in the graph.
type CycleError[K comparable] struct {
	Cycle []K // The nodes involved in the cycle, in order.
}

func (e *CycleError[K]) Error() string {
	return fmt.Sprintf("dag: cycle detected: %v", e.Cycle)
}

// DanglingDepError reports a dependency that references a node not in the graph.
type DanglingDepError[K comparable] struct {
	From K
	To   K
	Kind DepKind
}

func (e *DanglingDepError[K]) Error() string {
	kind := "hard"
	if e.Kind == Soft {
		kind = "soft"
	}
	return fmt.Sprintf("dag: node %v has %s dep on unknown node %v", e.From, kind, e.To)
}

// DuplicateIDError reports a duplicate node ID.
type DuplicateIDError[K comparable] struct {
	ID K
}

func (e *DuplicateIDError[K]) Error() string {
	return fmt.Sprintf("dag: duplicate node ID: %v", e.ID)
}

// New constructs a Graph from the given nodes. It validates the graph
// structure at construction time and returns an error if:
//   - Any node ID appears more than once (DuplicateIDError).
//   - Any dependency references a node ID not present in the graph
//     (DanglingDepError).
//   - The graph contains a cycle (CycleError).
//
// If New returns successfully, all query methods on the returned Graph are
// guaranteed to be structurally safe.
func New[K comparable](nodes []Node[K]) (*Graph[K], error) {
	g := &Graph[K]{
		nodes: make([]Node[K], len(nodes)),
		index: make(map[K]int, len(nodes)),
	}
	copy(g.nodes, nodes)

	// Check for duplicate IDs.
	for i, n := range g.nodes {
		if _, exists := g.index[n.ID]; exists {
			return nil, &DuplicateIDError[K]{ID: n.ID}
		}
		g.index[n.ID] = i
	}

	// Check for dangling dependencies.
	for _, n := range g.nodes {
		for _, dep := range n.HardDeps {
			if _, exists := g.index[dep]; !exists {
				return nil, &DanglingDepError[K]{From: n.ID, To: dep, Kind: Hard}
			}
		}
		for _, dep := range n.SoftDeps {
			if _, exists := g.index[dep]; !exists {
				return nil, &DanglingDepError[K]{From: n.ID, To: dep, Kind: Soft}
			}
		}
	}

	// Detect cycles using DFS with coloring.
	// white=0 (unvisited), gray=1 (in progress), black=2 (done)
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[K]int, len(g.nodes))
	parent := make(map[K]K, len(g.nodes))

	var dfs func(id K) *CycleError[K]
	dfs = func(id K) *CycleError[K] {
		color[id] = gray
		node := &g.nodes[g.index[id]]

		// Follow both hard and soft deps for cycle detection.
		allDeps := make([]K, 0, len(node.HardDeps)+len(node.SoftDeps))
		allDeps = append(allDeps, node.HardDeps...)
		allDeps = append(allDeps, node.SoftDeps...)

		for _, dep := range allDeps {
			switch color[dep] {
			case gray:
				// Found a cycle — reconstruct it.
				cycle := []K{dep, id}
				cur := id
				for cur != dep {
					p, ok := parent[cur]
					if !ok {
						break
					}
					cycle = append(cycle, p)
					cur = p
				}
				// Reverse to get the cycle in order.
				for i, j := 0, len(cycle)-1; i < j; i, j = i+1, j-1 {
					cycle[i], cycle[j] = cycle[j], cycle[i]
				}
				return &CycleError[K]{Cycle: cycle}
			case white:
				parent[dep] = id
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for _, n := range g.nodes {
		if color[n.ID] == white {
			if err := dfs(n.ID); err != nil {
				return nil, err
			}
		}
	}

	return g, nil
}

// Find returns the Node with the given ID, or nil if not found.
func (g *Graph[K]) Find(id K) *Node[K] {
	idx, ok := g.index[id]
	if !ok {
		return nil
	}
	n := g.nodes[idx] // copy
	return &n
}

// Nodes returns a copy of all nodes in the graph.
func (g *Graph[K]) Nodes() []Node[K] {
	result := make([]Node[K], len(g.nodes))
	copy(result, g.nodes)
	return result
}

// TopologicalOrder returns all node IDs in a valid topological order.
// Every node appears after all of its hard dependencies.
// Because the graph was validated at construction, this never fails.
func (g *Graph[K]) TopologicalOrder() []K {
	visited := make(map[K]bool, len(g.nodes))
	var result []K

	var visit func(id K)
	visit = func(id K) {
		if visited[id] {
			return
		}
		visited[id] = true
		node := &g.nodes[g.index[id]]

		// Visit hard deps first, then soft deps.
		for _, dep := range node.HardDeps {
			visit(dep)
		}
		for _, dep := range node.SoftDeps {
			visit(dep)
		}
		result = append(result, id)
	}

	for _, n := range g.nodes {
		visit(n.ID)
	}
	return result
}

// TransitiveDeps returns all nodes that must be completed before the given
// node, in topological order. Only hard dependencies are followed.
// Returns nil if the ID is not found.
func (g *Graph[K]) TransitiveDeps(id K) []K {
	if _, ok := g.index[id]; !ok {
		return nil
	}

	visited := make(map[K]bool)
	var collect func(K)
	collect = func(k K) {
		node := &g.nodes[g.index[k]]
		for _, dep := range node.HardDeps {
			if !visited[dep] {
				visited[dep] = true
				collect(dep)
			}
		}
	}
	collect(id)

	if len(visited) == 0 {
		return []K{}
	}

	// Return in topological order.
	order := g.TopologicalOrder()
	result := make([]K, 0, len(visited))
	for _, k := range order {
		if visited[k] {
			result = append(result, k)
		}
	}
	return result
}

// Dependents returns the IDs of all nodes that directly depend on the
// given node (i.e., nodes that list id in their HardDeps or SoftDeps).
func (g *Graph[K]) Dependents(id K) []K {
	if _, ok := g.index[id]; !ok {
		return nil
	}

	var result []K
	for _, n := range g.nodes {
		for _, dep := range n.HardDeps {
			if dep == id {
				result = append(result, n.ID)
				goto next
			}
		}
		for _, dep := range n.SoftDeps {
			if dep == id {
				result = append(result, n.ID)
				goto next
			}
		}
	next:
	}
	return result
}

// CheckHardDeps checks which hard dependencies of the given node are not
// satisfied according to the provided predicate. The predicate is called
// once per hard dep; it should return true if the dep is satisfied.
//
// Returns the IDs of unsatisfied hard deps, or nil if all are satisfied.
func (g *Graph[K]) CheckHardDeps(id K, satisfied func(K) bool) []K {
	node := g.Find(id)
	if node == nil {
		return nil
	}

	var unsatisfied []K
	for _, dep := range node.HardDeps {
		if !satisfied(dep) {
			unsatisfied = append(unsatisfied, dep)
		}
	}
	return unsatisfied
}

// CheckSoftDeps checks which soft dependencies of the given node are not
// satisfied according to the provided predicate.
//
// Returns the IDs of unsatisfied soft deps, or nil if all are satisfied.
func (g *Graph[K]) CheckSoftDeps(id K, satisfied func(K) bool) []K {
	node := g.Find(id)
	if node == nil {
		return nil
	}

	var unsatisfied []K
	for _, dep := range node.SoftDeps {
		if !satisfied(dep) {
			unsatisfied = append(unsatisfied, dep)
		}
	}
	return unsatisfied
}

// Available returns all node IDs whose hard dependencies are fully
// satisfied according to the provided predicate, but which are not
// themselves satisfied.
//
// This answers the question: "what can I do next?"
func (g *Graph[K]) Available(satisfied func(K) bool) []K {
	var result []K
	for _, n := range g.nodes {
		if satisfied(n.ID) {
			continue // already done
		}

		allHardMet := true
		for _, dep := range n.HardDeps {
			if !satisfied(dep) {
				allHardMet = false
				break
			}
		}
		if allHardMet {
			result = append(result, n.ID)
		}
	}
	return result
}

// Edges returns all dependency edges in the graph. Each edge indicates
// that Dep must be completed before Dependent, along with the kind
// (Hard or Soft).
func (g *Graph[K]) Edges() []struct {
	Edge[K]
	Kind DepKind
} {
	var result []struct {
		Edge[K]
		Kind DepKind
	}
	for _, n := range g.nodes {
		for _, dep := range n.HardDeps {
			result = append(result, struct {
				Edge[K]
				Kind DepKind
			}{
				Edge: Edge[K]{Dep: dep, Dependent: n.ID},
				Kind: Hard,
			})
		}
		for _, dep := range n.SoftDeps {
			result = append(result, struct {
				Edge[K]
				Kind DepKind
			}{
				Edge: Edge[K]{Dep: dep, Dependent: n.ID},
				Kind: Soft,
			})
		}
	}
	return result
}
