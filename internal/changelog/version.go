package changelog

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SemVer represents a parsed semantic version: major.minor.patch[prerelease].
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string // optional: "-alpha", "-rc1", etc. (without leading dash)
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-(.+))?$`)

// ParseSemver parses "1.2.3" or "v1.2.3" into a SemVer struct.
func ParseSemver(s string) (*SemVer, error) {
	s = strings.TrimSpace(s)
	m := semverRe.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf("invalid semantic version %q: expected format vX.Y.Z", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])

	v := &SemVer{
		Major: major, Minor: minor, Patch: patch,
	}
	if m[4] != "" {
		v.Prerelease = m[4] // already without leading dash
	}
	return v, nil
}

// String returns the version in bare format (no 'v' prefix): "X.Y.Z".
func (v *SemVer) String() string {
	if v.Prerelease != "" {
		return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Prerelease)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// PrefixedString returns the version with a 'v' prefix: "vX.Y.Z".
func (v *SemVer) PrefixedString() string {
	return "v" + v.String()
}

// BumpLevel extracts "major", "minor", or "patch" from a label like "bump-major".
func BumpLevel(label string) (string, error) {
	level := strings.TrimPrefix(label, "bump-")
	switch level {
	case "major", "minor", "patch":
		return level, nil
	default:
		if !strings.HasPrefix(label, "bump-") {
			return "", fmt.Errorf("invalid bump label %q: must start with 'bump-' (e.g. bump-minor)", label)
		}
		return "", fmt.Errorf("unknown bump level %q in label %q: expected bump-major, bump-minor, or bump-patch", level, label)
	}
}

// BumpLevelWithRC checks for rc-* style labels (e.g., "bump-rc-minor") and returns
// the bump kind ("major"/"minor"/"patch"), whether it's an RC, and any error.
func BumpLevelWithRC(label string) (level string, isRc bool, err error) {
	label = strings.TrimPrefix(label, "bump-")
	switch label {
	case "rc-major":
		return "major", true, nil
	case "rc-minor":
		return "minor", true, nil
	case "rc-patch":
		return "patch", true, nil
	case "rc":
		// Bare "rc" — default to rc-patch if no RC is in play.
		return "patch", true, nil
	case "major", "minor", "patch":
		return label, false, nil
	default:
		if strings.HasPrefix(label, "bump-") {
			return "", false, fmt.Errorf("invalid bump label %q: must start with 'bump-' (e.g. bump-minor)", label)
		}
		if !strings.HasPrefix(label, "rc-") && label != "rc" {
			return "", false, fmt.Errorf("unknown bump level %q in label %q", label, label)
		}
		level = strings.TrimPrefix(label, "rc-")
		switch level {
		case "major", "minor", "patch":
			return level, true, nil
		default:
			return "", false, fmt.Errorf("unknown bump level %q in rc label %q: expected rc-major, rc-minor, or rc-patch", level, label)
		}
	}
}

// Release strips the prerelease suffix (e.g., "0.3.0-rc.1" → "0.3.0").
func (v *SemVer) Release() *SemVer {
	return &SemVer{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
}

// BumpRC returns a new SemVer after bumping to an RC version.
// If level is empty ("rc" bare), only the RC counter increments without changing version numbers.
func (v *SemVer) BumpRC(level string, latestTag string) (*SemVer, error) {
	var base *SemVer

	if level != "" {
		bumped, err := v.Bump(level)
		if err != nil {
			return nil, err
		}
		base = bumped
	} else {
		// Bare "rc" — keep version the same.
		base = &SemVer{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
	}

	// If current version is already an RC, increment the rc counter.
	if v.Prerelease != "" && strings.HasPrefix(v.Prerelease, "rc.") {
		rcNum, err := strconv.Atoi(strings.TrimPrefix(v.Prerelease, "rc."))
		if err == nil {
			return &SemVer{Major: base.Major, Minor: base.Minor, Patch: base.Patch, Prerelease: fmt.Sprintf("rc.%d", rcNum+1)}, nil
		}
	}

	// Check if there's an existing RC tag for this version prefix (e.g., v0.3.0-rc.N).
	if latestTag != "" && strings.HasPrefix(latestTag, "v") {
		prefix := base.String() // e.g., "0.3.0"
		rcTags := findRcTags(prefix)
		if len(rcTags) > 0 {
			// Find the highest rc number and increment it.
			maxRcNum := parseRcNumber(rcTags[len(rcTags)-1])
			return &SemVer{Major: base.Major, Minor: base.Minor, Patch: base.Patch, Prerelease: fmt.Sprintf("rc.%d", maxRcNum+1)}, nil
		}
	}

	// First RC for this version.
	return &SemVer{Major: base.Major, Minor: base.Minor, Patch: base.Patch, Prerelease: "rc.1"}, nil
}

// findRcTags finds tags matching the pattern vX.Y.Z-rc.N for a given version prefix and returns them sorted ascending.
func findRcTags(versionPrefix string) []string {
	cmd := exec.Command("git", "tag", "--sort=v:refname")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var rcTags []string
	rcRe := regexp.MustCompile(`^v` + regexp.QuoteMeta(versionPrefix) + `-rc\.(\d+)$`)

	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "v"+versionPrefix+"-rc.") {
			continue
		}
		m := rcRe.FindStringSubmatch(line)
		if m != nil {
			rcTags = append(rcTags, line)
		}
	}

	// Already sorted ascending by --sort=v:refname.
	return rcTags
}

// parseRcNumber extracts the RC number from a tag string like "v0.3.0-rc.1".
func parseRcNumber(tag string) int {
	m := regexp.MustCompile(`-rc\.(\d+)$`).FindStringSubmatch(tag)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Bump returns a new SemVer after applying the given bump level.
func (v *SemVer) Bump(level string) (*SemVer, error) {
	switch level {
	case "major":
		return &SemVer{Major: v.Major + 1}, nil
	case "minor":
		return &SemVer{Major: v.Major, Minor: v.Minor + 1}, nil
	case "patch":
		return &SemVer{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}, nil
	default:
		return nil, fmt.Errorf("invalid bump level %q: must be major, minor, or patch", level)
	}
}

// BumpToFinal creates the final release version from an RC (strips prerelease suffix).
func (v *SemVer) BumpToFinal(level string) (*SemVer, error) {
	return v.Bump(level)
}

// LatestTag runs a single git call and returns the latest version tag string
// (e.g., "v2.1.0"), or an empty string if no tags exist.
func LatestTag() (string, error) {
	cmd := exec.Command("git", "tag", "--sort=-v:refname")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// No tags yet is not an error — return empty string.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return "", nil // no tags in repo
		}
		return "", fmt.Errorf("git tag --sort=-v:refname: %w\n%s", err, string(out))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		tag := strings.TrimSpace(scanner.Text())
		if tag == "" {
			continue
		}
		// Validate it looks like a version tag.
		if !strings.HasPrefix(tag, "v") {
			continue
		}
		if _, err := ParseSemver(tag); err != nil {
			continue // skip non-semver tags
		}
		return tag, nil // first = highest due to --sort=-v:refname
	}
	return "", nil // no valid version tags found
}

// LatestTagParsed runs a single git call and returns the latest parsed SemVer tag.
func LatestTagParsed() (*SemVer, error) {
	tagStr, err := LatestTag()
	if err != nil || tagStr == "" {
		return nil, err
	}
	return ParseSemver(tagStr)
}

// ValidateNewVersion checks that new version > latest git tag, with special handling for RC transitions.
func ValidateNewVersion(newVer *SemVer, latestTag string) error {
	if latestTag == "" {
		return nil // no existing tags — nothing to validate against
	}

	// RC-to-RC transition: if both new and latest are RCs of the same major.minor.patch,
	// only compare RC numbers. This allows rc1 → rc2 → ... → rcN without bumping version.
	if latestPrerelease := strings.TrimPrefix(latestTag, "v"); strings.Contains(latestPrerelease, "-rc.") {
		if newVer.Prerelease != "" && strings.HasPrefix(newVer.Prerelease, "rc.") {
			newRC := parseRcNumber("v" + newVer.String())
			tagRC := parseRcNumber(latestTag)
			if newRC <= tagRC {
				return fmt.Errorf(
					"new RC version %s (rc.%d) <= existing RC tag %s (rc.%d)",
					newVer.String(), newRC, latestTag, tagRC,
				)
			}
			return nil // valid RC increment
		}
	}

	latest, err := ParseSemver(latestTag)
	if err != nil {
		return fmt.Errorf("parse latest tag %q: %w", latestTag, err)
	}

	if newVer.Major < latest.Major ||
		(newVer.Major == latest.Major && newVer.Minor < latest.Minor) ||
		(newVer.Major == latest.Major && newVer.Minor == latest.Minor && newVer.Patch <= latest.Patch) {
		return fmt.Errorf(
			"new version %s <= existing tag %s (bump level too low for this tag)",
			newVer.String(), latestTag,
		)
	}
	return nil
}

// CreateTag creates an annotated git tag with the given version and message.
func CreateTag(ver string) error {
	tagName := "v" + ver
	msg := fmt.Sprintf("Release %s", tagName)
	cmd := exec.Command("git", "tag", "-a", tagName, "-m", msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git tag -a %s: %w\n%s", tagName, err, string(out))
	}
	fmt.Printf("Created tag: %s\n", tagName)
	return nil
}

// CreateLatestRef creates a lightweight "latest" tag pointing to the given version.
// This allows CI pipelines to reference refs/tags/latest for the most recent release.
func CreateLatestRef(ver string) error {
	tagName := ver // bare version without 'v' prefix (e.g., "1.0.0")

	// For RC versions, create "latest-rc" instead of "latest".
	if strings.Contains(tagName, "-rc.") || strings.HasPrefix(tagName, "rc.") {
		return nil // latest-rc handled separately in runBump for rc releases
	}

	cmd := exec.Command("git", "update-ref", "refs/tags/latest", "v"+tagName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git update-ref latest: %w\n%s", err, string(out))
	}
	fmt.Printf("Updated tag: latest -> v%s\n", tagName)
	return nil
}

// CreateLatestRcRef creates a lightweight "latest-rc" tag pointing to the RC version.
func CreateLatestRcRef(ver string) error {
	tagName := ver // bare version without 'v' prefix (e.g., "1.0.0-rc.1")

	cmd := exec.Command("git", "update-ref", "refs/tags/latest-rc", "v"+tagName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git update-ref latest-rc: %w\n%s", err, string(out))
	}
	fmt.Printf("Updated tag: latest-rc -> v%s\n", tagName)
	return nil
}

// ReadVersionFile reads and parses the version file (default VERSION.txt).
func ReadVersionFile(path string) (*SemVer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	versionStr := strings.TrimSpace(string(data))
	if versionStr == "" {
		return nil, fmt.Errorf("%s is empty", path)
	}
	return ParseSemver(versionStr)
}

// WriteVersionFile atomically writes the version string to the file.
func WriteVersionFile(path, content string) error {
	dir := "."
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		dir = path[:idx]
	}
	tmpPath := path + ".tmp"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	if err := os.WriteFile(tmpPath, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // clean up temp file on rename failure
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

// findBumpLabel searches a list of labels for exactly one matching bump label.
func findBumpLabel(labels []string) (found string, isRc bool, count int) {
	for _, l := range labels {
		l = strings.TrimSpace(l)
		level, rc, err := BumpLevelWithRC(l)
		if err == nil {
			found = level
			isRc = rc
			count++
		}
	}
	return found, isRc, count
}

// SplitMRLabels parses $CI_MERGE_REQUEST_LABELS which can be comma-separated,
// newline-separated, or space-separated. Returns individual label strings.
func SplitMRLabels(raw string) []string {
	var labels []string
	for line := range strings.SplitSeq(raw, "\n") {
		for part := range strings.FieldsSeq(line) {
			// Also handle comma-separated within a single field.
			for item := range strings.SplitSeq(part, ",") {
				item = strings.TrimSpace(item)
				if item != "" {
					labels = append(labels, item)
				}
			}
		}
	}
	return labels
}

// ResolveBumpLabel resolves the bump level from multiple sources in priority order:
// 1. CLI positional argument (a.BumpLevel)
// 2. $CI_MERGE_REQUEST_LABELS env var (GitLab MR labels, comma/newline separated)
// 3. $BUMP_LABEL env var (direct label format like "bump-minor")
func ResolveBumpLabel(cliLevel string) (level string, isRc bool, err error) {
	// 1. Explicit positional arg wins.
	if cliLevel != "" {
		switch cliLevel {
		case "major", "minor", "patch":
			return cliLevel, false, nil
		case "rc":
			// Bare rc — caller will determine default (rc-patch).
			return "", true, nil
		default:
			lvl, rc, err := BumpLevelWithRC(cliLevel)
			if err != nil {
				return "", false, err
			}
			return lvl, rc, nil
		}
	}

	// 2. Try GitLab MR labels first (CI context).
	mrLabels := os.Getenv("CI_MERGE_REQUEST_LABELS")
	if mrLabels != "" {
		labels := SplitMRLabels(mrLabels)
		found, isRc, count := findBumpLabel(labels)
		if count == 1 {
			return found, isRc, nil
		}
		if count > 1 {
			var rcLabels []string
			for _, l := range labels {
				level, rc, err := BumpLevelWithRC(l)
				if err == nil && rc {
					rcLabels = append(rcLabels, level)
				}
			}
			return "", true, fmt.Errorf("ambiguous: multiple bump labels found (%s)", strings.Join(rcLabels, ", "))
		}
	}

	// 3. Fallback to BUMP_LABEL env var (direct label format like "bump-minor").
	label := os.Getenv("BUMP_LABEL")
	if label != "" {
		level, isRc, err := BumpLevelWithRC(label)
		if err != nil {
			return "", false, fmt.Errorf("invalid $BUMP_LABEL %q: %w", label, err)
		}
		return level, isRc, nil
	}

	return "", false, fmt.Errorf("no bump source found — use 'changelog bump [major|minor|patch|rc]', set CI_MERGE_REQUEST_LABELS with a 'bump-*' label, or set BUMP_LABEL")
}

// BumpLevelNames returns all supported bump levels for interactive display and help text,
// including the standard release cycle: [rc-patch] → [rc] → [rc] → [patch].
// The bare "rc" label increments only the RC counter (0.1.2-rc5 → 0.1.2-rc6) without
// changing version numbers, allowing intermediate RC iterations before final release.
func BumpLevelNames() []string {
	return []string{
		"major", "minor", "patch", // standard bumps
		"rc",                               // bare rc: increments only the RC counter (0.1.2-rc5 → 0.1.2-rc6)
		"rc-major", "rc-minor", "rc-patch", // RC with version bump on the specified axis
	}
}

// BumpLevelDescriptions maps each label to a short description for interactive display.
var BumpLevelDescriptions = map[string]string{
	"major":    "Bump major version (e.g., 1.2.3 → 2.0.0)",
	"minor":    "Bump minor version (e.g., 1.2.3 → 1.3.0)",
	"patch":    "Bump patch version (e.g., 1.2.3 → 1.2.4)",
	"rc":       "Increment RC counter only (e.g., 1.0.0-rc.3 → 1.0.0-rc.4)",
	"rc-major": "Bump major then start RC series (e.g., 1.2.3 → 2.0.0-rc.1)",
	"rc-minor": "Bump minor then start RC series (e.g., 1.2.3 → 1.3.0-rc.1)",
	"rc-patch": "Bump patch then start RC series (e.g., 1.2.3 → 1.2.4-rc.1)",
}

// SortTagsBySemver sorts a slice of version tag strings in descending order by semver.
func SortTagsBySemver(tags []string) ([]string, error) {
	var versions []*SemVer
	for _, tag := range tags {
		v, err := ParseSemver(tag)
		if err != nil {
			continue // skip non-semver tags
		}
		versions = append(versions, v)
	}

	sort.SliceStable(versions, func(i, j int) bool {
		if versions[i].Major != versions[j].Major {
			return versions[i].Major > versions[j].Major
		}
		if versions[i].Minor != versions[j].Minor {
			return versions[i].Minor > versions[j].Minor
		}
		return versions[i].Patch > versions[j].Patch
	})

	var sorted []string
	for _, v := range versions {
		sorted = append(sorted, v.PrefixedString())
	}
	return sorted, nil
}

// ---------------------------------------------------------------------------
// ChangeEntry — machine-readable metadata stored in .changes.yaml.
// Mirrors upstream's changelog.yaml structure for generate().
// ---------------------------------------------------------------------------

// ChangeEntry represents a single release entry in the changes metadata file.
type ChangeEntry struct {
	Date     string              `yaml:"date"`
	Version  string              `yaml:"version"`
	Codename string              `yaml:"codename,omitempty"`
	Changes  map[string][]string `yaml:"changes"`
}

// SortFragmentsByIssue sorts fragments alphanumerically by issue ID.
func SortFragmentsByIssue(fragments []*Fragment) {
	sort.SliceStable(fragments, func(i, j int) bool {
		return fragments[i].Issue < fragments[j].Issue
	})
}

// SortFragmentsByVersion sorts fragments by version (for multi-version changelogs).
// Fragments are grouped by their issue prefix number.
func SortFragmentsByVersion(fragments []*Fragment) {
	sort.SliceStable(fragments, func(i, j int) bool {
		pi := extractVersionPriority(fragments[i].Issue)
		pj := extractVersionPriority(fragments[j].Issue)
		if pi != pj {
			return pi < pj
		}
		return fragments[i].Date > fragments[j].Date // newer date first for same priority
	})
}

// extractVersionPriority extracts a numeric priority from an issue string.
// For example, "PG-001" → 1, "ABC-42" → 42.
func extractVersionPriority(issue string) int {
	parts := strings.SplitN(issue, "-", 2)
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
}

// MergeFragmentSections combines all fragment sections into a single ChangeEntry.
func MergeFragmentSections(fragments []*Fragment) *ChangeEntry {
	entry := &ChangeEntry{Changes: make(map[string][]string)}

	for _, f := range fragments {
		if f.Issue != "" && f.Date != "" {
			continue // already has metadata from frontmatter
		}
		f.Issue = "unknown"
		f.Date = time.Now().Format("2006-01-02")
	}

	for _, f := range fragments {
		prefix := fmt.Sprintf("[%s] ", f.Issue)

		for sectionKey, items := range f.Sections {
			for _, item := range items {
				entry.Changes[sectionKey] = append(entry.Changes[sectionKey], prefix+item)
			}
		}

		if len(f.Bugfixes) > 0 {
			for _, item := range f.Bugfixes {
				entry.Changes["bugfixes"] = append(entry.Changes["bugfixes"], prefix+item.String())
			}
		}
		if len(f.MajorChanges) > 0 {
			for _, item := range f.MajorChanges {
				entry.Changes["major_changes"] = append(entry.Changes["major_changes"], prefix+item.String())
			}
		}
		if len(f.MinorChanges) > 0 {
			for _, item := range f.MinorChanges {
				entry.Changes["minor_changes"] = append(entry.Changes["minor_changes"], prefix+item.String())
			}
		}
	}

	return entry
}
