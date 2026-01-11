package query

import (
	"testing"
)

func TestWith(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}
	statusCol := StringColumn{Table: "orders", Name: "status"}

	pendingOrders := From(orders).
		Select(idCol).
		Where(statusCol.Eq(LiteralExpr{Value: "pending"}))

	cteBuilder := With("pending_orders", pendingOrders)

	if len(cteBuilder.ctes) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(cteBuilder.ctes))
	}
	if cteBuilder.ctes[0].Name != "pending_orders" {
		t.Errorf("expected CTE name = %q, got %q", "pending_orders", cteBuilder.ctes[0].Name)
	}
	if cteBuilder.ctes[0].Query == nil {
		t.Fatal("expected CTE Query to be set")
	}
}

func TestWithAnd(t *testing.T) {
	orders := mockTable{name: "orders"}
	customers := mockTable{name: "customers"}
	orderID := Int64Column{Table: "orders", Name: "id"}
	customerID := Int64Column{Table: "customers", Name: "id"}

	recentOrders := From(orders).Select(orderID)
	vipCustomers := From(customers).Select(customerID)

	cteBuilder := With("recent_orders", recentOrders).
		And("vip_customers", vipCustomers)

	if len(cteBuilder.ctes) != 2 {
		t.Fatalf("expected 2 CTEs, got %d", len(cteBuilder.ctes))
	}
	if cteBuilder.ctes[0].Name != "recent_orders" {
		t.Errorf("expected first CTE name = %q, got %q", "recent_orders", cteBuilder.ctes[0].Name)
	}
	if cteBuilder.ctes[1].Name != "vip_customers" {
		t.Errorf("expected second CTE name = %q, got %q", "vip_customers", cteBuilder.ctes[1].Name)
	}
}

func TestWithColumns(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}
	amountCol := Int64Column{Table: "orders", Name: "amount"}

	orderSummary := From(orders).Select(idCol, amountCol)

	cteBuilder := WithColumns("order_summary", []string{"order_id", "total"}, orderSummary)

	if len(cteBuilder.ctes) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(cteBuilder.ctes))
	}
	if len(cteBuilder.ctes[0].Columns) != 2 {
		t.Fatalf("expected 2 CTE columns, got %d", len(cteBuilder.ctes[0].Columns))
	}
	if cteBuilder.ctes[0].Columns[0] != "order_id" {
		t.Errorf("expected first column = %q, got %q", "order_id", cteBuilder.ctes[0].Columns[0])
	}
}

func TestCTESelect(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}
	statusCol := StringColumn{Table: "orders", Name: "status"}

	// WITH pending_orders AS (SELECT id FROM orders WHERE status = 'pending')
	// SELECT * FROM pending_orders
	pendingOrders := From(orders).
		Select(idCol).
		Where(statusCol.Eq(LiteralExpr{Value: "pending"}))

	pendingOrdersTable := CTERef("pending_orders")
	pendingIDCol := Int64Column{Table: "pending_orders", Name: "id"}

	ast := With("pending_orders", pendingOrders).
		Select(pendingOrdersTable).
		Select(pendingIDCol).
		Build()

	if len(ast.CTEs) != 1 {
		t.Fatalf("expected 1 CTE in AST, got %d", len(ast.CTEs))
	}
	if ast.CTEs[0].Name != "pending_orders" {
		t.Errorf("expected CTE name = %q, got %q", "pending_orders", ast.CTEs[0].Name)
	}
	if ast.FromTable.Name != "pending_orders" {
		t.Errorf("expected FromTable.Name = %q, got %q", "pending_orders", ast.FromTable.Name)
	}
}

func TestCTEWithWhere(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}

	recentOrders := From(orders).Select(idCol)
	recentOrdersTable := CTERef("recent_orders")
	recentIDCol := Int64Column{Table: "recent_orders", Name: "id"}

	ast := With("recent_orders", recentOrders).
		Select(recentOrdersTable).
		Select(recentIDCol).
		Where(recentIDCol.Gt(LiteralExpr{Value: 100})).
		Build()

	if ast.Where == nil {
		t.Fatal("expected Where to be set")
	}
}

