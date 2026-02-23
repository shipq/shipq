package handlergen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strings"
)

// Operation represents a CRUD operation type for per-operation generation.
type Operation string

const (
	OpCreate Operation = "create"
	OpGetOne Operation = "get_one"
	OpList   Operation = "list"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
)

// AllOperations returns all CRUD operations in the standard order.
func AllOperations() []Operation {
	return []Operation{OpCreate, OpGetOne, OpList, OpUpdate, OpDelete}
}

// RouteRegistration describes a single route to be registered.
type RouteRegistration struct {
	Method      string // "Post", "Get", "Patch", "Delete"
	Path        string // "/books" or "/books/:id"
	FuncName    string // "CreateBook"
	RequireAuth bool
}

// RegistrationForOp returns the route registration for a given operation and table.
func RegistrationForOp(op Operation, tableName string, requireAuth bool) RouteRegistration {
	res := resourceName(tableName)
	plural := toPascalCase(tableName)

	switch op {
	case OpCreate:
		return RouteRegistration{
			Method:      "Post",
			Path:        "/" + tableName,
			FuncName:    "Create" + res,
			RequireAuth: requireAuth,
		}
	case OpGetOne:
		return RouteRegistration{
			Method:      "Get",
			Path:        "/" + tableName + "/:id",
			FuncName:    "Get" + res,
			RequireAuth: requireAuth,
		}
	case OpList:
		return RouteRegistration{
			Method:      "Get",
			Path:        "/" + tableName,
			FuncName:    "List" + plural,
			RequireAuth: requireAuth,
		}
	case OpUpdate:
		return RouteRegistration{
			Method:      "Patch",
			Path:        "/" + tableName + "/:id",
			FuncName:    "Update" + res,
			RequireAuth: requireAuth,
		}
	case OpDelete:
		return RouteRegistration{
			Method:      "Delete",
			Path:        "/" + tableName + "/:id",
			FuncName:    "SoftDelete" + res,
			RequireAuth: requireAuth,
		}
	default:
		panic("unknown operation: " + string(op))
	}
}

// GenerateIncrementalRegister generates or updates a register.go file,
// adding only the specified operations. If the file already exists, it
// parses existing routes and merges the new ones.
func GenerateIncrementalRegister(registerPath string, modulePath string, tableName string, ops []Operation, requireAuth bool) ([]byte, error) {
	// Collect desired registrations
	existing := parseExistingRoutes(registerPath)
	for _, op := range ops {
		reg := RegistrationForOp(op, tableName, requireAuth)
		// Replace existing route for the same func, or add new
		found := false
		for i, e := range existing {
			if e.FuncName == reg.FuncName {
				existing[i] = reg
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, reg)
		}
	}

	// Sort routes in canonical order: Create, List, GetOne, Update, Delete
	existing = sortRoutes(existing)

	return renderRegisterFile(modulePath, tableName, existing)
}

// parseExistingRoutes reads an existing register.go and extracts the route registrations.
// Returns empty slice if the file doesn't exist or can't be parsed.
func parseExistingRoutes(registerPath string) []RouteRegistration {
	content, err := os.ReadFile(registerPath)
	if err != nil {
		return nil
	}

	var routes []RouteRegistration
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "app.") {
			continue
		}

		requireAuth := strings.HasSuffix(line, ".Auth()")
		// Strip .Auth() suffix for parsing
		parseLine := strings.TrimSuffix(line, ".Auth()")

		// Parse app.Method("path", FuncName)
		reg := parseRouteLine(parseLine)
		if reg != nil {
			reg.RequireAuth = requireAuth
			routes = append(routes, *reg)
		}
	}

	return routes
}

// parseRouteLine extracts a RouteRegistration from a line like:
// app.Post("/books", CreateBook)
func parseRouteLine(line string) *RouteRegistration {
	// Remove trailing semicolons/whitespace
	line = strings.TrimRight(line, " \t;")

	for _, method := range []string{"Post", "Get", "Put", "Patch", "Delete"} {
		prefix := "app." + method + "(\""
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := line[len(prefix):]
		// Find closing quote for path
		quoteIdx := strings.Index(rest, "\"")
		if quoteIdx < 0 {
			continue
		}
		path := rest[:quoteIdx]
		// Find function name
		rest = rest[quoteIdx+1:]
		rest = strings.TrimLeft(rest, ", ")
		rest = strings.TrimRight(rest, ")")
		funcName := strings.TrimSpace(rest)
		if funcName == "" {
			continue
		}

		return &RouteRegistration{
			Method:   method,
			Path:     path,
			FuncName: funcName,
		}
	}
	return nil
}

// sortRoutes sorts routes in the canonical CRUD order.
func sortRoutes(routes []RouteRegistration) []RouteRegistration {
	order := map[string]int{
		"Post":   0, // Create
		"Get":    1, // List or GetOne (list first due to shorter path)
		"Patch":  3,
		"Put":    3,
		"Delete": 4,
	}

	// Stable sort to preserve ordering within same method
	result := make([]RouteRegistration, len(routes))
	copy(result, routes)
	for i := 1; i < len(result); i++ {
		for j := i; j > 0; j-- {
			oi := routeOrder(result[j], order)
			oj := routeOrder(result[j-1], order)
			if oi < oj {
				result[j], result[j-1] = result[j-1], result[j]
			}
		}
	}
	return result
}

func routeOrder(r RouteRegistration, order map[string]int) int {
	base := order[r.Method]
	// For GET, list (no :id) should come before get_one (has :id)
	if r.Method == "Get" && strings.Contains(r.Path, ":") {
		return base + 1
	}
	return base
}

func renderRegisterFile(modulePath string, tableName string, routes []RouteRegistration) ([]byte, error) {
	var buf bytes.Buffer
	res := toPascalCase(toSingular(tableName))
	plural := toPascalCase(tableName)

	buf.WriteString(generatedFileHeader)
	buf.WriteString("package " + tableName + "\n\n")
	buf.WriteString("import \"" + modulePath + "/shipq/lib/handler\"\n\n")
	buf.WriteString("// Register registers all " + toSingular(tableName) + " handlers with the app.\n")
	buf.WriteString("//\n")
	buf.WriteString("// To add a new route, define your handler function in this package and\n")
	buf.WriteString("// register it here:\n")
	buf.WriteString("//\n")
	buf.WriteString("//   app.Get(\"/" + tableName + "/popular\", ListPopular" + plural + "\")         // public route\n")
	buf.WriteString("//   app.Post(\"/" + tableName + "/import\", Import" + plural + "\").Auth()       // requires auth\n")
	buf.WriteString("//   app.Put(\"/" + tableName + "/:id/publish\", Publish" + res + ").Auth()    // requires auth\n")
	buf.WriteString("//\n")
	buf.WriteString("// Available methods: app.Get, app.Post, app.Put, app.Patch, app.Delete\n")
	buf.WriteString("func Register(app *handler.App) {\n")

	for _, r := range routes {
		authSuffix := ""
		if r.RequireAuth {
			authSuffix = ".Auth()"
		}
		buf.WriteString(fmt.Sprintf("\tapp.%s(\"%s\", %s)%s\n", r.Method, r.Path, r.FuncName, authSuffix))
	}

	buf.WriteString("}\n")

	return format.Source(buf.Bytes())
}
