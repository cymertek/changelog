# Advanced Configuration Reference

This document provides a complete reference for all `config.yaml` options in changelog. The tool supports customizable configuration with template-based output generation.

## Configuration File Location

```
project-root/
└── changelogs/
    └── config.yaml   # Main configuration file
```

The init command generates this file automatically:

```bash
changelog init changelogs/fragments
```

## Global Settings

### `title`

**Type**: string  
**Required**: Yes  
**Default**: `"Project"`

Display title for the changelog header.

```yaml
title: My Project
```

Renders as:

```markdown
# My Project v1.0.0
```

---

### `version_pattern`

**Type**: string (regex)  
**Required**: No  
**Default**: `"^\\d+\\.\\d+\\.\\d+$"` (semver) or `"^\\d+\\.\\d+$"` (classic)

Regex pattern for validating version strings passed to the release command.

```yaml
# Semver format (recommended)
version_pattern: '^\d+\.\d+\.\d+$'

# Classic MAJOR.MINOR format
version_pattern: '^\d+\.\d+$'

# Custom with pre-release tags
version_pattern: '^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.]+)?$'
```

Invalid versions are rejected:

```bash
$ changelog release --version 1.0
Error: Invalid version "1.0" — must match pattern ^\d+\.\d+\.\d+$
```

---

### `use_semantic_versioning`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Enable semantic versioning (MAJOR.MINOR.PATCH) instead of classic MAJOR.MINOR versions.

```yaml
use_semantic_versioning: true
```

When enabled, defaults to semver regex patterns and supports pre-release tags.

---

### `release_tag_re`

**Type**: string (regex)  
**Required**: No  
**Default**: `'((?:[\d.ab]|rc)+)'`

Regex pattern matching stable release tag format. Used for version detection from Git tags.

```yaml
release_tag_re: 'v(?P<version>\d+\.\d+\.\d+)'
```

---

### `pre_release_tag_re`

**Type**: string (regex)  
**Required**: No  
**Default**: `'(?!.*rc\d*$)\d+\.\d+\.\d+$'`

Regex pattern matching pre-release tag format (e.g., `v1.0.0-beta.1`, `v2.0.0-rc1`).

```yaml
pre_release_tag_re: '(?P<pre_release>\.\d+(?:[ab]|rc)+\d*)$'
```

---

## Fragment Directory Settings

### `notes_dir`

**Type**: string  
**Required**: No  
**Default**: `"fragments"`

Subdirectory name for changelog fragment files (relative to `changelogs/`).

```yaml
notes_dir: fragments
# Full path: changelogs/fragments/
```

---

### `archive_path_template`

**Type**: string  
**Required**: No  
**Default**: `"changelogs/fragments/archive"`

Destination directory for archived fragments after release. Supports template variables:

- `{date}` — Release date (YYYY-MM-DD)
- `{version}` — Release version

```yaml
# Static archive path
archive_path_template: "changelogs/fragments/archive"

# Date-based subdirectories
archive_path_template: "changelogs/fragments/archive/{date}"

# Version-based subdirectories
archive_path_template: "changelogs/fragments/archive/{version}"
```

---

### `keep_fragments`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Control fragment retention after release:

- `false` — Archive fragments to `archive_path_template` (default)
- `true` — Delete fragments after release

```yaml
# Keep archived fragments for audit trail
keep_fragments: false

# Delete fragments after release to save space
keep_fragments: true
```

---

### `ignore_other_fragment_extensions`

**Type**: boolean  
**Required**: No  
**Default**: `true`

Ignore fragment files with extensions other than `.yml`/`.yaml`.

```yaml
# Only process .yml and .yaml files (default)
ignore_other_fragment_extensions: true

# Process all files regardless of extension
ignore_other_fragment_extensions: false
```

---

### `prevent_known_fragments`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Prevent re-adding fragments that already have a filename used in prior releases.

```yaml
# Prevent duplicate fragment filenames (default)
prevent_known_fragments: false

# Allow overwriting existing fragment files
prevent_known_fragments: true
```

---

## Output Sanitization Settings

### `sanitize_changelog`

