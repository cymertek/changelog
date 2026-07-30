package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// MergeCollect reads all fragment files from multiple directories and merges them.
// Supports both YAML (.yml/.yaml) fragments (new) and markdown frontmatter (.md) fragments (legacy).
// Fragments are deduplicated by filename basename. If the same filename exists in
// both the main dir and a notes dir, the notes dir version wins — matching upstream's
// behavior where later directories override earlier ones.
func MergeCollect(mainDir string, extraDirs []string) ([]*Fragment, error) {
	seen := make(map[string]*Fragment) // dedup by basename only; later dirs override

	collectFrom := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read directory %s: %w", dir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".md")) {
				continue
			}

			// Skip hidden files (e.g., .template.md, .gitkeep).
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			// Skip known config files that look like YAML fragments.
			base := entry.Name()
			switch base {
			case "changelog.yaml", "changelog.yml", "CHANGELOG.md":
				continue // not a fragment
			}

			path := filepath.Join(dir, entry.Name())
			frag, err := Parse(path)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			seen[entry.Name()] = frag // later dirs overwrite earlier ones with same basename
		}
		return nil
	}

	if err := collectFrom(mainDir); err != nil {
		return nil, err
	}

	// Also look inside common fragment subdirectories (fragments/, changelog.d/) in case
	// the main dir contains config files like changelog.yaml that match the extension filter.
	for _, subdir := range []string{"fragments", "changelog.d"} {
		subPath := filepath.Join(mainDir, subdir)
		if err := collectFrom(subPath); err != nil {
			continue // silently skip missing subdirs
		}
	}

	for _, dir := range extraDirs {
		_ = collectFrom(dir) // silently skip missing dirs to support optional notesdir
	}

	// Build result slice from dedup map — later dirs' files override earlier ones.
	var allFragments []*Fragment
	for _, frag := range seen {
		allFragments = append(allFragments, frag)
	}
	sortFragments(allFragments)
	return allFragments, nil
}

// Collect reads all fragment files from the given directory and parses them.
func Collect(dir string) ([]*Fragment, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var fragments []*Fragment
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".md")) {
			continue
		}

		// Skip hidden files (e.g., .template.md, .gitkeep).
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		frag, err := Parse(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		fragments = append(fragments, frag)
	}

	sortFragments(fragments)
	return fragments, nil
}

// sectionDef holds metadata for one changelog section: its key, display name, and typed getter.
type sectionDef struct {
	key     string
	display string
	get     func(*Fragment) []ChangelogItem
}

var sectionDefs = []sectionDef{
	{"release_summary", "Release Summary", func(f *Fragment) []ChangelogItem { return f.ReleaseSummary }},
	{"major_changes", "Major Changes", func(f *Fragment) []ChangelogItem { return f.MajorChanges }},
	{"minor_changes", "Minor Changes", func(f *Fragment) []ChangelogItem { return f.MinorChanges }},
	{"bugfixes", "Bugfixes", func(f *Fragment) []ChangelogItem { return f.Bugfixes }},
	{"breaking_changes", "Breaking Changes / Porting Guide", func(f *Fragment) []ChangelogItem { return f.BreakingChanges }},
	{"deprecated_features", "Deprecated Features", func(f *Fragment) []ChangelogItem { return f.DeprecatedFeatures }},
	{"removed_features", "Removed Features (previously deprecated)", func(f *Fragment) []ChangelogItem { return f.RemovedFeatures }},
	{"security_fixes", "Security Fixes", func(f *Fragment) []ChangelogItem { return f.SecurityFixes }},
	{"known_issues", "Known Issues", func(f *Fragment) []ChangelogItem { return f.KnownIssues }},
}

// hasTypedContent returns true if any typed slice field on the fragment is non-empty.
func hasTypedContent(f *Fragment) bool {
	for _, def := range sectionDefs {
		items := def.get(f)
		for _, item := range items {
			if !isEmptyItem(item) {
				return true
			}
		}
	}
	return false
}

// isEmptyItem returns true if the ChangelogItem has no renderable content.
func isEmptyItem(item ChangelogItem) bool {
	return item.Text == "" && len(item.Subsections) == 0 && item.Meta == nil && (item.Block == nil || item.Block.Content == "")
}

var anchorIDRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// toKebabCase converts a section key like "breaking_changes" to "breaking-changes".
func toKebabCase(s string) string {
	return strings.ToLower(anchorIDRe.ReplaceAllString(s, "-"))
}

