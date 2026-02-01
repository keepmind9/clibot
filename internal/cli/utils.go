package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandHome expands ~ to user's home directory
// This is a shared utility function used across multiple CLI adapters
// Returns an error if the home directory cannot be determined
func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
