package changelog

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LogEntry holds the lightweight metadata for one fragment in log output.
type LogEntry struct {
	Frag    *Fragment // parsed fragment data
	Author  string    // detected via git blame or from frontmatter author field
	PageIdx int       // position among all entries (0-based)
}

// CollectLogEntries reads all fragments, sorts by date descending (newest first),
// detects authors via git blame when available, and returns log entries.
func CollectLogEntries(dir string) ([]*LogEntry, error) {
	fragments, err := Collect(dir)
	if err != nil {
		return nil, fmt.Errorf("collect fragments for log: %w", err)
	}

	sortFragmentsDesc(fragments)

	var entries []*LogEntry
	for i, frag := range fragments {
		entry := &LogEntry{
			Frag:    frag,
			PageIdx: i,
		}

		// Check frontmatter for explicit author field.
		if frag.Author != "" {
			entry.Author = frag.Author
		} else {
			// Try git blame as fallback.
			fullPath := filepath.Join(dir, frag.Filename)
			entry.Author = detectAuthor(fullPath)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// PrintLogDirect displays paginated log output without interactive prompts (all pages shown).
func PrintLogDirect(dir string, pageSize int) error {
	fragments, err := Collect(dir)
	if err != nil {
		return fmt.Errorf("collect for direct display: %w", err)
	}

	sortFragmentsDesc(fragments)

	totalPages := 1
	if len(fragments) > 0 {
		totalPages = (len(fragments) + pageSize - 1) / pageSize // ceiling division
	}

	pageNum := 1

	for pageNum <= totalPages {
		start := (pageNum - 1) * pageSize
		end := start + pageSize
		if end > len(fragments) {
			end = min(len(fragments), end)
		}

		printPageHeader(pageNum, totalPages)
		printLogTable(fragments[start:end], pageNum)

		pageNum++
	}

	return nil
}

// PrintLogInteractiveFromDir displays interactive paginated log from directory.
func PrintLogInteractiveFromDir(dir string, pageSize int) error {
	fragments, err := Collect(dir)
	if err != nil {
		return fmt.Errorf("collect for interactive display: %w", err)
	}

	sortFragmentsDesc(fragments)

	totalPages := 1
	if len(fragments) > 0 {
		totalPages = (len(fragments) + pageSize - 1) / pageSize // ceiling division
	}

	pageNum := 1
	for pageNum <= totalPages {
		start := (pageNum - 1) * pageSize
		end := start + pageSize
		if end > len(fragments) {
			end = min(len(fragments), end)
		}

		fmt.Printf("\n=== Page %d of %d (%d total fragments) ===\n", pageNum, totalPages, len(fragments))
		for _, f := range fragments[start:end] {
			dateStr := f.Date
			if dateStr == "" {
				dateStr = "(no date)"
			}
			bodyPreview := shortenBody(f.Body, 60)
			fmt.Printf("  %s |  %s | %-13s | %s\n", dateStr, f.Issue, "["+f.Category+"]", bodyPreview)
		}

		pageNum++
	}

	return nil
}

// detectAuthor runs `git blame --porcelain` on the given file and extracts the author name.
// Returns empty string if not in a git repo or if git blame fails.
func detectAuthor(filePath string) string {
	cmd := exec.Command("git", "blame", "--porcelain", filePath)
	output, err := cmd.Output()
	if err != nil {
		return "" // not a git repo or other error — author is optional
	}

	for line := range strings.SplitSeq(string(output), "\n") {
		// Porcelain format: "hash linenum origin-lines [num-lines] final-rev (Author Name YYYY-MM-DD num-of-lines)"
		if after, ok := strings.CutPrefix(line, "author "); ok {
			return after
		}
	}

	return ""
}

func printPageHeader(pageNum, totalPages int) {
	fmt.Printf("\n--- Page %d of %d ---\n", pageNum, totalPages)
}

func printLogTable(fragments []*Fragment, pageNum int) {
	for _, f := range fragments {
		dateStr := f.Date
		if dateStr == "" {
			dateStr = "(no date)"
		}
		bodyPreview := shortenBody(f.Body, 50)
		fmt.Printf("  %s |  %s | %-13s | %s\n", dateStr, f.Issue, "["+f.Category+"]", bodyPreview)
	}
	fmt.Printf("\n[Page %d] Enter for next page, q to quit: ", pageNum)

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		if strings.TrimSpace(strings.ToLower(scanner.Text())) == "q" {
			os.Exit(0)
		}
	}
}

func shortenBody(body string, maxLen int) string {
	body = strings.TrimSpace(body)
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "..."
}
