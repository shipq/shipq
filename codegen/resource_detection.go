package codegen

import (
	"sort"
	"strings"

)

// ResourceInfo holds information about a resource package and its CRUD operations.
type ResourceInfo struct {
	PackagePath string // Full package path, e.g., "myapp/api/resources/accounts"
	PackageName string // Package name, e.g., "accounts"

	// CRUD operation detection
	HasCreate bool // POST without path param (or only parent resource params)
	HasGetOne bool // GET with path param for resource ID
	HasList   bool // GET without path params (or only query params)
	HasUpdate bool // PUT or PATCH with path param
	HasDelete bool // DELETE with path param

	// Handler references for each operation
	CreateHandler *SerializedHandlerInfo
	GetOneHandler *SerializedHandlerInfo
	ListHandler   *SerializedHandlerInfo
	UpdateHandler *SerializedHandlerInfo
	DeleteHandler *SerializedHandlerInfo
}

// IsFullResource returns true if all 5 CRUD operations are present.
func (r ResourceInfo) IsFullResource() bool {
	return r.HasCreate && r.HasGetOne && r.HasList && r.HasUpdate && r.HasDelete
}

// DetectFullResources analyzes handlers to detect which packages are "full resources"
// (packages that implement all 5 CRUD operations: Create, GetOne, List, Update, Delete).
func DetectFullResources(handlers []SerializedHandlerInfo) []ResourceInfo {
	// Group handlers by package path
	byPackage := make(map[string][]SerializedHandlerInfo)
	for _, h := range handlers {
		if h.PackagePath != "" {
			byPackage[h.PackagePath] = append(byPackage[h.PackagePath], h)
		}
	}

	// Analyze each package
	var resources []ResourceInfo
	for pkgPath, pkgHandlers := range byPackage {
		info := analyzePackage(pkgPath, pkgHandlers)
		resources = append(resources, info)
	}

	// Sort for deterministic output
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].PackagePath < resources[j].PackagePath
	})

	return resources
}

// FilterFullResources returns only the resources that have all 5 CRUD operations.
func FilterFullResources(resources []ResourceInfo) []ResourceInfo {
	var full []ResourceInfo
	for _, r := range resources {
		if r.IsFullResource() {
			full = append(full, r)
		}
	}
	return full
}

// analyzePackage determines which CRUD operations a package implements.
func analyzePackage(pkgPath string, handlers []SerializedHandlerInfo) ResourceInfo {
	info := ResourceInfo{
		PackagePath: pkgPath,
		PackageName: extractPackageName(pkgPath),
	}

	for i := range handlers {
		h := &handlers[i]
		switch classifyCRUDOperation(h) {
		case crudCreate:
			info.HasCreate = true
			info.CreateHandler = h
		case crudGetOne:
			info.HasGetOne = true
			info.GetOneHandler = h
		case crudList:
			info.HasList = true
			info.ListHandler = h
		case crudUpdate:
			info.HasUpdate = true
			info.UpdateHandler = h
		case crudDelete:
			info.HasDelete = true
			info.DeleteHandler = h
		}
	}

	return info
}

// crudOperation represents a CRUD operation type.
type crudOperation int

const (
	crudUnknown crudOperation = iota
	crudCreate
	crudGetOne
	crudList
	crudUpdate
	crudDelete
)

// classifyCRUDOperation determines which CRUD operation a handler represents.
// The heuristics are:
//   - Create: POST, no path params (or only parent resource params)
//   - GetOne: GET, has path param for resource ID
//   - List: GET, no path params (or only query params)
//   - Update: PUT or PATCH, has path param
//   - Delete: DELETE, has path param
func classifyCRUDOperation(h *SerializedHandlerInfo) crudOperation {
	switch h.Method {
	case "POST":
		// Create: POST without path params for the resource itself
		// (may have parent params like /users/:user_id/posts)
		if !hasResourceIDParam(h) {
			return crudCreate
		}
		return crudUnknown

	case "GET":
		// GetOne: GET with path param for resource ID
		// List: GET without path params
		if hasResourceIDParam(h) {
			return crudGetOne
		}
		return crudList

	case "PUT", "PATCH":
		// Update: PUT or PATCH with path param
		if hasResourceIDParam(h) {
			return crudUpdate
		}
		return crudUnknown

	case "DELETE":
		// Delete: DELETE with path param
		if hasResourceIDParam(h) {
			return crudDelete
		}
		return crudUnknown
	}

	return crudUnknown
}

