package codegen

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/portsql/migrate"
)

// =============================================================================
// ScanRelations Tests - Detecting relationships from schema
// =============================================================================

func TestScanRelations_HasMany(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "name", Type: ddl.StringType},
		},
	}

	relations := ScanRelations(plan)

	// Should find: categories HasMany pets, pets BelongsTo categories
	if len(relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(relations))
	}

	// Verify HasMany
	hasMany := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasMany == nil {
		t.Error("missing HasMany relation from categories to pets")
	}
	if hasMany != nil && hasMany.FKColumn != "category_id" {
		t.Errorf("expected FKColumn='category_id', got %q", hasMany.FKColumn)
	}

	// Verify BelongsTo
	belongsTo := findRelation(relations, "pets", "categories", RelationBelongsTo)
	if belongsTo == nil {
		t.Error("missing BelongsTo relation from pets to categories")
	}
}

func TestScanRelations_ManyToMany(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["tags"] = ddl.Table{
		Name: "tags",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["pet_tags"] = ddl.Table{
		Name:            "pet_tags",
		IsJunctionTable: true,
		Columns: []ddl.ColumnDefinition{
			{Name: "pet_id", Type: ddl.BigintType, References: "pets"},
			{Name: "tag_id", Type: ddl.BigintType, References: "tags"},
		},
	}

	relations := ScanRelations(plan)

	// Should find ManyToMany in both directions
	// pets ManyToMany tags, tags ManyToMany pets
	petToTags := findRelation(relations, "pets", "tags", RelationManyToMany)
	if petToTags == nil {
		t.Error("missing ManyToMany relation from pets to tags")
	}
	if petToTags != nil {
		if petToTags.JunctionTable != "pet_tags" {
			t.Errorf("expected JunctionTable='pet_tags', got %q", petToTags.JunctionTable)
		}
	}

	tagToPets := findRelation(relations, "tags", "pets", RelationManyToMany)
	if tagToPets == nil {
		t.Error("missing ManyToMany relation from tags to pets")
	}
}

func TestScanRelations_Empty(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["users"] = ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
		},
	}

	relations := ScanRelations(plan)

	if len(relations) != 0 {
		t.Errorf("expected 0 relations for table without references, got %d", len(relations))
	}
}

func TestScanRelations_MultipleFK(t *testing.T) {
	// Test a table with multiple foreign key columns
	plan := migrate.NewPlan()
	plan.Schema.Tables["users"] = ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
		},
	}
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
		},
	}
	plan.Schema.Tables["posts"] = ddl.Table{
		Name: "posts",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "author_id", Type: ddl.BigintType, References: "users"},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
		},
	}

	relations := ScanRelations(plan)

	// Should have 4 relations:
	// - users HasMany posts
	// - posts BelongsTo users
	// - categories HasMany posts
	// - posts BelongsTo categories
	if len(relations) != 4 {
		t.Errorf("expected 4 relations, got %d", len(relations))
	}

	// Check all expected relations exist
	if findRelation(relations, "users", "posts", RelationHasMany) == nil {
		t.Error("missing HasMany from users to posts")
	}
	if findRelation(relations, "posts", "users", RelationBelongsTo) == nil {
		t.Error("missing BelongsTo from posts to users")
	}
	if findRelation(relations, "categories", "posts", RelationHasMany) == nil {
		t.Error("missing HasMany from categories to posts")
	}
	if findRelation(relations, "posts", "categories", RelationBelongsTo) == nil {
		t.Error("missing BelongsTo from posts to categories")
	}
}

// Helper to find a specific relation
func findRelation(relations []Relation, from, to string, relType RelationType) *Relation {
	for i := range relations {
		if relations[i].FromTable == from && relations[i].ToTable == to && relations[i].Type == relType {
			return &relations[i]
		}
	}
	return nil
}

// =============================================================================
// GenerateRelationTypes Tests - Go type generation
// =============================================================================

