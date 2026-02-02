package migrate

import (
	"fmt"
	"strconv"

	"github.com/shipq/shipq/db/portsql/ddl"
	"github.com/shipq/shipq/db/proptest"
)

// =============================================================================
// Column Type Constants and Weights
// =============================================================================

// AllColumnTypes lists all supported column types for random generation
var AllColumnTypes = []string{
	ddl.IntegerType,
	ddl.BigintType,
	ddl.DecimalType,
	ddl.FloatType,
	ddl.BooleanType,
	ddl.StringType,
	ddl.TextType,
	ddl.DatetimeType,
	ddl.BinaryType,
	ddl.JSONType,
}

// CommonColumnTypes lists commonly used column types (weighted more heavily)
var CommonColumnTypes = []string{
	ddl.IntegerType,
	ddl.BigintType,
	ddl.StringType,
	ddl.BooleanType,
	ddl.DatetimeType,
}

// =============================================================================
// Identifier Generators
// =============================================================================

// GenerateTableName generates a valid table name for testing.
// Table names start with a letter, contain only alphanumeric and underscore,
// and are between 3-30 characters.
func GenerateTableName(g *proptest.Generator) string {
	// Use a prefix to avoid collision with reserved words
	prefix := "tbl_"
	suffix := g.IdentifierLower(20)
	return prefix + suffix
}

// GenerateColumnName generates a valid column name for testing.
// Column names start with a letter, contain only alphanumeric and underscore,
// and are between 2-20 characters.
func GenerateColumnName(g *proptest.Generator) string {
	return g.IdentifierLower(15)
}

// GenerateIndexName generates a valid index name for testing.
func GenerateIndexName(g *proptest.Generator, tableName string, columns []string) string {
	return ddl.GenerateIndexName(tableName, columns)
}

// GenerateUniqueColumnNames generates n unique column names.
func GenerateUniqueColumnNames(g *proptest.Generator, n int) []string {
	return g.UniqueIdentifiers(n, 15)
}

// =============================================================================
// Column Type Generator
// =============================================================================

// GenerateColumnType picks a random column type.
func GenerateColumnType(g *proptest.Generator) string {
	// 70% common types, 30% all types
	if g.Float64() < 0.7 {
		return proptest.Pick(g, CommonColumnTypes)
	}
	return proptest.Pick(g, AllColumnTypes)
}

// =============================================================================
// Default Value Generators
// =============================================================================

// GenerateDefaultForColumn generates an appropriate default value for a column,
// respecting the column's type and length constraints.
func GenerateDefaultForColumn(g *proptest.Generator, col *ddl.ColumnDefinition) *string {
	// 50% chance of no default
	if g.Bool() {
		return nil
	}

	var val string
	switch col.Type {
	case ddl.IntegerType, ddl.BigintType:
		val = strconv.Itoa(g.IntRange(-1000, 1000))

	case ddl.FloatType:
		val = strconv.FormatFloat(g.Float64Range(-1000.0, 1000.0), 'f', 2, 64)

	case ddl.DecimalType:
		// Respect precision and scale constraints
		// DECIMAL(precision, scale) means:
		// - precision = total digits
		// - scale = digits after decimal point
		// - digits before decimal = precision - scale
		precision := 10
		scale := 2
		if col.Precision != nil {
			precision = *col.Precision
		}
		if col.Scale != nil {
			scale = *col.Scale
		}
		intDigits := precision - scale
		if intDigits < 0 {
			intDigits = 0
		}
		// Generate a value that fits: max integer part is 10^intDigits - 1
		maxInt := 1
		for i := 0; i < intDigits; i++ {
			maxInt *= 10
		}
		maxInt-- // e.g., for 2 int digits, max is 99
		if maxInt < 0 {
			maxInt = 0
		}
		intPart := g.IntRange(0, maxInt)
		// For decimal part, generate up to 'scale' digits
		if scale > 0 {
			format := fmt.Sprintf("%%d.%%0%dd", scale)
			decMax := 1
			for i := 0; i < scale; i++ {
				decMax *= 10
			}
			decPart := g.IntRange(0, decMax-1)
			val = fmt.Sprintf(format, intPart, decPart)
		} else {
			val = strconv.Itoa(intPart)
		}

	case ddl.BooleanType:
		if g.Bool() {
			val = "true"
		} else {
			val = "false"
		}

	case ddl.StringType:
		// Respect the column length constraint
		maxLen := 255
		if col.Length != nil {
			maxLen = *col.Length
		}
		val = GenerateSafeStringDefaultWithMaxLen(g, maxLen)

	case ddl.TextType:
		// MySQL doesn't allow DEFAULT values on TEXT columns
		// For cross-database compatibility, don't generate defaults
		return nil

	case ddl.DatetimeType:
		// Return a fixed timestamp or nil
		if g.Bool() {
			val = "2024-01-01 00:00:00"
		} else {
			return nil
		}

	case ddl.JSONType:
		// MySQL doesn't allow DEFAULT values on JSON columns
		// For cross-database compatibility, don't generate defaults
		return nil

	case ddl.BinaryType:
		// MySQL doesn't allow DEFAULT values on BLOB columns
		// For cross-database compatibility, don't generate defaults
		return nil

	default:
		return nil
	}
	return &val
}

