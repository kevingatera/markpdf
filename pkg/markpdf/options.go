// options.go defines the user-facing configuration model plus sparse override
// behavior shared by config files, CLI flags, and Markdown frontmatter.
package markpdf

import "time"

type Options struct {
	Page      PageOptions    `yaml:"page"`
	Theme     string         `yaml:"theme"`
	CustomCSS string         `yaml:"custom_css"`
	TOC       bool           `yaml:"toc"`
	Cover     CoverOptions   `yaml:"cover"`
	Header    string         `yaml:"header"`
	Footer    string         `yaml:"footer"`
	Mermaid   MermaidOptions `yaml:"mermaid"`
	Title     string         `yaml:"title"`
	Subtitle  string         `yaml:"subtitle"`
	Author    string         `yaml:"author"`
}

type PageOptions struct {
	Size        string        `yaml:"size"`
	Orientation string        `yaml:"orientation"`
	Margins     MarginOptions `yaml:"margins"`
}

type MarginOptions struct {
	Top    string `yaml:"top"`
	Bottom string `yaml:"bottom"`
	Left   string `yaml:"left"`
	Right  string `yaml:"right"`
}

type CoverOptions struct {
	Enabled  bool   `yaml:"enabled"`
	Title    string `yaml:"title"`
	Subtitle string `yaml:"subtitle"`
	Author   string `yaml:"author"`
	Date     string `yaml:"date"`
}

type MermaidOptions struct {
	Theme string `yaml:"theme"`
}

type Option func(*Options)

func DefaultOptions() Options {
	return Options{
		Page: PageOptions{
			Size:        "A4",
			Orientation: "portrait",
			Margins: MarginOptions{
				Top:    "20mm",
				Bottom: "20mm",
				Left:   "25mm",
				Right:  "25mm",
			},
		},
		Theme:   "modern",
		Mermaid: MermaidOptions{Theme: "default"},
	}
}

func WithTheme(theme string) Option {
	return func(o *Options) { o.Theme = theme }
}

func WithPageSize(size string) Option {
	return func(o *Options) { o.Page.Size = size }
}

func WithTOC(enabled bool) Option {
	return func(o *Options) { o.TOC = enabled }
}

func WithCustomCSS(css string) Option {
	return func(o *Options) { o.CustomCSS = css }
}

func WithTitle(title string) Option {
	return func(o *Options) { o.Title = title }
}

func WithSubtitle(subtitle string) Option {
	return func(o *Options) { o.Subtitle = subtitle }
}

func WithCover(enabled bool) Option {
	return func(o *Options) { o.Cover.Enabled = enabled }
}

func (o Options) normalized() Options {
	// Options are loaded from YAML/frontmatter where omitted fields arrive as
	// zero values. Normalize once at API boundaries so rendering code can assume
	// usable page, theme, and Mermaid defaults.
	if o.Page.Size == "" {
		o.Page.Size = "A4"
	}
	if o.Page.Orientation == "" {
		o.Page.Orientation = "portrait"
	}
	if o.Page.Margins.Top == "" {
		o.Page.Margins.Top = "20mm"
	}
	if o.Page.Margins.Bottom == "" {
		o.Page.Margins.Bottom = "20mm"
	}
	if o.Page.Margins.Left == "" {
		o.Page.Margins.Left = "25mm"
	}
	if o.Page.Margins.Right == "" {
		o.Page.Margins.Right = "25mm"
	}
	if o.Theme == "" {
		o.Theme = "modern"
	}
	if o.Mermaid.Theme == "" {
		o.Mermaid.Theme = "default"
	}
	if o.Cover.Date == "auto" {
		o.Cover.Date = time.Now().Format("January 2, 2006")
	}
	return o
}

func mergeOptions(base Options, override Options) Options {
	// Treat override as a sparse patch. String fields only replace when present;
	// bool fields currently only support opt-in overrides because false is also
	// the Go zero value for "not specified" after YAML unmarshalling.
	if override.Page.Size != "" {
		base.Page.Size = override.Page.Size
	}
	if override.Page.Orientation != "" {
		base.Page.Orientation = override.Page.Orientation
	}
	if override.Page.Margins.Top != "" {
		base.Page.Margins.Top = override.Page.Margins.Top
	}
	if override.Page.Margins.Bottom != "" {
		base.Page.Margins.Bottom = override.Page.Margins.Bottom
	}
	if override.Page.Margins.Left != "" {
		base.Page.Margins.Left = override.Page.Margins.Left
	}
	if override.Page.Margins.Right != "" {
		base.Page.Margins.Right = override.Page.Margins.Right
	}
	if override.Theme != "" {
		base.Theme = override.Theme
	}
	if override.CustomCSS != "" {
		base.CustomCSS = override.CustomCSS
	}
	if override.TOC {
		base.TOC = true
	}
	if override.Cover.Enabled {
		base.Cover.Enabled = true
	}
	if override.Cover.Title != "" {
		base.Cover.Title = override.Cover.Title
	}
	if override.Cover.Subtitle != "" {
		base.Cover.Subtitle = override.Cover.Subtitle
	}
	if override.Cover.Author != "" {
		base.Cover.Author = override.Cover.Author
	}
	if override.Cover.Date != "" {
		base.Cover.Date = override.Cover.Date
	}
	if override.Header != "" {
		base.Header = override.Header
	}
	if override.Footer != "" {
		base.Footer = override.Footer
	}
	if override.Mermaid.Theme != "" {
		base.Mermaid.Theme = override.Mermaid.Theme
	}
	if override.Title != "" {
		base.Title = override.Title
	}
	if override.Subtitle != "" {
		base.Subtitle = override.Subtitle
	}
	if override.Author != "" {
		base.Author = override.Author
	}
	return base.normalized()
}
