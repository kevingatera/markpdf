# Makefile collects local developer commands used by CI and release checks.
.PHONY: build test vet fmt fmt-check tidy check clean

build:
	go build -o bin/markpdf ./cmd/markpdf

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./pkg ./internal

fmt-check:
	test -z "$$(gofmt -l ./cmd ./pkg ./internal)"

tidy:
	go mod tidy

check: fmt-check vet test build

clean:
	rm -rf bin tmp markpdf markpdf.exe coverage.out coverage.html
