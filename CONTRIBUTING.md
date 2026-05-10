# Contributing

Thanks for improving markpdf. This project is still pre-release, so prefer small, reviewable changes that keep the CLI and library APIs understandable.

## Local Checks

Run the same checks CI runs before opening a pull request:

```sh
make check
```

If you do not use `make`, run the commands directly:

```sh
gofmt -w ./cmd ./pkg ./internal
go vet ./...
go test ./...
go build -o bin/markpdf ./cmd/markpdf
```

## Rendering Checks

Rendering bugs are usually visual. When changing CSS, templates, Mermaid handling, headers/footers, or browser timing, generate at least one sample PDF:

```sh
./bin/markpdf examples/architecture.md -o tmp/release-check.pdf --theme rideau --report
```

Inspect the cover, table of contents, diagrams, wide tables, code blocks, and header/footer areas. Pay special attention to page breaks and clipped text.

## Code Style

- Keep package boundaries clear: CLI parsing stays under `cmd/markpdf`, public conversion APIs stay under `pkg/markpdf`, and embedded assets stay under `internal`.
- Prefer explicit errors over silent fallbacks when a missing asset or invalid option would make output misleading.
- Add comments when code handles a browser, print-layout, or sanitization edge case that is not obvious from the implementation.
- Do not commit generated PDFs, browser profiles, binaries, or files under `tmp/`.
