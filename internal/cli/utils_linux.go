//go:build linux

package cli

import (
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// isGo126OrLater returns true if the Go version is 1.26 or later.
// Go 1.26 changed from fork to vfork for exec in multi-threaded programs,
// which has complex interactions with Pdeathsig.
func isGo126OrLater() bool {
	ver := runtime.Version()
	// "go1.26.1" -> "1.26"
	if strings.HasPrefix(ver, "go") {
		ver = ver[2:]
	}
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return false
	}
	major, _ := strconv.Atoi(parts[0])
	minorStr := strings.Split(parts[1], "rc")[0] // strip rc suffix if present
	minor, _ := strconv.Atoi(minorStr)
	if major > 1 {
		return true
	}
	return major == 1 && minor >= 26
}

var vforkInUse = isGo126OrLater()

// setPdeathsig sets the parent death signal on Linux
// When the parent process dies, the kernel sends SIGTERM to the child.
// Disabled on Go 1.26+: vfork is used instead of fork, and Pdeathsig
// can cause the signal to be delivered prematurely in multi-threaded contexts.
func setPdeathsig(attrs *syscall.SysProcAttr) {
	if !vforkInUse {
		attrs.Pdeathsig = syscall.SIGTERM
	}
}

// setSetpgid sets the process group ID on Linux
// Safe to use on all Go versions: the attribute is inherited by the child
// and applied after vfork returns, before exec.
func setSetpgid(attrs *syscall.SysProcAttr) {
	attrs.Setpgid = true
}
