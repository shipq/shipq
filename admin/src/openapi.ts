/**
 * openapi.ts - Parse an OpenAPI 3.1 spec and extract resource capabilities.
 *
 * Pure functions with no DOM dependency, designed for easy testing.
 */

/** A single field in a resource's schema. */
export interface FieldSchema {
  name: string;
  type: "string" | "integer" | "number" | "boolean" | "datetime" | "unknown";
  required: boolean;
  /** True if the field is read-only (id, created_at, updated_at, etc.) */
  readonly: boolean;
}

/** Capabilities and metadata for a single API resource (table). */
export interface ResourceInfo {
  /** Display name / tag (e.g., "accounts") */
  name: string;

  /** Endpoint paths */
  listPath: string | null;
  createPath: string | null;
  getOnePath: string | null;
  updatePath: string | null;
  deletePath: string | null;
  adminListPath: string | null;
  restorePath: string | null;

  /** HTTP methods for update (PATCH or PUT) */
  updateMethod: string | null;

  /** Capability flags derived from endpoint presence */
  canList: boolean;
  canCreate: boolean;
  canGetOne: boolean;
  canUpdate: boolean;
  canDelete: boolean;
  canAdminList: boolean;
  canRestore: boolean;

  /** JSON key containing the array of items in list responses (e.g. "items") */
  listItemsKey: string;

  /** Fields from the list/get response schema */
  responseFields: FieldSchema[];
  /** Fields editable via update (from PATCH/PUT request body) */
  editableFields: FieldSchema[];
  /** Fields for creating (from POST request body) */
  creatableFields: FieldSchema[];
}

/** Minimal OpenAPI spec shape we care about. */
export interface OpenAPISpec {
  paths?: Record<string, PathItem>;
  components?: { schemas?: Record<string, SchemaObject> };
}

interface PathItem {
  get?: OperationObject;
  post?: OperationObject;
  put?: OperationObject;
  patch?: OperationObject;
  delete?: OperationObject;
}

interface OperationObject {
  operationId?: string;
  tags?: string[];
  parameters?: ParameterObject[];
  requestBody?: RequestBodyObject;
  responses?: Record<string, ResponseObject>;
}

interface ParameterObject {
  name: string;
  in: string;
  schema?: SchemaObject;
}

interface RequestBodyObject {
  content?: Record<string, { schema?: SchemaObject }>;
}

interface ResponseObject {
  content?: Record<string, { schema?: SchemaObject }>;
}

interface SchemaObject {
  type?: string;
  format?: string;
  properties?: Record<string, SchemaObject>;
  required?: string[];
  items?: SchemaObject;
  nullable?: boolean;
}

// ---- Read-only fields: these are auto-managed by the server ----
const READONLY_FIELDS = new Set([
  "id",
  "public_id",
  "created_at",
  "updated_at",
]);

/**
 * Map an OpenAPI schema type+format to our simplified FieldSchema type.
 */
export function mapFieldType(
  schema: SchemaObject | undefined
): FieldSchema["type"] {
  if (!schema) return "unknown";
  if (schema.type === "string" && schema.format === "date-time")
    return "datetime";
  if (schema.type === "string") return "string";
  if (schema.type === "integer") return "integer";
  if (schema.type === "number") return "number";
  if (schema.type === "boolean") return "boolean";
  return "unknown";
}

/**
 * Extract fields from a JSON schema's properties.
 */
export function extractFields(
  schema: SchemaObject | undefined,
  markReadonly: boolean
): FieldSchema[] {
  if (!schema?.properties) return [];
  const required = new Set(schema.required ?? []);
  return Object.entries(schema.properties).map(([name, prop]) => ({
    name,
    type: mapFieldType(prop),
    required: required.has(name),
    readonly: markReadonly && READONLY_FIELDS.has(name),
  }));
}

/**
 * Find the response schema for a successful response (200 or 201).
 */
function getSuccessResponseSchema(
  op: OperationObject | undefined
): SchemaObject | undefined {
  if (!op?.responses) return undefined;
  const resp = op.responses["200"] ?? op.responses["201"];
  return resp?.content?.["application/json"]?.schema;
}

/**
 * Find the request body schema for an operation.
 */
function getRequestBodySchema(
  op: OperationObject | undefined
): SchemaObject | undefined {
  return op?.requestBody?.content?.["application/json"]?.schema;
}

/**
 * Find the first array-typed property in a response schema.
 * Returns the property key (e.g. "items", "files") and the array element schema.
 */
function findListArrayKey(
  schema: SchemaObject | undefined
): { key: string; itemSchema: SchemaObject | undefined } | null {
  if (!schema?.properties) return null;
  for (const [key, prop] of Object.entries(schema.properties)) {
    if (prop.type === "array" && prop.items) {
      return { key, itemSchema: prop.items };
    }
  }
  return null;
}

