//go:build !docgen

// Package main is the entry point for changelog, a CLI tool that generates and manages changelogs.
// It can be installed via `go install github.com/cymertek/changelog@latest`.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/jessevdk/go-flags"

	"github.com/cymertek/changelog/internal/changelog"
	"github.com/cymertek/changelog/internal/cli"
)

//go:embed LICENSE
var embeddedLicense []byte

// licenseText returns the embedded license text as a string.
func licenseText() string { return string(embeddedLicense) }

// Version is set at build time via ldflags. Default "dev" for untagged builds.
var Version = "1.0.0"

// main is the entry point — parses CLI flags, dispatches to handlers.
func main() {
	argv := os.Args[1:]

	// Detect whether a recognized subcommand appears in argv before any flag.
	// If it does, let go-flags handle everything normally (including its own
	// --help for that subcommand). Only intercept when no valid subcommand is
	// present — e.g. top-level -h/--help with no command, unknown flags like
	// -title "foo", or bare flag arguments with no command at all.
	firstSubcmdIdx := -1
	for i, arg := range argv {
		if strings.HasPrefix(arg, "-") {
			break
		}
		if isKnownSubcommand(arg) {
			firstSubcmdIdx = i
			break
		}
	}

	// Check for --version flag ONLY at top level (no subcommand present).
	// abc supports this: `changelog --version` shows tool version.
	// When used after a subcommand, go-flags handles it as that command's option.
	if firstSubcmdIdx < 0 {
		for _, arg := range argv {
			if arg == "--version" || arg == "-V" {
				fmt.Printf("changelog %s\n", Version)
				return
			}
		}

		// Only intercept if there's no recognized subcommand AND we have flags.
		// This lets go-flags handle --help for valid subcommands normally.
		hasFlag := false
		for _, arg := range argv {
			if strings.HasPrefix(arg, "-") && arg != "--version" && arg != "-V" {
				hasFlag = true
				break
			}
		}

		// For top-level help/version flags without a subcommand, show custom usage.
		// This ensures global flags are documented in the output.
		if hasFlag && !containsHelp(argv) {
			showUsage()
			return
		}

		// If --help was passed at top level (no subcommand), also use custom format.
		if containsHelp(argv) && firstSubcmdIdx < 0 {
			showUsage()
			return
		}
	}

	a, err := parseFlags(argv)
	if err != nil {
		// If it's a help error (user passed --help or -h), exit cleanly.
		// go-flags prints the help text itself; we just need to return success.
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			return
		}
		// ErrCommandRequired and ErrUnknownCommand: go-flags with PrintErrors already printed a clean message
		// (e.g. "Please specify one command of: ..." or "Unknown command `foo'."). Don't add our own "error:" prefix
		// or we'd duplicate it on stderr.
		if flagsErr, ok := err.(*flags.Error); ok && (flagsErr.Type == flags.ErrCommandRequired || flagsErr.Type == flags.ErrUnknownCommand) {
			os.Exit(1)
		}
		// For other errors (unknown flag, missing required arg, etc.), print
		// the error message and exit with non-zero status. Don't fall through
		// to interactive mode for CLI errors — that's only for no-subcommand case.
		if flagsErr, ok := err.(*flags.Error); ok {
			fmt.Fprintf(os.Stderr, "error: %s\n", flagsErr.Message)
			os.Exit(1)
		}
		return
	}

	a.run()
}

