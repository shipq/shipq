/**
 * <admin-nav> - Sidebar navigation listing all resources.
 *
 * Attributes:
 *   resources - JSON array of resource names
 *   active    - currently active resource name
 */

export class AdminNav extends HTMLElement {
  static get observedAttributes() {
    return ["resources", "active"];
  }

  attributeChangedCallback() {
    this._render();
  }

  connectedCallback() {
    this._render();
  }

  private _render() {
    const resourceNames: string[] = JSON.parse(
      this.getAttribute("resources") ?? "[]"
    );
    const active = this.getAttribute("active") ?? "";

    this.className = "admin-sidebar";
    this.innerHTML = `<h2>Admin</h2>`;

    const homeLink = document.createElement("a");
    homeLink.href = "#/tables";
    homeLink.textContent = "All Tables";
    if (!active) homeLink.className = "active";
    this.appendChild(homeLink);

    for (const name of resourceNames) {
      const a = document.createElement("a");
      a.href = `#/tables/${encodeURIComponent(name)}`;
      a.textContent = name;
      if (name === active) a.className = "active";
      this.appendChild(a);
    }
  }
}
