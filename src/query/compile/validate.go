package compile

import (
	"fmt"
	"regexp"
)

// identifierRegex matches valid SQL identifiers.
// Identifiers must start with a letter or underscore, followed by letters, digits, or underscores.
var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier checks that a name is a valid SQL identifier.
// Returns an error if the identifier is invalid.
// Valid identifiers match: ^[a-zA-Z_][a-zA-Z0-9_]*$
func ValidateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if !identifierRegex.MatchString(name) {
		return fmt.Errorf("invalid identifier %q: must start with a letter or underscore and contain only letters, digits, and underscores", name)
	}
	return nil
}
