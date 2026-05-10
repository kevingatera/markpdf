// pdf_test.go covers PDF configuration helpers and Chrome header/footer
// template safeguards that protect printed output from clipping.
package markpdf

import (
	"strings"
	"testing"
)

func TestNormalizeHeaderFooter(t *testing.T) {
	got := normalizeHeaderFooter(`<span>{{title}}</span> {{page}} / {{pages}} {{date}}`, headerFooterBottom, "#ffffff")

	for _, expected := range []string{
		`font-size:9.5px`,
		`font-weight:600`,
		`background:#ffffff`,
		`height:calc(100% - 5mm)`,
		`margin:5mm 0 0 0`,
		`box-shadow:0 6mm 0 0 #ffffff`,
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
}

func TestThemeBackgroundIncludesWarmThemes(t *testing.T) {
	if got := themeBackground("academic"); got != "#fffdf5" {
		t.Fatalf("expected academic header/footer background, got %q", got)
	}
	if got := themeBackground("atelier"); got != "#fdfbf7" {
		t.Fatalf("expected atelier header/footer background, got %q", got)
	}
	if got := themeBackground("rideau"); got != "#f6fbfa" {
		t.Fatalf("expected rideau header/footer background, got %q", got)
	}
}
