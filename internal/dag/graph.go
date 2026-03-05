package shipqdag

import "github.com/shipq/shipq/dag"

// CommandID uniquely identifies a shipq command in the dependency graph.
type CommandID string

const (
	CmdInit           CommandID = "init"
	CmdDBSetup        CommandID = "db_setup"
	CmdMigrateNew     CommandID = "migrate_new"
	CmdMigrateUp      CommandID = "migrate_up"
	CmdMigrateReset   CommandID = "migrate_reset"
	CmdDBCompile      CommandID = "db_compile"
	CmdAuth           CommandID = "auth"
	CmdSignup         CommandID = "signup"
	CmdAuthGoogle     CommandID = "auth_google"
	CmdAuthGitHub     CommandID = "auth_github"
	CmdEmail          CommandID = "email"
	CmdFiles          CommandID = "files"
	CmdWorkers        CommandID = "workers"
	CmdWorkersCompile CommandID = "workers_compile"
	CmdHealth         CommandID = "health"
	CmdResource       CommandID = "resource"
	CmdHandlerGen     CommandID = "handler_generate"
	CmdHandlerCompile CommandID = "handler_compile"
	CmdLLMCompile     CommandID = "llm_compile"
	CmdSeed           CommandID = "seed"
	CmdDocker         CommandID = "docker"
	CmdNix            CommandID = "nix"
)

// commandNames maps each CommandID to its human-readable CLI command name.
var commandNames = map[CommandID]string{
	CmdInit:           "init",
	CmdDBSetup:        "db setup",
	CmdMigrateNew:     "migrate new",
	CmdMigrateUp:      "migrate up",
	CmdMigrateReset:   "migrate reset",
	CmdDBCompile:      "db compile",
	CmdAuth:           "auth",
	CmdSignup:         "signup",
	CmdAuthGoogle:     "auth google",
	CmdAuthGitHub:     "auth github",
	CmdEmail:          "email",
	CmdFiles:          "files",
	CmdWorkers:        "workers",
	CmdWorkersCompile: "workers compile",
	CmdHealth:         "health",
	CmdResource:       "resource",
	CmdHandlerGen:     "handler generate",
	CmdHandlerCompile: "handler compile",
	CmdLLMCompile:     "llm compile",
	CmdSeed:           "seed",
	CmdDocker:         "docker",
	CmdNix:            "nix",
}

// CommandName returns the human-readable CLI command name for a CommandID.
// e.g., CmdDBSetup → "db setup", CmdAuthGoogle → "auth google".
func CommandName(id CommandID) string {
	if name, ok := commandNames[id]; ok {
		return name
	}
	return string(id)
}

// Graph returns the ShipQ command dependency graph.
//
// This calls dag.New internally and panics if the graph is structurally
// invalid — this is intentional, because a broken graph is a programming
// error that should be caught by tests, not handled at runtime.
func Graph() *dag.Graph[CommandID] {
	g, err := dag.New([]dag.Node[CommandID]{
		{
			ID:          CmdInit,
			Description: "Initialize project (go.mod + shipq.ini)",
		},
		{
			ID:          CmdDBSetup,
			Description: "Create dev + test databases",
			HardDeps:    []CommandID{CmdInit},
		},
		{
			ID:          CmdMigrateNew,
			Description: "Create a new migration file",
			HardDeps:    []CommandID{CmdInit},
		},
		{
			ID:          CmdMigrateUp,
			Description: "Apply migrations to dev + test databases",
			HardDeps:    []CommandID{CmdDBSetup},
		},
		{
			ID:          CmdMigrateReset,
			Description: "Drop + recreate databases and reapply migrations",
			HardDeps:    []CommandID{CmdDBSetup},
		},
		{
			ID:          CmdDBCompile,
			Description: "Compile querydefs into typed query runner",
			HardDeps:    []CommandID{CmdMigrateUp},
		},
		{
			ID:          CmdAuth,
			Description: "Generate auth system (tables, handlers, tests)",
			HardDeps:    []CommandID{CmdDBSetup},
		},
		{
			ID:          CmdSignup,
			Description: "Generate signup handler",
			HardDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdAuthGoogle,
			Description: "Add Google OAuth login",
			HardDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdAuthGitHub,
			Description: "Add GitHub OAuth login",
			HardDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdWorkers,
			Description: "Bootstrap workers/channels system",
			HardDeps:    []CommandID{CmdDBSetup},
			SoftDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdWorkersCompile,
			Description: "Recompile channel codegen",
			HardDeps:    []CommandID{CmdWorkers},
		},
		{
			ID:          CmdEmail,
			Description: "Add email verification + password reset",
			HardDeps:    []CommandID{CmdAuth, CmdWorkers},
		},
		{
			ID:          CmdFiles,
			Description: "Generate S3-compatible file upload system",
			HardDeps:    []CommandID{CmdDBSetup},
			SoftDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdResource,
			Description: "Generate CRUD handler(s) for a table",
			HardDeps:    []CommandID{CmdMigrateUp},
			SoftDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdHandlerGen,
			Description: "Generate CRUD handlers for a table",
			HardDeps:    []CommandID{CmdMigrateUp},
		},
		{
			ID:          CmdHealth,
			Description: "Generate healthcheck endpoint and compile handlers",
			HardDeps:    []CommandID{CmdInit},
			SoftDeps:    []CommandID{CmdDBCompile},
		},
		{
			ID:          CmdHandlerCompile,
			Description: "Compile handler registry",
			HardDeps:    []CommandID{CmdInit},
			SoftDeps:    []CommandID{CmdDBCompile},
		},
		{
			ID:          CmdLLMCompile,
			Description: "Compile LLM tool registries",
			HardDeps:    []CommandID{CmdWorkers},
			SoftDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdSeed,
			Description: "Run seed files",
			HardDeps:    []CommandID{CmdDBSetup},
			SoftDeps:    []CommandID{CmdAuth},
		},
		{
			ID:          CmdDocker,
			Description: "Generate production Dockerfiles",
			HardDeps:    []CommandID{CmdInit},
		},
		{
			ID:          CmdNix,
			Description: "Generate shell.nix",
		},
	})
	if err != nil {
		panic("shipq: internal DAG is invalid: " + err.Error())
	}
	return g
}
