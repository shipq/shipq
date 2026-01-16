//go:build integration

package testing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/shipq/shipq/db/portsql/query"
	"github.com/shipq/shipq/db/proptest"
)

// trialCounter ensures unique identifiers across all tests
var trialCounter atomic.Int64

// uniqueID generates a unique identifier for testing
func uniqueID(base string) string {
	return fmt.Sprintf("%s_%d", base, trialCounter.Add(1))
}

func TestCrossDB_SimpleSelect(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "simple select returns same results", proptest.Config{NumTrials: 50, Verbose: true}, func(g *proptest.Generator) bool {
		// Clear previous data
		dbs.ClearAllData(t)

		// Generate unique author data
		publicID := uniqueID(g.Identifier(15))
		name := g.StringAlphaNum(30)
		if name == "" {
			name = "defaultname"
		}
		email := uniqueID(g.StringAlphaNum(15)) + "@test.com"

		// Insert same data into all databases
		dbs.InsertAuthor(t, publicID, name, email, nil, true)

		// Build query
		publicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		ast := query.From(MockTable("test_authors")).
			Select(publicIDCol, nameCol).
			Where(publicIDCol.Eq(query.Param[string]("public_id"))).
			Build()

		// Execute on all databases and collect results
		type Result struct {
			PublicID string
			Name     string
		}
		results := make(map[Dialect]Result)

		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var r Result
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr, publicID).Scan(&r.PublicID, &r.Name)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr, publicID).Scan(&r.PublicID, &r.Name)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr, publicID).Scan(&r.PublicID, &r.Name)
			}

			if err != nil {
				t.Logf("scan error for %s: %v", dialect, err)
				return false
			}
			results[dialect] = r
		}

		// Verify all results match
		pg := results[DialectPostgres]
		my := results[DialectMySQL]
		sq := results[DialectSQLite]

		if pg.PublicID != my.PublicID || my.PublicID != sq.PublicID {
			t.Logf("public_id mismatch: pg=%s my=%s sq=%s", pg.PublicID, my.PublicID, sq.PublicID)
			return false
		}
		if pg.Name != my.Name || my.Name != sq.Name {
			t.Logf("name mismatch: pg=%s my=%s sq=%s", pg.Name, my.Name, sq.Name)
			return false
		}

		return true
	})
}

func TestCrossDB_EdgeCaseStrings(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "edge case strings handled consistently across databases", proptest.Config{NumTrials: 100, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		publicID := uniqueID(g.Identifier(15))
		name := g.EdgeCaseString()
		if name == "" {
			name = "emptydefault" // Avoid empty string which may fail NOT NULL constraint
		}
		email := uniqueID(g.StringAlphaNum(10)) + "@test.com"

		// Try inserting into each database independently
		insertResults := dbs.TryInsertAuthorEach(publicID, name, email, nil, true)

		// Count successes and failures
		var succeeded, failed []Dialect
		for _, dialect := range AllDialects() {
			if insertResults[dialect] == nil {
				succeeded = append(succeeded, dialect)
			} else {
				failed = append(failed, dialect)
			}
		}

		// KEY INVARIANT: All databases must behave the same way.
		// Either all accept the string or all reject it.
		if len(succeeded) > 0 && len(failed) > 0 {
			t.Logf("INCONSISTENT: name=%q succeeded on %v, failed on %v", name, succeeded, failed)
			for d, err := range insertResults {
				if err != nil {
					t.Logf("  %s error: %v", d, err)
				}
			}
			return false
		}

		// If all databases rejected the string, that's consistent behavior - pass
		if len(failed) == len(AllDialects()) {
			return true
		}

		// All databases accepted the string - now verify roundtrip consistency
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}
		publicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}

		ast := query.From(MockTable("test_authors")).
			Select(nameCol).
			Where(publicIDCol.Eq(query.Param[string]("public_id"))).
			Build()

		names := make(map[Dialect]string)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			var gotName string
			var err error
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr, publicID).Scan(&gotName)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr, publicID).Scan(&gotName)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr, publicID).Scan(&gotName)
			}

			if err != nil {
				t.Logf("query error for %s: %v", dialect, err)
				return false
			}
			names[dialect] = gotName
		}

		// All databases should return the exact same string
		pg := names[DialectPostgres]
		my := names[DialectMySQL]
		sq := names[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("name mismatch: pg=%q my=%q sq=%q (original=%q)", pg, my, sq, name)
			return false
		}

		// Should match original
		if pg != name {
			t.Logf("roundtrip failed: got=%q want=%q", pg, name)
			return false
		}

		return true
	})
}

