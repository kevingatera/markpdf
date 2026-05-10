// toc.go extracts heading anchors from sanitized HTML and builds the optional
// table of contents inserted ahead of document content.
package markpdf

import (
	"fmt"
	"regexp"
	"strings"
)

type heading struct {
	Level int
	ID    string
	Text  string
}

var headingRE = regexp.MustCompile(`(?is)<h([1-6]) id="([^"]+)">(.*?)</h[1-6]>`)
var stripTagsRE = regexp.MustCompile(`(?is)<[^>]+>`)

func extractHeadings(content string) []heading {
	// TOC extraction runs on sanitized Goldmark HTML rather than the Markdown AST
	// so it also works for direct HTML input and any future renderer extensions.
	matches := headingRE.FindAllStringSubmatch(content, -1)
	out := make([]heading, 0, len(matches))
	for _, match := range matches {
		if len(match) != 4 {
			continue
		}
		level := int(match[1][0] - '0')
		if level > 3 {
			// Deep headings make long technical documents noisy and often create
			// multi-page TOCs, so the built-in TOC intentionally stops at h3.
			continue
		}
		out = append(out, heading{
			Level: level,
			ID:    match[2],
			Text:  strings.TrimSpace(stripTagsRE.ReplaceAllString(match[3], "")),
		})
	}
	return out
}

func buildTOC(content string) string {
	headings := extractHeadings(content)
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="markpdf-toc"><h2>Contents</h2><ol>`)
	for _, h := range headings {
		b.WriteString(fmt.Sprintf(`<li class="toc-level-%d"><a href="#%s">%s</a></li>`, h.Level, h.ID, h.Text))
	}
	b.WriteString(`</ol></nav>`)
	return b.String()
}
