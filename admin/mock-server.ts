/**
 * Mock server for developing and testing the admin panel UI.
 *
 * Run:
 *   npx tsx admin/mock-server.ts
 *
 * Then open http://localhost:3000/admin
 *
 * Login with: test@test.com / password  (GLOBAL_OWNER)
 * Non-owner:  user@test.com / password  (gets rejected)
 */

import { createServer, IncomingMessage, ServerResponse } from "node:http";
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { randomBytes } from "node:crypto";

const PORT = 3000;
const __dirname = typeof import.meta.dirname === "string"
  ? import.meta.dirname
  : dirname(fileURLToPath(import.meta.url));
const ROOT = join(__dirname, "..");

// ---- In-memory data store ----

interface Record {
  id: string;
  public_id: string;
  [key: string]: unknown;
}

let nextId = 100;
function genId(): string {
  return `mock_${++nextId}`;
}

const tables: { [tableName: string]: Record[] } = {
  posts: [
    {
      id: "post_1",
      public_id: "post_1",
      title: "Hello World",
      body: "This is the first post.",
      published: true,
      created_at: "2025-06-01T10:00:00Z",
      updated_at: "2025-06-01T10:00:00Z",
      deleted_at: null,
    },
    {
      id: "post_2",
      public_id: "post_2",
      title: "Second Post",
      body: "Another post here.",
      published: false,
      created_at: "2025-06-02T12:00:00Z",
      updated_at: "2025-06-02T12:00:00Z",
      deleted_at: null,
    },
    {
      id: "post_3",
      public_id: "post_3",
      title: "Deleted Draft",
      body: "This was soft-deleted.",
      published: false,
      created_at: "2025-05-15T09:00:00Z",
      updated_at: "2025-05-20T14:00:00Z",
      deleted_at: "2025-05-20T14:00:00Z",
    },
  ],
  accounts: [
    {
      id: "acc_1",
      public_id: "acc_1",
      email: "test@test.com",
      display_name: "Admin User",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      deleted_at: null,
    },
    {
      id: "acc_2",
      public_id: "acc_2",
      email: "user@test.com",
      display_name: "Regular User",
      created_at: "2025-02-15T08:30:00Z",
      updated_at: "2025-02-15T08:30:00Z",
      deleted_at: null,
    },
  ],
  tags: [
    {
      id: "tag_1",
      public_id: "tag_1",
      name: "go",
      color: "#00ADD8",
      created_at: "2025-01-10T00:00:00Z",
      updated_at: "2025-01-10T00:00:00Z",
      deleted_at: null,
    },
    {
      id: "tag_2",
      public_id: "tag_2",
      name: "typescript",
      color: "#3178C6",
      created_at: "2025-01-11T00:00:00Z",
      updated_at: "2025-01-11T00:00:00Z",
      deleted_at: null,
    },
    {
      id: "tag_3",
      public_id: "tag_3",
      name: "deprecated",
      color: "#999999",
      created_at: "2025-01-12T00:00:00Z",
      updated_at: "2025-01-12T00:00:00Z",
      deleted_at: "2025-03-01T00:00:00Z",
    },
  ],
  managed_files: [
    {
      id: "file_1",
      public_id: "file_1",
      file_key: "abc123",
      original_name: "report.pdf",
      content_type: "application/pdf",
      size_bytes: 1048576,
      status: "uploaded",
      visibility: "private",
      s3_upload_id: null,
      author_account_id: "acc_1",
      created_at: "2025-06-01T10:00:00Z",
      updated_at: "2025-06-01T10:00:00Z",
      deleted_at: null,
    },
    {
      id: "file_2",
      public_id: "file_2",
      file_key: "def456",
      original_name: "photo.jpg",
      content_type: "image/jpeg",
      size_bytes: 524288,
      status: "uploaded",
      visibility: "public",
      s3_upload_id: null,
      author_account_id: "acc_1",
      created_at: "2025-06-02T12:00:00Z",
      updated_at: "2025-06-02T12:00:00Z",
      deleted_at: null,
    },
  ],
};

// In-memory file storage for mock uploads
const mockFileStorage = new Map<string, Buffer>();

// ---- Session state (cookie-based) ----

const sessions = new Map<string, string>(); // token -> "admin" | "user"

