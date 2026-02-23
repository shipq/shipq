package ddl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// AlterTableBuilder captures table alteration operations for migrations.
type AlterTableBuilder struct {
	tableName     string
	existingTable *Table // optional, for column validation with ExistingColumn
	operations    []TableOperation
}

// AlterTable creates a new AlterTableBuilder for the specified table.
func AlterTable(name string) *AlterTableBuilder {
	return &AlterTableBuilder{
		tableName:     name,
		existingTable: nil,
		operations:    []TableOperation{},
	}
}

// AlterTableFrom creates a new AlterTableBuilder with access to the existing table.
// This enables type-safe column references via ExistingColumn.
func AlterTableFrom(table *Table) *AlterTableBuilder {
	return &AlterTableBuilder{
		tableName:     table.Name,
		existingTable: table,
		operations:    []TableOperation{},
	}
}

// Build returns the list of operations to be performed.
func (ab *AlterTableBuilder) Build() []TableOperation {
	return ab.operations
}

// TableName returns the name of the table being altered.
func (ab *AlterTableBuilder) TableName() string {
	return ab.tableName
}

// Serialize serializes the operations to a JSON string.
func (ab *AlterTableBuilder) Serialize() (string, error) {
	jsonBytes, err := json.Marshal(ab.operations)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// ExistingColumn returns a type-safe ColumnRef for an existing column.
// Returns an error if the column doesn't exist or if no existing table was provided.
func (ab *AlterTableBuilder) ExistingColumn(name string) (ColumnRef, error) {
	if ab.existingTable == nil {
		return ColumnRef{}, errors.New("no existing table provided; use AlterTableFrom to enable ExistingColumn")
	}
	for _, col := range ab.existingTable.Columns {
		if col.Name == name {
			return ColumnRef{name: name}, nil
		}
	}
	return ColumnRef{}, fmt.Errorf("column %q not found in table %q", name, ab.tableName)
}

// --- Basic Alteration Methods ---

// RenameColumn adds an operation to rename a column.
func (ab *AlterTableBuilder) RenameColumn(oldName, newName string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:    OpRenameColumn,
		Column:  oldName,
		NewName: newName,
	})
}

// DropColumn adds an operation to drop a column.
func (ab *AlterTableBuilder) DropColumn(name string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:   OpDropColumn,
		Column: name,
	})
}

// DropIndex adds an operation to drop an index.
func (ab *AlterTableBuilder) DropIndex(name string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:      OpDropIndex,
		IndexName: name,
	})
}

// RenameIndex adds an operation to rename an index.
func (ab *AlterTableBuilder) RenameIndex(oldName, newName string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:      OpRenameIndex,
		IndexName: oldName,
		NewName:   newName,
	})
}

// --- Column Modification Methods ---

// ChangeType adds an operation to change a column's type.
func (ab *AlterTableBuilder) ChangeType(column, newType string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:    OpChangeType,
		Column:  column,
		NewType: newType,
	})
}

// SetNullable adds an operation to make a column nullable.
func (ab *AlterTableBuilder) SetNullable(column string) {
	nullable := true
	ab.operations = append(ab.operations, TableOperation{
		Type:     OpChangeNullable,
		Column:   column,
		Nullable: &nullable,
	})
}

// SetNotNull adds an operation to make a column NOT NULL.
func (ab *AlterTableBuilder) SetNotNull(column string) {
	nullable := false
	ab.operations = append(ab.operations, TableOperation{
		Type:     OpChangeNullable,
		Column:   column,
		Nullable: &nullable,
	})
}

// SetDefault adds an operation to set a column's default value.
func (ab *AlterTableBuilder) SetDefault(column, value string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:    OpChangeDefault,
		Column:  column,
		Default: &value,
	})
}

// DropDefault adds an operation to remove a column's default value.
func (ab *AlterTableBuilder) DropDefault(column string) {
	ab.operations = append(ab.operations, TableOperation{
		Type:    OpChangeDefault,
		Column:  column,
		Default: nil,
	})
}

// --- Type-Safe *Ref Method Variants ---
// These methods accept ColumnRef instead of strings for type safety.

// RenameColumnRef adds an operation to rename a column using a type-safe reference.
func (ab *AlterTableBuilder) RenameColumnRef(col ColumnRef, newName string) {
	ab.RenameColumn(col.name, newName)
}

// DropColumnRef adds an operation to drop a column using a type-safe reference.
func (ab *AlterTableBuilder) DropColumnRef(col ColumnRef) {
	ab.DropColumn(col.name)
}

// ChangeTypeRef adds an operation to change a column's type using a type-safe reference.
func (ab *AlterTableBuilder) ChangeTypeRef(col ColumnRef, newType string) {
	ab.ChangeType(col.name, newType)
}

