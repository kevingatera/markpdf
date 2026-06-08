package markpdf

import (
	"strings"
	"testing"
)

func TestExtractHeadings(t *testing.T) {
	html := `<h1 id="top">Title</h1><h2 id="b1">Batch 1</h2><h3 id="r1">Record 1</h3><h3 id="r2">Record 2</h3><h2 id="b2">Batch 2</h2><h3 id="r3">Record 3</h3><h4 id="deep">skip this</h4>`

	headings := extractHeadings(html)

	if len(headings) != 6 {
		t.Fatalf("expected 6 headings (h4 skipped), got %d", len(headings))
	}
	if headings[0].Text != "Title" || headings[0].Level != 1 {
		t.Fatalf("unexpected first heading: %+v", headings[0])
	}
	if headings[2].Text != "Record 1" || headings[2].Level != 3 {
		t.Fatalf("unexpected third heading: %+v", headings[2])
	}
}

func TestBuildTOCFlat(t *testing.T) {
	html := `<h2 id="a">Alpha</h2><h2 id="b">Beta</h2><h2 id="g">Gamma</h2>`
	toc := buildTOC(html)

	if !strings.Contains(toc, `<nav class="markpdf-toc">`) {
		t.Fatal("missing nav wrapper")
	}
	if strings.Count(toc, `<ol>`) != 1 {
		t.Fatalf("single-level should have one <ol>, got:\n%s", toc)
	}
	if !strings.Contains(toc, "Alpha") || !strings.Contains(toc, "Beta") {
		t.Fatal("missing heading text")
	}
	// Each entry should have toc-text, toc-leader, and toc-page spans.
	if strings.Count(toc, `class="toc-text"`) != 3 {
		t.Fatalf("expected 3 toc-text spans, got:\n%s", toc)
	}
	if strings.Count(toc, `class="toc-leader"`) != 3 {
		t.Fatalf("expected 3 toc-leader spans, got:\n%s", toc)
	}
	if strings.Count(toc, `class="toc-page"`) != 3 {
		t.Fatalf("expected 3 toc-page spans, got:\n%s", toc)
	}
}

func TestBuildTOCHierarchy(t *testing.T) {
	html := `<h2 id="b1">Batch 1</h2><h3 id="r1">Record 1</h3><h3 id="r2">Record 2</h3><h2 id="b2">Batch 2</h2><h3 id="r3">Record 75</h3>`
	toc := buildTOC(html)

	if strings.Count(toc, `<ol>`) != 3 {
		t.Fatalf("two-level hierarchy needs 3 <ol> (1 outer + 2 nested), got:\n%s", toc)
	}

	if !strings.Contains(toc, "Batch 1") || !strings.Contains(toc, "Record 1") {
		t.Fatal("missing expected text")
	}

	// The inner <ol> should appear after Batch 1's link and before Batch 2.
	idxB1 := strings.Index(toc, "Batch 1")
	idxB2 := strings.Index(toc, "Batch 2")
	idxR1 := strings.Index(toc, "Record 1")

	if idxB1 < 0 || idxB2 < 0 || idxR1 < 0 {
		t.Fatal("could not find markers in TOC")
	}
	if !(idxB1 < idxR1 && idxR1 < idxB2) {
		t.Fatalf("Record 1 should be between Batch 1 and Batch 2, got:\n%s", toc)
	}

	// Page number placeholders should reference the heading IDs.
	if !strings.Contains(toc, `data-target="b1"`) {
		t.Fatal("missing data-target for b1")
	}
	if !strings.Contains(toc, `data-target="r1"`) {
		t.Fatal("missing data-target for r1")
	}
}

func TestBuildTOCDeepToShallow(t *testing.T) {
	html := `<h3 id="r1">Record 1</h3><h2 id="b2">Batch 2</h2><h3 id="r3">Record 3</h3>`
	toc := buildTOC(html)

	if strings.Count(toc, `<ol>`) != 2 {
		t.Fatalf("expected 2 <ol> elements (outer + one nested), got:\n%s", toc)
	}
	if !strings.Contains(toc, "Record 1") || !strings.Contains(toc, "Batch 2") || !strings.Contains(toc, "Record 3") {
		t.Fatal("missing expected text")
	}
}

func TestBuildTOCEmpty(t *testing.T) {
	if buildTOC("") != "" {
		t.Fatal("expected empty TOC for empty content")
	}
	html := `<h4 id="d">Deep only</h4>`
	if buildTOC(html) != "" {
		t.Fatal("expected empty TOC when all headings are > h3")
	}
}
