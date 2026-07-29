# Template Customization Guide

changelog supports multiple output formats beyond the default Markdown: **HTML**, **LaTeX/Tex**, and **plain text (txt/rst)**. Each format can use custom Go templates for branded or specialized output. This guide covers how to configure and write custom templates.

## Generating Multiple Formats Simultaneously

**A single `changelog release` command generates every configured template at once.** There is no need to invoke the release command separately for each format — all templates render together in one pass, each using its own encoding rules:

| Format | Encoding applied automatically |
|--------|-------------------------------|
| Markdown | No escaping needed (plain text) |
| HTML | `encodeHTML` converts `<`, `>`, `&`, `"`, `'` to safe entities |
| LaTeX/Tex | `encodeTeX` escapes `\ $ % & # _ { } ~ ^` for valid compilation |
| Plain text / RST | No escaping needed (passthrough) |

### Example: MD + HTML + LaTeX in One Release

Configure all three templates in `config.yaml`:

```yaml
templates:
  - name: markdown-default
    format: markdown
    output_file: CHANGELOG.md
  - name: html-report
    format: html
    path: changelogs/html-template.html
    output_file: changelog.html
  - name: latex-pdf
    format: latex
    path: changelogs/latex-template.tex
    output_file: changelog.tex
```

Then run **one command**:

```bash
changelog release --version v1.0.0
```

This produces three files simultaneously:

| File | Audience | Use case |
|------|----------|----------|
| `CHANGELOG.md` | Developers, GitHub releases | Readable in terminal and on GitHub |
| `changelog.html` | Web browsers, documentation sites | Styled report with CSS branding |
| `changelog.tex` | PDF publishers (pdflatex) | Typeset document for print or archiving |

### Example: Four Formats at Once (MD + HTML + LaTeX + TXT)

```yaml
templates:
  - name: markdown-default
    format: markdown
    output_file: CHANGELOG.md
  - name: html-report
    format: html
    path: changelogs/html-template.html
    output_file: changelog.html
  - name: latex-pdf
    format: latex
    path: changelogs/latex-template.tex
    output_file: changelog.tex
  - name: plain-text
    format: txt
    path: changelogs/txt-template.txt
    output_file: changelog.txt
```

One `release` call generates all four files. Each template is independent — change one without affecting the others.

### Working Example

The directory [`example_tests/multi_format_release/`](../example_tests/multi_format_release/) demonstrates this exact multi-format setup with a complete config.yaml, custom HTML and plain text templates, and fragment content that exercises special characters across all formats simultaneously.

## Supported Output Formats

| Format      | Template Required? | Use Case                                      |
|-------------|--------------------|-----------------------------------------------|
| `markdown`  | No                 | Default plain Markdown output (built-in)        |
| `html`      | Yes                | Branded HTML reports with CSS styling           |
| `latex`, `tex` | Yes             | LaTeX documents for PDF generation              |
| `txt`, `text` | Optional         | Plain text or RST output                        |

Configure formats in `changelogs/config.yaml`:

```yaml
templates:
  - name: markdown
    format: markdown
    output_file: CHANGELOG.md
  - name: html-report
    format: html
    path: changelogs/template.html
    output_file: changelog.html
  - name: latex-pdf
    format: latex
    path: changelogs/template.tex
    output_file: changelog.tex
```

## Built-in Markdown Template

The Markdown format uses a built-in template (no custom file needed):

```markdown
# {{ .Title }} {{ .Version }}

Released: {{ .Date }}

{{ range .Entries }}
## {{ .DisplayTitle }}

{{ range .Items }}* {{ . }}
{{ end }}
{{ end -}}
```

You can customize section display names via `config.yaml`:

```yaml
sections:
  bugfixes: "Bug Fixes"
  major_changes: "Major Changes"
```

## Custom HTML Template

