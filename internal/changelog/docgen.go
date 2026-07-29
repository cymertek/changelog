package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// GenerateConfigDocs reflects over the Config struct and writes per-field markdown files.
// Usage: go run cmd/gchangelog/internal/changelog/docgen.go <output-dir>
func GenerateConfigDocs(outDir string) error {
	cfg := &Config{}
	cfg.Defaults()
	t := reflect.TypeOf(cfg).Elem() // reflect on the struct type, not pointer

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		yamlTag := f.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue // skip fields with no yaml tag or yaml:"-" (hidden/internal)
		}
		yamlName := strings.Split(yamlTag, ",")[0]

		goType := typeString(f.Type)
		desc := f.Tag.Get("doc")
		if desc == "" {
			desc = fieldDescription(yamlName, goType) // fallback description
		}

		md := fmt.Sprintf("---\ntitle: %s\nfield: %s\ngo_type: %s\ncommand: config\nweight: 50\n---\n\n", yamlName, yamlName, goType)
		md += fmt.Sprintf("## `%s`\n\n", yamlName)
		md += desc + "\n\n"

		if err := os.MkdirAll(filepath.Join(outDir, "config-fields"), 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}

		filename := filepath.Join(outDir, "config-fields", yamlName+".md")
		if err := os.WriteFile(filename, []byte(md), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}

	fmt.Fprintf(os.Stderr, "Generated config field docs in %s/config-fields/\n", outDir)
	return nil
}

func typeString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int64:
		return "int"
	case reflect.Bool:
		return "bool"
	case reflect.Ptr:
		return "*" + typeString(t.Elem())
	case reflect.Slice:
		return "[]" + typeString(t.Elem())
	default:
		return t.String()
	}
}

// fieldDescription returns a brief description for common config fields.
func fieldDescription(name, goType string) string {
	descriptions := map[string]string{
		"title":                        "The project name displayed in the changelog header.",
		"version_pattern":              "Regex pattern to validate version strings (e.g., '^v\\d+\\.\\d+\\.\\d+$').",
		"changelog_filename_template":  "Template for the output CHANGELOG filename.",
		"date_format":                  "Date format string used in release dates. Default: '%Y-%m-%d'.",
		"default_category":             "Default category when none is specified in a fragment. Must be one of: breaking, enhancement, deprecation, removal, note.",
		"families":                     "Map of family names to lists of categories they include (e.g., 'releases': [breaking, enhancement]).",
		"flatmap":                      "Three-state boolean: nil=unset (use default), true=enabled, false=disabled. Controls flat category grouping.",
		"mention_ancestor":             "When true, mentions ancestor versions in release notes.",
		"sections":                     "Map of section names to their descriptions for customizing changelog sections.",
		"prelude_section_title":        "Title for the prelude/release summary section at the top of each version block.",
		"prelude_description_template": "Template for rendering the prelude section content. Uses Go template syntax.",
		"trivial_section_name":         "Name for the trivial/minor changes section (e.g., 'Trivial Changes').",
		"vcs":                          "Version control system identifier (git, hg). Used to detect repository root.",
		"notes_dir":                    "Path to changelog fragments directory. Must contain .md files with YAML frontmatter.",
	}

	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return fmt.Sprintf("Configuration field '%s' of type %s.", name, goType)
}
