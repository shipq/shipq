package query

// =============================================================================
// Aggregate Function Builders
// =============================================================================

// Count creates a COUNT(*) expression.
func Count() AggregateExpr {
	return AggregateExpr{Func: AggCount, Arg: nil}
}

// CountCol creates a COUNT(column) expression.
func CountCol(col Column) AggregateExpr {
	return AggregateExpr{Func: AggCount, Arg: ColumnExpr{Column: col}}
}

// CountDistinct creates a COUNT(DISTINCT column) expression.
func CountDistinct(col Column) AggregateExpr {
	return AggregateExpr{Func: AggCount, Arg: ColumnExpr{Column: col}, Distinct: true}
}

// CountExpr creates a COUNT(expr) expression.
func CountExpr(expr Expr) AggregateExpr {
	return AggregateExpr{Func: AggCount, Arg: expr}
}

// Sum creates a SUM(column) expression.
func Sum(col Column) AggregateExpr {
	return AggregateExpr{Func: AggSum, Arg: ColumnExpr{Column: col}}
}

// SumExpr creates a SUM(expr) expression.
func SumExpr(expr Expr) AggregateExpr {
	return AggregateExpr{Func: AggSum, Arg: expr}
}

// Avg creates an AVG(column) expression.
func Avg(col Column) AggregateExpr {
	return AggregateExpr{Func: AggAvg, Arg: ColumnExpr{Column: col}}
}

// AvgExpr creates an AVG(expr) expression.
func AvgExpr(expr Expr) AggregateExpr {
	return AggregateExpr{Func: AggAvg, Arg: expr}
}

// Min creates a MIN(column) expression.
func Min(col Column) AggregateExpr {
	return AggregateExpr{Func: AggMin, Arg: ColumnExpr{Column: col}}
}

// MinExpr creates a MIN(expr) expression.
func MinExpr(expr Expr) AggregateExpr {
	return AggregateExpr{Func: AggMin, Arg: expr}
}

// Max creates a MAX(column) expression.
func Max(col Column) AggregateExpr {
	return AggregateExpr{Func: AggMax, Arg: ColumnExpr{Column: col}}
}

// MaxExpr creates a MAX(expr) expression.
func MaxExpr(expr Expr) AggregateExpr {
	return AggregateExpr{Func: AggMax, Arg: expr}
}

// =============================================================================
// Aggregate SelectBuilder Methods
// =============================================================================

// SelectCount adds COUNT(*) to the SELECT clause.
func (b *SelectBuilder) SelectCount() *SelectBuilder {
	return b.SelectExpr(Count())
}

// SelectCountAs adds COUNT(*) AS alias to the SELECT clause.
func (b *SelectBuilder) SelectCountAs(alias string) *SelectBuilder {
	return b.SelectExprAs(Count(), alias)
}

// SelectCountCol adds COUNT(column) to the SELECT clause.
func (b *SelectBuilder) SelectCountCol(col Column) *SelectBuilder {
	return b.SelectExpr(CountCol(col))
}

// SelectCountDistinct adds COUNT(DISTINCT column) to the SELECT clause.
func (b *SelectBuilder) SelectCountDistinct(col Column) *SelectBuilder {
	return b.SelectExpr(CountDistinct(col))
}

// SelectSum adds SUM(column) to the SELECT clause.
func (b *SelectBuilder) SelectSum(col Column) *SelectBuilder {
	return b.SelectExpr(Sum(col))
}

// SelectSumAs adds SUM(column) AS alias to the SELECT clause.
func (b *SelectBuilder) SelectSumAs(col Column, alias string) *SelectBuilder {
	return b.SelectExprAs(Sum(col), alias)
}

// SelectAvg adds AVG(column) to the SELECT clause.
func (b *SelectBuilder) SelectAvg(col Column) *SelectBuilder {
	return b.SelectExpr(Avg(col))
}

// SelectAvgAs adds AVG(column) AS alias to the SELECT clause.
func (b *SelectBuilder) SelectAvgAs(col Column, alias string) *SelectBuilder {
	return b.SelectExprAs(Avg(col), alias)
}

// SelectMin adds MIN(column) to the SELECT clause.
func (b *SelectBuilder) SelectMin(col Column) *SelectBuilder {
	return b.SelectExpr(Min(col))
}

// SelectMinAs adds MIN(column) AS alias to the SELECT clause.
func (b *SelectBuilder) SelectMinAs(col Column, alias string) *SelectBuilder {
	return b.SelectExprAs(Min(col), alias)
}

// SelectMax adds MAX(column) to the SELECT clause.
func (b *SelectBuilder) SelectMax(col Column) *SelectBuilder {
	return b.SelectExpr(Max(col))
}

// SelectMaxAs adds MAX(column) AS alias to the SELECT clause.
func (b *SelectBuilder) SelectMaxAs(col Column, alias string) *SelectBuilder {
	return b.SelectExprAs(Max(col), alias)
}