// GenerateDefaultForType generates an appropriate default value for a column type.
// Returns nil for no default. Note: for string types, this doesn't respect length.
// Use GenerateDefaultForColumn when you have the full column definition.
func GenerateDefaultForType(g *proptest.Generator, colType string) *string {
	// 50% chance of no default
	if g.Bool() {
		return nil
	}

	var val string
	switch colType {
	case ddl.IntegerType, ddl.BigintType:
		val = strconv.Itoa(g.IntRange(-1000, 1000))

	case ddl.FloatType:
		val = strconv.FormatFloat(g.Float64Range(-1000.0, 1000.0), 'f', 2, 64)

	case ddl.DecimalType:
		val = strconv.FormatFloat(g.Float64Range(0.0, 999.99), 'f', 2, 64)

	case ddl.BooleanType:
		if g.Bool() {
			val = "true"
		} else {
			val = "false"
		}

	case ddl.StringType:
		val = GenerateSafeStringDefaultWithMaxLen(g, 50) // Conservative default

	case ddl.TextType:
		// MySQL doesn't allow DEFAULT values on TEXT columns
		// For cross-database compatibility, don't generate defaults
		return nil

	case ddl.DatetimeType:
		// Return a fixed timestamp or nil
		if g.Bool() {
			val = "2024-01-01 00:00:00"
		} else {
			return nil
		}

	case ddl.JSONType:
		// MySQL doesn't allow DEFAULT values on JSON columns
		// For cross-database compatibility, don't generate defaults
		return nil

	case ddl.BinaryType:
		// MySQL doesn't allow DEFAULT values on BLOB columns
		// For cross-database compatibility, don't generate defaults
		return nil

	default:
		return nil
	}
	return &val
}

// GenerateSafeStringDefaultWithMaxLen generates a string default that's safe for all databases,
// respecting the given maximum length.
func GenerateSafeStringDefaultWithMaxLen(g *proptest.Generator, maxLen int) string {
	// Start with simple strings, gradually add complexity
	roll := g.Float64()

	if roll < 0.3 {
		// Simple alphanumeric - respect max length
		length := min(20, maxLen)
		if length <= 0 {
			return ""
		}
		return g.StringAlphaNum(g.IntRange(1, length))
	} else if roll < 0.5 {
		// With spaces - need at least 11 chars for "xxxxx xxxxx"
		if maxLen < 3 {
			return g.StringAlpha(maxLen)
		}
		halfLen := min(5, (maxLen-1)/2)
		return g.StringAlpha(halfLen) + " " + g.StringAlpha(halfLen)
	} else if roll < 0.7 {
		// Edge case strings - filter by max length
		options := []string{
			"",            // empty
			"hello",       // simple
			"hello world", // with space
			"it's a test", // apostrophe
			"NULL",        // SQL keyword as string
			"true",        // boolean keyword
			"123",         // numeric string
		}
		// Filter options that fit
		var validOptions []string
		for _, opt := range options {
			if len(opt) <= maxLen {
				validOptions = append(validOptions, opt)
			}
		}
		if len(validOptions) == 0 {
			return ""
		}
		return proptest.Pick(g, validOptions)
	} else {
		// Random printable, but avoid problematic characters for now
		length := min(30, maxLen)
		if length <= 0 {
			return ""
		}
		return g.StringFrom(proptest.CharsetAlphaNum+" ", g.IntRange(1, length))
	}
}

// GenerateSafeStringDefault generates a string default that's safe for all databases.
// Deprecated: Use GenerateSafeStringDefaultWithMaxLen for length-aware generation.
func GenerateSafeStringDefault(g *proptest.Generator) string {
	return GenerateSafeStringDefaultWithMaxLen(g, 50)
}

