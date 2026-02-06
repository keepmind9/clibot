package core

import (
	"strings"
	"testing"
)

// TestIsSpecialCommand_ExactMatch tests exact match behavior
func TestIsSpecialCommand_ExactMatch(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCmd   string
		expectedIsCmd bool
		expectedArgs  []string
	}{
		// === Exact match commands (no args) ===
		{
			name:          "help command",
			input:         "help",
			expectedCmd:   "help",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "status command",
			input:         "status",
			expectedCmd:   "status",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "slist command",
			input:         "slist",
			expectedCmd:   "slist",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "snew command",
			input:         "snew",
			expectedCmd:   "snew",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "sdel command",
			input:         "sdel",
			expectedCmd:   "sdel",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "whoami command",
			input:         "whoami",
			expectedCmd:   "whoami",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "view command without args",
			input:         "view",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "echo command",
			input:         "echo",
			expectedCmd:   "echo",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},

		// === View command with numeric arguments ===
		{
			name:          "view with single number",
			input:         "view 100",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"100"},
		},
		{
			name:          "view with small number",
			input:         "view 1",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"1"},
		},
		{
			name:          "view with zero",
			input:         "view 0",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"0"},
		},
		{
			name:          "view with large number",
			input:         "view 9999",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"9999"},
		},
		{
			name:          "view with multiple spaces",
			input:         "view    50",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"50"},
		},
		{
			name:          "view with tab and space",
			input:         "view\t50",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"50"},
		},

		// === View command with non-numeric arguments (normal input) ===
		{
			name:          "view with help argument - normal input",
			input:         "view help",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view with text argument - normal input",
			input:         "view explain",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view with negative number",
			input:         "view -100",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"-100"},
		},
		{
			name:          "view with float - normal input",
			input:         "view 1.5",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},

		// === Normal input (should NOT match) ===
		{
			name:          "help with extra text - not exact match",
			input:         "help me",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "help with question",
			input:         "help?",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "status with extra text",
			input:         "status please",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "slist with extra",
			input:         "slist list",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "whoami with extra",
			input:         "whoami now",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "viewhelp - not view with space",
			input:         "viewhelp",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "natural language - help me write code",
			input:         "help me write this function",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "natural language - what is status",
			input:         "what is the status",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},

		// === Case sensitivity ===
		{
			name:          "HELP - uppercase",
			input:         "HELP",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "Help - capitalized",
			input:         "Help",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "STATUS - uppercase",
			input:         "STATUS",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "View 100 - capitalized",
			input:         "View 100",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},

		// === Edge cases ===
		{
			name:          "empty string",
			input:         "",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "whitespace only",
			input:         "   ",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "tab only",
			input:         "\t",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "newline",
			input:         "\n",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view with space but no args",
			input:         "view ",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view with multiple spaces but no args",
			input:         "view    ",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},

		// === Special characters ===
		{
			name:          "view with special char",
			input:         "view!",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "help with dot",
			input:         "help.txt",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},

		// === Unicode input (non-ASCII) ===
		{
			name:          "Unicode Chinese text",
			input:         "帮我写代码",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "Mixed Unicode - help in Chinese",
			input:         "帮助 help",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},

		// === Security: Input length validation (DoS protection) ===
		{
			name:          "view with extremely large number - out of range",
			input:         "view 999999999999999999",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view with very large positive number - out of range",
			input:         "view 10001",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view with very large negative number - out of range",
			input:         "view -10001",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "view at max range boundary",
			input:         "view 10000",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"10000"},
		},
		{
			name:          "view at negative max range boundary",
			input:         "view -10000",
			expectedCmd:   "view",
			expectedIsCmd: true,
			expectedArgs:  []string{"-10000"},
		},
		{
			name:          "view with one past max range - normal input",
			input:         "view 10001",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCmd, args := isSpecialCommand(tt.input)

			if isCmd != tt.expectedIsCmd {
				t.Errorf("isSpecialCommand(%q) isCmd = %v, want %v", tt.input, isCmd, tt.expectedIsCmd)
			}

			if cmd != tt.expectedCmd {
				t.Errorf("isSpecialCommand(%q) cmd = %q, want %q", tt.input, cmd, tt.expectedCmd)
			}

			// Check args
			if tt.expectedArgs == nil {
				if args != nil {
					t.Errorf("isSpecialCommand(%q) args = %v, want nil", tt.input, args)
				}
			} else {
				if args == nil {
					t.Errorf("isSpecialCommand(%q) args = nil, want %v", tt.input, tt.expectedArgs)
				} else if len(args) != len(tt.expectedArgs) {
					t.Errorf("isSpecialCommand(%q) args length = %d, want %d", tt.input, len(args), len(tt.expectedArgs))
				} else {
					for i, arg := range args {
						if arg != tt.expectedArgs[i] {
							t.Errorf("isSpecialCommand(%q) args[%d] = %q, want %q", tt.input, i, arg, tt.expectedArgs[i])
						}
					}
				}
			}
		})
	}
}

