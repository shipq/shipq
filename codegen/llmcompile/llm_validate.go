package llmcompile

import (
	"fmt"

	"github.com/shipq/shipq/inifile"
)

// ValidatePrerequisites checks that the channel/worker infrastructure
// is set up before LLM compilation can proceed.
//
// LLM support requires:
//   - [channels] or [workers] section (workers must be set up)
//   - Redis (for task queue)
//   - Centrifugo (for realtime streaming)
//   - A database (for conversation persistence)
//
// Auth is NOT required — public channels work fine for LLM. The
// account_id columns in llm_conversations are nullable.
func ValidatePrerequisites(ini *inifile.File) error {
	if ini.Section("workers") == nil {
		return fmt.Errorf("LLM support requires channel workers to be set up. Run `shipq workers` first.")
	}

	if ini.Get("workers", "redis_url") == "" {
		return fmt.Errorf("LLM support requires Redis (for task queue). Configure [workers] redis_url.")
	}

	if ini.Get("workers", "centrifugo_api_url") == "" {
		return fmt.Errorf("LLM support requires Centrifugo (for realtime). Configure [workers] centrifugo_api_url.")
	}

	if ini.Get("db", "database_url") == "" {
		return fmt.Errorf("LLM support requires a database. Run `shipq db setup` first.")
	}

	return nil
}
