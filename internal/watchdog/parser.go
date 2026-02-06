package watchdog

import (
	"strings"

	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/pkg/constants"
	"github.com/sirupsen/logrus"
)

/*
PARSER ARCHITECTURE
==================

This file contains the original "pattern matching" parser implementation.
New implementations are in parser_incremental.go.

Parser Functions:
-----------------

1. ExtractIncrement (parser_incremental.go) - NEW (Recommended)
   - Uses "incremental snapshot" approach (after - before)
   - Simple state comparison, no complex pattern matching
   - Handles scrolling, content replacement, disappearing menus gracefully
   - Use when: You have before/after snapshots available

2. ExtractContentAfterPrompt (this file) - LEGACY (Still supported)
   - Uses "pattern matching" approach
   - Searches backward from end to find user prompt
   - Complex heuristic rules for menu detection, cursor handling
   - Use when: You don't have snapshots, only have tmux output

3. ExtractContentAfterAnyInput (this file) - LEGACY (Still supported)
   - Tries multiple user inputs from newest to oldest
   - Fallback for short inputs that don't appear in tmux output
   - Use when: Hook mode fallback without incremental extraction

Migration Path:
--------------
Polling mode → Use ExtractIncrement (with before/after snapshots)
Hook mode   → Try ExtractIncrement first, fallback to ExtractContentAfterPrompt

The old functions are kept for:
- Backward compatibility
- Fallback when snapshots aren't available
- Hook mode edge cases

Helper Functions (shared by all parsers):
----------------------------------------
- isLikelyUserPromptLine: Validates if a line is user input vs AI response
- isMenuOption: Detects menu options (e.g., "❯ 1. Yes")
- isPromptOrCommand: Filters out UI prompts and commands
- canSkip: Filters out empty lines and UI borders
- extractLastAssistantContent: Basic extraction without prompt matching
- IsThinking: Checks if CLI is still processing
- RemoveUIStatusLines: Removes UI indicators from final response
*/

// hasPromptCharacterPrefix checks if a line has a cursor prefix
func hasPromptCharacterPrefix(line string) bool {
	// Strip borders and icons first for robust detection
	clean := StripANSI(line)
	clean = strings.TrimPrefix(clean, "│")
	clean = strings.TrimSpace(clean)
	clean = strings.TrimPrefix(clean, "-") // Confirm icon
	clean = strings.TrimPrefix(clean, "?") // Selection icon
	clean = strings.TrimSpace(clean)

	prefix := []string{
		"> ",
		"❯ ",
		">>>",
		"Shell", // Claude Code / Gemini CLI specific
	}
	for _, pattern := range prefix {
		if strings.HasPrefix(clean, pattern) {
			return true
		}
	}
	return false
}

// ExtractNewContent identifies new AI content from tmux output using dual anchors.
// It first tries to locate the user's prompt. If not found, it uses the 
// beforeSnapshot to identify the increment.
func ExtractNewContent(output, userPrompt, beforeSnapshot string) string {
	var inputs []InputRecord
	if userPrompt != "" {
		inputs = []InputRecord{{Content: userPrompt}}
	}
	return ExtractNewContentWithHistory(output, inputs, beforeSnapshot)
}

