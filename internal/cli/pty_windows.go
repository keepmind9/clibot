//go:build windows

package cli

import (
	"fmt"
	"os/exec"
)

// killProcessGroup terminates the process on Windows.
//
// Windows ConPTY Limitations:
// - Child processes may not be terminated when parent is killed
// - Orphan processes will be adopted by system and cleaned up eventually
// - For reliable child cleanup, use WSL 2 instead
//
// Recommendations for Windows users:
//  1. WSL 2: Best for full Unix compatibility (recommended)
//     wsl -e claude
//  2. ConPTY: Good for simple tools (current implementation)
//     Requires: Windows 10 1809+ (build 17763, Oct 2018)
//  3. Docker: Best for isolated environments
//     docker run -it --rm claude-code
//
// For complex CLI tools (vim, tmux, screen), use WSL 2.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	// Try to kill the process
	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill windows process: %w", err)
	}

	return nil
}
