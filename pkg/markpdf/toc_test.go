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
	// Single-level: flat list
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
}

func TestBuildTOCHierarchy(t *testing.T) {
	// Two-level hierarchy: H2 batches, H3 records nested inside
	html := `<h2 id="b1">Batch 1</h2><h3 id="r1">Record 1 — zone</h3><h3 id="r2">Record 2 — setback</h3><h2 id="b2">Batch 2</h2><h3 id="r3">Record 75 — schedule</h3>`
	toc := buildTOC(html)

	if strings.Count(toc, `<ol>`) != 3 {
		t.Fatalf("two-level hierarchy needs 3 <ol> (1 outer + 2 nested), got:\n%s", toc)
	}

	// Verify the H3 items are inside the H2 item's <ol>, not as direct children of the outer <ol>.
	if !strings.Contains(toc, "Batch 1") || !strings.Contains(toc, "Record 1") {
		t.Fatal("missing expected text")
	}

	// The inner <ol> should appear after Batch 1's link and before Batch 2.
	idxB1 := strings.Index(toc, "Batch 1")
	idxInnerOl := strings.Index(toc[idxB1:], `<ol>`)
	idxB2 := strings.Index(toc, "Batch 2")

	if idxInnerOl < 0 || idxB2 < 0 {
		t.Fatal("could not find markers in TOC")
	}
	// Record 1 should be inside the nested <ol> (after Batch 1, before Batch 2)
	idxR1 := strings.Index(toc, "Record 1")
	if !(idxB1 < idxR1 && idxR1 < idxB2) {
		t.Fatalf("Record 1 should be between Batch 1 and Batch 2, got:\n%s", toc)
	}
}

func TestBuildTOCDeepToShallow(t *testing.T) {
	// Transition from H3 back up to H2
	html := `<h3 id="r1">Record 1 — zone</h3><h2 id="b2">Batch 2</h2><h3 id="r3">Record 3 — parking</h3>`
	toc := buildTOC(html)

	// Should have: outer <ol> with H3, then H2 sibling, then nested <ol> for second H3
	if strings.Count(toc, `<ol>`) != 2 {
		t.Fatalf("expected 2 <ol> elements (outer + one nested), got:\n%s", toc)
	}
	if !strings.Contains(toc, "Record 1") && strings.Contains(toc, "Batch 2") && strings.Contains(toc, "Record 3") {
		t.Fatal("missing expected text")
	}
}

func TestBuildTOCEmpty(t *testing.T) {
	if buildTOC("") != "" {
		t.Fatal("expected empty TOC for empty content")
	}
	// HTML with only h4+ should also produce empty TOC (h4 skipped).
	html := `<h4 id="d">Deep only</h4>`
	if buildTOC(html) != "" {
		t.Fatal("expected empty TOC when all headings are > h3")
	}
}