func TestGenerateRelationTypes_HasMany(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "name", Type: ddl.StringType},
		},
	}

	relations := ScanRelations(plan)
	code, err := GenerateRelationTypes(plan, relations)
	if err != nil {
		t.Fatalf("GenerateRelationTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have type for category with pets
	if !strings.Contains(codeStr, "CategoryWithPets") {
		t.Error("expected CategoryWithPets type in generated code")
	}

	// Should have a nested Pet type
	if !strings.Contains(codeStr, "Pets []") {
		t.Error("expected Pets slice field in generated code")
	}
}

func TestGenerateRelationTypes_BelongsTo(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "name", Type: ddl.StringType},
		},
	}

	relations := ScanRelations(plan)
	code, err := GenerateRelationTypes(plan, relations)
	if err != nil {
		t.Fatalf("GenerateRelationTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have type for pet with category
	if !strings.Contains(codeStr, "PetWithCategory") {
		t.Error("expected PetWithCategory type in generated code")
	}

	// Should have a nested Category type (singular)
	if !strings.Contains(codeStr, "Category ") {
		t.Error("expected Category field in generated code")
	}
}

func TestGenerateRelationTypes_ManyToMany(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["tags"] = ddl.Table{
		Name: "tags",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "name", Type: ddl.StringType},
		},
	}
	plan.Schema.Tables["pet_tags"] = ddl.Table{
		Name:            "pet_tags",
		IsJunctionTable: true,
		Columns: []ddl.ColumnDefinition{
			{Name: "pet_id", Type: ddl.BigintType, References: "pets"},
			{Name: "tag_id", Type: ddl.BigintType, References: "tags"},
		},
	}

	relations := ScanRelations(plan)
	code, err := GenerateRelationTypes(plan, relations)
	if err != nil {
		t.Fatalf("GenerateRelationTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should have PetWithTags
	if !strings.Contains(codeStr, "PetWithTags") {
		t.Error("expected PetWithTags type in generated code")
	}

	// Should have TagWithPets
	if !strings.Contains(codeStr, "TagWithPets") {
		t.Error("expected TagWithPets type in generated code")
	}
}

func TestGenerateRelationTypes_Empty(t *testing.T) {
	plan := migrate.NewPlan()
	plan.Schema.Tables["users"] = ddl.Table{
		Name: "users",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
		},
	}

	relations := ScanRelations(plan)
	code, err := GenerateRelationTypes(plan, relations)
	if err != nil {
		t.Fatalf("GenerateRelationTypes failed: %v", err)
	}

	codeStr := string(code)

	// Should just have the package declaration
	if !strings.Contains(codeStr, "package") {
		t.Error("expected package declaration")
	}
}

// =============================================================================
// GenerateRelationSQL Tests - SQL generation for all dialects
// =============================================================================

func TestGenerateRelationSQL_HasMany_Postgres(t *testing.T) {
	plan := createCategoryPetsPlan()
	relations := ScanRelations(plan)
	hasManyRel := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasManyRel == nil {
		t.Fatal("expected HasMany relation")
	}

	sql := GenerateRelationSQL(plan, *hasManyRel, SQLDialectPostgres)

	// Should have SELECT from categories
	if !strings.Contains(sql, `"categories"`) {
		t.Error("SQL should contain categories table")
	}

	// Should have LEFT JOIN pets
	if !strings.Contains(sql, `LEFT JOIN "pets"`) {
		t.Error("SQL should contain LEFT JOIN pets")
	}

	// Should have JSON_AGG (Postgres JSON aggregation)
	if !strings.Contains(sql, "JSON_AGG") {
		t.Error("SQL should contain JSON_AGG for Postgres")
	}

	// Should have GROUP BY
	if !strings.Contains(sql, "GROUP BY") {
		t.Error("SQL should contain GROUP BY")
	}
}

func TestGenerateRelationSQL_HasMany_MySQL(t *testing.T) {
	plan := createCategoryPetsPlan()
	relations := ScanRelations(plan)
	hasManyRel := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasManyRel == nil {
		t.Fatal("expected HasMany relation")
	}

	sql := GenerateRelationSQL(plan, *hasManyRel, SQLDialectMySQL)

	// Should have JSON_ARRAYAGG (MySQL JSON aggregation)
	if !strings.Contains(sql, "JSON_ARRAYAGG") {
		t.Error("SQL should contain JSON_ARRAYAGG for MySQL")
	}

	// Should NOT have Postgres-style JSON_AGG
	if strings.Contains(sql, "JSON_AGG(") && !strings.Contains(sql, "JSON_ARRAYAGG") {
		t.Error("MySQL SQL should not contain Postgres-style JSON_AGG")
	}
}