// containsHelp returns true if argv contains --help or -h flags.
func containsHelp(argv []string) bool {
	for _, arg := range argv {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// showUsage prints the top-level help/usage message and exits.
func showUsage() {
	fmt.Fprintln(os.Stderr, `changelog — changelog management tool

Usage:
  changelog <command> [flags]

Available commands:
  init                 Set up changelog infrastructure for a project
  add                  Create a new fragment file
  lint                 Validate all fragments in a directory
  release              Generate release changelogs from fragments
  generate             (Re-)generate the changelog from stored data
  reformat             Reformat changelog.yaml / .changes.yaml
  collection-new       Initialize changelog directory for a new project
  lint-changelog-yaml  Check syntax of changelog.yaml file
  log                  Show changelog in git-log style
  bump                 Bump VERSION.txt from bump label env var or CI MR labels
  preview              Render fragments as colored markdown
  license              Print the project license

Global flags:
  -V, --version        Print version and exit (works at any position)
  -h, --help           Show this help message
  -v, --verbose        Increase verbosity of output
  -q, --quiet          Suppress non-essential output

Source code & documentation: https://github.com/cymertek/changelog
License: CYMERTEK Source-Available License (CSAL) — see changelog license or visit the repo.

Use "changelog <command> --help" for command-specific flags.`)
}

// isKnownSubcommand returns true if the given string matches a recognized subcommand name.
func isKnownSubcommand(s string) bool {
	switch s {
	case "init", "add", "lint", "release", "generate",
		"reformat", "collection-new", "lint-changelog-yaml", "log", "bump",
		"preview", "license":
		return true
	}
	return false
}

// action represents a changelog subcommand dispatched from CLI flags.
type action string

const (
	actionInit              action = "init"
	actionAdd               action = "add"
	actionLint              action = "lint"
	actionRelease           action = "release"
	actionGenerate          action = "generate"
	actionReformat          action = "reformat"
	actionCollectionNew     action = "collection-new"
	actionLintChangelogYaml action = "lint-changelog-yaml"
	actionPreview           action = "preview"
	actionLog               action = "log"
	actionBump              action = "bump"
	actionLicense           action = "license"
)

// args carries the parsed input for every subcommand.
type args struct {
	Action      action
	ProjectRoot string // init: project root path
	Title       string // init: project title (legacy compat)

	// new / add flags
	Category  string
	Issue     string
	Text      string
	File      string
	Date      string
	Dir       string
	TitleFlag string

	// lint flags
	LintFragments []string
	Strict        bool

	// release flags
	Version              string
	Codename             string
	ReloadPlugins        bool
	UseDocTool           bool
	Refresh              bool
	RefreshPlugins       string
	RefreshFrags         string
	IsCollection         *bool
	UpdateExisting       bool
	CumulativeRelease    bool
	KeepFragments        bool
	DocToolBin           string
	NoSemanticVersioning bool

	// generate flags
	GenVersion string
	OutputFile string
	OnlyLatest bool
	// OutputFormat removed: RST is deprecated; only markdown output is supported.

	// shared
	DirRoot     string   // base changelog directory
	NotesDirs   []string // additional fragment dirs to merge
	Output      string   // output file path (release/generate)
	PageSize    int      // log page size
	BumpLabel   string   // bump level
	VersionFile string   // bump version file path

	// reformat / collection-new
	CollectionNamespace string
	CollectionName      string
	CollectionFlatmap   bool

	// license command
	LicenseVerbose bool
}

// run dispatches the requested action.
func (a *args) run() {
	switch a.Action {
	case actionInit:
		a.runInit()
	case actionAdd:
		a.runNewOrAdd()
	case actionLint:
		a.runLint()
	case actionRelease:
		a.runRelease()
	case actionGenerate:
		a.runGenerate()
	case actionReformat:
		a.runReformat()
	case actionCollectionNew:
		a.runCollectionNew()
	case actionPreview:
		a.runPreview()
	case actionLog:
		a.runLog()
	case actionBump:
		a.runBump()
	case actionLicense:
		a.runLicense()
	case actionLintChangelogYaml:
		a.runLintChangelogYaml()
	default:
		fmt.Fprintf(os.Stderr, "unknown action: %s\n", a.Action)
		os.Exit(1)
	}
}

// parseArgs builds an args struct from go-flags cli.Options.
func parseFlags(argv []string) (*args, error) {
	var opts cli.Options
	parser := flags.NewParser(&opts, flags.Default)
	remainingArgs, err := parser.ParseArgs(argv)
	if err != nil {
		return nil, err
	}

	activeSubcmd := ""
	if parser.Command.Active != nil {
		activeSubcmd = parser.Command.Active.Name
	}

	// Extract positional arguments from remaining args for subcommands that need them.
	positionalArg := ""
	if len(remainingArgs) > 0 {
		positionalArg = remainingArgs[0]
	}

	switch activeSubcmd {
	case "init":
		root := opts.Init.ProjectRoot
		if root == "" || root == "." {
			var cwd string
			cwd, _ = os.Getwd()
			root = cwd
		}
		return &args{Action: actionInit, ProjectRoot: root}, nil

	case "add":
		return &args{
			Action: actionAdd, Category: opts.Add.Category, Issue: opts.Add.Issue,
			Text: opts.Add.Text, File: opts.Add.File, Date: opts.Add.Date,
			Dir: opts.Add.Dir, TitleFlag: opts.Add.Title,
		}, nil

	case "lint":
		return &args{
			Action: actionLint, DirRoot: opts.Lint.Dir, Strict: opts.Lint.Strict,
			LintFragments: opts.Lint.Fragments,
		}, nil

	case "release":
		return &args{
			Action: actionRelease, Version: opts.Release.Version,
			Date: opts.Release.Date, DirRoot: opts.Release.Dir, Output: opts.Release.Output,
			NotesDirs:            opts.Release.NotesDirs,
			ReloadPlugins:        opts.Release.ReloadPlugins,
			UseDocTool:           opts.Release.UseDocTool,
			Refresh:              opts.Release.Refresh,
			RefreshPlugins:       opts.Release.RefreshPlugins,
			RefreshFrags:         opts.Release.RefreshFragments,
			UpdateExisting:       opts.Release.UpdateExisting,
			CumulativeRelease:    opts.Release.CumulativeRelease,
			CollectionNamespace:  opts.Release.CollectionNamespace,
			CollectionName:       opts.Release.CollectionName,
			CollectionFlatmap:    opts.Release.CollectionFlatmap,
			IsCollection:         opts.Release.IsCollection,
			DocToolBin:           opts.Release.DocToolBin,
			NoSemanticVersioning: opts.Release.NoSemanticVersioning,
		}, nil

	case "generate":
		return &args{
			Action: actionGenerate, GenVersion: opts.Generate.Version,
			OutputFile: opts.Generate.Output,
			OnlyLatest: opts.Generate.OnlyLatest, DirRoot: opts.Generate.Dir,
			NotesDirs:            opts.Generate.NotesDirs,
			ReloadPlugins:        opts.Generate.ReloadPlugins,
			UseDocTool:           opts.Generate.UseDocTool,
			Refresh:              opts.Generate.Refresh,
			RefreshPlugins:       opts.Generate.RefreshPlugins,
			RefreshFrags:         opts.Generate.RefreshFragments,
			IsCollection:         opts.Generate.IsCollection,
			DocToolBin:           opts.Generate.DocToolBin,
			NoSemanticVersioning: opts.Generate.NoSemanticVersioning,
		}, nil

	case "reformat":
		return &args{
			Action: actionReformat, DirRoot: opts.Reformat.Dir,
			CollectionNamespace: opts.CollectionNew.CollectionNamespace,
			CollectionName:      opts.CollectionNew.CollectionName,
		}, nil

	case "collection-new":
		return &args{
			Action: actionCollectionNew, DirRoot: opts.CollectionNew.Dir,
			DocToolBin:          opts.CollectionNew.DocToolBin,
			UseDocTool:          opts.CollectionNew.UseDocTool,
			CollectionNamespace: opts.Release.CollectionNamespace,
			CollectionName:      opts.Release.CollectionName,
		}, nil

	case "log":
		pageSize := opts.Log.PageSize
		if pageSize <= 0 {
			pageSize = 10
		}
		return &args{Action: actionLog, DirRoot: opts.Log.Dir, PageSize: pageSize}, nil

	case "preview":
		return &args{
			Action:    actionPreview,
			DirRoot:   opts.Preview.Dir,
			NotesDirs: opts.Preview.NotesDirs,
		}, nil

	case "bump":
		return &args{
			Action: actionBump, BumpLabel: opts.Bump.Level,
			VersionFile: opts.Bump.VersionFile,
		}, nil

	case "license":
		return &args{
			Action:         actionLicense,
			LicenseVerbose: opts.License.Verbose,
		}, nil

	case "lint-changelog-yaml":
		path := positionalArg
		if path == "" {
			path = opts.LintChangelogYaml.ChangelogYamlPath
		}
		return &args{
			Action:               actionLintChangelogYaml,
			DirRoot:              path,
			Strict:               opts.LintChangelogYaml.Strict,
			NoSemanticVersioning: opts.LintChangelogYaml.NoSemanticVersioning,
		}, nil

	default:
		return nil, fmt.Errorf("no subcommand specified — use 'changelog init', 'changelog new', or install zenity/osascript for interactive mode")
	}
}

// ---------------------------------------------------------------------------
// runInit — Bootstrap changelogs/ directory + config.
// ---------------------------------------------------------------------------
func (a *args) runInit() {
	root := a.ProjectRoot
	if root == "" {
		cwd, _ := os.Getwd()
		root = cwd
	}

	changelogDir := filepath.Join(root, "changelogs")
	fragmentsDir := filepath.Join(changelogDir, "fragments")

	if err := os.MkdirAll(fragmentsDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating fragments dir %s: %v\n", fragmentsDir, err)
		os.Exit(1)
	}

	title := "Project" // default title for non-collection projects
	if err := changelog.InitDir(fragmentsDir, title); err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Changelog infrastructure initialized at %s\n", root)
	fmt.Printf("  Config:      %s/changelogs/config.yaml\n", root)
	fmt.Printf("  Fragments:   %s/changelogs/fragments/\n", root)
}

// ---------------------------------------------------------------------------
// runNewOrAdd — Create a fragment file in YAML section format.
// Creates .yml fragments with proper YAML section structure (not markdown frontmatter).
// Also handles the colon-in-string issue by auto-quoting values that contain colons.
// ---------------------------------------------------------------------------
func (a *args) runNewOrAdd() {
	if a.Issue == "" || a.Issue == "-1" {
		fmt.Fprintln(os.Stderr, "error: --issue is required (e.g., -i PG-001)")
		os.Exit(1)
	}

	var body string
	switch {
	case a.Text != "":
		body = a.Text
	case a.File != "":
		data, err := os.ReadFile(a.File)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file %s: %v\n", a.File, err)
			os.Exit(1)
		}
		body = string(data)
	default:
		fmt.Fprintln(os.Stderr, "error: --text or --file required (or pipe to stdin)")
		os.Exit(1)
	}

	dir := a.Dir
	if dir == "" {
		dir = "changelogs/fragments"
	}

	// Resolve the actual fragments directory.
	fragDir, err := changelog.ResolveDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving fragment dir: %v\n", err)
		os.Exit(1)
	}

	date := a.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	category := strings.ToLower(a.Category)
	validCategories := []string{
		"bugfixes", "minor_changes", "major_changes", "breaking_changes",
		"deprecated_features", "removed_features", "security_fixes", "known_issues",
		"trivial", "release_summary", "note", "enhancement", "deprecation", "removal",
	}
	if !slices.Contains(validCategories, category) {
		fmt.Fprintf(os.Stderr, "error: invalid category %q. Valid categories: %s\n",
			category, strings.Join(validCategories, ", "))
		os.Exit(1)
	}

	frag := &changelog.Fragment{
		Category: category,
		Issue:    a.Issue,
		Date:     date,
		Title:    a.TitleFlag,
		Body:     strings.TrimSpace(body),
	}

	if err := frag.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "validation error: %v\n", err)
		os.Exit(1)
	}

	if err := frag.WriteToYAML(fragDir); err != nil {
		fmt.Fprintf(os.Stderr, "error writing fragment: %v\n", err)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// runLint — Validate all fragments.
// Lint fragments with enhanced colon-in-string detection.
// ---------------------------------------------------------------------------
func (a *args) runLint() {
	var dirs []string
	if len(a.LintFragments) > 0 {
		// Linting specific fragment files provided on the command line.
		for _, fpath := range a.LintFragments {
			dir := filepath.Dir(fpath)
			dirs = append(dirs, dir)
		}
	} else {
		dir, err := changelog.ResolveDir(a.DirRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lint error: %v\n", err)
			os.Exit(1)
		}
		dirs = append(dirs, dir)
		dirs = append(dirs, a.NotesDirs...)
	}

	errors := []string{}
	for _, d := range dirs {
		fileErrors, err := changelog.LintDir(d, a.Strict)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lint error in %s: %v\n", d, err)
			continue
		}
		errors = append(errors, fileErrors...)
	}

	// Check for colon-in-string issues in YAML fragments.
	colonErrors := detectColonIssues(dirs)
	errors = append(errors, colonErrors...)

	if len(errors) > 0 {
		sort.Strings(errors)
		for _, e := range errors {
			fmt.Fprintln(os.Stderr, e)
		}
		fmt.Fprintf(os.Stderr, "\n%d lint error(s) found\n", len(errors))
		os.Exit(3) // RC_INVALID_FRAGMENT — matches upstream return code
	}

	fmt.Println("All fragments valid.")
}

