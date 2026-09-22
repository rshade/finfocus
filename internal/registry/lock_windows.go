//go:build windows

package registry

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/windows"
)

// lockFile acquires an exclusive lock for the named plugin using Windows
// byte-range locking (LockFileEx). Unlike file-existence-based locking
// (os.O_CREATE|os.O_EXCL), the lock is held on the open file handle and is
// released automatically by the OS when the handle closes or the owning
// process exits, so no stale-lock detection is needed. Lock requests fail
// immediately (LOCKFILE_FAIL_IMMEDIATELY) rather than blocking.
//
// Windows byte-range locks are associated with the file handle, so lock
// conflicts are detected both across processes and between concurrent
// goroutines in the same process.
func lockFile(lockPath, name string) (func(), error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	handle := windows.Handle(file.Fd())
	err = windows.LockFileEx(
		handle,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&windows.Overlapped{},
	)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf(
			"plugin %q is currently being modified by another process (lock file %s exists)",
			name, lockPath)
	}

	// Write our PID to the lock file for diagnostics only; the lock itself is
	// held on the handle, so the content is not used for stale detection.
	_, _ = file.WriteString(strconv.Itoa(os.Getpid()))

	return func() {
		_ = windows.UnlockFileEx(handle, 0, 1, 0, &windows.Overlapped{})
		_ = file.Close()
		_ = os.Remove(lockPath)
	}, nil
}
