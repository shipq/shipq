/**
 * <admin-spreadsheet> - Spreadsheet-style editable data grid for a resource.
 *
 * Attributes:
 *   resource - JSON-serialized ResourceInfo object
 *
 * Behavior:
 *   - Loads data from the admin list endpoint (or regular list if no admin list)
 *   - Renders an HTML table with one column per response field
 *   - Click a cell to edit inline; blur/Enter saves via PATCH
 *   - "+" button appends an empty new row; fill cells and click Save to POST
 *   - Delete/Restore buttons per row
 *   - Client-side search filters visible rows
 *   - Cursor-based pagination with "Load more"
 */

import { ResourceInfo, FieldSchema } from "../openapi.js";
import {
  listResource,
  createResource,
  updateResource,
  deleteResource,
  restoreResource,
} from "../api.js";
import { uploadFile, getDownloadUrl } from "../shipq-files.js";

interface RowData {
  values: Record<string, unknown>;
  isNew: boolean;
  saving: boolean;
  error: string | null;
}

export class AdminSpreadsheet extends HTMLElement {
  private _resource: ResourceInfo | null = null;
  private _rows: RowData[] = [];
  private _nextCursor: string | undefined;
  private _searchQuery = "";
  private _loading = false;
  private _uploading = false;
  private _uploadProgress = 0;
  private _uploadError: string | null = null;

  static get observedAttributes() {
    return ["resource"];
  }

  attributeChangedCallback() {
    const raw = this.getAttribute("resource");
    if (raw) {
      this._resource = JSON.parse(raw) as ResourceInfo;
      this._rows = [];
      this._nextCursor = undefined;
      this._searchQuery = "";
      this._loadData();
    }
  }

  connectedCallback() {
    const raw = this.getAttribute("resource");
    if (raw) {
      this._resource = JSON.parse(raw) as ResourceInfo;
      this._loadData();
    }
  }

  private async _loadData(append = false) {
    if (!this._resource || this._loading) return;
    this._loading = true;
    this._render();

    const listPath =
      this._resource.adminListPath ?? this._resource.listPath;
    if (!listPath) {
      this._loading = false;
      this._render();
      return;
    }

    const result = await listResource(
      listPath,
      append ? this._nextCursor : undefined
    );

    if (result.ok) {
      const key = this._resource.listItemsKey ?? "items";
      const rawItems =
        ((result.data as Record<string, unknown>)[key] as
          | Record<string, unknown>[]
          | undefined) ?? [];
      const newRows: RowData[] = rawItems.map((item) => ({
        values: item,
        isNew: false,
        saving: false,
        error: null,
      }));
      this._rows = append ? [...this._rows, ...newRows] : newRows;
      this._nextCursor = result.data.next_cursor as string | undefined;
    }

    this._loading = false;
    this._render();
  }

  private _getFilteredRows(): RowData[] {
    if (!this._searchQuery) return this._rows;
    const q = this._searchQuery.toLowerCase();
    return this._rows.filter((row) =>
      Object.values(row.values).some((v) =>
        String(v ?? "")
          .toLowerCase()
          .includes(q)
      )
    );
  }

  private _isDeleted(row: RowData): boolean {
    return row.values["deleted_at"] != null;
  }

  private _getRowId(row: RowData): string {
    return String(row.values["id"] ?? "");
  }

  private _getColumns(): FieldSchema[] {
    return this._resource?.responseFields ?? [];
  }

  private _isEditable(field: FieldSchema): boolean {
    if (!this._resource?.canUpdate) return false;
    // Check if field is in the editable fields list
    return this._resource.editableFields.some((f) => f.name === field.name);
  }

  private _isCreatable(fieldName: string): boolean {
    if (!this._resource?.canCreate) return false;
    return this._resource.creatableFields.some((f) => f.name === fieldName);
  }

  private async _handleCellSave(
    row: RowData,
    fieldName: string,
    newValue: unknown
  ) {
    if (!this._resource?.updatePath || !this._resource.updateMethod) return;
    const id = this._getRowId(row);
    if (!id) return;

    row.saving = true;
    this._render();

    // Determine the base path: updatePath is like /posts/{id}, strip the /{id} part
    const basePath = this._resource.updatePath.replace(/\/\{[^}]+\}$/, "");
    const result = await updateResource(
      basePath,
      id,
      { [fieldName]: newValue },
      this._resource.updateMethod
    );

    if (result.ok) {
      // Update local data with response
      Object.assign(row.values, result.data);
      row.error = null;
    } else {
      row.error = result.message;
    }

    row.saving = false;
    this._render();

