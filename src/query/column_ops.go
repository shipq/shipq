package query

// This file contains comparison and ordering methods for all column types.
// Each column type supports: Eq, Ne, Lt, Le, Gt, Ge, In, IsNull, IsNotNull, Asc, Desc
// String columns additionally support: Like, ILike

// --- Int32Column operations ---

func (c Int32Column) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c Int32Column) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c Int32Column) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c Int32Column) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c Int32Column) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c Int32Column) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c Int32Column) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c Int32Column) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c Int32Column) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c Int32Column) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c Int32Column) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- NullInt32Column operations ---

func (c NullInt32Column) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullInt32Column) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullInt32Column) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c NullInt32Column) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c NullInt32Column) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c NullInt32Column) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c NullInt32Column) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c NullInt32Column) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullInt32Column) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullInt32Column) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullInt32Column) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- Int64Column operations ---

func (c Int64Column) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c Int64Column) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c Int64Column) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c Int64Column) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c Int64Column) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c Int64Column) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c Int64Column) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c Int64Column) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c Int64Column) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c Int64Column) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c Int64Column) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- NullInt64Column operations ---

func (c NullInt64Column) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullInt64Column) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullInt64Column) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c NullInt64Column) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c NullInt64Column) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c NullInt64Column) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c NullInt64Column) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c NullInt64Column) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullInt64Column) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullInt64Column) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullInt64Column) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- Float64Column operations ---

func (c Float64Column) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c Float64Column) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c Float64Column) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c Float64Column) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c Float64Column) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c Float64Column) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c Float64Column) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c Float64Column) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c Float64Column) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c Float64Column) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c Float64Column) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- NullFloat64Column operations ---

func (c NullFloat64Column) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullFloat64Column) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullFloat64Column) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c NullFloat64Column) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c NullFloat64Column) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c NullFloat64Column) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c NullFloat64Column) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c NullFloat64Column) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullFloat64Column) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullFloat64Column) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullFloat64Column) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- DecimalColumn operations ---

func (c DecimalColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c DecimalColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c DecimalColumn) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c DecimalColumn) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c DecimalColumn) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c DecimalColumn) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c DecimalColumn) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c DecimalColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c DecimalColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c DecimalColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c DecimalColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- NullDecimalColumn operations ---

func (c NullDecimalColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullDecimalColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullDecimalColumn) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c NullDecimalColumn) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c NullDecimalColumn) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c NullDecimalColumn) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c NullDecimalColumn) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c NullDecimalColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullDecimalColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullDecimalColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullDecimalColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- BoolColumn operations ---

func (c BoolColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c BoolColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c BoolColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c BoolColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c BoolColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c BoolColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- NullBoolColumn operations ---

func (c NullBoolColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullBoolColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullBoolColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullBoolColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullBoolColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullBoolColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- StringColumn operations ---

func (c StringColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c StringColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c StringColumn) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c StringColumn) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c StringColumn) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c StringColumn) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c StringColumn) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c StringColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c StringColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c StringColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c StringColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// Like matches using SQL LIKE pattern.
func (c StringColumn) Like(pattern string) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLike, Right: LiteralExpr{Value: pattern}}
}

// ILike matches using case-insensitive pattern (translated per database).
func (c StringColumn) ILike(pattern string) Expr {
	return FuncExpr{
		Name: "ILIKE",
		Args: []Expr{ColumnExpr{c}, LiteralExpr{Value: pattern}},
	}
}

// --- NullStringColumn operations ---

func (c NullStringColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullStringColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullStringColumn) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c NullStringColumn) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c NullStringColumn) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c NullStringColumn) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c NullStringColumn) In(values ...any) Expr {
	exprs := make([]Expr, len(values))
	for i, v := range values {
		exprs[i] = toExpr(v)
	}
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpIn, Right: ListExpr{Values: exprs}}
}

func (c NullStringColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullStringColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullStringColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullStringColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

func (c NullStringColumn) Like(pattern string) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLike, Right: LiteralExpr{Value: pattern}}
}

func (c NullStringColumn) ILike(pattern string) Expr {
	return FuncExpr{
		Name: "ILIKE",
		Args: []Expr{ColumnExpr{c}, LiteralExpr{Value: pattern}},
	}
}

// --- TimeColumn operations ---

func (c TimeColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c TimeColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c TimeColumn) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c TimeColumn) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c TimeColumn) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c TimeColumn) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c TimeColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c TimeColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c TimeColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c TimeColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- NullTimeColumn operations ---

func (c NullTimeColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullTimeColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullTimeColumn) Lt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLt, Right: toExpr(other)}
}

func (c NullTimeColumn) Le(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpLe, Right: toExpr(other)}
}

func (c NullTimeColumn) Gt(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGt, Right: toExpr(other)}
}

func (c NullTimeColumn) Ge(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpGe, Right: toExpr(other)}
}

func (c NullTimeColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullTimeColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

func (c NullTimeColumn) Asc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: false}
}

func (c NullTimeColumn) Desc() OrderByExpr {
	return OrderByExpr{Expr: ColumnExpr{c}, Desc: true}
}

// --- BytesColumn operations ---

func (c BytesColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c BytesColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c BytesColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c BytesColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

// --- JSONColumn operations ---

func (c JSONColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c JSONColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c JSONColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c JSONColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}

// --- NullJSONColumn operations ---

func (c NullJSONColumn) Eq(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpEq, Right: toExpr(other)}
}

func (c NullJSONColumn) Ne(other any) Expr {
	return BinaryExpr{Left: ColumnExpr{c}, Op: OpNe, Right: toExpr(other)}
}

func (c NullJSONColumn) IsNull() Expr {
	return UnaryExpr{Op: OpIsNull, Expr: ColumnExpr{c}}
}

func (c NullJSONColumn) IsNotNull() Expr {
	return UnaryExpr{Op: OpNotNull, Expr: ColumnExpr{c}}
}
