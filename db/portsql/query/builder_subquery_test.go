package query

import (
	"testing"
)

func TestSubquery(t *testing.T) {
	vipCustomers := mockTable{name: "vip_customers"}
	idCol := Int64Column{Table: "vip_customers", Name: "id"}

	subBuilder := From(vipCustomers).Select(idCol)
	subExpr := Subquery(subBuilder)

	if subExpr.Query == nil {
		t.Fatal("expected Query to be set")
	}
	if subExpr.Query.FromTable.Name != "vip_customers" {
		t.Errorf("expected FromTable.Name = %q, got %q", "vip_customers", subExpr.Query.FromTable.Name)
	}
}

func TestExists(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}

	subBuilder := From(orders).Select(idCol)
	existsExpr := Exists(subBuilder)

	if existsExpr.Subquery == nil {
		t.Fatal("expected Subquery to be set")
	}
	if existsExpr.Negated {
		t.Error("expected Negated = false for EXISTS")
	}
}

func TestNotExists(t *testing.T) {
	orders := mockTable{name: "orders"}
	idCol := Int64Column{Table: "orders", Name: "id"}

	subBuilder := From(orders).Select(idCol)
	notExistsExpr := NotExists(subBuilder)

	if notExistsExpr.Subquery == nil {
		t.Fatal("expected Subquery to be set")
	}
	if !notExistsExpr.Negated {
		t.Error("expected Negated = true for NOT EXISTS")
	}
}

func TestInSubquery_Int64(t *testing.T) {
	orders := mockTable{name: "orders"}
	vipCustomers := mockTable{name: "vip_customers"}
	customerID := Int64Column{Table: "orders", Name: "customer_id"}
	vipID := Int64Column{Table: "vip_customers", Name: "id"}

	subBuilder := From(vipCustomers).Select(vipID)
	inExpr := customerID.InSubquery(subBuilder)

	if inExpr.Op != OpIn {
		t.Errorf("expected Op = OpIn, got %v", inExpr.Op)
	}

	// Check left side is the column
	leftCol, ok := inExpr.Left.(ColumnExpr)
	if !ok {
		t.Fatalf("expected Left to be ColumnExpr, got %T", inExpr.Left)
	}
	if leftCol.Column.ColumnName() != "customer_id" {
		t.Errorf("expected column = %q, got %q", "customer_id", leftCol.Column.ColumnName())
	}

	// Check right side is subquery
	subq, ok := inExpr.Right.(SubqueryExpr)
	if !ok {
		t.Fatalf("expected Right to be SubqueryExpr, got %T", inExpr.Right)
	}
	if subq.Query == nil {
		t.Fatal("expected subquery Query to be set")
	}

	// Use the expression in a query
	ast := From(orders).
		Select(customerID).
		Where(inExpr).
		Build()

	if ast.Where == nil {
		t.Fatal("expected Where to be set")
	}
}

func TestInSubquery_String(t *testing.T) {
	emails := mockTable{name: "emails"}
	blocklist := mockTable{name: "blocklist"}
	emailCol := StringColumn{Table: "emails", Name: "email"}
	blockedEmail := StringColumn{Table: "blocklist", Name: "email"}

	subBuilder := From(blocklist).Select(blockedEmail)
	inExpr := emailCol.InSubquery(subBuilder)

	if inExpr.Op != OpIn {
		t.Errorf("expected Op = OpIn, got %v", inExpr.Op)
	}

	// Use the expression in a query
	ast := From(emails).
		Select(emailCol).
		Where(inExpr).
		Build()

	if ast.FromTable.Name != "emails" {
		t.Errorf("expected FromTable.Name = %q, got %q", "emails", ast.FromTable.Name)
	}
}

func TestExistsInWhere(t *testing.T) {
	customers := mockTable{name: "customers"}
	orders := mockTable{name: "orders"}
	customerID := Int64Column{Table: "customers", Name: "id"}
	orderCustomerID := Int64Column{Table: "orders", Name: "customer_id"}

	// Customers who have placed orders
	subBuilder := From(orders).
		Select(orderCustomerID).
		Where(orderCustomerID.Eq(ColumnExpr{Column: customerID}))

	ast := From(customers).
		Select(customerID).
		Where(Exists(subBuilder)).
		Build()

	existsExpr, ok := ast.Where.(ExistsExpr)
	if !ok {
		t.Fatalf("expected Where to be ExistsExpr, got %T", ast.Where)
	}
	if existsExpr.Negated {
		t.Error("expected Negated = false")
	}
}

func TestNotExistsInWhere(t *testing.T) {
	customers := mockTable{name: "customers"}
	orders := mockTable{name: "orders"}
	customerID := Int64Column{Table: "customers", Name: "id"}
	orderCustomerID := Int64Column{Table: "orders", Name: "customer_id"}

	// Customers who have NOT placed orders
	subBuilder := From(orders).
		Select(orderCustomerID).
		Where(orderCustomerID.Eq(ColumnExpr{Column: customerID}))

	ast := From(customers).
		Select(customerID).
		Where(NotExists(subBuilder)).
		Build()

	existsExpr, ok := ast.Where.(ExistsExpr)
	if !ok {
		t.Fatalf("expected Where to be ExistsExpr, got %T", ast.Where)
	}
	if !existsExpr.Negated {
		t.Error("expected Negated = true for NOT EXISTS")
	}
}

func TestScalarSubqueryComparison(t *testing.T) {
	products := mockTable{name: "products"}
	priceCol := Int64Column{Table: "products", Name: "price"}

	// Products with price above average
	avgSubquery := From(products).SelectExprAs(Avg(priceCol), "avg_price")

	gtExpr := priceCol.GtSubquery(avgSubquery)

	if gtExpr.Op != OpGt {
		t.Errorf("expected Op = OpGt, got %v", gtExpr.Op)
	}

	_, ok := gtExpr.Right.(SubqueryExpr)
	if !ok {
		t.Fatalf("expected Right to be SubqueryExpr, got %T", gtExpr.Right)
	}
}
