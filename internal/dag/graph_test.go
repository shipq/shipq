package shipqdag_test

import (
	"testing"

	shipqdag "github.com/shipq/shipq/internal/dag"
)

func TestGraphIsValid(t *testing.T) {
	// Graph() panics on structural errors. If this doesn't panic, the
	// graph passes all validation (no cycles, no dangling deps, no dupes).
	_ = shipqdag.Graph()
}

func TestTopologicalOrderIsValid(t *testing.T) {
	g := shipqdag.Graph()
	order := g.TopologicalOrder()

	position := make(map[shipqdag.CommandID]int)
	for i, id := range order {
		position[id] = i
	}

	for _, node := range g.Nodes() {
		for _, dep := range node.HardDeps {
			if position[dep] >= position[node.ID] {
				t.Errorf("%s must come before %s in topological order", dep, node.ID)
			}
		}
	}
}

func TestEmailRequiresAuthAndWorkers(t *testing.T) {
	g := shipqdag.Graph()
	deps := g.TransitiveDeps(shipqdag.CmdEmail)
	depSet := make(map[shipqdag.CommandID]bool)
	for _, d := range deps {
		depSet[d] = true
	}
	for _, required := range []shipqdag.CommandID{
		shipqdag.CmdInit,
		shipqdag.CmdDBSetup,
		shipqdag.CmdAuth,
		shipqdag.CmdWorkers,
	} {
		if !depSet[required] {
			t.Errorf("email should transitively require %s", required)
		}
	}
}

func TestLLMRequiresWorkers(t *testing.T) {
	g := shipqdag.Graph()
	deps := g.TransitiveDeps(shipqdag.CmdLLMCompile)
	depSet := make(map[shipqdag.CommandID]bool)
	for _, d := range deps {
		depSet[d] = true
	}
	if !depSet[shipqdag.CmdWorkers] {
		t.Error("llm compile should transitively require workers")
	}
	if !depSet[shipqdag.CmdDBSetup] {
		t.Error("llm compile should transitively require db_setup (via workers)")
	}
}

func TestWorkersDoesNotHardRequireAuth(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdWorkers)
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdAuth {
			t.Error("workers should not hard-depend on auth; auth should be a soft dep")
		}
	}
}

func TestWorkersHasAuthAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdWorkers)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("workers should have auth as a soft dep")
	}
}

func TestWorkersHardRequiresDBSetup(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdWorkers)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdDBSetup {
			found = true
		}
	}
	if !found {
		t.Error("workers should hard-depend on db_setup")
	}
}

func TestEmailHardRequiresBothAuthAndWorkers(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdEmail)
	foundAuth, foundWorkers := false, false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdAuth {
			foundAuth = true
		}
		if dep == shipqdag.CmdWorkers {
			foundWorkers = true
		}
	}
	if !foundAuth {
		t.Error("email should hard-depend on auth")
	}
	if !foundWorkers {
		t.Error("email should hard-depend on workers")
	}
}

func TestFilesDoesNotHardRequireAuth(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdFiles)
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdAuth {
			t.Error("files should not hard-depend on auth")
		}
	}
}

func TestFilesHasAuthAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdFiles)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("files should have auth as a soft dep")
	}
}

func TestResourceHasAuthAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdResource)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("resource should have auth as a soft dep")
	}
}

func TestResourceHardRequiresMigrateUp(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdResource)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdMigrateUp {
			found = true
		}
	}
	if !found {
		t.Error("resource should hard-depend on migrate_up")
	}
}

func TestCommandNameRoundTrips(t *testing.T) {
	// Every CommandID should have a non-empty CommandName.
	g := shipqdag.Graph()
	for _, node := range g.Nodes() {
		name := shipqdag.CommandName(node.ID)
		if name == "" {
			t.Errorf("CommandName(%q) returned empty string", node.ID)
		}
	}
}