function parseCookie(req: IncomingMessage): string | null {
  const header = req.headers.cookie ?? "";
  for (const part of header.split(";")) {
    const [k, v] = part.trim().split("=");
    if (k === "session") return v ?? null;
  }
  return null;
}

function getUser(req: IncomingMessage): string | null {
  const token = parseCookie(req);
  return token ? sessions.get(token) ?? null : null;
}

// ---- OpenAPI spec ----

function buildOpenAPISpec(): object {
  function makeTablePaths(name: string, fields: string[]) {
    const fieldProps: { [k: string]: object } = {};
    for (const f of fields) {
      if (f.endsWith("_at")) {
        fieldProps[f] = { type: "string", format: "date-time" };
      } else if (f === "published") {
        fieldProps[f] = { type: "boolean" };
      } else {
        fieldProps[f] = { type: "string" };
      }
    }

    // Editable fields = everything except id, created_at, updated_at, deleted_at
    const editableFields: { [k: string]: object } = {};
    const creatableFields: { [k: string]: object } = {};
    for (const f of fields) {
      if (!["id", "created_at", "updated_at", "deleted_at"].includes(f)) {
        editableFields[f] = fieldProps[f];
        creatableFields[f] = fieldProps[f];
      }
    }

    const paths: { [p: string]: object } = {};

    // GET /{name} (list)
    paths[`/${name}`] = {
      get: {
        operationId: `List${name}`,
        tags: [name],
        responses: {
          "200": {
            content: {
              "application/json": {
                schema: {
                  properties: {
                    items: {
                      type: "array",
                      items: { properties: fieldProps, required: ["id"] },
                    },
                    next_cursor: { type: "string" },
                  },
                },
              },
            },
          },
        },
      },
      post: {
        operationId: `Create${name}`,
        tags: [name],
        requestBody: {
          content: {
            "application/json": {
              schema: {
                properties: creatableFields,
                required: Object.keys(creatableFields).slice(0, 1),
              },
            },
          },
        },
        responses: { "201": {} },
      },
    };

    // GET/PATCH/DELETE /{name}/{id}
    paths[`/${name}/{id}`] = {
      get: {
        operationId: `Get${name}`,
        tags: [name],
        responses: {
          "200": {
            content: {
              "application/json": {
                schema: { properties: fieldProps },
              },
            },
          },
        },
      },
      patch: {
        operationId: `Update${name}`,
        tags: [name],
        requestBody: {
          content: {
            "application/json": {
              schema: { properties: editableFields },
            },
          },
        },
        responses: { "200": {} },
      },
      delete: {
        operationId: `Delete${name}`,
        tags: [name],
        responses: { "200": {} },
      },
    };

    // Admin list: GET /admin/{name}
    paths[`/admin/${name}`] = {
      get: {
        operationId: `AdminList${name}`,
        tags: [name],
        responses: { "200": {} },
      },
    };

    // Admin restore: PATCH /admin/{name}/{id}/restore
    paths[`/admin/${name}/{id}/restore`] = {
      patch: {
        operationId: `Undelete${name}`,
        tags: [name],
        responses: { "200": {} },
      },
    };

    return paths;
  }

  let allPaths: { [p: string]: object } = {};

  allPaths = {
    ...allPaths,
    ...makeTablePaths("posts", [
      "id",
      "title",
      "body",
      "published",
      "created_at",
      "updated_at",
      "deleted_at",
    ]),
  };
  allPaths = {
    ...allPaths,
    ...makeTablePaths("accounts", [
      "id",
      "email",
      "display_name",
      "created_at",
      "updated_at",
      "deleted_at",
    ]),
  };
  allPaths = {
    ...allPaths,
    ...makeTablePaths("tags", [
      "id",
      "name",
      "color",
      "created_at",
      "updated_at",
      "deleted_at",
    ]),
  };
  allPaths = {
    ...allPaths,
    ...makeTablePaths("managed_files", [
      "id",
      "file_key",
      "original_name",
      "content_type",
      "size_bytes",
      "status",
      "visibility",
      "s3_upload_id",
      "author_account_id",
      "created_at",
      "updated_at",
      "deleted_at",
    ]),
  };

  return {
    openapi: "3.1.0",
    info: { title: "Mock Admin API", version: "1.0.0" },
    paths: allPaths,
    components: {
      securitySchemes: {
        cookieAuth: { type: "apiKey", in: "cookie", name: "session" },
      },
    },
  };
}

