// html_builder.go assembles the final browser document by combining sanitized
// content, embedded assets, theme CSS, page CSS, optional cover, and TOC.
package markpdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/kevingatera/markpdf/internal"
)

type htmlData struct {
	Title         string
	CSS           template.CSS
	Cover         template.HTML
	TOC           template.HTML
	Content       template.HTML
	HighlightJS   template.JS
	KaTeXJS       template.JS
	MermaidJS     template.JS
	RuntimeJS     template.JS
	RuntimeConfig template.JS
}

type browserRuntimeConfig struct {
	MermaidTheme string             `json:"mermaidTheme"`
	Print        browserPrintConfig `json:"print"`
}

type browserPrintConfig struct {
	PortraitContentWidth   float64 `json:"portraitContentWidth"`
	PortraitContentHeight  float64 `json:"portraitContentHeight"`
	LandscapeContentWidth  float64 `json:"landscapeContentWidth"`
	LandscapeContentHeight float64 `json:"landscapeContentHeight"`
}

func buildHTML(content string, opts Options) (string, error) {
	css, err := loadCSS(opts)
	if err != nil {
		return "", err
	}
	cover, err := buildCover(opts)
	if err != nil {
		return "", err
	}
	toc := ""
	if opts.TOC {
		toc = buildTOC(content)
	}
	highlightJS, err := internal.FS.ReadFile("assets/highlight.min.js")
	if err != nil {
		return "", fmt.Errorf("load highlight.js: %w", err)
	}
	katexJS, err := internal.FS.ReadFile("assets/katex.min.js")
	if err != nil {
		return "", fmt.Errorf("load katex.js: %w", err)
	}
	mermaidJS, err := internal.FS.ReadFile("assets/mermaid.min.js")
	if err != nil {
		return "", fmt.Errorf("load mermaid.js: %w", err)
	}
	// The runtime is first-party code that coordinates all browser-side work;
	// if it is missing, markpdfReady never flips and PDF generation hangs.
	runtimeJS, err := internal.FS.ReadFile("assets/runtime.js")
	if err != nil {
		return "", fmt.Errorf("load runtime.js: %w", err)
	}

	tplBytes, err := internal.FS.ReadFile("templates/document.html")
	if err != nil {
		return "", err
	}
	tpl, err := template.New("document").Parse(string(tplBytes))
	if err != nil {
		return "", err
	}
	title := opts.Title
	if title == "" {
		title = opts.Cover.Title
	}
	var out bytes.Buffer
	err = tpl.Execute(&out, htmlData{
		Title:         title,
		CSS:           template.CSS(css),
		Cover:         template.HTML(cover),
		TOC:           template.HTML(toc),
		Content:       template.HTML(content),
		HighlightJS:   template.JS(string(highlightJS)),
		KaTeXJS:       template.JS(string(katexJS)),
		MermaidJS:     template.JS(string(mermaidJS)),
		RuntimeJS:     template.JS(string(runtimeJS)),
		RuntimeConfig: runtimeConfigJS(opts),
	})
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func loadCSS(opts Options) (string, error) {
	base, err := internal.FS.ReadFile("themes/base.css")
	if err != nil {
		return "", err
	}
	theme, err := internal.FS.ReadFile("themes/" + opts.Theme + ".css")
	if err != nil {
		return "", err
	}
	highlight, err := internal.FS.ReadFile("assets/highlight.css")
	if err != nil {
		return "", fmt.Errorf("load highlight.css: %w", err)
	}
	katex, err := internal.FS.ReadFile("assets/katex.min.css")
	if err != nil {
		return "", fmt.Errorf("load katex.css: %w", err)
	}
	// Ordering matters: base sets structural print rules, theme overrides visual
	// language, pageCSS injects document-specific dimensions, then vendor styles
	// fill in syntax/math details.
	css := string(base) + "\n" + string(theme) + "\n" + pageCSS(opts) + "\n" + string(highlight) + "\n" + string(katex)
	if opts.CustomCSS != "" {
		// CustomCSS accepts either a path or raw CSS. Treat unreadable values as
		// literal CSS so programmatic callers can pass inline overrides directly.
		if data, readErr := os.ReadFile(opts.CustomCSS); readErr == nil {
			css += "\n" + string(data)
		} else {
			css += "\n" + opts.CustomCSS
		}
	}
	return css, nil
}

func pageCSS(opts Options) string {
	// Top/bottom are true @page margins because Chrome's native header/footer
	// templates live outside the DOM. Left/right are document padding so themed
	// backgrounds can paint edge-to-edge behind the content. A named landscape
	// page is emitted for runtime-classified diagrams without forcing the whole
	// document into landscape.
	top := printMargin(opts.Page.Margins.Top, opts.Header != "")
	bottom := printMargin(opts.Page.Margins.Bottom, opts.Footer != "")
	right := printMargin(opts.Page.Margins.Right, false)
	left := printMargin(opts.Page.Margins.Left, false)
	contentTop := printContentInset(opts.Header != "")
	contentBottom := printContentInset(opts.Footer != "")

	return fmt.Sprintf(`@page {
  size: %s;
  margin: %s 0 %s 0;
  background: var(--markpdf-bg);
}

@page markpdf-landscape {
  size: %s;
  margin: %s 0 %s 0;
  background: var(--markpdf-bg);
}

@media print {
  .markpdf-document {
    padding: %s %s %s %s !important;
    -webkit-box-decoration-break: clone;
    box-decoration-break: clone;
  }
	}`,
		pageSizeCSS(opts.Page.Size, opts.Page.Orientation),
		top,
		bottom,
		pageSizeCSS(opts.Page.Size, "landscape"),
		top,
		bottom,
		contentTop,
		right,
		contentBottom,
		left,
	)
}

func runtimeConfigJS(opts Options) template.JS {
	// Browser-side diagram sizing needs the same page dimensions as the print
	// pipeline. Passing them as JSON avoids duplicating Go's page/margin parsing
	// in JavaScript.
	data, err := json.Marshal(browserRuntimeConfig{
		MermaidTheme: opts.Mermaid.Theme,
		Print:        runtimePrintConfig(opts),
	})
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(data)
}

func runtimePrintConfig(opts Options) browserPrintConfig {
	// Use the effective margins after header/footer safety expansion; otherwise
	// the runtime would overestimate available height and choose landscape pages
	// that still clip under Chrome's native header/footer bands.
	top := parseLengthInches(printMargin(opts.Page.Margins.Top, opts.Header != ""))
	bottom := parseLengthInches(printMargin(opts.Page.Margins.Bottom, opts.Footer != ""))
	left := parseLengthInches(printMargin(opts.Page.Margins.Left, false))
	right := parseLengthInches(printMargin(opts.Page.Margins.Right, false))

	portraitWidth, portraitHeight := pageSizeInches(opts.Page.Size, "portrait")
	landscapeWidth, landscapeHeight := pageSizeInches(opts.Page.Size, "landscape")

	return browserPrintConfig{
		PortraitContentWidth:   inchesToCSSPixels(portraitWidth - left - right),
		PortraitContentHeight:  inchesToCSSPixels(portraitHeight - top - bottom),
		LandscapeContentWidth:  inchesToCSSPixels(landscapeWidth - left - right),
		LandscapeContentHeight: inchesToCSSPixels(landscapeHeight - top - bottom),
	}
}

func inchesToCSSPixels(inches float64) float64 {
	// Headless Chromium reports SVG and element dimensions in CSS pixels even
	// though CDP PDF configuration is inch-based.
	if inches <= 0 {
		return 0
	}
	return inches * 96
}

func pageSizeCSS(size, orientation string) string {
	size = strings.TrimSpace(size)
	orientation = strings.ToLower(strings.TrimSpace(orientation))
	if size == "" {
		size = "A4"
	}
	if strings.Contains(strings.ToLower(size), "x") {
		parts := strings.SplitN(strings.ToLower(size), "x", 2)
		if len(parts) == 2 {
			width := strings.TrimSpace(parts[0])
			height := strings.TrimSpace(parts[1])
			widthInches := parseLengthInches(width)
			heightInches := parseLengthInches(height)
			// Custom WxH values are literal CSS page sizes, so swap the pair for
			// named landscape pages instead of appending an invalid orientation.
			if orientation == "landscape" && widthInches > 0 && heightInches > 0 && widthInches < heightInches {
				width, height = height, width
			}
			if orientation != "landscape" && widthInches > 0 && heightInches > 0 && widthInches > heightInches {
				width, height = height, width
			}
			return width + " " + height
		}
	}
	if orientation == "landscape" {
		return size + " landscape"
	}
	return size
}

func printMargin(value string, hasHeaderFooter bool) string {
	// User margins smaller than Chrome's native header/footer band can cause
	// headings to be clipped on continuation pages, so enforce a safe minimum.
	minimum := headerFooterMinMarginInches
	if hasHeaderFooter {
		minimum += headerFooterContentGapInches
	}
	if !hasHeaderFooter || parseLengthInches(value) >= minimum {
		return value
	}
	return fmt.Sprintf("%.4fin", minimum)
}

func printContentInset(hasHeaderFooter bool) string {
	// Header/footer spacing is reserved by @page margins, not document padding.
	// Keeping this at zero avoids double-spacing the first page versus fragments.
	return "0"
}
