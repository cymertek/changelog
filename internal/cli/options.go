//go:build !docgen

// Package cli defines the command-line interface for changelog, providing
// commands that mirror a well-known Python changelog tool's surface.
package cli

import "github.com/jessevdk/go-flags"

// Options holds top-level CLI options.
// Mirrors a well-known Python tool's argparse-based CLI surface with 1:1 flag parity.
type Options struct {
	Init              InitCmd              `command:"init" description:"Set up changelog infrastructure for a project"`
	Add               AddCmd               `command:"add" description:"Create a new fragment file (compatible with upstream format)"`
	Lint              LintCmd              `command:"lint" description:"Validate all fragments in a directory"`
	Release           ReleaseCmd           `command:"release" description:"Generate release changelogs from fragments"`
	Generate          GenerateCmd          `command:"generate" description:"(Re-)generate the changelog from stored data"`
	Reformat          ReformatCmd          `command:"reformat" description:"Reformat changelog.yaml / .changes.yaml"`
	CollectionNew     CollectionNewCmd     `command:"collection-new" description:"Generate Ansible collection plugin docs for changelogs"`
	LintChangelogYaml LintChangelogYamlCmd `command:"lint-changelog-yaml" description:"Check syntax of changelog.yaml file"`
	Log               LogCmd               `command:"log" description:"Show changelog in git-log style"`
	Bump              BumpCmd              `command:"bump" description:"Bump VERSION.txt from bump label env var or CI MR labels"`
	Preview           PreviewCmd           `command:"preview" description:"Render fragments as colored markdown"`
	License           LicenseCmd           `command:"license" description:"Print the project license (CYMERTEK Source-Available License)"`
}

// InitCmd initializes changelog infrastructure for a project.
// Mirrors: init PROJECT_ROOT [--is-other-project] [-v]
type InitCmd struct { //nolint:revive // required by cobra CLI structure
	ProjectRoot    string `positional:"true" optional:"true" optionalarg:"." description:"Path to project root (defaults to current dir)"`
	IsOtherProject bool   `long:"is-other-project" description:"Project is not an Ansible collection"`
	Verbose        int    `short:"v" long:"verbose" default:"0" description:"Increase verbosity"`
}

// AddCmd creates a new changelog fragment file.
// Flags match the Python tool exactly.
type AddCmd struct { //nolint:revive // required by cobra CLI structure
	Category string `short:"c" long:"category" required:"false" default:"bugfixes" description:"Category of change: bugfixes, minor_changes, major_changes, breaking_changes, deprecated_features, removed_features, security_fixes, known_issues"`
	Issue    string `short:"i" long:"issue" required:"true" description:"Issue or JIRA ID (e.g. PG-001)"`
	Text     string `short:"T" long:"text" description:"Fragment body text content"`
	File     string `short:"f" long:"file" description:"Read fragment body from existing file"`
	Date     string `long:"date" default:"" description:"Date in YYYY-MM-DD format (defaults to today)"`
	Dir      string `short:"d" long:"dir" default:"changelogs/fragments" description:"Fragment output directory"`
	Title    string `short:"t" long:"title" description:"Human-readable title for the fragment"`
}

// LintCmd validates all fragments in a directory.
// Mirrors: lint [FRAGMENT ...]
type LintCmd struct { //nolint:revive // required by cobra CLI structure
	Fragments []string `positional:"true" optional:"true" description:"Specific fragment file(s) to check (if omitted, all fragments are checked)"`
	Dir       string   `short:"d" long:"dir" default:"changelogs/fragments" description:"Main fragment directory"`
	Strict    bool     `long:"strict" description:"Extra validation: require date field, check link syntax, no empty bodies"`
}

