/**
 * router.ts - Minimal hash-based router for the admin SPA.
 *
 * Routes:
 *   #/login          - Login form
 *   #/tables         - Table list (home)
 *   #/tables/{name}  - Spreadsheet view for a resource
 */

export interface Route {
  page: "login" | "tables" | "spreadsheet";
  /** Resource name, only set when page === "spreadsheet" */
  resource?: string;
}

/**
 * Parse the current hash into a Route.
 */
export function parseHash(hash: string): Route {
  const path = hash.replace(/^#\/?/, "");
  if (!path || path === "login") {
    return { page: "login" };
  }
  if (path === "tables") {
    return { page: "tables" };
  }
  const match = path.match(/^tables\/(.+)$/);
  if (match) {
    return { page: "spreadsheet", resource: decodeURIComponent(match[1]) };
  }
  // Default to login for unknown routes
  return { page: "login" };
}

/**
 * Navigate to a route by updating the hash.
 */
export function navigate(route: Route): void {
  switch (route.page) {
    case "login":
      location.hash = "#/login";
      break;
    case "tables":
      location.hash = "#/tables";
      break;
    case "spreadsheet":
      location.hash = `#/tables/${encodeURIComponent(route.resource ?? "")}`;
      break;
  }
}

/**
 * Get the current route.
 */
export function currentRoute(): Route {
  return parseHash(location.hash);
}

export type RouteChangeCallback = (route: Route) => void;

/**
 * Listen for hash changes and call the callback with the new route.
 * Returns an unsubscribe function.
 */
export function onRouteChange(cb: RouteChangeCallback): () => void {
  const handler = () => cb(parseHash(location.hash));
  window.addEventListener("hashchange", handler);
  return () => window.removeEventListener("hashchange", handler);
}
