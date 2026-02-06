package watchdog

import (
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/sirupsen/logrus"
)

// ExtractIncrement extracts new content from afterSnapshot by comparing it with beforeSnapshot
//
// Strategy: Sequence Alignment
// Finds the longest common sequence between the content of 'before' and the start of 'after'.
// This identifies where the 'before' content ends (or diverged) in 'after'.
//
// This is robust against:
// - Scrolling (lines moving up)
// - Disappearing menus (divergence in sequence)
// - Repeated lines (sequence context distinguishes them)
func ExtractIncrement(afterSnapshot, beforeSnapshot string) string {
	afterLines := strings.Split(afterSnapshot, "\n")
	beforeLines := strings.Split(beforeSnapshot, "\n")

	// Filter out empty snapshots
	if len(afterLines) == 0 {
		return ""
	}

	if len(beforeLines) == 0 {
		return cleanIncrementalContent(afterSnapshot)
	}

	bestMatchLen := 0

	// Iterate through all possible start positions in 'before'
	// to find where 'after' lines begin matching.
	for k := 0; k < len(beforeLines); k++ {
		matchLen := 0
		for i := 0; i < len(afterLines) && k+i < len(beforeLines); i++ {
			// Use StripANSI and TrimSpace for robust comparison (ignoring color/whitespace changes)
			aClean := strings.TrimSpace(StripANSI(afterLines[i]))
			bClean := strings.TrimSpace(StripANSI(beforeLines[k+i]))
			if aClean == bClean {
				matchLen++
			} else {
				break
			}
		}

		if matchLen > bestMatchLen {
			bestMatchLen = matchLen
		}
	}

	// Calculate result based on the best match
	if bestMatchLen < len(afterLines) {
		newLines := afterLines[bestMatchLen:]
		result := strings.Join(newLines, "\n")

		logger.WithFields(logrus.Fields{
			"best_match_len":  bestMatchLen,
			"new_lines_count": len(newLines),
		}).Debug("extracted-increment-by-alignment")

		return cleanIncrementalContent(result)
	}

	// If matchLen covers all of afterLines, it means afterLines is a subset of beforeLines.
	// No new content.
	logger.Debug("after-snapshot-is-subset-of-before-returning-empty")
	return ""
}

// cleanIncrementalContent removes multiple consecutive newlines, UI status lines, and UI border lines
// This is the final cleanup step before returning content to user
func cleanIncrementalContent(content string) string {
	// Split into lines for processing
	lines := strings.Split(content, "\n")
	var filteredLines []string

	for _, line := range lines {
		// Skip UI border lines (e.g., "──────", "══════")
		if canSkip(line) {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	// Rejoin content
	content = strings.Join(filteredLines, "\n")

	// Remove UI status lines (called AFTER thinking check)
	content = RemoveUIStatusLines(content)

	return content
}
