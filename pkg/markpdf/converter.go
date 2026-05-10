// Package markpdf exposes the public conversion API and coordinates fragment
// rendering, option merging, browser PDF generation, and output file writes.
package markpdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Converter struct {
	opts    Options
	browser *Browser
}

func New(options ...Option) (*Converter, error) {
	opts := DefaultOptions()
	for _, option := range options {
		option(&opts)
	}
	// Start Chromium during construction so conversion failures surface before
	// callers spend time preparing output paths or batching multiple files.
	browser, err := NewBrowser()
	if err != nil {
		return nil, err
	}
	return &Converter{opts: opts.normalized(), browser: browser}, nil
}

func NewWithOptions(opts Options) (*Converter, error) {
	browser, err := NewBrowser()
	if err != nil {
		return nil, err
	}
	return &Converter{opts: opts.normalized(), browser: browser}, nil
}

func (c *Converter) Close() error {
	if c.browser == nil {
		return nil
	}
	return c.browser.Close()
}

func (c *Converter) Convert(input []byte, outputPath string) error {
	fragment, opts, err := c.renderFragment(input, "markdown")
	if err != nil {
		return err
	}
	return c.writeDocument(fragment, opts, outputPath)
}

func (c *Converter) ConvertHTML(input []byte, outputPath string) error {
	fragment, opts, err := c.renderFragment(input, "html")
	if err != nil {
		return err
	}
	return c.writeDocument(fragment, opts, outputPath)
}

func (c *Converter) ConvertFile(inputPath, outputPath string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}
	kind := "markdown"
	if isHTML(inputPath) {
		kind = "html"
	}
	fragment, opts, err := c.renderFragment(data, kind)
	if err != nil {
		return fmt.Errorf("%s: %w", inputPath, err)
	}
	return c.writeDocument(fragment, opts, outputPath)
}

func (c *Converter) ConvertFiles(inputPaths []string, outputPath string) error {
	var parts []string
	opts := c.opts
	for _, inputPath := range inputPaths {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return err
		}
		kind := "markdown"
		if isHTML(inputPath) {
			kind = "html"
		}
		fragment, partOpts, err := c.renderFragment(data, kind)
		if err != nil {
			return fmt.Errorf("%s: %w", inputPath, err)
		}
		// Multi-file conversion treats later files as more specific. This lets a
		// final chapter/frontmatter block override book-wide defaults when needed.
		opts = mergeOptions(opts, partOpts)
		parts = append(parts, `<section class="markpdf-file">`+fragment+`</section>`)
	}
	return c.writeDocument(strings.Join(parts, "\n"), opts, outputPath)
}

func (c *Converter) renderFragment(input []byte, kind string) (string, Options, error) {
	if kind == "html" {
		// HTML input bypasses Goldmark, but it still goes through the same
		// sanitizer boundary as rendered Markdown before browser execution.
		return sanitizeContent(string(input)), c.opts, nil
	}
	result, err := renderMarkdown(input, c.opts)
	if err != nil {
		return "", Options{}, err
	}
	return result.HTML, result.Options, nil
}

func (c *Converter) writeDocument(content string, opts Options, outputPath string) error {
	content, opts = applyDocumentMetadata(content, opts)
	doc, err := buildHTML(content, opts)
	if err != nil {
		return err
	}
	return c.writePDF(doc, opts, outputPath)
}

func (c *Converter) writePDF(html string, opts Options, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil && filepath.Dir(outputPath) != "." {
		return err
	}
	var buf bytes.Buffer
	// Rod streams the PDF reader from Chrome; buffering keeps file writes atomic
	// from the caller's perspective and avoids leaving a partial PDF on failure.
	if err := c.browser.PrintPDF(&buf, html, opts); err != nil {
		return err
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0o644)
}

func isHTML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".html" || ext == ".htm"
}
