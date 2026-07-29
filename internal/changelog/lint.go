package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// LintDir reads all fragment files in a directory and returns lint errors.
// Supports both .md (legacy frontmatter) and .yml/.yaml (YAML section format).
func LintDir(dir string, strict bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var errors []string

	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)

		if entry.IsDir() {
			continue // skip subdirectories (fragment files are flat)
		}

		if strings.HasPrefix(name, ".") {
			continue // skip hidden files
		}

		// Skip config files that look like changelog configs
		lower := strings.ToLower(name)
		if lower == "changelog.yaml" || lower == "changelog.yml" ||
			lower == ".changes.yaml" || lower == "changlog.yaml" {
			continue // skip config files
		}

		// Accept .yml/.yaml fragments AND .md fragments.
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yml" && ext != ".yaml" && ext != ".md" {
			continue
		}

		switch ext {
		case ".yml", ".yaml":
			errors = append(errors, lintYAMLFragment(path, strict)...)
		default: // .md — legacy frontmatter format
			fragErrors := lintFragmentMarkdown(path, dir, strict)
			errors = append(errors, fragErrors...)
		}
	}

	return errors, nil
}

// lintYAMLFragment validates a single YAML-format fragment file.
func lintYAMLFragment(path string, strict bool) []string {
	var errs []string
	name := filepath.Base(path)

	data, err := os.ReadFile(path)
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s: read error: %v", name, err))
		return errs
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		errs = append(errs, fmt.Sprintf("%s: file is empty", name))
		return errs
	}

	// Check for frontmatter delimiters.
	if strings.HasPrefix(content, "---") && strings.Count(content, "\n---\n") >= 1 {
		// Has YAML frontmatter — parse it.
		parts := splitFrontmatterBodyCompat(content)
		if parts.frontmatter == "" {
			errs = append(errs, fmt.Sprintf("%s: no valid YAML frontmatter", name))
			return errs
		}

		var metadata map[string]any
		if err := yaml.Unmarshal([]byte(parts.frontmatter), &metadata); err != nil {
			errs = append(errs, fmt.Sprintf("%s: invalid YAML frontmatter: %v", name, err))
			return errs
		}

		// Validate required fields.
		if _, hasCategory := metadata["category"]; !hasCategory {
			errs = append(errs, fmt.Sprintf("%s: missing 'category' field in fragment", name))
		}
		if _, hasIssue := metadata["issue"]; !hasIssue {
			errs = append(errs, fmt.Sprintf("%s: missing 'issue' field in fragment", name))
		}

		if strict {
			if dateVal, ok := metadata["date"].(string); !ok || dateVal == "" {
				errs = append(errs, fmt.Sprintf("%s: date is required in strict mode", name))
			}
		}

		// Check body sections.
		if parts.body != "" && strings.TrimSpace(parts.body) == "" {
			errs = append(errs, fmt.Sprintf("%s: fragment body is empty", name))
		}
	} else {
		// No frontmatter — treat as plain YAML section format.
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			errs = append(errs, fmt.Sprintf("%s: invalid YAML: %v", name, err))
			return errs
		}

		for key, val := range raw {
			if key == "category" || key == "issue" || key == "date" || key == "title" || key == "author" {
				continue // metadata fields
			}

			switch v := val.(type) {
			case string:
				if strings.TrimSpace(v) == "" {
					errs = append(errs, fmt.Sprintf("%s: section %q is empty", name, key))
				}
			case []any:
				for i, item := range v {
					itemStr := fmt.Sprintf("%v", item)
					if strings.TrimSpace(itemStr) == "" || strings.TrimSpace(itemStr) == "-" {
						continue // skip empty items
					}

					// Check for map values (colon-in-string issue).
					if m, ok := item.(map[string]any); ok && len(m) > 0 {
						for mk := range m {
							if strings.Contains(mk, " ") {
								errs = append(errs, fmt.Sprintf("%s:%d: section %q list item %d looks like unquoted colon string — got dict instead of string; quote it with double-quotes", name, i+1, key, i))
							}
						}
					}
				}
			default:
				errs = append(errs, fmt.Sprintf("%s: section %q has unexpected type", name, key))
			}
		}

		// Count actual section keys (non-metadata fields).
		sectionCount := 0
		for key := range raw {
			if key != "category" && key != "issue" && key != "date" && key != "title" && key != "author" {
				sectionCount++
			}
		}
		if len(errs) == 0 && sectionCount == 0 {
			errs = append(errs, fmt.Sprintf("%s: fragment body is empty (only metadata fields found)", name))
		}
	}

	return errs
}

