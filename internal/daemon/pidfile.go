package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// WritePID writes the current process PID to the given file path.
func WritePID(pidPath string) error {
	pid := os.Getpid()
	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0600)
}

// ReadPID reads a PID from the given file path.
// Returns 0 and an error if the file doesn't exist or is malformed.
func ReadPID(pidPath string) (int, error) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("malformed PID file: %w", err)
	}

	return pid, nil
}

// CheckDaemonRunning checks if a daemon process is running.
// Returns (pid, true) if the daemon is alive, (0, false) otherwise.
func CheckDaemonRunning(pidPath string) (int, bool) {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return 0, false
	}

	if !isProcessAlive(pid) {
		return 0, false
	}

	return pid, true
}

// RemovePID removes the PID file, ignoring not-exist errors.
func RemovePID(pidPath string) error {
	err := os.Remove(pidPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
