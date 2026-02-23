package parser

import (
	"fmt"
	"testing"

	"github.com/shipq/shipq/proptest"
)

func TestParseColumnSpec_ValidSimpleTypes(t *testing.T) {
	tests := []struct {
		spec     string
		wantName string
		wantType string
	}{
		{"name:string", "name", "string"},
		{"email:string", "email", "string"},
		{"body:text", "body", "text"},
		{"age:int", "age", "int"},
		{"count:bigint", "count", "bigint"},
		{"active:bool", "active", "bool"},
		{"price:float", "price", "float"},
		{"amount:decimal", "amount", "decimal"},
		{"created_at:datetime", "created_at", "datetime"},
		{"updated_at:timestamp", "updated_at", "timestamp"},
		{"data:binary", "data", "binary"},
		{"metadata:json", "metadata", "json"},
		{"_private:string", "_private", "string"},
		{"col123:int", "col123", "int"},
		{"my_col_name:text", "my_col_name", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			spec, err := ParseColumnSpec(tt.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", spec.Name, tt.wantName)
			}
			if spec.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", spec.Type, tt.wantType)
			}
			if spec.References != "" {
				t.Errorf("References = %q, want empty", spec.References)
			}
		})
	}
}

func TestParseColumnSpec_ValidReferences(t *testing.T) {
	tests := []struct {
		spec           string
		wantName       string
		wantReferences string
	}{
		{"user_id:references:users", "user_id", "users"},
		{"org_id:references:organizations", "org_id", "organizations"},
		{"parent_id:references:categories", "parent_id", "categories"},
		{"_ref:references:_table", "_ref", "_table"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			spec, err := ParseColumnSpec(tt.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", spec.Name, tt.wantName)
			}
			if spec.Type != "references" {
				t.Errorf("Type = %q, want %q", spec.Type, "references")
			}
			if spec.References != tt.wantReferences {
				t.Errorf("References = %q, want %q", spec.References, tt.wantReferences)
			}
		})
	}
}