func TestCrossDB_BooleanValues(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "boolean values consistent across dbs", proptest.Config{NumTrials: 50, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		publicID := uniqueID(g.Identifier(15))
		name := g.StringAlphaNum(20)
		if name == "" {
			name = "defaultname"
		}
		email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
		active := g.Bool()

		dbs.InsertAuthor(t, publicID, name, email, nil, active)

		// Query with boolean literal
		activeCol := query.BoolColumn{Table: "test_authors", Name: "active"}
		publicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}

		ast := query.From(MockTable("test_authors")).
			Select(publicIDCol).
			Where(activeCol.Eq(query.Literal(active))).
			Build()

		// All databases should find the same row (or no row)
		foundPublicIDs := make(map[Dialect]string)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			var found string
			var err error
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr).Scan(&found)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr).Scan(&found)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr).Scan(&found)
			}

			if err == sql.ErrNoRows {
				foundPublicIDs[dialect] = ""
			} else if err != nil {
				t.Logf("query error for %s: %v", dialect, err)
				return false
			} else {
				foundPublicIDs[dialect] = found
			}
		}

		pg := foundPublicIDs[DialectPostgres]
		my := foundPublicIDs[DialectMySQL]
		sq := foundPublicIDs[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("boolean query mismatch: pg=%q my=%q sq=%q (active=%v)", pg, my, sq, active)
			return false
		}

		// Should find the row we inserted
		if pg != publicID {
			t.Logf("expected to find %q but got %q", publicID, pg)
			return false
		}

		return true
	})
}

func TestCrossDB_NullHandling(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "NULL values handled consistently", proptest.Config{NumTrials: 50, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		publicID := uniqueID(g.Identifier(15))
		name := g.StringAlphaNum(20)
		if name == "" {
			name = "defaultname"
		}
		email := uniqueID(g.StringAlphaNum(10)) + "@test.com"

		// Randomly make bio NULL or non-NULL
		var bio *string
		if g.Bool() {
			s := g.StringAlphaNum(50)
			bio = &s
		}

		dbs.InsertAuthor(t, publicID, name, email, bio, true)

		// Query bio column
		bioCol := query.NullStringColumn{Table: "test_authors", Name: "bio"}
		publicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}

		ast := query.From(MockTable("test_authors")).
			Select(bioCol).
			Where(publicIDCol.Eq(query.Param[string]("public_id"))).
			Build()

		bios := make(map[Dialect]*string)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			var gotBio sql.NullString
			var err error
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr, publicID).Scan(&gotBio)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr, publicID).Scan(&gotBio)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr, publicID).Scan(&gotBio)
			}

			if err != nil {
				t.Logf("query error for %s: %v", dialect, err)
				return false
			}

			if gotBio.Valid {
				bios[dialect] = &gotBio.String
			} else {
				bios[dialect] = nil
			}
		}

		pg := bios[DialectPostgres]
		my := bios[DialectMySQL]
		sq := bios[DialectSQLite]

		// All should be nil or all should have same value
		allNil := pg == nil && my == nil && sq == nil
		allEqual := pg != nil && my != nil && sq != nil && *pg == *my && *my == *sq

		if !allNil && !allEqual {
			var pgStr, myStr, sqStr string
			if pg != nil {
				pgStr = *pg
			}
			if my != nil {
				myStr = *my
			}
			if sq != nil {
				sqStr = *sq
			}
			t.Logf("NULL handling mismatch: pg=%q my=%q sq=%q", pgStr, myStr, sqStr)
			return false
		}

		return true
	})
}