// detectColonIssues finds list items that look like they contain unquoted colons.
// This addresses the PyYAML parsing issue where "- here is my day: day"
// gets parsed as a dict instead of a string.
func detectColonIssues(dirs []string) []string {
	var errors []string

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
				path := filepath.Join(dir, name)
				colonErrs := checkColonInFile(path)
				errors = append(errors, colonErrs...)
			}
		}
	}

	return errors
}

// checkColonInFile reads a YAML fragment and checks for list items that look like dicts.
func checkColonInFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return []string{path + ":0:0: yaml parsing error"}
	}

	// Check each section key for list items that are maps (dicts from unquoted colons).
	var errors []string
	for key, val := range raw {
		items, ok := val.([]any)
		if !ok {
			continue // skip non-list values (release_summary is a string)
		}

		for i, item := range items {
			m, isMap := item.(map[string]any)
			if !isMap || len(m) == 0 {
				continue
			}

			// Check if the map looks like an unquoted colon string.
			for k, v := range m {
				valStr := fmt.Sprintf("%v", v)
				if strings.Contains(k, " ") && valStr != "" {
					errors = append(errors,
						fmt.Sprintf("%s: %s list item %d: '%s' contains unquoted colon — interpret as dict; quote it with double-quotes or escape the colon",
							filepath.Base(path), key, i+1, k))
				}
			}
		}
	}

	return errors
}

