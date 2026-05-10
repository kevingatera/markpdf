# Security Policy

## Supported Versions

markpdf is pre-1.0. Security fixes target the default branch until versioned releases are published.

## Reporting a Vulnerability

If this repository is public on GitHub, report vulnerabilities through GitHub private vulnerability reporting or a private maintainer contact. Avoid opening public issues for vulnerabilities that include exploit details.

## Security Model

markpdf is designed as a local CLI/library for converting trusted or semi-trusted documents. It is not a hardened multi-tenant rendering sandbox.

User-provided Markdown and HTML are sanitized before they reach Chromium. The browser then runs only embedded runtime assets for syntax highlighting, KaTeX, Mermaid, and print layout. This keeps normal Markdown from executing arbitrary script.

Some sanitized HTML can still reference external resources such as links or images depending on the sanitizer policy. If you expose markpdf through a server, render untrusted input in an isolated worker with restricted network access, temporary storage, CPU/memory limits, and short-lived credentials.
