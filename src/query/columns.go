package query

// Column is the base interface for all column types.
// Each column carries its table name, column name, and type information
// to enable type-safe query building.
type Column interface {
	TableName() string
	ColumnName() string
	IsNullable() bool
	GoType() string // "int64", "string", "*string", etc.
}

// --- Int32 Columns (for integer type) ---

// Int32Column represents a non-nullable integer column.
type Int32Column struct {
	Table string
	Name  string
}

func (c Int32Column) TableName() string  { return c.Table }
func (c Int32Column) ColumnName() string { return c.Name }
func (c Int32Column) IsNullable() bool   { return false }
func (c Int32Column) GoType() string     { return "int32" }

// NullInt32Column represents a nullable integer column.
type NullInt32Column struct {
	Table string
	Name  string
}

func (c NullInt32Column) TableName() string  { return c.Table }
func (c NullInt32Column) ColumnName() string { return c.Name }
func (c NullInt32Column) IsNullable() bool   { return true }
func (c NullInt32Column) GoType() string     { return "*int32" }

// --- Int64 Columns (for bigint type) ---

// Int64Column represents a non-nullable bigint column.
type Int64Column struct {
	Table string
	Name  string
}

func (c Int64Column) TableName() string  { return c.Table }
func (c Int64Column) ColumnName() string { return c.Name }
func (c Int64Column) IsNullable() bool   { return false }
func (c Int64Column) GoType() string     { return "int64" }

// NullInt64Column represents a nullable bigint column.
type NullInt64Column struct {
	Table string
	Name  string
}

func (c NullInt64Column) TableName() string  { return c.Table }
func (c NullInt64Column) ColumnName() string { return c.Name }
func (c NullInt64Column) IsNullable() bool   { return true }
func (c NullInt64Column) GoType() string     { return "*int64" }

// --- Float64 Columns (for float type) ---

// Float64Column represents a non-nullable float column.
type Float64Column struct {
	Table string
	Name  string
}

func (c Float64Column) TableName() string  { return c.Table }
func (c Float64Column) ColumnName() string { return c.Name }
func (c Float64Column) IsNullable() bool   { return false }
func (c Float64Column) GoType() string     { return "float64" }

// NullFloat64Column represents a nullable float column.
type NullFloat64Column struct {
	Table string
	Name  string
}

func (c NullFloat64Column) TableName() string  { return c.Table }
func (c NullFloat64Column) ColumnName() string { return c.Name }
func (c NullFloat64Column) IsNullable() bool   { return true }
func (c NullFloat64Column) GoType() string     { return "*float64" }

// --- Decimal Columns (for decimal type - stored as string for precision) ---

// DecimalColumn represents a non-nullable decimal column.
// Decimals are represented as strings in Go to preserve precision.
type DecimalColumn struct {
	Table string
	Name  string
}

func (c DecimalColumn) TableName() string  { return c.Table }
func (c DecimalColumn) ColumnName() string { return c.Name }
func (c DecimalColumn) IsNullable() bool   { return false }
func (c DecimalColumn) GoType() string     { return "string" }

// NullDecimalColumn represents a nullable decimal column.
type NullDecimalColumn struct {
	Table string
	Name  string
}

func (c NullDecimalColumn) TableName() string  { return c.Table }
func (c NullDecimalColumn) ColumnName() string { return c.Name }
func (c NullDecimalColumn) IsNullable() bool   { return true }
func (c NullDecimalColumn) GoType() string     { return "*string" }

// --- Bool Columns (for boolean type) ---

// BoolColumn represents a non-nullable boolean column.
type BoolColumn struct {
	Table string
	Name  string
}

func (c BoolColumn) TableName() string  { return c.Table }
func (c BoolColumn) ColumnName() string { return c.Name }
func (c BoolColumn) IsNullable() bool   { return false }
func (c BoolColumn) GoType() string     { return "bool" }

