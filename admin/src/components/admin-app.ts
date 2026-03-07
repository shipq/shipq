/**
 * <admin-app> - Root component for the admin SPA.
 *
 * Manages auth state, loads OpenAPI spec, routes between pages.
 */

import { me, logout, setBasePath, getBasePath } from "../api.js";
import { parseSpec, ResourceInfo, OpenAPISpec } from "../openapi.js";
import { currentRoute, onRouteChange, navigate, Route } from "../router.js";

export class AdminApp extends HTMLElement {
  private _authenticated = false;
  private _resources: ResourceInfo[] = [];
  private _route: Route = { page: "login" };
  private _unsubRoute: (() => void) | null = null;

  connectedCallback() {
    // Read optional URL prefix from the HTML element (set by the Go codegen
    // when [server] strip_prefix is configured, e.g. "/api").
    const base = this.getAttribute("data-base-path") ?? "";
    if (base) {
      setBasePath(base);
    }

    this._route = currentRoute();
    this._unsubRoute = onRouteChange((route) => {
      this._route = route;
      this._render();
    });
    this._checkAuth();
  }

  disconnectedCallback() {
    this._unsubRoute?.();
  }

  get resources(): ResourceInfo[] {
    return this._resources;
  }

  private async _checkAuth() {
    const result = await me();
    if (result.ok && result.data.roles?.some((r) => r.name === "GLOBAL_OWNER")) {
      this._authenticated = true;
      await this._loadSpec();
      if (this._route.page === "login") {
        navigate({ page: "tables" });
        this._render();
        return;
      }
    } else {
      this._authenticated = false;
      if (this._route.page !== "login") {
        navigate({ page: "login" });
      }
    }
    this._render();
  }

  private async _loadSpec() {
    try {
      const resp = await fetch(getBasePath() + "/openapi", { credentials: "include" });
      if (resp.ok) {
        const spec = (await resp.json()) as OpenAPISpec;
        this._resources = parseSpec(spec);
      }
    } catch {
      // spec unavailable - admin will show empty table list
    }
  }

  async handleLogin() {
    await this._checkAuth();
  }

  async handleLogout() {
    await logout();
    this._authenticated = false;
    this._resources = [];
    navigate({ page: "login" });
  }

  private _render() {
    this.innerHTML = "";

    if (!this._authenticated || this._route.page === "login") {
      const loginEl = document.createElement("admin-login");
      this.appendChild(loginEl);
      return;
    }

    // Authenticated layout: sidebar + main
    const layout = document.createElement("div");
    layout.className = "admin-layout";

    // Sidebar
    const nav = document.createElement("admin-nav") as HTMLElement;
    nav.setAttribute(
      "resources",
      JSON.stringify(this._resources.map((r) => r.name))
    );
    nav.setAttribute("active", this._route.resource ?? "");
    layout.appendChild(nav);

    // Main content
    const main = document.createElement("div");
    main.className = "admin-main";

    if (this._route.page === "tables") {
      main.innerHTML = `<h2 style="margin-bottom:16px">Tables</h2>`;
      const list = document.createElement("ul");
      list.className = "table-list";
      for (const r of this._resources) {
        const li = document.createElement("li");
        const a = document.createElement("a");
        a.href = `#/tables/${encodeURIComponent(r.name)}`;
        a.textContent = r.name;
        li.appendChild(a);
        list.appendChild(li);
      }
      main.appendChild(list);
    } else if (this._route.page === "spreadsheet" && this._route.resource) {
      const resource = this._resources.find(
        (r) => r.name === this._route.resource
      );
      if (resource) {
        const sheet = document.createElement(
          "admin-spreadsheet"
        ) as HTMLElement;
        sheet.setAttribute("resource", JSON.stringify(resource));
        main.appendChild(sheet);
      } else {
        main.innerHTML = `<p>Resource "${this._route.resource}" not found.</p>`;
      }
    }

    // Logout button in the header area of main
    const header = document.createElement("div");
    header.style.cssText =
      "display:flex;justify-content:flex-end;margin-bottom:12px";
    const logoutBtn = document.createElement("button");
    logoutBtn.textContent = "Logout";
    logoutBtn.style.cssText =
      "padding:6px 14px;font-size:13px;border:1px solid #d1d5db;border-radius:4px;background:#fff;cursor:pointer";
    logoutBtn.addEventListener("click", () => this.handleLogout());
    header.appendChild(logoutBtn);
    main.insertBefore(header, main.firstChild);

    layout.appendChild(main);
    this.appendChild(layout);
  }
}
