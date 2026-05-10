// assets_test.go guards embedded CSS/JS assumptions that are easy to break
// without compiler errors, especially print layout and runtime highlighting.
package markpdf

import (
	"strings"
	"testing"

	"github.com/kevingatera/markpdf/internal"
)

func TestBaseCSSAvoidsMidElementBreaks(t *testing.T) {
	data, err := internal.FS.ReadFile("themes/base.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)

	for _, expected := range []string{
		".markpdf-table-wrapper",
		".markpdf-code-block",
		"data-language",
		"break-inside: avoid",
		"page-break-inside: avoid",
		".markpdf-diagram-wide",
		".markpdf-diagram-small",
		".markpdf-diagram-tall",
		".markpdf-diagram-fit-page",
		".markpdf-diagram-oversized",
		".markpdf-heading-diagram-group",
		"overflow-wrap: anywhere",
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("expected base CSS to contain %q", expected)
		}
	}
}

func TestRuntimeNormalizesCodeHighlighting(t *testing.T) {
	data, err := internal.FS.ReadFile("assets/runtime.js")
	if err != nil {
		t.Fatal(err)
	}
	runtime := string(data)

	for _, expected := range []string{
		"highlightCodeBlocks",
		"groupDiagramHeadings",
		"markpdf-command",
		"markpdf-http",
		"markpdf-template",
		"normalizeMermaidSource",
		"inferAPILanguage",
		`"commands": "markpdf-command"`,
		`"partial": "markpdf-template"`,
		`return "markpdf-http"`,
		`.replace(/\\n/g, "<br/>")`,
	} {
		if !strings.Contains(runtime, expected) {
			t.Fatalf("expected runtime JS to contain %q", expected)
		}
	}
}

func TestRideauThemeIsEmbedded(t *testing.T) {
	data, err := internal.FS.ReadFile("themes/rideau.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)

	for _, expected := range []string{
		"Rideau",
		"--rideau-blue: #004890",
		"--rideau-green: #00a78d",
		"--markpdf-bg: #f6fbfa",
		".mermaid",
		"border: 0",
	} {
		if !strings.Contains(css, expected) {
			t.Fatalf("expected Rideau theme CSS to contain %q", expected)
		}
	}
}

func TestCoverSubtitleStylesAreEmbedded(t *testing.T) {
	templateData, err := internal.FS.ReadFile("templates/cover.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(templateData), "markpdf-cover-subtitle") {
		t.Fatal("expected cover template to render subtitles")
	}

	for _, theme := range []string{"base", "modern", "academic", "github", "atelier", "rideau"} {
		data, err := internal.FS.ReadFile("themes/" + theme + ".css")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), ".markpdf-cover-subtitle") {
			t.Fatalf("expected %s theme to style cover subtitles", theme)
		}
	}
}