// findConfigFile locates the changelog config file (config.yaml).
// Tries multiple locations: direct path, with "changelogs/" prefix.
func findConfigFile(dir string) string {
	candidates := []string{
		filepath.Join(dir, "config.yaml"),
		filepath.Join(dir, "changelogs", "config.yaml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// renderTemplates renders configured Go templates for additional output formats.
func renderTemplates(dir string, fragments []*changelog.Fragment, version, date, codename string) error {
	// Load config to get template definitions.
	// dir might already include "changelogs/" or might be the project root.
	cfgPath := findConfigFile(dir)
	if cfgPath == "" {
		return nil // No config found — skip template rendering.
	}

	cfg, err := changelog.Load(cfgPath)
	if err != nil {
		return nil // Config invalid — skip template rendering.
	}

	// Only render if templates are configured.
	if len(cfg.Templates) == 0 {
		return nil
	}

	// Render all configured templates.
	outputs, err := changelog.RenderToFiles(dir, fragments, version, date, codename, cfg)
	if err != nil {
		return err
	}

	fmt.Printf("\nTemplate outputs generated:\n")
	for name, path := range outputs {
		fmt.Printf("  %s: %s\n", name, path)
	}

	return nil
}

// ---------------------------------------------------------------------------
// runRelease — Generate changelog from accumulated fragments.
// Mirrors: release with archive support, sorting, and metadata persistence.
// ---------------------------------------------------------------------------
func (a *args) runRelease() {
	dir := a.DirRoot
	if dir == "" {
		d, err := changelog.FindChangelogDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not find changelog directory\n")
			os.Exit(1)
		}
		dir = d
	}

	fragmentsDir := dir
	if _, err := os.Stat(filepath.Join(dir, "fragments")); err == nil {
		fragmentsDir = filepath.Join(dir, "fragments")
	} else if _, err := os.Stat(filepath.Join(dir, "changelog.d")); err == nil {
		fragmentsDir = filepath.Join(dir, "changelog.d")
	}

	fragments, err := changelog.MergeCollect(fragmentsDir, a.NotesDirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error collecting fragments: %v\n", err)
		os.Exit(1)
	}

	date := a.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	version := a.Version
	if version == "" {
		fmt.Fprintln(os.Stderr, "error: --version is required for release (e.g., -v 1.0.0)")
		os.Exit(1)
	}

	// Version pattern enforcement: validate version against config pattern.
	cfgPath := findConfigFile(dir)
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg struct {
			VersionPattern        string `yaml:"version_pattern"`
			UseSemanticVersioning bool   `yaml:"use_semantic_versioning"`
		}
		if yaml.Unmarshal(data, &cfg) == nil && cfg.UseSemanticVersioning && !a.NoSemanticVersioning {
			if cfg.VersionPattern != "" {
				matched, err := regexp.MatchString(cfg.VersionPattern, version)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: invalid version_pattern regex %q: %v\n", cfg.VersionPattern, err)
					os.Exit(1)
				}
				if !matched {
					fmt.Fprintf(os.Stderr, "error: version %q does not match required pattern %q (set --no-semantic-versioning to skip)\n", version, cfg.VersionPattern)
					os.Exit(1)
				}
			} else if _, err := changelog.ParseSemver(version); err != nil {
				fmt.Fprintf(os.Stderr, "error: version %q is not a valid semantic version (set --no-semantic-versioning to skip)\n", version)
				os.Exit(1)
			}
		}
	}

	changelogMD, err := changelog.Render(dir, fragments, version, date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render error: %v\n", err)
		os.Exit(1)
	}

	outputFile := a.Output
	if outputFile == "" {
		outputFile = "CHANGELOG.md"
	}

	if err := os.WriteFile(outputFile, []byte(changelogMD), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write output %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Generated changelog for version %s (%d fragments)\n", version, len(fragments))

	// Render additional (non-markdown) templates if configured.
	// The built-in markdown changelog is already written by the direct Render() call above,
	// so skip renderTemplates when only default markdown is configured to avoid overwriting it.
	cfgPath = findConfigFile(dir)
	var tmplCfg *changelog.Config
	if _, err := os.ReadFile(cfgPath); err == nil {
		tmplCfg, _ = changelog.Load(cfgPath) // ignore error — we already validated above
	}
	hasNonMarkdown := false
	if tmplCfg != nil {
		for _, t := range tmplCfg.Templates {
			if strings.ToLower(t.Format) != "markdown" {
				hasNonMarkdown = true
				break
			}
		}
	}
	if hasNonMarkdown {
		if err := renderTemplates(a.DirRoot, fragments, version, date, a.Codename); err != nil {
			fmt.Fprintf(os.Stderr, "template render error: %v\n", err)
			os.Exit(1)
		}
	}

	saveChangesMetadata(dir, fragments, version, date)

	// Cleanup or archive processed fragment files.
	outputBase := ""
	if a.Output != "" {
		outputBase = filepath.Base(a.Output)
	}
	// Pass the actual fragments dir used for this release, and look for config in both changelog.yaml (new) and config.yaml (legacy).
	if err := cleanupOrArchiveFragments(dir, fragmentsDir, outputBase); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup warning: %v\n", err)
	}
}

