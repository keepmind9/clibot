// Package watchdog provides utilities for tmux session monitoring and output parsing.
//
// This file handles keyword-based key sequence conversion for AI CLI tools.
// It converts entire input messages matching specific keywords to actual key sequences.
package watchdog

import (
	"strings"
)

// ProcessKeyWords converts entire input matching key words to key sequences.
//
// Only processes when the ENTIRE input matches (case-insensitive, trimmed).
// This is designed for AI CLI tools where users frequently send special keys
// like Tab, Esc, and Shift+Tab.
//
// Supported keywords:
//   - "tab"     → Tab key (\t)
//   - "esc"     → Escape key (\x1b)
//   - "stab"    → Shift+Tab (\x1b[Z)
//   - "s-tab"   → Shift+Tab (\x1b[Z) (alias)
//   - "enter"   → Enter key (\n)
//   - "ctrlc"   → Ctrl+C interrupt (\x03)
//   - "ctrl-c"  → Ctrl+C interrupt (\x03) (alias)
//
// Examples:
//   ProcessKeyWords("tab")      → "\t"
//   ProcessKeyWords("TAB")      → "\t"
//   ProcessKeyWords("  tab  ")  → "\t"
//   ProcessKeyWords("stab")     → "\x1b[Z" (Shift+Tab)
//   ProcessKeyWords("s-tab")    → "\x1b[Z" (Shift+Tab)
//   ProcessKeyWords("ctrlc")    → "\x03" (Ctrl+C)
//   ProcessKeyWords("CTRL-C")   → "\x03" (Ctrl+C)
//   ProcessKeyWords("help tab") → "help tab" (no match, return as-is)
//
// Returns the original input if no keyword match is found.
func ProcessKeyWords(input string) string {
	normalized := strings.ToLower(strings.TrimSpace(input))

	// Handle empty string after trimming
	if normalized == "" {
		return normalized
	}

	switch normalized {
	case "tab":
		return "\t"
	case "esc":
		return "\x1b"
	case "stab", "s-tab":
		return "\x1b[Z" // Shift+Tab escape sequence
	case "enter":
		return "\n"
	case "ctrlc", "ctrl-c":
		return "\x03" // Ctrl+C interrupt signal
	default:
		return input
	}
}
