package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// pluginLockMutexes serializes lock acquisition within a single process,
// keyed by lock file path. It complements the platform file lock by giving
// concurrent goroutines in the same process deterministic try-lock semantics
// instead of racing on file creation (which is unreliable on Windows due to
// delayed file handle release).
var pluginLockMutexes sync.Map //nolint:gochecknoglobals // Process-wide lock registry shared by all Installer instances.

// acquireLock attempts to acquire a lock for the specified plugin name.
// It returns a function to release the lock, or an error if the lock is
// already held by another goroutine or process.
//
// Locking is two-tiered: an in-process try-mutex serializes goroutines within
// this process, and a platform-specific file lock (see lock_unix.go and
// lock_windows.go) protects against concurrent processes.
func (i *Installer) acquireLock(name string) (func(), error) {
	// Ensure plugin base directory exists
	if err := os.MkdirAll(i.pluginDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	lockPath := filepath.Join(i.pluginDir, name+".lock")

	mu := mutexForLockPath(lockPath)
	if !mu.TryLock() {
		return nil, fmt.Errorf(
			"plugin %q is currently being modified by another process (lock file %s exists)",
			name, lockPath)
	}

	unlock, err := lockFile(lockPath, name)
	if err != nil {
		mu.Unlock()
		return nil, err
	}

	return func() {
		unlock()
		mu.Unlock()
	}, nil
}

// mutexForLockPath returns the in-process mutex guarding the given lock file
// path, creating it on first use.
func mutexForLockPath(lockPath string) *sync.Mutex {
	actual, _ := pluginLockMutexes.LoadOrStore(lockPath, &sync.Mutex{})
	mu, _ := actual.(*sync.Mutex)
	return mu
}
