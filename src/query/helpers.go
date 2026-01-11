package query

import "time"

// Param creates a typed parameter reference.
// The type parameter T is used to infer the Go type for codegen.
func Param[T any](name string) ParamExpr {
	var zero T
	return ParamExpr{
		Name:   name,
		GoType: typeNameOf(zero),
	}
}

// Literal creates a literal value expression.
func Literal[T any](value T) LiteralExpr {
	return LiteralExpr{Value: value}
}

// Now represents the current timestamp (translated per-database).
func Now() FuncExpr {
	return FuncExpr{Name: "NOW", Args: nil}
}

// And combines expressions with AND.
// Returns nil if no expressions are provided.
// Returns the single expression if only one is provided.
func And(exprs ...Expr) Expr {
	if len(exprs) == 0 {
		return nil
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	result := exprs[0]
	for _, expr := range exprs[1:] {
		result = BinaryExpr{Left: result, Op: OpAnd, Right: expr}
	}
	return result
}

// Or combines expressions with OR.
// Returns nil if no expressions are provided.
// Returns the single expression if only one is provided.
func Or(exprs ...Expr) Expr {
	if len(exprs) == 0 {
		return nil
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	result := exprs[0]
	for _, expr := range exprs[1:] {
		result = BinaryExpr{Left: result, Op: OpOr, Right: expr}
	}
	return result
}

// Not negates an expression.
func Not(expr Expr) Expr {
	return UnaryExpr{Op: OpNot, Expr: expr}
}

// toExpr converts any value to an Expr.
// If the value is already an Expr, it's returned as-is.
// If it's a Column, it's wrapped in ColumnExpr.
// Otherwise, it's wrapped in LiteralExpr.
func toExpr(v any) Expr {
	switch val := v.(type) {
	case Expr:
		return val
	case Column:
		return ColumnExpr{Column: val}
	default:
		return LiteralExpr{Value: val}
	}
}

// typeNameOf returns the Go type name for a value.
// Used for parameter type tracking in codegen.
func typeNameOf(v any) string {
	switch v.(type) {
	case int:
		return "int"
	case int8:
		return "int8"
	case int16:
		return "int16"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint:
		return "uint"
	case uint8:
		return "uint8"
	case uint16:
		return "uint16"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case string:
		return "string"
	case bool:
		return "bool"
	case []byte:
		return "[]byte"
	case time.Time:
		return "time.Time"
	case *int:
		return "*int"
	case *int64:
		return "*int64"
	case *int32:
		return "*int32"
	case *string:
		return "*string"
	case *bool:
		return "*bool"
	case *float64:
		return "*float64"
	case *time.Time:
		return "*time.Time"
	default:
		return "any"
	}
}
