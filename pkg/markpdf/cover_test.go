// cover_test.go verifies title-page metadata derivation from document content
// and cover rendering fallbacks used by CLI/config driven PDFs.
package markpdf

import (
	"strings"
	"testing"
)

func TestApplyDocumentMetadataUsesFirstH1AndImplicitSubtitle(t *testing.T) {
	content := `<h1 id="quarterly-report">Quarterly <em>Report</em></h1>
<p><em>Service quality review</em></p>
<p>Body copy remains in the document.</p>`

	content, opts := applyDocumentMetadata(content, Options{Cover: CoverOptions{Enabled: true}})
	if opts.Title != "Quarterly Report" {
		t.Fatalf("expected first H1 to become title, got %q", opts.Title)
	}
	if opts.Subtitle != "Service quality review" {
		t.Fatalf("expected italic paragraph after H1 to become subtitle, got %q", opts.Subtitle)
	}
	if strings.Contains(content, "<h1") || strings.Contains(content, "Service quality review") {
		t.Fatalf("expected auto cover title/subtitle to be removed from body, got:\n%s", content)
	}
	if !strings.Contains(content, "Body copy remains") {
		t.Fatalf("expected body copy to remain, got:\n%s", content)
	}

	cover, err := buildCover(opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"<h1>Quarterly Report</h1>",
		`class="markpdf-cover-subtitle">Service quality review</p>`,
	} {
		if !strings.Contains(cover, expected) {
			t.Fatalf("expected cover to contain %q, got:\n%s", expected, cover)
		}
	}
}

func TestExplicitCoverSubtitleWinsOverImplicitSubtitle(t *testing.T) {
	content := `<h1>Document Title</h1><p><em>Implicit subtitle</em></p>`
	_, opts := applyDocumentMetadata(content, Options{
		Cover: CoverOptions{
			Enabled:  true,
			Subtitle: "Explicit subtitle",
		},
	})

	cover, err := buildCover(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cover, "Explicit subtitle") {
		t.Fatalf("expected explicit subtitle in cover, got:\n%s", cover)
	}
	if strings.Contains(cover, "Implicit subtitle") {
		t.Fatalf("implicit subtitle should not override explicit cover subtitle, got:\n%s", cover)
	}
}

func TestCoverConsumesH1AndMetadataWithConfiguredTitle(t *testing.T) {
	content := `<h1>Civic Permit Portal: Vendor Risk Review Package</h1>
<p>Last updated: 2026-05-08 America/Toronto
Prepared by: Alex Morgan</p>
<hr>
<h2>What this document covers</h2>`

	content, opts := applyDocumentMetadata(content, Options{
		Title: "Vendor Risk Review Package",
		Cover: CoverOptions{
			Enabled: true,
		},
	})
	if opts.Cover.Title != "Vendor Risk Review Package" {
		t.Fatalf("expected suffix title on cover, got %q", opts.Cover.Title)
	}
	if opts.Subtitle != "Civic Permit Portal" {
		t.Fatalf("expected prefix subtitle on cover, got %q", opts.Subtitle)
	}
	if opts.Cover.Date != "2026-05-08 America/Toronto" {
		t.Fatalf("expected document date on cover, got %q", opts.Cover.Date)
	}
	if opts.Cover.Author != "Alex Morgan" {
		t.Fatalf("expected document author on cover, got %q", opts.Cover.Author)
	}
	for _, unexpected := range []string{"<h1", "Last updated", "Prepared by", "<hr"} {
		if strings.Contains(content, unexpected) {
			t.Fatalf("expected %q to be moved off body page, got:\n%s", unexpected, content)
		}
	}
	if !strings.Contains(content, "What this document covers") {
		t.Fatalf("expected body section to remain, got:\n%s", content)
	}
}

func TestCoverSplitsDashTitleWhenNoConfiguredTitleExists(t *testing.T) {
	content := `<h1>Civic Permit API -- Endpoint Specification</h1>
<p>Base URL: <code>https://api.example.test</code></p>`

	content, opts := applyDocumentMetadata(content, Options{
		Cover: CoverOptions{Enabled: true},
	})
	if opts.Cover.Title != "Endpoint Specification" {
		t.Fatalf("expected suffix title on cover, got %q", opts.Cover.Title)
	}
	if opts.Subtitle != "Civic Permit API" {
		t.Fatalf("expected prefix subtitle on cover, got %q", opts.Subtitle)
	}
	if strings.Contains(content, "<h1") {
		t.Fatalf("expected generated cover title to be removed from body, got:\n%s", content)
	}
	if !strings.Contains(content, "Base URL") {
		t.Fatalf("expected body content to remain, got:\n%s", content)
	}
}
