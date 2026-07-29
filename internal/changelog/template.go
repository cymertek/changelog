package changelog

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// Sanitizer provides format-specific escaping for template data.
type Sanitizer interface {
	Sanitize(input string) string
}

// HTMLSanitizer escapes HTML special characters using Go's html/template package.
// Automatically handles <, >, &, ", ' to prevent XSS and ensure valid HTML.
type HTMLSanitizer struct{}

// Sanitize escapes HTML special characters in the input string.
func (s HTMLSanitizer) Sanitize(input string) string {
	// Use html/template's auto-escaping by rendering through a template.
	tmpl := template.Must(template.New("").Parse("{{ . }}"))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		return input // fallback: return as-is if escaping fails
	}
	return buf.String()
}

// LaTeXSanitizer escapes LaTeX special characters.
// Handles: \ % $ & # _ { } ~ ^
type LaTeXSanitizer struct{}

// Sanitize escapes LaTeX special characters using a two-pass approach to avoid cascading replacements.
func (s LaTeXSanitizer) Sanitize(input string) string {
	// Two-pass approach to avoid cascading replacements.
	// Replacements for \, ~, ^ produce strings containing { } characters,
	// which would then be escaped by subsequent { } replacement rules.
	// We use placeholders for those "dangerous" replacements first,
	// apply the safe replacements (%, $, &, #, _, {, }), then swap back.

	const (
		placeholderBackslash = "\x00BACKSLASH\x00"
		placeholderTilde     = "\x00TILDE\x00"
		placeholderCirc      = "\x00CIRCUMFLEX\x00"
	)

	result := input

	// Step 1: Replace characters whose replacement strings contain { or }
	// with placeholders that don't.
	result = strings.ReplaceAll(result, "\\", placeholderBackslash)
	result = strings.ReplaceAll(result, "~", placeholderTilde)
	result = strings.ReplaceAll(result, "^", placeholderCirc)

	// Step 2: Replace all other special characters (safe replacements).
	safeReplacements := []struct{ from, to string }{
		{"%", "\\%"},
		{"$", "\\$"},
		{"&", "\\&"},
		{"#", "\\#"},
		{"_", "\\_"},
		{"{", "\\{"},
		{"}", "\\}"},
	}
	for _, r := range safeReplacements {
		result = strings.ReplaceAll(result, r.from, r.to)
	}

	// Step 3: Restore placeholders to their actual LaTeX escape sequences.
	result = strings.ReplaceAll(result, placeholderBackslash, "\\textbackslash{}")
	result = strings.ReplaceAll(result, placeholderTilde, "\\textasciitilde{}")
	result = strings.ReplaceAll(result, placeholderCirc, "\\textasciicircum{}")

	return result
}

// PlainSanitizer passes text through unchanged (for txt/rst formats).
type PlainSanitizer struct{}

// Sanitize returns the input unchanged — plain text and markdown don't need escaping.
func (s PlainSanitizer) Sanitize(input string) string {
	return input
}

// GetSanitizer returns the appropriate sanitizer for the given format.
func GetSanitizer(format string) Sanitizer {
	switch strings.ToLower(format) {
	case "html":
		return HTMLSanitizer{}
	case "latex", "tex":
		return LaTeXSanitizer{}
	default: // txt, rst, markdown
		return PlainSanitizer{}
	}
}

// TemplateData holds all data available to Go templates during changelog rendering.
type TemplateData struct {
	Title        string           // project title from config
	Version      string           // release version (e.g., "v1.0.0")
	Date         string           // release date in YYYY-MM-DD format
	Codename     string           // optional release codename
	Entries      []ChangelogEntry // rendered changelog entries grouped by section
	RawFragments []*Fragment      // unprocessed fragment data for custom rendering
	Config       *Config          // full config for template access

	// Sanitizer is set based on output format to ensure proper escaping.
	Sanitizer Sanitizer `yaml:"-"`
}

// ChangelogEntry represents a single entry in the changelog output.
type ChangelogEntry struct {
	Section      string   // category/section name (e.g., "bugfixes", "enhancements")
	DisplayTitle string   // human-readable section title
	Items        []string // list of change items in this section
}

