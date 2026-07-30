package changelog

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindChangelogDir walks up the directory tree from CWD looking for
// changelogs/config.yaml (new, upstream-compatible), falling back to
// .changelogs/config.yaml or .changelog/config.yaml (legacy). Returns the absolute path to the project root.
func FindChangelogDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	for {
		// Check new layout first (changelogs/config.yaml).
		configPath := filepath.Join(dir, "changelogs", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil // project root is current directory
		}

		// Fall back to legacy layout (.changelogs/config.yaml).
		configPath = filepath.Join(dir, ".changelogs", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return filepath.Dir(filepath.Dir(configPath)), nil // project root is parent of .changelogs/
		}

		// Fall back to legacy layout (.changelog/config.yaml).
		configPath = filepath.Join(dir, ".changelog", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return filepath.Dir(filepath.Dir(configPath)), nil // project root is parent of .changelog/
		}

		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			return "", fmt.Errorf("no config.yaml found — run 'changelog init' first")
		}
		dir = parent
	}
}

// FindFragmentsDir returns the full path to the fragments directory.
func FindFragmentsDir() (string, error) {
	root, err := FindChangelogDir()
	if err != nil {
		return "", err
	}

	// Try new layout first (changelogs/fragments).
	newPath := filepath.Join(root, "changelogs", "fragments")
	if _, err := os.Stat(newPath); err == nil {
		return newPath, nil
	}

	// Fall back to legacy layout (.changelogs/fragments).
	oldPath := filepath.Join(root, ".changelogs", "fragments")
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath, nil
	}

	// Fall back to legacy layout (.changelog/changelog.d).
	legacyPath := filepath.Join(root, ".changelog", "changelog.d")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	}

	return "", fmt.Errorf("no fragments directory found in %s/changelogs/fragments or legacy locations", filepath.Base(root))
}

// ResolveDir checks if the given dir exists; if not and it matches a default pattern,
// walks up to find the fragments directory via FindFragmentsDir. Returns the resolved path.
func ResolveDir(dir string) (string, error) {
	if _, err := os.Stat(dir); err == nil {
		return dir, nil // dir exists — use as-is
	}

	walked, err := FindFragmentsDir()
	if err != nil {
		return "", err
	}
	return walked, nil
}

// DeleteChangelogConfig removes the config.yaml file from either layout.
func DeleteChangelogConfig() error {
	dirs := []string{"changelogs", ".changelogs", ".changelog"}
	for _, d := range dirs {
		configPath := filepath.Join(d, "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				return fmt.Errorf("delete config %s: %w", configPath, err)
			}
			fmt.Printf("Removed config: %s\n", configPath)
			return nil
		}
	}
	return fmt.Errorf("no config.yaml found")
}