// GenerateEdgeCaseStringDefault generates string defaults that stress-test escaping.
func GenerateEdgeCaseStringDefault(g *proptest.Generator) string {
	edgeCases := []string{
		"",              // empty
		"'",             // single quote
		"''",            // escaped single quote
		"it's",          // apostrophe
		"say \"hello\"", // embedded double quotes
		"back\\slash",   // backslash
		"line1\nline2",  // newline
		"tab\there",     // tab
		"NULL",          // SQL keyword
		"日本語",           // Japanese
		"emoji 🎉",       // emoji (may not work everywhere)
	}
	return proptest.Pick(g, edgeCases)
}

// =============================================================================
// Column Definition Generator
// =============================================================================

// ColumnConfig controls random column generation
type ColumnConfig struct {
	AllowPrimaryKey bool
	AllowUnique     bool
	AllowNullable   bool
	AllowDefault    bool
	AllowIndex      bool
}

// DefaultColumnConfig returns a configuration that allows all options
func DefaultColumnConfig() ColumnConfig {
	return ColumnConfig{
		AllowPrimaryKey: true,
		AllowUnique:     true,
		AllowNullable:   true,
		AllowDefault:    true,
		AllowIndex:      true,
	}
}

// GenerateColumn generates a random column definition.
func GenerateColumn(g *proptest.Generator, name string, cfg ColumnConfig) ddl.ColumnDefinition {
	colType := GenerateColumnType(g)

	col := ddl.ColumnDefinition{
		Name:       name,
		Type:       colType,
		Nullable:   cfg.AllowNullable && g.BoolWithProb(0.3),
		PrimaryKey: false,
		Unique:     false,
		Index:      false,
	}

	// Set type-specific properties
	switch colType {
	case ddl.StringType:
		length := g.IntRange(10, 255)
		col.Length = &length
	case ddl.DecimalType:
		precision := g.IntRange(5, 18)
		scale := g.IntRange(0, precision-1)
		col.Precision = &precision
		col.Scale = &scale
	}

	// Primary key (mutually exclusive with nullable)
	if cfg.AllowPrimaryKey && !col.Nullable && g.BoolWithProb(0.1) {
		col.PrimaryKey = true
		col.Unique = true // Primary keys are implicitly unique
	}

	// Unique (if not primary key)
	if cfg.AllowUnique && !col.PrimaryKey && g.BoolWithProb(0.15) {
		col.Unique = true
	}

	// Index (non-unique)
	if cfg.AllowIndex && !col.Unique && !col.PrimaryKey && g.BoolWithProb(0.2) {
		col.Index = true
	}

	// Default value - must respect column length for string types
	if cfg.AllowDefault && g.BoolWithProb(0.3) {
		col.Default = GenerateDefaultForColumn(g, &col)
	}

	return col
}

// =============================================================================
// Table Generator
// =============================================================================

// TableConfig controls random table generation
type TableConfig struct {
	MinColumns     int
	MaxColumns     int
	AllowIndexes   bool
	RequirePrimary bool
}

// DefaultTableConfig returns sensible defaults for table generation
func DefaultTableConfig() TableConfig {
	return TableConfig{
		MinColumns:     1,
		MaxColumns:     8,
		AllowIndexes:   true,
		RequirePrimary: true,
	}
}

// GenerateTable generates a random table with columns.
func GenerateTable(g *proptest.Generator, tableName string, cfg TableConfig) *ddl.Table {
	numColumns := g.IntRange(cfg.MinColumns, cfg.MaxColumns)
	columnNames := GenerateUniqueColumnNames(g, numColumns)

	columns := make([]ddl.ColumnDefinition, numColumns)

	// If we require a primary key, make the first column the primary key
	hasPrimary := false
	for i, name := range columnNames {
		colCfg := DefaultColumnConfig()

		// Only allow one primary key
		if hasPrimary {
			colCfg.AllowPrimaryKey = false
		}

		// If we require primary and this is the first column, force it
		if cfg.RequirePrimary && i == 0 {
			columns[i] = ddl.ColumnDefinition{
				Name:       name,
				Type:       ddl.BigintType,
				Nullable:   false,
				PrimaryKey: true,
				Unique:     true,
			}
			hasPrimary = true
			continue
		}

		columns[i] = GenerateColumn(g, name, colCfg)
		if columns[i].PrimaryKey {
			hasPrimary = true
		}
	}

	// Generate indexes
	var indexes []ddl.IndexDefinition

	// Add indexes for columns marked as indexed or unique
	for _, col := range columns {
		if col.Unique && !col.PrimaryKey {
			indexes = append(indexes, ddl.IndexDefinition{
				Name:    ddl.GenerateIndexName(tableName, []string{col.Name}),
				Columns: []string{col.Name},
				Unique:  true,
			})
		} else if col.Index {
			indexes = append(indexes, ddl.IndexDefinition{
				Name:    ddl.GenerateIndexName(tableName, []string{col.Name}),
				Columns: []string{col.Name},
				Unique:  false,
			})
		}
	}

	// Add some composite indexes
	if cfg.AllowIndexes && numColumns >= 2 && g.BoolWithProb(0.3) {
		// Pick 2-3 columns for a composite index
		numIndexCols := g.IntRange(2, min(3, numColumns))
		indexCols := proptest.Sample(g, columnNames, numIndexCols)
		indexes = append(indexes, ddl.IndexDefinition{
			Name:    ddl.GenerateIndexName(tableName, indexCols),
			Columns: indexCols,
			Unique:  g.BoolWithProb(0.3),
		})
	}

	return &ddl.Table{
		Name:    tableName,
		Columns: columns,
		Indexes: indexes,
	}
}

