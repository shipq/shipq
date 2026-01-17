package ddl

import (
	"strconv"

	"github.com/shipq/shipq/db/portsql/ref"
)

// TableBuilder owns the table and provides methods to add columns and indexes.
type TableBuilder struct {
	table *Table
}

// Type-specific column builders that hold a reference to their specific column
// and the parent TableBuilder.
type IntColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type BoolColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type StringColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type FloatColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type DecimalColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type TimeColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type BinaryColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type JSONColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

type TextColumnBuilder struct {
	tableBuilder *TableBuilder
	col          *ColumnDefinition
}

// MakeEmptyTable constructs a new table with no columns.
func MakeEmptyTable(name string) *TableBuilder {
	return &TableBuilder{
		table: &Table{
			Name:    name,
			Columns: []ColumnDefinition{},
			Indexes: []IndexDefinition{},
		},
	}
}

// MakeTable constructs a new table with the default columns: id, public_id, created_at,
// deleted_at, updated_at.
func MakeTable(name string) *TableBuilder {
	return &TableBuilder{
		table: &Table{
			Name: name,
			Columns: []ColumnDefinition{
				{
					Name:       "id",
					Type:       BigintType,
					PrimaryKey: true,
					Nullable:   false,
					Unique:     true,
					Index:      true,
					ForeignKey: "",
				},
				{
					Name:       "public_id",
					Type:       StringType,
					Precision:  nil,
					Scale:      nil,
					Nullable:   false,
					Unique:     true,
					Index:      true,
					ForeignKey: "",
				},
				{
					Name:       "created_at",
					Type:       DatetimeType,
					Nullable:   false,
					Unique:     false,
					Index:      false,
					ForeignKey: "",
				},
				{
					Name:       "deleted_at",
					Type:       DatetimeType,
					Nullable:   false,
					Unique:     false,
					Index:      false,
					ForeignKey: "",
				},
				{
					Name:       "updated_at",
					Type:       DatetimeType,
					Nullable:   false,
					Unique:     false,
					Index:      false,
					ForeignKey: "",
				},
			},
			Indexes: []IndexDefinition{
				{
					Name:    GenerateIndexName(name, []string{"id"}),
					Columns: []string{"id"},
					Unique:  true,
				},
				{
					Name:    GenerateIndexName(name, []string{"public_id"}),
					Columns: []string{"public_id"},
					Unique:  true,
				},
			},
		},
	}
}

// Build returns the constructed table.
func (tb *TableBuilder) Build() *Table {
	return tb.table
}

// JunctionTable marks this table as a many-to-many junction table.
// Junction tables should have exactly 2 columns with References set.
func (tb *TableBuilder) JunctionTable() *TableBuilder {
	tb.table.IsJunctionTable = true
	return tb
}

// AddIndex adds a composite index on the specified columns.
func (tb *TableBuilder) AddIndex(cols ...ColumnRef) *TableBuilder {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	tb.table.Indexes = append(tb.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(tb.table.Name, names),
		Columns: names,
		Unique:  false,
	})
	return tb
}

// AddUniqueIndex adds a unique composite index on the specified columns.
func (tb *TableBuilder) AddUniqueIndex(cols ...ColumnRef) *TableBuilder {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	tb.table.Indexes = append(tb.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(tb.table.Name, names),
		Columns: names,
		Unique:  true,
	})
	return tb
}

// --- Column Type Methods on TableBuilder ---

// Integer adds an integer column.
func (tb *TableBuilder) Integer(name string) *IntColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       IntegerType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &IntColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Bigint adds a bigint (64-bit integer) column.
func (tb *TableBuilder) Bigint(name string) *IntColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       BigintType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &IntColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Decimal adds a decimal column with specified precision and scale.
func (tb *TableBuilder) Decimal(name string, precision int, scale int) *DecimalColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       DecimalType,
		Precision:  &precision,
		Scale:      &scale,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &DecimalColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Float adds a float column.
func (tb *TableBuilder) Float(name string) *FloatColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       FloatType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &FloatColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Bool adds a boolean column.
func (tb *TableBuilder) Bool(name string) *BoolColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       BooleanType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &BoolColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// String adds a string column with VARCHAR(255).
func (tb *TableBuilder) String(name string) *StringColumnBuilder {
	length := 255
	col := ColumnDefinition{
		Name:       name,
		Type:       StringType,
		Length:     &length,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &StringColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Varchar adds a string column with specified length.
func (tb *TableBuilder) Varchar(name string, length int) *StringColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       StringType,
		Length:     &length,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &StringColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Text adds an unlimited text column.
func (tb *TableBuilder) Text(name string) *TextColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       TextType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &TextColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Datetime adds a datetime column (with timezone).
func (tb *TableBuilder) Datetime(name string) *TimeColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       DatetimeType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &TimeColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Timestamp adds a timestamp column (with timezone).
func (tb *TableBuilder) Timestamp(name string) *TimeColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       TimestampType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &TimeColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// Binary adds a binary/blob column.
func (tb *TableBuilder) Binary(name string) *BinaryColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       BinaryType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &BinaryColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// JSON adds a JSON column.
func (tb *TableBuilder) JSON(name string) *JSONColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       JSONType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	tb.table.Columns = append(tb.table.Columns, col)
	return &JSONColumnBuilder{
		tableBuilder: tb,
		col:          &tb.table.Columns[len(tb.table.Columns)-1],
	}
}

// --- IntColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *IntColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// PrimaryKey marks the column as a primary key.
func (b *IntColumnBuilder) PrimaryKey() *IntColumnBuilder {
	b.col.PrimaryKey = true
	return b
}

// Nullable marks the column as nullable.
func (b *IntColumnBuilder) Nullable() *IntColumnBuilder {
	b.col.Nullable = true
	return b
}

// Unique marks the column as unique and adds a unique index.
func (b *IntColumnBuilder) Unique() *IntColumnBuilder {
	b.col.Unique = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  true,
	})
	return b
}