// SetNullableRef adds an operation to make a column nullable using a type-safe reference.
func (ab *AlterTableBuilder) SetNullableRef(col ColumnRef) {
	ab.SetNullable(col.name)
}

// SetNotNullRef adds an operation to make a column NOT NULL using a type-safe reference.
func (ab *AlterTableBuilder) SetNotNullRef(col ColumnRef) {
	ab.SetNotNull(col.name)
}

// SetDefaultRef adds an operation to set a column's default value using a type-safe reference.
func (ab *AlterTableBuilder) SetDefaultRef(col ColumnRef, value string) {
	ab.SetDefault(col.name, value)
}

// DropDefaultRef adds an operation to remove a column's default value using a type-safe reference.
func (ab *AlterTableBuilder) DropDefaultRef(col ColumnRef) {
	ab.DropDefault(col.name)
}

// --- Index Methods ---

// AddIndex adds a composite index on the specified columns.
func (ab *AlterTableBuilder) AddIndex(cols ...ColumnRef) {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	ab.operations = append(ab.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(ab.tableName, names),
			Columns: names,
			Unique:  false,
		},
	})
}

// AddUniqueIndex adds a unique composite index on the specified columns.
func (ab *AlterTableBuilder) AddUniqueIndex(cols ...ColumnRef) {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	ab.operations = append(ab.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(ab.tableName, names),
			Columns: names,
			Unique:  true,
		},
	})
}

// --- Column Creation Methods (Add Column) ---

// Alter*ColumnBuilder types for adding columns during alterations.
// They store a pointer to their operation so modifiers can update it.

type AlterIntColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterBoolColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterStringColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterFloatColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterDecimalColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterTimeColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterBinaryColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterJSONColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

type AlterTextColumnBuilder struct {
	alterBuilder *AlterTableBuilder
	op           *TableOperation
}

// Integer adds an integer column.
func (ab *AlterTableBuilder) Integer(name string) *AlterIntColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       IntegerType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterIntColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Bigint adds a bigint (64-bit integer) column.
func (ab *AlterTableBuilder) Bigint(name string) *AlterIntColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       BigintType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterIntColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Decimal adds a decimal column with specified precision and scale.
func (ab *AlterTableBuilder) Decimal(name string, precision int, scale int) *AlterDecimalColumnBuilder {
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
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterDecimalColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Float adds a float column.
func (ab *AlterTableBuilder) Float(name string) *AlterFloatColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       FloatType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterFloatColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Bool adds a boolean column.
func (ab *AlterTableBuilder) Bool(name string) *AlterBoolColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       BooleanType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterBoolColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// String adds a string column with VARCHAR(255).
func (ab *AlterTableBuilder) String(name string) *AlterStringColumnBuilder {
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
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterStringColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// VarChar adds a string column with specified length.
func (ab *AlterTableBuilder) VarChar(name string, length int) *AlterStringColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       StringType,
		Length:     &length,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterStringColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Text adds an unlimited text column.
func (ab *AlterTableBuilder) Text(name string) *AlterTextColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       TextType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterTextColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Datetime adds a datetime column.
func (ab *AlterTableBuilder) Datetime(name string) *AlterTimeColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       DatetimeType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterTimeColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Timestamp adds a timestamp column.
func (ab *AlterTableBuilder) Timestamp(name string) *AlterTimeColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       TimestampType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterTimeColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// Binary adds a binary/blob column.
func (ab *AlterTableBuilder) Binary(name string) *AlterBinaryColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       BinaryType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterBinaryColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// JSON adds a JSON column.
func (ab *AlterTableBuilder) JSON(name string) *AlterJSONColumnBuilder {
	col := ColumnDefinition{
		Name:       name,
		Type:       JSONType,
		Nullable:   false,
		Unique:     false,
		PrimaryKey: false,
		Index:      false,
	}
	op := TableOperation{
		Type:      OpAddColumn,
		ColumnDef: &col,
	}
	ab.operations = append(ab.operations, op)
	return &AlterJSONColumnBuilder{
		alterBuilder: ab,
		op:           &ab.operations[len(ab.operations)-1],
	}
}

// --- AlterIntColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterIntColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// PrimaryKey marks the column as a primary key.
func (b *AlterIntColumnBuilder) PrimaryKey() *AlterIntColumnBuilder {
	b.op.ColumnDef.PrimaryKey = true
	return b
}

// Nullable marks the column as nullable.
func (b *AlterIntColumnBuilder) Nullable() *AlterIntColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Unique marks the column as unique and adds a unique index operation.
func (b *AlterIntColumnBuilder) Unique() *AlterIntColumnBuilder {
	b.op.ColumnDef.Unique = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  true,
		},
	})
	return b
}

