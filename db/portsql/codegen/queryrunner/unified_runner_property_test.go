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

func TestProperty_InsertSelect_GeneratesValidGoCode(t *testing.T) {
	proptest.QuickCheck(t, "INSERT...SELECT generates valid Go", func(g *proptest.Generator) bool {
		// Generate a random table and column names
		tableName := g.Identifier(15)
		if tableName == "" {
			tableName = "target"
		}
		sourceTable := g.Identifier(15)
		if sourceTable == "" {
			sourceTable = "source"
		}
		colName := g.Identifier(15)
		if colName == "" {
			colName = "name"
		}

		userQuery := query.SerializedQuery{
			Name:       "InsertFromSource",
			ReturnType: "exec",
			AST: &query.SerializedAST{
				Kind:      "insert",
				FromTable: query.SerializedTableRef{Name: tableName},
				InsertCols: []query.SerializedColumn{
					{Table: tableName, Name: colName, GoType: "string"},
				},
				InsertSource: &query.SerializedAST{
					Kind:      "select",
					FromTable: query.SerializedTableRef{Name: sourceTable},
					SelectCols: []query.SerializedSelectExpr{
						{Expr: query.SerializedExpr{
							Type:   "column",
							Column: &query.SerializedColumn{Table: sourceTable, Name: colName, GoType: "string"},
						}},
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

			runnerCode, err := GenerateUnifiedRunner(cfg)
			if err != nil {
				t.Logf("GenerateUnifiedRunner(%s) failed: %v", dialect, err)
				return false
			}

			_, parseErr := parser.ParseFile(token.NewFileSet(), "runner.go", runnerCode, parser.AllErrors)
			if parseErr != nil {
				t.Logf("[%s] generated runner code is not valid Go: %v\n%s", dialect, parseErr, string(runnerCode))
				return false
			}
		}

		return true
	})
}
