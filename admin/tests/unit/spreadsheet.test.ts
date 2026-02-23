import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { AdminSpreadsheet } from "../../src/components/admin-spreadsheet.js";
import type { ResourceInfo } from "../../src/openapi.js";

// Register the custom element once
beforeAll(() => {
  if (!customElements.get("admin-spreadsheet")) {
    customElements.define("admin-spreadsheet", AdminSpreadsheet);
  }
});

function makeResource(overrides: Partial<ResourceInfo> = {}): ResourceInfo {
  return {
    name: "posts",
    listPath: "/posts",
    createPath: "/posts",
    getOnePath: "/posts/{id}",
    updatePath: "/posts/{id}",
    deletePath: "/posts/{id}",
    adminListPath: null,
    restorePath: null,
    updateMethod: "PATCH",
    canList: true,
    canCreate: true,
    canGetOne: true,
    canUpdate: true,
    canDelete: true,
    canAdminList: false,
    canRestore: false,
    listItemsKey: "items",
    responseFields: [
      { name: "id", type: "string", required: true, readonly: true },
      { name: "title", type: "string", required: true, readonly: false },
      {
        name: "created_at",
        type: "datetime",
        required: false,
        readonly: true,
      },
    ],
    editableFields: [
      { name: "title", type: "string", required: false, readonly: false },
    ],
    creatableFields: [
      { name: "title", type: "string", required: true, readonly: false },
    ],
    ...overrides,
  };
}

function mockListResponse(
  items: Record<string, unknown>[],
  nextCursor?: string
) {
  globalThis.fetch = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    statusText: "OK",
    json: () => Promise.resolve({ items, next_cursor: nextCursor }),
  });
}