func TestCrossDB_JSONAggregation(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "JSON aggregation produces equivalent results", proptest.Config{NumTrials: 30, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Create author with random number of books
		authorPublicID := uniqueID(g.Identifier(15))
		authorName := g.StringAlphaNum(20)
		if authorName == "" {
			authorName = "defaultauthor"
		}
		email := uniqueID(g.StringAlphaNum(10)) + "@test.com"

		dbs.InsertAuthor(t, authorPublicID, authorName, email, nil, true)

		numBooks := g.IntRange(0, 5)
		for i := 0; i < numBooks; i++ {
			bookPublicID := uniqueID(g.Identifier(15))
			title := g.StringAlphaNum(30)
			if title == "" {
				title = "defaulttitle"
			}
			price := fmt.Sprintf("%.2f", g.Float64Range(1.0, 100.0))

			dbs.InsertBook(t, bookPublicID, authorPublicID, title, &price)
		}

		// Query with JSON aggregation
		authorNameCol := query.StringColumn{Table: "test_authors", Name: "name"}
		authorIDCol := query.Int64Column{Table: "test_authors", Name: "id"}
		authorPublicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}
		bookAuthorIDCol := query.Int64Column{Table: "test_books", Name: "author_id"}
		bookTitleCol := query.StringColumn{Table: "test_books", Name: "title"}

		ast := query.From(MockTable("test_authors")).
			Select(authorNameCol).
			SelectJSONAgg("books", bookTitleCol).
			LeftJoin(MockTable("test_books")).On(authorIDCol.Eq(bookAuthorIDCol)).
			Where(authorPublicIDCol.Eq(query.Param[string]("public_id"))).
			GroupBy(authorNameCol).
			Build()

		type Result struct {
			Name  string
			Books []map[string]any
		}

		results := make(map[Dialect]Result)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var name string
			var booksJSON []byte
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr, authorPublicID).Scan(&name, &booksJSON)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr, authorPublicID).Scan(&name, &booksJSON)
			case DialectSQLite:
				var booksStr string
				err = dbs.SQLite.QueryRow(sqlStr, authorPublicID).Scan(&name, &booksStr)
				booksJSON = []byte(booksStr)
			}

			if err != nil {
				t.Logf("query error for %s: %v", dialect, err)
				return false
			}

			var books []map[string]any
			if err := json.Unmarshal(booksJSON, &books); err != nil {
				t.Logf("JSON unmarshal error for %s: %v (json=%s)", dialect, err, booksJSON)
				return false
			}

			results[dialect] = Result{Name: name, Books: books}
		}

		pg := results[DialectPostgres]
		my := results[DialectMySQL]
		sq := results[DialectSQLite]

		// Names should match
		if pg.Name != my.Name || my.Name != sq.Name {
			t.Logf("name mismatch: pg=%s my=%s sq=%s", pg.Name, my.Name, sq.Name)
			return false
		}

		// Filter out null entries from SQLite (which doesn't have FILTER clause)
		pgBooks := filterNullBooks(pg.Books)
		myBooks := filterNullBooks(my.Books)
		sqBooks := filterNullBooks(sq.Books)

		// Book counts should match
		if len(pgBooks) != len(myBooks) || len(myBooks) != len(sqBooks) {
			t.Logf("book count mismatch: pg=%d my=%d sq=%d", len(pgBooks), len(myBooks), len(sqBooks))
			return false
		}

		// Book count should match expected
		if len(pgBooks) != numBooks {
			t.Logf("unexpected book count: got=%d want=%d", len(pgBooks), numBooks)
			return false
		}

		return true
	})
}

// filterNullBooks removes entries where all values are null (from LEFT JOIN with no matches)
func filterNullBooks(books []map[string]any) []map[string]any {
	result := make([]map[string]any, 0)
	for _, book := range books {
		hasNonNull := false
		for _, v := range book {
			if v != nil {
				hasNonNull = true
				break
			}
		}
		if hasNonNull {
			result = append(result, book)
		}
	}
	return result
}