// GenerateSimpleTable generates a simple table with basic columns (good for fuzzing).
func GenerateSimpleTable(g *proptest.Generator, tableName string) *ddl.Table {
	// 1-4 columns, no indexes, simple types
	numColumns := g.IntRange(1, 4)
	columnNames := GenerateUniqueColumnNames(g, numColumns)

	columns := make([]ddl.ColumnDefinition, numColumns)

	// First column is always a simple primary key
	columns[0] = ddl.ColumnDefinition{
		Name:       columnNames[0],
		Type:       ddl.IntegerType,
		Nullable:   false,
		PrimaryKey: true,
	}

	// Rest are simple columns
	simpleTypes := []string{ddl.IntegerType, ddl.StringType, ddl.BooleanType}
	for i := 1; i < numColumns; i++ {
		colType := proptest.Pick(g, simpleTypes)
		col := ddl.ColumnDefinition{
			Name:     columnNames[i],
			Type:     colType,
			Nullable: g.Bool(),
		}
		if colType == ddl.StringType {
			length := 255
			col.Length = &length
		}
		columns[i] = col
	}

	return &ddl.Table{
		Name:    tableName,
		Columns: columns,
		Indexes: []ddl.IndexDefinition{},
	}
}

// =============================================================================
// Alter Operation Generators
// =============================================================================

// GenerateAddColumnOp generates an ADD COLUMN operation.
func GenerateAddColumnOp(g *proptest.Generator) ddl.TableOperation {
	colName := GenerateColumnName(g)
	cfg := DefaultColumnConfig()
	cfg.AllowPrimaryKey = false // Can't add primary key columns in ALTER
	col := GenerateColumn(g, colName, cfg)

	return ddl.TableOperation{
		Type:      ddl.OpAddColumn,
		ColumnDef: &col,
	}
}

// GenerateDropColumnOp generates a DROP COLUMN operation for an existing column.
func GenerateDropColumnOp(g *proptest.Generator, existingColumns []string) ddl.TableOperation {
	if len(existingColumns) == 0 {
		panic("GenerateDropColumnOp: no existing columns")
	}
	return ddl.TableOperation{
		Type:   ddl.OpDropColumn,
		Column: proptest.Pick(g, existingColumns),
	}
}

// GenerateRenameColumnOp generates a RENAME COLUMN operation.
func GenerateRenameColumnOp(g *proptest.Generator, existingColumns []string) ddl.TableOperation {
	if len(existingColumns) == 0 {
		panic("GenerateRenameColumnOp: no existing columns")
	}
	return ddl.TableOperation{
		Type:    ddl.OpRenameColumn,
		Column:  proptest.Pick(g, existingColumns),
		NewName: GenerateColumnName(g),
	}
}

// GenerateSetNullableOp generates a SET NULL/NOT NULL operation.
func GenerateSetNullableOp(g *proptest.Generator, columnName string, makeNullable bool) ddl.TableOperation {
	return ddl.TableOperation{
		Type:     ddl.OpChangeNullable,
		Column:   columnName,
		Nullable: &makeNullable,
	}
}

// GenerateAddIndexOp generates an ADD INDEX operation.
func GenerateAddIndexOp(g *proptest.Generator, tableName string, columns []string) ddl.TableOperation {
	if len(columns) == 0 {
		panic("GenerateAddIndexOp: no columns")
	}

	// Pick 1-3 columns for the index
	numCols := g.IntRange(1, min(3, len(columns)))
	indexCols := proptest.Sample(g, columns, numCols)
	unique := g.BoolWithProb(0.3)

	return ddl.TableOperation{
		Type: ddl.OpAddIndex,
		IndexDef: &ddl.IndexDefinition{
			Name:    ddl.GenerateIndexName(tableName, indexCols),
			Columns: indexCols,
			Unique:  unique,
		},
	}
}

