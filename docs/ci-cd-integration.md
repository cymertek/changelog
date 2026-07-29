# CI/CD Integration Guide

Automate changelog generation, validation, and release workflows in your CI/CD pipeline. This guide covers GitHub Actions examples, pre-release hooks, fragment validation, and automated version bumping.

## Why Automate Changelogs?

Manual changelog management leads to:

- **Forgotten fragments** — Changes slip through the cracks
- **Inconsistent formatting** — Different authors use different styles
- **Release delays** — Changelog generation becomes a bottleneck
- **Human error** — Wrong version numbers, missing sections

Automation ensures every release has an accurate, complete changelog without manual intervention.

## GitHub Actions Workflow

### Basic Changelog Generation

Add this workflow to `.github/workflows/changelog.yml`:

```yaml
name: Generate Changelog

on:
  push:
    tags:
      - 'v*'

jobs:
  generate-changelog:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0  # Fetch all history for fragment scanning

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install changelog
        run: |
          go build -o changelog .
          sudo mv changelog /usr/local/bin/

      - name: Run release
        run: |
          VERSION=${GITHUB_REF#refs/tags/}
          changelog release --version "$VERSION"

      - name: Commit changelog
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add CHANGELOG.md changelogs/fragments/archive/
          git commit -m "chore: update changelog for ${{ github.ref_name }}" || echo "No changes to commit"
          git push
```

### Fragment Validation on PR

Fail the build if a code change isn't accompanied by a fragment:

```yaml
name: Validate Fragments

on:
  pull_request:
    branches: [main]

jobs:
  validate-fragments:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install changelog
        run: |
          go build -o changelog .
          sudo mv changelog /usr/local/bin/

      - name: Check for fragment changes
        id: fragment-check
        run: |
          if git diff --name-only ${{ github.event.pull_request.base.sha }}...${{ github.event.pull_request.head.sha }} | grep -q 'changelogs/fragments/'; then
            echo "has_fragments=true" >> $GITHUB_OUTPUT
          else
            echo "has_fragments=false" >> $GITHUB_OUTPUT
          fi

      - name: Fail if no fragments added
        if: steps.fragment-check.outputs.has_fragments == 'false' && github.event.pull_request.body != ''
        run: |
          echo "::error::No changelog fragment found in this PR."
          echo "Add a fragment with: changelog add --category <section> -i <issue> -T '<title>'"
          exit 1
```

### Automated Release with Semver Bumping

Combine changelog generation with automatic version bumping:

```yaml
name: Automated Release

on:
  push:
    branches: [main]

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write  # Required for creating releases
      issues: write    # Required for creating discussions

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install changelog
        run: |
          go build -o changelog .
          sudo mv changelog /usr/local/bin/

      - name: Determine next version
        id: version
        run: |
          # Read current version from CHANGELOG.md or tag
          CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
          echo "current=$CURRENT_VERSION" >> $GITHUB_OUTPUT

          # Parse semver and increment patch
          IFS='.' read -r major minor patch <<< "${CURRENT_VERSION#v}"
          NEXT_PATCH=$((patch + 1))
          echo "next=v${major}.${minor}.${NEXT_PATCH}" >> $GITHUB_OUTPUT

      - name: Generate changelog
        run: |
          changelog release --version ${{ steps.version.outputs.next }}

      - name: Create Git tag
        run: |
          git config user.name "Release Bot"
          git config user.email "release-bot@example.com"
          git add CHANGELOG.md
          git commit -m "Release ${{ steps.version.outputs.next }}" || echo "No changes to commit"
          git push origin main
          git tag -a "${{ steps.version.outputs.next }}" -m "${{ steps.version.outputs.next }}"
          git push origin "${{ steps.version.outputs.next }}"

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: ${{ steps.version.outputs.next }}
          name: "Release ${{ steps.version.outputs.next }}"
          body_path: CHANGELOG.md
          draft: false
          prerelease: false
```

## Pre-Release Hooks

### Git Hook: Require Fragment on Commit

Add a pre-commit hook to enforce fragment creation:

```bash
#!/bin/bash
# .git/hooks/pre-commit

CHANGED_FILES=$(git diff --cached --name-only)

if echo "$CHANGED_FILES" | grep -qE '\.(go|py|yaml|yml)$'; then
    if ! echo "$CHANGED_FILES" | grep -q '^changelogs/fragments/'; then
        echo "Error: Code changes require a changelog fragment."
        echo "Add one with: changelog add --category <section> -i <issue> -T '<title>'"
        exit 1
    fi
fi

exit 0
```

Make it executable:

```bash
chmod +x .git/hooks/pre-commit
```

### CI Check: Validate All Fragments

Run fragment validation before release:

```yaml
name: Validate Fragments

on:
  workflow_dispatch:  # Manual trigger for releases

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install changelog
        run: |
          go build -o changelog .
          sudo mv changelog /usr/local/bin/

      - name: Lint fragments
        run: |
          changelog lint changelogs/fragments/
```

## Fragment Validation in CI

### Syntax Check

Ensure all fragment files are valid YAML:

```yaml
- name: Validate YAML syntax
  run: |
    for file in changelogs/fragments/*.yml; do
      python3 -c "import yaml; yaml.safe_load(open('$file'))" || {
        echo "::error::Invalid YAML in $file"
        exit 1
      }
    done
```

### Required Fields Check

Verify fragments have required frontmatter fields:

