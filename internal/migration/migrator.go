package migration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DetectLegacy checks if the legacy ~/.finfocus directory exists.
func DetectLegacy() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	legacyPath := filepath.Join(home, ".finfocus")
	info, err := os.Stat(legacyPath)
	if err != nil {
		return "", false
	}
	return legacyPath, info.IsDir()
}

// GetNewPath returns the path to the new ~/.finfocus directory.
func GetNewPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".finfocus"), nil
}

// SafeCopy recursively copies the source directory to the destination.
// It preserves original data by performing a copy instead of a move.
func SafeCopy(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path to source root
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target)
	})
}

// RunMigration performs an interactive migration of a legacy configuration directory
// to the new location.
//
// RunMigration writes status and prompt messages to out and reads the user's response
// from in. If no legacy configuration is detected, if the new path already exists,
// or if the user declines the prompt, the function returns nil and no changes are made.
// If obtaining the new path fails, the function returns that error. If the copy of the
// legacy configuration to the new location fails, the function returns an error
// wrapping the underlying copy failure.
//
// out is used for writing prompts and status messages. in is used for reading the
// user's response to the migration prompt.
func RunMigration(out io.Writer, in io.Reader) error {
	legacyPath, exists := DetectLegacy()
	if !exists {
		return nil
	}

	newPath, err := GetNewPath()
	if err != nil {
		return err
	}

	// If new path already exists, don't prompt for migration
	if _, statErr := os.Stat(newPath); statErr == nil {
		return nil
	}

	fmt.Fprintf(out, "Detected legacy configuration at %s.\n", legacyPath)
	fmt.Fprintf(out, "Would you like to migrate to %s? [y/N] ", newPath)

	var response string
	if _, scanErr := fmt.Fscanln(in, &response); scanErr != nil {
		// If we can't read input, treat as "no"
		response = ""
	}
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" && response != "yes" {
		fmt.Fprintln(out, "Migration skipped. Legacy configuration will be ignored "+
			"unless FINFOCUS_COMPAT=1 is set.")
		return nil
	}

	fmt.Fprintln(out, "Migrating configuration...")
	if copyErr := SafeCopy(legacyPath, newPath); copyErr != nil {
		return fmt.Errorf("migration failed: %w", copyErr)
	}

	fmt.Fprintf(out, "Migration complete. Your old config has been preserved at %s.\n", legacyPath)
	return nil
}

// copyFile copies the file at src to dst, creating any missing parent directories
// and preserving the source file's permission bits.
//
// The src and dst parameters are filesystem paths. If necessary, parent directories
// of dst are created with mode 0700. The destination file is created or truncated
// and its contents are replaced with those from src. The returned error is non-nil
// if any filesystem operation (opening, creating, copying, stat'ing, or chmod'ing)
// fails.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure parent directory exists
	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0700); mkdirErr != nil {
		return mkdirErr
	}

	destFile, createErr := os.Create(dst)
	if createErr != nil {
		return createErr
	}
	defer destFile.Close()

	if _, copyErr := io.Copy(destFile, sourceFile); copyErr != nil {
		return copyErr
	}

	sourceInfo, statErr := os.Stat(src)
	if statErr != nil {
		return statErr
	}

	return os.Chmod(dst, sourceInfo.Mode())
}