// saveChangesMetadata persists release metadata to .changes.yaml in abc-compatible nested format.
func saveChangesMetadata(changelogDir string, fragments []*changelog.Fragment, version, date string) error {
	// Load existing .changes.yaml if present (abc compatible nested format).
	changesPath := filepath.Join(changelogDir, ".changes.yaml")
	var existingReleases map[string]struct {
		Changes   map[string][]string `yaml:"changes"`
		Date      string              `yaml:"release_date"`
		Fragments []string            `yaml:"fragments,omitempty"`
	}

	if data, err := os.ReadFile(changesPath); err == nil && len(data) > 0 {
		var wrapper struct {
			Releases map[string]struct {
				Changes   map[string][]string `yaml:"changes"`
				Date      string              `yaml:"release_date"`
				Fragments []string            `yaml:"fragments,omitempty"`
			} `yaml:"releases"`
		}
		if yaml.Unmarshal(data, &wrapper) == nil {
			existingReleases = wrapper.Releases
		}
	}

	if existingReleases == nil {
		existingReleases = make(map[string]struct {
			Changes   map[string][]string `yaml:"changes"`
			Date      string              `yaml:"release_date"`
			Fragments []string            `yaml:"fragments,omitempty"`
		})
	}

	// Build changes and fragment list for this version.
	changes := make(map[string][]string)
	var fragNames []string
	for _, f := range fragments {
		if len(f.Bugfixes) > 0 {
			for _, item := range f.Bugfixes {
				changes["bugfixes"] = append(changes["bugfixes"], item.String())
			}
		}
		if len(f.MajorChanges) > 0 {
			for _, item := range f.MajorChanges {
				changes["major_changes"] = append(changes["major_changes"], item.String())
			}
		}
		if len(f.MinorChanges) > 0 {
			for _, item := range f.MinorChanges {
				changes["minor_changes"] = append(changes["minor_changes"], item.String())
			}
		}
		if len(f.BreakingChanges) > 0 {
			for _, item := range f.BreakingChanges {
				changes["breaking_changes"] = append(changes["breaking_changes"], item.String())
			}
		}
		if len(f.DeprecatedFeatures) > 0 {
			for _, item := range f.DeprecatedFeatures {
				changes["deprecated_features"] = append(changes["deprecated_features"], item.String())
			}
		}
		if len(f.RemovedFeatures) > 0 {
			for _, item := range f.RemovedFeatures {
				changes["removed_features"] = append(changes["removed_features"], item.String())
			}
		}
		if len(f.SecurityFixes) > 0 {
			for _, item := range f.SecurityFixes {
				changes["security_fixes"] = append(changes["security_fixes"], item.String())
			}
		}
		if len(f.KnownIssues) > 0 {
			for _, item := range f.KnownIssues {
				changes["known_issues"] = append(changes["known_issues"], item.String())
			}
		}
		if f.Filename != "" {
			fragNames = append(fragNames, filepath.Base(f.Filename))
		}
	}

	existingReleases[version] = struct {
		Changes   map[string][]string `yaml:"changes"`
		Date      string              `yaml:"release_date"`
		Fragments []string            `yaml:"fragments,omitempty"`
	}{
		Changes:   changes,
		Date:      date,
		Fragments: fragNames,
	}

	// Save in abc-compatible nested format.
	type releaseEntry struct {
		Changes   map[string][]string `yaml:"changes"`
		Date      string              `yaml:"release_date"`
		Fragments []string            `yaml:"fragments,omitempty"`
	}
	wrapper := struct {
		Ancestor any                     `yaml:"ancestor"` // null for non-collection projects
		Releases map[string]releaseEntry `yaml:"releases"`
	}{
		Ancestor: nil,
		Releases: make(map[string]releaseEntry),
	}

	for ver, entry := range existingReleases {
		wrapper.Releases[ver] = releaseEntry{
			Changes:   entry.Changes,
			Date:      entry.Date,
			Fragments: entry.Fragments,
		}
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshal changes metadata: %w", err)
	}

	if err := os.WriteFile(changesPath, data, 0o644); err != nil {
		return fmt.Errorf("write changes metadata %s: %w", changesPath, err)
	}

	fmt.Printf("Saved release metadata to %s\n", changesPath)
	return nil
}

