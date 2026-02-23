package registry

import "github.com/shipq/shipq/codegen/admingen"

// generateAdminPanel generates the admin panel HTML.
// The admin JS is served from the embedded assets (shipq/assets/admin.min.js),
// so only the HTML shell needs to be generated as a Go string constant.
func generateAdminPanel(cfg CompileConfig) string {
	return admingen.GenerateAdminHTML("")
}
