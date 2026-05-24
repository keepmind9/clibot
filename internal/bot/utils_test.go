package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected string
	}{
		{
			name:     "normal secret",
			secret:   "cli_1234567890abcdef",
			expected: "cli_***cdef",
		},
		{
			name:     "short secret",
			secret:   "1234567890",
			expected: "***",
		},
		{
			name:     "very short secret",
			secret:   "1234",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSecret(tt.secret)
			assert.Equal(t, tt.expected, result)
		})
	}
}
