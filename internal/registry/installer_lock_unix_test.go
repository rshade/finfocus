//go:build !windows

package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsProcessRunning(t *testing.T) {
	// Test with current process - should be running
	currentPID := os.Getpid()
	assert.True(t, isProcessRunning(currentPID), "Expected current process to be running")

	// Test with invalid PID - should not be running
	assert.False(t, isProcessRunning(99999999), "Expected invalid PID to not be running")

	// Test with PID 0 - typically kernel, but behavior varies
	// Just ensure it doesn't panic
	_ = isProcessRunning(0)
}

func TestIsLockStale(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty file is stale",
			content:  "",
			expected: true,
		},
		{
			name:     "whitespace only is stale",
			content:  "   \n  ",
			expected: true,
		},
		{
			name:     "invalid PID is stale",
			content:  "not-a-number",
			expected: true,
		},
		{
			name:     "very large PID is stale",
			content:  "99999999",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockPath := filepath.Join(tmpDir, "test-"+tt.name+".lock")
			require.NoError(t, os.WriteFile(lockPath, []byte(tt.content), 0600), "Failed to create lock file")

			result := isLockStale(lockPath)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test with non-existent file - should not be stale (safe default)
	assert.False(
		t,
		isLockStale(filepath.Join(tmpDir, "nonexistent.lock")),
		"Non-existent lock file should not be considered stale",
	)
}
