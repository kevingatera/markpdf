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
      const heading = previousMeaningfulElement(diagram);
      if (!heading || !/^H[1-6]$/.test(heading.tagName) || heading.parentElement.classList.contains("markpdf-heading-diagram-group")) {
        return;
      }

      // Chromium respects break-inside more consistently on an explicit block
      // wrapper than on a heading followed by a rendered SVG.
      const group = document.createElement("div");
      group.className = "markpdf-heading-diagram-group";
      heading.parentNode.insertBefore(group, heading);
      group.appendChild(heading);
      group.appendChild(diagram);
    });
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
    // covers request/status lines and headers for API documentation examples.
    return {
      name: "HTTP/API",
      aliases: ["api", "endpoint", "http", "http-request", "http-response", "request", "response", "rest"],
      contains: [
        {
          className: "meta",
          begin: /^(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+\S+\s+HTTP\/\d(?:\.\d)?/m
        },
        {
          className: "meta",
          begin: /^HTTP\/\d(?:\.\d)?\s+\d{3}.*$/m
        },
        {
          className: "attr",
          begin: /^[A-Za-z0-9-]+(?=:)/m
        },
        {
          className: "string",
          begin: /:\s*/,
          end: /$/,
          excludeBegin: true
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

    if (/^(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+\S+/i.test(trimmed)) return "markpdf-http";
    if (/^HTTP\/\d(?:\.\d)?\s+\d{3}/i.test(trimmed)) return "markpdf-http";
    if (/^[\[{]/.test(trimmed)) return "json";
    if (/^[A-Za-z0-9_.-]+:\s+/m.test(trimmed)) return "yaml";

    return "markpdf-http";
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

    window.mermaid.initialize({
      startOnLoad: false,
      theme: config.mermaidTheme || "default",
      flowchart: {
        // Mermaid's default max-width behavior fights our own print scaling.
        // Fixed spacing gives tall state diagrams a chance to fit one page.
        useMaxWidth: false,
        htmlLabels: true,
        nodeSpacing: 32,
        rankSpacing: 36,
        padding: 8
      }
    });

    const diagrams = Array.from(document.querySelectorAll(".mermaid"));
    for (const [index, element] of diagrams.entries()) {
      await renderMermaidDiagram(element, index);
    }
  }

  async function renderMermaidDiagram(element, index) {
    const source = normalizeMermaidSource(element.textContent);

    try {
      const result = await window.mermaid.render(`markpdf-mermaid-${index}`, source, element);
      element.innerHTML = result.svg;
      if (result.bindFunctions) result.bindFunctions(element);
      fitDiagram(element);
    } catch (error) {
      renderError(element, "Mermaid", error, source);
    }
  }

  function normalizeMermaidSource(source) {
    // Mermaid diagrams in Markdown commonly use "\n" inside quoted labels.
    // Recent Mermaid/Chromium combinations can print those literally; <br/>
    // keeps the author's intended line breaks and reduces oversized label boxes.
    return (source || "").replace(/\\n/g, "<br/>");
  }

  // Mermaid emits SVGs with wildly different aspect ratios; this keeps them printable.
  function fitDiagram(element) {
    const svg = element.querySelector("svg");
    if (!svg) return;

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
    const layout = diagramLayout(size.width, size.height, containerWidth);

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

    const isSmallDiagram = size.width < containerWidth * 0.72 && size.height < layout.maxReadableHeight * 0.72;
    element.classList.toggle("markpdf-diagram-small", isSmallDiagram);
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

  function diagramLayout(width, height, containerWidth) {
    const ratio = width / height;
    const maxReadableHeight = diagramMaxReadableHeight(ratio, containerWidth);
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

  function diagramMaxReadableHeight(ratio, containerWidth) {
    // Tall diagrams need a lower cap than wide diagrams; otherwise Chrome tries
    // to fragment an avoid-break SVG and can orphan the preceding heading.
    if (ratio < 0.35) return Math.max(440, Math.min(500, containerWidth * 0.74));
    if (ratio < 0.65) return Math.max(430, Math.min(500, containerWidth * 0.74));
    if (ratio < 1.15) return Math.max(520, Math.min(640, containerWidth * 0.9));
    return Math.max(520, Math.min(760, containerWidth * 1.08));
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

  function renderError(element, renderer, error, source) {
    element.classList.add("markpdf-render-error");
    element.textContent = `${renderer} render failed: ${errorMessage(error)}\n\n${source}`;
  }

  function errorMessage(error) {
    return error && error.message ? error.message : String(error);
  }
})();
