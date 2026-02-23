package openapigen

// GenerateDocsHTML returns an HTML page that renders an OpenAPI spec using
// Stoplight Elements. The page loads the Elements web component JS and CSS
// from /openapi/assets/ and points the <elements-api> component at /openapi
// for the spec JSON.
func GenerateDocsHTML(title string) string {
	if title == "" {
		title = "API Documentation"
	}
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
    <title>` + title + `</title>
    <script src="/openapi/assets/web-components.min.js"></script>
    <link rel="stylesheet" href="/openapi/assets/styles.min.css">
  </head>
  <body>
    <elements-api
      apiDescriptionUrl="/openapi"
      router="memory"
      layout="sidebar"
    />
  </body>
</html>`
}
