// Package admingen generates the admin panel HTML page for shipq projects.
// The admin panel is a single-page application using browser-native web components
// that reads the OpenAPI spec at runtime to dynamically render CRUD interfaces.
package admingen

// GenerateAdminHTML returns the HTML shell for the admin SPA.
// It loads the admin JavaScript from /admin/app.js.
func GenerateAdminHTML(title string) string {
	if title == "" {
		title = "Admin"
	}
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>` + title + `</title>
    <style>` + adminCSS + `</style>
  </head>
  <body>
    <admin-app></admin-app>
    <script src="/admin/app.js"></script>
  </body>
</html>`
}

const adminCSS = `
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #1a1a1a; }

/* Layout */
.admin-layout { display: flex; min-height: 100vh; }
.admin-sidebar { width: 220px; background: #1e293b; color: #e2e8f0; padding: 16px 0; flex-shrink: 0; }
.admin-sidebar h2 { font-size: 15px; padding: 0 16px 12px; border-bottom: 1px solid #334155; margin-bottom: 8px; font-weight: 600; letter-spacing: 0.03em; }
.admin-sidebar a { display: block; padding: 6px 16px; color: #94a3b8; text-decoration: none; font-size: 13px; }
.admin-sidebar a:hover, .admin-sidebar a.active { color: #fff; background: #334155; }
.admin-main { flex: 1; padding: 24px; overflow-x: auto; }

/* Login */
.login-wrap { display: flex; align-items: center; justify-content: center; min-height: 100vh; }
.login-box { background: #fff; padding: 32px; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); width: 340px; }
.login-box h1 { font-size: 20px; margin-bottom: 20px; }
.login-box label { display: block; font-size: 13px; font-weight: 500; margin-bottom: 4px; color: #475569; }
.login-box input { width: 100%; padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 14px; margin-bottom: 14px; }
.login-box button { width: 100%; padding: 9px; background: #2563eb; color: #fff; border: none; border-radius: 4px; font-size: 14px; cursor: pointer; }
.login-box button:hover { background: #1d4ed8; }
.login-box .error { color: #dc2626; font-size: 13px; margin-bottom: 10px; }

/* Table list (home) */
.table-list { list-style: none; }
.table-list li { margin-bottom: 4px; }
.table-list a { display: inline-block; padding: 8px 14px; background: #fff; border: 1px solid #e2e8f0; border-radius: 4px; text-decoration: none; color: #1e293b; font-size: 14px; }
.table-list a:hover { background: #f1f5f9; }

/* Spreadsheet */
.spreadsheet-toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.spreadsheet-toolbar h2 { font-size: 18px; font-weight: 600; }
.spreadsheet-toolbar input[type="search"] { padding: 6px 10px; border: 1px solid #d1d5db; border-radius: 4px; font-size: 13px; width: 220px; }
.spreadsheet-toolbar button { padding: 6px 14px; font-size: 13px; border: 1px solid #d1d5db; border-radius: 4px; background: #fff; cursor: pointer; }
.spreadsheet-toolbar button:hover { background: #f1f5f9; }
.spreadsheet-toolbar .btn-add { background: #2563eb; color: #fff; border-color: #2563eb; }
.spreadsheet-toolbar .btn-add:hover { background: #1d4ed8; }

table.spreadsheet { width: 100%; border-collapse: collapse; background: #fff; font-size: 13px; font-family: "SF Mono", "Fira Code", "Fira Mono", Menlo, Consolas, monospace; table-layout: auto; }
table.spreadsheet th { background: #f8fafc; padding: 6px 10px; border: 1px solid #e2e8f0; font-weight: 600; text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: #64748b; white-space: nowrap; position: sticky; top: 0; z-index: 1; }
table.spreadsheet td { padding: 0; border: 1px solid #e2e8f0; height: 32px; vertical-align: middle; max-width: 300px; overflow: hidden; text-overflow: ellipsis; }
table.spreadsheet td .cell-display { padding: 4px 10px; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-height: 24px; }
table.spreadsheet td .cell-display:hover { background: #eff6ff; }
table.spreadsheet td.readonly .cell-display { cursor: default; color: #64748b; }
table.spreadsheet td.readonly .cell-display:hover { background: transparent; }
table.spreadsheet td input, table.spreadsheet td select { width: 100%; height: 100%; padding: 4px 10px; border: 2px solid #2563eb; border-radius: 0; font-size: 13px; font-family: inherit; outline: none; background: #fff; box-sizing: border-box; }
table.spreadsheet td.saving { background: #dcfce7; transition: background 0.3s; }
table.spreadsheet td.error { background: #fef2f2; }
table.spreadsheet td.error input { border-color: #dc2626; }
table.spreadsheet tr.deleted td { color: #94a3b8; }
table.spreadsheet tr.deleted td .cell-display { text-decoration: line-through; }
table.spreadsheet tr.new-row td { background: #fefce8; }
table.spreadsheet td.actions { padding: 2px 6px; text-align: center; white-space: nowrap; }
table.spreadsheet td.actions button { padding: 2px 8px; font-size: 11px; border: 1px solid #d1d5db; border-radius: 3px; background: #fff; cursor: pointer; margin: 0 1px; }
table.spreadsheet td.actions button:hover { background: #f1f5f9; }
table.spreadsheet td.actions .btn-delete { color: #dc2626; border-color: #fca5a5; }
table.spreadsheet td.actions .btn-delete:hover { background: #fef2f2; }
table.spreadsheet td.actions .btn-restore { color: #16a34a; border-color: #86efac; }
table.spreadsheet td.actions .btn-restore:hover { background: #f0fdf4; }
table.spreadsheet td.actions .btn-save { color: #2563eb; border-color: #93c5fd; }
table.spreadsheet td.actions .btn-save:hover { background: #eff6ff; }

.load-more { margin-top: 12px; }
.load-more button { padding: 8px 20px; font-size: 13px; border: 1px solid #d1d5db; border-radius: 4px; background: #fff; cursor: pointer; }
.load-more button:hover { background: #f1f5f9; }

.status-bar { margin-top: 8px; font-size: 12px; color: #64748b; }

/* Upload progress */
.upload-progress-container { margin-bottom: 12px; padding: 8px 12px; background: #fff; border: 1px solid #e2e8f0; border-radius: 4px; }
.upload-progress-bar { width: 100%; height: 8px; background: #e2e8f0; border-radius: 4px; overflow: hidden; margin-bottom: 4px; }
.upload-progress-fill { height: 100%; background: #2563eb; border-radius: 4px; transition: width 0.3s ease; }
.upload-progress-label { font-size: 12px; color: #64748b; }
.upload-error { font-size: 12px; color: #dc2626; margin-top: 4px; }

/* Download link */
.btn-download { display: inline-block; padding: 2px 8px; font-size: 11px; border: 1px solid #93c5fd; border-radius: 3px; background: #eff6ff; color: #2563eb; text-decoration: none; cursor: pointer; }
.btn-download:hover { background: #dbeafe; }
.spreadsheet-toolbar .btn-add:disabled { opacity: 0.5; cursor: not-allowed; }
`
