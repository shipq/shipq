/**
 * api.ts - Thin fetch wrappers for the admin panel.
 *
 * All functions return typed results and throw on network errors.
 * HTTP errors are returned as { ok: false, status, message } so callers
 * can handle them gracefully.
 *
 * The `basePath` variable holds an optional URL prefix (e.g., "/api") that
 * is prepended to all API calls. This supports deployments behind
 * http.StripPrefix where the server's public URLs differ from the mux paths.
 * It is set once via `setBasePath()` during app initialization.
 */

let basePath = "";

/**
 * setBasePath configures the URL prefix for all subsequent API calls.
 * For example, setBasePath("/api") causes login() to fetch "/api/login".
 */
export function setBasePath(prefix: string) {
  basePath = prefix;
}

/**
 * getBasePath returns the currently configured URL prefix.
 */
export function getBasePath(): string {
  return basePath;
}

export interface ApiOk<T> {
  ok: true;
  data: T;
}

export interface ApiError {
  ok: false;
  status: number;
  message: string;
}

export type ApiResult<T> = ApiOk<T> | ApiError;

async function apiCall<T>(
  url: string,
  init?: RequestInit
): Promise<ApiResult<T>> {
  const resp = await fetch(basePath + url, {
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!resp.ok) {
    let message = resp.statusText;
    try {
      const body = await resp.json();
      if (body.error) message = body.error;
      if (body.message) message = body.message;
    } catch {
      // ignore parse errors
    }
    return { ok: false, status: resp.status, message };
  }
  const data = (await resp.json()) as T;
  return { ok: true, data };
}

// ---- Auth ----

export interface RoleInfo {
  name: string;
  description?: string;
}

export interface MeResponse {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  roles: RoleInfo[];
  organization?: {
    id: string;
    name: string;
    description?: string;
  };
  [key: string]: unknown;
}

export function login(
  email: string,
  password: string
): Promise<ApiResult<unknown>> {
  return apiCall("/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export function logout(): Promise<ApiResult<unknown>> {
  return apiCall("/logout", { method: "DELETE" });
}

export function me(): Promise<ApiResult<MeResponse>> {
  return apiCall<MeResponse>("/me");
}

// ---- CRUD ----

export interface ListResponse {
  [key: string]: unknown;
  next_cursor?: string;
}

export function listResource(
  path: string,
  cursor?: string,
  limit = 20
): Promise<ApiResult<ListResponse>> {
  const params = new URLSearchParams({ limit: String(limit) });
  if (cursor) params.set("cursor", cursor);
  return apiCall<ListResponse>(`${path}?${params}`);
}

export function createResource(
  path: string,
  data: Record<string, unknown>
): Promise<ApiResult<Record<string, unknown>>> {
  return apiCall(path, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export function updateResource(
  path: string,
  id: string,
  data: Record<string, unknown>,
  method: string = "PATCH"
): Promise<ApiResult<Record<string, unknown>>> {
  // path is like /posts, we need /posts/{id}
  const url = `${path}/${encodeURIComponent(id)}`;
  return apiCall(url, {
    method,
    body: JSON.stringify(data),
  });
}

export function deleteResource(
  path: string,
  id: string
): Promise<ApiResult<{ success: boolean }>> {
  const url = `${path}/${encodeURIComponent(id)}`;
  return apiCall(url, { method: "DELETE" });
}

export function restoreResource(
  restorePath: string,
  id: string
): Promise<ApiResult<{ success: boolean }>> {
  // restorePath is like /admin/posts/{id}/restore
  // We need to replace {id} with the actual ID
  const url = restorePath.replace(/\{[^}]+\}/, encodeURIComponent(id));
  return apiCall(url, { method: "PATCH" });
}

// ---- File Upload ----
// File upload utilities (uploadFile, getDownloadUrl) are in shipq-files.ts
