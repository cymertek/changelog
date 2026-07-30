// Package changelog provides changelog management functionality including fragment parsing,
// configuration loading, and template rendering for generating upstream-compatible outputs.
package changelog

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// TemplateConfig represents a custom output template configuration.
type TemplateConfig struct {
	Name        string `yaml:"name"`                  // template name identifier (e.g., "html", "latex")
	Format      string `yaml:"format"`                // output format: html, txt, latex, rst
	Path        string `yaml:"path"`                  // path to template file relative to changelogs/ dir
	OutputFile  string `yaml:"output_file,omitempty"` // optional output filename pattern (default: CHANGELOG.{ext})
	Description string `yaml:"description,omitempty"` // human-readable description
}

// Config represents the changelog.yaml configuration file.
// Fields follow upstream changelog tool conventions where applicable.
type Config struct {
	Title                         string `yaml:"title"`
	VersionPattern                string `yaml:"version_pattern,omitempty"`                  // semver regex pattern
	KeepFragments                 bool   `yaml:"keep_fragments,omitempty"`                   // retain fragments after release (default: false)
	IsOtherProject                bool   `yaml:"is_other_project,omitempty"`                 // true when this is a wrapper project, not the upstream itself (default: true)
	SanitizeChangelog             bool   `yaml:"sanitize_changelog,omitempty"`               // run body through markdown formatter (default: true)
	UseFQCN                       bool   `yaml:"use_fqcn,omitempty"`                         // qualify plugin names with FQCN in release output (default: false)
	TrivialSectionName            string `yaml:"trivial_section_name,omitempty"`             // name for the "trivial" section header (default: "Trivial")
	UseSemanticVersioning         bool   `yaml:"use_semantic_versioning,omitempty"`          // enforce semver for version strings in release output (default: false)
	IgnoreOtherFragmentExtensions bool   `yaml:"ignore_other_fragment_extensions,omitempty"` // only process .md fragments, not other extensions (default: true)
	// MentionAncestor controls whether an "Ancestor Changes" section appears when is_other_project=true.
	// Stored as bool=True; we mirror that semantics.
	MentionAncestor bool `yaml:"mention_ancestor,omitempty"` // enable ancestor changelog mention (default: true)

	// Fragment directory layout — maps upstream's "notes_dir" concept.
	// The first entry is the primary fragment dir; subsequent entries are merged into it
	// (with later dirs overriding earlier ones on filename collision).
	NotesDirs []string `yaml:"-"` // not serialized to YAML, derived from config

	// Filename / template control — these affect how fragments are named and rendered.
	ChangelogFilenameTemplate     string `yaml:"changelog_filename_template,omitempty"`      // template for fragment filenames (default: "{issue}-{category}")
	ChangelogFilenameVersionDepth int    `yaml:"changelog_filename_version_depth,omitempty"` // depth to include in version segment of filename, e.g. 1 = "v1" (0 = none)

	// Section customization — overrides the built-in category headings.
	// Uses Mapping[str,str] format: {"key": "Display Title"}.
	// The special prelude_name/prelude_title pair is prepended by postprocessing.
	PreludeSectionTitle string            `yaml:"prelude_section_title,omitempty"` // title for unreleased/pre-release section header (default: "## [Unreleased]")
	PreludeSectionName  string            `yaml:"prelude_section_name,omitempty"`  // name/key for unreleased/pre-release fragments (default: "")
	Sections            map[string]string `yaml:"sections,omitempty"`              // maps internal keys to display titles

	// Plugin-specific fields (useful when is_other_project=true for wrapper repos).
	NewPluginsAfterName string `yaml:"new_plugins_after_name,omitempty"` // name prefix for "new plugins" marker in release output

	// Changes format control.
	ChangesFormat string   `yaml:"changes_format,omitempty"` // how to render fragment bodies (empty = raw markdown)
	OutputFormats []string `yaml:"output_formats,omitempty"` // which formats to emit (default: ["markdown"])

	// Template outputs — custom Go templates for additional changelog formats.
	Templates []TemplateConfig `yaml:"templates,omitempty"` // list of template configurations for custom output formats

	// Additional upstream compat fields.
	AddPluginPeriod     bool   `yaml:"add_plugin_period,omitempty"`     // add period after plugin names in output
	AlwaysRefresh       string `yaml:"always_refresh,omitempty"`        // "none" or "full" — fragments to include on release
	ArchivePathTemplate string `yaml:"archive_path_template,omitempty"` // destination dir for archived/moved fragments
	ChangesFile         string `yaml:"changes_file,omitempty"`          // machine-readable storage path (e.g. ".changes.yaml")
	ChangelogNiceYaml   bool   `yaml:"changelog_nice_yaml,omitempty"`   // YAML encoding adjustment for linter compat
	ChangelogSort       string `yaml:"changelog_sort,omitempty"`        // "none" or field name to sort entries by (e.g. "issue")
	// Flatmap is a three-state bool (nil=unset, true=enabled, false=disabled).
	// Uses Optional[bool]=None; we mirror with *bool.
	Flatmap               *bool  `yaml:"flatmap,omitempty"`                 // use short plugin names instead of FQCN in output
	PreventKnownFragments bool   `yaml:"prevent_known_fragments,omitempty"` // block duplicate fragment filenames on release
	ReleaseTagRe          string `yaml:"release_tag_re,omitempty"`          // regex matching stable release tag format (e.g. "((?:[\d.ab]|rc)+)")
	PreReleaseTagRe       string `yaml:"pre_release_tag_re,omitempty"`      // regex matching prerelease tag format
	Vcs                   string `yaml:"vcs,omitempty"`                     // version control detection ("none", "git", "auto")
	NotesDir              string `yaml:"notes_dir,omitempty"`               // subdirectory name for fragment files (default: "fragments" in changelog)

}