**Type**: boolean  
**Required**: No  
**Default**: `true`

Run fragment bodies through a markdown formatter before inclusion in the changelog.

```yaml
# Enable sanitization (default)
sanitize_changelog: true

# Disable to preserve raw fragment content
sanitize_changelog: false
```

---

### `always_refresh`

**Type**: string  
**Required**: No  
**Default**: `"full"`

Control which fragments are included on release:

- `"full"` — Include all fragments (default)
- `"none"` — Only include new (unreleased) fragments since last release

```yaml
# Always regenerate changelog from all fragments
always_refresh: full

# Only include fragments added since last release
always_refresh: none
```

---

## Entry Sorting Settings

### `changelog_sort`

**Type**: string  
**Required**: No  
**Default**: `"none"`

Sort mode for changelog entries. Options:

- `"none"` — No sorting (default; uses category priority)
- `"alphanumerical"` — Sort alphabetically by issue number
- `"version"` — Sort by version (requires semver)

```yaml
# Default: no sorting
changelog_sort: none

# Sort entries alphabetically by issue number
changelog_sort: alphanumerical

# Sort by semantic version
changelog_sort: version
```

---

## Plugin / Collection Settings

### `use_fqcn`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Qualify plugin names with Fully Qualified Collection Name (FQCN) in changelog output.

```yaml
# Enable fully qualified plugin names in output
use_fqcn: true

# Use short plugin names (default)
use_fqcn: false
```

---

### `add_plugin_period`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Append a period after plugin short names in output.

```yaml
# Add period to short names (e.g., "debug.")
add_plugin_period: true

# Don't add period (default)
add_plugin_period: false
```

---

### `flatmap`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Use flat (short) plugin paths instead of FQCN. Nil = unset (default), true = enabled, false = disabled.

```yaml
# Use short plugin names
flatmap: true

# Use FQCN
flatmap: false
```

---

### `new_plugins_after_name`

**Type**: string  
**Required**: No  
**Default**: `""`

Name prefix for the "new plugins" marker section in release output.

```yaml
# Custom prefix for new plugins section
new_plugins_after_name: "New Plugins:"

# Default (empty)
new_plugins_after_name: ''
```

---

## Unreleased Section Settings

### `prelude_section_title`

**Type**: string  
**Required**: No  
**Default**: `"## [Unreleased]"`

Display title for the unreleased/pre-release section in CHANGELOG.md output.

```yaml
# Custom prelude title
prelude_section_title: '## [Upcoming Release]'

# Default
prelude_section_title: '## [Unreleased]'
```

---

### `prelude_section_name`

**Type**: string  
**Required**: No  
**Default**: `""`

Name/key used to categorize pre-release fragments. Empty = use category labels.

```yaml
# Use a specific section name for unreleased fragments
prelude_section_name: "unreleased"

# Default (use fragment category directly)
prelude_section_name: ''
```

---

## Custom Section Headings

### `sections`

**Type**: map[string]string  
**Required**: No  
**Default**: Built-in section mappings

Maps internal section keys to display titles. Customize or add new sections.

```yaml
# Default sections with custom titles
sections:
  major_changes: "Major Changes"
  minor_changes: "Minor Changes & Enhancements"
  bugfixes: "Bug Fixes"
  breaking_changes: "Breaking Changes / Porting Guide"
  deprecated_features: "Deprecated Features"
  removed_features: "Removed Features"
  security_fixes: "Security Fixes"
  known_issues: "Known Issues"

# Add custom sections
sections:
  performance: "Performance Improvements"
  documentation: "Documentation Updates"
```

If a section key isn't in `config.yaml`, the key itself is used as the display title (e.g., `custom_section` → "custom_section").

---

## Trivial / Hidden Changes

### `trivial_section_name`

**Type**: string  
**Required**: No  
**Default**: `"Trivial"`

Section header name for trivial changes. These are excluded from RST output but included in Markdown/HTML.

```yaml
# Custom trivial section name
trivial_section_name: "Minor Tweaks"

# Default
trivial_section_name: Trivial
```

---

## Output Format Settings

### `output_formats` (Deprecated)