func TestCrossDB_SoftDeleteFiltering(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "soft delete filtering works", proptest.Config{NumTrials: 30, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert some active and some deleted authors
		numActive := g.IntRange(1, 5)
		numDeleted := g.IntRange(0, 3)

		for i := 0; i < numActive; i++ {
			publicID := uniqueID(g.Identifier(15))
			name := g.StringAlphaNum(20)
			if name == "" {
				name = "activename"
			}
			email := uniqueID(g.StringAlphaNum(10)) + fmt.Sprintf("%d@test.com", i)
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		for i := 0; i < numDeleted; i++ {
			publicID := uniqueID(g.Identifier(15))
			name := g.StringAlphaNum(20)
			if name == "" {
				name = "deletedname"
			}
			email := uniqueID(g.StringAlphaNum(10)) + fmt.Sprintf("del%d@test.com", i)
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
			// Soft delete
			dbs.SoftDeleteAuthor(t, publicID)
		}

		// Query with deleted_at IS NULL filter
		publicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}
		deletedAtCol := query.NullTimeColumn{Table: "test_authors", Name: "deleted_at"}

		ast := query.From(MockTable("test_authors")).
			Select(publicIDCol).
			Where(deletedAtCol.IsNull()).
			Build()

		counts := make(map[Dialect]int)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			count := 0

			switch dialect {
			case DialectPostgres:
				rows, err := dbs.Postgres.Query(ctx, sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					count++
				}
				rows.Close()
			case DialectMySQL:
				rows, err := dbs.MySQL.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					count++
				}
				rows.Close()
			case DialectSQLite:
				rows, err := dbs.SQLite.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					count++
				}
				rows.Close()
			}

			counts[dialect] = count
		}

		pg := counts[DialectPostgres]
		my := counts[DialectMySQL]
		sq := counts[DialectSQLite]

		// All should return same count
		if pg != my || my != sq {
			t.Logf("count mismatch: pg=%d my=%d sq=%d", pg, my, sq)
			return false
		}

		// Should only return active (non-deleted) authors
		if pg != numActive {
			t.Logf("expected %d active authors, got %d", numActive, pg)
			return false
		}

		return true
	})
}

func TestCrossDB_OrderByConsistency(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "ORDER BY produces consistent results", proptest.Config{NumTrials: 30, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert several authors with mixed case names
		// The MySQL compiler uses COLLATE utf8mb4_bin to ensure case-sensitive
		// ordering that matches Postgres and SQLite behavior
		numAuthors := g.IntRange(3, 10)
		for i := 0; i < numAuthors; i++ {
			publicID := uniqueID(g.Identifier(15))
			name := g.StringAlphaNum(20)
			if name == "" {
				name = fmt.Sprintf("name%d", i)
			}
			email := uniqueID(g.StringAlphaNum(10)) + fmt.Sprintf("%d@test.com", i)
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		// Query with ORDER BY
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		ast := query.From(MockTable("test_authors")).
			Select(nameCol).
			OrderBy(nameCol.Asc()).
			Build()

		allNames := make(map[Dialect][]string)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			var names []string

			switch dialect {
			case DialectPostgres:
				rows, err := dbs.Postgres.Query(ctx, sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				rows.Close()
			case DialectMySQL:
				rows, err := dbs.MySQL.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				rows.Close()
			case DialectSQLite:
				rows, err := dbs.SQLite.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				rows.Close()
			}

			allNames[dialect] = names
		}

		pg := allNames[DialectPostgres]
		my := allNames[DialectMySQL]
		sq := allNames[DialectSQLite]

		// Count should match
		if len(pg) != len(my) || len(my) != len(sq) {
			t.Logf("count mismatch: pg=%d my=%d sq=%d", len(pg), len(my), len(sq))
			return false
		}

		// Order should match (same sorting)
		for i := range pg {
			if pg[i] != my[i] || my[i] != sq[i] {
				t.Logf("order mismatch at %d: pg=%s my=%s sq=%s", i, pg[i], my[i], sq[i])
				return false
			}
		}

		return true
	})
}

func TestCrossDB_LimitOffset(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "LIMIT OFFSET works consistently", proptest.Config{NumTrials: 30, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert several authors with predictable order
		numAuthors := 10
		baseID := trialCounter.Add(1)
		for i := 0; i < numAuthors; i++ {
			publicID := fmt.Sprintf("author_%d_%02d", baseID, i)
			name := fmt.Sprintf("Author %02d", i) // Names are consistent for ordering
			email := fmt.Sprintf("author%d_%d@test.com", baseID, i)
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		limit := g.IntRange(1, 5)
		offset := g.IntRange(0, 5)

		// Query with LIMIT and OFFSET
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		ast := query.From(MockTable("test_authors")).
			Select(nameCol).
			OrderBy(nameCol.Asc()).
			Limit(query.Param[int]("limit")).
			Offset(query.Param[int]("offset")).
			Build()

		allNames := make(map[Dialect][]string)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			var names []string

			switch dialect {
			case DialectPostgres:
				rows, err := dbs.Postgres.Query(ctx, sqlStr, limit, offset)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				rows.Close()
			case DialectMySQL:
				rows, err := dbs.MySQL.Query(sqlStr, limit, offset)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				rows.Close()
			case DialectSQLite:
				rows, err := dbs.SQLite.Query(sqlStr, limit, offset)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				rows.Close()
			}

			allNames[dialect] = names
		}

		pg := allNames[DialectPostgres]
		my := allNames[DialectMySQL]
		sq := allNames[DialectSQLite]

		// Count should match
		if len(pg) != len(my) || len(my) != len(sq) {
			t.Logf("count mismatch: pg=%d my=%d sq=%d (limit=%d offset=%d)", len(pg), len(my), len(sq), limit, offset)
			return false
		}

		// Results should match
		for i := range pg {
			if pg[i] != my[i] || my[i] != sq[i] {
				t.Logf("result mismatch at %d: pg=%s my=%s sq=%s", i, pg[i], my[i], sq[i])
				return false
			}
		}

		return true
	})
}

