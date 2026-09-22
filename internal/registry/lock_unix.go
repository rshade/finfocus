//go:build !windows

package registry

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// lockFile acquires an exclusive lock for the named plugin by atomically
// creating a lock file (os.O_CREATE|os.O_EXCL). If a stale lock file is found
// (the owning process is no longer running), it is removed and acquisition is
// retried. The returned function releases the lock.
func lockFile(lockPath, name string) (func(), error) {
	file, err := tryCreateLockFile(lockPath, name)
	if err != nil {
		return nil, err
	}

	// Write our PID to the lock file for stale lock detection
	_, _ = file.WriteString(strconv.Itoa(os.Getpid()))
	_ = file.Close()

	return func() {
		_ = os.Remove(lockPath)
	}, nil
}

// tryCreateLockFile attempts to create an exclusive lock file.
// If the lock file exists and is stale, it removes the stale lock and retries.
func tryCreateLockFile(lockPath, name string) (*os.File, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		return file, nil
	}

	if !os.IsExist(err) {
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	// Lock file exists - check if it's stale
	if !isLockStale(lockPath) {
		return nil, fmt.Errorf(
			"plugin %q is currently being modified by another process (lock file %s exists)",
			name, lockPath)
	}

	// Remove stale lock and try again
	_ = os.Remove(lockPath)
	file, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock file after removing stale lock: %w", err)
	}
	return file, nil
}

// isLockStale checks if a lock file is stale by reading the PID from it
// and checking if that process is still running.
func isLockStale(lockPath string) bool {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		// Can't read the file - assume not stale to be safe
		return false
	}

	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		// Empty lock file (legacy or corrupt) - treat as stale
		return true
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Invalid PID - treat as stale
		return true
	}

	// Check if process is still running
	return !isProcessRunning(pid)
}

// isProcessRunning checks if a process with the given PID is still running.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0 to
	// check if the process actually exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