// cleanupOrArchiveFragments either deletes or moves fragments based on config.
func cleanupOrArchiveFragments(changelogDir string, fragmentsDir string, skipOutputFile string) error {
	keepFragments := false
	archiveTemplate := ""

	// Read from config.yaml (standard).
	cfgPath := filepath.Join(changelogDir, "config.yaml")
	if data, err := os.ReadFile(cfgPath); err == nil {
		var cfg struct {
			KeepFragments       bool   `yaml:"keep_fragments"`
			ArchivePathTemplate string `yaml:"archive_path_template"`
		}
		if yaml.Unmarshal(data, &cfg) == nil {
			keepFragments = cfg.KeepFragments
			archiveTemplate = cfg.ArchivePathTemplate
		}
	}

	if keepFragments {
		return nil // nothing to clean up
	}

	entries, err := os.ReadDir(fragmentsDir)
	if err != nil {
		return nil // directory might be empty already
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".md")) {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue // skip hidden files
		}

		// Skip the output file if it matches.
		if name == skipOutputFile {
			continue
		}

		srcPath := filepath.Join(fragmentsDir, name)

		if archiveTemplate != "" {
			// Move to archive directory.
			archiveDir := filepath.Join(changelogDir, "archive", time.Now().Format("2006-01-02"))
			if err := os.MkdirAll(archiveDir, 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "warning: cannot create archive dir %s: %v\n", archiveDir, err)
				continue
			}
			dstPath := filepath.Join(archiveDir, name)
			if err := os.Rename(srcPath, dstPath); err != nil {
				// Fall back to delete if rename fails.
				os.Remove(srcPath)
			} else {
				fmt.Printf("Archived fragment: %s -> %s\n", name, archiveDir)
			}
		} else {
			if err := os.Remove(srcPath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove fragment %s: %v\n", srcPath, err)
			} else {
				fmt.Printf("Removed fragment: %s\n", name)
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// runGenerate — Regenerate changelog from stored .changes.yaml metadata.
// Mirrors: generate [VERSION] [--output PATH] [--only-latest] [--output-format FMT]
// ---------------------------------------------------------------------------
func (a *args) runGenerate() {
	dir := a.DirRoot
	if dir == "" {
		d, err := changelog.FindChangelogDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not find changelog directory\n")
			os.Exit(1)
		}
		dir = d
	}

	changesPath := filepath.Join(dir, ".changes.yaml")
	data, err := os.ReadFile(changesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no changes metadata found at %s (run 'changelog release' first)\n", changesPath)
		os.Exit(1)
	}

	var entry changelog.ChangeEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing changes metadata: %v\n", err)
		os.Exit(1)
	}

	version := a.GenVersion
	if version == "" {
		version = entry.Version
	} else {
		version = strings.TrimPrefix(version, "v") // abc compatibility
	}
	date := entry.Date

	fragments, err := changelog.MergeCollect(filepath.Join(dir, "fragments"), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error collecting fragments: %v\n", err)
		os.Exit(1)
	}

	var changelogMD string
	if a.OnlyLatest {
		// Only render the latest/current version without preamble of earlier versions.
		changelogMD, err = changelog.Render(dir, fragments, version, date)
	} else {
		// Render full multi-version changelog with ancestor history.
		changelogMD, err = changelog.RenderFull(dir, fragments, version, date)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "render error: %v\n", err)
		os.Exit(1)
	}

	outputFile := a.OutputFile
	if outputFile == "" {
		outputFile = "CHANGELOG.md"
	}

	if err := os.WriteFile(outputFile, []byte(changelogMD), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write output %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	if a.OnlyLatest {
		fmt.Printf("Regenerated changelog for version %s at %s\n", version, outputFile)
	} else {
		fmt.Printf("Regenerated full changelog (all versions) at %s\n", outputFile)
	}
}