func TestCommandNameSpecificValues(t *testing.T) {
	tests := []struct {
		id   shipqdag.CommandID
		want string
	}{
		{shipqdag.CmdInit, "init"},
		{shipqdag.CmdDBSetup, "db setup"},
		{shipqdag.CmdMigrateNew, "migrate new"},
		{shipqdag.CmdMigrateUp, "migrate up"},
		{shipqdag.CmdMigrateReset, "migrate reset"},
		{shipqdag.CmdDBCompile, "db compile"},
		{shipqdag.CmdAuth, "auth"},
		{shipqdag.CmdSignup, "signup"},
		{shipqdag.CmdAuthGoogle, "auth google"},
		{shipqdag.CmdAuthGitHub, "auth github"},
		{shipqdag.CmdEmail, "email"},
		{shipqdag.CmdFiles, "files"},
		{shipqdag.CmdWorkers, "workers"},
		{shipqdag.CmdWorkersCompile, "workers compile"},
		{shipqdag.CmdResource, "resource"},
		{shipqdag.CmdHandlerGen, "handler generate"},
		{shipqdag.CmdHealth, "health"},
		{shipqdag.CmdHandlerCompile, "handler compile"},
		{shipqdag.CmdLLMCompile, "llm compile"},
		{shipqdag.CmdSeed, "seed"},
		{shipqdag.CmdDocker, "docker"},
		{shipqdag.CmdNix, "nix"},
	}
	for _, tt := range tests {
		got := shipqdag.CommandName(tt.id)
		if got != tt.want {
			t.Errorf("CommandName(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestCommandNameUnknown(t *testing.T) {
	// Unknown IDs should fall back to the string value.
	name := shipqdag.CommandName("unknown_cmd")
	if name != "unknown_cmd" {
		t.Errorf("expected fallback to string, got %q", name)
	}
}

func TestSignupRequiresAuth(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdSignup)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("signup should hard-depend on auth")
	}
}

func TestAuthGoogleRequiresAuth(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdAuthGoogle)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("auth google should hard-depend on auth")
	}
}

func TestAuthGitHubRequiresAuth(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdAuthGitHub)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("auth github should hard-depend on auth")
	}
}

func TestDBCompileRequiresMigrateUp(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdDBCompile)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdMigrateUp {
			found = true
		}
	}
	if !found {
		t.Error("db compile should hard-depend on migrate_up")
	}
}

func TestMigrateUpRequiresDBSetup(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdMigrateUp)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdDBSetup {
			found = true
		}
	}
	if !found {
		t.Error("migrate up should hard-depend on db_setup")
	}
}

func TestDBSetupRequiresInit(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdDBSetup)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdInit {
			found = true
		}
	}
	if !found {
		t.Error("db setup should hard-depend on init")
	}
}

func TestNixHasNoDeps(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdNix)
	if len(node.HardDeps) != 0 {
		t.Errorf("nix should have no hard deps, got %v", node.HardDeps)
	}
	if len(node.SoftDeps) != 0 {
		t.Errorf("nix should have no soft deps, got %v", node.SoftDeps)
	}
}

func TestDockerRequiresInit(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdDocker)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdInit {
			found = true
		}
	}
	if !found {
		t.Error("docker should hard-depend on init")
	}
}

func TestHandlerCompileHasDBCompileAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdHandlerCompile)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdDBCompile {
			found = true
		}
	}
	if !found {
		t.Error("handler compile should have db_compile as a soft dep")
	}
}

func TestWorkersCompileRequiresWorkers(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdWorkersCompile)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdWorkers {
			found = true
		}
	}
	if !found {
		t.Error("workers compile should hard-depend on workers")
	}
}

func TestLLMCompileHasAuthAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdLLMCompile)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("llm compile should have auth as a soft dep")
	}
}

func TestSeedHasAuthAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdSeed)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdAuth {
			found = true
		}
	}
	if !found {
		t.Error("seed should have auth as a soft dep")
	}
}

