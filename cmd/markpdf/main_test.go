// main_test.go covers CLI option precedence that is not exercised by package
// tests, especially the no-config report preset used for polished PDFs.
package main

import "testing"

func TestLoadOptionsAppliesReportDefaultsAndExplicitOverrides(t *testing.T) {
	opts, err := loadOptions(cliOptions{
		report:       true,
		theme:        "rideau",
		margin:       "22mm",
		marginLeft:   "28mm",
		footer:       "Custom footer {{page}}/{{pages}}",
		mermaidTheme: "default",
	})
	if err != nil {
		t.Fatal(err)
	}

	if opts.Theme != "rideau" {
		t.Fatalf("expected explicit theme override, got %q", opts.Theme)
	}
	if !opts.TOC || !opts.Cover.Enabled {
		t.Fatalf("expected report defaults to enable TOC and cover, got toc=%v cover=%v", opts.TOC, opts.Cover.Enabled)
	}
	if opts.Header != "{{title}}" {
		t.Fatalf("expected report header default, got %q", opts.Header)
	}
	if opts.Footer != "Custom footer {{page}}/{{pages}}" {
		t.Fatalf("expected explicit footer override, got %q", opts.Footer)
	}
	if opts.Page.Margins.Top != "22mm" || opts.Page.Margins.Right != "22mm" || opts.Page.Margins.Bottom != "22mm" || opts.Page.Margins.Left != "28mm" {
		t.Fatalf("expected margin shorthand plus side override, got %+v", opts.Page.Margins)
	}
	if opts.Mermaid.Theme != "default" {
		t.Fatalf("expected explicit Mermaid theme override, got %q", opts.Mermaid.Theme)
	}
}
