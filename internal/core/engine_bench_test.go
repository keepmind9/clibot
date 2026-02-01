package core

import (
	"strings"
	"testing"
)

// BenchmarkIsSpecialCommand_ExactMatch benchmarks exact match commands (fast path)
func BenchmarkIsSpecialCommand_ExactMatch(b *testing.B) {
	tests := []string{
		"help",
		"status",
		"sessions",
		"whoami",
		"view",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range tests {
			isSpecialCommand(test)
		}
	}
}

// BenchmarkIsSpecialCommand_ViewWithArgs benchmarks view command with arguments
func BenchmarkIsSpecialCommand_ViewWithArgs(b *testing.B) {
	tests := []string{
		"view 100",
		"view 50",
		"view 200",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range tests {
			isSpecialCommand(test)
		}
	}
}

// BenchmarkIsSpecialCommand_NormalInput benchmarks normal input (not a command)
func BenchmarkIsSpecialCommand_NormalInput(b *testing.B) {
	tests := []string{
		"help me write this code",
		"what is the status",
		"can you help me",
		"write a function to parse this",
		"analyze this code please",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range tests {
			isSpecialCommand(test)
		}
	}
}

// BenchmarkIsSpecialCommand_ChineseInput benchmarks Chinese input
func BenchmarkIsSpecialCommand_ChineseInput(b *testing.B) {
	tests := []string{
		"帮我写代码",
		"请分析这个函数",
		"解释一下这段代码",
		"优化这个算法",
		"帮我重构代码",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range tests {
			isSpecialCommand(test)
		}
	}
}

// BenchmarkIsSpecialCommand_LongInput benchmarks very long input
func BenchmarkIsSpecialCommand_LongInput(b *testing.B) {
	// Simulate long natural language input
	longInputs := make([]string, 10)
	for i := range longInputs {
		longInputs[i] = strings.Repeat("help me understand and analyze this complex piece of code ", 10)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range longInputs {
			isSpecialCommand(input)
		}
	}
}

// BenchmarkIsSpecialCommand_Mixed benchmarks mixed workload
func BenchmarkIsSpecialCommand_Mixed(b *testing.B) {
	tests := []string{
		"help",                   // 20% - exact match
		"status",                 // 20% - exact match
		"view 100",               // 15% - view with args
		"help me write code",     // 20% - normal input
		"帮我优化这段代码",          // 15% - Chinese input
		"sessions",               // 10% - exact match
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range tests {
			isSpecialCommand(test)
		}
	}
}

// BenchmarkMapLookup compares pure map lookup performance
func BenchmarkMapLookup(b *testing.B) {
	m := map[string]struct{}{
		"help":     {},
		"status":   {},
		"sessions": {},
		"whoami":   {},
		"view":     {},
	}

	keys := []string{"help", "status", "sessions", "whoami", "view"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, key := range keys {
			_, _ = m[key]
		}
	}
}

// BenchmarkHasPrefix benchmarks HasPrefix performance
func BenchmarkHasPrefix(b *testing.B) {
	inputs := []string{
		"view 100",
		"view 50",
		"viewhelp",
		"view! 100",
		"view command",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			if len(input) > 4 && input[:4] == "view" && input[4] == ' ' {
				_ = strings.Fields(input[5:])
			}
		}
	}
}

// BenchmarkOldApproach benchmarks the old Fields-first approach (for comparison)
func BenchmarkOldApproach(b *testing.B) {
	specialCommands := map[string]struct{}{
		"help":     {},
		"status":   {},
		"sessions": {},
		"whoami":   {},
		"view":     {},
	}

	tests := []string{
		"help",
		"view 100",
		"help me write code",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range tests {
			// Old approach: always use Fields
			parts := strings.Fields(input)
			if len(parts) > 0 {
				if _, exists := specialCommands[parts[0]]; exists {
					// Command found
				}
			}
		}
	}
}

// BenchmarkNewApproach benchmarks the new exact-match-first approach
func BenchmarkNewApproach(b *testing.B) {
	specialCommands := map[string]struct{}{
		"help":     {},
		"status":   {},
		"sessions": {},
		"whoami":   {},
		"view":     {},
	}

	tests := []string{
		"help",
		"view 100",
		"help me write code",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range tests {
			// New approach: exact match first
			if _, exists := specialCommands[input]; exists {
				// Fast path hit
				continue
			}

			// Slow path: view with args
			if len(input) > 5 && input[:4] == "view" && input[4] == ' ' {
				_ = strings.Fields(input[5:])
			}
		}
	}
}
