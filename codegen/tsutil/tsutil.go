package tsutil

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/shipq/shipq/codegen"
)

// GoTypeToTS converts a SerializedFieldInfo to its TypeScript type equivalent.
// If the field has nested struct fields, it generates an inline interface.
func GoTypeToTS(field codegen.SerializedFieldInfo) string {
	// If we have nested struct fields, generate inline interface
	if field.StructFields != nil && len(field.StructFields.Fields) > 0 {
		inline := GoStructToInlineTS(field.StructFields)
		// Check if the Go type is a slice — if so, the TS type must be an array.
		goType := field.Type
		if strings.HasPrefix(goType, "*") {
			goType = goType[1:]
		}
		if strings.HasPrefix(goType, "[]") {
			return inline + "[]"
		}
		return inline
	}
	return GoTypeStringToTS(field.Type)
}

// GoTypeStringToTS converts a Go type string to its TypeScript equivalent.
func GoTypeStringToTS(goType string) string {
	// Handle slices
	if strings.HasPrefix(goType, "[]") {
		elemType := goType[2:]
		return GoTypeStringToTS(elemType) + "[]"
	}

	// Handle maps
	if strings.HasPrefix(goType, "map[") {
		rest := goType[4:] // after "map["
		bracketDepth := 0
		keyEnd := -1
		for i, c := range rest {
			if c == '[' {
				bracketDepth++
			} else if c == ']' {
				if bracketDepth == 0 {
					keyEnd = i
					break
				}
				bracketDepth--
			}
		}
		if keyEnd >= 0 {
			keyType := rest[:keyEnd]
			valType := rest[keyEnd+1:]
			return fmt.Sprintf("Record<%s, %s>", GoTypeStringToTS(keyType), GoTypeStringToTS(valType))
		}
	}

	// Handle pointer types
	if strings.HasPrefix(goType, "*") {
		return GoTypeStringToTS(goType[1:])
	}

	// Primitive type mapping
	switch goType {
	case "string":
		return "string"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "json.RawMessage":
		return "any"
	case "any", "interface{}":
		return "any"
	default:
		// Unknown types default to any
		return "any"
	}
}

// GoStructToInlineTS generates an inline TypeScript interface for a nested struct.
func GoStructToInlineTS(structInfo *codegen.SerializedStructInfo) string {
	var buf bytes.Buffer
	buf.WriteString("{ ")
	first := true
	for _, field := range structInfo.Fields {
		if field.JSONOmit {
			continue
		}
		if !first {
			buf.WriteString(" ")
		}
		first = false
		jsonName := field.JSONName
		if jsonName == "" {
			jsonName = field.Name
		}
		optional := ""
		if !field.Required {
			optional = "?"
		}
		tsType := GoTypeToTS(field)
		fmt.Fprintf(&buf, "%s%s: %s;", jsonName, optional, tsType)
	}
	buf.WriteString(" }")
	return buf.String()
}

// ToPascalCase converts a snake_case, hyphenated, or dotted name to PascalCase.
// e.g., "email_notification" -> "EmailNotification", "chatbot" -> "Chatbot"
func ToPascalCase(name string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range name {
		if r == '_' || r == '-' || r == '.' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ToCamelCase converts a snake_case or PascalCase name to camelCase.
// e.g., "CreatePost" -> "createPost", "soft_delete" -> "softDelete"
func ToCamelCase(name string) string {
	pascal := ToPascalCase(name)
	if pascal == "" {
		return ""
	}
	runes := []rune(pascal)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// GenerateTSInterface writes a TypeScript export interface block for a set of fields.
// It writes to the provided buffer with the given interface name.
func GenerateTSInterface(buf *bytes.Buffer, name string, fields []codegen.SerializedFieldInfo) {
	fmt.Fprintf(buf, "\nexport interface %s {\n", name)
	for _, field := range fields {
		if field.JSONOmit && field.JSONName == "" {
			continue
		}
		jsonName := field.JSONName
		if jsonName == "" {
			jsonName = field.Name
		}
		optional := ""
		if !field.Required {
			optional = "?"
		}
		tsType := GoTypeToTS(field)
		fmt.Fprintf(buf, "  %s%s: %s;\n", jsonName, optional, tsType)
	}
	buf.WriteString("}\n")
}
