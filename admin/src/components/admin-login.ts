/**
 * <admin-login> - Login form component.
 *
 * POSTs to /login, then checks /me to verify GLOBAL_OWNER role.
 * If not GLOBAL_OWNER, shows an error and logs out.
 */

import { login, me, logout } from "../api.js";

export class AdminLogin extends HTMLElement {
  private _error = "";

  connectedCallback() {
    this._render();
  }

  private _render() {
    this.innerHTML = `
      <div class="login-wrap">
        <div class="login-box">
          <h1>Admin Login</h1>
          ${this._error ? `<div class="error">${this._escapeHtml(this._error)}</div>` : ""}
          <label for="admin-email">Email</label>
          <input id="admin-email" type="email" autocomplete="email" />
          <label for="admin-password">Password</label>
          <input id="admin-password" type="password" autocomplete="current-password" />
          <button id="admin-login-btn">Log in</button>
        </div>
      </div>
    `;

    const btn = this.querySelector("#admin-login-btn") as HTMLButtonElement;
    const emailInput = this.querySelector("#admin-email") as HTMLInputElement;
    const passInput = this.querySelector("#admin-password") as HTMLInputElement;

    const doLogin = async () => {
      this._error = "";
      const email = emailInput.value.trim();
      const password = passInput.value;

      if (!email || !password) {
        this._error = "Email and password are required.";
        this._render();
        return;
      }

      btn.disabled = true;
      btn.textContent = "Logging in...";

      const result = await login(email, password);
      if (!result.ok) {
        this._error = result.message || "Login failed.";
        this._render();
        return;
      }

      // Verify GLOBAL_OWNER role
      const meResult = await me();
      if (!meResult.ok) {
        this._error = "Failed to verify permissions.";
        this._render();
        return;
      }

      if (!meResult.data.roles?.some((r) => r.name === "GLOBAL_OWNER")) {
        await logout();
        this._error = "Access denied. Admin requires GLOBAL_OWNER role.";
        this._render();
        return;
      }

      // Success - notify parent
      const app = this.closest("admin-app") as HTMLElement & {
        handleLogin?: () => void;
      };
      app?.handleLogin?.();
    };

    btn.addEventListener("click", doLogin);
    passInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") doLogin();
    });
  }

  private _escapeHtml(s: string): string {
    const div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }
}