// ---- HTML shell (mirrors admin_gen.go) ----

function adminHTML(): string {
  const css = readFileSync(
    join(ROOT, "codegen", "admingen", "admin_gen.go"),
    "utf-8"
  );
  // Extract CSS from the Go file between backtick-delimited adminCSS constant
  const cssMatch = css.match(/const adminCSS = `([\s\S]*?)`/);
  const adminCSS = cssMatch ? cssMatch[1] : "";

  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>Admin (Dev)</title>
    <style>${adminCSS}</style>
  </head>
  <body>
    <admin-app></admin-app>
    <script src="/admin/app.js"></script>
  </body>
</html>`;
}

// ---- HTTP helpers ----

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve) => {
    let data = "";
    req.on("data", (chunk: Buffer) => (data += chunk.toString()));
    req.on("end", () => resolve(data));
  });
}

function json(res: ServerResponse, status: number, body: unknown) {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(body));
}

function parseUrl(url: string): { path: string; query: URLSearchParams } {
  const [path, qs] = (url ?? "/").split("?");
  return { path, query: new URLSearchParams(qs ?? "") };
}

// ---- Route matching ----

function matchRoute(
  method: string,
  path: string
):
  | { handler: string; params: { [k: string]: string } }
  | null {
  const m = method.toUpperCase();

  // Static routes
  if (m === "GET" && path === "/admin") return { handler: "adminPage", params: {} };
  if (m === "GET" && path === "/admin/app.js") return { handler: "adminJS", params: {} };
  if (m === "GET" && path === "/openapi") return { handler: "openapi", params: {} };
  if (m === "POST" && path === "/login") return { handler: "login", params: {} };
  if (m === "DELETE" && path === "/logout") return { handler: "logout", params: {} };
  if (m === "GET" && path === "/me") return { handler: "me", params: {} };

  // File upload/download routes
  if (m === "POST" && path === "/files/upload-url") return { handler: "fileUploadURL", params: {} };
  if (m === "POST" && path === "/files/complete") return { handler: "fileComplete", params: {} };

  // File download: GET /files/{id}/download
  let fileDownloadMatch = path.match(/^\/files\/([^/]+)\/download$/);
  if (fileDownloadMatch && m === "GET") {
    return { handler: "fileDownload", params: { id: fileDownloadMatch[1] } };
  }

  // Mock S3 PUT endpoint for testing uploads
  let mockS3Match = path.match(/^\/mock-s3\/([^/]+)$/);
  if (mockS3Match && m === "PUT") {
    return { handler: "mockS3Put", params: { key: mockS3Match[1] } };
  }
  if (mockS3Match && m === "GET") {
    return { handler: "mockS3Get", params: { key: mockS3Match[1] } };
  }

  // Admin restore: PATCH /admin/{table}/{id}/restore
  let match = path.match(/^\/admin\/([^/]+)\/([^/]+)\/restore$/);
  if (match && m === "PATCH") {
    return { handler: "restore", params: { table: match[1], id: match[2] } };
  }

  // Admin list: GET /admin/{table}
  match = path.match(/^\/admin\/([^/]+)$/);
  if (match && m === "GET") {
    return { handler: "adminList", params: { table: match[1] } };
  }

  // CRUD: /{table}/{id}
  match = path.match(/^\/([^/]+)\/([^/]+)$/);
  if (match) {
    const table = match[1];
    const id = match[2];
    if (tables[table]) {
      if (m === "GET") return { handler: "getOne", params: { table, id } };
      if (m === "PATCH") return { handler: "update", params: { table, id } };
      if (m === "DELETE") return { handler: "delete", params: { table, id } };
    }
  }

  // CRUD: /{table}
  match = path.match(/^\/([^/]+)$/);
  if (match && tables[match[1]]) {
    const table = match[1];
    if (m === "GET") return { handler: "list", params: { table } };
    if (m === "POST") return { handler: "create", params: { table } };
  }

  return null;
}

// ---- Server ----

const server = createServer(async (req, res) => {
  // CORS for dev
  const origin = req.headers.origin ?? "http://localhost:3000";
  res.setHeader("Access-Control-Allow-Origin", origin);
  res.setHeader("Access-Control-Allow-Credentials", "true");
  res.setHeader("Access-Control-Allow-Methods", "GET,POST,PATCH,PUT,DELETE,OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Content-Type");
  if (req.method === "OPTIONS") { res.writeHead(204); res.end(); return; }

  const { path, query } = parseUrl(req.url ?? "/");
  const route = matchRoute(req.method ?? "GET", path);

  if (!route) {
    json(res, 404, { error: "not found" });
    return;
  }

  const { handler, params } = route;

  try {
    switch (handler) {
      case "adminPage": {
        res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        res.end(adminHTML());
        return;
      }

      case "adminJS": {
        const js = readFileSync(join(ROOT, "assets", "admin.min.js"), "utf-8");
        res.writeHead(200, { "Content-Type": "application/javascript" });
        res.end(js);
        return;
      }

      case "openapi": {
        json(res, 200, buildOpenAPISpec());
        return;
      }

      case "login": {
        const body = JSON.parse(await readBody(req));
        let role: string | null = null;
        if (body.email === "test@test.com" && body.password === "password") {
          role = "admin";
        } else if (body.email === "user@test.com" && body.password === "password") {
          role = "user";
        }
        if (role) {
          const token = randomBytes(16).toString("hex");
          sessions.set(token, role);
          res.setHeader("Set-Cookie", `session=${token}; Path=/; HttpOnly`);
          json(res, 200, { session: token });
        } else {
          json(res, 401, { error: "invalid credentials" });
        }
        return;
      }

      case "logout": {
        const token = parseCookie(req);
        if (token) sessions.delete(token);
        res.setHeader("Set-Cookie", "session=; Path=/; HttpOnly; Max-Age=0");
        json(res, 200, { success: true });
        return;
      }

      case "me": {
        const user = getUser(req);
        if (!user) {
          json(res, 401, { error: "not authenticated" });
          return;
        }
        json(res, 200, {
          id: user === "admin" ? "admin-id" : "user-id",
          email: user === "admin" ? "test@test.com" : "user@test.com",
          first_name: user === "admin" ? "Admin" : "User",
          last_name: "Test",
          roles: user === "admin" ? [{ name: "GLOBAL_OWNER", description: null }] : [],
        });
        return;
      }

      case "list": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const limit = parseInt(query.get("limit") ?? "20", 10);
        // Regular list: exclude deleted
        const active = table.filter((r) => r.deleted_at == null);
        const items = active.slice(0, limit).map((r) => {
          const { public_id: _pid, ...rest } = r;
          return { ...rest, id: r.public_id ?? r.id };
        });
        json(res, 200, { items });
        return;
      }

      case "adminList": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const limit = parseInt(query.get("limit") ?? "20", 10);
        // Admin list: include deleted
        const items = table.slice(0, limit).map((r) => {
          const { public_id: _pid, ...rest } = r;
          return { ...rest, id: r.public_id ?? r.id };
        });
        json(res, 200, { items });
        return;
      }

      case "getOne": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const record = table.find((r) => r.public_id === params.id || r.id === params.id);
        if (!record) { json(res, 404, { error: "not found" }); return; }
        const { public_id: _pid, ...rest } = record;
        json(res, 200, { ...rest, id: record.public_id ?? record.id });
        return;
      }

      case "create": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const body = JSON.parse(await readBody(req));
        const id = genId();
        const now = new Date().toISOString();
        const record: Record = {
          id,
          public_id: id,
          ...body,
          created_at: now,
          updated_at: now,
          deleted_at: null,
        };
        table.push(record);
        const { public_id: _pid, ...rest } = record;
        json(res, 201, { ...rest, id: record.public_id });
        return;
      }

      case "update": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const record = table.find((r) => r.public_id === params.id || r.id === params.id);
        if (!record) { json(res, 404, { error: "not found" }); return; }
        const body = JSON.parse(await readBody(req));
        Object.assign(record, body, { updated_at: new Date().toISOString() });
        const { public_id: _pid, ...rest } = record;
        json(res, 200, { ...rest, id: record.public_id ?? record.id });
        return;
      }

      case "delete": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const record = table.find((r) => r.public_id === params.id || r.id === params.id);
        if (!record) { json(res, 404, { error: "not found" }); return; }
        record.deleted_at = new Date().toISOString();
        json(res, 200, { success: true });
        return;
      }

      case "restore": {
        const table = tables[params.table];
        if (!table) { json(res, 404, { error: "table not found" }); return; }
        const record = table.find((r) => r.public_id === params.id || r.id === params.id);
        if (!record) { json(res, 404, { error: "not found" }); return; }
        record.deleted_at = null;
        json(res, 200, { success: true });
        return;
      }

      case "fileUploadURL": {
        const user = getUser(req);
        if (!user) { json(res, 401, { error: "not authenticated" }); return; }
        const body = JSON.parse(await readBody(req));
        const fileId = genId();
        const fileKey = `mock_key_${fileId}`;
        const now = new Date().toISOString();

        // Create managed_files record with status "pending"
        const fileRecord: Record = {
          id: fileId,
          public_id: fileId,
          file_key: fileKey,
          original_name: body.name ?? "unnamed",
          content_type: body.content_type ?? "application/octet-stream",
          size_bytes: body.size_bytes ?? 0,
          status: "pending",
          visibility: body.visibility ?? "private",
          s3_upload_id: null,
          author_account_id: "acc_1",
          created_at: now,
          updated_at: now,
          deleted_at: null,
        };
        tables.managed_files.push(fileRecord);

        // Return a mock presigned PUT URL pointing to our mock S3 endpoint
        json(res, 200, {
          file_id: fileId,
          method: "PUT",
          upload_url: `http://localhost:${PORT}/mock-s3/${fileKey}`,
        });
        return;
      }

      case "fileComplete": {
        const user = getUser(req);
        if (!user) { json(res, 401, { error: "not authenticated" }); return; }
        const body = JSON.parse(await readBody(req));
        const fileRecord = tables.managed_files.find(
          (r) => r.public_id === body.file_id || r.id === body.file_id
        );
        if (!fileRecord) { json(res, 404, { error: "file not found" }); return; }
        fileRecord.status = "uploaded";
        fileRecord.updated_at = new Date().toISOString();
        json(res, 200, {
          file_id: fileRecord.public_id,
          name: fileRecord.original_name,
          content_type: fileRecord.content_type,
          size_bytes: fileRecord.size_bytes,
        });
        return;
      }

      case "fileDownload": {
        const fileRecord = tables.managed_files.find(
          (r) => (r.public_id === params.id || r.id === params.id) && r.deleted_at == null
        );
        if (!fileRecord || fileRecord.status !== "uploaded") {
          json(res, 404, { error: "file not found" });
          return;
        }
        // Redirect to mock S3 GET URL
        const downloadUrl = `http://localhost:${PORT}/mock-s3/${fileRecord.file_key}`;
        res.writeHead(302, { Location: downloadUrl });
        res.end();
        return;
      }

      case "mockS3Put": {
        // Mock S3: accept PUT and store in memory
        const chunks: Buffer[] = [];
        req.on("data", (chunk: Buffer) => chunks.push(chunk));
        req.on("end", () => {
          mockFileStorage.set(params.key, Buffer.concat(chunks));
          res.writeHead(200, { ETag: `"mock-etag-${params.key}"` });
          res.end();
        });
        return;
      }

      case "mockS3Get": {
        // Mock S3: return stored file
        const data = mockFileStorage.get(params.key);
        if (!data) { json(res, 404, { error: "not found" }); return; }
        res.writeHead(200, {
          "Content-Type": "application/octet-stream",
          "Content-Length": String(data.length),
        });
        res.end(data);
        return;
      }

      default:
        json(res, 404, { error: "not found" });
    }
  } catch (err) {
    console.error("Error handling request:", err);
    json(res, 500, { error: "internal server error" });
  }
});

server.listen(PORT, () => {
  console.log(`\n  Admin mock server running at http://localhost:${PORT}/admin\n`);
  console.log(`  Login:  test@test.com / password`);
  console.log(`  Tables: posts, accounts, tags, managed_files (with sample data + soft-deleted rows)\n`);
});
