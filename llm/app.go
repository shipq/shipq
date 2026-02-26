package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// App is a registration shim that captures tool metadata.
// It is NOT a runtime client — it exists purely to collect
// information for code generation (same pattern as handler.App
// and channel.App).
//
// Critically, App has NO provider or model configuration.
// Provider choice happens at runtime in the user's Setup function.
type App struct {
	Tools []ToolDef
}

// NewApp creates a new App for registering LLM tools.
func NewApp() *App {
	return &App{}
}

// contextType is the reflect.Type of context.Context.
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// errorType is the reflect.Type of the error interface.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Tool registers a Go function as an LLM tool.
//
// The function must have one of these signatures:
//   - func(context.Context, *InputType) (*OutputType, error)
//   - func(*InputType) (*OutputType, error)
//
// InputType and OutputType must be structs. The JSON Schema is generated
// from InputType's fields using `json` tags for property names and `desc`
// tags for descriptions.
//
// Panics on invalid signatures (fail-fast at registration time).
func (a *App) Tool(name, description string, fn any) *App {
	// Check for duplicate names.
	for _, t := range a.Tools {
		if t.Name == name {
			panic(fmt.Sprintf("llm.App.Tool(%q): duplicate tool name", name))
		}
	}

	fnVal := reflect.ValueOf(fn)
	fnType := fnVal.Type()

	if fnType.Kind() != reflect.Func {
		panic(fmt.Sprintf("llm.App.Tool(%q): expected a function, got %s", name, fnType.Kind()))
	}

	// Validate argument count: 1 or 2.
	numIn := fnType.NumIn()
	if numIn < 1 || numIn > 2 {
		panic(fmt.Sprintf("llm.App.Tool(%q): function must take 1 or 2 arguments, got %d", name, numIn))
	}

	// Determine if the function takes a context as the first argument.
	hasContext := false
	inputArgIdx := 0
	if numIn == 2 {
		if !fnType.In(0).Implements(contextType) {
			panic(fmt.Sprintf("llm.App.Tool(%q): when function takes 2 arguments, first must implement context.Context, got %s", name, fnType.In(0)))
		}
		hasContext = true
		inputArgIdx = 1
	}

	// Validate the input argument: must be a pointer to a struct.
	inputArgType := fnType.In(inputArgIdx)
	if inputArgType.Kind() != reflect.Ptr || inputArgType.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("llm.App.Tool(%q): last argument must be a pointer to a struct (*T), got %s", name, inputArgType))
	}
	inputStructType := inputArgType.Elem()

	// Validate return count: exactly 2.
	if fnType.NumOut() != 2 {
		panic(fmt.Sprintf("llm.App.Tool(%q): function must return exactly 2 values, got %d", name, fnType.NumOut()))
	}

	// Validate first return value: must be a pointer to a struct.
	outType := fnType.Out(0)
	if outType.Kind() != reflect.Ptr || outType.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("llm.App.Tool(%q): first return value must be a pointer to a struct (*T), got %s", name, outType))
	}

	// Validate second return value: must implement error.
	if !fnType.Out(1).Implements(errorType) {
		panic(fmt.Sprintf("llm.App.Tool(%q): second return value must implement error, got %s", name, fnType.Out(1)))
	}

	// Generate JSON Schema from the input struct type.
	schema, err := SchemaFromType(inputStructType)
	if err != nil {
		panic(fmt.Sprintf("llm.App.Tool(%q): failed to generate schema: %v", name, err))
	}

	// Extract type metadata via reflection for the compile program.
	inputTypeName := inputStructType.Name()
	outputTypeName := outType.Elem().Name()

	// Use the input struct's package path as the tool's package.
	// This is the import path the compile program needs.
	pkgPath := inputStructType.PkgPath()

	// Build the ToolFunc closure using reflection.
	toolFunc := buildToolFunc(fnVal, inputStructType, hasContext)

	a.Tools = append(a.Tools, ToolDef{
		Name:        name,
		Description: description,
		InputSchema: schema,
		Func:        toolFunc,
		InputType:   inputTypeName,
		OutputType:  outputTypeName,
		Package:     pkgPath,
	})

	return a
}

// Registry returns a *Registry containing all registered tools.
// This allows the App to be used directly at runtime without codegen:
//
//	app := llm.NewApp()
//	weather.Register(app)
//	client := llm.NewClient(provider, llm.WithTools(app.Registry()))
func (a *App) Registry() *Registry {
	return &Registry{Tools: a.Tools}
}

// buildToolFunc creates a ToolFunc closure that uses reflection to:
// 1. Allocate a new instance of the input struct
// 2. Unmarshal JSON args into it
// 3. Call the function with the correct arguments
// 4. Marshal the output struct to JSON
func buildToolFunc(fnVal reflect.Value, inputStructType reflect.Type, hasContext bool) ToolFunc {
	return func(ctx context.Context, argsJSON []byte) ([]byte, error) {
		// 1. Allocate a new instance of the input struct.
		inputPtr := reflect.New(inputStructType)

		// 2. Unmarshal the raw JSON arguments into the input struct.
		if err := json.Unmarshal(argsJSON, inputPtr.Interface()); err != nil {
			return nil, fmt.Errorf("llm: unmarshal tool input: %w", err)
		}

		// 3. Build the argument list.
		var args []reflect.Value
		if hasContext {
			args = []reflect.Value{reflect.ValueOf(ctx), inputPtr}
		} else {
			args = []reflect.Value{inputPtr}
		}

		// 4. Call the function via reflection.
		results := fnVal.Call(args)

		// 5. Check the error return value.
		if errVal := results[1].Interface(); errVal != nil {
			return nil, errVal.(error)
		}

		// 6. Marshal the output struct to JSON.
		resultJSON, err := json.Marshal(results[0].Interface())
		if err != nil {
			return nil, fmt.Errorf("llm: marshal tool output: %w", err)
		}

		return resultJSON, nil
	}
}
