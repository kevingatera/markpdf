// markdown.go owns Markdown ingestion: Azure DevOps fence normalization,
// Goldmark rendering, frontmatter extraction, and special Mermaid/KaTeX blocks.
package markpdf

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	htmlRenderer "github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

type markdownResult struct {
	HTML        string
	Options     Options
	FrontMatter map[string]any
}

func renderMarkdown(source []byte, base Options) (markdownResult, error) {
	source = normalizeColonFences(source)
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, meta.Meta),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		// We allow Goldmark to emit raw HTML so supported Markdown/HTML hybrids
		// work, then sanitize the final fragment before it reaches Chromium.
		goldmark.WithRendererOptions(htmlRenderer.WithUnsafe()),
	)
	ctx := parser.NewContext()
	var out bytes.Buffer
	if err := md.Convert(source, &out, parser.WithContext(ctx)); err != nil {
		return markdownResult{}, err
	}

	frontMatter := meta.Get(ctx)
	override, err := optionsFromMap(frontMatter)
	if err != nil {
		return markdownResult{}, err
	}
	return markdownResult{
		HTML:        sanitizeContent(renderSpecialCodeBlocks(out.String())),
		Options:     mergeOptions(base, override),
		FrontMatter: frontMatter,
	}, nil
}

var specialCodeBlockRE = regexp.MustCompile(`(?s)<pre><code class="language-(mermaid|math)">(.+?)</code></pre>`)
var colonFenceStartRE = regexp.MustCompile(`^:::\s*([A-Za-z][A-Za-z0-9_-]*)\s*$`)

func normalizeColonFences(source []byte) []byte {
	// Azure DevOps wiki uses ::: language fences while most Markdown renderers
	// use backticks. Convert only the outer fence syntax so Goldmark can still
	// own code-block parsing, escaping, and line preservation.
	lines := strings.Split(string(source), "\n")
	var out strings.Builder
	var fence string

	for index, line := range lines {
		switch {
		case fence != "" && strings.TrimSpace(line) == ":::":
			if strings.HasPrefix(fence, "admonition:") {
				out.WriteString("</div>")
			} else {
				out.WriteString("```")
			}
			fence = ""
		case fence == "":
			if matches := colonFenceStartRE.FindStringSubmatch(line); len(matches) == 2 {
				label := strings.ToLower(matches[1])
				if isAdmonition(label) {
					out.WriteString(admonitionStart(label))
					fence = "admonition:" + label
				} else {
					out.WriteString("```")
					out.WriteString(matches[1])
					fence = "code"
				}
			} else {
				out.WriteString(line)
			}
		default:
			out.WriteString(line)
		}

		if index < len(lines)-1 {
			out.WriteByte('\n')
		}
	}

	return []byte(out.String())
}

func isAdmonition(label string) bool {
	switch label {
	case "note", "tip", "warning", "important", "caution":
		return true
	default:
		return false
	}
}

func admonitionStart(label string) string {
	title := strings.ToUpper(label[:1]) + label[1:]
	return `<div class="markpdf-admonition markpdf-admonition-` + html.EscapeString(label) + `">` +
		`<p class="markpdf-admonition-title">` + html.EscapeString(title) + `</p>`
}

func renderSpecialCodeBlocks(html string) string {
	// Mermaid and KaTeX need their source text intact for browser-side renderers.
	// Convert only those fenced blocks into runtime placeholders; ordinary code
	// blocks stay as <pre><code> for highlight.js.
	return specialCodeBlockRE.ReplaceAllStringFunc(html, func(block string) string {
		matches := specialCodeBlockRE.FindStringSubmatch(block)
		if len(matches) != 3 {
			return block
		}
		content := strings.TrimSpace(matches[2])
		if matches[1] == "math" {
			return `<div class="math-display">` + content + `</div>`
		}
		return `<pre class="mermaid">` + content + `</pre>`
	})
}

func optionsFromMap(values map[string]any) (Options, error) {
	if len(values) == 0 {
		return Options{}, nil
	}
	// Re-marshal frontmatter through YAML so nested maps share the same tags and
	// defaults as markpdf.yaml instead of maintaining a second parser by hand.
	data, err := yaml.Marshal(values)
	if err != nil {
		return Options{}, fmt.Errorf("marshal frontmatter: %w", err)
	}
	var opts Options
	if err := yaml.Unmarshal(data, &opts); err != nil {
		return Options{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	return opts, nil
}