const (
	defaultTrivialSectionName        = "Trivial"
	defaultChangelogFilenameTemplate = "{issue}-{category}"
	defaultPreludeSectionTitle       = "## [Unreleased]"
)

// categoryPriority defines render ordering for categories (lower = earlier in output).
var categoryPriority = map[string]int{
	"breaking":    0,
	"enhancement": 1,
	"deprecation": 2,
	"removal":     3,
	"note":        4,
	"trivial":     5,
}

// Defaults returns a Config with all upstream-compatible defaults populated.
func (c *Config) Defaults() {
	if !c.IsOtherProject {
		c.IsOtherProject = true
	}
	if !c.MentionAncestor && c.IsOtherProject {
		c.MentionAncestor = true
	}
	if !c.SanitizeChangelog {
		c.SanitizeChangelog = true
	}
	if !c.IgnoreOtherFragmentExtensions {
		c.IgnoreOtherFragmentExtensions = true
	}
	if c.ChangelogFilenameTemplate == "" {
		c.ChangelogFilenameTemplate = defaultChangelogFilenameTemplate
	}
	if c.PreludeSectionTitle == "" {
		c.PreludeSectionTitle = defaultPreludeSectionTitle
	}
	if c.TrivialSectionName == "" {
		c.TrivialSectionName = defaultTrivialSectionName
	}
	// Set default templates if none configured.
	if len(c.Templates) == 0 {
		c.Templates = []TemplateConfig{
			{Name: "markdown", Format: "markdown", OutputFile: "CHANGELOG.md"},
		}
	}

	if c.AlwaysRefresh == "" {
		c.AlwaysRefresh = "full" // include all fragments on release by default
	}
	if c.ChangelogSort == "" {
		c.ChangelogSort = "none" // no sorting — rely on category priority in render.go
	}
	if c.ArchivePathTemplate == "" {
		c.ArchivePathTemplate = ".changelogs/archive" // move fragments to archive dir after release
	}
	if c.ChangesFile == "" {
		c.ChangesFile = ".changes.yaml"
	}
	if c.ReleaseTagRe == "" {
		c.ReleaseTagRe = `((?:[\d.ab]|rc)+)`
	}
	if c.PreReleaseTagRe == "" {
		c.PreReleaseTagRe = `(?P<pre_release>\.\d+(?:[ab]|rc)+\d*)$`
	}
	if c.Vcs == "" {
		c.Vcs = "auto" // auto-detect version control (git, none)
	}
	if c.NotesDir == "" {
		c.NotesDir = "fragments" // fragment directory name for YAML fragments (upstream compat)
	}
}

