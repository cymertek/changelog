# Documentation Index

Welcome to the changelog documentation. This index provides quick access to all guides, organized by topic and complexity level.

## Quick Start

| Guide | Description | Time to Read |
|-------|-------------|--------------|
| [Getting Started](getting-started.md) | Installation, initialization, first release workflow | 10 minutes |
| [Fragment Writing Guide](fragment-guide.md) | How to write fragments with proper YAML frontmatter and body sections | 15 minutes |

## Core Workflows

| Guide | Description | Time to Read |
|-------|-------------|--------------|
| [Release Workflow](release-workflow.md) | Versioning strategies (semver vs classic), release commands, best practices | 20 minutes |
| [Template Customization](template-customization.md) | HTML, LaTeX, and plain text templates with Go template syntax | 25 minutes |

## Automation & CI/CD

| Guide | Description | Time to Read |
|-------|-------------|--------------|
| [CI/CD Integration](ci-cd-integration.md) | GitHub Actions workflows, fragment validation, automated releases | 30 minutes |
| [End-to-End Workflow](end-to-end-workflow.md) | Complete example from initialization through production deployment | 45 minutes |

## Reference Documentation

| Guide | Description | Time to Read |
|-------|-------------|--------------|
| [Advanced Configuration](advanced-config.md) | All `config.yaml` options with examples and defaults | 20 minutes (reference) |

---

## Recommended Reading Order

### For New Users

1. **[Getting Started](getting-started.md)** — Install changelog and create your first release
2. **[Fragment Writing Guide](fragment-guide.md)** — Learn to write proper fragments
3. **[Release Workflow](release-workflow.md)** — Understand versioning and release strategies
4. **[End-to-End Workflow](end-to-end-workflow.md)** — Follow a complete example

### For CI/CD Integration

1. **[CI/CD Integration](ci-cd-integration.md)** — Set up automated pipelines
2. **[Template Customization](template-customization.md)** — Add custom HTML/LaTeX output
3. **[End-to-End Workflow](end-to-end-workflow.md)** — Deploy with full automation

### For Advanced Users

1. **[Advanced Configuration](advanced-config.md)** — Customize every aspect of changelog generation
2. **[Template Customization](template-customization.md)** — Write branded templates
3. **[CI/CD Integration](ci-cd-integration.md)** — Build production-grade pipelines

---

## Cross-References

These guides reference each other for related topics:

- **Fragment Writing Guide** → Release Workflow (when to add fragments)
- **Release Workflow** → CI/CD Integration (automating releases)
- **CI/CD Integration** → End-to-End Workflow (full automation example)
- **Template Customization** → Advanced Configuration (template settings in config.yaml)

---

## Need Help?

- **FAQ**: See the [README.md FAQ section](../README.md#faq) for common questions
- **Issues**: Open a GitHub issue at https://github.com/cymertek/changelog/issues
- **Examples**: Check `example_templates/` directory for ready-to-use templates

---

*Last updated: 2026-07-27 | Documentation version: v1.0.0*