func TestCrossDB_InClause(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "IN clause works consistently", proptest.Config{NumTrials: 30, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert several authors
		numAuthors := 5
		baseID := trialCounter.Add(1)
		publicIDs := make([]string, numAuthors)
		for i := 0; i < numAuthors; i++ {
			publicID := fmt.Sprintf("author_%d_%02d", baseID, i)
			publicIDs[i] = publicID
			name := fmt.Sprintf("Author %02d", i)
			email := fmt.Sprintf("author%d_%d@test.com", baseID, i)
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		// Select a random subset to query
		numToQuery := g.IntRange(1, numAuthors)
		queryIDs := publicIDs[:numToQuery]

		// Build query with IN clause using string literals
		publicIDCol := query.StringColumn{Table: "test_authors", Name: "public_id"}
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		// Convert to interface slice for In()
		inValues := make([]any, len(queryIDs))
		for i, id := range queryIDs {
			inValues[i] = id
		}

		ast := query.From(MockTable("test_authors")).
			Select(publicIDCol, nameCol).
			Where(publicIDCol.In(inValues...)).
			OrderBy(publicIDCol.Asc()).
			Build()

		allResults := make(map[Dialect][]string)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, _ := CompileFor(ast, dialect)

			var results []string

			switch dialect {
			case DialectPostgres:
				rows, err := dbs.Postgres.Query(ctx, sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var id, name string
					rows.Scan(&id, &name)
					results = append(results, id)
				}
				rows.Close()
			case DialectMySQL:
				rows, err := dbs.MySQL.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var id, name string
					rows.Scan(&id, &name)
					results = append(results, id)
				}
				rows.Close()
			case DialectSQLite:
				rows, err := dbs.SQLite.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					var id, name string
					rows.Scan(&id, &name)
					results = append(results, id)
				}
				rows.Close()
			}

			allResults[dialect] = results
		}

		pg := allResults[DialectPostgres]
		my := allResults[DialectMySQL]
		sq := allResults[DialectSQLite]

		// Count should match expected
		if len(pg) != numToQuery || len(my) != numToQuery || len(sq) != numToQuery {
			t.Logf("count mismatch: pg=%d my=%d sq=%d want=%d", len(pg), len(my), len(sq), numToQuery)
			return false
		}

		// Results should match
		for i := range pg {
			if pg[i] != my[i] || my[i] != sq[i] {
				t.Logf("result mismatch at %d: pg=%s my=%s sq=%s", i, pg[i], my[i], sq[i])
				return false
			}
		}

		return true
	})
}

// =============================================================================
// Phase 7: Property Tests for Advanced SQL Features
// =============================================================================

func TestCrossDB_CountAggregate(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "COUNT(*) returns same results across all databases", proptest.Config{NumTrials: 20, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Generate random number of authors
		numAuthors := g.IntRange(1, 10)

		for i := 0; i < numAuthors; i++ {
			publicID := uniqueID(g.Identifier(10))
			name := g.StringAlphaNum(20)
			if name == "" {
				name = "defaultname"
			}
			email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		// Build COUNT(*) query using builder DSL
		ast := query.From(MockTable("test_authors")).
			SelectCount().
			Build()

		counts := make(map[Dialect]int)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var count int
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr).Scan(&count)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr).Scan(&count)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr).Scan(&count)
			}
			if err != nil {
				t.Logf("query error for %s: %v", dialect, err)
				return false
			}
			counts[dialect] = count
		}

		pg := counts[DialectPostgres]
		my := counts[DialectMySQL]
		sq := counts[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("count mismatch: pg=%d my=%d sq=%d", pg, my, sq)
			return false
		}
		if pg != numAuthors {
			t.Logf("count != expected: got %d, want %d", pg, numAuthors)
			return false
		}

		return true
	})
}

