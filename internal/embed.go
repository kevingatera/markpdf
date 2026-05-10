// Package internal embeds all runtime assets, templates, and themes so markpdf
// can run as a single binary without external asset files at execution time.
package internal

import "embed"

//go:embed themes/*.css templates/*.html assets/*
var FS embed.FS
