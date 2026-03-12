package bot

import (
	"regexp"
	"strings"

	"github.com/keepmind9/clibot/pkg/constants"
)

// maskSecret masks sensitive information for logging
func maskSecret(s string) string {
	if len(s) <= constants.MinSecretLengthForMasking {
		return "***"
	}
	return s[:constants.SecretMaskPrefixLength] + "***" + s[len(s)-constants.SecretMaskSuffixLength:]
}

// WrapTablesInCodeBlocks detects markdown tables and wraps them in code blocks
// if they are not already inside one. This helps with mobile rendering by
// using fixed-width fonts.
func WrapTablesInCodeBlocks(text string) string {
	// Simple regex to detect markdown table header separator: | --- | or |---|
	// This is a common pattern for markdown tables.
	tableRegex := regexp.MustCompile(`(?m)^(\|?\s*:?-+:?\s*\|?)+\s*$`)
	lines := strings.Split(text, "\n")
	
	var result []string
	inCodeBlock := false
	inTable := false
	tableStart := -1
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Track code blocks to avoid double-wrapping
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result = append(result, line)
			continue
		}
		
		if inCodeBlock {
			result = append(result, line)
			continue
		}
		
		// Detect table separator
		if tableRegex.MatchString(trimmed) {
			if !inTable && i > 0 {
				// We found a separator, the previous line was likely the header
				inTable = true
				tableStart = len(result) - 1
				// Insert opening backticks before header
				if tableStart >= 0 {
					header := result[tableStart]
					result[tableStart] = "```\n" + header
				}
			}
		} else if inTable {
			// Check if table ended (empty line or doesn't start/contain |)
			if trimmed == "" || (!strings.Contains(trimmed, "|") && !tableRegex.MatchString(trimmed)) {
				inTable = false
				// Close the code block
				if len(result) > 0 {
					result[len(result)-1] = result[len(result)-1] + "\n```"
				}
			}
		}
		
		result = append(result, line)
	}
	
	// Close table if it reached the end of text
	if inTable && len(result) > 0 {
		result[len(result)-1] = result[len(result)-1] + "\n```"
	}
	
	return strings.Join(result, "\n")
}
