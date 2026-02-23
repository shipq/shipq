import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  login,
  me,
  logout,
  listResource,
  createResource,
  updateResource,
  deleteResource,
  restoreResource,
} from "../../src/api.js";

function mockFetch(response: {
  ok?: boolean;
  status?: number;
  statusText?: string;
  json?: unknown;
}) {
  return vi.fn().mockResolvedValue({
    ok: response.ok ?? true,
    status: response.status ?? 200,
    statusText: response.statusText ?? "OK",
    json: () => Promise.resolve(response.json ?? {}),
  });
}

describe("api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe("login", () => {
    it("sends POST /login with email and password", async () => {
      const fetchMock = mockFetch({ ok: true, json: { session: "abc" } });
      globalThis.fetch = fetchMock;

      const result = await login("admin@test.com", "password123");

      expect(result.ok).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith("/login", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        method: "POST",
        body: JSON.stringify({ email: "admin@test.com", password: "password123" }),
      });
    });

    it("returns error for failed login", async () => {
      globalThis.fetch = mockFetch({
        ok: false,
        status: 401,
        statusText: "Unauthorized",
        json: { error: "invalid credentials" },
      });

      const result = await login("bad@test.com", "wrong");
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.status).toBe(401);
        expect(result.message).toBe("invalid credentials");
      }
    });
  });

  describe("me", () => {
    it("sends GET /me", async () => {
      const fetchMock = mockFetch({
        ok: true,
        json: {
          id: "abc123",
          email: "admin@test.com",
          first_name: "Admin",
          last_name: "User",
          roles: [{ name: "GLOBAL_OWNER" }],
        },
      });
      globalThis.fetch = fetchMock;

      const result = await me();
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.data.email).toBe("admin@test.com");
        expect(result.data.roles).toEqual([{ name: "GLOBAL_OWNER" }]);
      }
    });
  });

  describe("logout", () => {
    it("sends DELETE /logout", async () => {
      const fetchMock = mockFetch({ ok: true });
      globalThis.fetch = fetchMock;

      await logout();

      expect(fetchMock).toHaveBeenCalledWith("/logout", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        method: "DELETE",
      });
    });
  });

  describe("listResource", () => {
    it("sends GET with limit and cursor params", async () => {
      const fetchMock = mockFetch({
        ok: true,
        json: { items: [{ id: "1" }], next_cursor: "abc" },
      });
      globalThis.fetch = fetchMock;

      const result = await listResource("/posts", "prev_cursor", 50);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.data.items).toHaveLength(1);
        expect(result.data.next_cursor).toBe("abc");
      }
      expect(fetchMock).toHaveBeenCalledWith(
        "/posts?limit=50&cursor=prev_cursor",
        expect.objectContaining({ credentials: "include" })
      );
    });

    it("omits cursor when not provided", async () => {
      const fetchMock = mockFetch({
        ok: true,
        json: { items: [] },
      });
      globalThis.fetch = fetchMock;

      await listResource("/posts");
      expect(fetchMock).toHaveBeenCalledWith(
        "/posts?limit=20",
        expect.anything()
      );
    });
  });

  describe("createResource", () => {
    it("sends POST with body", async () => {
      const fetchMock = mockFetch({
        ok: true,
        status: 201,
        json: { id: "new-1", title: "Hello" },
      });
      globalThis.fetch = fetchMock;

      const result = await createResource("/posts", { title: "Hello" });
      expect(result.ok).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith("/posts", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        method: "POST",
        body: JSON.stringify({ title: "Hello" }),
      });
    });
  });

  describe("updateResource", () => {
    it("sends PATCH to /{resource}/{id}", async () => {
      const fetchMock = mockFetch({
        ok: true,
        json: { id: "abc", title: "Updated" },
      });
      globalThis.fetch = fetchMock;

      const result = await updateResource("/posts", "abc", { title: "Updated" });
      expect(result.ok).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith("/posts/abc", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        method: "PATCH",
        body: JSON.stringify({ title: "Updated" }),
      });
    });

    it("supports PUT method", async () => {
      const fetchMock = mockFetch({ ok: true, json: {} });
      globalThis.fetch = fetchMock;

      await updateResource("/posts", "abc", { title: "X" }, "PUT");
      expect(fetchMock).toHaveBeenCalledWith(
        "/posts/abc",
        expect.objectContaining({ method: "PUT" })
      );
    });
  });

  describe("deleteResource", () => {
    it("sends DELETE to /{resource}/{id}", async () => {
      const fetchMock = mockFetch({
        ok: true,
        json: { success: true },
      });
      globalThis.fetch = fetchMock;

      const result = await deleteResource("/posts", "abc");
      expect(result.ok).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith("/posts/abc", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        method: "DELETE",
      });
    });
  });

  describe("restoreResource", () => {
    it("sends PATCH to restore path with ID substituted", async () => {
      const fetchMock = mockFetch({
        ok: true,
        json: { success: true },
      });
      globalThis.fetch = fetchMock;

      const result = await restoreResource(
        "/admin/posts/{id}/restore",
        "abc"
      );
      expect(result.ok).toBe(true);
      expect(fetchMock).toHaveBeenCalledWith(
        "/admin/posts/abc/restore",
        expect.objectContaining({ method: "PATCH" })
      );
    });
  });
});
