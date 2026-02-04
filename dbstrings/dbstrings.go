// Package dbstrings provides string manipulation utilities for database-related
// code generation, including case conversion and pluralization helpers.
package dbstrings

import (
	"strings"
	"unicode"
)

// ToPascalCase converts a snake_case string to PascalCase.
// Examples:
//
//	"user_id" -> "UserId"
//	"created_at" -> "CreatedAt"
//	"id" -> "Id"
func ToPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// ToLowerCamel converts a PascalCase string to lowerCamelCase.
// Examples:
//
//	"GetUser" -> "getUser"
//	"UserID" -> "userID"
//	"ID" -> "iD"
func ToLowerCamel(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// ToSnakeCase converts a PascalCase or camelCase string to snake_case.
// Examples:
//
//	"UserID" -> "user_id"
//	"CreatedAt" -> "created_at"
//	"GetUserByEmail" -> "get_user_by_email"
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ToSingular converts a plural English word to its singular form.
// This is a simple implementation that handles common cases.
// Examples:
//
//	"users" -> "user"
//	"categories" -> "category"
//	"addresses" -> "address"
//	"children" -> "child"
func ToSingular(s string) string {
	// Handle common irregular plurals
	irregulars := map[string]string{
		"children": "child",
		"people":   "person",
		"men":      "man",
		"women":    "woman",
		"teeth":    "tooth",
		"feet":     "foot",
		"geese":    "goose",
		"mice":     "mouse",
		"indices":  "index",
		"matrices": "matrix",
		"vertices": "vertex",
		"quizzes":  "quiz",
	}

	lower := strings.ToLower(s)
	if singular, ok := irregulars[lower]; ok {
		// Preserve original casing of first letter
		if len(s) > 0 && unicode.IsUpper(rune(s[0])) {
			return strings.ToUpper(singular[:1]) + singular[1:]
		}
		return singular
	}

	// Handle regular patterns
	if strings.HasSuffix(s, "ies") && len(s) > 3 {
		// categories -> category
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "zzes") && len(s) > 4 {
		// buzzes -> buzz (but quizzes is handled above)
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") ||
		strings.HasSuffix(s, "ches") || strings.HasSuffix(s, "shes") {
		// addresses -> address, boxes -> box
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") && len(s) > 1 {
		// users -> user
		return s[:len(s)-1]
	}

	return s
}

// ToPlural converts a singular English word to its plural form.
// This is a simple implementation that handles common cases.
// Examples:
//
//	"user" -> "users"
//	"category" -> "categories"
//	"address" -> "addresses"
//	"child" -> "children"
func ToPlural(s string) string {
	// Handle common irregular plurals
	irregulars := map[string]string{
		"child":  "children",
		"person": "people",
		"man":    "men",
		"woman":  "women",
		"tooth":  "teeth",
		"foot":   "feet",
		"goose":  "geese",
		"mouse":  "mice",
		"index":  "indices",
		"matrix": "matrices",
		"vertex": "vertices",
		"quiz":   "quizzes",
	}

	lower := strings.ToLower(s)
	if plural, ok := irregulars[lower]; ok {
		// Preserve original casing of first letter
		if len(s) > 0 && unicode.IsUpper(rune(s[0])) {
			return strings.ToUpper(plural[:1]) + plural[1:]
		}
		return plural
	}

	// Handle regular patterns
	if strings.HasSuffix(s, "y") && len(s) > 1 {
		// Check if preceded by consonant
		prev := s[len(s)-2]
		if prev != 'a' && prev != 'e' && prev != 'i' && prev != 'o' && prev != 'u' {
			// category -> categories
			return s[:len(s)-1] + "ies"
		}
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		// address -> addresses, box -> boxes
		return s + "es"
	}

	// Default: add s
	return s + "s"
}

// ToTableName converts a model name to a table name (plural snake_case).
// Examples:
//
//	"User" -> "users"
//	"OrderItem" -> "order_items"
//	"Category" -> "categories"
func ToTableName(modelName string) string {
	return ToPlural(ToSnakeCase(modelName))
}

// ToModelName converts a table name to a model name (singular PascalCase).
// Examples:
//
//	"users" -> "User"
//	"order_items" -> "OrderItem"
//	"categories" -> "Category"
func ToModelName(tableName string) string {
	return ToPascalCase(ToSingular(tableName))
}
