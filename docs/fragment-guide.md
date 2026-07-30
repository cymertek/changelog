# Fragment Writing Guide

Fragments are the building blocks of your changelog. Each fragment describes a single change (bug fix, feature addition, breaking change) and gets rendered into the final `CHANGELOG.md` on release. This guide covers everything you need to know about writing fragments correctly.

## Fragment File Format

Fragment files use YAML with frontmatter metadata followed by body sections:

```yaml
---
category: bugfixes
issue: PG-001
date: "2026-07-27"
title: "Fixed null pointer crash in auth handler"
author: "Jane Doe"
---

bugfixes: |
    - Fixed null pointer crash in auth handler when processing empty payloads
```

### Frontmatter Fields

| Field      | Required | Type   | Description                                      | Example                          |
|------------|----------|--------|--------------------------------------------------|----------------------------------|
| `category` | Yes      | string | Section key matching `config.yaml` sections      | `"bugfixes"`, `"enhancement"`    |
| `issue`    | Yes      | string | Issue/PR number or unique identifier             | `"PG-001"`, `"GH-42"`, `"ABC-123"` |
| `date`     | No       | string | Release date in `YYYY-MM-DD` format              | `"2026-07-27"`                   |
| `author`   | No       | string | Author name (defaults to empty)                  | `"Jane Doe"`                     |
| `title`    | Yes      | string | One-line summary of the change                   | `"Fixed null pointer crash"`     |

If `date` is omitted, changelog uses the current date at release time.

### Body Sections

The body contains section-based content. Each key must match a section defined in `config.yaml`:

```yaml
bugfixes: |
    - Fixed null pointer crash in auth handler when processing empty payloads

major_changes: |
    - Added support for YAML fragments with frontmatter metadata
    - Improved performance by 40% in parallel processing mode

breaking_changes: |
    - Removed deprecated --legacy flag (use --modern instead)
```

#### Section Keys and Display Names

Section keys map to display titles via `config.yaml`:

```yaml
sections:
  bugfixes: "Bug Fixes"
  major_changes: "Major Changes"
  breaking_changes: "Breaking Changes / Porting Guide"
```

If a section key isn't in `config.yaml`, the key itself is used as the display title (e.g., `custom_section` → "custom_section").

## Writing Good Fragments

### Do's

✅ **Be specific about what changed**:

```yaml
bugfixes: |
    - Fixed null pointer crash when processing empty payloads in auth handler
```

❌ **Don't be vague**:

```yaml
bugfixes: |
    - Fixed a bug
```

✅ **Include context for breaking changes**:

```yaml
breaking_changes: |
    - Removed deprecated --legacy flag (use --modern instead)
      Migration: Replace all instances of --legacy with --modern in your scripts.
```

✅ **Reference issues and PRs**:

```yaml
issue: PG-001
title: "Fixed null pointer crash"
author: "Jane Doe"
```

### Don'ts

❌ **Don't include implementation details**:

```yaml
bugfixes: |
    - Changed line 42 in auth.go to check for nil before dereferencing
```

✅ **Do describe the user-facing impact**:

```yaml
bugfixes: |
    - Fixed null pointer crash when processing empty payloads
```

❌ **Don't use markdown formatting in fragments** (it gets rendered as-is):

```yaml
bugfixes: |
    - Fixed **null pointer** crash in `auth handler`
```

✅ **Do keep it plain text**:

```yaml
bugfixes: |
    - Fixed null pointer crash in auth handler
```

## Fragment Examples

### Bug Fix Fragment

```yaml
---
category: bugfixes
issue: PG-042
date: "2026-07-15"
title: "Fixed connection timeout on slow networks"
author: "Alex Chen"
---

bugfixes: |
    - Increased default connection timeout from 30s to 60s for slow network environments
```

### Enhancement Fragment

```yaml
---
category: enhancement
issue: PG-043
date: "2026-07-15"
title: "Added support for custom template variables"
author: "Jordan Lee"
---

enhancement: |
    - Added {{ .Config.Title }} variable to HTML templates for dynamic branding
```

### Breaking Change Fragment

```yaml
---
category: breaking_changes
issue: PG-044
date: "2026-07-15"
title: "Removed deprecated --legacy flag"
author: "Sam Patel"
---

breaking_changes: |
    - Removed deprecated --legacy flag (use --modern instead)
      Migration: Replace all instances of --legacy with --modern in your scripts.
```

### Multiple Sections in One Fragment

A single fragment can contribute to multiple sections:

```yaml
---
category: enhancement
issue: PG-045
date: "2026-07-15"
title: "Added parallel processing support with performance improvements"
author: "Taylor Kim"
---

enhancement: |
    - Added --parallel flag to enable multi-threaded fragment processing
major_changes: |
    - Performance improved by 40% in parallel mode for large collections
```

### Security Fix Fragment

```yaml
---
category: security_fixes
issue: PG-046
date: "2026-07-15"
title: "Patched CVE-2026-12345 in dependency handling"
author: "Security Team"
---

security_fixes: |
    - Patched CVE-2026-12345: Input validation bypass in YAML parser
      Advisory: https://github.com/advisories/GHSA-xxxx-yyyy-zzzz
```

### Deprecated Feature Fragment

```yaml
---
category: deprecated_features
issue: PG-047
date: "2026-07-15"
title: "Deprecated --old-format flag (removal in v2.0)"
author: "Release Manager"
---

deprecated_features: |
    - Deprecated --old-format flag (use --new-format instead)
      Removal: This flag will be removed in v2.0. Update your scripts before then.
```

## Advanced Fragment Features

### Multiline Content

Use YAML block scalars (`|`) for multiline content:

```yaml
bugfixes: |
    - Fixed null pointer crash when processing empty payloads
    - Resolved race condition in parallel fragment merging
    - Improved error messages for invalid YAML syntax
```

Or use explicit newlines with `\n`:

```yaml
bugfixes: "- Fixed null pointer crash\n- Resolved race condition"
```

### Lists of Items

Fragments support both string and list formats in the body:

**String format** (most common):

```yaml
bugfixes: |
    - Item 1
    - Item 2
    - Item 3
```

**List format**:

```yaml
bugfixes:
    - Item 1
    - Item 2
    - Item 3
```

Both produce identical output.

### Empty Sections

To mark a section as empty (e.g., no breaking changes), use an empty string or omit the section entirely:

```yaml
breaking_changes: ""
# OR just don't include it at all
```

changelog skips empty sections during rendering unless `always_refresh` is set to `"full"`.

## Fragment Naming Convention

Fragment filenames follow the pattern `{issue}-{category}.yml`:

- **PG-001-bugfixes.yml** — Issue PG-001 in bugfixes category
- **GH-42-enhancement.yml** — GitHub PR #42 in enhancement category
- **ABC-123-breaking_changes.yml** — Internal ticket ABC-123

You can customize the naming template via `config.yaml`:

```yaml
changelog_filename_template: '{issue}-{category}'
# Output: PG-001-bugfixes.yml
```

## Validating Fragments

Use the built-in linter to catch errors before release:

```bash
changelog lint changelogs/fragments/
```

Common validation errors:

- **Invalid category**: Category not defined in `config.yaml` sections
- **Missing required fields**: `category`, `issue`, or `title` missing from frontmatter
- **Malformed YAML**: Syntax errors in fragment file
- **Duplicate issue numbers**: Two fragments with the same `issue` value

## Testing Fragments Without Releasing

To preview how a fragment will render without committing to a release:

```bash
# Generate a dry-run changelog (fragments not archived)
changelog add --category bugfixes -i PG-999 -T "Test fragment"
changelog release --version v0.0.0-dryrun --no-archive
```

This creates `CHANGELOG.md` with the test fragment but doesn't archive it. Delete the test changelog when done:

```bash
rm CHANGELOG.md
```

## Common Pitfalls

### Forgetting to Add Fragments

**Problem**: You make changes but forget to add fragments, so nothing appears in the changelog.

**Solution**: Make fragment creation part of your PR workflow. Every PR that touches code should include a fragment.

### Using Wrong Section Keys

**Problem**: Fragment uses `bug_fixes` but config.yaml defines `bugfixes`.

**Solution**: Always use section keys exactly as defined in `config.yaml`. Check with:

```bash
changelog lint changelogs/fragments/
```

### Including Markdown Formatting

**Problem**: Fragments contain `**bold**` or `*italic*` which render literally in the output.

**Solution**: Keep fragment content plain text. changelog handles formatting during rendering.

### Duplicate Issue Numbers

**Problem**: Two fragments use the same `issue: PG-001`.

**Solution**: Each fragment must have a unique issue number. Use PR numbers, ticket IDs, or sequential integers.

---

## Next Steps

- **[Getting Started](getting-started.md)** — Learn how to initialize your project and run your first release
- **[Release Workflow](release-workflow.md)** — Understand versioning strategies and release automation
- **[CI/CD Integration](ci-cd-integration.md)** — Set up automated changelog generation in GitHub Actions

---

**Need help?** Check the [FAQ](../README.md#faq) or open an issue on [GitHub](https://github.com/cymertek/changelog/issues).
