package watchdog

import (
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

// hasPromptCharacterPrefix checks if a line has a cursor prefix
func hasPromptCharacterPrefix(line string) bool {
	prefix := []string{
		"> ",
		"❯ ",
		">>>",
	}
	for _, pattern := range prefix {
		if strings.HasPrefix(line, pattern) {
			return true
		}
	}
	return false
}

// eqPromptCharacterData checks if line exactly matches prefix + userPrompt
func eqPromptCharacterData(line string, userPrompt string) bool {
	prefix := []string{
		"> ",
		"❯ ",
		">>>",
	}
	for _, pattern := range prefix {
		if line == pattern+userPrompt {
			return true
		}
	}
	return false
}

// isLikelyUserPromptLine checks if a line is likely to be the user's prompt input
// rather than AI-generated content containing the same keywords
func isLikelyUserPromptLine(line, userPrompt string) bool {
	// Priority 1: Lines with cursor prefix (most reliable indicator)
	if strings.HasPrefix(line, "❯ ") && strings.Contains(line, userPrompt) {
		logger.WithFields(logrus.Fields{
			"line":        line,
			"user_prompt": userPrompt,
			"reason":      "has cursor prefix",
		}).Debug("accepting-line-has-cursor-prefix")
		return true
	}

	// Priority 2: Exact match (for cases without cursor prefix)
	if line == userPrompt {
		logger.WithFields(logrus.Fields{
			"line":        line,
			"user_prompt": userPrompt,
			"reason":      "exact match",
		}).Debug("accepting-line-exact-match")
		return true
	}

	// Reject all other cases (including AI responses like "test content follows")
	logger.WithFields(logrus.Fields{
		"line":       line,
		"user_prompt": userPrompt,
		"line_len":   len(line),
		"prompt_len": len(userPrompt),
		"reason":     "no cursor prefix and not exact match",
	}).Debug("rejecting-line-does-not-look-like-user-prompt")

	return false
}

// isPromptOrCommand checks if a line is a prompt/command rather than assistant output
func isPromptOrCommand(line string) bool {
	promptPatterns := []string{
		"user@",
		"$ ",
		">>>",
		"...",
		"[?]",
		"Press Enter",
		"Confirm?",
	}

	for _, pattern := range promptPatterns {
		if strings.Contains(line, pattern) {
			return true
		}
	}

	return false
}

// canSkip checks if a line should be skipped (empty or UI border)
func canSkip(line string) bool {
	if line == "" {
		return true
	}
	// Detect and skip UI borders (box drawing characters)
	for _, runeValue := range line {
		if strings.ContainsRune("─│┌└┐┘├┤ ╭╮╰╯_", runeValue) {
			continue
		}
		return false
	}
	return true
}

// extractLastAssistantContent extracts the last meaningful assistant response from tmux output
// Filters out UI borders, prompts, and system messages
func extractLastAssistantContent(output string) string {
	lines := strings.Split(output, "\n")

	// Filter out borders and UI elements
	var contentLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if canSkip(trimmed) {
			continue
		}

		// Skip prompts and commands
		if isPromptOrCommand(trimmed) {
			continue
		}

		contentLines = append(contentLines, trimmed)
	}

	// Join with newlines
	result := strings.Join(contentLines, "\n")

	// Remove duplicate consecutive blank lines
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// PromptMatcher handles finding and extracting content after user prompts
type PromptMatcher struct {
	userPrompt   string
	promptPrefix string
	promptLen    int
}

// NewPromptMatcher creates a new PromptMatcher instance
func NewPromptMatcher(userPrompt string) *PromptMatcher {
	promptPrefix := userPrompt
	if len(userPrompt) > constants.MaxPromptPrefixLength {
		promptPrefix = userPrompt[:constants.MaxPromptPrefixLength]
	}
	return &PromptMatcher{
		userPrompt:   userPrompt,
		promptPrefix: promptPrefix,
		promptLen:    constants.MaxPromptPrefixLength,
	}
}

// findPromptIndex searches backwards to find the last occurrence of the user prompt
func (pm *PromptMatcher) findPromptIndex(lines []string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}

		// Handle empty userPrompt case
		if pm.userPrompt == "" {
			if hasPromptCharacterPrefix(trimmed) {
				return i
			}
			continue
		}

		// Priority 1: Exact match (most reliable)
		if trimmed == pm.userPrompt || eqPromptCharacterData(trimmed, pm.userPrompt) {
			logger.WithFields(logrus.Fields{
				"line_index":     i,
				"prompt_matched": trimmed,
				"match_type":     "exact",
			}).Debug("found-user-prompt-exact-match")
			return i
		}

		// Priority 2: Match with cursor prefix
		if hasPromptCharacterPrefix(trimmed) && strings.Contains(trimmed, pm.userPrompt) {
			if isLikelyUserPromptLine(trimmed, pm.userPrompt) {
				logger.WithFields(logrus.Fields{
					"line_index":     i,
					"prompt_matched": trimmed,
					"match_type":     "cursor_prefix",
				}).Debug("found-user-prompt-with-cursor-prefix")
				return i
			}
		}

		// Priority 3: Partial match (fallback, but with validation)
		if strings.Contains(trimmed, pm.userPrompt) {
			if isLikelyUserPromptLine(trimmed, pm.userPrompt) {
				logger.WithFields(logrus.Fields{
					"line_index":     i,
					"prompt_matched": trimmed,
					"match_type":     "partial",
				}).Debug("found-user-prompt-partial-match-with-validation")
				return i
			}
		}

		// Priority 4: Prefix match for long prompts
		if len(pm.userPrompt) > pm.promptLen && strings.Contains(trimmed, pm.promptPrefix) {
			if isLikelyUserPromptLine(trimmed, pm.userPrompt) {
				logger.WithFields(logrus.Fields{
					"line_index":            i,
					"prompt_matched_prefix": trimmed,
					"match_type":            "prefix",
				}).Debug("found-user-prompt-prefix-match-with-validation")
				return i
			}
		}
	}
	return -1
}

