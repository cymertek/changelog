# Getting Started with changelog

This guide walks you through installing changelog, initializing your changelog infrastructure, and creating your first release. By the end, you'll have a working changelog pipeline ready for CI/CD integration.

## Prerequisites

- **Go 1.21+** (for building from source) or download a pre-built binary
- A Git repository with version-controlled code
- Basic familiarity with YAML configuration files

## Installation

### Option 1: Download Pre-built Binary

Visit the [GitHub Releases page](https://github.com/cymertek/changelog/releases) and download the latest release for your platform:

```bash
# Linux (amd64)
wget https://github.com/cymertek/changelog/releases/latest/download/changelog-linux-amd64
chmod +x changelog-linux-amd64
sudo mv changelog-linux-amd64 /usr/local/bin/changelog

# macOS (arm64)
wget https://github.com/cymertek/changelog/releases/latest/download/changelog-darwin-arm64
chmod +x changelog-darwin-arm64
sudo mv changelog-darwin-arm64 /usr/local/bin/changelog

# Windows (amd64)
wget https://github.com/cymertek/changelog/releases/latest/download/changelog-windows-amd64.exe
```

### Option 2: Build from Source

```bash
git clone https://github.com/cymertek/changelog.git
cd changelog
go build -o changelog .
sudo mv changelog /usr/local/bin/
```

Verify installation:

```bash
changelog version
# Output: changelog v0.1.0 (built with Go 1.23)
```

### Option 3: Install via Homebrew (macOS/Linux)

```bash
brew tap cymertek/changelog
brew install changelog
```

## Initialize Your Project

Navigate to your project root and run the init command:

```bash
cd /path/to/your/project
changelog init changelogs/fragments
```

This creates:

```
project-root/
├── changelogs/
│   ├── config.yaml              # Changelog configuration
│   └── fragments/               # Directory for fragment files
│       └── archive/             # (created on first release)
```

### What config.yaml Contains

The generated `config.yaml` includes all standard changelog settings:

- **title**: Project name displayed in changelog header
- **version_pattern**: Regex for validating version strings
- **sections**: Default section definitions (major_changes, bugfixes, etc.)
- **output_formats**: Configured output formats (markdown by default)
- **templates**: Custom template paths and format settings

Edit `config.yaml` to customize:

```yaml
title: My Project
version_pattern: '^\d+\.\d+\.\d+$'
sections:
  major_changes: "Major Changes"
  bugfixes: "Bug Fixes"
  breaking_changes: "Breaking Changes / Porting Guide"
output_formats:
  - markdown
templates:
  - name: markdown
    format: markdown
    output_file: CHANGELOG.md
```

See [Advanced Configuration](advanced-config.md) for the full reference.

## Your First Fragment

Add a fragment describing your first change:

```bash
changelog add --category bugfixes -i PG-001 -T "Fixed null pointer crash in auth handler"
```

This creates `changelogs/fragments/PG-001-bugfixes.yml`:

```yaml
---
category: bugfixes
issue: PG-001
date: "2026-07-27"
title: "Fixed null pointer crash in auth handler"
author: ""
---

bugfixes: |
    - Fixed null pointer crash in auth handler when processing empty payloads
```

### Fragment Structure Explained

**Frontmatter** (between `---` delimiters):

| Field      | Required | Description                          | Example                |
|------------|----------|--------------------------------------|------------------------|
| category   | Yes      | Section key matching config.yaml     | `bugfixes`, `enhancement` |
| issue      | Yes      | Issue/PR number or unique identifier | `PG-001`, `GH-42`      |
| date       | No       | Release date (defaults to today)     | `"2026-07-27"`         |
| author     | No       | Author name                          | `"Jane Doe"`           |
| title      | Yes      | One-line summary of the change       | `"Fixed null pointer crash"` |

**Body**: Section-based content. The key must match a section defined in `config.yaml`:

```yaml
bugfixes: |
    - Fixed null pointer crash in auth handler when processing empty payloads

major_changes: |
    - Added support for YAML fragments with frontmatter metadata
```

### Adding More Fragments

Each change gets its own fragment file. Add multiple fragments before releasing:

```bash
changelog add --category enhancement -i PG-002 -T "Added support for custom templates"
changelog add --category bugfixes -i PG-003 -T "Fixed race condition in parallel processing"
changelog add --category breaking_changes -i PG-004 -T "Removed deprecated --legacy flag"
```

## Generate Your First Release

Once you have fragments ready, run the release command:

```bash
changelog release --version v1.0.0
```

This generates `CHANGELOG.md` (or other configured formats) and archives released fragments to `changelogs/fragments/archive/2026-07-27/`.

### Output Example

```
Generated changelog for version v1.0.0 (4 fragments)
Generated: changelogs/CHANGELOG.md
Saved release metadata to changelogs/.changes.yaml
Archived fragment: PG-001-bugfixes.yml -> changelogs/fragments/archive/2026-07-27
Archived fragment: PG-002-enhancement.yml -> changelogs/fragments/archive/2026-07-27
Archived fragment: PG-003-bugfixes.yml -> changelogs/fragments/archive/2026-07-27
Archived fragment: PG-004-breaking_changes.yml -> changelogs/fragments/archive/2026-07-27
```

### Release Output (`CHANGELOG.md`)

```markdown
# My Project v1.0.0

Released: 2026-07-27

## Major Changes

* Added support for YAML fragments with frontmatter metadata

## Bug Fixes

* Fixed null pointer crash in auth handler when processing empty payloads
* Fixed race condition in parallel processing

## Breaking Changes / Porting Guide

* Removed deprecated --legacy flag
```

## Commit and Tag Your Release

Follow standard Git workflow:

```bash
git add CHANGELOG.md changelogs/fragments/archive/
git commit -m "Release v1.0.0"
git tag -a v1.0.0 -m "v1.0.0"
git push origin main --tags
```

## Next Steps

- **[Fragment Writing Guide](fragment-guide.md)** — Learn advanced fragment formats, multiline content, and edge cases
- **[Release Workflow](release-workflow.md)** — Understand versioning strategies (semver vs classic) and release automation
- **[CI/CD Integration](ci-cd-integration.md)** — Set up automated changelog generation in GitHub Actions

---

**Need help?** Check the [FAQ](../README.md#faq) or open an issue on [GitHub](https://github.com/cymertek/changelog/issues).
