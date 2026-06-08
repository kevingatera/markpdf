// Command markpdf provides the CLI entrypoint, flag precedence, watch mode, and
// browser/theme management subcommands for the Markdown/HTML to PDF converter.
package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	"github.com/kevingatera/markpdf/pkg/markpdf"
)

var version = "dev"

type cliOptions struct {
	configPath   string
	outputPath   string
	dumpHTMLPath string
	theme        string
	customCSS    string
	title        string
	subtitle     string
	author       string
	header       string
	footer       string
	pageSize     string
	orientation  string
	margin       string
	marginTop    string
	marginRight  string
	marginBottom string
	marginLeft   string
	mermaidTheme string
	watch        bool
	report       bool
	cover        bool
	coverSet     bool
	toc          bool
	tocSet       bool
}

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var cli cliOptions
	cmd := &cobra.Command{
		Use:     "markpdf [input.md|input.html ...]",
		Short:   "Convert Markdown or HTML files to styled PDFs",
		Version: version,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cli.outputPath == "" {
				return errors.New("output path is required; pass -o or --output")
			}
			// Cobra bool flags default to false, so track whether --toc was
			// explicitly supplied before overriding config/frontmatter behavior.
			cli.tocSet = cmd.Flags().Changed("toc")
			cli.coverSet = cmd.Flags().Changed("cover")
			return runConvert(cli, args)
		},
	}
	cmd.Flags().StringVarP(&cli.outputPath, "output", "o", "", "output PDF path")
	cmd.Flags().StringVar(&cli.configPath, "config", "", "YAML configuration file")
	cmd.Flags().StringVar(&cli.theme, "theme", "", "theme name: modern, academic, github, atelier, rideau")
	cmd.Flags().StringVar(&cli.customCSS, "css", "", "custom CSS file path or inline CSS")
	cmd.Flags().StringVar(&cli.title, "title", "", "document title")
	cmd.Flags().StringVar(&cli.subtitle, "subtitle", "", "document subtitle")
	cmd.Flags().StringVar(&cli.author, "author", "", "document author")
	cmd.Flags().StringVar(&cli.header, "header", "", "header HTML/template, supports {{title}}, {{page}}, {{pages}}, {{date}}, {{url}}")
	cmd.Flags().StringVar(&cli.footer, "footer", "", "footer HTML/template, supports {{title}}, {{page}}, {{pages}}, {{date}}, {{url}}")
	cmd.Flags().StringVar(&cli.pageSize, "page-size", "", "page size: A4, Letter, Legal, or custom WxH such as 210mmx297mm")
	cmd.Flags().StringVar(&cli.orientation, "orientation", "", "page orientation: portrait or landscape")
	cmd.Flags().StringVar(&cli.margin, "margin", "", "set all page margins, e.g. 24mm")
	cmd.Flags().StringVar(&cli.marginTop, "margin-top", "", "top page margin")
	cmd.Flags().StringVar(&cli.marginRight, "margin-right", "", "right page margin")
	cmd.Flags().StringVar(&cli.marginBottom, "margin-bottom", "", "bottom page margin")
	cmd.Flags().StringVar(&cli.marginLeft, "margin-left", "", "left page margin")
	cmd.Flags().StringVar(&cli.mermaidTheme, "mermaid-theme", "", "Mermaid theme: default, neutral, dark, forest")
	cmd.Flags().BoolVar(&cli.watch, "watch", false, "regenerate the PDF when inputs change")
	cmd.Flags().BoolVar(&cli.report, "report", false, "apply polished report defaults: cover, TOC, header/footer, 24mm margins, neutral Mermaid")
	cmd.Flags().BoolVar(&cli.cover, "cover", false, "include a title page using cover metadata or the first H1")
	cmd.Flags().BoolVar(&cli.toc, "toc", false, "include a table of contents")
	cmd.Flags().StringVar(&cli.dumpHTMLPath, "dump-html", "", "write intermediate HTML to this path instead of generating PDF")

	cmd.AddCommand(newThemesCommand(), newInitCommand(), newBrowserCommand())
	return cmd
}

func runConvert(cli cliOptions, inputs []string) error {
	convert := func() error {
		opts, err := loadOptions(cli)
		if err != nil {
			return err
		}
		if cli.dumpHTMLPath != "" {
			opts.DumpHTMLPath = cli.dumpHTMLPath
		}
		converter, err := markpdf.NewWithOptions(opts)
		if err != nil {
			return err
		}
		defer converter.Close()
		if len(inputs) == 1 {
			return converter.ConvertFile(inputs[0], cli.outputPath)
		}
		return converter.ConvertFiles(inputs, cli.outputPath)
	}
	if err := convert(); err != nil {
		return err
	}
	if !cli.watch {
		return nil
	}
	return watch(inputs, convert)
}

