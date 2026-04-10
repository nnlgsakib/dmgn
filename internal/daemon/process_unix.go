//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, sending signal 0 checks if process exists
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func signalProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func forceKillProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGKILL)
}