```yaml
- name: Check required fragment fields
  run: |
    for file in changelogs/fragments/*.yml; do
      if ! grep -q '^category:' "$file"; then
        echo "::error::$file missing 'category' field"
        exit 1
      fi
      if ! grep -q '^issue:' "$file"; then
        echo "::error::$file missing 'issue' field"
        exit 1
      fi
      if ! grep -q '^title:' "$file"; then
        echo "::error::$file missing 'title' field"
        exit 1
      fi
    done
```

### Section Key Validation

Ensure fragment section keys match `config.yaml`:

```yaml
- name: Validate section keys
  run: |
    CONFIG_SECTIONS=$(grep -A 100 '^sections:' changelogs/config.yaml | grep '^\s*[a-z_]*:' | sed 's/.*: *//' | tr '\n' '|' | sed 's/|$//')
    for file in changelogs/fragments/*.yml; do
      FRAG_SECTIONS=$(grep -E '^[a-z_]+:' "$file" | head -1 | sed 's/:.*//')
      if ! echo "$CONFIG_SECTIONS" | grep -q "$FRAG_SECTIONS"; then
        echo "::warning::$file uses section '$FRAG_SECTIONS' not in config.yaml"
      fi
    done
```

## Automated Version Detection

### From Git Tags

Detect the next version based on existing tags:

```bash
# Get latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

# Parse semver
IFS='.' read -r major minor patch <<< "${LATEST_TAG#v}"

# Increment patch version
NEXT_VERSION="v${major}.${minor}.$((patch + 1))"

echo "Next version: $NEXT_VERSION"
```

### From CHANGELOG.md

Extract version from the latest changelog entry:

```bash
VERSION=$(head -n 5 CHANGELOG.md | grep -oP 'v\d+\.\d+\.\d+' | tail -1)
```

### From config.yaml

Use the `version_pattern` to validate detected versions:

```yaml
# config.yaml
version_pattern: '^\d+\.\d+\.\d+$'
use_semantic_versioning: true
```

## Release Automation Strategies

### Strategy 1: Tag-Triggered Releases

Automate release when a maintainer pushes a tag:

```yaml
on:
  push:
    tags:
      - 'v*'
```

**Pros**: Simple, explicit control  
**Cons**: Requires manual tag creation

### Strategy 2: Merge-Triggered Releases

Automate release when PRs are merged to main:

```yaml
on:
  pull_request:
    types: [closed]
    branches: [main]
```

**Pros**: Fully automated  
**Cons**: Risk of unintended releases

### Strategy 3: Scheduled Releases

Run release on a schedule (e.g., weekly):

```yaml
on:
  schedule:
    - cron: '0 9 * * 1'  # Every Monday at 9 AM UTC
```

**Pros**: Predictable release cadence  
**Cons**: May release empty changelogs

### Strategy 4: Manual Workflow Dispatch

Trigger releases manually via GitHub Actions UI:

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Release version (e.g., v1.2.3)'
        required: true
```

**Pros**: Full control, auditable  
**Cons**: Requires manual intervention

## Best Practices

### 1. Use Semantic Versioning

Adopt semver for clear release semantics:

```yaml
use_semantic_versioning: true
version_pattern: '^\d+\.\d+\.\d+$'
```

### 2. Enforce Fragment Requirements

Require fragments for every code change via CI checks or git hooks.

### 3. Validate Before Release

Run `changelog lint` in CI before allowing releases:

```yaml
- name: Lint fragments
  run: changelog lint changelogs/fragments/
```

### 4. Archive Released Fragments

Keep archives for audit trails:

```yaml
keep_fragments: false  # false = archive, true = delete
archive_path_template: "changelogs/fragments/archive/{date}"
```

### 5. Use Dry Runs for Testing

Test release workflows with `--dry-run` before committing:

```bash
changelog release --version v1.0.0 --dry-run
```

### 6. Pin Go Versions in CI

Use specific Go versions to avoid build inconsistencies:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version: '1.23.4'  # Pin exact version
```

## Troubleshooting

### Fragment Validation Fails in CI

**Error**: `changelog lint` reports invalid fragments

**Solution**: Run lint locally first:

```bash
changelog lint changelogs/fragments/
```

Check for:

- Missing required fields (`category`, `issue`, `title`)
- Invalid YAML syntax
- Section keys not in `config.yaml`

### Release Fails with "Invalid Version"

**Error**: `Error: Invalid version "1.0" — must match pattern ^\d+\.\d+\.\d+$`

**Solution**: Ensure version string matches `version_pattern` in `config.yaml`:

```bash
# ✅ Valid for semver pattern
changelog release --version v1.2.3

# ❌ Invalid (missing patch version)
changelog release --version v1.2
```

### Templates Not Found

**Error**: `template not found: changelogs/template.html`

**Solution**: Verify template path in `config.yaml` and check file exists:

```bash
ls -la changelogs/template.html
# Or place it in project root:
ls -la template.html
```

### Duplicate Release Error

**Error**: `Error: Version v1.0.0 already released`

**Solution**: Delete or rename the old release metadata:

```bash
rm changelogs/.changes.yaml
changelog release --version v1.0.0
```

## Next Steps

- **[End-to-End Workflow](end-to-end-workflow.md)** — Full example from init to production deployment
- **[Advanced Configuration](advanced-config.md)** — All config.yaml options explained in detail

---

**Need help?** Check the [FAQ](../README.md#faq) or open an issue on [GitHub](https://github.com/cymertek/changelog/issues).