func TestCTEWithJoin(t *testing.T) {
	orders := mockTable{name: "orders"}
	customers := mockTable{name: "customers"}
	orderID := Int64Column{Table: "orders", Name: "id"}
	orderCustomerID := Int64Column{Table: "orders", Name: "customer_id"}
	customerID := Int64Column{Table: "customers", Name: "id"}

	recentOrders := From(orders).Select(orderID, orderCustomerID)
	recentOrdersTable := CTERef("recent_orders")
	recentOrderIDCol := Int64Column{Table: "recent_orders", Name: "id"}
	recentOrderCustomerIDCol := Int64Column{Table: "recent_orders", Name: "customer_id"}

	ast := With("recent_orders", recentOrders).
		Select(recentOrdersTable).
		Select(recentOrderIDCol).
		LeftJoin(customers).On(recentOrderCustomerIDCol.Eq(ColumnExpr{Column: customerID})).
		Build()

	if len(ast.Joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(ast.Joins))
	}
	if ast.Joins[0].Type != LeftJoin {
		t.Errorf("expected LeftJoin, got %v", ast.Joins[0].Type)
	}
}

func TestCTEWithGroupBy(t *testing.T) {
	orders := mockTable{name: "orders"}
	customerID := Int64Column{Table: "orders", Name: "customer_id"}
	amountCol := Int64Column{Table: "orders", Name: "amount"}

	orderSummary := From(orders).
		Select(customerID).
		SelectSumAs(amountCol, "total").
		GroupBy(customerID)

	orderSummaryTable := CTERef("order_summary")
	summaryCustomerID := Int64Column{Table: "order_summary", Name: "customer_id"}

	ast := With("order_summary", orderSummary).
		Select(orderSummaryTable).
		Select(summaryCustomerID).
		Build()

	if len(ast.CTEs) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(ast.CTEs))
	}
	if len(ast.CTEs[0].Query.GroupBy) != 1 {
		t.Errorf("expected 1 GroupBy in CTE query, got %d", len(ast.CTEs[0].Query.GroupBy))
	}
}

func TestCTEWithOrderByLimit(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}

	recentOrders := From(orders).Select(idCol)
	recentOrdersTable := CTERef("recent_orders")
	recentIDCol := Int64Column{Table: "recent_orders", Name: "id"}

	ast := With("recent_orders", recentOrders).
		Select(recentOrdersTable).
		Select(recentIDCol).
		OrderBy(recentIDCol.Desc()).
		Limit(LiteralExpr{Value: 10}).
		Offset(LiteralExpr{Value: 5}).
		Build()

	if len(ast.OrderBy) != 1 {
		t.Fatalf("expected 1 OrderBy, got %d", len(ast.OrderBy))
	}
	if ast.Limit == nil {
		t.Fatal("expected Limit to be set")
	}
	if ast.Offset == nil {
		t.Fatal("expected Offset to be set")
	}
}

func TestCTETableName(t *testing.T) {
	cteTable := CTERef("my_cte")

	if cteTable.TableName() != "my_cte" {
		t.Errorf("expected TableName() = %q, got %q", "my_cte", cteTable.TableName())
	}
}

func TestMultipleCTEs(t *testing.T) {
	orders := mockTable{name: "orders"}
	customers := mockTable{name: "customers"}
	orderID := Int64Column{Table: "orders", Name: "id"}
	customerID := Int64Column{Table: "customers", Name: "id"}

	recentOrders := From(orders).Select(orderID)
	vipCustomers := From(customers).Select(customerID)

	recentOrdersTable := CTERef("recent_orders")
	recentIDCol := Int64Column{Table: "recent_orders", Name: "id"}

	ast := With("recent_orders", recentOrders).
		And("vip_customers", vipCustomers).
		Select(recentOrdersTable).
		Select(recentIDCol).
		Build()

	if len(ast.CTEs) != 2 {
		t.Fatalf("expected 2 CTEs, got %d", len(ast.CTEs))
	}
}
