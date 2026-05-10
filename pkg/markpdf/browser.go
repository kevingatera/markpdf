// browser.go manages Rod/Chromium lifecycle, waits for client-side rendering,
// and configures Chrome DevTools PDF output including native headers/footers.
package markpdf

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type Browser struct {
	browser *rod.Browser
}

type BrowserStatus struct {
	Path string
}

func NewBrowser() (*Browser, error) {
	// Rod's launcher resolves an installed Chrome/Chromium first and can fall
	// back to its managed browser download path when users run `browser install`.
	url, err := launcher.New().Headless(true).Launch()
	if err != nil {
		return nil, err
	}
	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, err
	}
	return &Browser{browser: browser}, nil
}

func (b *Browser) Close() error {
	if b == nil || b.browser == nil {
		return nil
	}
	return b.browser.Close()
}

func DetectBrowser() BrowserStatus {
	path, ok := launcher.LookPath()
	if !ok {
		return BrowserStatus{}
	}
	return BrowserStatus{Path: path}
}

func InstallBrowser() error {
	_, err := launcher.NewBrowser().Get()
	return err
}

func (b *Browser) PrintPDF(w io.Writer, html string, opts Options) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	page, err := b.browser.Context(ctx).Page(proto.TargetCreateTarget{})
	if err != nil {
		return err
	}
	defer page.Close()
	page = page.Context(ctx)

	if err := page.SetDocumentContent(html); err != nil {
		return err
	}
	if err := page.WaitLoad(); err != nil {
		return err
	}
	// The document runtime flips this flag only after client-side rendering has
	// finished: syntax highlighting, KaTeX, Mermaid SVG generation, and diagram
	// sizing all need to complete before Chrome snapshots the PDF.
	if err := page.Wait(rod.Eval(`() => window.markpdfReady === true`)); err != nil {
		return err
	}
	// Give layout a final quiet window after async rendering. This catches late
	// font/SVG/style recalculations that otherwise show up as clipped print output.
	if err := page.WaitIdle(10 * time.Second); err != nil {
		return err
	}
	pdf, err := page.PDF(pdfOptions(opts))
	if err != nil {
		return err
	}
	_, err = io.Copy(w, pdf)
	return err
}

func pdfOptions(opts Options) *proto.PagePrintToPDF {
	paperWidth, paperHeight := pageSizeInches(opts.Page.Size, opts.Page.Orientation)
	background := themeBackground(opts.Theme)
	header := normalizeHeaderFooter(opts.Header, headerFooterTop, background)
	footer := normalizeHeaderFooter(opts.Footer, headerFooterBottom, background)
	// CSS @page owns margins and printable area. CDP margins stay zero so Chrome
	// does not add a second, independent margin model around our themed page.
	return &proto.PagePrintToPDF{
		PrintBackground:     true,
		PreferCSSPageSize:   true,
		PaperWidth:          ptr(paperWidth),
		PaperHeight:         ptr(paperHeight),
		MarginTop:           ptr(0.0),
		MarginBottom:        ptr(0.0),
		MarginLeft:          ptr(0.0),
		MarginRight:         ptr(0.0),
		DisplayHeaderFooter: opts.Header != "" || opts.Footer != "",
		HeaderTemplate:      header,
		FooterTemplate:      footer,
		Landscape:           strings.EqualFold(opts.Page.Orientation, "landscape"),
	}
}

func ptr[T any](value T) *T {
	return &value
}

type headerFooterPosition string

const (
	headerFooterTop    headerFooterPosition = "top"
	headerFooterBottom headerFooterPosition = "bottom"
	// Chrome's native header/footer boxes are outside the document DOM. These
	// values reserve enough @page space for them while keeping body content from
	// starting underneath the native template paint area.
	headerFooterMinMarginInches  = 1.05
	headerFooterContentGapInches = 5.0 / mmPerInch
)

func normalizeHeaderFooter(input string, position headerFooterPosition, background string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	// Chrome replaces these class names inside native header/footer templates.
	// We do the substitution here so user config can stay template-engine agnostic.
	content := strings.NewReplacer(
		"{{page}}", `<span class="pageNumber"></span>`,
		"{{pages}}", `<span class="totalPages"></span>`,
		"{{title}}", `<span class="title"></span>`,
		"{{url}}", `<span class="url"></span>`,
		"{{date}}", `<span class="date"></span>`,
	).Replace(input)
	return wrapHeaderFooter(content, position, background)
}

func wrapHeaderFooter(content string, position headerFooterPosition, background string) string {
	// The template box must not extend into body content. The one-sided shadow
	// fills the tiny edge strip Chrome otherwise leaves unpainted on themed pages.
	alignItems := "center"
	justifyContent := "center"
	textAlign := "center"
	edgeShadow := "0 6mm 0 0 " + background
	edgeMargin := "5mm 0 0 0"
	if position == headerFooterTop {
		justifyContent = "flex-start"
		textAlign = "left"
		edgeShadow = "0 -6mm 0 0 " + background
		edgeMargin = "0 0 5mm 0"
	}

	templateStyle := `<style>html,body{margin:0;padding:0;}</style>`

	return templateStyle + `<div style="` +
		`box-sizing:border-box;` +
		`display:flex;` +
		`align-items:` + alignItems + `;` +
		`justify-content:` + justifyContent + `;` +
		`width:100%;` +
		`height:calc(100% - 5mm);` +
		`margin:` + edgeMargin + `;` +
		`padding-left:10mm;` +
		`padding-right:10mm;` +
		`background:` + background + `;` +
		`box-shadow:` + edgeShadow + `;` +
		`-webkit-print-color-adjust:exact;` +
		`print-color-adjust:exact;` +
		`font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;` +
		`font-size:9.5px;` +
		`font-weight:600;` +
		`line-height:1.35;` +
		`letter-spacing:.02em;` +
		`color:#111;` +
		`text-align:` + textAlign + `;` +
		`"><span>` + content + `</span></div>`
}

func themeBackground(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "academic":
		return "#fffdf5"
	case "github":
		return "#fafafa"
	case "atelier":
		return "#fdfbf7"
	case "rideau":
		return "#f6fbfa"
	default:
		return "#ffffff"
	}
}
