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

	// First heading opens the outermost <li>.
	writeTOCItem(&b, headings[0])
	prevLevel := headings[0].Level

	for _, h := range headings[1:] {
		if h.Level > prevLevel {
			// Deeper — start a nested <ol> inside the current <li>.
			b.WriteString(`<ol>`)
		} else if h.Level < prevLevel {
			// Shallower — close nested </ol> and </li> for each level we pop.
			for l := prevLevel; l > h.Level; l-- {
				b.WriteString(`</li></ol>`)
			}
			// Close the sibling <li> at the new level.
			b.WriteString(`</li>`)
		} else {
			// Same level — close previous <li>, open new sibling.
			b.WriteString(`</li>`)
		}
		writeTOCItem(&b, h)
		prevLevel = h.Level
	}

	// Close remaining open tags.
	minLevel := headings[0].Level
	if prevLevel > minLevel {
		b.WriteString(`</li>`) // close innermost <li>
	}
	for l := prevLevel; l > minLevel; l-- {
		b.WriteString(`</ol>`)
	}
	b.WriteString(`</li>`) // close outermost <li>
	b.WriteString(`</ol></nav>`)

	return b.String()
}

func writeTOCItem(b *strings.Builder, h heading) {
	b.WriteString(fmt.Sprintf(
		`<li class="toc-level-%d"><a href="#%s"><span class="toc-text">%s</span><span class="toc-leader"></span><span class="toc-page" data-target="%s"></span></a>`,
		h.Level, h.ID, h.Text, h.ID,
	))
}
