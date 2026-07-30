# Release Workflow Guide

This guide covers how to manage releases with changelog, including versioning strategies (semver vs classic), the release command, and best practices for maintaining a clean changelog history.

## Versioning Strategies

changelog supports two versioning modes: **semantic versioning** (semver) and **classic MAJOR.MINOR versions**. Choose based on your project's needs.

### Semantic Versioning (Recommended)

Semver uses `MAJOR.MINOR.PATCH` format with clear semantics:

- **MAJOR** — Breaking changes (e.g., `v2.0.0`)
- **MINOR** — New features, no breaking changes (e.g., `v1.1.0`)
- **PATCH** — Bug fixes only (e.g., `v1.0.3`)

Enable semver in `config.yaml`:

```yaml
use_semantic_versioning: true
release_tag_re: 'v(?P<version>\d+\.\d+\.\d+)'
pre_release_tag_re: '(?P<pre_release>\.\d+(?:[ab]|rc)+\d*)$'
```

### Classic MAJOR.MINOR Versions

Classic versions use `MAJOR.MINOR` format (e.g., `v1.0`, `v2.1`). This is the default for :

```yaml
use_semantic_versioning: false
version_pattern: '^\d+\.\d+$'
```

## The Release Command

### Basic Usage

```bash
changelog release --version v1.0.0
```

This command:

1. Reads all fragments from `changelogs/fragments/`
2. Validates fragment syntax and required fields
3. Generates changelog in configured output formats (Markdown, HTML, LaTeX, etc.)
4. Archives released fragments to `changelogs/fragments/archive/YYYY-MM-DD/`
5. Saves release metadata to `changelogs/.changes.yaml`

### Output Example

```
Generated changelog for version v1.0.0 (12 fragments)
Generated: CHANGELOG.md
Saved release metadata to changelogs/.changes.yaml
Archived fragment: PG-001-bugfixes.yml -> changelogs/fragments/archive/2026-07-27
Archived fragment: PG-002-enhancement.yml -> changelogs/fragments/archive/2026-07-27
...
```

### Dry Run Mode

Preview the release without archiving fragments or writing output:

```bash
changelog release --version v1.0.0 --dry-run
```

This is useful for testing configuration changes or validating fragment content before committing to a release.

## Release Workflow Best Practices

### 1. Maintain Fragments Throughout Development

Add fragments **as you make changes**, not just before release. This ensures:

- No forgotten changes slip into the changelog
- Each fragment describes a single, coherent change
- Reviewers can validate fragment content during PR review

**CI/CD Integration**: Add a pre-commit hook or CI check that fails if a code change isn't accompanied by a fragment.

### 2. One Fragment Per Change

Each fragment should describe exactly one change:

```bash
# ✅ Good: Two separate fragments for two changes
changelog add --category bugfixes -i PG-042 -T "Fixed connection timeout"
changelog add --category enhancement -i PG-043 -T "Added parallel processing support"

# ❌ Bad: One fragment with multiple unrelated changes
changelog add --category bugfixes -i PG-042 -T "Fixed timeout AND added parallel processing"
```

### 3. Use Descriptive Issue Numbers

Issue numbers should be unique and traceable:

- **GitHub PRs**: `GH-123` (matches pull request number)
- **Internal tickets**: `PG-001`, `ABC-456`
- **CVEs**: `CVE-2026-12345`

Avoid sequential integers without context (`001`, `002`) unless you're tracking changes internally.

### 4. Review Fragments Before Release

Before running release, audit your fragments:

```bash
# List all unreleased fragments
ls changelogs/fragments/*.yml

# Validate fragment syntax
changelog lint changelogs/fragments/

# Preview changelog (dry run)
changelog release --version v1.0.0 --dry-run
cat CHANGELOG.md  # Review output
rm CHANGELOG.md    # Clean up
```

### 5. Archive vs Delete Fragments

By default, changelog archives released fragments to `archive/YYYY-MM-DD/`. Configure this behavior in `config.yaml`:

```yaml
# Keep archived fragments (default)
keep_fragments: false  # false = archive, true = delete after release

# Custom archive path template
archive_path_template: "changelogs/fragments/archive/{date}"
```

**Recommendation**: Keep archives for audit trails. Delete only if storage is a concern.

## Version Pattern Validation

The `version_pattern` in `config.yaml` validates version strings passed to the release command:

