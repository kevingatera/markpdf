// pdf_test.go covers PDF configuration helpers and Chrome header/footer
// template safeguards that protect printed output from clipping.
package markpdf

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/kevingatera/markpdf/internal"
)

func TestNormalizeHeaderFooter(t *testing.T) {
	got := normalizeHeaderFooter(`<span>{{title}}</span> {{page}} / {{pages}} {{date}}`, headerFooterBottom, "#ffffff")

	for _, expected := range []string{
		`background:#ffffff`,
		`print-color-adjust:exact`,
		`<span class="title"></span>`,
		`<span class="pageNumber"></span>`,
		`<span class="totalPages"></span>`,
		`<span class="date"></span>`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected header/footer template to contain %q, got:\n%s", expected, got)
		}
	}
	if strings.Contains(got, `html,body{margin:0;padding:0;background`) {
		t.Fatalf("header/footer template body background can cover document content, got:\n%s", got)
	}
	if strings.Contains(got, `height:calc(100% + 6mm)`) {
		t.Fatalf("header/footer template must not extend into document content, got:\n%s", got)
	}
}

func TestPageSizeInches(t *testing.T) {
	width, height := pageSizeInches("A4", "landscape")
	if width <= height {
		t.Fatalf("expected landscape A4 width > height, got %.2f x %.2f", width, height)
	}

	width, height = pageSizeInches("210mmx297mm", "portrait")
	if width < 8.2 || width > 8.3 || height < 11.6 || height > 11.8 {
		t.Fatalf("expected custom mm page near A4, got %.2f x %.2f", width, height)
	}
}

func TestPDFOptionsUsesCSSPageMargins(t *testing.T) {
	opts := DefaultOptions()
	opts.Page.Margins.Top = "20mm"
	opts.Page.Margins.Bottom = "20mm"
	opts.Header = "{{title}}"
	opts.Footer = "{{page}} / {{pages}}"

	pdf := pdfOptions(opts)
	if !pdf.PreferCSSPageSize {
		t.Fatal("expected PDF generation to use CSS @page sizing")
	}
	if *pdf.MarginTop != 0 || *pdf.MarginBottom != 0 || *pdf.MarginLeft != 0 || *pdf.MarginRight != 0 {
		t.Fatalf("expected CDP margins to be zero when CSS @page controls layout, got %.3f %.3f %.3f %.3f", *pdf.MarginTop, *pdf.MarginRight, *pdf.MarginBottom, *pdf.MarginLeft)
	}
}

func TestPageCSSReservesHeaderFooterMargins(t *testing.T) {
	opts := DefaultOptions()
	opts.Page.Margins.Top = "4mm"
	opts.Page.Margins.Bottom = "4mm"
	opts.Header = "{{title}}"
	opts.Footer = "{{page}} / {{pages}}"

	css := pageCSS(opts)
	if !strings.Contains(css, "1.2469in") {
		t.Fatalf("expected CSS page margins to reserve header/footer space, got:\n%s", css)
	}
	if !strings.Contains(css, "@page markpdf-landscape") {
		t.Fatalf("expected CSS to define a named landscape page, got:\n%s", css)
	}
	if !strings.Contains(css, "size: A4 landscape") {
		t.Fatalf("expected named landscape page to use landscape page size, got:\n%s", css)
	}
}

func TestRuntimeConfigIncludesPrintMetrics(t *testing.T) {
	opts := DefaultOptions()
	opts.Header = "{{title}}"
	opts.Footer = "{{page}} / {{pages}}"

	var config browserRuntimeConfig
	if err := json.Unmarshal([]byte(runtimeConfigJS(opts)), &config); err != nil {
		t.Fatal(err)
	}

	if config.MermaidTheme != opts.Mermaid.Theme {
		t.Fatalf("expected Mermaid theme %q, got %q", opts.Mermaid.Theme, config.MermaidTheme)
	}
	if config.Print.LandscapeContentWidth <= config.Print.PortraitContentWidth {
		t.Fatalf("expected landscape content width to exceed portrait width, got %.2f <= %.2f", config.Print.LandscapeContentWidth, config.Print.PortraitContentWidth)
	}
	if config.Print.PortraitContentHeight <= config.Print.LandscapeContentHeight {
		t.Fatalf("expected portrait content height to exceed landscape height, got %.2f <= %.2f", config.Print.PortraitContentHeight, config.Print.LandscapeContentHeight)
	}
}

func TestCustomPageSizeCSSCanRotateNamedPages(t *testing.T) {
	if got := pageSizeCSS("210mmx297mm", "landscape"); got != "297mm 210mm" {
		t.Fatalf("expected custom landscape page to rotate dimensions, got %q", got)
	}
	if got := pageSizeCSS("297mmx210mm", "portrait"); got != "210mm 297mm" {
		t.Fatalf("expected custom portrait page to rotate dimensions, got %q", got)
	}
}

func TestThemeBackgroundIncludesWarmThemes(t *testing.T) {
	for _, theme := range []string{"academic", "atelier", "rideau"} {
		expected := embeddedThemeBackground(t, theme)
		if got := themeBackground(theme); got != expected {
			t.Fatalf("expected %s header/footer background to match theme CSS %q, got %q", theme, expected, got)
		}
	}
}

var themeBackgroundPattern = regexp.MustCompile(`--markpdf-bg:\s*([^;]+);`)

func embeddedThemeBackground(t *testing.T, theme string) string {
	t.Helper()

	data, err := internal.FS.ReadFile("themes/" + theme + ".css")
	if err != nil {
		t.Fatal(err)
	}
	match := themeBackgroundPattern.FindStringSubmatch(string(data))
	if len(match) != 2 {
		t.Fatalf("expected %s theme to define --markpdf-bg", theme)
	}
	return strings.TrimSpace(match[1])
}