// renderChangelogItem formats a single ChangelogItem for markdown output.
// Uses `*` bullet markers matching abc's markdown output style.
func renderChangelogItem(item ChangelogItem, indent string) []string {
	var lines []string

	if len(item.Subsections) > 0 {
		for _, sub := range item.Subsections {
			lines = append(lines, indent+"* "+sub.Text)
			for _, inner := range sub.Subsections {
				lines = append(lines, indent+"  - "+inner.Text)
			}
		}
		return lines
	}

	if len(item.Meta) > 0 {
		metaLines := []string{}
		var keys []string
		for k := range item.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys) // deterministic output order.
		for _, k := range keys {
			metaLines = append(metaLines, fmt.Sprintf("%s=%s", k, item.Meta[k]))
		}
		if item.Text != "" {
			lines = append(lines, indent+"* **"+strings.Join(metaLines, ", ")+"**")
			lines = append(lines, indent+"  "+item.Text)
		} else {
			lines = append(lines, indent+"* "+strings.Join(metaLines, "; "))
		}
		return lines
	}

	if item.Block != nil && item.Block.Content != "" {
		lines = append(lines, indent+"* "+item.Text)
		if item.Block.Language == "" {
			item.Block.Language = "text"
		}
		lines = append(lines, fmt.Sprintf("%s  ```%s", indent, item.Block.Language))
		for l := range strings.SplitSeq(strings.TrimSpace(item.Block.Content), "\n") {
			lines = append(lines, fmt.Sprintf("%s  %s", indent, l))
		}
		lines = append(lines, indent+"  ```")
		return lines
	}

	if item.Text != "" {
		lines = append(lines, indent+"* "+item.Text)
	}

	return lines
}

// versionToAnchorID converts a version string like "v1.0.0" to an anchor ID like "v1-0-0".
func versionToAnchorID(version string) string {
	// Strip 'v' prefix for the ID, replace dots with hyphens.
	clean := strings.TrimPrefix(version, "v")
	return "v" + strings.ReplaceAll(clean, ".", "-")
}

// renderTopics builds the Topics/TOC section that abc renders before version headers.
// It lists each rendered section with a nested anchor link under the version TOC entry.
func renderTopics(sb *strings.Builder, version string, fragments []*Fragment) {
	anchorID := versionToAnchorID(version)

	// Determine which sections have content.
	var activeSections []sectionDef
	for _, def := range sectionDefs {
		var hasContent bool
		for _, f := range fragments {
			items := def.get(f)
			for _, item := range items {
				if !isEmptyItem(item) {
					hasContent = true
					break
				}
			}
			if hasContent {
				break
			}
		}
		if hasContent {
			activeSections = append(activeSections, def)
		}
	}

	if len(activeSections) == 0 {
		return // no sections to render TOC for
	}

	fmt.Fprintf(sb, "**Topics**\n\n")
	fmt.Fprintf(sb, "- <a id=\"%s\">%s</a>\n", anchorID, version)

	for _, sec := range activeSections {
		secID := "#" + toKebabCase(sec.key)
		fmt.Fprintf(sb, "  - <a href=\"%s\">%s</a>\n", secID, sec.display)
	}
}

// renderSectionAnchor emits an `<a id>` anchor before a section heading.
func renderSectionAnchor(sb *strings.Builder, sectionKey string) {
	fmt.Fprintf(sb, "<a id=\"%s\"></a>\n\n", toKebabCase(sectionKey))
}