func TestParseColumnSpec_InvalidSpecs(t *testing.T) {
	tests := []struct {
		spec    string
		wantErr string
	}{
		{"", "column spec cannot be empty"},
		{"name", "expected format"},
		{"name:", "unknown column type"},
		{":string", "identifier cannot be empty"},
		{"123name:string", "must start with a letter or underscore"},
		{"name-col:string", "invalid character"},
		{"name:unknown", "unknown column type"},
		{"name:xyz", "unknown column type"},
		{"user_id:references", "references requires table name"},
		{"user_id:references:", "identifier cannot be empty"},
		{"user_id:references:123table", "must start with a letter or underscore"},
		{"name:string:extra", "too many parts"},
		{"a:b:c:d", "unknown column type"},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			_, err := ParseColumnSpec(tt.spec)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != "" && !containsString(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseColumnSpecs_Multiple(t *testing.T) {
	args := []string{"name:string", "email:string", "user_id:references:users"}
	specs, err := ParseColumnSpecs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("len(specs) = %d, want 3", len(specs))
	}

	// Check first spec
	if specs[0].Name != "name" || specs[0].Type != "string" {
		t.Errorf("specs[0] = %+v, want name:string", specs[0])
	}

	// Check second spec
	if specs[1].Name != "email" || specs[1].Type != "string" {
		t.Errorf("specs[1] = %+v, want email:string", specs[1])
	}

	// Check third spec (reference)
	if specs[2].Name != "user_id" || specs[2].Type != "references" || specs[2].References != "users" {
		t.Errorf("specs[2] = %+v, want user_id:references:users", specs[2])
	}
}

func TestParseColumnSpecs_Empty(t *testing.T) {
	specs, err := ParseColumnSpecs([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("len(specs) = %d, want 0", len(specs))
	}
}

func TestParseColumnSpecs_ErrorOnInvalid(t *testing.T) {
	args := []string{"name:string", "invalid", "email:string"}
	_, err := ParseColumnSpecs(args)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestValidateMigrationName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"users", false},
		{"create_posts", false},
		{"_private", false},
		{"migration123", false},
		{"", true},
		{"123start", true},
		{"has-hyphen", true},
		{"has space", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMigrationName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMigrationName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

// Property-based tests

func TestParseColumnSpec_Roundtrip(t *testing.T) {
	validTypes := []string{"string", "text", "int", "bigint", "bool", "float", "decimal", "datetime", "timestamp", "binary", "json"}

	proptest.QuickCheck(t, "roundtrip for simple types", func(g *proptest.Generator) bool {
		colName := g.IdentifierLower(20)
		colType := validTypes[g.Intn(len(validTypes))]

		spec := fmt.Sprintf("%s:%s", colName, colType)
		parsed, err := ParseColumnSpec(spec)
		if err != nil {
			return false
		}
		return parsed.Name == colName && parsed.Type == colType && parsed.References == ""
	})
}

func TestParseColumnSpec_ReferencesRoundtrip(t *testing.T) {
	proptest.QuickCheck(t, "roundtrip for references", func(g *proptest.Generator) bool {
		colName := g.IdentifierLower(20)
		tableName := g.IdentifierLower(20)

		spec := fmt.Sprintf("%s:references:%s", colName, tableName)
		parsed, err := ParseColumnSpec(spec)
		if err != nil {
			return false
		}
		return parsed.Name == colName &&
			parsed.Type == "references" &&
			parsed.References == tableName
	})
}

func TestParseColumnSpec_InvalidInputsNoPanic(t *testing.T) {
	proptest.QuickCheck(t, "no panic on garbage input", func(g *proptest.Generator) bool {
		garbage := g.EdgeCaseString()
		// We don't care if it errors or not, just that it doesn't panic
		_, _ = ParseColumnSpec(garbage)
		return true
	})
}

func TestParseColumnSpec_RejectsInvalidTypes(t *testing.T) {
	proptest.QuickCheck(t, "rejects invalid types", func(g *proptest.Generator) bool {
		colName := g.IdentifierLower(20)
		// Generate a type that is definitely not in our valid list
		invalidType := g.IdentifierLower(10) + "_invalid"

		spec := fmt.Sprintf("%s:%s", colName, invalidType)
		_, err := ParseColumnSpec(spec)
		// Should error because the type is invalid
		return err != nil
	})
}

func TestParseColumnSpec_AcceptsReservedWords(t *testing.T) {
	proptest.QuickCheck(t, "reserved SQL words accepted as column names", func(g *proptest.Generator) bool {
		reserved := g.EdgeCaseIdentifier()
		spec := fmt.Sprintf("%s:string", reserved)
		parsed, err := ParseColumnSpec(spec)
		if err != nil {
			return false
		}
		return parsed.Name == reserved
	})
}

func TestParseColumnSpecs_Idempotent(t *testing.T) {
	validTypes := []string{"string", "text", "int", "bigint", "bool", "float", "decimal", "datetime", "timestamp", "binary", "json"}

	proptest.QuickCheck(t, "parsing is deterministic", func(g *proptest.Generator) bool {
		// Generate a valid spec
		colName := g.IdentifierLower(15)
		colType := validTypes[g.Intn(len(validTypes))]
		spec := fmt.Sprintf("%s:%s", colName, colType)

		// Parse twice
		parsed1, err1 := ParseColumnSpec(spec)
		parsed2, err2 := ParseColumnSpec(spec)

		// Both should succeed and produce the same result
		if err1 != nil || err2 != nil {
			return false
		}
		return parsed1.Name == parsed2.Name &&
			parsed1.Type == parsed2.Type &&
			parsed1.References == parsed2.References
	})
}

func TestValidateIdentifier_ValidInputs(t *testing.T) {
	proptest.QuickCheck(t, "valid identifiers pass validation", func(g *proptest.Generator) bool {
		ident := g.IdentifierLower(25)
		err := validateIdentifier(ident)
		return err == nil
	})
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