/**
 * Check if a path has a resource ID parameter (e.g., /{resource}/{id}).
 * Looks for any segment starting with "{", not just the last one.
 */
function hasIdParam(path: string): boolean {
  const segments = path.split("/").filter(Boolean);
  return segments.length >= 2 && segments.some((s) => s.startsWith("{"));
}

/**
 * Check if a path matches /admin/{resource}/{id}/restore
 */
function isRestorePath(path: string): boolean {
  return /^\/admin\/[^/]+\/\{[^}]+\}\/restore$/.test(path);
}

/**
 * Check if a path matches /admin/{resource} (admin list).
 */
function isAdminListPath(path: string): boolean {
  const segments = path.split("/").filter(Boolean);
  return (
    segments.length === 2 &&
    segments[0] === "admin" &&
    !segments[1].startsWith("{")
  );
}

/**
 * Parse an OpenAPI spec and extract resource information.
 *
 * Resources are identified by tags. For each tag, we scan all paths
 * to find CRUD endpoints and admin endpoints.
 */
export function parseSpec(spec: OpenAPISpec): ResourceInfo[] {
  if (!spec.paths) return [];

  // Collect all tags
  const tagSet = new Set<string>();
  for (const pathItem of Object.values(spec.paths)) {
    for (const op of [
      pathItem.get,
      pathItem.post,
      pathItem.put,
      pathItem.patch,
      pathItem.delete,
    ]) {
      if (op?.tags) {
        for (const tag of op.tags) {
          tagSet.add(tag);
        }
      }
    }
  }

  const resources: ResourceInfo[] = [];

  for (const tag of Array.from(tagSet).sort()) {
    const resource: ResourceInfo = {
      name: tag,
      listPath: null,
      createPath: null,
      getOnePath: null,
      updatePath: null,
      deletePath: null,
      adminListPath: null,
      restorePath: null,
      updateMethod: null,
      canList: false,
      canCreate: false,
      canGetOne: false,
      canUpdate: false,
      canDelete: false,
      canAdminList: false,
      canRestore: false,
      listItemsKey: "items",
      responseFields: [],
      editableFields: [],
      creatableFields: [],
    };

    for (const [path, pathItem] of Object.entries(spec.paths)) {
      const ops: [string, OperationObject | undefined][] = [
        ["get", pathItem.get],
        ["post", pathItem.post],
        ["put", pathItem.put],
        ["patch", pathItem.patch],
        ["delete", pathItem.delete],
      ];

      for (const [method, op] of ops) {
        if (!op?.tags?.includes(tag)) continue;

        // Admin restore: PATCH /admin/{resource}/{id}/restore
        if (method === "patch" && isRestorePath(path)) {
          resource.restorePath = path;
          resource.canRestore = true;
          continue;
        }

        // Admin list: GET /admin/{resource}
        if (method === "get" && isAdminListPath(path)) {
          resource.adminListPath = path;
          resource.canAdminList = true;
          continue;
        }

        // Standard CRUD
        if (method === "get" && !hasIdParam(path)) {
          resource.listPath = path;
          resource.canList = true;

          // Extract response fields from list response items
          const respSchema = getSuccessResponseSchema(op);
          const arrayInfo = findListArrayKey(respSchema);
          if (arrayInfo) {
            resource.listItemsKey = arrayInfo.key;
            resource.responseFields = extractFields(
              arrayInfo.itemSchema,
              true
            );
          }
        } else if (method === "get" && hasIdParam(path)) {
          resource.getOnePath = path;
          resource.canGetOne = true;

          // If we haven't got fields from list, try get-one
          if (resource.responseFields.length === 0) {
            const respSchema = getSuccessResponseSchema(op);
            if (respSchema) {
              resource.responseFields = extractFields(respSchema, true);
            }
          }
        } else if (method === "post" && !hasIdParam(path)) {
          resource.createPath = path;
          resource.canCreate = true;

          const reqSchema = getRequestBodySchema(op);
          if (reqSchema) {
            resource.creatableFields = extractFields(reqSchema, false);
          }
        } else if (
          (method === "patch" || method === "put") &&
          hasIdParam(path)
        ) {
          resource.updatePath = path;
          resource.updateMethod = method.toUpperCase();
          resource.canUpdate = true;

          const reqSchema = getRequestBodySchema(op);
          if (reqSchema) {
            resource.editableFields = extractFields(reqSchema, false);
          }
        } else if (method === "delete" && hasIdParam(path)) {
          resource.deletePath = path;
          resource.canDelete = true;
        }
      }
    }

    // Only include resources that have at least a list capability
    if (resource.canList || resource.canAdminList) {
      resources.push(resource);
    }
  }

  return resources;
}