```yaml
# Semver pattern (default when use_semantic_versioning=true)
version_pattern: '^\d+\.\d+\.\d+$'

# Classic MAJOR.MINOR pattern
version_pattern: '^\d+\.\d+$'

# Custom pattern (e.g., allow pre-release tags)
version_pattern: '^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.]+)?$'
```

Invalid version strings are rejected with an error:

```bash
$ changelog release --version 1.0
Error: Invalid version "1.0" — must match pattern ^\d+\.\d+\.\d+$
```

## Multiple Output Formats

Configure multiple output formats in `config.yaml`:

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

**A single `changelog release` command generates all configured formats simultaneously.** Running the release command produces one file per template entry — no separate invocations needed. Each template renders independently with its own encoding rules: HTML templates use `encodeHTML`, LaTeX templates use `encodeTeX`, and plain text/markdown use passthrough.

The Markdown template is built-in and doesn't require a custom file. HTML, LaTeX, and plain text formats need custom templates (see [Template Customization](template-customization.md)).

Add as many templates as you like — four common configurations:

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

One release call generates all four files at once. See the working example at [`example_tests/multi_format_release/`](../example_tests/multi_format_release/) which demonstrates MD + HTML + TXT generation from a single command.

## Release Metadata

After release, changelog saves metadata to `changelogs/.changes.yaml`:

```yaml
releases:
  - version: v1.0.0
    date: "2026-07-27"
    fragments:
      - PG-001-bugfixes.yml
      - PG-002-enhancement.yml
    output_files:
      - CHANGELOG.md
```

This file is used for:

- Detecting duplicate releases (prevents releasing the same version twice)
- Tracking which fragments were included in each release
- Auditing changelog history

## Handling Pre-Releases

For beta or release candidate versions, use pre-release tags:

```bash
changelog release --version v1.0.0-beta.1
changelog release --version v2.0.0-rc1
```

Configure pre-release patterns in `config.yaml`:

```yaml
pre_release_tag_re: '(?P<pre_release>\.\d+(?:[ab]|rc)+\d*)$'
```

Pre-releases follow the same workflow as stable releases but use different display titles (e.g., "v1.0.0-beta.1" instead of "v1.0.0").

## Rolling Back a Release

If you need to undo a release:

1. **Delete the changelog output**: `rm CHANGELOG.md`
2. **Restore archived fragments**: Move files from `archive/YYYY-MM-DD/` back to `fragments/`
3. **Remove release metadata**: Edit or delete `changelogs/.changes.yaml`
4. **Re-release with corrected version**: `changelog release --version v1.0.1`

**Note**: Once fragments are archived, they're not deleted by default (unless `keep_fragments: true`). This makes rollback straightforward.

## Common Release Scenarios

### Scenario 1: First Release of a New Project

```bash
# Initialize changelog infrastructure
changelog init changelogs/fragments

# Add initial fragments describing the first release
changelog add --category major_changes -i INIT-001 -T "Initial release with core functionality"
changelog add --category enhancement -i INIT-002 -T "Added support for YAML fragments"

# Generate changelog and tag version
changelog release --version v1.0.0
git commit -am "Release v1.0.0"
git tag -a v1.0.0 -m "v1.0.0"
```

### Scenario 2: Patch Release (Bug Fixes Only)

Increment the PATCH version:

```bash
changelog add --category bugfixes -i PG-042 -T "Fixed connection timeout on slow networks"
changelog release --version v1.0.1
git commit -am "Release v1.0.1" && git tag -a v1.0.1 -m "v1.0.1"
```

### Scenario 3: Minor Release (New Features)

Increment the MINOR version, reset PATCH to 0:

```bash
changelog add --category enhancement -i PG-043 -T "Added parallel processing support"
changelog release --version v1.1.0
git commit -am "Release v1.1.0" && git tag -a v1.1.0 -m "v1.1.0"
```

### Scenario 4: Major Release (Breaking Changes)

Increment the MAJOR version, reset MINOR and PATCH to 0:

```bash
changelog add --category breaking_changes -i PG-044 -T "Removed deprecated --legacy flag"
changelog release --version v2.0.0
git commit -am "Release v2.0.0" && git tag -a v2.0.0 -m "v2.0.0"
```

## Next Steps

- **[CI/CD Integration](ci-cd-integration.md)** — Automate release workflows in GitHub Actions
- **[End-to-End Workflow](end-to-end-workflow.md)** — Full example from init to production deployment

---

**Need help?** Check the [FAQ](../README.md#faq) or open an issue on [GitHub](https://github.com/cymertek/changelog/issues).