// NullBoolColumn represents a nullable boolean column.
type NullBoolColumn struct {
	Table string
	Name  string
}

func (c NullBoolColumn) TableName() string  { return c.Table }
func (c NullBoolColumn) ColumnName() string { return c.Name }
func (c NullBoolColumn) IsNullable() bool   { return true }
func (c NullBoolColumn) GoType() string     { return "*bool" }

// --- String Columns (for string and text types) ---

// StringColumn represents a non-nullable string/text column.
type StringColumn struct {
	Table string
	Name  string
}

func (c StringColumn) TableName() string  { return c.Table }
func (c StringColumn) ColumnName() string { return c.Name }
func (c StringColumn) IsNullable() bool   { return false }
func (c StringColumn) GoType() string     { return "string" }

// NullStringColumn represents a nullable string/text column.
type NullStringColumn struct {
	Table string
	Name  string
}

func (c NullStringColumn) TableName() string  { return c.Table }
func (c NullStringColumn) ColumnName() string { return c.Name }
func (c NullStringColumn) IsNullable() bool   { return true }
func (c NullStringColumn) GoType() string     { return "*string" }

// --- Time Columns (for datetime and timestamp types) ---

// TimeColumn represents a non-nullable datetime/timestamp column.
type TimeColumn struct {
	Table string
	Name  string
}

func (c TimeColumn) TableName() string  { return c.Table }
func (c TimeColumn) ColumnName() string { return c.Name }
func (c TimeColumn) IsNullable() bool   { return false }
func (c TimeColumn) GoType() string     { return "time.Time" }

// NullTimeColumn represents a nullable datetime/timestamp column.
type NullTimeColumn struct {
	Table string
	Name  string
}

func (c NullTimeColumn) TableName() string  { return c.Table }
func (c NullTimeColumn) ColumnName() string { return c.Name }
func (c NullTimeColumn) IsNullable() bool   { return true }
func (c NullTimeColumn) GoType() string     { return "*time.Time" }

// --- Bytes Column (for binary type) ---

// BytesColumn represents a binary column.
// Binary columns are always represented as []byte (not nullable in the pointer sense).
type BytesColumn struct {
	Table string
	Name  string
}

func (c BytesColumn) TableName() string  { return c.Table }
func (c BytesColumn) ColumnName() string { return c.Name }
func (c BytesColumn) IsNullable() bool   { return false }
func (c BytesColumn) GoType() string     { return "[]byte" }

// --- JSON Columns (for json type) ---

// JSONColumn represents a non-nullable JSON column.
type JSONColumn struct {
	Table string
	Name  string
}

func (c JSONColumn) TableName() string  { return c.Table }
func (c JSONColumn) ColumnName() string { return c.Name }
func (c JSONColumn) IsNullable() bool   { return false }
func (c JSONColumn) GoType() string     { return "json.RawMessage" }

// NullJSONColumn represents a nullable JSON column.
type NullJSONColumn struct {
	Table string
	Name  string
}

func (c NullJSONColumn) TableName() string  { return c.Table }
func (c NullJSONColumn) ColumnName() string { return c.Name }
func (c NullJSONColumn) IsNullable() bool   { return true }
func (c NullJSONColumn) GoType() string     { return "json.RawMessage" }

// Compile-time verification that all column types implement Column interface
var (
	_ Column = Int32Column{}
	_ Column = NullInt32Column{}
	_ Column = Int64Column{}
	_ Column = NullInt64Column{}
	_ Column = Float64Column{}
	_ Column = NullFloat64Column{}
	_ Column = DecimalColumn{}
	_ Column = NullDecimalColumn{}
	_ Column = BoolColumn{}
	_ Column = NullBoolColumn{}
	_ Column = StringColumn{}
	_ Column = NullStringColumn{}
	_ Column = TimeColumn{}
	_ Column = NullTimeColumn{}
	_ Column = BytesColumn{}
	_ Column = JSONColumn{}
	_ Column = NullJSONColumn{}
)