// ReleaseCmd generates changelog from collected fragments.
// Mirrors: release with all shared build + collection flags
type ReleaseCmd struct { //nolint:revive // required by cobra CLI structure
	Version  string `short:"v" long:"version" description:"Release version (e.g., v1.0.0)"`
	Date     string `long:"date" default:"" description:"Release date YYYY-MM-DD (defaults to today)"`
	Codename string `long:"codename" description:"Release codename for version display"`

	// Shared build options (common with generate)
	ReloadPlugins    bool   `long:"reload-plugins" description:"Force reload of plugin cache"`
	UseDocTool       bool   `long:"use-ansible-doc" description:"Always use doc tool to find plugins"`
	DocToolBin       string `long:"ansible-doc-bin" default:"" description:"Path to doc binary (overrides autodetect)"`
	Refresh          bool   `long:"refresh" description:"Update existing entries from fragments and update plugin descriptions"`
	RefreshPlugins   string `long:"refresh-plugins" optional:"true" optionalarg:"allow-removal" description:"Update plugin descriptions: allow-removal or prevent-removal"`
	RefreshFragments string `long:"refresh-fragments" optional:"true" optionalarg:"with-archives" description:"Update entries from fragments: without-archives or with-archives"`

	// Collection detection flags
	CollectionNamespace string `long:"collection-namespace" default:"" description:"Override collection namespace"`
	CollectionName      string `long:"collection-name" default:"" description:"Override collection name"`
	CollectionFlatmap   bool   `long:"collection-flatmap" description:"Enable collection flatmapping"`
	IsCollection        *bool  `long:"is-collection" optional:"true" optionalarg:"yes" description:"Override whether this is a collection (true/false/yes/no)"`

	// Release behavior flags
	UpdateExisting    bool `long:"update-existing" description:"If release exists, update date/codename instead of erroring"`
	CumulativeRelease bool `long:"cummulative-release" description:"Include all plugins/modules added since previous release/ancestor"`

	Dir       string   `short:"d" long:"dir" default:"changelogs" description:"Changelog base directory (contains fragments/)"`
	NotesDirs []string `short:"n" long:"notes-dir" multiple:"true" description:"Additional fragment directories to merge"`

	Output string `short:"o" long:"output" default:"CHANGELOG.md" description:"Output file path"`

	// Lint-changelog-yaml compat flag (used by release to match generate).
	NoSemanticVersioning bool `long:"no-semantic-versioning" description:"Assume use_semantic_versioning=false in the changelog config."`
}

// GenerateCmd regenerates the changelog from stored fragment data.
// Mirrors: generate [VERSION] [--output PATH] [--only-latest] [--output-format FMT]
type GenerateCmd struct { //nolint:revive,golint // required by cobra CLI structure
	Version string `positional:"true" optional:"true" description:"Generate changelog for this specific version instead of latest"`

	// Shared build options (common with release)
	ReloadPlugins    bool   `long:"reload-plugins" description:"Force reload of plugin cache"`
	UseDocTool       bool   `long:"use-ansible-doc" description:"Always use ansible-doc to find plugins"`
	DocToolBin       string `long:"ansible-doc-bin" default:"" description:"Path to ansible-doc binary (overrides autodetect)"`
	Refresh          bool   `long:"refresh" description:"Update existing entries from fragments and update plugin descriptions"`
	RefreshPlugins   string `long:"refresh-plugins" optional:"true" optionalarg:"allow-removal" description:"Update plugin descriptions: allow-removal or prevent-removal"`
	RefreshFragments string `long:"refresh-fragments" optional:"true" optionalarg:"with-archives" description:"Update entries from fragments: without-archives or with-archives"`

	// Collection detection flags
	CollectionNamespace string `long:"collection-namespace" default:"" description:"Override collection namespace"`
	CollectionName      string `long:"collection-name" default:"" description:"Override collection name"`
	CollectionFlatmap   bool   `long:"collection-flatmap" description:"Enable collection flatmapping"`
	IsCollection        *bool  `long:"is-collection" optional:"true" optionalarg:"yes" description:"Override whether this is a collection (true/false/yes/no)"`

	// Output-specific flags
	Output     string `short:"o" long:"output" default:"CHANGELOG.md" description:"Write the changelog to this file instead of the default location"`
	OnlyLatest bool   `long:"only-latest" description:"Only write the changelog entry for the latest version, without preamble (use with --output)"`
	// OutputFormat removed: RST is deprecated; only markdown output is supported.

	Dir       string   `short:"d" long:"dir" default:"changelogs" description:"Changelog base directory (contains fragments/)"`
	NotesDirs []string `short:"n" long:"notes-dir" multiple:"true" description:"Additional fragment directories to merge"`

	// Lint-changelog-yaml compat flags.
	NoSemanticVersioning bool `long:"no-semantic-versioning" description:"Assume use_semantic_versioning=false in the changelog config."`
}