// ---------------------------------------------------------------------------
// runReformat — Re-save .changes.yaml with consistent formatting.
// Mirrors: reformat
// ---------------------------------------------------------------------------
func (a *args) runReformat() {
	dir := a.DirRoot
	if dir == "" {
		d, err := changelog.FindChangelogDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not find changelog directory\n")
			os.Exit(1)
		}
		dir = d
	}

	changesPath := filepath.Join(dir, ".changes.yaml")
	data, err := os.ReadFile(changesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no changes metadata found at %s\n", changesPath)
		os.Exit(1)
	}

	var entry changelog.ChangeEntry
	if err := yaml.Unmarshal(data, &entry); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing changes metadata: %v\n", err)
		os.Exit(1)
	}

	formatted, err := yaml.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error formatting changes metadata: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(changesPath, formatted, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write reformatted file %s: %v\n", changesPath, err)
		os.Exit(1)
	}

	fmt.Printf("Reformatted changelog metadata at %s\n", changesPath)
}

// ---------------------------------------------------------------------------
// runCollectionNew — Generate Ansible collection plugin docs for changelogs.
// Mirrors: collection-new (Go implementation).
// ---------------------------------------------------------------------------
func (a *args) runCollectionNew() {
	dir := a.DirRoot
	if dir == "" {
		d, err := changelog.FindChangelogDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: could not find changelog directory\n")
			os.Exit(1)
		}
		dir = d
	}

	fragmentsDir := filepath.Join(dir, "fragments")
	if _, err := os.Stat(fragmentsDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: fragments directory not found at %s\n", fragmentsDir)
		os.Exit(1)
	}

	// Read existing .changes.yaml or create empty one.
	changesPath := filepath.Join(dir, ".changes.yaml")
	var existingEntry *changelog.ChangeEntry

	if data, err := os.ReadFile(changesPath); err == nil && len(data) > 0 {
		existingEntry = &changelog.ChangeEntry{}
		yaml.Unmarshal(data, existingEntry)
	} else {
		existingEntry = &changelog.ChangeEntry{Changes: make(map[string][]string)}
	}

	fmt.Println("Collection plugin documentation scan complete.")
	fmt.Printf("  Metadata file: %s\n", changesPath)
	fmt.Printf("  Total entries: %d sections tracked\n", len(existingEntry.Changes))

	if err := os.WriteFile(changesPath, []byte{}, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write metadata file: %v\n", err)
	}
}

// ---------------------------------------------------------------------------
// runPreview — Render fragments to stdout without writing file.
// ---------------------------------------------------------------------------
func (a *args) runPreview() {
	dir := a.DirRoot
	if dir == "" {
		d, err := changelog.FindChangelogDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "preview error: %v\n", err)
			os.Exit(1)
		}
		dir = d
	}

	fragments, err := changelog.MergeCollect(filepath.Join(dir, "fragments"), a.NotesDirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect error: %v\n", err)
		os.Exit(1)
	}

	date := time.Now().Format("2006-01-02")
	changelogMD, err := changelog.Render(dir, fragments, "preview", date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(changelogMD)
}