**Type**: list[string]  
**Required**: No  
**Default**: `["markdown"]`

Supported output formats. **Deprecated**: Use `templates` instead for fine-grained control.

```yaml
# Legacy format (still supported)
output_formats:
  - markdown
  - html
  - latex
```

---

### `templates`

**Type**: list[object]  
**Required**: No  
**Default**: Built-in Markdown template

Configure custom templates for different output formats. Each template object has:

| Field          | Type   | Required | Description                                  | Example                          |
|----------------|--------|----------|----------------------------------------------|----------------------------------|
| `name`         | string | Yes      | Template display name                        | `"html-report"`                  |
| `format`       | string | Yes      | Output format (`markdown`, `html`, `latex`, `txt`) | `"html"`                   |
| `path`         | string | No       | Path to template file (relative to project root or changelogs/) | `"changelogs/template.html"` |
| `output_file`  | string | No       | Output filename (defaults to `CHANGELOG.{ext}`) | `"changelog.html"`          |

**Important**: All configured templates render simultaneously in a single `release` command. You do not need separate invocations for each format — every template entry produces its own output file at the same time.

```yaml
templates:
  - name: markdown-default
    format: markdown
    output_file: CHANGELOG.md

  - name: html-branded
    format: html
    path: changelogs/template.html
    output_file: changelog.html

  - name: latex-pdf
    format: latex
    path: changelogs/template.tex
    output_file: changelog.tex

  - name: plain-text
    format: txt
    path: changelogs/template.txt
    output_file: changelog.txt
```

#### Template Resolution Order

When resolving template paths, changelog checks:

1. `baseDir/{path}` (project root)
2. `baseDir/changelogs/{path}` (changelogs directory)

If not found in either location, release fails with an error.

#### Simultaneous Rendering

All templates listed under `templates:` render together when you run:

```bash
changelog release --version v1.0.0
```

This command produces one output file per template entry — for the example above, it generates all four files (`CHANGELOG.md`, `changelog.html`, `changelog.tex`, `changelog.txt`) in a single invocation. Each template uses its own encoding rules: HTML templates escape with `encodeHTML`, LaTeX templates escape with `encodeTeX`, and plain text/markdown use passthrough.

See the working example at [`example_tests/multi_format_release/`](../example_tests/multi_format_release/) for a complete MD + HTML + TXT setup that generates all three formats from one release command.

---

## Archive and Metadata Settings

### `changes_file`

**Type**: string  
**Required**: No  
**Default**: `".changes.yaml"`

Path to the machine-readable changelog metadata file (relative to project root).

```yaml
# Default location
changes_file: '.changes.yaml'

# Custom location
changes_file: 'changelogs/.release-metadata.yaml'
```

---

### `changes_format`

**Type**: string  
**Required**: No  
**Default**: `""`

How fragment bodies are rendered (empty = raw markdown). Options:

- `""` — Raw Markdown (default)
- `"markdown"` — Sanitized Markdown
- `"rst"` — Restructured Text
- `"html"` — HTML-escaped

```yaml
# Default: raw markdown
changes_format: ''

# Sanitize fragment bodies as Markdown
changes_format: markdown
```

---

## Deprecated / Legacy Settings

### `changelog_filename_template` (Deprecated)

**Type**: string  
**Required**: No  
**Default**: `"{issue}-{category}"`

Template for fragment filenames. Available placeholders: `{issue}`, `{category}`.

```yaml
# Default naming convention
changelog_filename_template: '{issue}-{category}'

# Custom naming with date
changelog_filename_template: '{date}-{issue}-{category}'
```

---

### `changelog_filename_version_depth` (Deprecated)

**Type**: integer  
**Required**: No  
**Default**: `0`

Number of version segments to include in fragment filenames. Deprecated: use the structured `output` field instead.

```yaml
# Default: no version depth
changelog_filename_version_depth: 0

# Include major.minor in filename (e.g., v1.2-PG-001-bugfixes.yml)
changelog_filename_version_depth: 2
```

---

### `changelog_nice_yaml`

**Type**: boolean  
**Required**: No  
**Default**: `false`

