//go:build windows

package stdio

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func terminateSignal() syscall.Signal {
	return syscall.SIGTERM
}
