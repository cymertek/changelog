package changelog

import (
	"fmt"
	"os"
	"path/filepath"
)

// InitDir bootstraps a changelogs/ directory structure with config.
func InitDir(dir string, title string) error {
	// Use the standard changelog directory layout: changelogs/ (upstream compatible).
	changelogDir := filepath.Join(".", "changelogs")
	fragmentsDir := filepath.Join(changelogDir, "fragments")

	// Create directory structure.
	for _, d := range []string{changelogDir, fragmentsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	cfg := &Config{
		Title:                         title,
		VersionPattern:                `^\d+\.\d+\.\d+$`,
		KeepFragments:                 false,
		IsOtherProject:                true,                      // default to true — most wrapper projects are "other"
		SanitizeChangelog:             true,                      // run body through markdown formatter
		UseFQCN:                       false,                     // only relevant when wrapping plugin collections
		Flatmap:                       boolPtr(false),            // use short plugin paths instead of FQCN (nil=unset)
		TrivialSectionName:            defaultTrivialSectionName, // section header for trivial changes
		UseSemanticVersioning:         false,                     // semver enforcement toggle
		IgnoreOtherFragmentExtensions: true,                      // only process .md fragments
		MentionAncestor:               true,                      // enable ancestor changelog mention (matches upstream bool=True default)
		ChangelogFilenameTemplate:     defaultChangelogFilenameTemplate,
		ChangelogFilenameVersionDepth: 0,                          // no version segment in fragment filename by default
		PreludeSectionTitle:           defaultPreludeSectionTitle, // unreleased section header
		PreludeSectionName:            "",                         // empty = no prelude category
		OutputFormats:                 []string{"markdown"},       // only output markdown changelog

		// Additional upstream compat defaults.
		AlwaysRefresh:       "full",                                   // include all fragments on release by default
		ChangelogSort:       "none",                                   // no sorting — rely on category priority in render.go
		ArchivePathTemplate: filepath.Join(dir, "archive"),            // move fragments to archive dir after release
		ChangesFile:         ".changes.yaml",                          // machine-readable storage path
		ReleaseTagRe:        `((?:[\d.ab]|rc)+)`,                      // regex matching stable release tag format
		PreReleaseTagRe:     `(?P<pre_release>\.\d+(?:[ab]|rc)+\d*)$`, // regex matching prerelease tag format
		Vcs:                 "auto",                                   // auto-detect version control (git, none)
		NotesDir:            "fragments",                              // fragment subdirectory name
	}

	cfg.Defaults()

	configPath := filepath.Join(changelogDir, "config.yaml")
	if err := cfg.SaveCommented(configPath); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
	}
	fmt.Printf("Created: %s\n", configPath)

	fmt.Printf("Changelog directory initialized at %s\n", dir)
	fmt.Printf("  - Edit changelogs/config.yaml to configure your changelog.\n")
	fmt.Printf("  - Add fragments: changelog add --category bugfixes -i PG-001 -T \"Fixed null pointer crash\"\n")
	fmt.Printf("  - Add fragments with: changelog add --category enhancement -i <issue> -T \"Your text\"\n")
	fmt.Printf("  - Generate CHANGELOG.md with: changelog release --version v1.0.0\n")
	fmt.Printf("  - For full documentation, visit: https://github.com/cymertek/changelog\n")

	return nil
}

// boolPtr returns a pointer to the provided bool value. Used for three-state fields
// that follow upstream's Optional[bool] semantics (nil=unset, true/false explicit).
func boolPtr(v bool) *bool {
	return &v
}
