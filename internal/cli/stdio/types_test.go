package stdio

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissionRequest_FormatOptions(t *testing.T) {
	tests := []struct {
		name     string
		perm     PermissionRequest
		contains []string
		excludes []string
	}{
		{
			name: "basic with no input",
			perm: PermissionRequest{
				ToolName: "Bash",
				Options:  []PermissionOption{{ID: "allow", Text: "Allow"}, {ID: "deny", Text: "Deny"}},
			},
			contains: []string{
				"Permission requested: Bash",
				"Reply 1-2:",
				"1. Allow",
				"2. Deny",
			},
			excludes: []string{"Input:"},
		},
		{
			name: "with short input",
			perm: PermissionRequest{
				ToolName: "Bash",
				Input:    "ls -la",
				Options:  []PermissionOption{{ID: "allow", Text: "Allow"}, {ID: "deny", Text: "Deny"}},
			},
			contains: []string{
				"Permission requested: Bash",
				"Input: ls -la",
				"Reply 1-2:",
			},
		},
		{
			name: "with long input truncated",
			perm: PermissionRequest{
				ToolName: "Bash",
				Input:    strings.Repeat("x", 600),
				Options:  []PermissionOption{{ID: "allow", Text: "Allow"}, {ID: "deny", Text: "Deny"}},
			},
			contains: []string{
				"Input: " + strings.Repeat("x", 500) + "...",
			},
		},
		{
			name: "single option",
			perm: PermissionRequest{
				ToolName: "Read",
				Input:    "/tmp/file",
				Options:  []PermissionOption{{ID: "yes", Text: "Yes"}},
			},
			contains: []string{
				"Reply 1-1:",
				"1. Yes",
			},
		},
		{
			name: "three options",
			perm: PermissionRequest{
				ToolName: "Write",
				Options: []PermissionOption{
					{ID: "allow", Text: "Allow"},
					{ID: "deny", Text: "Deny"},
					{ID: "always", Text: "Always Allow"},
				},
			},
			contains: []string{
				"Reply 1-3:",
				"1. Allow",
				"2. Deny",
				"3. Always Allow",
			},
		},
		{
			name: "input exactly 500 chars not truncated",
			perm: PermissionRequest{
				ToolName: "Tool",
				Input:    strings.Repeat("a", 500),
				Options:  []PermissionOption{{ID: "ok", Text: "OK"}},
			},
			contains: []string{
				"Input: " + strings.Repeat("a", 500) + "\n",
			},
			excludes: []string{"Input: " + strings.Repeat("a", 500) + "..."},
		},
		{
			name: "input 501 chars truncated",
			perm: PermissionRequest{
				ToolName: "Tool",
				Input:    strings.Repeat("b", 501),
				Options:  []PermissionOption{{ID: "ok", Text: "OK"}},
			},
			contains: []string{
				"Input: " + strings.Repeat("b", 500) + "...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.perm.FormatOptions()
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
			for _, s := range tt.excludes {
				assert.NotContains(t, result, s)
			}
		})
	}
}

func TestPermissionRequest_OptionByID(t *testing.T) {
	perm := PermissionRequest{
		Options: []PermissionOption{
			{ID: "allow", Text: "Allow"},
			{ID: "deny", Text: "Deny"},
			{ID: "always", Text: "Always Allow"},
		},
	}

	tests := []struct {
		name     string
		id       string
		expected *PermissionOption
	}{
		{"found allow", "allow", &PermissionOption{ID: "allow", Text: "Allow"}},
		{"found deny", "deny", &PermissionOption{ID: "deny", Text: "Deny"}},
		{"found always", "always", &PermissionOption{ID: "always", Text: "Always Allow"}},
		{"not found", "unknown", nil},
		{"empty string", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := perm.OptionByID(tt.id)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.Text, result.Text)
			}
		})
	}
}

func TestPermissionRequest_OptionByIndex(t *testing.T) {
	perm := PermissionRequest{
		Options: []PermissionOption{
			{ID: "allow", Text: "Allow"},
			{ID: "deny", Text: "Deny"},
		},
	}

	tests := []struct {
		name     string
		index    int
		expected *PermissionOption
	}{
		{"first option", 1, &PermissionOption{ID: "allow", Text: "Allow"}},
		{"second option", 2, &PermissionOption{ID: "deny", Text: "Deny"}},
		{"zero out of range", 0, nil},
		{"negative out of range", -1, nil},
		{"overflow", 3, nil},
		{"large overflow", 100, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := perm.OptionByIndex(tt.index)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.Text, result.Text)
			}
		})
	}
}