describe("AdminSpreadsheet", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("creates without error", () => {
    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    expect(el).toBeTruthy();
  });

  it("renders column headers from resource response fields", async () => {
    mockListResponse([]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    // Wait for async data load
    await new Promise((r) => setTimeout(r, 50));

    const headers = el.querySelectorAll("table.spreadsheet th");
    expect(headers.length).toBeGreaterThanOrEqual(3);
    expect(headers[0].textContent).toBe("id");
    expect(headers[1].textContent).toBe("title");
    expect(headers[2].textContent).toBe("created_at");

    document.body.removeChild(el);
  });

  it("renders rows from list response", async () => {
    mockListResponse([
      { id: "abc", title: "First Post", created_at: "2025-01-01T00:00:00Z" },
      { id: "def", title: "Second Post", created_at: "2025-01-02T00:00:00Z" },
    ]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const rows = el.querySelectorAll("table.spreadsheet tbody tr");
    expect(rows.length).toBe(2);

    // Check readonly cells have cell-display
    const firstRowCells = rows[0].querySelectorAll("td");
    expect(firstRowCells[0].classList.contains("readonly")).toBe(true);
    expect(firstRowCells[0].querySelector(".cell-display")?.textContent).toBe(
      "abc"
    );

    document.body.removeChild(el);
  });

  it("shows search input and filters rows", async () => {
    mockListResponse([
      { id: "1", title: "Alpha", created_at: "" },
      { id: "2", title: "Beta", created_at: "" },
    ]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const search = el.querySelector(
      'input[type="search"]'
    ) as HTMLInputElement;
    expect(search).toBeTruthy();

    // Type in search
    search.value = "Alpha";
    search.dispatchEvent(new Event("input"));

    await new Promise((r) => setTimeout(r, 50));

    const rows = el.querySelectorAll("table.spreadsheet tbody tr");
    expect(rows.length).toBe(1);

    document.body.removeChild(el);
  });

  it("shows + New Row button when canCreate is true", async () => {
    mockListResponse([]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const addBtn = el.querySelector(".btn-add") as HTMLButtonElement;
    expect(addBtn).toBeTruthy();
    expect(addBtn.textContent).toBe("+ New Row");

    document.body.removeChild(el);
  });

  it("does not show + New Row when canCreate is false", async () => {
    mockListResponse([]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute(
      "resource",
      JSON.stringify(makeResource({ canCreate: false, createPath: null }))
    );
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const addBtn = el.querySelector(".btn-add");
    expect(addBtn).toBeNull();

    document.body.removeChild(el);
  });

  it("appends empty row when + New Row is clicked", async () => {
    mockListResponse([{ id: "1", title: "Existing", created_at: "" }]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const addBtn = el.querySelector(".btn-add") as HTMLButtonElement;
    addBtn.click();

    await new Promise((r) => setTimeout(r, 50));

    const rows = el.querySelectorAll("table.spreadsheet tbody tr");
    expect(rows.length).toBe(2);

    // New row should have new-row class
    expect(rows[1].classList.contains("new-row")).toBe(true);

    document.body.removeChild(el);
  });

  it("shows Delete button for non-deleted rows when canDelete", async () => {
    mockListResponse([{ id: "1", title: "Active", created_at: "" }]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const deleteBtn = el.querySelector(".btn-delete") as HTMLButtonElement;
    expect(deleteBtn).toBeTruthy();
    expect(deleteBtn.textContent).toBe("Delete");

    document.body.removeChild(el);
  });

  it("shows Restore button for deleted rows when canRestore", async () => {
    mockListResponse([
      {
        id: "1",
        title: "Deleted Item",
        created_at: "",
        deleted_at: "2025-01-01T00:00:00Z",
      },
    ]);

    const resource = makeResource({
      canRestore: true,
      restorePath: "/admin/posts/{id}/restore",
      adminListPath: "/admin/posts",
      canAdminList: true,
    });

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(resource));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const row = el.querySelector("table.spreadsheet tbody tr");
    expect(row?.classList.contains("deleted")).toBe(true);

    const restoreBtn = el.querySelector(".btn-restore") as HTMLButtonElement;
    expect(restoreBtn).toBeTruthy();
    expect(restoreBtn.textContent).toBe("Restore");

    document.body.removeChild(el);
  });

  it("shows Load more button when next_cursor is present", async () => {
    mockListResponse([{ id: "1", title: "Post", created_at: "" }], "cursor123");

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const loadMore = el.querySelector(".load-more button") as HTMLButtonElement;
    expect(loadMore).toBeTruthy();
    expect(loadMore.textContent).toBe("Load more...");

    document.body.removeChild(el);
  });

  it("does not show Load more when no next_cursor", async () => {
    mockListResponse([{ id: "1", title: "Post", created_at: "" }]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const loadMore = el.querySelector(".load-more button");
    expect(loadMore).toBeNull();

    document.body.removeChild(el);
  });

  it("makes editable cells clickable to open input", async () => {
    mockListResponse([
      { id: "1", title: "Editable Post", created_at: "" },
    ]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    // Title cell (index 1) should be editable
    const titleCell = el.querySelectorAll("table.spreadsheet tbody td")[1];
    const display = titleCell.querySelector(".cell-display") as HTMLElement;
    expect(display).toBeTruthy();
    expect(display.textContent).toBe("Editable Post");

    // Click should turn it into an input
    display.click();
    await new Promise((r) => setTimeout(r, 10));

    const input = titleCell.querySelector("input") as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.value).toBe("Editable Post");

    document.body.removeChild(el);
  });

  it("read-only cells are not clickable to edit", async () => {
    mockListResponse([{ id: "1", title: "Post", created_at: "" }]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    // ID cell (index 0) should be readonly
    const idCell = el.querySelectorAll("table.spreadsheet tbody td")[0];
    expect(idCell.classList.contains("readonly")).toBe(true);

    // Clicking should not create an input
    const display = idCell.querySelector(".cell-display") as HTMLElement;
    display.click();
    await new Promise((r) => setTimeout(r, 10));

    const input = idCell.querySelector("input");
    expect(input).toBeNull();

    document.body.removeChild(el);
  });

  it("shows status bar with row count", async () => {
    mockListResponse([
      { id: "1", title: "A", created_at: "" },
      { id: "2", title: "B", created_at: "" },
    ]);

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(makeResource()));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 50));

    const status = el.querySelector(".status-bar") as HTMLElement;
    expect(status.textContent).toBe("2 rows displayed");

    document.body.removeChild(el);
  });

  it("download button fetches presigned URL and redirects (no new tab)", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          items: [
            {
              id: "file-abc",
              original_name: "photo.jpg",
              status: "uploaded",
              visibility: "private",
            },
          ],
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          download_url: "https://s3.example.com/presigned-get?token=abc",
          name: "photo.jpg",
          content_type: "image/jpeg",
          size_bytes: 12345,
        }),
      });
    globalThis.fetch = fetchMock;

    const filesResource = makeResource({
      name: "managed_files",
      listItemsKey: "items",
      responseFields: [
        { name: "id", type: "string", required: true, readonly: true },
        { name: "original_name", type: "string", required: true, readonly: true },
        { name: "status", type: "string", required: true, readonly: true },
        { name: "visibility", type: "string", required: true, readonly: true },
      ],
    });

    const el = document.createElement("admin-spreadsheet") as AdminSpreadsheet;
    el.setAttribute("resource", JSON.stringify(filesResource));
    document.body.appendChild(el);

    await new Promise((r) => setTimeout(r, 100));

    const downloadBtn = el.querySelector(".btn-download") as HTMLButtonElement;
    expect(downloadBtn).toBeTruthy();
    expect(downloadBtn.textContent).toBe("Download");

    // Mock window.location.href setter
    const hrefSetter = vi.fn();
    const origLocation = window.location;
    Object.defineProperty(window, "location", {
      value: { ...origLocation, set href(v: string) { hrefSetter(v); } },
      writable: true,
      configurable: true,
    });

    downloadBtn.click();
    await new Promise((r) => setTimeout(r, 100));

    // The second fetch call should be to /files/file-abc/download with credentials
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const downloadCall = fetchMock.mock.calls[1];
    expect(downloadCall[0]).toContain("/files/file-abc/download");
    expect(downloadCall[1]?.credentials).toBe("include");

    // Restore
    Object.defineProperty(window, "location", {
      value: origLocation,
      writable: true,
      configurable: true,
    });

    document.body.removeChild(el);
  });
});
