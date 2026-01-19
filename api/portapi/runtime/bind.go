package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
)

// BindError represents an error that occurred during request binding.
type BindError struct {
	Source string // "path", "query", "header", "body"
	Field  string
	Err    error
}

func (e *BindError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s %s: %s", e.Source, e.Field, e.Err.Error())
	}
	return fmt.Sprintf("%s: %s", e.Source, e.Err.Error())
}

func (e *BindError) Unwrap() error {
	return e.Err
}

// Bind populates req from the HTTP request (path, query, header, JSON body).
// req must be a pointer to a struct.
func Bind(r *http.Request, req any) error {
	rv := reflect.ValueOf(req)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return errors.New("req must be pointer to struct")
	}
	rv = rv.Elem()
	rt := rv.Type()

	hasJSON := false

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)

		// Check for path binding
		if tag := field.Tag.Get("path"); tag != "" {
			if err := bindPath(r, fv, field, tag); err != nil {
				return err
			}
		}

		// Check for query binding
		if tag := field.Tag.Get("query"); tag != "" {
			if err := bindQuery(r, fv, field, tag); err != nil {
				return err
			}
		}

		// Check for header binding
		if tag := field.Tag.Get("header"); tag != "" {
			if err := bindHeader(r, fv, field, tag); err != nil {
				return err
			}
		}

		// Check for JSON fields
		if tag := field.Tag.Get("json"); tag != "" && tag != "-" {
			hasJSON = true
		}
	}

	// Bind JSON body if there are json-tagged fields
	if hasJSON {
		if err := bindJSONBody(r, req); err != nil {
			return err
		}
	}

	return nil
}

func bindPath(r *http.Request, fv reflect.Value, field reflect.StructField, tag string) error {
	value := r.PathValue(tag)
	if value == "" {
		return &BindError{
			Source: "path",
			Field:  tag,
			Err:    errors.New("missing required path variable"),
		}
	}

	converted, err := ConvertString(value, field.Type)
	if err != nil {
		return &BindError{
			Source: "path",
			Field:  tag,
			Err:    err,
		}
	}

	fv.Set(converted)
	return nil
}

func bindQuery(r *http.Request, fv reflect.Value, field reflect.StructField, tag string) error {
	query := r.URL.Query()

	// Handle pointer types (optional)
	if field.Type.Kind() == reflect.Ptr {
		if !query.Has(tag) {
			// Optional param not present, leave as nil
			return nil
		}
		// Get the value and convert to the pointed-to type
		value := query.Get(tag)
		elemType := field.Type.Elem()
		converted, err := ConvertString(value, elemType)
		if err != nil {
			return &BindError{
				Source: "query",
				Field:  tag,
				Err:    err,
			}
		}
		// Create a new pointer and set its value
		ptr := reflect.New(elemType)
		ptr.Elem().Set(converted)
		fv.Set(ptr)
		return nil
	}

	// Handle slice types (multi-value)
	if field.Type.Kind() == reflect.Slice {
		values := query[tag]
		if len(values) == 0 {
			// Leave slice as nil/empty
			return nil
		}
		elemType := field.Type.Elem()
		converted, err := ConvertStrings(values, elemType)
		if err != nil {
			return &BindError{
				Source: "query",
				Field:  tag,
				Err:    err,
			}
		}
		fv.Set(converted)
		return nil
	}

	// Handle scalar types (required)
	if !query.Has(tag) {
		return &BindError{
			Source: "query",
			Field:  tag,
			Err:    errors.New("missing required query parameter"),
		}
	}

	value := query.Get(tag)
	converted, err := ConvertString(value, field.Type)
	if err != nil {
		return &BindError{
			Source: "query",
			Field:  tag,
			Err:    err,
		}
	}

	fv.Set(converted)
	return nil
}

func bindHeader(r *http.Request, fv reflect.Value, field reflect.StructField, tag string) error {
	value := r.Header.Get(tag)

	// Handle pointer types (optional)
	if field.Type.Kind() == reflect.Ptr {
		if value == "" {
			// Optional header not present, leave as nil
			return nil
		}
		elemType := field.Type.Elem()
		converted, err := ConvertString(value, elemType)
		if err != nil {
			return &BindError{
				Source: "header",
				Field:  tag,
				Err:    err,
			}
		}
		ptr := reflect.New(elemType)
		ptr.Elem().Set(converted)
		fv.Set(ptr)
		return nil
	}

	// Required header
	if value == "" {
		return &BindError{
			Source: "header",
			Field:  tag,
			Err:    errors.New("missing required header"),
		}
	}

	converted, err := ConvertString(value, field.Type)
	if err != nil {
		return &BindError{
			Source: "header",
			Field:  tag,
			Err:    err,
		}
	}

	fv.Set(converted)
	return nil
}

func bindJSONBody(r *http.Request, req any) error {
	if r.Body == nil {
		return &BindError{
			Source: "body",
			Err:    errors.New("empty request body"),
		}
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return &BindError{
			Source: "body",
			Err:    err,
		}
	}

	if len(body) == 0 {
		return &BindError{
			Source: "body",
			Err:    errors.New("empty request body"),
		}
	}

	if err := json.Unmarshal(body, req); err != nil {
		return &BindError{
			Source: "body",
			Err:    err,
		}
	}

	return nil
}