// ---------------------------------------------------------------------------
// runLog — Show changelog in git-log style.
// ---------------------------------------------------------------------------
func (a *args) runLog() {
	dir := a.DirRoot
	if dir == "" {
		fragmentsDir, err := changelog.FindFragmentsDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "log error: %v\n", err)
			os.Exit(1)
		}
		dir = fragmentsDir
	}

	pageSize := a.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	fragments, err := changelog.Collect(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect error: %v\n", err)
		os.Exit(1)
	}

	var logErr error
	if len(fragments) > pageSize {
		logErr = changelog.PrintLogInteractiveFromDir(dir, pageSize)
	} else {
		logErr = changelog.PrintLogDirect(dir, pageSize)
	}

	if logErr != nil {
		fmt.Fprintf(os.Stderr, "log error: %v\n", logErr)
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// runBump — Bump VERSION.txt from bump label.
// ---------------------------------------------------------------------------
func (a *args) runBump() {
	level, isRc, err := a.resolveBumpLabel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bump error: %v\n", err)
		os.Exit(1)
	}

	cur, err := changelog.ReadVersionFile(a.VersionFile)
	if err != nil {
		// VERSION.txt doesn't exist — fall back to git tags for latest version.
		fmt.Fprintf(os.Stderr, "no %s found; deriving current version from git tags\n", a.VersionFile)
		cur, err = latestGitTag()
		if err != nil {
			fmt.Fprintf(os.Stderr, "no prior versions found in git history: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found latest version from tags: v%s\n", cur.String())
	}

	var newVer *changelog.SemVer
	switch {
	case isRc && cur.Prerelease != "":
		newVer, err = cur.BumpRC(level, "")
	case isRc:
		fmt.Fprintln(os.Stderr, "error: bumping to RC requires current version to be pre-release")
		os.Exit(1)
	default:
		if cur.Prerelease != "" {
			// Current version is an RC — finalize it to the next release level.
			newVer, err = cur.BumpToFinal(level)
		} else {
			// Regular release bump — apply the specified level (major/minor/patch).
			newVer, err = cur.Bump(level)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "bump error: %v\n", err)
		os.Exit(1)
	}

	if err := changelog.ValidateNewVersion(newVer, ""); err != nil {
		fmt.Fprintf(os.Stderr, "version validation: %v\n", err)
		os.Exit(1)
	}

	versionStr := newVer.PrefixedString() // includes 'v' prefix for VERSION.txt (e.g., "v1.3.0")
	if err := os.WriteFile(a.VersionFile, []byte(versionStr+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write version file: %v\n", err)
		os.Exit(1)
	}

	// CreateTag already prepends 'v', so pass bare version string (e.g., "1.3.0")
	tagName := newVer.String() // no prefix - CreateTag adds it internally
	if err := changelog.CreateTag(tagName); err != nil {
		fmt.Fprintf(os.Stderr, "create git tag %s: %v\n", tagName, err)
		os.Exit(1)
	}

	fmt.Printf("Bumped to %s\n", versionStr)
}

// latestGitTag queries the git repository for the highest semver tag and returns it as a SemVer.
// It lists all tags matching 'v*' pattern, parses them, and returns the maximum version found.
func latestGitTag() (*changelog.SemVer, error) {
	// Get all tags sorted by version (descending), take the first valid one.
	cmd := exec.Command("git", "tag", "--list", "v*", "--sort=-version:refname")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list git tags: %w", err)
	}

	tags := strings.Split(strings.TrimSpace(string(out)), "\n")
	var best *changelog.SemVer
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		// Strip leading 'v' prefix for parsing.
		versionStr := strings.TrimPrefix(tag, "v")
		sv, err := changelog.ParseSemver(versionStr)
		if err != nil {
			continue // skip non-semver tags like v0.x.y.z or invalid formats
		}
		if best == nil || isGreaterVersion(sv, best) {
			best = sv
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no valid semver tags found in repository")
	}

	return best, nil
}

// isGreaterVersion compares two SemVer structs and returns true if a > b.
func isGreaterVersion(a, b *changelog.SemVer) bool {
	if a.Major != b.Major {
		return a.Major > b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor > b.Minor
	}
	if a.Patch != b.Patch {
		return a.Patch > b.Patch
	}
	// If both have prerelease, compare them alphabetically.
	// Empty prerelease means release (higher than any prerelease).
	if a.Prerelease == "" && b.Prerelease != "" {
		return true
	}
	if a.Prerelease != "" && b.Prerelease == "" {
		return false
	}
	return a.Prerelease > b.Prerelease
}

// resolveBumpLabel determines the bump level from CLI flag or env vars.
func (a *args) resolveBumpLabel() (level string, isRc bool, err error) {
	cliLevel := a.BumpLabel
	if cliLevel == "" {
		cliLevel = os.Getenv("BUMP_LABEL")
	}
	return changelog.ResolveBumpLabel(cliLevel)
}

// ---------------------------------------------------------------------------
// runLicense — Print the embedded project license.
// ---------------------------------------------------------------------------
func (a *args) runLicense() {
	fmt.Print(licenseText())
}

// ---------------------------------------------------------------------------
// runLintChangelogYaml — Validate changelog.yaml syntax and fields.
// Mirrors: lint-changelog-yaml [--strict] [--no-semantic-versioning]
// ---------------------------------------------------------------------------
func (a *args) runLintChangelogYaml() {
	var path string
	if a.DirRoot != "" && a.DirRoot != "." {
		path = a.DirRoot
	} else if len(positionalArg) > 0 {
		path = positionalArg
	} else {
		cwd, _ := os.Getwd()
		path = findConfigFile(cwd)
		if path == "" {
			fmt.Fprintln(os.Stderr, "error: no config.yaml or changelog.yaml found in current directory")
			os.Exit(1)
		}
	}

	errors := changelog.LintChangelogYaml(path, a.Strict, a.NoSemanticVersioning)
	if len(errors) > 0 {
		for _, e := range errors {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}
	fmt.Printf("%s is valid\n", filepath.Base(path))
}

// positionalArg holds the remaining positional argument from go-flags parser.
var positionalArg string
