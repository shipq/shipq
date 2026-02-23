/**
 * Admin SPA entry point.
 *
 * Registers all web components and bootstraps the app.
 */

import { AdminApp } from "./components/admin-app.js";
import { AdminLogin } from "./components/admin-login.js";
import { AdminNav } from "./components/admin-nav.js";
import { AdminSpreadsheet } from "./components/admin-spreadsheet.js";

customElements.define("admin-app", AdminApp);
customElements.define("admin-login", AdminLogin);
customElements.define("admin-nav", AdminNav);
customElements.define("admin-spreadsheet", AdminSpreadsheet);