func TestGenerateRelationSQL_HasMany_SQLite(t *testing.T) {
	plan := createCategoryPetsPlan()
	relations := ScanRelations(plan)
	hasManyRel := findRelation(relations, "categories", "pets", RelationHasMany)
	if hasManyRel == nil {
		t.Fatal("expected HasMany relation")
	}

	sql := GenerateRelationSQL(plan, *hasManyRel, SQLDialectSQLite)

	// Should have JSON_GROUP_ARRAY (SQLite JSON aggregation)
	if !strings.Contains(sql, "JSON_GROUP_ARRAY") {
		t.Error("SQL should contain JSON_GROUP_ARRAY for SQLite")
	}
}

func TestGenerateRelationSQL_BelongsTo_Postgres(t *testing.T) {
	plan := createCategoryPetsPlan()
	relations := ScanRelations(plan)
	belongsToRel := findRelation(relations, "pets", "categories", RelationBelongsTo)
	if belongsToRel == nil {
		t.Fatal("expected BelongsTo relation")
	}

	sql := GenerateRelationSQL(plan, *belongsToRel, SQLDialectPostgres)

	// Should have SELECT from pets
	if !strings.Contains(sql, `"pets"`) {
		t.Error("SQL should contain pets table")
	}

	// Should have LEFT JOIN categories
	if !strings.Contains(sql, `LEFT JOIN "categories"`) {
		t.Error("SQL should contain LEFT JOIN categories")
	}

	// BelongsTo returns single object, so JSON_BUILD_OBJECT not JSON_AGG
	if !strings.Contains(sql, "JSON_BUILD_OBJECT") {
		t.Error("SQL should contain JSON_BUILD_OBJECT for BelongsTo")
	}
}

func TestGenerateRelationSQL_ManyToMany_Postgres(t *testing.T) {
	plan := createPetTagsPlan()
	relations := ScanRelations(plan)
	m2mRel := findRelation(relations, "pets", "tags", RelationManyToMany)
	if m2mRel == nil {
		t.Fatal("expected ManyToMany relation")
	}

	sql := GenerateRelationSQL(plan, *m2mRel, SQLDialectPostgres)

	// Should have SELECT from pets
	if !strings.Contains(sql, `"pets"`) {
		t.Error("SQL should contain pets table")
	}

	// Should have double JOIN (junction + target)
	if !strings.Contains(sql, `"pet_tags"`) {
		t.Error("SQL should contain pet_tags junction table")
	}
	if !strings.Contains(sql, `"tags"`) {
		t.Error("SQL should contain tags table")
	}

	// Should have JSON_AGG for array of tags
	if !strings.Contains(sql, "JSON_AGG") {
		t.Error("SQL should contain JSON_AGG for ManyToMany")
	}

	// Should have GROUP BY
	if !strings.Contains(sql, "GROUP BY") {
		t.Error("SQL should contain GROUP BY")
	}
}

// Helper to create a plan with categories and pets
func createCategoryPetsPlan() *migrate.MigrationPlan {
	plan := migrate.NewPlan()
	plan.Schema.Tables["categories"] = ddl.Table{
		Name: "categories",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "category_id", Type: ddl.BigintType, References: "categories"},
			{Name: "name", Type: ddl.StringType},
			{Name: "status", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	return plan
}

// Helper to create a plan with pets, tags, and pet_tags junction
func createPetTagsPlan() *migrate.MigrationPlan {
	plan := migrate.NewPlan()
	plan.Schema.Tables["pets"] = ddl.Table{
		Name: "pets",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	plan.Schema.Tables["tags"] = ddl.Table{
		Name: "tags",
		Columns: []ddl.ColumnDefinition{
			{Name: "id", Type: ddl.BigintType, PrimaryKey: true},
			{Name: "public_id", Type: ddl.StringType},
			{Name: "name", Type: ddl.StringType},
			{Name: "created_at", Type: ddl.DatetimeType},
			{Name: "updated_at", Type: ddl.DatetimeType},
		},
	}
	plan.Schema.Tables["pet_tags"] = ddl.Table{
		Name:            "pet_tags",
		IsJunctionTable: true,
		Columns: []ddl.ColumnDefinition{
			{Name: "pet_id", Type: ddl.BigintType, References: "pets"},
			{Name: "tag_id", Type: ddl.BigintType, References: "tags"},
		},
	}
	return plan
}