// splitFrontmatterBodyCompat splits YAML frontmatter from body content.
func splitFrontmatterBodyCompat(content string) frontmatterParts {
	const delimiter = "---"
	sp := strings.SplitN(content, delimiter, 3)
	if len(sp) < 3 {
		return frontmatterParts{}
	}
	return frontmatterParts{frontmatter: sp[1], body: sp[2]}
}

// lintFragmentMarkdown validates a single markdown-format fragment file.
func lintFragmentMarkdown(path string, _ string, strict bool) []string {
	var errs []string
	name := filepath.Base(path)

	frag, err := Parse(path)
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		return errs
	}

	if frag.Issue == "" || frag.Issue == "-1" {
		errs = append(errs, fmt.Sprintf("%s: missing or invalid 'issue' field", name))
	}

	if strict && frag.Date == "" {
		errs = append(errs, fmt.Sprintf("%s: date is required in strict mode", name))
	}

	body := strings.TrimSpace(frag.Body)
	if body == "" {
		for _, sectionItems := range frag.Sections {
			for _, item := range sectionItems {
				if strings.TrimSpace(item) != "" && !strings.HasPrefix(strings.TrimSpace(item), "#") {
					body = "has-content"
					break
				}
			}
			if body == "has-content" {
				break
			}
		}
	}

	// Accept fragments with valid metadata (category/issue) but no body — matches abc's behavior.
	// abc's linter only checks sections in content dict; frontmatter-only fragments have empty content → valid.
	if frag.Body != "" || body == "has-content" || frag.Category != "" || frag.Issue != "" {
		return errs // has content or valid metadata, ok
	}

	errs = append(errs, fmt.Sprintf("%s: fragment body is empty", name))

	// Check for unknown section keys.
	for key := range frag.Sections {
		if !isValidSectionKey(key) && key != "category" && key != "issue" && key != "date" && key != "title" {
			errs = append(errs, fmt.Sprintf("%s: unknown section key %q", name, key))
		}
	}

	return errs
}

// LintChangelogYaml validates the changelog.yaml configuration file for syntax and required fields.
func LintChangelogYaml(path string, strict bool, noSemanticVersioning bool) []string {
	var errs []string

	data, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("%s: read error: %v", path, err)}
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		errs = append(errs, fmt.Sprintf("%s: invalid YAML: %v", path, err))
		return errs
	}

	// Check for missing optional fields and warn (title is optional in abc).
	if _, ok := raw["title"]; !ok {
		fmt.Fprintf(os.Stderr, "warning: %s: missing optional field \"title\" — consider adding a project title\n", path)
	}

	// Validate known fields.
	validKeys := map[string]bool{
		"title": true, "version_template": true, "output_format": true,
		"changelog_filename": true, "sections": true, "changelog_sort": true,
		"date_change_log_entry": true, "commitish": true, "empty_section_style": true,
		"footer": true, "homepage": true, "releases_url": true, "release_summary_template": true,
		"issue_locations": true, "default_branch": true, "version_pattern": true,
		"use_semantic_versioning": true, "keep_fragments": true, "sanitize_changelog": true,
		"changelog_metadata_dir": true, "notesdir": true, "other_project_fragment_extensions": true,
		"always_refresh": true, "use_fqcn": true, "flatmap": true, "trivial_section_name": true,
		"prelude_section_title": true, "prelude_section_name": true, "output_formats": true,
		"mention_ancestor": true, "changelog_filename_template": true,
		"changelog_filename_version_depth": true,
	}

	for key := range raw {
		if !validKeys[key] && !strings.HasPrefix(key, "_") {
			msg := fmt.Sprintf("%s: unknown field %q", path, key)
			if strict {
				errs = append(errs, msg)
			} else {
				fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
			}
		}
	}

	// Validate use_semantic_versioning field type.
	if v, ok := raw["use_semantic_versioning"]; ok {
		switch val := v.(type) {
		case bool:
			_ = val
		default:
			errs = append(errs, fmt.Sprintf("%s: 'use_semantic_versioning' must be a boolean", path))
		}
	}

	if noSemanticVersioning && errs == nil {
		fmt.Fprintf(os.Stderr, "note: --no-semantic-versioning flag set — ignoring semantic versioning checks\n")
	}

	return errs
}