// hasResourceIDParam checks if the handler has a path parameter that looks like a resource ID.
// Common patterns: :id, :public_id, :{resource}_id
func hasResourceIDParam(h *SerializedHandlerInfo) bool {
	if len(h.PathParams) == 0 {
		return false
	}

	// Check if any path param looks like an ID
	for _, param := range h.PathParams {
		name := strings.ToLower(param.Name)
		// Common ID patterns
		if name == "id" ||
			name == "public_id" ||
			name == "publicid" ||
			strings.HasSuffix(name, "_id") ||
			strings.HasSuffix(name, "id") {
			return true
		}
	}

	// If there are path params but none look like IDs, still consider them as resource identifiers
	// This handles cases like /posts/:slug
	return len(h.PathParams) > 0
}

// extractPackageName extracts the package name from a full package path.
func extractPackageName(pkgPath string) string {
	parts := strings.Split(pkgPath, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// GetResourceBasePath attempts to extract the base path for a resource.
// For example, if handlers are at /accounts, /accounts/:id, etc., returns "/accounts".
func GetResourceBasePath(info ResourceInfo) string {
	// Try to find the List handler path (which typically has no path params)
	if info.ListHandler != nil {
		return info.ListHandler.Path
	}

	// Fall back to Create handler
	if info.CreateHandler != nil {
		return info.CreateHandler.Path
	}

	// Try to extract from GetOne by removing the last path segment
	if info.GetOneHandler != nil {
		path := info.GetOneHandler.Path
		// Remove trailing /:param
		if idx := strings.LastIndex(path, "/:"); idx > 0 {
			return path[:idx]
		}
		if idx := strings.LastIndex(path, "/"); idx > 0 {
			return path[:idx]
		}
	}

	return ""
}

// GetResourceIDField returns the name of the ID field in the request struct
// used to identify a single resource.
func GetResourceIDField(info ResourceInfo) string {
	// Check GetOne handler for the ID field
	if info.GetOneHandler != nil && info.GetOneHandler.Request != nil {
		for _, field := range info.GetOneHandler.Request.Fields {
			name := strings.ToLower(field.JSONName)
			if name == "id" ||
				name == "public_id" ||
				name == "publicid" ||
				strings.HasSuffix(name, "_id") {
				return field.Name
			}
		}
		// If no obvious ID field, use the first path param field
		if len(info.GetOneHandler.PathParams) > 0 {
			paramName := info.GetOneHandler.PathParams[0].Name
			for _, field := range info.GetOneHandler.Request.Fields {
				if strings.EqualFold(field.JSONName, paramName) ||
					strings.EqualFold(field.Name, paramName) {
					return field.Name
				}
			}
		}
	}

	// Default to "PublicID" which is a common pattern
	return "PublicID"
}

// GetResourceIDJSONName returns the JSON name of the ID field in responses.
func GetResourceIDJSONName(info ResourceInfo) string {
	// Check GetOne response for the ID field
	if info.GetOneHandler != nil && info.GetOneHandler.Response != nil {
		for _, field := range info.GetOneHandler.Response.Fields {
			name := strings.ToLower(field.JSONName)
			if name == "id" ||
				name == "public_id" ||
				name == "publicid" ||
				strings.HasSuffix(name, "_id") {
				return field.JSONName
			}
		}
	}

	// Default to "public_id" which is a common pattern
	return "public_id"
}
