package querydef

import (
	schema "github.com/portsql/portsql/demo/schematypes"
	"github.com/portsql/portsql/src/query"
)

func init() {
	// GetPetById - Simple SELECT with WHERE clause (returns 0 or 1 row)
	query.MustDefineOne("GetPetById",
		query.From(schema.Pets).
			Select(
				schema.Pets.Id(),
				schema.Pets.Name(),
				schema.Pets.CategoryId(),
				schema.Pets.Status(),
				schema.Pets.PhotoUrls(),
			).
			Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
			Build(),
	)

	// FindPetsByStatus - Filter pets by status (returns multiple rows)
	query.MustDefineMany("FindPetsByStatus",
		query.From(schema.Pets).
			Select(
				schema.Pets.Id(),
				schema.Pets.Name(),
				schema.Pets.CategoryId(),
				schema.Pets.Status(),
			).
			Where(schema.Pets.Status().Eq(query.Param[string]("status"))).
			Build(),
	)

	// GetUserByUsername - Find user by username (returns 0 or 1 row)
	query.MustDefineOne("GetUserByUsername",
		query.From(schema.Users).
			Select(
				schema.Users.Id(),
				schema.Users.PublicId(),
				schema.Users.Username(),
				schema.Users.FirstName(),
				schema.Users.LastName(),
				schema.Users.Email(),
				schema.Users.CreatedAt(),
			).
			Where(schema.Users.Username().Eq(query.Param[string]("username"))).
			Build(),
	)

	// GetOrderById - Order lookup (returns 0 or 1 row)
	query.MustDefineOne("GetOrderById",
		query.From(schema.Orders).
			Select(
				schema.Orders.Id(),
				schema.Orders.PetId(),
				schema.Orders.Quantity(),
				schema.Orders.ShipDate(),
				schema.Orders.Status(),
				schema.Orders.Complete(),
			).
			Where(schema.Orders.Id().Eq(query.Param[int64]("id"))).
			Build(),
	)

	// ListPetsWithCategory - JOIN example (returns multiple rows)
	query.MustDefineMany("ListPetsWithCategory",
		query.From(schema.Pets).
			Join(schema.Categories).On(schema.Pets.CategoryId().Eq(schema.Categories.Id())).
			Select(
				schema.Pets.Id(),
				schema.Pets.Name(),
				schema.Pets.Status(),
			).
			SelectAs(schema.Categories.Name(), "category_name").
			Build(),
	)

	// GetPetWithPhotos - Example using JSON column for nested data (returns 0 or 1 row)
	// The photo_urls column stores an array of URLs as JSON, e.g. ["http://example.com/photo1.jpg", "http://example.com/photo2.jpg"]
	// This demonstrates how portsql handles JSON columns - the result will be json.RawMessage
	query.MustDefineOne("GetPetWithPhotos",
		query.From(schema.Pets).
			Join(schema.Categories).On(schema.Pets.CategoryId().Eq(schema.Categories.Id())).
			Select(
				schema.Pets.Id(),
				schema.Pets.Name(),
				schema.Pets.PhotoUrls(), // JSON column - returns json.RawMessage
				schema.Pets.Status(),
			).
			SelectAs(schema.Categories.Name(), "category_name").
			Where(schema.Pets.Id().Eq(query.Param[int64]("id"))).
			Build(),
	)

	// ListCategoriesWithPets - JSON aggregation example (returns multiple rows)
	// This demonstrates using JSON aggregation to return nested data:
	// { "category": "Dogs", "pets": [{"id": 1, "name": "Buddy"}, ...] }
	// Uses database-native JSON functions (json_group_array for SQLite, JSON_ARRAYAGG for MySQL, json_agg for Postgres)
	query.MustDefineMany("ListCategoriesWithPets",
		query.From(schema.Categories).
			LeftJoin(schema.Pets).On(schema.Categories.Id().Eq(schema.Pets.CategoryId())).
			Select(
				schema.Categories.Id(),
				schema.Categories.Name(),
			).
			SelectJSONAgg("pets", // Aggregates into a "pets" JSON array
				schema.Pets.Id(),
				schema.Pets.Name(),
				schema.Pets.Status(),
			).
			GroupBy(schema.Categories.Id(), schema.Categories.Name()).
			Build(),
	)

}