func TestCrossDB_SelectDistinct(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "SELECT DISTINCT returns same results across all databases", proptest.Config{NumTrials: 20, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Generate some authors with duplicate names
		names := []string{"Alice", "Bob", "Charlie"}
		name := names[g.IntRange(0, len(names)-1)]

		// Insert 2-5 authors with the same name
		numAuthors := g.IntRange(2, 5)
		for i := 0; i < numAuthors; i++ {
			publicID := uniqueID(g.Identifier(10))
			email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		// Build SELECT DISTINCT query using builder DSL
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		ast := query.From(MockTable("test_authors")).
			Distinct().
			Select(nameCol).
			Build()

		distinctCounts := make(map[Dialect]int)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var count int
			switch dialect {
			case DialectPostgres:
				rows, err := dbs.Postgres.Query(ctx, sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					count++
				}
				rows.Close()
			case DialectMySQL:
				rows, err := dbs.MySQL.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					count++
				}
				rows.Close()
			case DialectSQLite:
				rows, err := dbs.SQLite.Query(sqlStr)
				if err != nil {
					t.Logf("query error for %s: %v", dialect, err)
					return false
				}
				for rows.Next() {
					count++
				}
				rows.Close()
			}
			distinctCounts[dialect] = count
		}

		pg := distinctCounts[DialectPostgres]
		my := distinctCounts[DialectMySQL]
		sq := distinctCounts[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("distinct count mismatch: pg=%d my=%d sq=%d", pg, my, sq)
			return false
		}
		// Should only have 1 distinct name
		if pg != 1 {
			t.Logf("distinct count should be 1, got %d", pg)
			return false
		}

		return true
	})
}

func TestCrossDB_CountDistinctAggregate(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "COUNT(DISTINCT) returns same results across all databases", proptest.Config{NumTrials: 20, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert authors with some duplicate names
		names := []string{"Alice", "Bob", "Charlie"}
		numAuthors := g.IntRange(5, 10)
		namesUsed := make(map[string]bool)

		for i := 0; i < numAuthors; i++ {
			publicID := uniqueID(g.Identifier(10))
			name := names[g.IntRange(0, len(names)-1)]
			namesUsed[name] = true
			email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		expectedDistinct := len(namesUsed)

		// Build COUNT(DISTINCT name) query using builder DSL
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		ast := query.From(MockTable("test_authors")).
			SelectCountDistinct(nameCol).
			Build()

		counts := make(map[Dialect]int)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var count int
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr).Scan(&count)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr).Scan(&count)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr).Scan(&count)
			}
			if err != nil {
				t.Logf("query error for %s: %v", dialect, err)
				return false
			}
			counts[dialect] = count
		}

		pg := counts[DialectPostgres]
		my := counts[DialectMySQL]
		sq := counts[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("count distinct mismatch: pg=%d my=%d sq=%d", pg, my, sq)
			return false
		}
		if pg != expectedDistinct {
			t.Logf("count distinct != expected: got %d, want %d", pg, expectedDistinct)
			return false
		}

		return true
	})
}

