package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// expandHome expands ~ to user's home directory
// This is a shared utility function used across multiple CLI adapters
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