// Indexed adds a non-unique index on this column.
func (b *IntColumnBuilder) Indexed() *IntColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Default sets the default value for an integer column.
func (b *IntColumnBuilder) Default(v int64) *IntColumnBuilder {
	s := strconv.FormatInt(v, 10)
	b.col.Default = &s
	return b
}

// References marks this column as referencing another table.
// This is metadata for automatic relation code generation - no actual FK constraint is created.
func (b *IntColumnBuilder) References(tableRef *ref.TableRef) *IntColumnBuilder {
	b.col.References = tableRef.TableName()
	return b
}

// --- BoolColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *BoolColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *BoolColumnBuilder) Nullable() *BoolColumnBuilder {
	b.col.Nullable = true
	return b
}

// Indexed adds a non-unique index on this column.
func (b *BoolColumnBuilder) Indexed() *BoolColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Default sets the default value for a boolean column.
func (b *BoolColumnBuilder) Default(v bool) *BoolColumnBuilder {
	s := strconv.FormatBool(v)
	b.col.Default = &s
	return b
}

// --- StringColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *StringColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *StringColumnBuilder) Nullable() *StringColumnBuilder {
	b.col.Nullable = true
	return b
}

// Unique marks the column as unique and adds a unique index.
func (b *StringColumnBuilder) Unique() *StringColumnBuilder {
	b.col.Unique = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  true,
	})
	return b
}

// Indexed adds a non-unique index on this column.
func (b *StringColumnBuilder) Indexed() *StringColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Default sets the default value for a string column.
func (b *StringColumnBuilder) Default(v string) *StringColumnBuilder {
	b.col.Default = &v
	return b
}

// --- FloatColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *FloatColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *FloatColumnBuilder) Nullable() *FloatColumnBuilder {
	b.col.Nullable = true
	return b
}

// Indexed adds a non-unique index on this column.
func (b *FloatColumnBuilder) Indexed() *FloatColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Default sets the default value for a float column.
func (b *FloatColumnBuilder) Default(v float64) *FloatColumnBuilder {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	b.col.Default = &s
	return b
}

// --- DecimalColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *DecimalColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *DecimalColumnBuilder) Nullable() *DecimalColumnBuilder {
	b.col.Nullable = true
	return b
}

// Indexed adds a non-unique index on this column.
func (b *DecimalColumnBuilder) Indexed() *DecimalColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Default sets the default value for a decimal column.
func (b *DecimalColumnBuilder) Default(v string) *DecimalColumnBuilder {
	b.col.Default = &v
	return b
}

// --- TimeColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *TimeColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *TimeColumnBuilder) Nullable() *TimeColumnBuilder {
	b.col.Nullable = true
	return b
}

// Indexed adds a non-unique index on this column.
func (b *TimeColumnBuilder) Indexed() *TimeColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Default sets the default value for a time column (datetime/timestamp).
func (b *TimeColumnBuilder) Default(v string) *TimeColumnBuilder {
	b.col.Default = &v
	return b
}

// --- BinaryColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *BinaryColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *BinaryColumnBuilder) Nullable() *BinaryColumnBuilder {
	b.col.Nullable = true
	return b
}

// Note: Binary columns cannot have DEFAULT values in MySQL (BLOB columns).
// For cross-database compatibility, Default() is intentionally not provided.

// --- JSONColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *JSONColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *JSONColumnBuilder) Nullable() *JSONColumnBuilder {
	b.col.Nullable = true
	return b
}

// Note: JSON columns cannot have DEFAULT values in MySQL.
// For cross-database compatibility, Default() is intentionally not provided.

// --- TextColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *TextColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.col.Name}
}

// Nullable marks the column as nullable.
func (b *TextColumnBuilder) Nullable() *TextColumnBuilder {
	b.col.Nullable = true
	return b
}

// Unique adds a unique constraint to the column.
func (b *TextColumnBuilder) Unique() *TextColumnBuilder {
	b.col.Unique = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  true,
	})
	return b
}

// Indexed adds an index to the column.
func (b *TextColumnBuilder) Indexed() *TextColumnBuilder {
	b.col.Index = true
	b.tableBuilder.table.Indexes = append(b.tableBuilder.table.Indexes, IndexDefinition{
		Name:    GenerateIndexName(b.tableBuilder.table.Name, []string{b.col.Name}),
		Columns: []string{b.col.Name},
		Unique:  false,
	})
	return b
}

// Note: TEXT columns cannot have DEFAULT values in MySQL.
// For cross-database compatibility, Default() is intentionally not provided.
