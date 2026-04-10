package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SpawnDaemon spawns a detached child process.
// Returns the child PID on success.
func SpawnDaemon(execPath string, args []string, env []string) (int, error) {
	cmd := exec.Command(execPath, args...)
	cmd.Env = env
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = detachAttrs()

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to spawn daemon: %w", err)
	}

	// Release so the parent doesn't wait for the child
	go cmd.Wait()

	return cmd.Process.Pid, nil
}

// StopDaemon stops a running daemon by PID file.
// Sends a graceful signal first, then force kills after timeout.
// Cleans up PID and port files.
func StopDaemon(pidPath string, portPath string, timeout time.Duration) error {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return fmt.Errorf("cannot read daemon PID: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process not found, clean up stale files
		RemovePID(pidPath)
		os.Remove(portPath)
		return nil
	}

	// Send graceful signal
	if err := signalProcess(pid); err != nil {
		// Process may already be gone
		RemovePID(pidPath)
		os.Remove(portPath)
		return nil
	}

	// Poll for process exit
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			RemovePID(pidPath)
			os.Remove(portPath)
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	// Force kill
	if err := forceKillProcess(proc); err != nil {
		return fmt.Errorf("failed to force kill daemon (PID %d): %w", pid, err)
	}

	RemovePID(pidPath)
	os.Remove(portPath)
	return nil
}

// WaitForHealthy polls until the PID file exists and the process is alive.
func WaitForHealthy(pidPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pid, running := CheckDaemonRunning(pidPath); running && pid > 0 {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not start within %s", timeout)
}