### Example Template (`changelogs/template.html`)

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{ .Title }} Changelog</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 900px; margin: 0 auto; padding: 2rem; }
        h1 { color: #2c3e50; border-bottom: 2px solid #3498db; }
        h2 { color: #2980b9; margin-top: 2rem; }
        .entry { background: #f8f9fa; padding: 1rem; margin: 1rem 0; border-radius: 4px; }
        .section-title { font-weight: bold; color: #e74c3c; }
    </style>
</head>
<body>
    <h1>{{ .Title }} v{{ .Version }}</h1>
    <p>Released: {{ .Date }}</p>

    {{ range .Entries }}
    <div class="entry">
        <h2 class="section-title">{{ .DisplayTitle }}</h2>
        <ul>
            {{ range .Items }}
            <li>{{ encodeHTML . }}</li>
            {{ end }}
        </ul>
    </div>
    {{ end }}
</body>
</html>
```

### Template Variables

| Variable      | Type           | Description                              | Example                          |
|---------------|----------------|------------------------------------------|----------------------------------|
| `.Title`      | string         | Project title from config.yaml           | `"My Project"`                   |
| `.Version`    | string         | Release version (e.g., `v1.0.0`)         | `"v1.2.3"`                       |
| `.Date`       | string         | Release date in YYYY-MM-DD format        | `"2026-07-27"`                   |
| `.Codename`   | string         | Optional release codename                | `"Apollo"` (if set)              |
| `.Entries`    | []ChangelogEntry | Grouped changelog entries by section   | See below                        |
| `.Config`     | *Config        | Full config object for template access   | `{{ .Config.Title }}`            |

### ChangelogEntry Structure

```go
type ChangelogEntry struct {
    Section      string   // Internal section key (e.g., "bugfixes")
    DisplayTitle string   // Human-readable title (e.g., "Bug Fixes")
    Items        []string // List of change items in this section
}
```

### Template Functions

| Function             | Description                                                  | Example                                      |
|----------------------|--------------------------------------------------------------|----------------------------------------------|
| `encodeHTML`         | Escapes HTML special characters (`<`, `>`, `&`, `"`, `'`)   | `{{ encodeHTML . }}` → `&lt;script&gt;`      |
| `encodeTeX`          | Escapes LaTeX special characters (`\ $ % # _ { } ~ ^`)       | `{{ encodeTeX . }}` → `\$variable`           |
| `encodePlainText`    | Pass-through (no encoding needed for txt/rst)                | `{{ encodePlainText . }}` → raw text         |

## Custom LaTeX Template

### Example Template (`changelogs/template.tex`)

```latex
\documentclass{article}
\usepackage[utf8]{inputenc}
\usepackage{hyperref}

\title{\textbf{ {{ .Title }} Changelog }}
\author{{ .Version } \\ Released: \today}
\date{}

\begin{document}
\maketitle

\section*{Changelog for {{ .Version }}}

{{ range .Entries }}
\subsection*{{ encodeTeX .DisplayTitle }}

\begin{itemize}
    {{ range .Items }}
    \item {{ encodeTeX . }}
    {{ end }}
\end{itemize}

{{ end }}
\end{document}
```

### LaTeX Sanitization

The `LaTeXSanitizer` escapes these characters: `\ $ % & # _ { } ~ ^`. Use `encodeTeX` in templates to ensure proper escaping.

**Important**: Do NOT pre-unescape HTML entities before sending to `encodeTeX` — users may want `&amp;` literally in LaTeX output.

## Plain Text / RST Template

### Example Template (`changelogs/template.txt`)

```
{{ .Title }} Changelog
{{ repeat "=" (len .Title) }}

Version {{ .Version }} — Released: {{ .Date }}
{{ repeat "-" (len .Version) }}

{{ range .Entries }}
{{ .DisplayTitle }}
{{ repeat "=" (len .DisplayTitle) }}

{{ range .Items }}  - {{ . }}

{{ end }}
{{ end }}
```

Plain text templates don't require encoding functions. Use `encodePlainText` for clarity if desired, but it's a no-op.

## Template Resolution Order

When you specify a template path in `config.yaml`, changelog resolves it relative to:

1. **Project root** (`baseDir/template.html`)
2. **Changelogs directory** (`baseDir/changelogs/template.html`)

If the file isn't found in either location, release fails with an error:

```bash
$ changelog release --version v1.0.0
Error: template not found: changelogs/template.html (tried /project/changelogs/template.html and /project/changelogs/changelogs/template.html)
```

## Example Templates Directory

The `example_templates/` directory contains ready-to-use templates:

```
example_templates/
├── README.md              # How to use example templates
├── template-html.html     # Branded HTML changelog template
└── template-latex.tex     # LaTeX PDF report template
```

Copy and customize these for your project:

```bash
cp example_templates/template-html.html changelogs/my-template.html
# Edit changelogs/my-template.html to match your branding
```

## Sanitization Best Practices

### HTML Output

Use `encodeHTML` for all user-provided content (fragment titles, descriptions):

```html
<li>{{ encodeHTML . }}</li>  <!-- Escapes < > & " ' -->
```

This prevents XSS and ensures valid HTML:

- `<script>` → `&lt;script&gt;`
- `He said "hello"` → `He said &quot;hello&quot;`

### LaTeX Output

Use `encodeTeX` for all content to escape LaTeX special characters:

```latex
\item {{ encodeTeX . }}  <!-- Escapes \ $ % # _ { } ~ ^ -->
```

This prevents compilation errors and ensures proper rendering.

### Plain Text / Markdown

No encoding needed. Use content as-is:

```markdown
* {{ . }}
```

## Advanced Template Patterns

### Conditional Sections

Show sections only if they have content:

```html
{{ range .Entries }}
{{ if .Items }}
<div class="entry">
    <h2>{{ encodeHTML .DisplayTitle }}</h2>
    <ul>
        {{ range .Items }}
        <li>{{ encodeHTML . }}</li>
        {{ end }}
    </ul>
</div>
{{ end }}
{{ end }}
```

### Custom Headers

Add version-specific headers or codenames:

```html
<h1>{{ .Title }} v{{ .Version }}</h1>
{{ if .Codename }}<p><em>Codename: {{ .Codename }}</em></p>{{ end }}
<p>Released: {{ .Date }}</p>
```

### Accessing Config

Use `.Config` to pull settings from `config.yaml`:

```html
<p>Project: {{ .Config.Title }}</p>
<p>Version Pattern: {{ .Config.VersionPattern }}</p>
```

## Testing Templates

1. Create a test fragment:

```bash
changelog add --category bugfixes -i TEST-001 -T "Test template rendering"
```

2. Run release with `--dry-run`:

```bash
changelog release --version v0.0.0-dryrun --no-archive
```

3. Inspect the generated output file and fix any issues.

4. Clean up:

```bash
rm CHANGELOG.md changelog.html  # Remove test outputs
# Move fragment back to fragments/ if archived
```

## Next Steps

- **[CI/CD Integration](ci-cd-integration.md)** — Automate template rendering in GitHub Actions
- **[End-to-End Workflow](end-to-end-workflow.md)** — Full example with custom HTML template

---

**Need help?** Check the [FAQ](../README.md#faq) or open an issue on [GitHub](https://github.com/cymertek/changelog/issues).
