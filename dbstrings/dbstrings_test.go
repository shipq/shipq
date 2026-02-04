package dbstrings

import "testing"

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user", "User"},
		{"user_id", "UserId"},
		{"created_at", "CreatedAt"},
		{"public_id", "PublicId"},
		{"id", "Id"},
		{"email", "Email"},
		{"", ""},
		{"a", "A"},
		{"user_email_address", "UserEmailAddress"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToLowerCamel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "user"},
		{"UserID", "userID"},
		{"GetUserByEmail", "getUserByEmail"},
		{"ID", "iD"},
		{"", ""},
		{"a", "a"},
		{"A", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToLowerCamel(tt.input)
			if result != tt.expected {
				t.Errorf("ToLowerCamel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "user"},
		{"UserID", "user_i_d"},
		{"CreatedAt", "created_at"},
		{"GetUserByEmail", "get_user_by_email"},
		{"", ""},
		{"a", "a"},
		{"ABC", "a_b_c"},
		{"userEmail", "user_email"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToSingular(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"orders", "order"},
		{"categories", "category"},
		{"addresses", "address"},
		{"boxes", "box"},
		{"churches", "church"},
		{"bushes", "bush"},
		{"quizzes", "quiz"},
		{"children", "child"},
		{"people", "person"},
		{"men", "man"},
		{"women", "woman"},
		{"teeth", "tooth"},
		{"feet", "foot"},
		{"geese", "goose"},
		{"mice", "mouse"},
		{"user", "user"},
		{"", ""},
		{"s", "s"}, // single 's' stays as 's'
		{"indices", "index"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToSingular(tt.input)
			if result != tt.expected {
				t.Errorf("ToSingular(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToPlural(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user", "users"},
		{"order", "orders"},
		{"category", "categories"},
		{"address", "addresses"},
		{"box", "boxes"},
		{"church", "churches"},
		{"bush", "bushes"},
		{"quiz", "quizzes"},
		{"child", "children"},
		{"person", "people"},
		{"man", "men"},
		{"woman", "women"},
		{"tooth", "teeth"},
		{"foot", "feet"},
		{"goose", "geese"},
		{"mouse", "mice"},
		{"day", "days"},
		{"key", "keys"},
		{"", "s"},
		{"index", "indices"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToPlural(tt.input)
			if result != tt.expected {
				t.Errorf("ToPlural(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToTableName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "users"},
		{"OrderItem", "order_items"},
		{"Category", "categories"},
		{"Person", "people"},
		{"Child", "children"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToTableName(tt.input)
			if result != tt.expected {
				t.Errorf("ToTableName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "User"},
		{"order_items", "OrderItem"},
		{"categories", "Category"},
		{"people", "Person"},
		{"children", "Child"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToModelName(tt.input)
			if result != tt.expected {
				t.Errorf("ToModelName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToSingular_PreservesCasing(t *testing.T) {
	// Test that irregular plurals preserve the casing of the first letter
	tests := []struct {
		input    string
		expected string
	}{
		{"Children", "Child"},
		{"children", "child"},
		{"People", "Person"},
		{"people", "person"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToSingular(tt.input)
			if result != tt.expected {
				t.Errorf("ToSingular(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToPlural_PreservesCasing(t *testing.T) {
	// Test that irregular singulars preserve the casing of the first letter
	tests := []struct {
		input    string
		expected string
	}{
		{"Child", "Children"},
		{"child", "children"},
		{"Person", "People"},
		{"person", "people"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ToPlural(tt.input)
			if result != tt.expected {
				t.Errorf("ToPlural(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
