package queryrunner

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/dbstrings"
	"github.com/shipq/shipq/proptest"
)

func TestProperty_ExistsInSelect_InfersBoolean(t *testing.T) {
	proptest.QuickCheck(t, "EXISTS in SELECT position infers bool", func(g *proptest.Generator) bool {
		// Generate a random alias for the EXISTS column
		alias := g.Identifier(15)

		// Build a user query with EXISTS in SELECT position
		userQuery := query.SerializedQuery{
			Name:       "CheckExists",
			ReturnType: "one",
			AST: &query.SerializedAST{
				Kind:      "select",
				FromTable: query.SerializedTableRef{Name: "items"},
				SelectCols: []query.SerializedSelectExpr{
					{
						Expr: query.SerializedExpr{
							Type: "exists",
							Exists: &query.SerializedExists{
								Subquery: &query.SerializedAST{
									Kind:      "select",
									FromTable: query.SerializedTableRef{Name: "items"},
									SelectCols: []query.SerializedSelectExpr{
										{Expr: query.SerializedExpr{
											Type:    "literal",
											Literal: float64(1),
										}},
									},
								},
							},
						},
						Alias: alias,
					},
				},
			},
		}

		for _, dialect := range []string{"postgres", "mysql", "sqlite"} {
			cfg := UnifiedRunnerConfig{
				ModulePath:  "example.com/myapp",
				Dialect:     dialect,
				UserQueries: []query.SerializedQuery{userQuery},
			}

			// Check the types file (where result structs are defined)
			typesCode, err := GenerateSharedTypes(cfg)
			if err != nil {
				t.Logf("GenerateSharedTypes(%s) failed: %v", dialect, err)
				return false
			}

			typesStr := string(typesCode)

			// The EXISTS alias field should be bool, not any
			pascalAlias := dbstrings.ToPascalCase(alias)
			boolField := pascalAlias + " bool"
			if !strings.Contains(typesStr, boolField) {
				t.Logf("[%s] types code does not contain %q for EXISTS field", dialect, boolField)
				return false
			}

			// Should NOT infer "any" for the EXISTS alias
			anyField := dbstrings.ToPascalCase(alias) + " any"
			if strings.Contains(typesStr, anyField) {
				t.Logf("[%s] EXISTS field incorrectly inferred as any: found %q", dialect, anyField)
				return false
			}

			// Verify types file is syntactically valid Go
			_, parseErr := parser.ParseFile(token.NewFileSet(), "types.go", typesCode, parser.AllErrors)
			if parseErr != nil {
				t.Logf("[%s] generated types code is not valid Go: %v", dialect, parseErr)
				return false
			}

			// Also verify runner compiles
			runnerCode, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Logf("GenerateUnifiedRunner(%s) failed: %v", dialect, err)
				return false
			}
			_, parseErr = parser.ParseFile(token.NewFileSet(), "runner.go", runnerCode, parser.AllErrors)
			if parseErr != nil {
				t.Logf("[%s] generated runner code is not valid Go: %v", dialect, parseErr)
				return false
			}
		}

		return true
	})
}
