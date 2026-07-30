package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
)

// Fragment represents a parsed changelog fragment file.
type Fragment struct {
	Category           string
	Issue              string
	Date               string
	Author             string
	Title              string
	Body               string
	Filename           string
	Sections           map[string][]string
	ReleaseSummary     []ChangelogItem
	MajorChanges       []ChangelogItem
	MinorChanges       []ChangelogItem
	Bugfixes           []ChangelogItem
	BreakingChanges    []ChangelogItem
	DeprecatedFeatures []ChangelogItem
	RemovedFeatures    []ChangelogItem
	SecurityFixes      []ChangelogItem
	KnownIssues        []ChangelogItem
}

// ChangelogItem represents a single change entry in a changelog section.
type ChangelogItem struct {
	Text        string
	Meta        map[string]string
	Block       *CodeBlock
	Subsections []ChangelogItem
}

// CodeBlock represents a fenced code block within a changelog item.
type CodeBlock struct {
	Language string
	Content  string
}

func (item *ChangelogItem) String() string {
	if item != nil {
		return item.Text
	}
	return ""
}

// NewTextItem creates a ChangelogItem with the given text content.
func NewTextItem(text string) ChangelogItem { //nolint:unused // exported helper for external callers
	return ChangelogItem{Text: text}
}

// Parse reads a fragment file and returns a parsed Fragment.
func Parse(filename string) (*Fragment, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return &Fragment{Filename: filepath.Base(filename)}, nil
	}
	frag := &Fragment{Filename: filepath.Base(filename)}
	ext := filepath.Ext(filename)
	switch ext {
	case ".yml", ".yaml":
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err == nil {
			frag.Sections = make(map[string][]string)
			for k, v := range raw {
				frag.Sections[k] = toStringSlice(v)
			}
		}
	default:
		return frag, nil
	}
	frag.InitSlices()
	return frag, nil
}

// InitSlices initializes all typed slice fields to prevent nil pointer panics.
func (f *Fragment) InitSlices() {
	if f.Sections == nil {
		f.Sections = make(map[string][]string)
	}
	if f.ReleaseSummary == nil {
		f.ReleaseSummary = []ChangelogItem{}
	}
	if f.MajorChanges == nil {
		f.MajorChanges = []ChangelogItem{}
	}
	if f.MinorChanges == nil {
		f.MinorChanges = []ChangelogItem{}
	}
	if f.Bugfixes == nil {
		f.Bugfixes = []ChangelogItem{}
	}
	if f.BreakingChanges == nil {
		f.BreakingChanges = []ChangelogItem{}
	}
	if f.DeprecatedFeatures == nil {
		f.DeprecatedFeatures = []ChangelogItem{}
	}
	if f.RemovedFeatures == nil {
		f.RemovedFeatures = []ChangelogItem{}
	}
	if f.SecurityFixes == nil {
		f.SecurityFixes = []ChangelogItem{}
	}
	if f.KnownIssues == nil {
		f.KnownIssues = []ChangelogItem{}
	}
}

// Validate checks that the fragment has required fields.
func (f *Fragment) Validate() error {
	if f.Issue == "" {
		return fmt.Errorf("fragment %s missing issue identifier", f.Filename)
	}
	return nil
}

// WriteToYAML writes the fragment to a YAML file in the specified directory.
func (f *Fragment) WriteToYAML(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal fragment: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.yml", f.Issue, f.Category)
	path := filepath.Join(dir, filename)
	return os.WriteFile(path, data, 0o644)
}

func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []any:
		var strs []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				strs = append(strs, s)
			}
		}
		return strs
	case string:
		return []string{val}
	default:
		return nil
	}
}

// isValidSectionKey checks if a section key is recognized.
func isValidSectionKey(key string) bool { //nolint:unused // used by lint.go for fragment validation
	return slices.Contains(validSectionKeys, key)
}

// validSectionKeys lists all recognized fragment category keys.
var validSectionKeys = []string{
	"bugfixes", "minor_changes", "major_changes", "breaking_changes",
	"deprecated_features", "removed_features", "security_fixes", "known_issues",
}

// sectionDisplayNames maps internal keys to display titles shown in rendered changelogs.
var sectionDisplayNames = map[string]string{
	"bugfixes":            "Bug Fixes",
	"minor_changes":       "Minor Changes",
	"major_changes":       "Major Changes",
	"breaking_changes":    "Breaking Changes / Porting Guide",
	"deprecated_features": "Deprecated Features",
	"removed_features":    "Removed Features (previously deprecated)",
	"security_fixes":      "Security Fixes",
	"known_issues":        "Known Issues",
}

// sectionPriority defines render ordering for fragment sections (lower = earlier in output).
var sectionPriority = map[string]int{
	"release_summary":     0,
	"major_changes":       1,
	"minor_changes":       2,
	"bugfixes":            3,
	"breaking_changes":    4,
	"deprecated_features": 5,
	"removed_features":    6,
	"security_fixes":      7,
	"known_issues":        8,
}

// frontmatterParts holds the split YAML frontmatter and body from a fragment file.
type frontmatterParts struct {
	frontmatter string
	body        string
}
