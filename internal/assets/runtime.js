/*
 * runtime.js is the browser-side print pipeline. It normalizes code highlighting,
 * renders KaTeX and Mermaid, sizes diagrams for PDF pagination, and signals Go
 * when the document is safe to snapshot through Chrome's PrintToPDF API.
 */
(() => {
  "use strict";

  const config = window.markpdfConfig || {};

  window.markpdfReady = false;
  window.markpdfRenderError = "";

  // Go waits on markpdfReady before invoking Chrome's PrintToPDF command.
  // Keep all browser-only rendering work behind this single synchronization point.
  runRenderPipeline();

  async function runRenderPipeline() {
    try {
      wrapTables();
      groupDiagramHeadings();
      highlightCodeBlocks();
      renderMath();
      await renderMermaidDiagrams();
      populateTOCPageNumbers();
    } catch (error) {
      window.markpdfRenderError = errorMessage(error);
      console.error(error);
    } finally {
      window.markpdfReady = true;
    }
  }

  // Chrome prints tables more reliably when wide tables are isolated in a wrapper.
  function wrapTables() {
    document.querySelectorAll(".markpdf-content table").forEach((table) => {
      if (table.parentElement && table.parentElement.classList.contains("markpdf-table-wrapper")) {
        return;
      }

      const wrapper = document.createElement("div");
      wrapper.className = "markpdf-table-wrapper";
      table.parentNode.insertBefore(wrapper, table);
      wrapper.appendChild(table);
    });
  }

  function groupDiagramHeadings() {
    document.querySelectorAll(".markpdf-content .mermaid").forEach((diagram) => {
      const heading = headingForDiagramGroup(diagram);
      if (!heading || !/^H[1-6]$/.test(heading.tagName) || heading.parentElement.classList.contains("markpdf-heading-diagram-group")) {
        return;
      }

      // Chromium respects break-inside more consistently on an explicit block
      // wrapper than on a loose heading followed by rendered SVG content.
      // Portrait page-hinted diagrams may include a short paragraph lead-in, so
      // move every sibling from the heading through the diagram into the group.
      const group = document.createElement("div");
      group.className = "markpdf-heading-diagram-group";
      heading.parentNode.insertBefore(group, heading);
      let child = heading;
      while (child) {
        const next = child.nextSibling;
        group.appendChild(child);
        if (child === diagram) break;
        child = next;
      }
    });
  }

  function headingForDiagramGroup(diagram) {
    const hints = mermaidPrintHints(diagram.textContent);
    if (hints.forcePage && !hints.forceLandscape && !hints.detachHeading) {
      // A portrait page hint is usually author intent for "this section's
      // diagram deserves breathing room." Keep a short textual lead-in with the
      // diagram so the preceding page is not mostly blank.
      return nearestLeadInHeading(diagram);
    }
    return previousMeaningfulElement(diagram);
  }

  function nearestLeadInHeading(diagram) {
    let current = diagram.previousElementSibling;
    while (current) {
      if (/^H[1-6]$/.test(current.tagName)) return current;
      if (!isDiagramLeadInElement(current)) return null;
      current = current.previousElementSibling;
    }
    return null;
  }

  function isDiagramLeadInElement(element) {
    // Keep this intentionally narrow. Pulling lists, tables, or code blocks
    // into an avoid-break diagram wrapper can create the same blank-page
    // problems the wrapper is meant to prevent.
    return element.tagName === "P" || element.tagName === "HR";
  }

  function previousMeaningfulElement(element) {
    let previous = element.previousElementSibling;
    while (previous && previous.tagName === "HR") {
      previous = previous.previousElementSibling;
    }
    return previous;
  }

  // Highlight.js is good at common languages, but docs often use practical aliases.
  function highlightCodeBlocks() {
    if (!window.hljs) return;

    registerHighlightLanguages();
    document.querySelectorAll("pre > code").forEach((code) => highlightCodeBlock(code));
  }

  function highlightCodeBlock(code) {
    const pre = code.parentElement;
    if (!pre || pre.classList.contains("mermaid")) return;

    const originalLanguage = codeLanguage(code);
    const source = code.textContent || "";
    const language = normalizeHighlightLanguage(originalLanguage, source);
    const displayLanguage = codeLanguageLabel(originalLanguage || language);

    pre.classList.add("markpdf-code-block");
    if (displayLanguage) pre.setAttribute("data-language", displayLanguage);

    try {
      if (language && window.hljs.getLanguage(language)) {
        setCodeLanguage(code, language);
        code.innerHTML = window.hljs.highlight(source, {
          language,
          ignoreIllegals: true
        }).value;
        code.classList.add("hljs");
      } else {
        window.hljs.highlightElement(code);
      }
    } catch (error) {
      code.textContent = source;
    }
  }

  function registerHighlightLanguages() {
    if (window.markpdfHighlightLanguagesRegistered) return;
    window.markpdfHighlightLanguagesRegistered = true;

    window.hljs.registerLanguage("markpdf-command", commandLanguage);
    window.hljs.registerLanguage("markpdf-http", httpLanguage);
    window.hljs.registerLanguage("markpdf-template", templateLanguage);
  }

  function commandLanguage(hljs) {
    // This is intentionally not a full shell or PowerShell grammar. It catches
    // prompts, common CLIs, flags, and variables without mis-highlighting output.
    return {
      name: "Command Session",
      aliases: [
        "command",
        "commands",
        "cmd",
        "cli",
        "console",
        "powershell",
        "ps1",
        "pwsh",
        "shell-session",
        "shellsession",
        "terminal"
      ],
      contains: [
        hljs.HASH_COMMENT_MODE,
        {
          className: "meta",
          begin: /^\s*(?:[$#>]|PS [^>\n]+>|[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+:[^\n$#]*[$#])(?=\s)/m
        },
        {
          className: "built_in",
          begin: /\b(?:[A-Z][A-Za-z]+-[A-Z][A-Za-z]+|az|aws|curl|docker|git|go|kubectl|markpdf|node|npm|pnpm|python3?|uv)\b/
        },
        {
          className: "variable",
          begin: /\$\{?[A-Za-z_][A-Za-z0-9_]*\}?/
        },
        {
          className: "literal",
          begin: /--?[A-Za-z][A-Za-z0-9-]*/
        },
        hljs.QUOTE_STRING_MODE,
        hljs.APOS_STRING_MODE
      ]
    };
  }

  function templateLanguage(hljs) {
    // Partial snippets are usually HTML with {{mustache}} directives. A tiny
    // grammar is enough to make template variables stand out in PDF examples.
    return {
      name: "Template Partial",
      aliases: ["partial", "partials", "template", "tmpl", "handlebars", "hbs", "mustache"],
      contains: [
        {
          className: "comment",
          begin: /\{\{!/,
          end: /\}\}/
        },
        {
          className: "template-tag",
          begin: /\{\{[#/>!]?\s*/,
          end: /\}\}/,
          contains: [
            { className: "keyword", begin: /[#/>!]/ },
            { className: "name", begin: /[A-Za-z_][\w.-]*/ },
            { className: "attr", begin: /[A-Za-z_][\w.-]*(?=\=)/ },
            hljs.QUOTE_STRING_MODE,
            hljs.APOS_STRING_MODE
          ]
        },
        {
          className: "tag",
          begin: /<\/?[A-Za-z][\w:-]*/,
          end: /\/?>/,
          contains: [
            { className: "name", begin: /[A-Za-z][\w:-]*/ },
            { className: "attr", begin: /\s[A-Za-z_:][\w:.-]*(?=\=|\s|\/?>)/ },
            hljs.QUOTE_STRING_MODE,
            hljs.APOS_STRING_MODE
          ]
        }
      ]
    };
  }

  function httpLanguage(hljs) {
    // The embedded highlight.js build may not include HTTP. This small grammar
    // covers request/status lines, wrapped query strings, and headers for API
    // documentation examples. It intentionally highlights parts of a request
    // instead of treating the whole line as "meta" so examples like
    // `GET http://localhost:8000/chat-stream?message=...` have useful contrast.
    const queryParts = [
      {
        className: "operator",
        begin: /[?&]/
      },
      {
        className: "attr",
        begin: /[?&]?[A-Za-z_][\w.-]*(?==)/
      },
      {
        className: "string",
        begin: /=/,
        end: /(?=&|\s|$)/,
        excludeBegin: true
      }
    ];

    return {
      name: "HTTP/API",
      aliases: ["api", "endpoint", "http", "http-request", "http-response", "request", "response", "rest"],
      contains: [
        {
          className: "meta",
          begin: /^HTTP\/\d(?:\.\d)?\s+\d{3}.*$/m,
          contains: [
            {
              className: "number",
              begin: /\b\d{3}\b/
            }
          ]
        },
        {
          begin: /^(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+/m,
          end: /$/,
          returnBegin: true,
          contains: [
            {
              className: "keyword",
              begin: /^(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)(?=\s)/
            },
            {
              className: "string",
              begin: /https?:\/\/[^\s?&]+|\/[^\s?&]*/
            },
            {
              className: "meta",
              begin: /HTTP\/\d(?:\.\d)?/
            },
            ...queryParts
          ]
        },
        {
          // Authors often wrap a long request after `?` to keep examples readable.
          // Treat the continuation as query parameters instead of plain text.
          begin: /^[?&]?[A-Za-z_][\w.-]*=/m,
          end: /$/,
          returnBegin: true,
          contains: queryParts
        },
        {
          begin: /^[A-Za-z0-9-]+:\s*/m,
          end: /$/,
          returnBegin: true,
          contains: [
            {
              className: "attr",
              begin: /^[A-Za-z0-9-]+(?=:)/
            },
            {
              className: "string",
              begin: /:\s*/,
              end: /$/,
              excludeBegin: true
            }
          ]
        },
        {
          className: "literal",
          begin: /\b(?:true|false|null)\b/
        },
        {
          className: "number",
          begin: /\b\d+(?:\.\d+)?\b/
        },
        hljs.QUOTE_STRING_MODE
      ]
    };
  }

  function setCodeLanguage(code, language) {
    const classes = code.className
      .split(/\s+/)
      .filter((name) => name && !/^language-/.test(name) && !/^lang-/.test(name));

    code.className = classes.join(" ");
    code.classList.add(`language-${language}`);
  }

  function codeLanguage(code) {
    for (const className of code.classList) {
      const match = /^(?:language|lang)-(.+)$/.exec(className);
      if (match) return match[1].toLowerCase();
    }
    return "";
  }

  function normalizeHighlightLanguage(language, source) {
    const normalized = normalizeLanguageName(language);
    const aliases = {
      "c#": "csharp",
      "command": "markpdf-command",
      "commands": "markpdf-command",
      "cmd": "markpdf-command",
      "console": "markpdf-command",
      "cs": "csharp",
      "dockerfile": "markpdf-command",
      "dotenv": "ini",
      "env": "ini",
      "handlebars": "markpdf-template",
      "hbs": "markpdf-template",
      "http": "markpdf-http",
      "http-request": "markpdf-http",
      "http-response": "markpdf-http",
      "js": "javascript",
      "mustache": "markpdf-template",
      "partial": "markpdf-template",
      "partials": "markpdf-template",
      "powershell": "markpdf-command",
      "ps1": "markpdf-command",
      "pwsh": "markpdf-command",
      "request": "markpdf-http",
      "response": "markpdf-http",
      "rest": "markpdf-http",
      "shell-session": "markpdf-command",
      "shellsession": "markpdf-command",
      "terminal": "markpdf-command",
      "template": "markpdf-template",
      "tmpl": "markpdf-template",
      "ts": "typescript",
      "yml": "yaml"
    };

    if (!normalized) return inferUnlabeledLanguage(source);
    // "api" is ambiguous in prose, so infer from the block body when possible:
    // HTTP request/response, JSON payload, or YAML-ish configuration.
    if (isAPILanguage(normalized)) return inferAPILanguage(source);
    return aliases[normalized] || normalized;
  }

  function isAPILanguage(language) {
    return ["api", "endpoint", "http-request", "http-response", "request", "response", "rest"].includes(language);
  }

  function inferAPILanguage(source) {
    const trimmed = source.trim();

    if (looksLikeHTTPRequest(trimmed)) return "markpdf-http";
    if (looksLikeHTTPResponse(trimmed)) return "markpdf-http";
    if (/^[\[{]/.test(trimmed)) return "json";
    if (/^[A-Za-z0-9_.-]+:\s+/m.test(trimmed)) return "yaml";

    return "markpdf-http";
  }

  function inferUnlabeledLanguage(source) {
    const trimmed = source.trim();
    if (!trimmed) return "";

    // Only infer unlabeled snippets when the shape is strong. Generic prose,
    // stack traces, and code in other languages should still be left to hljs.
    if (looksLikeHTTPRequest(trimmed) || looksLikeHTTPResponse(trimmed)) return "markpdf-http";
    if (/^[\[{]/.test(trimmed)) return "json";
    return "";
  }

  function looksLikeHTTPRequest(source) {
    return /^(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+(?:https?:\/\/|\/)\S+/im.test(source);
  }

  function looksLikeHTTPResponse(source) {
    return /^HTTP\/\d(?:\.\d)?\s+\d{3}/im.test(source);
  }

  function codeLanguageLabel(language) {
    const normalized = normalizeLanguageName(language);
    const labels = {
      "api": "API",
      "cmd": "COMMAND",
      "commands": "COMMANDS",
      "http": "HTTP",
      "http-request": "HTTP",
      "http-response": "HTTP",
      "json": "JSON",
      "markpdf-command": "COMMAND",
      "markpdf-http": "HTTP",
      "markpdf-template": "TEMPLATE",
      "partial": "PARTIAL",
      "powershell": "POWERSHELL",
      "ps1": "POWERSHELL",
      "pwsh": "POWERSHELL",
      "request": "REQUEST",
      "response": "RESPONSE",
      "rest": "REST",
      "sh": "SHELL",
      "shell": "SHELL",
      "shell-session": "SHELL",
      "terminal": "TERMINAL",
      "yaml": "YAML",
      "yml": "YAML"
    };

    return labels[normalized] || normalized.toUpperCase();
  }

  function normalizeLanguageName(language) {
    return (language || "").toLowerCase().replace(/_/g, "-");
  }

  function renderMath() {
    if (!window.katex) return;

    document.querySelectorAll(".math-display").forEach((element) => {
      const source = element.textContent;
      try {
        window.katex.render(source, element, {
          displayMode: true,
          throwOnError: false,
          strict: "warn"
        });
      } catch (error) {
        renderError(element, "KaTeX", error, source);
      }
    });
  }

  async function renderMermaidDiagrams() {
    if (!window.mermaid) return;

    window.mermaid.initialize(mermaidRenderConfig({}));

    const diagrams = Array.from(document.querySelectorAll(".mermaid"));
    for (const [index, element] of diagrams.entries()) {
      await renderMermaidDiagram(element, index);
    }
  }

  async function renderMermaidDiagram(element, index) {
    const source = normalizeMermaidSource(element.textContent);
    const stats = mermaidSourceStats(source);

    try {
      window.mermaid.initialize(mermaidRenderConfig(stats));
      const result = await window.mermaid.render(`markpdf-mermaid-${index}`, source, element);
      element.innerHTML = result.svg;
      if (result.bindFunctions) result.bindFunctions(element);
      fitDiagram(element, stats);
    } catch (error) {
      renderError(element, "Mermaid", error, source);
    }
  }

  function mermaidRenderConfig(stats) {
    const flowchart = {
      // Mermaid's default max-width behavior fights our own print scaling.
      // Fixed spacing gives tall state diagrams a chance to fit one page.
      useMaxWidth: false,
      htmlLabels: true,
      nodeSpacing: 32,
      rankSpacing: 36,
      padding: 8
    };

    if (shouldUseBalancedFlowchartLayout(stats)) {
      // Subgraph-heavy system diagrams can become extremely wide and shallow
      // under Mermaid's default spacing. The generous vertical spacing is only
      // useful when markpdf is free to move the diagram to landscape; diagrams
      // explicitly hinted as portrait pages need a denser layout so internal
      // whitespace does not consume the whole page.
      const portraitPageHint = stats.forcePage && !stats.forceLandscape;
      flowchart.nodeSpacing = portraitPageHint ? 8 : 12;
      flowchart.rankSpacing = portraitPageHint ? 36 : 176;
      flowchart.wrappingWidth = portraitPageHint ? 96 : 112;
      flowchart.padding = portraitPageHint ? 24 : 6;
    }

    return {
      startOnLoad: false,
      theme: config.mermaidTheme || "default",
      flowchart
    };
  }

  function shouldUseBalancedFlowchartLayout(stats) {
    if (!stats.isFlowchart) return false;
    return stats.subgraphs >= 5 || (stats.lines >= 56 && stats.edges >= 18);
  }

  function normalizeMermaidSource(source) {
    // Mermaid diagrams in Markdown commonly use "\n" inside quoted labels.
    // Recent Mermaid/Chromium combinations can print those literally; <br/>
    // keeps the author's intended line breaks and reduces oversized label boxes.
    return (source || "").replace(/\\n/g, "<br/>");
  }

  // Mermaid emits SVGs with wildly different aspect ratios; this keeps them printable.
  function fitDiagram(element, stats) {
    const svg = element.querySelector("svg");
    if (!svg) return;

    trimMermaidSvgViewport(svg);

    const size = readSvgSize(svg);
    if (!size.width || !size.height) return;

    if (!svg.getAttribute("viewBox")) {
      svg.setAttribute("viewBox", `0 0 ${size.width} ${size.height}`);
    }
    svg.removeAttribute("width");
    svg.removeAttribute("height");
    svg.setAttribute("preserveAspectRatio", "xMidYMid meet");

    const containerWidth =
      element.clientWidth ||
      (element.parentElement && element.parentElement.clientWidth) ||
      size.width;
    const print = printMetrics(containerWidth);
    const useLandscape = shouldUseLandscapePage(size, stats, print);
    const useForcedPage = useLandscape || stats.forcePage;
    const layoutWidth = useLandscape ? Math.max(containerWidth, print.landscapeContentWidth) : containerWidth;
    const layoutHeight = useLandscape
      ? landscapeDiagramMaxHeight(size, print)
      : portraitForcedDiagramMaxHeight(stats, print);
    const layout = diagramLayout(size.width, size.height, layoutWidth, layoutHeight);

    if (useLandscape) {
      // Chromium still lays out the element in the surrounding portrait flow
      // before assigning it to the named landscape page. Expand and offset the
      // block so the SVG is centered in the landscape printable area instead of
      // overflowing to the right from a portrait-width container.
      const landscapeBleed = Math.max(0, (layoutWidth - containerWidth) / 2);
      element.style.width = `${Math.round(layoutWidth)}px`;
      element.style.maxWidth = "none";
      element.style.marginLeft = `-${Math.round(landscapeBleed)}px`;
      element.style.marginRight = `-${Math.round(landscapeBleed)}px`;
    } else {
      element.style.removeProperty("width");
      element.style.removeProperty("max-width");
      element.style.removeProperty("margin-left");
      element.style.removeProperty("margin-right");
    }

    svg.style.width = `${Math.round(layout.width)}px`;
    svg.style.maxWidth = "100%";
    svg.style.height = layout.maxHeight ? `${Math.round(layout.width / layout.ratio)}px` : "auto";
    if (layout.maxHeight) {
      const maxHeight = `${Math.round(layout.maxHeight)}px`;
      svg.style.maxHeight = maxHeight;
      element.style.setProperty("--markpdf-diagram-max-height", maxHeight);
    } else {
      svg.style.removeProperty("max-height");
      element.style.removeProperty("--markpdf-diagram-max-height");
    }

    element.classList.toggle("markpdf-diagram-wide", layout.mode === "fit-width" && layout.ratio > 1.35);
    element.classList.toggle("markpdf-diagram-tall", layout.ratio < 0.65);
    element.classList.toggle("markpdf-diagram-fit-page", layout.mode === "fit-page");
    element.classList.toggle("markpdf-diagram-oversized", layout.mode === "oversized");
    element.classList.toggle("markpdf-diagram-landscape", useLandscape);
    applyDiagramPageClass(element, useLandscape, useForcedPage, layout.mode === "oversized", stats.detachHeading);

    const isSmallDiagram = size.width < containerWidth * 0.72 && size.height < layout.maxReadableHeight * 0.72;
    element.classList.toggle("markpdf-diagram-small", isSmallDiagram);
  }

  function printMetrics(containerWidth) {
    // These values are calculated in Go from the effective print page and
    // margin settings. Fallbacks keep programmatic HTML tests usable if the
    // runtime is embedded without the full markpdf template.
    const print = config.print || {};
    const portraitWidth = positiveNumber(print.portraitContentWidth, containerWidth || 780);
    const portraitHeight = positiveNumber(print.portraitContentHeight, Math.max(720, portraitWidth * 1.25));
    const landscapeWidth = positiveNumber(print.landscapeContentWidth, Math.max(portraitWidth, portraitWidth * 1.42));
    const landscapeHeight = positiveNumber(print.landscapeContentHeight, Math.max(420, portraitHeight * 0.72));

    return {
      portraitContentWidth: portraitWidth,
      portraitContentHeight: portraitHeight,
      landscapeContentWidth: landscapeWidth,
      landscapeContentHeight: landscapeHeight
    };
  }

  function positiveNumber(value, fallback) {
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
  }

  function shouldUseLandscapePage(size, stats, print) {
    // Landscape is only helpful for complex diagrams that are wider than the
    // portrait printable area. Tall state charts should stay portrait so they
    // can use the existing page-fit scaling instead of becoming shorter.
    const ratio = size.width / size.height;
    if (stats.forceLandscape) return true;
    if (stats.forcePage) return false;
    if (!stats.isFlowchart) return false;
    if (ratio < 0.85) return false;
    if (print.landscapeContentWidth < print.portraitContentWidth * 1.16) return false;

    const tooShallowToBenefit = size.height < 300 || stats.lines < 52;
    if (tooShallowToBenefit) return false;

    const widerThanPortrait = size.width > print.portraitContentWidth * 1.28;
    const complexEnough = stats.subgraphs >= 5 || stats.edges >= 18;
    return widerThanPortrait && complexEnough;
  }

  function mermaidSourceStats(source) {
    // Source size is a useful proxy for graph complexity, but init directives
    // and style-only lines should not push a simple diagram onto its own page.
    const hints = mermaidPrintHints(source);
    const lines = (source || "")
      .split(/\n/)
      .filter((line) => {
        const trimmed = line.trim();
        return trimmed && !trimmed.startsWith("%%") && !trimmed.startsWith("style ");
      });

    return {
      lines: lines.length,
      edges: lines.filter((line) => /-->|---|-.->|==>/.test(line)).length,
      subgraphs: lines.filter((line) => /^\s*subgraph\b/i.test(line)).length,
      isFlowchart: lines.some((line) => /^\s*(?:flowchart|graph)\b/i.test(line)),
      forcePage: hints.forcePage,
      forceLandscape: hints.forceLandscape,
      detachHeading: hints.detachHeading
    };
  }

  function mermaidPrintHints(source) {
    // Authors sometimes know a diagram needs print treatment that heuristics
    // should not apply globally. Mermaid ignores %% comments, so these hints
    // are safe in normal Mermaid renderers while giving markpdf precise control.
    const hints = { forcePage: false, forceLandscape: false, detachHeading: false };
    (source || "").split(/\n/).forEach((line) => {
      const match = line.match(/^\s*%%\s*markpdf\s*:\s*(.+)$/i);
      if (!match) return;

      const tokens = match[1].toLowerCase().split(/[\s,]+/);
      hints.forceLandscape = hints.forceLandscape || tokens.includes("landscape");
      hints.forcePage = hints.forcePage ||
        hints.forceLandscape ||
        tokens.includes("page") ||
        tokens.includes("page-break");
      hints.detachHeading = hints.detachHeading ||
        tokens.includes("heading-before") ||
        tokens.includes("detach-heading") ||
        tokens.includes("diagram-only");
    });
    return hints;
  }

  function portraitForcedDiagramMaxHeight(stats, print) {
    // A page-hinted portrait diagram is expected to use most of its own page.
    // Keep unhinted diagrams on the more conservative readability caps so
    // ordinary inline charts do not suddenly dominate surrounding text.
    if (!stats.forcePage || stats.forceLandscape) return 0;
    const headingReserve = stats.detachHeading ? 120 : 150;
    return Math.max(420, print.portraitContentHeight - headingReserve);
  }

  function landscapeDiagramMaxHeight(size, print) {
    // Do not squeeze a diagram just because it moved to landscape. Only cap it
    // when the rendered SVG would overflow the landscape content height.
    const naturalLandscapeHeight = Math.min(size.width, print.landscapeContentWidth) / (size.width / size.height);
    const availableHeight = Math.max(340, print.landscapeContentHeight - 12);
    if (naturalLandscapeHeight <= availableHeight) return 0;
    return availableHeight;
  }

  function applyDiagramPageClass(element, useLandscape, useForcedPage, isOversized, detachHeadingHint) {
    // Landscape page changes must apply to only the diagram so section headings
    // can stay in portrait flow. Portrait forced pages can keep a short heading
    // and lead-in with the diagram because they do not need a named page size.
    const group = element.closest(".markpdf-heading-diagram-group");
    const detachHeading = useLandscape || (useForcedPage && detachHeadingHint);
    const pageElement = detachHeading ? element : diagramPageElement(element);
    const skipForcedBreakBefore = useForcedPage && !useLandscape && startsAfterLandscapePage(pageElement);
    if (group) {
      group.classList.toggle("markpdf-heading-before-diagram-page", detachHeading);
      group.classList.toggle("markpdf-heading-before-landscape", useLandscape);
      if (detachHeading) {
        group.classList.remove(
          "markpdf-forced-page",
          "markpdf-forced-page-after",
          "markpdf-landscape-page",
          "markpdf-landscape-page-fragmentable"
        );
      }
    }

    pageElement.classList.toggle("markpdf-forced-page", useForcedPage && !skipForcedBreakBefore);
    pageElement.classList.toggle("markpdf-forced-page-after", useForcedPage && !useLandscape);
    pageElement.classList.toggle("markpdf-landscape-page", useLandscape);
    pageElement.classList.toggle("markpdf-landscape-page-fragmentable", useLandscape && isOversized);

    if (pageElement !== element) {
      element.classList.remove(
        "markpdf-forced-page",
        "markpdf-forced-page-after",
        "markpdf-landscape-page",
        "markpdf-landscape-page-fragmentable"
      );
    }
  }

  function startsAfterLandscapePage(element) {
    // Switching from a named landscape page back to the default portrait page
    // already forces Chromium to start a new page. Adding break-before on the
    // next forced portrait section creates an unnecessary blank page.
    const previous = previousMeaningfulElement(element);
    return Boolean(previous && containsLandscapePage(previous));
  }

  function containsLandscapePage(element) {
    return element.classList.contains("markpdf-landscape-page") ||
      Boolean(element.querySelector(".markpdf-landscape-page"));
  }

  function diagramPageElement(element) {
    // Ungrouped diagrams can still request landscape; grouped diagrams need the
    // wrapper to prevent Chrome from printing the heading on the previous page.
    return element.closest(".markpdf-heading-diagram-group") || element;
  }

  function readSvgSize(svg) {
    const viewBox = svg.getAttribute("viewBox");
    let width = numericLength(svg.getAttribute("width"));
    let height = numericLength(svg.getAttribute("height"));

    if ((!width || !height) && viewBox) {
      const parts = viewBox.split(/\s+/).map(Number);
      width = parts[2] || width;
      height = parts[3] || height;
    }

    return { width, height };
  }

  function trimMermaidSvgViewport(svg) {
    // Mermaid sometimes leaves large empty areas in the SVG viewBox after graph
    // layout. If we size against that untrimmed viewBox, the actual diagram
    // looks unnecessarily tiny in the PDF. getBBox measures drawn content. Use
    // the whole SVG rather than the first child group because Mermaid can nest
    // labels and markers outside the first graph group.
    const originalViewBox = readSvgSize(svg);
    const content = svg;
    let box;
    try {
      box = content.getBBox();
    } catch (_error) {
      return;
    }
    if (!box || box.width <= 0 || box.height <= 0) return;

    const padding = 44;
    const x = Math.floor(box.x - padding);
    const y = Math.floor(box.y - padding);
    const width = Math.ceil(box.width + padding * 2);
    const height = Math.ceil(box.height + padding * 2);
    if (width <= 0 || height <= 0) return;
    if (originalViewBox.width && originalViewBox.height) {
      // A dramatic shrink usually means the browser omitted a label,
      // foreignObject, or marker from getBBox. Skip those cases; it is better to
      // leave some whitespace than to crop labels or enlarge partial content.
      const widthRatio = width / originalViewBox.width;
      const heightRatio = height / originalViewBox.height;
      if (widthRatio < 0.72 || heightRatio < 0.72) return;
      if (width >= originalViewBox.width && height >= originalViewBox.height) return;
    }

    svg.setAttribute("viewBox", `${x} ${y} ${width} ${height}`);
    svg.setAttribute("width", String(width));
    svg.setAttribute("height", String(height));
  }

  function diagramLayout(width, height, containerWidth, maxDiagramHeight) {
    const ratio = width / height;
    const maxReadableHeight = diagramMaxReadableHeight(ratio, containerWidth, maxDiagramHeight);
    const targetWidth = Math.min(width, containerWidth);
    const intrinsicMode = width > containerWidth ? "fit-width" : "intrinsic";

    // Prefer natural size first, then scale down to a page-fit height if that
    // still leaves enough width for labels to remain legible.
    if (targetWidth / ratio <= maxReadableHeight) {
      return { mode: intrinsicMode, ratio, width: targetWidth, maxHeight: 0, maxReadableHeight };
    }

    const heightLimitedWidth = Math.min(targetWidth, maxReadableHeight * ratio);
    if (heightLimitedWidth >= diagramMinReadableWidth(ratio, containerWidth)) {
      return { mode: "fit-page", ratio, width: heightLimitedWidth, maxHeight: maxReadableHeight, maxReadableHeight };
    }

    return { mode: "oversized", ratio, width: targetWidth, maxHeight: 0, maxReadableHeight };
  }

  function diagramMaxReadableHeight(ratio, containerWidth, maxDiagramHeight) {
    // Tall diagrams need a lower cap than wide diagrams; otherwise Chrome tries
    // to fragment an avoid-break SVG and can orphan the preceding heading.
    let height;
    if (ratio < 0.35) height = Math.max(440, Math.min(500, containerWidth * 0.74));
    else if (ratio < 0.65) height = Math.max(430, Math.min(500, containerWidth * 0.74));
    else if (ratio < 1.15) height = Math.max(520, Math.min(640, containerWidth * 0.9));
    else height = Math.max(520, Math.min(760, containerWidth * 1.08));

    return maxDiagramHeight > 0 ? maxDiagramHeight : height;
  }

  function diagramMinReadableWidth(ratio, containerWidth) {
    // Allow very tall diagrams to become narrow, but stop before labels collapse
    // into unreadable columns. Wider diagrams require a larger readable floor.
    if (ratio < 0.35) return Math.min(92, containerWidth * 0.18);
    if (ratio < 0.65) return Math.min(220, containerWidth * 0.36);
    if (ratio < 1.15) return Math.min(360, containerWidth * 0.54);
    return Math.min(460, containerWidth * 0.58);
  }

  function numericLength(value) {
    if (!value) return 0;

    const normalized = String(value).trim();
    if (!normalized || normalized.endsWith("%")) return 0;

    const parsed = parseFloat(normalized);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  // Populate TOC page numbers by measuring where each target heading lands
  // relative to the page height. Runs after all content rendering so diagrams
  // and tables have their final layout dimensions.
  function populateTOCPageNumbers() {
    const spans = document.querySelectorAll(".markpdf-toc .toc-page[data-target]");
    if (!spans.length) return;

    const print = (config.print || {});
    // Page height in CSS pixels. Fall back to A4 with 24mm top+bottom margins.
    const pageHeight = print.portraitContentHeight || ((11.69 - 2 * 24 / 25.4) * 96);

    // Count pages consumed by cover and TOC itself (they force page breaks).
    let prefixPages = 0;
    const cover = document.querySelector(".markpdf-cover");
    if (cover) prefixPages += 1;
    const toc = document.querySelector(".markpdf-toc");
    if (toc) {
      // TOC may span multiple pages; estimate from its rendered height.
      const tocRect = toc.getBoundingClientRect();
      prefixPages += Math.max(1, Math.ceil(tocRect.height / pageHeight));
    }

    // The content starts after prefix pages. Measure each heading's offset
    // from the content top to determine which page it falls on.
    const content = document.querySelector(".markpdf-content");
    if (!content) return;
    const contentTop = content.getBoundingClientRect().top;

    spans.forEach((span) => {
      const targetID = span.getAttribute("data-target");
      const heading = document.getElementById(targetID);
      if (!heading) return;

      const headingTop = heading.getBoundingClientRect().top;
      const offset = headingTop - contentTop;
      const page = prefixPages + 1 + Math.floor(Math.max(0, offset) / pageHeight);
      span.textContent = String(page);
    });
  }

  function renderError(element, renderer, error, source) {
    element.classList.add("markpdf-render-error");
    element.textContent = `${renderer} render failed: ${errorMessage(error)}\n\n${source}`;
  }

  function errorMessage(error) {
    return error && error.message ? error.message : String(error);
  }
})();