// Load reads and validates a changelog.yaml config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s at %v", path, err)
	}

	cfg.Defaults()

	if cfg.Title == "" {
		return nil, fmt.Errorf("config %s: 'title' is required", path)
	}

	if cfg.VersionPattern != "" {
		if _, err := regexp.Compile(cfg.VersionPattern); err != nil {
			return nil, fmt.Errorf("config %s: invalid version_pattern regex %q: %w", path, cfg.VersionPattern, err)
		}
	}

	if cfg.IsOtherProject && !cfg.MentionAncestor {
		return nil, fmt.Errorf("config %s: set mention_ancestor to enable ancestor changelog mention (or disable is_other_project)", path)
	}

	return &cfg, nil
}

// Save writes the config to a YAML file. The output starts with "---" so it lints correctly.
func Save(c *Config, path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	out := append([]byte("---\n"), data...)

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}

	return nil
}

// SaveCommented writes the config as a richly-commented YAML file suitable for init().
// Each section is grouped with a header comment and every field has an inline explanation.
func (c *Config) SaveCommented(path string) error {
	f := func(v bool) string {
		if v {
			return "true"
		}
		return "false"
	}

	// fPtr handles *bool: nil=unset, true="true", false="false".
	// Mirrors upstream's Optional[bool] semantics.
	fPtr := func(v *bool) string {
		if v == nil {
			return "# (unset)"
		}
		return f(*v)
	}

	// quoteIfNecessary wraps values that need quoting in YAML.
	// Uses single quotes (YAML literal scalar) instead of double quotes to avoid
	// PyYAML/ansible-lint issues with regex escape sequences like \d, \., etc.
	// Single-quoted strings treat everything literally except '' which = one '.
	quoteIfNecessary := func(s string) string {
		if s == "" || strings.ContainsAny(s, ":#[]{}|>&!*?,.%\n\r\\") {
			// Escape single quotes by doubling them (YAML 1.2 literal scalar rule)
			return "'" + strings.ReplaceAll(s, "'", "''") + "'"
		}
		return s
	}

	var lines []string
	lines = append(lines, "---")
	lines = append(lines, "")
	lines = append(lines, "# ============================================================================")
	lines = append(lines, "# Changelog Configuration")
	lines = append(lines, "# Generated by changelog init — edit this file to customize changelog behavior.")
	lines = append(lines, "# Full reference: see docs within this project")
	lines = append(lines, "# ============================================================================")
	lines = append(lines, "")

	// --- General ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# General settings                                                           ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Project title displayed in the changelog header.")
	lines = append(lines, "title: "+quoteIfNecessary(c.Title))
	lines = append(lines, "")
	lines = append(lines, "# Regex pattern used to validate version strings during release (e.g., v1.2.3).")
	lines = append(lines, "version_pattern: "+quoteIfNecessary(c.VersionPattern))
	lines = append(lines, "")

	// --- Fragment layout ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Fragment directory layout                                                   ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Subdirectory name for changelog fragment files (default: \"fragments\").")
	lines = append(lines, "notes_dir: "+quoteIfNecessary(c.NotesDir))
	lines = append(lines, "")
	lines = append(lines, "# Version control detection mode when scanning plugin files.")
	lines = append(lines, "# Options: \"auto\" (default), \"git\", \"none\"")
	lines = append(lines, "vcs: "+quoteIfNecessary(c.Vcs))
	lines = append(lines, "")

	// --- Project type ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Project type — set is_other_project=true for wrapper/derived repos.         ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# True if this project wraps another (e.g., a derived project using")
	lines = append(lines, "# the upstream as a dependency). Enables ancestor mention and skips galaxy.yml lookup.")
	lines = append(lines, "is_other_project: "+f(c.IsOtherProject))
	lines = append(lines, "")
	lines = append(lines, "# Enables an \"Ancestor Changes\" section in release output when is_other_project=true.")
	lines = append(lines, "# Set to true to include ancestor changelog details from the upstream project.")
	lines = append(lines, "mention_ancestor: "+f(c.MentionAncestor))
	lines = append(lines, "")

	// --- Fragment processing ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Fragment file handling                                                      ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# After release: delete old fragments (false) or keep them (true).")
	lines = append(lines, "keep_fragments: "+f(c.KeepFragments))
	lines = append(lines, "")
	lines = append(lines, "# Ignore fragment files with extensions other than .md (and optionally .yml/.yaml).")
	lines = append(lines, "ignore_other_fragment_extensions: "+f(c.IgnoreOtherFragmentExtensions))
	lines = append(lines, "")
	lines = append(lines, "# Prevent re-adding fragments that already have a filename used in prior releases.")
	lines = append(lines, "# Defaults to the value of keep_fragments.")
	lines = append(lines, "prevent_known_fragments: "+f(c.PreventKnownFragments))
	lines = append(lines, "")

	// --- Sanitization & refresh ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Output sanitization and release behavior                                    ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Run fragment bodies through a markdown formatter before inclusion.")
	lines = append(lines, "sanitize_changelog: "+f(c.SanitizeChangelog))
	lines = append(lines, "")
	lines = append(lines, "# Which fragments to include on release:")
	lines = append(lines, "#   \"full\"  — include all (default)")
	lines = append(lines, "#   \"none\"  — only include new (unreleased) fragments")
	lines = append(lines, "always_refresh: "+quoteIfNecessary(c.AlwaysRefresh))
	lines = append(lines, "")

	// --- Sorting ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Entry sorting                                                               ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Sort changelog entries by field. Options:")
	lines = append(lines, "#   \"none\"          — no sorting (default; uses category priority)")
	lines = append(lines, "#   \"alphanumerical\" — sort alphabetically by issue number")
	lines = append(lines, "#   \"version\"       — sort by version")
	lines = append(lines, "changelog_sort: "+quoteIfNecessary(c.ChangelogSort))
	lines = append(lines, "")

	// --- Plugin / Ansible-specific ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Plugin / Collection settings (relevant when is_other_project=true)  ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Qualify plugin names with Fully Qualified Collection Name (FQCN)")
	lines = append(lines, "# in the changelog output (e.g., collection.plugin_name).")
	lines = append(lines, "use_fqcn: "+f(c.UseFQCN))
	lines = append(lines, "")
	lines = append(lines, "# Append a period after plugin short names in output.")
	lines = append(lines, "add_plugin_period: "+f(c.AddPluginPeriod))
	lines = append(lines, "")
	lines = append(lines, "# Use flat (short) plugin paths instead of FQCN. nil=unset (default), true=enabled, false=disabled.")
	lines = append(lines, "flatmap: "+fPtr(c.Flatmap))
	lines = append(lines, "")
	lines = append(lines, "# Name prefix for the \"new plugins\" marker section in release output.")
	lines = append(lines, "new_plugins_after_name: "+quoteIfNecessary(c.NewPluginsAfterName))
	lines = append(lines, "")

	// --- Semantic versioning ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Semantic versioning controls                                                ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Use semantic versioning (MAJOR.MINOR.PATCH) instead of classic MAJOR.MINOR versions")
	lines = append(lines, "# version numbers. When true, release_tag_re and pre_release_tag_re are used.")
	lines = append(lines, "use_semantic_versioning: "+f(c.UseSemanticVersioning))
	lines = append(lines, "")
	lines = append(lines, "# Regex pattern matching stable release tag format (e.g., \"v1.2.3\").")
	lines = append(lines, "release_tag_re: "+quoteIfNecessary(c.ReleaseTagRe))
	lines = append(lines, "")
	lines = append(lines, "# Regex pattern matching pre-release tag format (e.g., \"v1.0.0rc1\").")
	lines = append(lines, "pre_release_tag_re: "+quoteIfNecessary(c.PreReleaseTagRe))
	lines = append(lines, "")

	// --- Filename templates ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Fragment filename and version depth                                         ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Template for fragment filenames. Available placeholders: {issue}, {category}.")
	lines = append(lines, "# The deprecated changelog_filename_template (RST-focused) is also supported.")
	lines = append(lines, "changelog_filename_template: "+quoteIfNecessary(c.ChangelogFilenameTemplate))
	lines = append(lines, "")
	lines = append(lines, "# Number of version segments to include in fragment filenames (0 = none).")
	lines = append(lines, "# Deprecated: use the structured \"output\" field instead.")
	lines = append(lines, "changelog_filename_version_depth: "+fmt.Sprintf("%d", c.ChangelogFilenameVersionDepth))
	lines = append(lines, "")

	// --- Prelude / unreleased section ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Unreleased (prelude) section                                                ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Display title for the unreleased/pre-release section in CHANGELOG.md output.")
	lines = append(lines, "prelude_section_title: "+quoteIfNecessary(c.PreludeSectionTitle))
	lines = append(lines, "")
	lines = append(lines, "# Name/key used to categorize pre-release fragments. Empty = use category labels.")
	lines = append(lines, "prelude_section_name: "+quoteIfNecessary(c.PreludeSectionName))
	lines = append(lines, "")

	// --- Section overrides ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Custom section heading overrides                                            ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	if len(c.Sections) > 0 {
		lines = append(lines, "# Override the default category headings. Maps internal keys to display titles.")
		lines = append(lines, "sections:")
		for k, v := range c.Sections {
			lines = append(lines, "  "+quoteIfNecessary(k)+": "+quoteIfNecessary(v))
		}
	} else {
		lines = append(lines, "# Maps internal section keys to display titles (Mapping[str,str] format).")
		lines = append(lines, "# Uncomment and customize below:")
		lines = append(lines, "# sections:")
		lines = append(lines, "#   major_changes: \"Major Changes\"")
		lines = append(lines, "#   breaking_changes: \"Breaking Changes / Porting Guide\"")
	}
	lines = append(lines, "")

	// --- Trivial section ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Trivial / hidden changes (CI-only updates)                                  ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Section header name for trivial changes. These are excluded from RST output.")
	lines = append(lines, "trivial_section_name: "+quoteIfNecessary(c.TrivialSectionName))
	lines = append(lines, "")

	// --- Output formats ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Output format control                                                       ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	if len(c.OutputFormats) > 0 {
		lines = append(lines, "# Supported output formats (changelog only produces markdown).")
		lines = append(lines, "output_formats:")
		for _, fmt := range c.OutputFormats {
			lines = append(lines, "  - "+quoteIfNecessary(fmt))
		}
	} else {
		lines = append(lines, "# Supported output formats (changelog only produces markdown).")
		lines = append(lines, "output_formats:")
		lines = append(lines, "  - markdown")
	}
	lines = append(lines, "")

	// --- Archive & machine-readable ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Archive and machine-readable storage                                        ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# Destination directory for fragments after a release (move vs delete).")
	lines = append(lines, "archive_path_template: "+quoteIfNecessary(c.ArchivePathTemplate))
	lines = append(lines, "")
	lines = append(lines, "# Path to the machine-readable changelog metadata file.")
	lines = append(lines, "changes_file: "+quoteIfNecessary(c.ChangesFile))
	lines = append(lines, "")
	lines = append(lines, "# How fragment bodies are rendered (empty = raw markdown).")
	lines = append(lines, "changes_format: "+quoteIfNecessary(c.ChangesFormat))
	lines = append(lines, "")

	// --- Deprecated compat fields ---
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "# Deprecated / legacy compat                                                  ")
	lines = append(lines, "# --------------------------------------------------------------------------")
	lines = append(lines, "")
	lines = append(lines, "# YAML encoding adjustment for linting tool compatibility.")
	lines = append(lines, "changelog_nice_yaml: "+f(c.ChangelogNiceYaml))
	lines = append(lines, "")

	// Join with newlines and write
	content := strings.Join(lines, "\n") + "\n"

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}

	return nil
}