// ExtractNewContentWithHistory identifies new AI content from tmux output using multiple anchors.
// It iterates through historical inputs (newest to oldest) to find a match.
// If no prompt match is found, it falls back to using the beforeSnapshot to identify the increment.
func ExtractNewContentWithHistory(output string, inputs []InputRecord, beforeSnapshot string) string {
	if output == "" {
		return ""
	}

	var activeContent string
	lines := strings.Split(output, "\n")

	// Step 1: Try to locate any user prompt from history (newest first)
	for _, input := range inputs {
		if input.Content == "" {
			continue
		}
		matcher := NewPromptMatcher(input.Content)
		promptIdx := matcher.findPromptIndex(lines)
		if promptIdx != -1 {
			activeContent = strings.Join(lines[promptIdx+1:], "\n")
			logger.WithFields(logrus.Fields{
				"match_type": "prompt",
				"prompt":     truncateString(input.Content, 20),
			}).Debug("identified-new-content-by-prompt")
			break
		}
	}

	// Step 2: Use beforeSnapshot anchor if no prompt match found
	if activeContent == "" && beforeSnapshot != "" {
		activeContent = ExtractIncrement(output, beforeSnapshot)
		logger.WithField("match_type", "snapshot").Debug("identified-new-content-by-snapshot")
	}

	// Step 3: No anchors found, use basic assistant extraction.
	if activeContent == "" {
		// CRITICAL: We only use the last 10 lines here to avoid 
		// leaking massive amounts of historical data from the 200-line capture.
		smallWindow := lines
		if len(lines) > 10 {
			smallWindow = lines[len(lines)-10:]
		}
		activeContent = extractLastAssistantContent(strings.Join(smallWindow, "\n"))
		logger.WithField("match_type", "basic_small_window").Debug("identified-new-content-by-basic-extraction")
	}

	// Always clean the result (remove UI status lines, consecutive newlines)
	cleaned := cleanContent(activeContent)
	cleaned = RemoveUIStatusLines(cleaned)

	// FINAL STEP: Strip all ANSI escape codes.
	return StripANSI(cleaned)
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
	// Support all cursor prefixes: "❯ ", "> ", ">>>"
	cursorPrefixes := []string{"❯ ", "> ", ">>>"}
	var matchedPrefix string
	var cleanLine string

	for _, prefix := range cursorPrefixes {
		if strings.HasPrefix(line, prefix) && strings.Contains(line, userPrompt) {
			matchedPrefix = prefix
			cleanLine = strings.TrimPrefix(line, prefix)
			cleanLine = strings.TrimSpace(cleanLine)
			break
		}
	}

	if matchedPrefix != "" {
		// Check if this is a menu option (e.g., "❯ 1. Yes", "❯ 2. Option")
		// Menu options have the pattern: cursor + number + punctuation + text
		// User input has the pattern: cursor + userPrompt (possibly with text after)

		// If line starts with userPrompt followed by menu punctuation, it's a menu option
		if strings.HasPrefix(cleanLine, userPrompt) {
			afterPrompt := cleanLine[len(userPrompt):]
			afterPrompt = strings.TrimSpace(afterPrompt)

			// Menu indicators: ". " (period), ", " (comma), etc.
			if strings.HasPrefix(afterPrompt, ". ") ||
			   strings.HasPrefix(afterPrompt, ",") ||
			   strings.HasPrefix(afterPrompt, "、") {
				logger.WithFields(logrus.Fields{
					"line":        line,
					"user_prompt": userPrompt,
					"reason":      "menu option pattern detected",
				}).Debug("rejecting-line-is-menu-option-not-user-input")
				return false
			}
		}

		// If cleanLine exactly matches userPrompt, it's user input
		if cleanLine == userPrompt || strings.HasPrefix(cleanLine, userPrompt+" ") {
			logger.WithFields(logrus.Fields{
				"line":        line,
				"user_prompt": userPrompt,
				"reason":      "has cursor prefix and matches user prompt",
			}).Debug("accepting-line-has-cursor-prefix")
			return true
		}
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

	// Priority 3: Match with cursor prefix characters (for > and >>> without exact match)
	if hasPromptCharacterPrefix(line) {
		cleanLine := line
		for _, prefix := range cursorPrefixes {
			if strings.HasPrefix(line, prefix) {
				cleanLine = strings.TrimPrefix(line, prefix)
				cleanLine = strings.TrimSpace(cleanLine)
				break
			}
		}
		if cleanLine == userPrompt || strings.HasPrefix(cleanLine, userPrompt+" ") {
			logger.WithFields(logrus.Fields{
				"line":        line,
				"user_prompt": userPrompt,
				"reason":      "has prompt character prefix and matches",
			}).Debug("accepting-line-has-prompt-char-prefix")
			return true
		}
	}

	// Reject all other cases (including AI responses like "test content follows")
	logger.WithFields(logrus.Fields{
		"line":       line,
		"user_prompt": userPrompt,
		"line_len":   len(line),
		"prompt_len": len(userPrompt),
		"reason":     "no valid cursor prefix and not exact match",
	}).Debug("rejecting-line-does-not-look-like-user-prompt")

	return false
}

// isMenuOption checks if a line is a menu option (e.g., "❯ 1. Yes", "❯ 2. No")
// Menu options should be preserved in final responses but used as anchor points
func isMenuOption(line string) bool {
	// Check for cursor prefix
	cursorPrefixes := []string{"❯ ", "> ", ">>>"}
	for _, prefix := range cursorPrefixes {
		if strings.HasPrefix(line, prefix) {
			cleanLine := strings.TrimPrefix(line, prefix)
			cleanLine = strings.TrimSpace(cleanLine)

			// Menu options typically start with a number followed by punctuation
			// e.g., "1. Yes", "2. No", "3. Cancel"
			// Match pattern: digit + punctuation
			if len(cleanLine) > 0 {
				firstChar := cleanLine[0]
				if firstChar >= '0' && firstChar <= '9' {
					// Check if followed by punctuation
					if len(cleanLine) > 2 {
						secondPart := cleanLine[1:]
						if strings.HasPrefix(secondPart, ". ") ||
						   strings.HasPrefix(secondPart, ",") ||
						   strings.HasPrefix(secondPart, "、") {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// isPromptOrCommand checks if a line is a prompt/command rather than assistant output
func isPromptOrCommand(line string) bool {
	clean := StripANSI(line)

	// Exact patterns (must match from start or be standalone)
	if strings.HasPrefix(clean, "user@") ||
		strings.HasPrefix(clean, "$ ") ||
		strings.HasPrefix(clean, ">>>") ||
		strings.HasPrefix(clean, "Shell") || // Gemini CLI / Claude Code
		clean == "..." ||
		strings.Contains(clean, "[?]") ||
		strings.Contains(clean, "Press Enter") ||
		strings.Contains(clean, "Confirm?") ||
		strings.Contains(clean, "lines hidden") { // TUI hidden lines message
		return true
	}

	// Special case: Lines with cursor prefix "❯ "
	if strings.HasPrefix(clean, "❯ ") || strings.HasPrefix(clean, "> ") {
		// Menu options like "❯ 1. Yes" should be kept
		if isMenuOption(line) {
			return false // Keep menu options
		}
		// User input prompts should be filtered
		return true
	}

	// Special case: Lines ending with "..." that are single words or common status indicators
	// These are typically loading/paused indicators, not AI responses
	// But we need to distinguish from AI sentences that end with "..."
	if strings.HasSuffix(line, "...") {
		// Single word with ellipsis (e.g., "Loading...", "Waiting...")
		// are considered prompts, but full sentences are not
		words := strings.Fields(line)
		if len(words) == 1 {
			// Check if it's an AI status message (verbs with -ing/-ed)
			// vs UI indicator (nouns like "Loading", "Waiting")
			word := strings.ToLower(strings.TrimSuffix(words[0], "..."))

			// AI status messages: verbs with common endings
			if strings.HasSuffix(word, "ing") || strings.HasSuffix(word, "ed") {
				return false  // "Thinking...", "Processing...", "Compiling..." are AI status
			}

			return true  // Single word like "Loading..." is a UI prompt
		}

		// Multi-word phrases with ellipsis are AI responses, not prompts
		// e.g., "Great! Proceeding with the operation..." is an AI response
		return false
	}

	return false
}

// canSkip checks if a line should be skipped (empty or UI border)
func canSkip(line string) bool {
	// First strip ANSI codes to ensure we're looking at pure characters
	cleanLine := StripANSI(line)
	trimmed := strings.TrimSpace(cleanLine)
	
	if trimmed == "" {
		return true
	}

	// Detect and skip UI borders (box drawing characters and blocks)
	// Single-line: ─ │ ┌ └ ┐ ┘ ├ ┤
	// Double-line: ═ ║ ╒ ╓ ╔ ╕ ╖ ╗ ╘ ╙ ╚ ╛ ╜ ╝ ╞ ╟ ╠ ╡ ╢ ╣ ╤ ╥ ╦ ╧ ╨ ╩
	// Rounded: ╭ ╮ ╰ ╯
	// Blocks: ▀ ▄ █ ▌ ▐
	// Other space-like: · • ● ○ ◌ ■
	for _, runeValue := range trimmed {
		if strings.ContainsRune("─│┌└┐┘├┤═║╒╓╔╕╖╗╘╙╚╛╜╝╞╟╠╡╢╣╤╥╦╧╨╩╭╮╰╯·•●○◌■ ▀▄█▌▐", runeValue) {
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
		// CRITICAL: Strip ANSI codes first because CLI tools (like Gemini)
		// wrap user input in color codes (e.g. \x1b[34m> input\x1b[0m)
		// Without this, HasPrefix("> ") fails.
		cleanLine := StripANSI(lines[i])
		trimmed := strings.TrimSpace(cleanLine)
		
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

// InputRecord represents a historical user input
type InputRecord struct {
	Timestamp int64
	Content   string
}

// ExtractContentAfterAnyInput extracts content appearing after ANY of the provided inputs
// Searches inputs from newest to oldest, returning the first match found
// This is useful for handling short inputs (menu selections) that may not appear in tmux output
//
// Parameters:
//   - tmuxOutput: the captured tmux pane output
//   - inputs: historical user inputs ordered from newest to oldest
//
// Returns:
//   - Extracted content after the first matching input
//   - If no inputs match, returns result of extractLastAssistantContent (fallback)
func ExtractContentAfterAnyInput(tmuxOutput string, inputs []InputRecord) string {
	if len(inputs) == 0 {
		logger.Debug("no-inputs-provided-using-basic-extraction")
		return extractLastAssistantContent(tmuxOutput)
	}

	lines := strings.Split(tmuxOutput, "\n")

	// Try each input from newest to oldest
	for _, input := range inputs {
		logger.WithFields(logrus.Fields{
			"input_length": len(input.Content),
			"timestamp":    input.Timestamp,
		}).Debug("trying-to-match-input-in-tmux-output")

		matcher := NewPromptMatcher(input.Content)
		promptIndex := matcher.findPromptIndex(lines)

		if promptIndex != -1 {
			// Found a match!
			contentLines := matcher.extractContent(lines, promptIndex)

			logger.WithFields(logrus.Fields{
				"matched_input":      input.Content,
				"matched_timestamp":  input.Timestamp,
				"total_lines":        len(lines),
				"prompt_index":       promptIndex,
				"content_lines":      len(contentLines),
			}).Info("found-input-match-extracting-content")

			if len(contentLines) > 0 {
				result := strings.Join(contentLines, "\n")
				return cleanContent(result)
			}
		}
	}

	// No inputs matched, use fallback
	logger.WithFields(logrus.Fields{
		"tried_inputs": len(inputs),
	}).Debug("no-input-matched-using-basic-extraction")
	return extractLastAssistantContent(tmuxOutput)
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
		"loading",
		"working",
		"summoning",
		"generating",
		"computing",
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
	clean := StripANSI(line)

	// UI status line patterns
	uiPatterns := []string{
		"Undulating…",
		"running stop hook",
		"esc to interrupt",
		"press escape",
		"? for shortcuts",
	}

	lowerLine := strings.ToLower(clean)
	for _, pattern := range uiPatterns {
		if strings.Contains(lowerLine, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for single-character cursor indicators
	if clean == "❯" || clean == ">" || clean == "$" {
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

		// Remove UI status lines and prompts
		// NOTE: Prompts need to be removed as a safety net for extraction paths
		// that may not fully filter them out (snapshot matching, basic extraction)
		if isUIStatusLine(line) || isPromptOrCommand(line) {
			logger.WithField("line", trimmed).Debug("removing-ui-status-or-prompt-line-from-response")
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
