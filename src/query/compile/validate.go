package compile

import (
	"fmt"
	"regexp"

	"github.com/portsql/portsql/src/query"
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

// ValidateAST validates basic AST invariants before compilation.
// This catches common errors early with clear messages rather than
// producing invalid SQL or panicking during compilation.
func ValidateAST(ast *query.AST) error {
	if ast == nil {
		return fmt.Errorf("AST cannot be nil")
	}

	// Validate based on query kind
	switch ast.Kind {
	case query.InsertQuery:
		if err := validateInsert(ast); err != nil {
			return err
		}
	case query.UpdateQuery:
		if err := validateUpdate(ast); err != nil {
			return err
		}
	}

	// Validate CTEs
	for i, cte := range ast.CTEs {
		if err := ValidateIdentifier(cte.Name); err != nil {
			return fmt.Errorf("CTE %d: %w", i, err)
		}
		// Validate column names in CTE
		for j, col := range cte.Columns {
			if err := ValidateIdentifier(col); err != nil {
				return fmt.Errorf("CTE %q column %d: %w", cte.Name, j, err)
			}
		}
		// Recursively validate CTE query
		if cte.Query != nil {
			if err := ValidateAST(cte.Query); err != nil {
				return fmt.Errorf("CTE %q: %w", cte.Name, err)
			}
		}
	}

	// Validate set operation branches
	if ast.SetOp != nil {
		if ast.SetOp.Left != nil {
			if err := ValidateAST(ast.SetOp.Left); err != nil {
				return fmt.Errorf("set operation left branch: %w", err)
			}
		}
		if ast.SetOp.Right != nil {
			if err := ValidateAST(ast.SetOp.Right); err != nil {
				return fmt.Errorf("set operation right branch: %w", err)
			}
		}
	}

	return nil
}

func validateInsert(ast *query.AST) error {
	// If column list is provided, it must match the number of values
	if len(ast.InsertCols) > 0 && len(ast.InsertCols) != len(ast.InsertVals) {
		return fmt.Errorf("INSERT column count (%d) does not match value count (%d)",
			len(ast.InsertCols), len(ast.InsertVals))
	}
	return nil
}

func validateUpdate(ast *query.AST) error {
	// UPDATE must have at least one SET clause
	if len(ast.SetClauses) == 0 {
		return fmt.Errorf("UPDATE requires at least one SET clause")
	}
	return nil
}
