// sanitize.go defines the HTML trust boundary between user-provided
// Markdown/HTML and the headless browser that executes embedded render assets.
package markpdf

import "github.com/microcosm-cc/bluemonday"

func sanitizeContent(content string) string {
	policy := bluemonday.UGCPolicy()
	// Runtime features use class hooks for styling/rendering, while heading ids
	// are required for table-of-contents anchors. Everything else stays on the
	// UGC allowlist so user-supplied Markdown/HTML cannot execute script.
	policy.AllowAttrs("class").Globally()
	policy.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	return policy.Sanitize(content)
}