YAML encoding adjustment for linting tool compatibility.

```yaml
# Enable for linting tool compliance
changelog_nice_yaml: true

# Default (standard YAML)
changelog_nice_yaml: false
```

---

## Project Type Settings

### `is_other_project`

**Type**: boolean  
**Required**: No  
**Default**: `false`

True if this project wraps another (e.g., a derived project using the upstream as a dependency). Enables ancestor mention and skips galaxy.yml lookup.

```yaml
# Standard project (default)
is_other_project: false

# Wrapper/derived project
is_other_project: true
```

---

### `mention_ancestor`

**Type**: boolean  
**Required**: No  
**Default**: `false`

Enables an "Ancestor Changes" section in release output when `is_other_project=true`. Set to true to include ancestor changelog details from the upstream project.

```yaml
# Include ancestor changes section
mention_ancestor: true

# Default (no ancestor section)
mention_ancestor: false
```

---

## VCS Settings

### `vcs`

**Type**: string  
**Required**: No  
**Default**: `"auto"`

Version control detection mode when scanning plugin files. Options:

- `"auto"` — Detect VCS automatically (default)
- `"git"` — Force Git detection
- `"none"` — Disable VCS scanning

```yaml
# Auto-detect version control system
vcs: auto

# Force Git
vcs: git

# Disable VCS scanning
vcs: none
```

---

## Complete Example config.yaml

```yaml
# ============================================================================
# Changelog Configuration
# Generated by changelog init — edit this file to customize changelog behavior.
# Full reference: see docs within this project
# ============================================================================

# --------------------------------------------------------------------------
# General settings
# --------------------------------------------------------------------------

title: My Project
version_pattern: '^\d+\.\d+\.\d+$'

# --------------------------------------------------------------------------
# Fragment directory layout
# --------------------------------------------------------------------------

notes_dir: fragments
vcs: auto

# --------------------------------------------------------------------------
# Project type
# --------------------------------------------------------------------------

is_other_project: false
mention_ancestor: false

# --------------------------------------------------------------------------
# Fragment file handling
# --------------------------------------------------------------------------

keep_fragments: false
ignore_other_fragment_extensions: true
prevent_known_fragments: false

# --------------------------------------------------------------------------
# Output sanitization and release behavior
# --------------------------------------------------------------------------

sanitize_changelog: true
always_refresh: full

# --------------------------------------------------------------------------
# Entry sorting
# --------------------------------------------------------------------------

changelog_sort: none

# --------------------------------------------------------------------------
# Custom section headings
# --------------------------------------------------------------------------

sections:
  major_changes: "Major Changes"
  minor_changes: "Minor Changes & Enhancements"
  bugfixes: "Bug Fixes"
  breaking_changes: "Breaking Changes / Porting Guide"
  deprecated_features: "Deprecated Features"
  removed_features: "Removed Features"
  security_fixes: "Security Fixes"
  known_issues: "Known Issues"

# --------------------------------------------------------------------------
# Output formats and templates
# --------------------------------------------------------------------------

templates:
  - name: markdown-default
    format: markdown
    output_file: CHANGELOG.md

  - name: html-branded
    format: html
    path: changelogs/template.html
    output_file: changelog.html

  - name: latex-pdf
    format: latex
    path: changelogs/template.tex
    output_file: changelog.tex

# --------------------------------------------------------------------------
# Archive settings
# --------------------------------------------------------------------------

archive_path_template: "changelogs/fragments/archive/{date}"
changes_file: '.changes.yaml'

# --------------------------------------------------------------------------
# Deprecated / legacy compat
# --------------------------------------------------------------------------

changelog_filename_template: '{issue}-{category}'
changelog_filename_version_depth: 0
changelog_nice_yaml: false
```

---

## Next Steps

- **[End-to-End Workflow](end-to-end-workflow.md)** — Full example from init to production deployment
- **[Template Customization](template-customization.md)** — Write custom HTML/LaTeX templates

---

**Need help?** Check the [FAQ](../README.md#faq) or open an issue on [GitHub](https://github.com/cymertek/changelog/issues).
