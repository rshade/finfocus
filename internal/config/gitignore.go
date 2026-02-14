package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// gitignoreContent is the standard .gitignore content for project-local .finfocus/ directories.
const gitignoreContent = `# FinFocus project-local data (auto-generated)
# Config is tracked; user-specific state is not.
dismissed.json
dismissed.json.lock
dismissed.json.tmp
*.log
`

// GitignoreContent returns the standard .gitignore content used for
// project-local .finfocus/ directories. Exported for testing.
func GitignoreContent() string {
	return gitignoreContent
}

// EnsureGitignore creates a .gitignore file in the given directory if one
// does not already exist. Returns true if a new file was created, false if
// one already existed. Never overwrites an existing .gitignore (FR-007).
func EnsureGitignore(dir string) (bool, error) {
	gitignorePath := filepath.Join(dir, ".gitignore")

	_, err := os.Stat(gitignorePath)
	if err == nil {
		// File already exists — do not overwrite.
		return false, nil
	}

	if !os.IsNotExist(err) {
		// Unexpected stat error (e.g. permission denied on the file itself).
		return false, fmt.Errorf("checking .gitignore at %s: %w", gitignorePath, err)
	}

	// Ensure the parent directory exists.
	if mkdirErr := os.MkdirAll(dir, 0o750); mkdirErr != nil {
		return false, fmt.Errorf("creating directory %s: %w", dir, mkdirErr)
	}

	//nolint:gosec // .gitignore must be world-readable (0644).
	if writeErr := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); writeErr != nil {
		return false, fmt.Errorf("writing .gitignore at %s: %w", gitignorePath, writeErr)
	}

	return true, nil
}
