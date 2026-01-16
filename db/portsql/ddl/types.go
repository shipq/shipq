package ddl

import (
	"encoding/json"
	"strings"
)

// Type constants for ActiveRecord-style column types
const (
	IntegerType   = "integer"
	BigintType    = "bigint"
	DecimalType   = "decimal"
	FloatType     = "float"
	BooleanType   = "boolean"
	StringType    = "string"
	TextType      = "text"
	DatetimeType  = "datetime"
	TimestampType = "timestamp"
	BinaryType    = "binary"
	JSONType      = "json"
)

// ColumnDefinition represents a column in a database table.
type ColumnDefinition struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Length     *int    `json:"length"`
	Precision  *int    `json:"precision"`
	Scale      *int    `json:"scale"`
	Nullable   bool    `json:"nullable"`
	Default    *string `json:"default"` // nil = no default, &"" = empty string default
	Unique     bool    `json:"unique"`
	PrimaryKey bool    `json:"primary_key"`
	Index      bool    `json:"index"`
	ForeignKey string  `json:"foreign_key"`
}

// IndexDefinition represents an index on a database table.
type IndexDefinition struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

// Table represents a database table with its columns and indexes.
type Table struct {
	Name    string             `json:"name"`
	Columns []ColumnDefinition `json:"columns"`
	Indexes []IndexDefinition  `json:"indexes"`
}

// Serialize serializes the table to a JSON string.
func (t *Table) Serialize() (string, error) {
	jsonBytes, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// ColumnRef is a type-safe reference to a column, used for building indexes.
type ColumnRef struct {
	name string
}

// GenerateIndexName creates an index name from table and column names.
func GenerateIndexName(tableName string, columns []string) string {
	return "idx_" + tableName + "_" + strings.Join(columns, "_")
}

// --- Operation Types for ALTER TABLE ---

// OperationType represents the type of table alteration operation.
type OperationType string

const (
	OpAddColumn      OperationType = "add_column"
	OpDropColumn     OperationType = "drop_column"
	OpRenameColumn   OperationType = "rename_column"
	OpChangeType     OperationType = "change_type"
	OpChangeNullable OperationType = "change_nullable"
	OpChangeDefault  OperationType = "change_default"
	OpAddIndex       OperationType = "add_index"
	OpDropIndex      OperationType = "drop_index"
	OpRenameIndex    OperationType = "rename_index"
)

// TableOperation represents a single alteration operation on a table.
type TableOperation struct {
	Type      OperationType     `json:"type"`
	Column    string            `json:"column,omitempty"`
	NewName   string            `json:"new_name,omitempty"`
	ColumnDef *ColumnDefinition `json:"column_def,omitempty"`
	IndexDef  *IndexDefinition  `json:"index_def,omitempty"`
	IndexName string            `json:"index_name,omitempty"`
	NewType   string            `json:"new_type,omitempty"`
	Nullable  *bool             `json:"nullable,omitempty"`
	Default   *string           `json:"default,omitempty"`
}