// LoadTemplate reads and parses a Go template file with format-appropriate encoding.
func LoadTemplate(path string, format string) (*template.Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}

	// Create sanitizer for this format.
	sanitizer := GetSanitizer(format)

	// Build function map with format-specific encoding helpers.
	funcMap := template.FuncMap{
		"encodeHTML": func(input string) string {
			if strings.ToLower(format) == "html" {
				return sanitizer.Sanitize(input)
			}
			return input // Only encode for HTML format
		},
		"encodeTeX": func(input string) string {
			if strings.Contains(strings.ToLower(format), "tex") || strings.Contains(strings.ToLower(format), "latex") {
				return sanitizer.Sanitize(input)
			}
			return input // Only encode for TeX/LaTeX format
		},
		"encodePlainText": func(input string) string {
			// Plain text formats don't need encoding, but this makes intent clear
			return input
		},
	}

	tmpl, err := template.New(filepath.Base(path)).Funcs(funcMap).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}

	return tmpl, nil
}

// RenderTemplate executes a Go template with the given data and returns the output.
func RenderTemplate(tmpl *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// RenderMarkdownTemplate renders the built-in markdown changelog using a Go template.
// This provides consistency between custom templates and the default markdown output.
func RenderMarkdownTemplate(data TemplateData) (string, error) {
	markdownTmpl := `{{- if .Codename -}}
# {{ .Title }} {{ .Version }} ({{ .Codename }})
{{- else -}}
# {{ .Title }} {{ .Version }}
{{- end }}

Released: {{ .Date }}

{{ range .Entries }}## {{ .DisplayTitle }}

{{ range .Items }}* {{ . }}
{{ end }}
{{ end -}}`

	tmpl, err := template.New("markdown").Parse(markdownTmpl)
	if err != nil {
		return "", fmt.Errorf("parse markdown template: %w", err)
	}

	return RenderTemplate(tmpl, data)
}

// RenderToFiles renders changelog in all configured formats and writes to disk.
func RenderToFiles(baseDir string, fragments []*Fragment, version, date, codename string, cfg *Config) (map[string]string, error) {
	// Load config if not provided.
	if cfg == nil {
		var err error
		cfgPath := filepath.Join(baseDir, "config.yaml")
		cfg, err = Load(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("load config %s: %w", cfgPath, err)
		}
	}

	outputs := make(map[string]string) // format name -> output path

	for _, tmplCfg := range cfg.Templates {
		var rendered string
		var err error

		switch strings.ToLower(tmplCfg.Format) {
		case "markdown":
			rendered, err = RenderMarkdownTemplate(TemplateData{
				Title:        cfg.Title,
				Version:      version,
				Date:         date,
				Codename:     codename,
				RawFragments: fragments,
				Config:       cfg,
				Sanitizer:    PlainSanitizer{}, // markdown doesn't need escaping
			})
		case "html", "txt", "latex", "rst":
			// Load and render custom template.
			tmplPath := tmplCfg.Path

			// Resolve template path relative to baseDir (project root).
			if !filepath.IsAbs(tmplPath) {
				// Normalize: if baseDir ends with "changelogs"/".changelogs"/".changelog"
				// and tmplPath starts with the same prefix, strip it — otherwise we'd
				// double-prefix (e.g. "changelogs/changelogs/template.html").
				baseSuffix := ""
				if baseHasChangelogDir(baseDir) {
					baseSuffix = baseChangelogDirName(baseDir)
				}
				if baseSuffix != "" && strings.HasPrefix(tmplPath, baseSuffix+string(filepath.Separator)) {
					tmplPath = strings.TrimPrefix(tmplPath, baseSuffix+string(filepath.Separator))
				}

				// Try relative to baseDir first, then relative to baseDir/changelogs/,
				// and finally relative to the parent of baseDir (project root) when
				// baseDir itself is a changelogs-style directory.
				var tried []string
				candidates := []string{
					filepath.Join(baseDir, tmplPath),
					filepath.Join(baseDir, "changelogs", tmplPath),
				}
				if baseHasChangelogDir(baseDir) {
					candidates = append(candidates, filepath.Join(filepath.Dir(baseDir), tmplPath))
				}
				for _, c := range candidates {
					if _, statErr := os.Stat(c); statErr == nil {
						tmplPath = c
						tried = append(tried, c)
						break
					}
				}
				if len(tried) == 0 {
					tried = candidates // report all tried paths in error message
				}
				if tmplPath == "" {
					return nil, fmt.Errorf("template not found: %s (tried %s and %s)", tmplCfg.Name,
						tried[0], tried[len(tried)-1])
				}
			}

			tmpl, err := LoadTemplate(tmplPath, tmplCfg.Format)
			if err != nil {
				return nil, fmt.Errorf("load template %s: %w", tmplCfg.Name, err)
			}

			// Prepare template data with appropriate sanitizer.
			sanitizer := GetSanitizer(tmplCfg.Format)
			tmplData := TemplateData{
				Title:        cfg.Title,
				Version:      version,
				Date:         date,
				Codename:     codename,
				RawFragments: fragments,
				Config:       cfg,
				Sanitizer:    sanitizer,
			}

			// Build changelog entries from fragments.
			entries := buildChangelogEntries(fragments, cfg)
			tmplData.Entries = entries

			rendered, err = RenderTemplate(tmpl, tmplData)
			if err != nil {
				return nil, fmt.Errorf("render template %s: %w", tmplCfg.Name, err)
			}
		default:
			return nil, fmt.Errorf("unsupported template format: %s", tmplCfg.Format)
		}

		if err != nil {
			return nil, err
		}

		// Determine output path.
		outputFile := tmplCfg.OutputFile
		if outputFile == "" {
			ext := getTemplateExtension(tmplCfg.Format)
			outputFile = fmt.Sprintf("CHANGELOG.%s", ext)
		}

		// Try writing to parent of baseDir (project root) first, then baseDir,
		// then baseDir/changelogs/. This ensures outputs land in the project root
		// when baseDir is a changelogs-style directory.
		outputPath := ""
		var tried []string
		candidates := []string{filepath.Join(baseDir, outputFile)}
		if baseHasChangelogDir(baseDir) {
			candidates = append([]string{filepath.Join(filepath.Dir(baseDir), outputFile)}, candidates...)
		}
		candidates = append(candidates, filepath.Join(baseDir, "changelogs", outputFile))

		for _, c := range candidates {
			if _, statErr := os.Stat(filepath.Dir(c)); statErr == nil {
				outputPath = c
				tried = []string{c}
				break
			}
			tried = append(tried, c)
		}
		if outputPath == "" && len(tried) > 0 {
			outputPath = tried[len(tried)-1] // fall back to last candidate
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, fmt.Errorf("create output dir %s: %w", filepath.Dir(outputPath), err)
		}

		if err := os.WriteFile(outputPath, []byte(rendered), 0o644); err != nil {
			return nil, fmt.Errorf("write output %s: %w", outputPath, err)
		}

		outputs[tmplCfg.Name] = outputPath
		fmt.Printf("Generated: %s\n", outputPath)
	}

	return outputs, nil
}

// buildChangelogEntries groups fragments by section for template rendering.
func buildChangelogEntries(fragments []*Fragment, cfg *Config) []ChangelogEntry {
	sections := make(map[string]*ChangelogEntry)

	for _, frag := range fragments {
		for sectionName, items := range frag.Sections {
			if _, ok := sections[sectionName]; !ok {
				displayTitle := sectionName
				if cfg.Sections != nil {
					if title, exists := cfg.Sections[sectionName]; exists {
						displayTitle = title
					}
				}

				sections[sectionName] = &ChangelogEntry{
					Section:      sectionName,
					DisplayTitle: displayTitle,
				}
			}
			sections[sectionName].Items = append(sections[sectionName].Items, items...)
		}
	}

	var entries []ChangelogEntry
	for _, entry := range sections {
		entries = append(entries, *entry)
	}

	return entries
}

// getTemplateExtension returns the file extension for a template format.
func getTemplateExtension(format string) string {
	switch strings.ToLower(format) {
	case "html":
		return "html"
	case "txt", "text":
		return "txt"
	case "latex", "tex":
		return "tex"
	case "rst":
		return "rst"
	default:
		return strings.ToLower(format)
	}
}

// baseHasChangelogDir reports whether baseDir looks like a changelogs directory
// (e.g. "changelogs", ".changelogs", ".changelog"). Used to avoid double-prefixing
// template paths that already include the changelog dir name.
func baseHasChangelogDir(baseDir string) bool {
	base := filepath.Base(baseDir)
	return base == "changelogs" || base == ".changelogs" || base == ".changelog"
}

// baseChangelogDirName returns the changelog directory component of baseDir, or "" if
// baseDir is not a changelogs-style path. Returns just the basename (e.g. "changelogs").
func baseChangelogDirName(baseDir string) string {
	base := filepath.Base(baseDir)
	switch base {
	case "changelogs", ".changelogs", ".changelog":
		return base
	default:
		return ""
	}
}
