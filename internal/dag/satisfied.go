package shipqdag

import (
	"os"
	"path/filepath"

	"github.com/shipq/shipq/inifile"
)

// SatisfiedFunc returns a predicate that checks whether a given ShipQ command's
// postconditions are met, by inspecting the project at shipqRoot.
func SatisfiedFunc(shipqRoot string) func(CommandID) bool {
	return func(id CommandID) bool {
		switch id {
		case CmdInit:
			return initSatisfied(shipqRoot)
		case CmdDBSetup:
			return dbSetupSatisfied(shipqRoot)
		case CmdMigrateUp:
			return migrateUpSatisfied(shipqRoot)
		case CmdDBCompile:
			return dbCompileSatisfied(shipqRoot)
		case CmdAuth:
			return authSatisfied(shipqRoot)
		case CmdSignup:
			return signupSatisfied(shipqRoot)
		case CmdAuthGoogle:
			return oauthGoogleSatisfied(shipqRoot)
		case CmdAuthGitHub:
			return oauthGitHubSatisfied(shipqRoot)
		case CmdWorkers:
			return workersSatisfied(shipqRoot)
		case CmdEmail:
			return emailSatisfied(shipqRoot)
		case CmdFiles:
			return filesSatisfied(shipqRoot)
		case CmdLLMCompile:
			return llmSatisfied(shipqRoot)
		default:
			return false // Run-only commands are never "satisfied"
		}
	}
}

func initSatisfied(shipqRoot string) bool {
	_, err := os.Stat(filepath.Join(shipqRoot, "shipq.ini"))
	return err == nil
}

func dbSetupSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Get("db", "database_url") != ""
}

func authSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Section("auth") != nil
}

func workersSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Section("workers") != nil
}

func migrateUpSatisfied(shipqRoot string) bool {
	_, err := os.Stat(filepath.Join(shipqRoot, "shipq", "db", "migrate", "schema.json"))
	return err == nil
}

func dbCompileSatisfied(shipqRoot string) bool {
	// Check for the existence of the typed runner; the dialect-specific
	// path varies, so just check the queries directory is non-empty.
	entries, err := os.ReadDir(filepath.Join(shipqRoot, "shipq", "queries"))
	return err == nil && len(entries) > 0
}

func emailSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Section("email") != nil
}

func filesSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Section("files") != nil
}

func signupSatisfied(shipqRoot string) bool {
	_, err := os.Stat(filepath.Join(shipqRoot, "api", "auth", "signup.go"))
	return err == nil
}

func oauthGoogleSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Get("auth", "oauth_google") == "true"
}

func oauthGitHubSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Get("auth", "oauth_github") == "true"
}

func llmSatisfied(shipqRoot string) bool {
	ini, err := inifile.ParseFile(filepath.Join(shipqRoot, "shipq.ini"))
	if err != nil {
		return false
	}
	return ini.Section("llm") != nil
}
