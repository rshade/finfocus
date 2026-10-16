package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// acquirePathLock acquires the cross-process lock used by dismissal writes and
// configuration patches. The returned function releases the lock.
func acquirePathLock(targetPath string) (func(), error) {
	lockPath := targetPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	const maxRetries = 10
	const retryDelay = 100 * time.Millisecond
	const staleLockAge = 30 * time.Second

	for range maxRetries {
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(file, "%d", os.Getpid())
			_ = file.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		if removeStaleLock(lockPath, staleLockAge) {
			continue
		}
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("could not acquire lock on %s after retries", lockPath)
}
