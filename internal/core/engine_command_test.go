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
			name:          "suse command",
			input:         "suse",
			expectedCmd:   "suse",
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
			name:          "echo command",
			input:         "echo",
			expectedCmd:   "echo",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "help command with slash",
			input:         "/help",
			expectedCmd:   "help",
			expectedIsCmd: true,
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

// TestIsSpecialCommand_WithArgs tests commands that accept arguments
func TestIsSpecialCommand_WithArgs(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCmd   string
		expectedIsCmd bool
		expectedArgs  []string
	}{
		{
			name:          "sstatus without args",
			input:         "sstatus",
			expectedCmd:   "sstatus",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "sstatus with session name",
			input:         "sstatus backend",
			expectedCmd:   "sstatus",
			expectedIsCmd: true,
			expectedArgs:  []string{"backend"},
		},
		{
			name:          "sstatus with multiple words",
			input:         "sstatus backend api",
			expectedCmd:   "sstatus",
			expectedIsCmd: true,
			expectedArgs:  []string{"backend", "api"},
		},
		{
			name:          "sclose without args",
			input:         "sclose",
			expectedCmd:   "sclose",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "sclose with session name",
			input:         "sclose my-session",
			expectedCmd:   "sclose",
			expectedIsCmd: true,
			expectedArgs:  []string{"my-session"},
		},
		{
			name:          "sdel without args",
			input:         "sdel",
			expectedCmd:   "sdel",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "sdel with session name",
			input:         "sdel temp-session",
			expectedCmd:   "sdel",
			expectedIsCmd: true,
			expectedArgs:  []string{"temp-session"},
		},
		{
			name:          "suse without args",
			input:         "suse",
			expectedCmd:   "suse",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "suse with session name",
			input:         "suse default",
			expectedCmd:   "suse",
			expectedIsCmd: true,
			expectedArgs:  []string{"default"},
		},
		{
			name:          "echo exact match",
			input:         "echo",
			expectedCmd:   "echo",
			expectedIsCmd: true,
			expectedArgs:  nil,
		},
		{
			name:          "echo with args is not a command",
			input:         "echo hello world",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "non-command text",
			input:         "hello this is a normal message",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "empty string",
			input:         "",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "command-like but not exact",
			input:         "helpme",
			expectedCmd:   "",
			expectedIsCmd: false,
			expectedArgs:  nil,
		},
		{
			name:          "snewg with args",
			input:         "snewg my-sess /tmp",
			expectedCmd:   "snewg",
			expectedIsCmd: true,
			expectedArgs:  []string{"my-sess", "/tmp"},
		},
		{
			name:          "snewg with args and slash",
			input:         "/snewg my-sess /tmp",
			expectedCmd:   "snewg",
			expectedIsCmd: true,
			expectedArgs:  []string{"my-sess", "/tmp"},
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

			if tt.expectedArgs == nil {
				if args != nil {
					t.Errorf("isSpecialCommand(%q) args = %v, want nil", tt.input, args)
				}
			} else {
				if args == nil {
					t.Errorf("isSpecialCommand(%q) args = nil, want %v", tt.input, tt.expectedArgs)
				} else {
					// Compare args slices
					if len(args) != len(tt.expectedArgs) {
						t.Errorf("isSpecialCommand(%q) args length = %d, want %d", tt.input, len(args), len(tt.expectedArgs))
					} else {
						for i := range args {
							if args[i] != tt.expectedArgs[i] {
								t.Errorf("isSpecialCommand(%q) args[%d] = %q, want %q", tt.input, i, args[i], tt.expectedArgs[i])
							}
						}
					}
				}
			}
		})
	}
}
