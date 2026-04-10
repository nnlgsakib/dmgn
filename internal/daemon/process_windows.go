//go:build windows

package daemon

import (
	"os"
	"syscall"
)

const (
	_DETACHED_PROCESS = 0x00000008
)

func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: _DETACHED_PROCESS,
	}
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds. Use Signal(0) trick:
	// sending nil signal checks process existence.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func signalProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// Windows doesn't support SIGTERM; use Kill
	return proc.Kill()
}

func forceKillProcess(proc *os.Process) error {
	return proc.Kill()
}
