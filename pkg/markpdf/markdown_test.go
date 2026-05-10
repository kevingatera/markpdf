// markdown_test.go verifies Markdown-specific behavior such as Azure DevOps
// fences, Mermaid source preservation, admonitions, and HTML sanitization.
package markpdf

import (
	"strings"
	"testing"
)

func TestRenderMarkdownPreservesMermaidNewlines(t *testing.T) {
	source := []byte("```mermaid\nrequirementDiagram\n    requirement secure_upload {\n        id: REQ-001\n    }\n```\n")

	result, err := renderMarkdown(source, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.HTML, "secure_upload {\n        id: REQ-001") {
		t.Fatalf("expected mermaid source newlines to be preserved, got:\n%s", result.HTML)
	}
}

func TestRenderMarkdownSupportsColonFences(t *testing.T) {
	source := []byte("::: mermaid\nflowchart LR\n    A --> B\n:::\n")

	result, err := renderMarkdown(source, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.HTML, `<pre class="mermaid">flowchart LR`) {
		t.Fatalf("expected colon fence to become a mermaid block, got:\n%s", result.HTML)
	}
}

func TestRenderMarkdownSupportsAdmonitions(t *testing.T) {
	source := []byte("::: warning\nReview queue depth before scaling down workers.\n:::\n")

	result, err := renderMarkdown(source, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.HTML, `markpdf-admonition-warning`) {
		t.Fatalf("expected Azure DevOps admonition HTML, got:\n%s", result.HTML)
	}
}

func TestRenderMarkdownSanitizesRawHTML(t *testing.T) {
	source := []byte(`<script>alert("xss")</script><h1 onclick="bad()">Safe</h1>`)

	result, err := renderMarkdown(source, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result.HTML, "<script") || strings.Contains(result.HTML, "onclick") {
		t.Fatalf("expected unsafe HTML to be removed, got:\n%s", result.HTML)
	}
}