func TestSeedHardRequiresDBSetup(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdSeed)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdDBSetup {
			found = true
		}
	}
	if !found {
		t.Error("seed should hard-depend on db_setup")
	}
}

func TestAllNodesHaveDescriptions(t *testing.T) {
	g := shipqdag.Graph()
	for _, node := range g.Nodes() {
		if node.Description == "" {
			t.Errorf("node %q has no description", node.ID)
		}
	}
}

func TestGraphContainsAllExpectedCommands(t *testing.T) {
	g := shipqdag.Graph()
	expected := []shipqdag.CommandID{
		shipqdag.CmdInit,
		shipqdag.CmdDBSetup,
		shipqdag.CmdMigrateNew,
		shipqdag.CmdMigrateUp,
		shipqdag.CmdMigrateReset,
		shipqdag.CmdDBCompile,
		shipqdag.CmdAuth,
		shipqdag.CmdSignup,
		shipqdag.CmdAuthGoogle,
		shipqdag.CmdAuthGitHub,
		shipqdag.CmdEmail,
		shipqdag.CmdFiles,
		shipqdag.CmdWorkers,
		shipqdag.CmdWorkersCompile,
		shipqdag.CmdResource,
		shipqdag.CmdHandlerGen,
		shipqdag.CmdHealth,
		shipqdag.CmdHandlerCompile,
		shipqdag.CmdLLMCompile,
		shipqdag.CmdSeed,
		shipqdag.CmdDocker,
		shipqdag.CmdNix,
	}
	for _, id := range expected {
		if g.Find(id) == nil {
			t.Errorf("expected graph to contain command %q", id)
		}
	}
	nodes := g.Nodes()
	if len(nodes) != len(expected) {
		t.Errorf("expected %d nodes, got %d", len(expected), len(nodes))
	}
}

func TestInitHasNoDeps(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdInit)
	if len(node.HardDeps) != 0 {
		t.Errorf("init should have no hard deps, got %v", node.HardDeps)
	}
	if len(node.SoftDeps) != 0 {
		t.Errorf("init should have no soft deps, got %v", node.SoftDeps)
	}
}

func TestMigrateNewRequiresInit(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdMigrateNew)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdInit {
			found = true
		}
	}
	if !found {
		t.Error("migrate new should hard-depend on init")
	}
}

func TestMigrateResetRequiresDBSetup(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdMigrateReset)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdDBSetup {
			found = true
		}
	}
	if !found {
		t.Error("migrate reset should hard-depend on db_setup")
	}
}

func TestAuthRequiresDBSetup(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdAuth)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdDBSetup {
			found = true
		}
	}
	if !found {
		t.Error("auth should hard-depend on db_setup")
	}
}

func TestHandlerGenRequiresMigrateUp(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdHandlerGen)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdMigrateUp {
			found = true
		}
	}
	if !found {
		t.Error("handler generate should hard-depend on migrate_up")
	}
}

func TestHealthRequiresInit(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdHealth)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdInit {
			found = true
		}
	}
	if !found {
		t.Error("health should hard-depend on init")
	}
}

func TestHealthHasDBCompileAsSoftDep(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdHealth)
	found := false
	for _, dep := range node.SoftDeps {
		if dep == shipqdag.CmdDBCompile {
			found = true
		}
	}
	if !found {
		t.Error("health should have db_compile as a soft dep")
	}
}

func TestHandlerCompileRequiresInit(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdHandlerCompile)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdInit {
			found = true
		}
	}
	if !found {
		t.Error("handler compile should hard-depend on init")
	}
}

func TestLLMCompileHardRequiresWorkers(t *testing.T) {
	g := shipqdag.Graph()
	node := g.Find(shipqdag.CmdLLMCompile)
	found := false
	for _, dep := range node.HardDeps {
		if dep == shipqdag.CmdWorkers {
			found = true
		}
	}
	if !found {
		t.Error("llm compile should hard-depend on workers")
	}
}
