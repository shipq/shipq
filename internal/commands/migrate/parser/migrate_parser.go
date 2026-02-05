package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// ColumnSpec represents a parsed column specification.
type ColumnSpec struct {
	Name       string
	Type       string
	References string // empty if not a reference
}

// validColumnTypes is the set of supported column types.
var validColumnTypes = map[string]bool{
	"string":    true,
	"text":      true,
	"int":       true,
	"bigint":    true,
	"bool":      true,
	"float":     true,
	"decimal":   true,
	"datetime":  true,
	"timestamp": true,
	"binary":    true,
	"json":      true,
}

// ValidColumnTypesList returns a sorted list of valid column types for error messages.
func ValidColumnTypesList() string {
	return "string, text, int, bigint, bool, float, decimal, datetime, timestamp, binary, json"
}

// ParseColumnSpec parses a column spec like "name:string" or "user_id:references:users".
// Returns an error if the spec is invalid.
func ParseColumnSpec(spec string) (*ColumnSpec, error) {
	if spec == "" {
		return nil, fmt.Errorf("column spec cannot be empty")
	}

	parts := strings.Split(spec, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid column spec %q: expected format 'name:type' or 'name:references:table'", spec)
	}

	name := parts[0]
	colType := parts[1]

	// Validate column name
	if err := validateIdentifier(name); err != nil {
		return nil, fmt.Errorf("invalid column name in spec %q: %w", spec, err)
	}

	// Handle references type
	if colType == "references" {
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid column spec %q: references requires table name (format: 'name:references:table')", spec)
		}
		tableName := parts[2]
		if err := validateIdentifier(tableName); err != nil {
			return nil, fmt.Errorf("invalid table name in spec %q: %w", spec, err)
		}
		return &ColumnSpec{
			Name:       name,
			Type:       "references",
			References: tableName,
		}, nil
	}

	// Validate column type
	if !validColumnTypes[colType] {
		return nil, fmt.Errorf("unknown column type %q in spec %q, valid types are: %s", colType, spec, ValidColumnTypesList())
	}

	// Check for extra parts
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid column spec %q: too many parts (expected 'name:type' or 'name:references:table')", spec)
	}

	return &ColumnSpec{
		Name: name,
		Type: colType,
	}, nil
}

// ParseColumnSpecs parses multiple column specs from command line args.
func ParseColumnSpecs(args []string) ([]ColumnSpec, error) {
	specs := make([]ColumnSpec, 0, len(args))
	for _, arg := range args {
		spec, err := ParseColumnSpec(arg)
		if err != nil {
			return nil, err
		}
		specs = append(specs, *spec)
	}
	return specs, nil
}

// validateIdentifier checks if a string is a valid identifier.
// An identifier must start with a letter or underscore and contain only
// letters, digits, and underscores.
func validateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("identifier cannot be empty")
	}

	runes := []rune(s)
	first := runes[0]

	// First character must be letter or underscore
	if !unicode.IsLetter(first) && first != '_' {
		return fmt.Errorf("identifier must start with a letter or underscore, got %q", string(first))
	}

	// Rest must be letters, digits, or underscores
	for i, r := range runes[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("identifier contains invalid character %q at position %d", string(r), i+1)
		}
	}

	return nil
}

// ValidateMigrationName validates that a migration name is a valid identifier.
func ValidateMigrationName(name string) error {
	if name == "" {
		return fmt.Errorf("migration name required")
	}
	if err := validateIdentifier(name); err != nil {
		return fmt.Errorf("invalid migration name %q: must be a valid identifier", name)
	}
	return nil
}