// extractContent collects content lines after the prompt index
func (pm *PromptMatcher) extractContent(lines []string, promptIndex int) []string {
	var contentLines []string
	for i := promptIndex + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if canSkip(trimmed) {
			continue
		}
		if isPromptOrCommand(trimmed) {
			continue
		}
		contentLines = append(contentLines, trimmed)
	}
	return contentLines
}

// cleanContent removes multiple consecutive newlines
func cleanContent(content string) string {
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}
	return content
}

// ExtractContentAfterPrompt extracts content appearing after the user's prompt
// Searches from the END to find the LAST occurrence of the prompt
// This filters out historical messages and only returns the latest response
func ExtractContentAfterPrompt(tmuxOutput, userPrompt string) string {
	lines := strings.Split(tmuxOutput, "\n")
	matcher := NewPromptMatcher(userPrompt)

	promptIndex := matcher.findPromptIndex(lines)

	// If prompt not found, fall back to basic extraction
	if promptIndex == -1 {
		logger.Debug("user-prompt-not-found-in-tmux-output-using-basic-extraction")
		return extractLastAssistantContent(tmuxOutput)
	}

	contentLines := matcher.extractContent(lines, promptIndex)

	logger.WithFields(logrus.Fields{
		"total_lines":  len(lines),
		"prompt_index": promptIndex,
		"content_lines": len(contentLines),
	}).Debug("extracted-content-after-prompt")

	if len(contentLines) == 0 {
		logger.Debug("no-content-found-after-prompt-using-basic-extraction")
		return extractLastAssistantContent(tmuxOutput)
	}

	result := strings.Join(contentLines, "\n")
	return cleanContent(result)
}

// IsThinking checks if AI CLI is still thinking based on tmux output
// Uses universal keywords that work across different AI CLI tools
// Only checks the last N lines to accurately determine current state
func IsThinking(output string) bool {
	// Only check the last N lines for accurate current state
	lines := strings.Split(output, "\n")
	startIndex := 0
	if len(lines) > constants.ThinkingCheckLines {
		startIndex = len(lines) - constants.ThinkingCheckLines
	}
	recentLines := lines[startIndex:]

	// Universal thinking indicators (work across Claude, Gemini, etc.)
	thinkingIndicators := []string{
		"thinking",
		"esc to interrupt",
		"press escape to interrupt",
		"interrupt",
	}

	recentOutput := strings.Join(recentLines, "\n")
	outputLower := strings.ToLower(recentOutput)

	for _, indicator := range thinkingIndicators {
		if strings.Contains(outputLower, indicator) {
			logger.WithFields(logrus.Fields{
				"indicator":     indicator,
				"checked_lines": len(recentLines),
				"total_lines":   len(lines),
			}).Debug("detected-thinking-state-in-recent-lines")
			return true
		}
	}

	return false
}

// isUIStatusLine checks if a line is a UI status line
// UI status lines include indicators like "running stop hook", "esc to interrupt", etc.
func isUIStatusLine(line string) bool {
	// UI status line patterns
	uiPatterns := []string{
		"Undulating…",
		"running stop hook",
		"esc to interrupt",
		"press escape",
		"? for shortcuts",
	}

	lowerLine := strings.ToLower(line)
	for _, pattern := range uiPatterns {
		if strings.Contains(lowerLine, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for single-character cursor indicators
	if line == "❯" || line == ">" || line == "$" {
		return true
	}

	return false
}

// RemoveUIStatusLines removes UI status lines from the response
// This should be called AFTER IsThinking() check, when response is ready to send to user
func RemoveUIStatusLines(output string) string {
	lines := strings.Split(output, "\n")
	var filteredLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Remove UI status lines
		if isUIStatusLine(trimmed) {
			logger.WithField("line", trimmed).Debug("removing-ui-status-line-from-response")
			continue
		}

		filteredLines = append(filteredLines, line)
	}

	result := strings.Join(filteredLines, "\n")
	return cleanContent(result)
}

// ExtractLastAssistantContent is a public wrapper for extractLastAssistantContent
func ExtractLastAssistantContent(output string) string {
	return extractLastAssistantContent(output)
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