func loadOptions(cli cliOptions) (markpdf.Options, error) {
	opts, err := markpdf.LoadConfig(cli.configPath)
	if err != nil {
		return opts, err
	}
	if cli.report {
		applyReportDefaults(&opts)
	}
	// CLI flags are the final user intent layer: defaults < config/frontmatter
	// < explicit flags. Only flags with values should override the config file.
	if cli.theme != "" {
		opts.Theme = cli.theme
	}
	if cli.customCSS != "" {
		opts.CustomCSS = cli.customCSS
	}
	if cli.title != "" {
		opts.Title = cli.title
	}
	if cli.subtitle != "" {
		opts.Subtitle = cli.subtitle
	}
	if cli.author != "" {
		opts.Author = cli.author
	}
	if cli.header != "" {
		opts.Header = cli.header
	}
	if cli.footer != "" {
		opts.Footer = cli.footer
	}
	if cli.pageSize != "" {
		opts.Page.Size = cli.pageSize
	}
	if cli.orientation != "" {
		opts.Page.Orientation = cli.orientation
	}
	if cli.margin != "" {
		opts.Page.Margins.Top = cli.margin
		opts.Page.Margins.Right = cli.margin
		opts.Page.Margins.Bottom = cli.margin
		opts.Page.Margins.Left = cli.margin
	}
	if cli.marginTop != "" {
		opts.Page.Margins.Top = cli.marginTop
	}
	if cli.marginRight != "" {
		opts.Page.Margins.Right = cli.marginRight
	}
	if cli.marginBottom != "" {
		opts.Page.Margins.Bottom = cli.marginBottom
	}
	if cli.marginLeft != "" {
		opts.Page.Margins.Left = cli.marginLeft
	}
	if cli.mermaidTheme != "" {
		opts.Mermaid.Theme = cli.mermaidTheme
	}
	if cli.tocSet {
		opts.TOC = cli.toc
	}
	if cli.coverSet {
		opts.Cover.Enabled = cli.cover
	}
	return opts, nil
}

func applyReportDefaults(opts *markpdf.Options) {
	opts.TOC = true
	opts.Cover.Enabled = true
	opts.Header = "{{title}}"
	opts.Footer = "Page {{page}} of {{pages}}"
	opts.Mermaid.Theme = "neutral"
	opts.Page.Margins.Top = "24mm"
	opts.Page.Margins.Right = "24mm"
	opts.Page.Margins.Bottom = "24mm"
	opts.Page.Margins.Left = "24mm"
}

func watch(inputs []string, convert func() error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	for _, input := range inputs {
		if err := watcher.Add(input); err != nil {
			return err
		}
	}
	fmt.Println("watching for changes...")
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				// Editors often save via temp-file create/write bursts. A short
				// debounce avoids rendering while the file is still being replaced.
				time.Sleep(150 * time.Millisecond)
				if err := convert(); err != nil {
					fmt.Fprintln(os.Stderr, err)
				} else {
					fmt.Println("regenerated", time.Now().Format(time.RFC3339))
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func newThemesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "themes",
		Short: "List built-in themes",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join([]string{
				"modern    – editorial sophistication (serif headings, strong rules, deep navy)",
				"academic  – literary manuscript warmth (justified text, cream paper, crimson accent)",
				"github    – swiss engineering precision (grid starkness, black borders, orange accent)",
				"atelier   – artful warmth (terracotta accents, gallery ivory, elegant italics)",
				"rideau    – civic clarity (Ottawa-inspired blue/green palette, report polish)",
			}, "\n"))
		},
	}
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create an example markpdf.yaml in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "markpdf.yaml"
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists", path)
			}
			return os.WriteFile(path, []byte(exampleConfig), 0o644)
		},
	}
}

func newBrowserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Manage Chromium used for PDF generation",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show detected browser path",
		Run: func(cmd *cobra.Command, args []string) {
			status := markpdf.DetectBrowser()
			if status.Path == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "no browser detected")
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), status.Path)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Download a Chromium binary managed by rod",
		RunE: func(cmd *cobra.Command, args []string) error {
			return markpdf.InstallBrowser()
		},
	})
	return cmd
}

const exampleConfig = `page:
  size: A4
  orientation: portrait
  margins:
    top: 20mm
    bottom: 20mm
    left: 25mm
    right: 25mm
theme: modern
toc: true
cover:
  enabled: false
  title: My Document
  subtitle: Optional subtitle
  author: Jane Doe
  date: auto
header: ''
footer: '{{page}} / {{pages}}'
mermaid:
  theme: default
`