    // Brief green flash for success
    if (result.ok) {
      setTimeout(() => this._render(), 600);
    }
  }

  private _addNewRow() {
    if (!this._resource) return;
    const values: Record<string, unknown> = {};
    for (const field of this._resource.creatableFields) {
      values[field.name] = "";
    }
    this._rows.push({
      values,
      isNew: true,
      saving: false,
      error: null,
    });
    this._render();
  }

  private async _saveNewRow(row: RowData) {
    if (!this._resource?.createPath) return;

    row.saving = true;
    this._render();

    // Only send creatable fields
    const data: Record<string, unknown> = {};
    for (const field of this._resource.creatableFields) {
      const val = row.values[field.name];
      if (val !== "" && val !== undefined) {
        data[field.name] = this._coerceValue(
          val,
          field.type
        );
      }
    }

    const result = await createResource(this._resource.createPath, data);

    if (result.ok) {
      row.values = result.data;
      row.isNew = false;
      row.error = null;
    } else {
      row.error = result.message;
    }

    row.saving = false;
    this._render();
  }

  private async _deleteRow(row: RowData) {
    if (!this._resource?.deletePath) return;
    const id = this._getRowId(row);
    if (!id) return;

    const basePath = this._resource.deletePath.replace(/\/\{[^}]+\}$/, "");
    const result = await deleteResource(basePath, id);

    if (result.ok) {
      // Mark as deleted locally
      row.values["deleted_at"] = new Date().toISOString();
    }

    this._render();
  }

  private async _restoreRow(row: RowData) {
    if (!this._resource?.restorePath) return;
    const id = this._getRowId(row);
    if (!id) return;

    const result = await restoreResource(this._resource.restorePath, id);

    if (result.ok) {
      row.values["deleted_at"] = null;
    }

    this._render();
  }

  private _isManagedFiles(): boolean {
    return this._resource?.name === "managed_files";
  }

  private _uploadAbort: AbortController | null = null;

  private async _handleFileUpload() {
    const input = document.createElement("input");
    input.type = "file";
    input.addEventListener("change", async () => {
      const file = input.files?.[0];
      if (!file) return;

      this._uploading = true;
      this._uploadProgress = 0;
      this._uploadError = null;
      this._uploadAbort = new AbortController();
      this._render();

      try {
        await uploadFile(file, {
          onProgress: (progress) => {
            this._uploadProgress = progress.percent;
            this._render();
          },
          signal: this._uploadAbort.signal,
        });

        // Success - reload data
        this._uploadProgress = 100;
        this._uploading = false;
        this._uploadError = null;
        this._uploadAbort = null;
        await this._loadData();
      } catch (err) {
        if ((err as Error).name === "AbortError") {
          this._uploadError = "Upload cancelled.";
        } else {
          this._uploadError = String(err);
        }
        this._uploading = false;
        this._uploadAbort = null;
        this._render();
      }
    });
    input.click();
  }

  private _coerceValue(value: unknown, type: FieldSchema["type"]): unknown {
    const s = String(value ?? "");
    switch (type) {
      case "integer":
        return parseInt(s, 10) || 0;
      case "number":
        return parseFloat(s) || 0;
      case "boolean":
        return s === "true" || s === "1";
      default:
        return s;
    }
  }

  _render() {
    if (!this._resource) {
      this.innerHTML = "<p>No resource selected.</p>";
      return;
    }

    const columns = this._getColumns();
    const rows = this._getFilteredRows();
    const hasActions =
      this._resource.canDelete || this._resource.canRestore || this._isManagedFiles();

    this.innerHTML = "";

    // Toolbar
    const toolbar = document.createElement("div");
    toolbar.className = "spreadsheet-toolbar";

    const h2 = document.createElement("h2");
    h2.textContent = this._resource.name;
    toolbar.appendChild(h2);

    // Search
    const search = document.createElement("input");
    search.type = "search";
    search.placeholder = "Filter rows...";
    search.value = this._searchQuery;
    search.addEventListener("input", () => {
      this._searchQuery = search.value;
      this._render();
    });
    toolbar.appendChild(search);

    // Add row button
    if (this._resource.canCreate) {
      const addBtn = document.createElement("button");
      addBtn.className = "btn-add";
      addBtn.textContent = "+ New Row";
      addBtn.addEventListener("click", () => this._addNewRow());
      toolbar.appendChild(addBtn);
    }

    // Upload file button (for managed_files resource)
    if (this._isManagedFiles()) {
      const uploadBtn = document.createElement("button");
      uploadBtn.className = "btn-add";
      uploadBtn.textContent = "Upload File";
      uploadBtn.disabled = this._uploading;
      uploadBtn.addEventListener("click", () => this._handleFileUpload());
      toolbar.appendChild(uploadBtn);
    }

    this.appendChild(toolbar);

    // Upload progress bar
    if (this._isManagedFiles() && (this._uploading || this._uploadError)) {
      const progressContainer = document.createElement("div");
      progressContainer.className = "upload-progress-container";

      if (this._uploading) {
        const bar = document.createElement("div");
        bar.className = "upload-progress-bar";
        const fill = document.createElement("div");
        fill.className = "upload-progress-fill";
        fill.style.width = `${this._uploadProgress}%`;
        bar.appendChild(fill);
        progressContainer.appendChild(bar);

        const labelRow = document.createElement("div");
        labelRow.style.cssText = "display:flex;align-items:center;gap:8px";

        const label = document.createElement("span");
        label.className = "upload-progress-label";
        label.textContent = `Uploading... ${this._uploadProgress}%`;
        labelRow.appendChild(label);

        if (this._uploadAbort) {
          const cancelBtn = document.createElement("button");
          cancelBtn.className = "btn-delete";
          cancelBtn.textContent = "Cancel";
          cancelBtn.style.cssText = "font-size:12px;padding:2px 8px";
          cancelBtn.addEventListener("click", () => {
            this._uploadAbort?.abort();
          });
          labelRow.appendChild(cancelBtn);
        }

        progressContainer.appendChild(labelRow);
      }

      if (this._uploadError) {
        const errorEl = document.createElement("div");
        errorEl.className = "upload-error";
        errorEl.textContent = `Upload error: ${this._uploadError}`;
        progressContainer.appendChild(errorEl);
      }

      this.appendChild(progressContainer);
    }

    // Table
    const table = document.createElement("table");
    table.className = "spreadsheet";

    // Header
    const thead = document.createElement("thead");
    const headerRow = document.createElement("tr");
    for (const col of columns) {
      const th = document.createElement("th");
      th.textContent = col.name;
      headerRow.appendChild(th);
    }
    if (hasActions) {
      const th = document.createElement("th");
      th.textContent = "";
      th.style.width = "80px";
      headerRow.appendChild(th);
    }
    thead.appendChild(headerRow);
    table.appendChild(thead);

    // Body
    const tbody = document.createElement("tbody");

    for (const row of rows) {
      const tr = document.createElement("tr");
      if (this._isDeleted(row)) tr.className = "deleted";
      if (row.isNew) tr.classList.add("new-row");

      for (const col of columns) {
        const td = document.createElement("td");
        const value = row.values[col.name];

        if (row.isNew) {
          // New row: show input for creatable fields
          if (this._isCreatable(col.name)) {
            this._renderEditableCell(td, row, col, value);
          } else {
            td.className = "readonly";
            const display = document.createElement("div");
            display.className = "cell-display";
            display.textContent = String(value ?? "");
            td.appendChild(display);
          }
        } else if (this._isEditable(col) && !this._isDeleted(row)) {
          this._renderClickToEditCell(td, row, col, value);
        } else {
          td.className = "readonly";
          const display = document.createElement("div");
          display.className = "cell-display";
          display.textContent = this._formatValue(value, col.type);
          td.appendChild(display);
        }

        if (row.saving) td.classList.add("saving");
        if (row.error) td.classList.add("error");

        tr.appendChild(td);
      }

      // Actions column
      if (hasActions || row.isNew || this._isManagedFiles()) {
        const actionTd = document.createElement("td");
        actionTd.className = "actions";

        if (row.isNew) {
          const saveBtn = document.createElement("button");
          saveBtn.className = "btn-save";
          saveBtn.textContent = "Save";
          saveBtn.addEventListener("click", () => this._saveNewRow(row));
          actionTd.appendChild(saveBtn);
        } else {
          // Download button for managed_files with status "uploaded"
          if (this._isManagedFiles() && row.values["status"] === "uploaded" && !this._isDeleted(row)) {
            const downloadBtn = document.createElement("button");
            downloadBtn.textContent = "Download";
            downloadBtn.className = "btn-download";
            downloadBtn.addEventListener("click", async () => {
              downloadBtn.disabled = true;
              downloadBtn.textContent = "Loading…";
              try {
                const resp = await fetch(getDownloadUrl(this._getRowId(row)), {
                  credentials: "include",
                });
                if (!resp.ok) {
                  const body = await resp.text();
                  throw new Error(`${resp.status} ${body}`);
                }
                const data = await resp.json();
                window.location.href = data.download_url;
              } catch (e) {
                alert(`Download failed: ${e instanceof Error ? e.message : e}`);
              } finally {
                downloadBtn.disabled = false;
                downloadBtn.textContent = "Download";
              }
            });
            actionTd.appendChild(downloadBtn);
            actionTd.appendChild(document.createTextNode(" "));
          }

          if (this._isDeleted(row) && this._resource!.canRestore) {
            const restoreBtn = document.createElement("button");
            restoreBtn.className = "btn-restore";
            restoreBtn.textContent = "Restore";
            restoreBtn.addEventListener("click", () => this._restoreRow(row));
            actionTd.appendChild(restoreBtn);
          } else if (!this._isDeleted(row) && this._resource!.canDelete) {
            const deleteBtn = document.createElement("button");
            deleteBtn.className = "btn-delete";
            deleteBtn.textContent = "Delete";
            deleteBtn.addEventListener("click", () => this._deleteRow(row));
            actionTd.appendChild(deleteBtn);
          }
        }

        tr.appendChild(actionTd);
      }

      tbody.appendChild(tr);
    }

    table.appendChild(tbody);
    this.appendChild(table);

    // Status bar
    const status = document.createElement("div");
    status.className = "status-bar";
    status.textContent = this._loading
      ? "Loading..."
      : `${rows.length} row${rows.length !== 1 ? "s" : ""} displayed`;
    this.appendChild(status);

    // Load more
    if (this._nextCursor) {
      const loadMore = document.createElement("div");
      loadMore.className = "load-more";
      const btn = document.createElement("button");
      btn.textContent = "Load more...";
      btn.addEventListener("click", () => this._loadData(true));
      loadMore.appendChild(btn);
      this.appendChild(loadMore);
    }
  }

  /**
   * Render a click-to-edit cell for existing rows.
   */
  private _renderClickToEditCell(
    td: HTMLTableCellElement,
    row: RowData,
    field: FieldSchema,
    value: unknown
  ) {
    const display = document.createElement("div");
    display.className = "cell-display";
    display.textContent = this._formatValue(value, field.type);

    display.addEventListener("click", () => {
      td.innerHTML = "";
      const input = this._createInput(field, value);
      td.appendChild(input);
      input.focus();

      const save = () => {
        const newVal = this._getInputValue(input, field);
        if (newVal !== value) {
          this._handleCellSave(row, field.name, newVal);
        } else {
          this._render();
        }
      };

      input.addEventListener("blur", save);
      input.addEventListener("keydown", (e: Event) => {
        const ke = e as KeyboardEvent;
        if (ke.key === "Enter") {
          e.preventDefault();
          input.blur();
        }
        if (ke.key === "Escape") {
          this._render();
        }
      });
    });

    td.appendChild(display);
  }

  /**
   * Render an always-editable cell for new rows.
   */
  private _renderEditableCell(
    td: HTMLTableCellElement,
    row: RowData,
    field: FieldSchema,
    value: unknown
  ) {
    const input = this._createInput(field, value);
    input.addEventListener("input", () => {
      row.values[field.name] = this._getInputValue(input, field);
    });
    input.addEventListener("keydown", (e: Event) => {
      if ((e as KeyboardEvent).key === "Enter") {
        e.preventDefault();
        this._saveNewRow(row);
      }
    });
    td.appendChild(input);
  }

  private _createInput(
    field: FieldSchema,
    value: unknown
  ): HTMLInputElement | HTMLSelectElement {
    if (field.type === "boolean") {
      const select = document.createElement("select");
      const optTrue = document.createElement("option");
      optTrue.value = "true";
      optTrue.textContent = "true";
      const optFalse = document.createElement("option");
      optFalse.value = "false";
      optFalse.textContent = "false";
      select.appendChild(optTrue);
      select.appendChild(optFalse);
      select.value = String(value ?? "false");
      return select;
    }

    const input = document.createElement("input");
    switch (field.type) {
      case "integer":
      case "number":
        input.type = "number";
        break;
      case "datetime":
        input.type = "datetime-local";
        break;
      default:
        input.type = "text";
    }
    input.value = String(value ?? "");
    return input;
  }

  private _getInputValue(
    input: HTMLInputElement | HTMLSelectElement,
    field: FieldSchema
  ): unknown {
    return this._coerceValue(input.value, field.type);
  }

  private _formatValue(value: unknown, type: FieldSchema["type"]): string {
    if (value === null || value === undefined) return "";
    if (type === "boolean") return value ? "true" : "false";
    return String(value);
  }
}
