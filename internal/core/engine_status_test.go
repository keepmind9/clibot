package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatUptime tests the formatUptime function
func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name     string
		seconds  float64
		expected string
	}{
		{"less than 1 minute", 30.0, "30s"},
		{"exactly 1 minute", 60.0, "1m"},
		{"5 minutes", 300.0, "5m"},
		{"1 hour", 3600.0, "1h0m"},
		{"2 hours 30 minutes", 9000.0, "2h30m"},
		{"24 hours", 86400.0, "24h0m"},
		{"large value", 172800.0, "48h0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUptime(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsSpecialCommandWithSstatus tests sstatus command parsing
func TestIsSpecialCommandWithSstatus(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCmd   string
		expectedArgs  []string
		expectedIsCmd bool
	}{
		{
			name:          "sstatus without args",
			input:         "sstatus",
			expectedCmd:   "sstatus",
			expectedArgs:  nil,
			expectedIsCmd: true,
		},
		{
			name:          "sstatus with session name",
			input:         "sstatus backend",
			expectedCmd:   "sstatus",
			expectedArgs:  []string{"backend"},
			expectedIsCmd: true,
		},
		{
			name:          "sstatus with extra spaces",
			input:         "sstatus   backend",
			expectedCmd:   "sstatus",
			expectedArgs:  []string{"backend"},
			expectedIsCmd: true,
		},
		{
			name:          "sstatus with slash",
			input:         "/sstatus backend",
			expectedCmd:   "sstatus",
			expectedArgs:  []string{"backend"},
			expectedIsCmd: true,
		},
		{
			name:          "non-special command",
			input:         "hello world",
			expectedCmd:   "",
			expectedArgs:  nil,
			expectedIsCmd: false,
		},
		{
			name:          "empty string",
			input:         "",
			expectedCmd:   "",
			expectedArgs:  nil,
			expectedIsCmd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCmd, args := isSpecialCommand(tt.input)
			assert.Equal(t, tt.expectedIsCmd, isCmd)
			if isCmd {
				assert.Equal(t, tt.expectedCmd, cmd)
				assert.Equal(t, tt.expectedArgs, args)
			}
		})
	}
}