// Indexed adds a non-unique index operation on this column.
func (b *AlterIntColumnBuilder) Indexed() *AlterIntColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Default sets the default value for an integer column.
func (b *AlterIntColumnBuilder) Default(v int64) *AlterIntColumnBuilder {
	s := strconv.FormatInt(v, 10)
	b.op.ColumnDef.Default = &s
	return b
}

// --- AlterBoolColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterBoolColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterBoolColumnBuilder) Nullable() *AlterBoolColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Indexed adds a non-unique index operation on this column.
func (b *AlterBoolColumnBuilder) Indexed() *AlterBoolColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Default sets the default value for a boolean column.
func (b *AlterBoolColumnBuilder) Default(v bool) *AlterBoolColumnBuilder {
	s := strconv.FormatBool(v)
	b.op.ColumnDef.Default = &s
	return b
}

// --- AlterStringColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterStringColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterStringColumnBuilder) Nullable() *AlterStringColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Unique marks the column as unique and adds a unique index operation.
func (b *AlterStringColumnBuilder) Unique() *AlterStringColumnBuilder {
	b.op.ColumnDef.Unique = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  true,
		},
	})
	return b
}

// Indexed adds a non-unique index operation on this column.
func (b *AlterStringColumnBuilder) Indexed() *AlterStringColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Default sets the default value for a string column.
func (b *AlterStringColumnBuilder) Default(v string) *AlterStringColumnBuilder {
	b.op.ColumnDef.Default = &v
	return b
}

// --- AlterFloatColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterFloatColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterFloatColumnBuilder) Nullable() *AlterFloatColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Indexed adds a non-unique index operation on this column.
func (b *AlterFloatColumnBuilder) Indexed() *AlterFloatColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Default sets the default value for a float column.
func (b *AlterFloatColumnBuilder) Default(v float64) *AlterFloatColumnBuilder {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	b.op.ColumnDef.Default = &s
	return b
}

// --- AlterDecimalColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterDecimalColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterDecimalColumnBuilder) Nullable() *AlterDecimalColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Indexed adds a non-unique index operation on this column.
func (b *AlterDecimalColumnBuilder) Indexed() *AlterDecimalColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Default sets the default value for a decimal column.
func (b *AlterDecimalColumnBuilder) Default(v string) *AlterDecimalColumnBuilder {
	b.op.ColumnDef.Default = &v
	return b
}

// --- AlterTimeColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterTimeColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterTimeColumnBuilder) Nullable() *AlterTimeColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Indexed adds a non-unique index operation on this column.
func (b *AlterTimeColumnBuilder) Indexed() *AlterTimeColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Default sets the default value for a time column.
func (b *AlterTimeColumnBuilder) Default(v string) *AlterTimeColumnBuilder {
	b.op.ColumnDef.Default = &v
	return b
}

// --- AlterBinaryColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterBinaryColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterBinaryColumnBuilder) Nullable() *AlterBinaryColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Note: Binary columns cannot have DEFAULT values in MySQL (BLOB columns).
// For cross-database compatibility, Default() is intentionally not provided.

// --- AlterJSONColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterJSONColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterJSONColumnBuilder) Nullable() *AlterJSONColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Note: JSON columns cannot have DEFAULT values in MySQL.
// For cross-database compatibility, Default() is intentionally not provided.

// --- AlterTextColumnBuilder Methods ---

// Col returns a type-safe column reference for use in index definitions.
func (b *AlterTextColumnBuilder) Col() ColumnRef {
	return ColumnRef{name: b.op.ColumnDef.Name}
}

// Nullable marks the column as nullable.
func (b *AlterTextColumnBuilder) Nullable() *AlterTextColumnBuilder {
	b.op.ColumnDef.Nullable = true
	return b
}

// Unique adds a unique constraint to the column.
func (b *AlterTextColumnBuilder) Unique() *AlterTextColumnBuilder {
	b.op.ColumnDef.Unique = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  true,
		},
	})
	return b
}

// Indexed adds an index to the column.
func (b *AlterTextColumnBuilder) Indexed() *AlterTextColumnBuilder {
	b.op.ColumnDef.Index = true
	b.alterBuilder.operations = append(b.alterBuilder.operations, TableOperation{
		Type: OpAddIndex,
		IndexDef: &IndexDefinition{
			Name:    GenerateIndexName(b.alterBuilder.tableName, []string{b.op.ColumnDef.Name}),
			Columns: []string{b.op.ColumnDef.Name},
			Unique:  false,
		},
	})
	return b
}

// Note: TEXT columns cannot have DEFAULT values in MySQL.
// For cross-database compatibility, Default() is intentionally not provided.
