// pdf.go contains page-size and length conversion helpers needed to bridge CSS
// configuration with Chrome DevTools Protocol PDF dimensions.
package markpdf

import (
	"strconv"
	"strings"
)

const (
	mmPerInch = 25.4
	cmPerInch = 2.54
)

func pageSizeInches(size, orientation string) (float64, float64) {
	// Chrome's CDP API requires inches even when CSS @page uses A4/Letter/mm.
	// Keep this conversion centralized so custom and named page sizes agree.
	width, height := 8.27, 11.69
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "letter":
		width, height = 8.5, 11
	case "legal":
		width, height = 8.5, 14
	case "a4", "":
		width, height = 8.27, 11.69
	default:
		if w, h, ok := parseCustomPageSize(size); ok {
			width, height = w, h
		}
	}
	switch strings.ToLower(strings.TrimSpace(orientation)) {
	case "landscape":
		if height > width {
			return height, width
		}
	case "portrait", "":
		if width > height {
			return height, width
		}
	}
	return width, height
}

func parseCustomPageSize(size string) (float64, float64, bool) {
	// Config accepts simple "WxH" values such as "210mmx297mm". More complex
	// CSS page-size syntax is intentionally left to pageSizeCSS for @page.
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width := parseLengthInches(parts[0])
	height := parseLengthInches(parts[1])
	return width, height, width > 0 && height > 0
}

func parseLengthInches(value string) float64 {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasSuffix(value, "mm"):
		return parseFloat(strings.TrimSuffix(value, "mm")) / mmPerInch
	case strings.HasSuffix(value, "cm"):
		return parseFloat(strings.TrimSuffix(value, "cm")) / cmPerInch
	case strings.HasSuffix(value, "in"):
		return parseFloat(strings.TrimSuffix(value, "in"))
	default:
		return parseFloat(value)
	}
}

func parseFloat(value string) float64 {
	// Invalid lengths collapse to zero, letting callers fall back to safe
	// defaults instead of partially applying malformed page settings.
	out, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return out
}
