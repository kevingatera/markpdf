# markpdf

markpdf converts Markdown or HTML into styled PDFs using Go, Goldmark, and headless Chromium through Rod. It is built for technical reports that need real CSS, Mermaid diagrams, KaTeX math, syntax-highlighted code, tables, cover pages, and repeatable CLI/library workflows.

## Features

- Markdown and HTML input.
- Multiple input files merged into one PDF.
- Built-in themes: `modern`, `academic`, `github`, `atelier`, and `rideau`.
- Report preset with cover page, table of contents, header/footer, print margins, and neutral Mermaid.
- Mermaid diagrams rendered in Chromium for SVG fidelity.
- KaTeX inline and display math rendering.
- Syntax highlighting with practical aliases for API, command, and template snippets.
- Azure DevOps-style `:::` containers plus standard fenced code blocks.
- YAML config files and Markdown frontmatter overrides.
- Go library API for embedding conversion in other tools.

## Install

From source:

```sh
go install github.com/kevingatera/markpdf/cmd/markpdf@latest
```

For local development:

```sh
go build -o bin/markpdf ./cmd/markpdf
```

markpdf uses an installed Chrome or Chromium when available. To download a Rod-managed Chromium binary:

```sh
markpdf browser install
markpdf browser status
```

## CLI Usage

```sh
markpdf input.md -o output.pdf
markpdf page.html -o page.pdf --theme github
markpdf input.md -o report.pdf --theme rideau --report
markpdf chapter1.md chapter2.md chapter3.md -o book.pdf --toc
markpdf input.md -o output.pdf --config markpdf.yaml
markpdf input.md -o output.pdf --css custom.css
markpdf input.md -o output.pdf --watch
markpdf themes
markpdf init
```

`--report` applies polished report defaults without requiring a YAML file: cover page, table of contents, `{{title}}` header, `Page {{page}} of {{pages}}` footer, 24 mm margins, and neutral Mermaid rendering.

Explicit CLI flags override config and frontmatter values:

```sh
markpdf input.md -o output.pdf \
  --theme rideau \
  --title "Reference Architecture" \
  --subtitle "Document Automation Platform" \
  --author "Jane Doe" \
  --page-size A4 \
  --margin 24mm \
  --mermaid-theme neutral \
  --cover \
  --toc
```

## Cover Pages

`--cover` creates a first page using configured metadata or the document content.

- `cover.title` or `title` wins when supplied.
- Otherwise the first `h1` becomes the cover title.
- `subtitle` or `cover.subtitle` renders under the title.
- If no subtitle is configured, an italic paragraph immediately after the first `h1` is treated as the subtitle.
- Compound titles such as `Project: Document Title` or `Project -- Document Title` use the prefix as the subtitle and suffix as the cover title.

## Configuration

Create a starter config:

```sh
markpdf init
```

Example:

```yaml
page:
  size: A4
  orientation: portrait
  margins:
    top: 20mm
    bottom: 20mm
    left: 25mm
    right: 25mm
theme: modern
custom_css: ""
toc: true
cover:
  enabled: false
  title: My Document
  subtitle: Optional subtitle
  author: Jane Doe
  date: auto
header: ""
footer: "{{page}} / {{pages}}"
mermaid:
  theme: default
```

Markdown frontmatter can override the same fields per document:

```markdown
---
title: Quarterly Report
subtitle: Service Quality Review
theme: academic
toc: true
cover:
  enabled: true
---
```

## Themes

- `modern`: editorial layout with strong typography and generous whitespace.
- `academic`: warm manuscript styling for long-form reports.
- `github`: developer-focused Markdown styling with high-contrast code blocks.
- `atelier`: artful report styling with ivory backgrounds and terracotta accents.
- `rideau`: civic report theme inspired by Ottawa waterways, green space, and public-report polish.

`rideau` is not an official City of Ottawa brand package. Treat it as an inspired theme unless formal brand approval is obtained.

## Library Usage

```go
package main

import "github.com/kevingatera/markpdf/pkg/markpdf"

func main() {
	converter, err := markpdf.New(
		markpdf.WithTheme("modern"),
		markpdf.WithPageSize("A4"),
		markpdf.WithTOC(true),
		markpdf.WithCover(true),
	)
	if err != nil {
		panic(err)
	}
	defer converter.Close()

	if err := converter.ConvertFile("input.md", "output.pdf"); err != nil {
		panic(err)
	}
}
```

## Development

Run the standard checks:

```sh
make check
```

Generate a visual sample:

```sh
./bin/markpdf examples/architecture.md -o tmp/release-check.pdf --theme rideau --report
```

The `tmp/` and `bin/` directories are ignored and should not be committed.
