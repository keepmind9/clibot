// Package watchdog provides utilities for tmux session monitoring and output parsing.
//
// This file handles keyword-based key sequence conversion for AI CLI tools.
// It converts entire input messages matching specific keywords to actual key sequences.
package watchdog

import (
	"strings"
)

// ProcessKeyWords converts entire input matching key words to tmux key names.
//
// Only processes when the ENTIRE input matches (case-insensitive, trimmed).
// This is designed for AI CLI tools where users frequently send special keys
// like Tab, Esc, and Shift+Tab.
//
// Supported keywords:
//   - "tab"     → Tab key (C-i)
//   - "esc"     → Escape key (C-[)
//   - "stab"    → Shift+Tab
//   - "s-tab"   → Shift+Tab (alias)
//   - "enter"   → Enter key (C-m)
//   - "ctrlc"   → Ctrl+C interrupt (C-c)
//   - "ctrl-c"  → Ctrl+C interrupt (C-c) (alias)
//   - "ctrlt"   → Ctrl+T (C-t)
//   - "ctrl-t"  → Ctrl+T (C-t) (alias)
//
// Examples:
//   ProcessKeyWords("tab")      → "C-i"
//   ProcessKeyWords("TAB")      → "C-i"
//   ProcessKeyWords("  tab  ")  → "C-i"
//   ProcessKeyWords("esc")      → "C-["
//   ProcessKeyWords("stab")     → "S-Tab"
//   ProcessKeyWords("s-tab")    → "S-Tab"
//   ProcessKeyWords("ctrlc")    → "C-c"
//   ProcessKeyWords("CTRL-C")   → "C-c"
//   ProcessKeyWords("help tab") → "help tab" (no match, return as-is)
//
// Returns the original input if no keyword match is found.
//
// Note: This returns tmux key names (like "C-[", "C-i") instead of raw
// escape sequences because SendKeys uses the "-l" (literal) flag,
// which sends input as literal text rather than interpreting key names.
func ProcessKeyWords(input string) string {
	normalized := strings.ToLower(strings.TrimSpace(input))

	// Handle empty string after trimming
	if normalized == "" {
		return normalized
	}

	switch normalized {
	case "tab":
		return "C-i" // Tab key in tmux notation
	case "esc":
		return "C-[" // Escape key in tmux notation
	case "stab", "s-tab":
		return "\x1b[Z" // Shift+Tab escape sequence (tmux doesn't support S-Tab key name)
	case "enter":
		return "C-m" // Enter key in tmux notation
	case "ctrlc", "ctrl-c":
		return "C-c" // Ctrl+C in tmux notation
	case "ctrlt", "ctrl-t":
		return "C-t" // Ctrl+T in tmux notation
	default:
		return input
	}
}