// GenerateDropIndexOp generates a DROP INDEX operation.
func GenerateDropIndexOp(indexName string) ddl.TableOperation {
	return ddl.TableOperation{
		Type:      ddl.OpDropIndex,
		IndexName: indexName,
	}
}

// =============================================================================
// Reserved Word Testing
// =============================================================================

// SQLReservedWords is a list of SQL reserved words that should be properly quoted
var SQLReservedWords = []string{
	"select", "from", "where", "table", "index",
	"create", "drop", "alter", "insert", "update",
	"delete", "and", "or", "not", "null", "true",
	"false", "is", "in", "like", "between", "join",
	"left", "right", "inner", "outer", "on", "as",
	"order", "by", "group", "having", "limit",
	"offset", "union", "all", "distinct", "case",
	"when", "then", "else", "end", "cast", "user",
	"key", "primary", "foreign", "references",
	"constraint", "unique", "check", "default",
}

// GenerateReservedWordIdentifier picks a reserved word to use as an identifier.
func GenerateReservedWordIdentifier(g *proptest.Generator) string {
	return proptest.Pick(g, SQLReservedWords)
}

// =============================================================================
// Utility Functions
// =============================================================================

// BuildTableFromDef builds a TableBuilder and returns the table.
func BuildTableFromDef(table *ddl.Table) (*ddl.TableBuilder, error) {
	tb := ddl.MakeEmptyTable(table.Name)

	for _, col := range table.Columns {
		if err := addColumnToBuilder(tb, col); err != nil {
			return nil, err
		}
	}

	return tb, nil
}

// addColumnToBuilder adds a column definition to a table builder
func addColumnToBuilder(tb *ddl.TableBuilder, col ddl.ColumnDefinition) error {
	switch col.Type {
	case ddl.IntegerType:
		b := tb.Integer(col.Name)
		if col.PrimaryKey {
			b.PrimaryKey()
		}
		if col.Nullable {
			b.Nullable()
		}
		if col.Default != nil {
			if v, err := strconv.ParseInt(*col.Default, 10, 64); err == nil {
				b.Default(v)
			}
		}

	case ddl.BigintType:
		b := tb.Bigint(col.Name)
		if col.PrimaryKey {
			b.PrimaryKey()
		}
		if col.Nullable {
			b.Nullable()
		}
		if col.Default != nil {
			if v, err := strconv.ParseInt(*col.Default, 10, 64); err == nil {
				b.Default(v)
			}
		}

	case ddl.StringType:
		var b *ddl.StringColumnBuilder
		if col.Length != nil {
			b = tb.VarChar(col.Name, *col.Length)
		} else {
			b = tb.String(col.Name)
		}
		if col.Nullable {
			b.Nullable()
		}
		if col.Unique {
			b.Unique()
		}
		if col.Default != nil {
			b.Default(*col.Default)
		}

	case ddl.TextType:
		b := tb.Text(col.Name)
		if col.Nullable {
			b.Nullable()
		}
		// Note: TEXT columns cannot have defaults (MySQL limitation)

	case ddl.BooleanType:
		b := tb.Bool(col.Name)
		if col.Nullable {
			b.Nullable()
		}
		if col.Default != nil {
			b.Default(*col.Default == "true")
		}

	case ddl.FloatType:
		b := tb.Float(col.Name)
		if col.Nullable {
			b.Nullable()
		}
		if col.Default != nil {
			if v, err := strconv.ParseFloat(*col.Default, 64); err == nil {
				b.Default(v)
			}
		}

	case ddl.DecimalType:
		precision := 10
		scale := 2
		if col.Precision != nil {
			precision = *col.Precision
		}
		if col.Scale != nil {
			scale = *col.Scale
		}
		b := tb.Decimal(col.Name, precision, scale)
		if col.Nullable {
			b.Nullable()
		}
		if col.Default != nil {
			b.Default(*col.Default)
		}

	case ddl.DatetimeType:
		b := tb.Datetime(col.Name)
		if col.Nullable {
			b.Nullable()
		}
		if col.Default != nil {
			b.Default(*col.Default)
		}

	case ddl.BinaryType:
		b := tb.Binary(col.Name)
		if col.Nullable {
			b.Nullable()
		}

	case ddl.JSONType:
		b := tb.JSON(col.Name)
		if col.Nullable {
			b.Nullable()
		}
		// Note: JSON columns cannot have defaults (MySQL limitation)

	default:
		return fmt.Errorf("unsupported column type: %s", col.Type)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
