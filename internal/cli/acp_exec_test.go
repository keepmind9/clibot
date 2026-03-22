package cli

import (
	"sync"
	"testing"
)

func TestBuildShellCommandExecInGoroutine(t *testing.T) {
	// Simulate multi-goroutine environment
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := buildShellCommand("echo hello")
			out, err := cmd.CombinedOutput()
			t.Logf("[goroutine] out=%q err=%v", string(out), err)
		}()
	}

	// Also test from the main goroutine
	cmd := buildShellCommand("echo hello from main")
	out, err := cmd.CombinedOutput()
	t.Logf("[main] out=%q err=%v", string(out), err)

	wg.Wait()
}
