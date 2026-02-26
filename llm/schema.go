package llm

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// SchemaFromType generates a JSON Schema (as json.RawMessage) from a Go reflect.Type.
//
// Type mappings:
//
//	string              → {"type": "string"}
//	int, int32, int64   → {"type": "integer"}
//	float32, float64    → {"type": "number"}
//	bool                → {"type": "boolean"}
//	*T                  → schema of T with "null" added to the type union
//	[]T                 → {"type": "array", "items": <schema of T>}
//	struct{...}         → {"type": "object", "properties": {...}, "required": [...], "additionalProperties": false}
//	map[string]T        → {"type": "object", "additionalProperties": <schema of T>}
//	time.Time           → {"type": "string", "format": "date-time"}
//
// Tag handling:
//   - `json` tag → property name. `json:"-"` excludes the field.
//   - `desc` tag → "description" field in the schema.
//
// Strict mode (applied to all objects):
//   - All fields (pointer and non-pointer alike) go into "required".
//   - Pointer fields get a ["type", "null"] union.
//   - "additionalProperties": false on all objects.
func SchemaFromType(t reflect.Type) (json.RawMessage, error) {
	s, err := schemaFromType(t, false)
	if err != nil {
		return nil, err
	}
	return marshalSchema(s)
}

// schemaNode is an intermediate representation of a JSON Schema object.
// We build this tree first, then marshal it to JSON.
type schemaNode map[string]any

func marshalSchema(s schemaNode) (json.RawMessage, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("llm.SchemaFromType: marshal: %w", err)
	}
	return json.RawMessage(b), nil
}

// schemaFromType is the recursive workhorse. nullable is true when we are
// processing the element of a pointer type (so the caller adds "null" to the
// type union).
func schemaFromType(t reflect.Type, nullable bool) (schemaNode, error) {
	// Dereference pointer — but track that we became nullable.
	if t.Kind() == reflect.Ptr {
		inner, err := schemaFromType(t.Elem(), true)
		if err != nil {
			return nil, err
		}
		return inner, nil
	}

	// time.Time is special-cased before the struct branch.
	if t == timeType {
		s := schemaNode{"type": "string", "format": "date-time"}
		if nullable {
			s["type"] = []string{"string", "null"}
		}
		return s, nil
	}

	switch t.Kind() {
	case reflect.String:
		s := schemaNode{"type": "string"}
		if nullable {
			s["type"] = []string{"string", "null"}
		}
		return s, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s := schemaNode{"type": "integer"}
		if nullable {
			s["type"] = []string{"integer", "null"}
		}
		return s, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s := schemaNode{"type": "integer"}
		if nullable {
			s["type"] = []string{"integer", "null"}
		}
		return s, nil

	case reflect.Float32, reflect.Float64:
		s := schemaNode{"type": "number"}
		if nullable {
			s["type"] = []string{"number", "null"}
		}
		return s, nil

	case reflect.Bool:
		s := schemaNode{"type": "boolean"}
		if nullable {
			s["type"] = []string{"boolean", "null"}
		}
		return s, nil

	case reflect.Slice:
		itemSchema, err := schemaFromType(t.Elem(), false)
		if err != nil {
			return nil, fmt.Errorf("array element: %w", err)
		}
		s := schemaNode{"type": "array", "items": itemSchema}
		if nullable {
			s["type"] = []string{"array", "null"}
		}
		return s, nil

	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("llm.SchemaFromType: map key must be string, got %s", t.Key().Kind())
		}
		valueSchema, err := schemaFromType(t.Elem(), false)
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}
		s := schemaNode{
			"type":                 "object",
			"additionalProperties": valueSchema,
		}
		if nullable {
			s["type"] = []string{"object", "null"}
		}
		return s, nil

	case reflect.Struct:
		return structSchema(t, nullable)

	default:
		return nil, fmt.Errorf("llm.SchemaFromType: unsupported kind %s (type %s)", t.Kind(), t)
	}
}

// structSchema builds the JSON Schema for a struct type.
func structSchema(t reflect.Type, nullable bool) (schemaNode, error) {
	properties := map[string]any{}
	required := []string{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields.
		if !field.IsExported() {
			continue
		}

		// Resolve the JSON property name and check for exclusion.
		propName, excluded := jsonFieldName(field)
		if excluded {
			continue
		}

		// Determine if the field type is a pointer (nullable in the schema).
		isPtr := field.Type.Kind() == reflect.Ptr

		fieldSchema, err := schemaFromType(field.Type, false)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		// If it's a pointer, convert single "type" string to ["type", "null"] union.
		if isPtr {
			fieldSchema = addNullToType(fieldSchema)
		}

		// Apply description tag if present.
		if desc, ok := field.Tag.Lookup("desc"); ok && desc != "" {
			fieldSchema["description"] = desc
		}

		properties[propName] = fieldSchema
		// All fields go into required (both pointer and non-pointer).
		required = append(required, propName)
	}

	s := schemaNode{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
	if nullable {
		s["type"] = []string{"object", "null"}
	}
	return s, nil
}

// jsonFieldName returns the JSON property name for a struct field and whether
// the field should be excluded from the schema.
func jsonFieldName(field reflect.StructField) (name string, excluded bool) {
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return field.Name, false
	}
	parts := strings.SplitN(tag, ",", 2)
	if parts[0] == "-" {
		return "", true
	}
	if parts[0] == "" {
		return field.Name, false
	}
	return parts[0], false
}

// addNullToType takes a schemaNode that has a "type" field (either a string or
// []string) and returns a copy with "null" added to the type union.
func addNullToType(s schemaNode) schemaNode {
	// Shallow copy to avoid mutating the original.
	out := make(schemaNode, len(s)+1)
	for k, v := range s {
		out[k] = v
	}

	switch tv := s["type"].(type) {
	case string:
		out["type"] = []string{tv, "null"}
	case []string:
		// Avoid duplicates.
		for _, t := range tv {
			if t == "null" {
				return out
			}
		}
		newTypes := make([]string, len(tv)+1)
		copy(newTypes, tv)
		newTypes[len(tv)] = "null"
		out["type"] = newTypes
	}
	return out
}