func TestCrossDB_CTE(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "CTEs return same results across all databases", proptest.Config{NumTrials: 20, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert authors with varying active status
		numActive := g.IntRange(2, 5)
		numInactive := g.IntRange(1, 3)

		for i := 0; i < numActive; i++ {
			publicID := uniqueID(g.Identifier(10))
			name := fmt.Sprintf("Active_%d", i)
			email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
			dbs.InsertAuthor(t, publicID, name, email, nil, true) // active = true
		}

		for i := 0; i < numInactive; i++ {
			publicID := uniqueID(g.Identifier(10))
			name := fmt.Sprintf("Inactive_%d", i)
			email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
			dbs.InsertAuthor(t, publicID, name, email, nil, false) // active = false
		}

		// Build CTE query using builder DSL:
		// WITH active_authors AS (SELECT name FROM test_authors WHERE active = true)
		// SELECT COUNT(*) FROM active_authors
		activeCol := query.BoolColumn{Table: "test_authors", Name: "active"}
		nameCol := query.StringColumn{Table: "test_authors", Name: "name"}

		cteQuery := query.From(MockTable("test_authors")).
			Select(nameCol).
			Where(activeCol.Eq(query.Literal(true)))

		ast := query.With("active_authors", cteQuery).
			Select(query.CTERef("active_authors")).
			SelectCount().
			Build()

		counts := make(map[Dialect]int)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var count int
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr).Scan(&count)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr).Scan(&count)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr).Scan(&count)
			}
			if err != nil {
				t.Logf("query error for %s: %v\nSQL: %s", dialect, err, sqlStr)
				return false
			}
			counts[dialect] = count
		}

		pg := counts[DialectPostgres]
		my := counts[DialectMySQL]
		sq := counts[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("CTE count mismatch: pg=%d my=%d sq=%d", pg, my, sq)
			return false
		}
		if pg != numActive {
			t.Logf("CTE count != expected: got %d, want %d", pg, numActive)
			return false
		}

		return true
	})
}

func TestCrossDB_CTEWithJoin(t *testing.T) {
	dbs, cleanup := SetupTestDBs(t)
	if dbs == nil {
		return
	}
	defer cleanup()

	proptest.Check(t, "CTEs with aggregation return same results across all databases", proptest.Config{NumTrials: 15, Verbose: true}, func(g *proptest.Generator) bool {
		dbs.ClearAllData(t)

		// Insert authors
		numAuthors := g.IntRange(2, 4)
		authorIDs := make([]string, numAuthors)
		for i := 0; i < numAuthors; i++ {
			publicID := uniqueID(g.Identifier(10))
			authorIDs[i] = publicID
			name := fmt.Sprintf("Author_%d", i)
			email := uniqueID(g.StringAlphaNum(10)) + "@test.com"
			dbs.InsertAuthor(t, publicID, name, email, nil, true)
		}

		// Insert books for the first author only
		numBooks := g.IntRange(1, 4)
		for i := 0; i < numBooks; i++ {
			bookID := uniqueID(g.Identifier(10))
			title := fmt.Sprintf("Book_%d", i)
			dbs.InsertBook(t, bookID, authorIDs[0], title, nil)
		}

		// Build CTE query using builder DSL:
		// WITH author_books AS (SELECT author_id, COUNT(*) as book_count FROM test_books GROUP BY author_id)
		// SELECT COUNT(*) FROM author_books WHERE book_count > 0
		authorIDCol := query.Int64Column{Table: "test_books", Name: "author_id"}

		cteQuery := query.From(MockTable("test_books")).
			Select(authorIDCol).
			SelectCountAs("book_count").
			GroupBy(authorIDCol)

		// Count how many authors have books
		bookCountCol := query.Int64Column{Table: "author_books", Name: "book_count"}

		ast := query.With("author_books", cteQuery).
			Select(query.CTERef("author_books")).
			SelectCount().
			Where(bookCountCol.Gt(query.Literal(0))).
			Build()

		counts := make(map[Dialect]int)
		ctx := context.Background()

		for _, dialect := range AllDialects() {
			sqlStr, _, err := CompileFor(ast, dialect)
			if err != nil {
				t.Logf("compile error for %s: %v", dialect, err)
				return false
			}

			var count int
			switch dialect {
			case DialectPostgres:
				err = dbs.Postgres.QueryRow(ctx, sqlStr).Scan(&count)
			case DialectMySQL:
				err = dbs.MySQL.QueryRow(sqlStr).Scan(&count)
			case DialectSQLite:
				err = dbs.SQLite.QueryRow(sqlStr).Scan(&count)
			}
			if err != nil {
				t.Logf("query error for %s: %v\nSQL: %s", dialect, err, sqlStr)
				return false
			}
			counts[dialect] = count
		}

		pg := counts[DialectPostgres]
		my := counts[DialectMySQL]
		sq := counts[DialectSQLite]

		if pg != my || my != sq {
			t.Logf("CTE with aggregation count mismatch: pg=%d my=%d sq=%d", pg, my, sq)
			return false
		}
		// Only 1 author has books
		if pg != 1 {
			t.Logf("CTE count != expected: got %d, want 1 (only one author has books)", pg)
			return false
		}

		return true
	})
}
