package watchdog

import (
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ExtractIncrement extracts new content from afterSnapshot by comparing it with beforeSnapshot
//
// Core principle: Focus on "what's new at the end of after" rather than "what comes after before"
// This handles scrolling, content replacement, and disappearing menus gracefully.
//
// Parameters:
//   - afterSnapshot: tmux output captured after user input
//   - beforeSnapshot: tmux output captured before user input
//
// Returns:
//   - Newly added content (may be empty if no new content)
//
// Strategy:
// 1. Take last N lines from after as candidate (handles scrolling/disappearing content)
// 2. Filter out lines that existed in before (removes duplicates)
// 3. If that yields nothing, fall back to line-by-line diff
func ExtractIncrement(afterSnapshot, beforeSnapshot string) string {
	afterLines := strings.Split(afterSnapshot, "\n")
	beforeLines := strings.Split(beforeSnapshot, "\n")

	// Filter out empty snapshots
	if len(afterLines) == 0 {
		logger.Debug("after-snapshot-empty-returning-empty")
		return ""
	}

	// Strategy 1: Tail heuristic (95% of cases)
	// New content always appears at the end of after
	candidate := extractTailContent(afterLines, constants.IncrementalSnapshotTailLines)

	// Filter out before content from candidate
	filtered := filterOutBeforeContent(candidate, beforeSnapshot, beforeLines)

	if filtered != "" {
		logger.WithFields(logrus.Fields{
			"candidate_len":    len(candidate),
			"filtered_len":     len(filtered),
			"after_lines":      len(afterLines),
			"before_lines":     len(beforeLines),
			"strategy":         "tail_heuristic",
		}).Debug("extracted-increment-using-tail-heuristic")

		return cleanIncrementalContent(filtered)
	}

	// Strategy 2: Line-by-line diff (fallback)
	logger.Debug("tail-heared-short-yielded-no-results-falling-back-to-line-diff")

	result := extractByLineDiff(afterLines, beforeLines)

	logger.WithFields(logrus.Fields{
		"result_len":   len(result),
		"after_lines":  len(afterLines),
		"before_lines": len(beforeLines),
		"strategy":     "line_diff",
	}).Debug("extracted-increment-using-line-diff")

	return cleanIncrementalContent(result)
}

// extractTailContent takes the last N lines from after snapshot
// This is the primary extraction strategy as new content always appears at the end
func extractTailContent(afterLines []string, count int) string {
	if len(afterLines) == 0 {
		return ""
	}

	startIdx := len(afterLines) - count
	if startIdx < 0 {
		startIdx = 0
	}

	return strings.Join(afterLines[startIdx:], "\n")
}

// filterOutBeforeContent removes lines from candidate that existed in beforeSnapshot
//
// This handles the case where before content has scrolled away but we still
// want to avoid duplicates in the extracted content.
func filterOutBeforeContent(candidate, beforeSnapshot string, beforeLines []string) string {
	if candidate == "" {
		return ""
	}

	// Build a set of lines that exist in before
	// Use trimmed lines for comparison to handle whitespace differences
	beforeLineSet := make(map[string]bool)
	for _, line := range beforeLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			beforeLineSet[trimmed] = true
		}
	}

	// Filter candidate lines
	candidateLines := strings.Split(candidate, "\n")
	var resultLines []string

	for _, line := range candidateLines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Keep line if it doesn't exist in before
		if !beforeLineSet[trimmed] {
			resultLines = append(resultLines, line)
		}
	}

	result := strings.Join(resultLines, "\n")

	// If filtering removed everything, return empty
	// This will trigger the fallback line-diff strategy
	if result == "" && len(candidateLines) > 0 {
		logger.Debug("filter-removed-all-candidates-returning-empty")
	}

	return result
}

// extractByLineDiff performs line-by-line diff between after and before
//
// This is a fallback strategy for edge cases where tail heuristic doesn't work.
// It finds the first line in after that doesn't match any line in before.
func extractByLineDiff(afterLines, beforeLines []string) string {
	// Handle empty before
	if len(beforeLines) == 0 {
		// No before snapshot, return all non-empty after content
		var nonEmpty []string
		for _, line := range afterLines {
			if strings.TrimSpace(line) != "" {
				nonEmpty = append(nonEmpty, line)
			}
		}
		return strings.Join(nonEmpty, "\n")
	}

	// Build a set of before lines for quick lookup
	beforeSet := make(map[string]bool)
	for _, line := range beforeLines {
		beforeSet[strings.TrimSpace(line)] = true
	}

	// Find the first line in after that's not in before
	// This handles scrolling where before content has disappeared
	startIdx := -1
	for i, line := range afterLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !beforeSet[trimmed] {
			// Found a line that's not in before
			// But we need to check if this is the start of NEW content
			// or just content that scrolled away

			// Heuristic: if we're near the end of after, this is likely new content
			if i > len(afterLines)/constants.IncrementalSnapshotMinimumStartRatio {
				startIdx = i
				break
			}
		}
	}

	// If we couldn't find a clear start point, take last 20% of lines
	if startIdx == -1 {
		startIdx = len(afterLines) * constants.IncrementalSnapshotFallbackRatio / constants.IncrementalSnapshotFallbackDenominator
	}

	// Extract content from startIdx
	var contentLines []string
	for i := startIdx; i < len(afterLines); i++ {
		line := afterLines[i]

		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			contentLines = append(contentLines, line)
		}
	}

	return strings.Join(contentLines, "\n")
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

	// Remove duplicate consecutive blank lines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	// Remove UI status lines (called AFTER thinking check)
	content = RemoveUIStatusLines(content)

	// Clean up again after removing status lines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return content
}