// LintChangelogYamlCmd validates the syntax of a changelog.yaml file.
// Mirrors: lint-changelog-yaml PATH [--strict] [--no-semantic-versioning]
type LintChangelogYamlCmd struct { //nolint:revive,golint // required by cobra CLI structure
	ChangelogYamlPath    string `positional:"true" required:"true" description:"Path to changelog.yaml file"`
	Strict               bool   `long:"strict" description:"Extra validation: complain about extra entries not in changelog.yaml spec"`
	NoSemanticVersioning bool   `long:"no-semantic-versioning" description:"Assume use_semantic_versioning=false in the changelog config."`
}

// ReformatCmd reformats a changelog.yaml or .changes.yaml file.
// Mirrors: reformat [--is-collection]
type ReformatCmd struct { //nolint:revive,golint // required by cobra CLI structure
	IsCollection bool   `long:"is-collection" description:"Override whether this is a collection"`
	Dir          string `short:"d" long:"dir" default:"changelogs" description:"Changelog base directory"`
}

// CollectionNewCmd initializes a new collection's changelog directory.
// Mirrors: collection-new [--doc-bin PATH] [--use-doc-tool] [--reload-plugins]
type CollectionNewCmd struct { //nolint:revive,golint // required by cobra CLI structure
	DocToolBin string `long:"ansible-doc-bin" default:"" description:"Path to doc binary (overrides autodetect)"`
	UseDocTool bool   `long:"use-ansible-doc" description:"Always use doc tool to find plugins"`

	// Collection detection flags.
	CollectionNamespace string `long:"collection-namespace" default:"" description:"Override collection namespace"`
	CollectionName      string `long:"collection-name" default:"" description:"Override collection name"`

	// Shared build options.
	ReloadPlugins    bool   `long:"reload-plugins" description:"Force reload of plugin cache"`
	Refresh          bool   `long:"refresh" description:"Update existing entries from fragments and update plugin descriptions"`
	RefreshPlugins   string `long:"refresh-plugins" optional:"true" optionalarg:"allow-removal" description:"Update plugin descriptions: allow-removal or prevent-removal"`
	RefreshFragments string `long:"refresh-fragments" optional:"true" optionalarg:"with-archives" description:"Update entries from fragments: without-archives or with-archives"`

	Dir string `short:"d" long:"dir" default:"changelogs" description:"Changelog base directory"`
}

// LogCmd displays the changelog in a git-log style format.
// ---------------------------------------------------------------------------
type LogCmd struct { //nolint:revive,golint // required by cobra CLI structure
	Dir         string `short:"d" long:"dir" default:"changelogs/fragments" description:"Fragment directory to display"`
	PageSize    int    `long:"page-size" default:"-1" description:"Number of entries per page (default 10 when not set)"`
	Interactive bool   `long:"interactive" short:"i" description:"Enable interactive pagination with keypress navigation"`
}

// BumpCmd bumps VERSION.txt using a bump label from env vars or CI MR labels.
type BumpCmd struct { //nolint:revive,golint // required by cobra CLI structure
	Level       string `long:"level" description:"Bump level: major, minor, patch, or rc"`
	VersionFile string `long:"version-file" default:"VERSION.txt" description:"Path to VERSION.txt"`
	Rc          bool   `long:"rc" description:"Append -rc.N suffix for non-master PR pipelines (auto-detects existing RC tags)"`
}

// GetLevel returns the bump level.
func (b *BumpCmd) GetLevel() string { return b.Level }

// PreviewCmd renders fragment changes as colored markdown to stdout.
// Mirrors: preview (extension)
type PreviewCmd struct { //nolint:revive,golint // required by cobra CLI structure
	Dir       string   `short:"d" long:"dir" default:"changelogs/fragments" description:"Fragment directory to render"`
	NotesDirs []string `short:"n" long:"notes-dir" multiple:"true" description:"Additional fragment directories to merge"`
}

// LicenseCmd prints the project license embedded in the binary.
type LicenseCmd struct { //nolint:revive,golint // required by cobra CLI structure
	Verbose bool `long:"verbose" short:"V" description:"Print full license text including all sections"`
}

// ParseFlags parses CLI args into the given Options struct using go-flags.
func ParseFlags(args []string) (*Options, error) {
	var opts Options
	parser := flags.NewParser(&opts, flags.Default)
	_, err := parser.ParseArgs(args)
	return &opts, err
}
