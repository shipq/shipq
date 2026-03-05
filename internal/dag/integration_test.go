package shipqdag_test

import (
	"os"
	"path/filepath"
	"testing"

	shipqdag "github.com/shipq/shipq/internal/dag"
)

// TestDAGChecksMatchExistingChecks runs each command's prerequisite logic
// against a matrix of project states and verifies the DAG check agrees.
func TestDAGChecksMatchExistingChecks(t *testing.T) {
	g := shipqdag.Graph()

	toSet := func(ids []shipqdag.CommandID) map[shipqdag.CommandID]bool {
		m := make(map[shipqdag.CommandID]bool)
		for _, id := range ids {
			m[id] = true
		}
		return m
	}

	t.Run("empty dir: everything except nix/init should have unmet deps", func(t *testing.T) {
		dir := t.TempDir()
		satisfied := shipqdag.SatisfiedFunc(dir)

		// Commands that should have no hard deps (roots)
		for _, cmd := range []shipqdag.CommandID{shipqdag.CmdInit, shipqdag.CmdNix} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) != 0 {
				t.Errorf("%s should have no unmet hard deps in empty dir, got %v", cmd, unmet)
			}
		}

		// Commands that depend on init should have unmet deps
		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdDBSetup,
			shipqdag.CmdMigrateNew,
			shipqdag.CmdDocker,
			shipqdag.CmdHealth,
			shipqdag.CmdHandlerCompile,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) == 0 {
				t.Errorf("%s should have unmet hard deps in empty dir", cmd)
			}
			unmetSet := toSet(unmet)
			if !unmetSet[shipqdag.CmdInit] {
				t.Errorf("%s should list init as unmet dep, got %v", cmd, unmet)
			}
		}
	})

	t.Run("after init: db setup, migrate new, docker should pass", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[project]\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		// These should have all hard deps met after init
		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdDBSetup,
			shipqdag.CmdMigrateNew,
			shipqdag.CmdDocker,
			shipqdag.CmdHealth,
			shipqdag.CmdHandlerCompile,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) != 0 {
				t.Errorf("%s should have no unmet hard deps after init, got %v", cmd, unmet)
			}
		}

		// These should still have unmet deps (need db_setup)
		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdMigrateUp,
			shipqdag.CmdMigrateReset,
			shipqdag.CmdAuth,
			shipqdag.CmdWorkers,
			shipqdag.CmdFiles,
			shipqdag.CmdSeed,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) == 0 {
				t.Errorf("%s should have unmet hard deps after init only, got none", cmd)
			}
		}
	})

	t.Run("after init + db setup: migrate up, auth, workers, seed, files should pass", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdMigrateUp,
			shipqdag.CmdMigrateReset,
			shipqdag.CmdAuth,
			shipqdag.CmdWorkers,
			shipqdag.CmdFiles,
			shipqdag.CmdSeed,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) != 0 {
				t.Errorf("%s should have no unmet hard deps after init + db_setup, got %v", cmd, unmet)
			}
		}

		// These should still have unmet deps (need migrate_up or auth)
		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdDBCompile,
			shipqdag.CmdResource,
			shipqdag.CmdHandlerGen,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) == 0 {
				t.Errorf("%s should have unmet deps (needs migrate_up)", cmd)
			}
			unmetSet := toSet(unmet)
			if !unmetSet[shipqdag.CmdMigrateUp] {
				t.Errorf("%s should list migrate_up as unmet dep, got %v", cmd, unmet)
			}
		}

		// signup, auth google, auth github should need auth
		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdSignup,
			shipqdag.CmdAuthGoogle,
			shipqdag.CmdAuthGitHub,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) == 0 {
				t.Errorf("%s should have unmet deps (needs auth)", cmd)
			}
			unmetSet := toSet(unmet)
			if !unmetSet[shipqdag.CmdAuth] {
				t.Errorf("%s should list auth as unmet dep, got %v", cmd, unmet)
			}
		}
	})

	t.Run("after init + db setup + auth: signup, auth google, workers should pass", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdSignup,
			shipqdag.CmdAuthGoogle,
			shipqdag.CmdAuthGitHub,
			shipqdag.CmdWorkers,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) != 0 {
				t.Errorf("%s should have no unmet hard deps after init + db_setup + auth, got %v", cmd, unmet)
			}
		}

		// email should still need workers
		unmet := g.CheckHardDeps(shipqdag.CmdEmail, satisfied)
		if len(unmet) == 0 {
			t.Error("email should have unmet deps (needs workers)")
		}
		unmetSet := toSet(unmet)
		if !unmetSet[shipqdag.CmdWorkers] {
			t.Errorf("email should list workers as unmet dep, got %v", unmet)
		}
	})

	t.Run("after init + db setup + auth + workers: email, llm should pass", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\n[workers]\nredis_url = redis://localhost\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		for _, cmd := range []shipqdag.CommandID{
			shipqdag.CmdEmail,
			shipqdag.CmdLLMCompile,
			shipqdag.CmdWorkersCompile,
		} {
			unmet := g.CheckHardDeps(cmd, satisfied)
			if len(unmet) != 0 {
				t.Errorf("%s should have no unmet hard deps after init + db_setup + auth + workers, got %v", cmd, unmet)
			}
		}
	})

	t.Run("workers soft-dep on auth: workers available without auth but with warning", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		// Hard deps should be met
		unmetHard := g.CheckHardDeps(shipqdag.CmdWorkers, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("workers hard deps should be met after db_setup, got %v", unmetHard)
		}

		// Soft dep on auth should be unmet
		unmetSoft := g.CheckSoftDeps(shipqdag.CmdWorkers, satisfied)
		if len(unmetSoft) == 0 {
			t.Error("workers should have unmet soft deps (auth)")
		}
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdAuth] {
			t.Errorf("workers should list auth as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("resource soft-dep on auth: available without auth but with warning", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		// Create schema.json so migrate_up is satisfied
		schemaDir := filepath.Join(dir, "shipq", "db", "migrate")
		os.MkdirAll(schemaDir, 0755)
		os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte("{}"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		// Hard deps should be met
		unmetHard := g.CheckHardDeps(shipqdag.CmdResource, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("resource hard deps should be met, got %v", unmetHard)
		}

		// Soft dep on auth should be unmet
		unmetSoft := g.CheckSoftDeps(shipqdag.CmdResource, satisfied)
		if len(unmetSoft) == 0 {
			t.Error("resource should have unmet soft deps (auth)")
		}
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdAuth] {
			t.Errorf("resource should list auth as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("files soft-dep on auth: available without auth", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		unmetHard := g.CheckHardDeps(shipqdag.CmdFiles, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("files hard deps should be met after db_setup, got %v", unmetHard)
		}

		unmetSoft := g.CheckSoftDeps(shipqdag.CmdFiles, satisfied)
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdAuth] {
			t.Errorf("files should list auth as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("llm compile soft-dep on auth", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[workers]\nredis_url = redis://localhost\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		unmetHard := g.CheckHardDeps(shipqdag.CmdLLMCompile, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("llm compile hard deps should be met after workers, got %v", unmetHard)
		}

		unmetSoft := g.CheckSoftDeps(shipqdag.CmdLLMCompile, satisfied)
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdAuth] {
			t.Errorf("llm compile should list auth as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("handler compile soft-dep on db_compile", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[project]\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		unmetHard := g.CheckHardDeps(shipqdag.CmdHandlerCompile, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("handler compile hard deps should be met after init, got %v", unmetHard)
		}

		unmetSoft := g.CheckSoftDeps(shipqdag.CmdHandlerCompile, satisfied)
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdDBCompile] {
			t.Errorf("handler compile should list db_compile as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("health soft-dep on db_compile", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"), []byte("[project]\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		unmetHard := g.CheckHardDeps(shipqdag.CmdHealth, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("health hard deps should be met after init, got %v", unmetHard)
		}

		unmetSoft := g.CheckSoftDeps(shipqdag.CmdHealth, satisfied)
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdDBCompile] {
			t.Errorf("health should list db_compile as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("seed soft-dep on auth", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		satisfied := shipqdag.SatisfiedFunc(dir)

		unmetHard := g.CheckHardDeps(shipqdag.CmdSeed, satisfied)
		if len(unmetHard) != 0 {
			t.Errorf("seed hard deps should be met after db_setup, got %v", unmetHard)
		}

		unmetSoft := g.CheckSoftDeps(shipqdag.CmdSeed, satisfied)
		softSet := toSet(unmetSoft)
		if !softSet[shipqdag.CmdAuth] {
			t.Errorf("seed should list auth as unmet soft dep, got %v", unmetSoft)
		}
	})

	t.Run("full project: all soft deps satisfied", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n[auth]\nprotect_by_default = true\noauth_google = true\noauth_github = true\n[workers]\nredis_url = redis://localhost\n[email]\nsmtp_host = localhost\n[files]\nbucket = test\n[llm]\ntool_pkgs = tools\n"), 0644)

		// Create schema.json and queries
		schemaDir := filepath.Join(dir, "shipq", "db", "migrate")
		os.MkdirAll(schemaDir, 0755)
		os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte("{}"), 0644)

		queriesDir := filepath.Join(dir, "shipq", "queries")
		os.MkdirAll(queriesDir, 0755)
		os.WriteFile(filepath.Join(queriesDir, "types.go"), []byte("package queries\n"), 0644)

		// Create signup.go
		signupDir := filepath.Join(dir, "api", "auth")
		os.MkdirAll(signupDir, 0755)
		os.WriteFile(filepath.Join(signupDir, "signup.go"), []byte("package auth\n"), 0644)

		satisfied := shipqdag.SatisfiedFunc(dir)

		// All commands that have satisfiable state should be satisfied
		satisfiableCmds := []shipqdag.CommandID{
			shipqdag.CmdInit,
			shipqdag.CmdDBSetup,
			shipqdag.CmdMigrateUp,
			shipqdag.CmdDBCompile,
			shipqdag.CmdAuth,
			shipqdag.CmdSignup,
			shipqdag.CmdAuthGoogle,
			shipqdag.CmdAuthGitHub,
			shipqdag.CmdWorkers,
			shipqdag.CmdEmail,
			shipqdag.CmdFiles,
			shipqdag.CmdLLMCompile,
		}
		for _, cmd := range satisfiableCmds {
			if !satisfied(cmd) {
				t.Errorf("%s should be satisfied in full project", cmd)
			}
		}

		// All soft deps should also be satisfied
		for _, cmd := range satisfiableCmds {
			unmetSoft := g.CheckSoftDeps(cmd, satisfied)
			if len(unmetSoft) != 0 {
				t.Errorf("%s should have no unmet soft deps in full project, got %v", cmd, unmetSoft)
			}
		}
	})
}

// TestCheckPrerequisitesHelper verifies the convenience function returns
// the expected result for various scenarios.
func TestCheckPrerequisitesHelper(t *testing.T) {
	t.Run("returns true when all hard deps met", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		if !shipqdag.CheckPrerequisites(shipqdag.CmdAuth, dir) {
			t.Error("CheckPrerequisites should return true when hard deps are met")
		}
	})

	t.Run("returns false when hard deps unmet", func(t *testing.T) {
		dir := t.TempDir()
		// No shipq.ini, so init is not satisfied
		if shipqdag.CheckPrerequisites(shipqdag.CmdDBSetup, dir) {
			t.Error("CheckPrerequisites should return false when hard deps are unmet")
		}
	})

	t.Run("returns true with unmet soft deps (warns only)", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "shipq.ini"),
			[]byte("[project]\n[db]\ndatabase_url = sqlite://dev.db\n"), 0644)
		// Auth is a soft dep of workers — should pass but warn
		if !shipqdag.CheckPrerequisites(shipqdag.CmdWorkers, dir) {
			t.Error("CheckPrerequisites should return true when only soft deps are unmet")
		}
	})
}