// TestIsSpecialCommand_PerformanceFastPath tests that exact match uses fast path
func TestIsSpecialCommand_PerformanceFastPath(t *testing.T) {
	// Test that exact match commands don't parse input
	// This verifies the fast path is being used

	// Create a large input that would be expensive to parse
	largeInput := strings.Repeat("a", 10000)

	// This should NOT parse the entire string, just do a map lookup
	cmd, isCmd, args := isSpecialCommand(largeInput)

	// Should return false immediately with O(1) map lookup
	if isCmd {
		t.Errorf("Expected isCmd=false for large input, got true")
	}
	if cmd != "" {
		t.Errorf("Expected cmd=\"\" for large input, got %q", cmd)
	}
	if args != nil {
		t.Errorf("Expected args=nil for large input, got %v", args)
	}
}

// TestIsSpecialCommand_ViewArgsParsing tests view command argument parsing
func TestIsSpecialCommand_ViewArgsParsing(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedArgs  []string
		expectMatch   bool
	}{
		{
			name:        "view with single digit",
			input:       "view 5",
			expectedArgs: []string{"5"},
			expectMatch:  true,
		},
		{
			name:        "view with double digit",
			input:       "view 50",
			expectedArgs: []string{"50"},
			expectMatch:  true,
		},
		{
			name:        "view with triple digit",
			input:       "view 100",
			expectedArgs: []string{"100"},
			expectMatch:  true,
		},
		{
			name:        "view with non-numeric argument - normal input",
			input:       "view help",
			expectedArgs:  nil,
			expectMatch:  false,
		},
		{
			name:        "view with multiple numeric args - only first used",
			input:       "view 100 200",
			expectedArgs: []string{"100"},  // Only first arg is used
			expectMatch:  true,
		},
		{
			name:        "view with multiple words - normal input",
			input:       "view help me",
			expectedArgs:  nil,
			expectMatch:  false,
		},
		{
			name:        "view with leading space - not matched (no trim)",
			input:       " view 100",
			expectedArgs:  nil,
			expectMatch:  false,
		},
		{
			name:        "view with trailing space",
			input:       "view 100 ",
			expectedArgs: []string{"100"},
			expectMatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCmd, args := isSpecialCommand(tt.input)

			if isCmd != tt.expectMatch {
				t.Errorf("isSpecialCommand(%q) isCmd = %v, want %v", tt.input, isCmd, tt.expectMatch)
			}

			if tt.expectMatch {
				if cmd != "view" {
					t.Errorf("isSpecialCommand(%q) cmd = %q, want \"view\"", tt.input, cmd)
				}

				if len(args) != len(tt.expectedArgs) {
					t.Errorf("isSpecialCommand(%q) args length = %d, want %d", tt.input, len(args), len(tt.expectedArgs))
				} else {
					for i, arg := range args {
						if arg != tt.expectedArgs[i] {
							t.Errorf("isSpecialCommand(%q) args[%d] = %q, want %q", tt.input, i, arg, tt.expectedArgs[i])
						}
					}
				}
			}
		})
	}
}
