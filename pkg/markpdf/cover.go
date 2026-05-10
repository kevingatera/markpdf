// cover.go derives document-level title metadata and renders the optional title
// page from explicit cover settings, frontmatter, or the first document H1.
package markpdf

import (
	"bytes"
	"html"
	"html/template"
	"regexp"
	"strings"

	"github.com/kevingatera/markpdf/internal"
)

var (
	firstH1RE                   = regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</h1>`)
	italicSubtitleAfterH1       = regexp.MustCompile(`(?is)<h1\b[^>]*>.*?</h1>\s*<p>\s*<(?:em|strong)>(.*?)</(?:em|strong)>\s*</p>`)
	firstH1WithItalicSubtitleRE = regexp.MustCompile(`(?is)<h1\b[^>]*>.*?</h1>\s*<p>\s*<(?:em|strong)>.*?</(?:em|strong)>\s*</p>`)
	leadingParagraphRE          = regexp.MustCompile(`(?is)^\s*<p>(.*?)</p>`)
	leadingHRRE                 = regexp.MustCompile(`(?is)^\s*<hr\s*/?>`)
)

func applyDocumentMetadata(content string, opts Options) (string, Options) {
	documentTitle := firstH1Text(content)
	documentSubtitle := implicitSubtitle(content)
	prefixTitle, suffixTitle := splitCompoundTitle(documentTitle)
	if documentSubtitle == "" {
		documentSubtitle = prefixTitle
	}
	documentDate, documentAuthor := leadingDocumentMetadata(content)

	if opts.Cover.Enabled && opts.Cover.Title == "" {
		if suffixTitle != "" {
			opts.Cover.Title = suffixTitle
		} else {
			opts.Cover.Title = documentTitle
		}
	}
	if opts.Title == "" {
		opts.Title = documentTitle
	}
	if opts.Subtitle == "" && opts.Cover.Subtitle == "" {
		opts.Subtitle = documentSubtitle
	}
	if opts.Cover.Date == "" {
		opts.Cover.Date = documentDate
	}
	if opts.Cover.Author == "" && opts.Author == "" {
		opts.Cover.Author = documentAuthor
	}
	if opts.Cover.Enabled && documentTitle != "" {
		content = removeCoverMetadataFromBody(content)
	}
	return content, opts
}

func buildCover(opts Options) (string, error) {
	if !opts.Cover.Enabled && opts.Cover.Title == "" {
		return "", nil
	}
	data := opts.Cover
	// Cover metadata falls back to document-level metadata so users can define
	// title/author once in frontmatter and opt into a cover without duplication.
	if data.Title == "" {
		data.Title = opts.Title
	}
	if data.Subtitle == "" {
		data.Subtitle = opts.Subtitle
	}
	if data.Author == "" {
		data.Author = opts.Author
	}
	if data.Title == "" && data.Subtitle == "" && data.Author == "" && data.Date == "" {
		return "", nil
	}
	tplBytes, err := internal.FS.ReadFile("templates/cover.html")
	if err != nil {
		return "", err
	}
	tpl, err := template.New("cover").Parse(string(tplBytes))
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

func firstH1Text(content string) string {
	matches := firstH1RE.FindStringSubmatch(content)
	if len(matches) != 2 {
		return ""
	}
	return plainText(matches[1])
}

func implicitSubtitle(content string) string {
	matches := italicSubtitleAfterH1.FindStringSubmatch(content)
	if len(matches) != 2 {
		return ""
	}
	return plainText(matches[1])
}

func splitCompoundTitle(title string) (string, string) {
	for _, delimiter := range []string{":", " -- ", " - "} {
		before, after, ok := strings.Cut(title, delimiter)
		if ok && strings.TrimSpace(before) != "" && strings.TrimSpace(after) != "" {
			return strings.TrimSpace(before), strings.TrimSpace(after)
		}
	}
	return "", ""
}

func leadingDocumentMetadata(content string) (string, string) {
	afterH1 := content
	if loc := firstH1RE.FindStringIndex(content); loc != nil {
		afterH1 = content[loc[1]:]
	}
	date := ""
	author := ""

	for range 3 {
		matches := leadingParagraphRE.FindStringSubmatch(afterH1)
		if len(matches) != 2 {
			break
		}
		text := plainText(matches[1])
		if !isCoverMetadataText(text) {
			break
		}
		if date == "" {
			date = metadataValueBefore(text, "Last updated:", "Prepared by:")
		}
		if author == "" {
			author = metadataValue(text, "Prepared by:")
		}
		afterH1 = afterH1[len(matches[0]):]
	}

	return date, author
}

func metadataValue(text, label string) string {
	index := strings.Index(strings.ToLower(text), strings.ToLower(label))
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(text[index+len(label):])
}

func metadataValueBefore(text, label, stopLabel string) string {
	value := metadataValue(text, label)
	stop := strings.Index(strings.ToLower(value), strings.ToLower(stopLabel))
	if stop >= 0 {
		value = value[:stop]
	}
	return strings.TrimSpace(value)
}

func isCoverMetadataText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "last updated:") || strings.Contains(lower, "prepared by:")
}

func plainText(input string) string {
	withoutTags := regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(input, "")
	return strings.Join(strings.Fields(html.UnescapeString(withoutTags)), " ")
}

func removeCoverMetadataFromBody(content string) string {
	before := content
	content = removeFirstMatch(firstH1WithItalicSubtitleRE, content)
	if content == before {
		content = removeFirstMatch(firstH1RE, content)
	}

	for range 3 {
		matches := leadingParagraphRE.FindStringSubmatch(content)
		if len(matches) != 2 || !isCoverMetadataText(plainText(matches[1])) {
			break
		}
		content = strings.TrimLeft(content[len(matches[0]):], "\n")
	}
	return strings.TrimLeft(leadingHRRE.ReplaceAllString(content, ""), "\n")
}

func removeFirstMatch(pattern *regexp.Regexp, content string) string {
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return content
	}
	return strings.TrimLeft(content[:loc[0]]+content[loc[1]:], "\n")
}
