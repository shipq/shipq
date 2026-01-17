package ref

import "testing"

func TestTableRef_TableName(t *testing.T) {
	tr := &TableRef{Name: "users"}
	if tr.TableName() != "users" {
		t.Errorf("expected 'users', got %q", tr.TableName())
	}
}

func TestTableRef_TableName_Empty(t *testing.T) {
	tr := &TableRef{Name: ""}
	if tr.TableName() != "" {
		t.Errorf("expected empty string, got %q", tr.TableName())
	}
}

func TestTableRef_TableName_SpecialChars(t *testing.T) {
	// Table names with underscores (common in SQL)
	tr := &TableRef{Name: "pet_tags"}
	if tr.TableName() != "pet_tags" {
		t.Errorf("expected 'pet_tags', got %q", tr.TableName())
	}
}