// loadAncestorChanges reads the .changes.yaml file and extracts prior version change data.
// Returns a map of section key -> []string for ancestor entries not in current fragments.
func loadAncestorChanges(changelogDir string) (map[string][]string, error) {
	changesPath := filepath.Join(changelogDir, ".changes.yaml")
	data, err := os.ReadFile(changesPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", changesPath, err)
	}

	// abc-compatible nested format: releases -> version -> changes.
	var wrapper struct {
		Releases map[string]struct {
			Changes map[string][]string `yaml:"changes"`
		} `yaml:"releases"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("parse %s: %w", changesPath, err)
	}

	// Merge all prior release changes into a single map.
	result := make(map[string][]string)
	for ver, entry := range wrapper.Releases {
		if entry.Changes == nil {
			continue
		}
		for key, items := range entry.Changes {
			result[key] = append(result[key], items...)
		}
		_ = ver // version not needed for ancestor aggregation
	}

	return result, nil
}

// hasAncestorContent returns true if the ancestor map has any content to render.
func hasAncestorContent(ancestor map[string][]string) bool {
	for _, items := range ancestor {
		if len(items) > 0 {
			return true
		}
	}
	return false
}

// Render produces a CHANGELOG.md string from the given fragments using abc-compatible markdown format.
// The version is rendered with escaped dots in headings/anchors (e.g., ## v1\.0\.0).
func Render(baseDir string, fragments []*Fragment, version, _ string) (string, error) {
	var sb strings.Builder

	// Banner header — setext H1 matching abc.
	title := fragmentsToTitleFromDir(fragments, baseDir)
	fmt.Fprintf(&sb, "%s Release Notes\n", title)
	fmt.Fprintf(&sb, "%s\n\n", strings.Repeat("=", len(title)+len(" Release Notes")))

	// Topics / TOC section with anchor links.
	renderTopics(&sb, version, fragments)

	// Version header — [version] format matching abc's markdown renderer.
	anchorID := versionToAnchorID(version)
	fmt.Fprintf(&sb, "<a id=\"%s\"></a>\n\n", anchorID)
	fmt.Fprintf(&sb, "## [%s]\n\n", version)

	renderedSections := make(map[string]bool) // track which section headers we've emitted

	for _, def := range sectionDefs {
		var allItems []ChangelogItem

		for _, f := range fragments {
			items := def.get(f)
			allItems = append(allItems, items...)
		}

		if len(allItems) == 0 {
			continue // no content for this section across any fragment.
		}

		// Filter out empty ChangelogItem{} values that can appear from empty YAML list entries.
		var filtered []ChangelogItem
		for _, item := range allItems {
			if isEmptyItem(item) {
				continue
			}
			filtered = append(filtered, item)
		}
		allItems = filtered

		if len(allItems) == 0 {
			continue // all items were empty.
		}

		// Section anchor before heading.
		renderSectionAnchor(&sb, def.key)

		fmt.Fprintf(&sb, "### %s\n\n", def.display)
		for _, item := range allItems {
			lines := renderChangelogItem(item, "")
			for _, line := range lines {
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
		}
		renderedSections[def.key] = true
	}

	// Handle legacy markdown fragments that have no typed fields but do have a body.
	for _, f := range fragments {
		if hasTypedContent(f) || (f.Category == "" && f.Body == "") {
			continue // skip fragments with typed content or no body.
		}
		catKey := f.Category
		displayName, ok := sectionDisplayNames[catKey]
		if !ok {
			continue
		}

		text := strings.TrimSpace(f.Body)
		if text == "" {
			continue
		}

		// Section anchor before heading (if not already rendered).
		if !renderedSections[catKey] {
			renderSectionAnchor(&sb, catKey)
			fmt.Fprintf(&sb, "### %s\n\n", displayName)
		} else {
			fmt.Fprintf(&sb, "\n%s\n\n", displayName)
		}
		sb.WriteString("* ")
		sb.WriteString(text)
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}

// RenderFull produces a complete multi-version CHANGELOG.md matching abc's generate output.
// It reads prior versions from .changes.yaml and renders them in reverse-chronological order,
// followed by the current version's fragments (if any).
func RenderFull(baseDir string, fragments []*Fragment, version, _ string) (string, error) {
	var sb strings.Builder

	// Banner header — setext H1 matching abc.
	title := fragmentsToTitleFromDir(fragments, baseDir)
	fmt.Fprintf(&sb, "%s Release Notes\n", title)
	fmt.Fprintf(&sb, "%s\n\n", strings.Repeat("=", len(title)+len(" Release Notes")))

	// Load prior versions from .changes.yaml for ancestor changelog rendering.
	ancestorChanges, _ := loadAncestorChanges(baseDir) // ignore error if no metadata yet

	// Read existing .changes.yaml to get prior release entries.
	changesPath := filepath.Join(baseDir, ".changes.yaml")

	type verInfo struct {
		Date      string
		Fragments []string
	}
	var allVersions map[string]verInfo

	if data, err := os.ReadFile(changesPath); err == nil && len(data) > 0 {
		type PriorRelease struct {
			Changes   map[string][]string `yaml:"changes"`
			Date      string              `yaml:"release_date"`
			Fragments []string            `yaml:"fragments,omitempty"`
		}
		var wrapper struct {
			Releases map[string]PriorRelease `yaml:"releases"`
		}
		if yaml.Unmarshal(data, &wrapper) == nil {
			allVersions = make(map[string]verInfo)
			for ver, entry := range wrapper.Releases {
				allVersions[ver] = verInfo{Date: entry.Date, Fragments: entry.Fragments}
			}
		}
	}

	// Sort versions descending (newest first).
	var versionKeys []string
	for v := range allVersions {
		if v != version { // skip current version
			versionKeys = append(versionKeys, v)
		}
	}
	sort.SliceStable(versionKeys, func(i, j int) bool {
		aV, aErr := ParseSemver(versionKeys[i])
		bV, bErr := ParseSemver(versionKeys[j])
		if aErr != nil || bErr != nil {
			return versionKeys[i] > versionKeys[j] // fallback to lexicographic
		}
		if aV.Major != bV.Major {
			return aV.Major > bV.Major
		}
		if aV.Minor != bV.Minor {
			return aV.Minor > bV.Minor
		}
		return aV.Patch > bV.Patch
	})

	// Render prior versions (ancestor changelog).
	for _, v := range versionKeys {
		vInfo := allVersions[v]

		fmt.Fprintf(&sb, "**Topics**\n\n")
		fmt.Fprintf(&sb, "- <a id=\"%s\">%s</a>\n", versionToAnchorID(v), v)

		fmt.Fprintf(&sb, "<a id=\"%s\"></a>\n\n", versionToAnchorID(v))
		fmt.Fprintf(&sb, "## [%s]\n\n", v)

		if vInfo.Date != "" {
			fmt.Fprintf(&sb, "* Release Date: %s\n\n", vInfo.Date)
		}
	}

	// If there are ancestor changes to render.
	config := loadConfigForDir(baseDir)
	if config.MentionAncestor && hasAncestorContent(ancestorChanges) {
		fmt.Fprintf(&sb, "<a id=\"ancestor-changes\"></a>\n\n")
		fmt.Fprintf(&sb, "### Ancestor Changes\n\n")

		for key, items := range ancestorChanges {
			displayName := getSectionDisplayName(key)
			if displayName == "" {
				continue
			}
			fmt.Fprintf(&sb, "**%s**\n", displayName)
			fmt.Fprintf(&sb, "\n")
			for _, item := range items {
				fmt.Fprintf(&sb, "* %s\n", item)
			}
		}
		fmt.Fprintf(&sb, "\n")
	}

	// Render current version.
	if len(fragments) > 0 {
		renderTopics(&sb, version, fragments)

		fmt.Fprintf(&sb, "<a id=\"%s\"></a>\n\n", versionToAnchorID(version))
		fmt.Fprintf(&sb, "## [%s]\n\n", version)

		for _, def := range sectionDefs {
			var allItems []ChangelogItem
			for _, f := range fragments {
				items := def.get(f)
				allItems = append(allItems, items...)
			}

			if len(allItems) == 0 {
				continue
			}

			// Filter out empty ChangelogItem{} values.
			var filtered []ChangelogItem
			for _, item := range allItems {
				if isEmptyItem(item) {
					continue
				}
				filtered = append(filtered, item)
			}
			allItems = filtered

			if len(allItems) == 0 {
				continue
			}

			renderSectionAnchor(&sb, def.key)
			fmt.Fprintf(&sb, "### %s\n\n", def.display)
			for _, item := range allItems {
				lines := renderChangelogItem(item, "")
				for _, line := range lines {
					sb.WriteString(line)
					sb.WriteByte('\n')
				}
			}
		}

		// Plugin doc injection if configured.
		if config.NewPluginsAfterName != "" {
			var pluginNames []string
			for _, f := range fragments {
				// Collect any plugins/modules mentioned in fragment titles or categories.
				if f.Title != "" && strings.Contains(strings.ToLower(f.Category), "plugin") {
					pluginNames = append(pluginNames, f.Title)
				}
				// Also scan for module names in bugfixes/major_changes items.
				for _, item := range f.MajorChanges {
					if item.Text != "" && strings.Contains(strings.ToLower(item.Text), "module") ||
						strings.Contains(strings.ToLower(item.Text), "plugin") {
						pluginNames = append(pluginNames, item.Text)
					}
				}
			}
			if len(pluginNames) > 0 {
				fmt.Fprintf(&sb, "\n### %s\n\n", config.NewPluginsAfterName)
				for _, p := range pluginNames {
					fmt.Fprintf(&sb, "* %s\n", p)
				}
			}
		}
	}

	return sb.String(), nil
}

// getSectionDisplayName returns the human-readable display name for a section key.
func getSectionDisplayName(key string) string {
	for _, def := range sectionDefs {
		if def.key == key {
			return def.display
		}
	}
	return ""
}

// loadConfigForDir reads changelog config from the given directory.
func loadConfigForDir(dir string) Config {
	var cfg Config
	for _, name := range []string{"config.yaml"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if yaml.Unmarshal(data, &cfg) == nil && cfg.Title != "" {
			cfg.Defaults()
			return cfg
		}
	}
	return cfg
}

// fragmentsToTitleFromDir extracts the project title from config or fragment filenames.
func fragmentsToTitleFromDir(fragments []*Fragment, baseDir string) string {
	// Try to read title from config.yaml in common parent directories.
	for _, candidate := range []string{"config.yaml", "../config.yaml", ".changelogs/config.yaml"} {
		if data, err := os.ReadFile(filepath.Join(baseDir, candidate)); err == nil {
			var cfg struct {
				Title string `yaml:"title"`
			}
			if yaml.Unmarshal(data, &cfg) == nil && cfg.Title != "" {
				return cfg.Title
			}
		}
	}

	for _, f := range fragments {
		if f.Filename != "" && !strings.HasSuffix(f.Filename, ".yml") && !strings.HasSuffix(f.Filename, ".yaml") && !strings.HasSuffix(f.Filename, ".md") {
			return strings.TrimPrefix(filepath.Base(f.Filename), "v")
		}
	}

	for _, f := range fragments {
		if f.Title != "" {
			return f.Title
		}
	}

	// Default fallback.
	return "Project"
}

// sortFragments sorts fragments by section priority.
func sortFragments(fragments []*Fragment) {
	sort.SliceStable(fragments, func(i, j int) bool {
		pi := getSectionPriority(fragments[i])
		pj := getSectionPriority(fragments[j])
		if pi != pj {
			return pi < pj
		}
		return fragments[i].Issue < fragments[j].Issue
	})
}

// getSectionPriority returns the priority for a fragment's primary section.
func getSectionPriority(f *Fragment) int {
	// Prefer YAML sections over legacy category.
	for key := range f.Sections {
		if p, ok := sectionPriority[key]; ok {
			return p
		}
	}
	if p, ok := sectionPriority[f.Category]; ok {
		return p
	}
	// Legacy categories.
	if p, ok := categoryPriority[f.Category]; ok {
		return p
	}
	return 99 // unknown sections sort last
}

// sortFragmentsDesc sorts fragments in reverse chronological order for log display:
// newest date first, then by issue number descending. Within the same date and issue,
// section priority is preserved (breaking before enhancement). This ordering mirrors
// git-log-style output where most recent commits appear at the top.
func sortFragmentsDesc(fragments []*Fragment) {
	sort.SliceStable(fragments, func(i, j int) bool {
		// Primary: date descending (newest first) — "2006-01-02" sorts lexicographically correctly.
		if fragments[i].Date != fragments[j].Date {
			return fragments[i].Date > fragments[j].Date
		}
		// Secondary: issue number descending for same date.
		if fragments[i].Issue != fragments[j].Issue {
			return fragments[i].Issue > fragments[j].Issue
		}
		// Tertiary: section priority ascending (breaking before note).
		pi := getSectionPriority(fragments[i])
		pj := getSectionPriority(fragments[j])
		if pi != pj {
			return pi < pj
		}
		return fragments[i].Issue > fragments[j].Issue // tiebreaker: higher issue first.
	})
}

// CleanupFragments removes or archives processed fragment files from the directory.
func CleanupFragments(dir string, keep bool) error {
	if keep {
		return nil // nothing to do
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read fragments dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)

		if entry.IsDir() {
			// Recurse into subdirectories (e.g., .changelogs/fragments/).
			if err := CleanupFragments(path, false); err != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup %s: %v\n", path, err)
			}
			continue
		}

		if !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".md")) {
			continue
		}

		if err := os.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove fragment %s: %v\n", path, err)
		} else {
			fmt.Printf("Removed fragment: %s\n", name)
		}
	}

	return nil
}
