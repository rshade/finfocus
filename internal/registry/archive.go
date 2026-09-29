package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// maxFileSize limits decompression to prevent zip bombs (500MB).
	// Plugins with embedded data (e.g., aws-public with pricing data) can be large.
	maxFileSize = 500 * 1024 * 1024
	// dirPerm is the permission for directories.
	dirPerm = 0750
	// execPerm is the permission for executable files.
	execPerm = 0750
)

// ExtractArchive extracts an archive to the destination directory.
// ExtractArchive extracts the archive at archivePath into destDir.
// It supports archives with .tar.gz, .tgz, and .zip extensions.
// archivePath is the path to the archive file to extract.
// destDir is the directory where extracted files will be written.
// An error is returned if the archive format is unsupported or if extraction fails.
func ExtractArchive(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return extractTarGz(archivePath, destDir)
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destDir)
	}
	return fmt.Errorf("unsupported archive format: %s", archivePath)
}

// extractTarGz extracts the contents of a gzip-compressed tar archive into destDir.
//
// It opens the archive at archivePath, iterates over tar entries, and delegates each
// entry to extractTarEntry for writing to disk. The function returns an error if the
// archive file cannot be opened, the gzip reader cannot be created, a tar entry cannot
// be read, or any entry fails to extract.
//
// archivePath is the path to the .tar.gz or .tgz archive to extract.
// destDir is the directory into which archive entries will be written.
//
// The returned error wraps the underlying cause when file access, gzip initialization,
// tar reading, or entry extraction fails.
func extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)

	for {
		header, headerErr := tr.Next()
		if headerErr == io.EOF {
			break
		}
		if headerErr != nil {
			return fmt.Errorf("failed to read tar entry: %w", headerErr)
		}

		if extractErr := extractTarEntry(tr, header, destDir); extractErr != nil {
			return extractErr
		}
	}
	return nil
}

// extractTarEntry extracts a single tar entry named by header into destDir.
// It sanitizes the entry name to prevent path traversal, creates directories for
// directory entries, and delegates regular file extraction to extractTarFile.
// tr is the tar reader positioned at the entry's data; header describes the entry.
// Returns an error if path sanitization fails, directory creation fails, or file
// extraction returns an error. Other tar entry types are ignored and return nil.
func extractTarEntry(tr *tar.Reader, header *tar.Header, destDir string) error {
	target, err := sanitizePath(destDir, header.Name)
	if err != nil {
		return err
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, dirPerm)
	case tar.TypeReg:
		return extractTarFile(tr, header, target)
	}
	return nil
}

// extractTarFile extracts a regular file from the provided tar reader into the
// filesystem at target, enforcing a maximum decompressed size and preserving the
// executable bit when present in the tar header.
//
// Parameters:
//   - tr: tar reader positioned at the file's data (the caller must have read the header).
//   - header: tar header for the file being extracted; used for mode/size information.
//   - target: destination path on disk where the file will be written.
//
// It returns an error if creating parent directories or the destination file fails,
// if copying the file data fails or writes more than maxFileSize bytes, if closing
// the output file fails, or if setting executable permissions fails.
func extractTarFile(tr *tar.Reader, header *tar.Header, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	outFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	written, copyErr := io.CopyN(outFile, tr, maxFileSize+1)
	closeErr := outFile.Close()

	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return fmt.Errorf("failed to extract file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close file: %w", closeErr)
	}
	if written > maxFileSize {
		return fmt.Errorf("file exceeds maximum size: %s", target)
	}

	// Preserve executable bit
	if header.Mode&0111 != 0 {
		if chmodErr := os.Chmod(target, execPerm); chmodErr != nil {
			return fmt.Errorf("failed to set permissions: %w", chmodErr)
		}
	}
	return nil
}

// extractZip opens the zip archive at archivePath and extracts its files into destDir.
// It returns an error if the archive cannot be opened or if extracting any entry fails.
func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if extractErr := extractZipFile(f, destDir); extractErr != nil {
			return extractErr
		}
	}
	return nil
}

// extractZipFile extracts a single entry from the provided [zip.File] into destDir.
// It sanitizes the zip entry path to prevent path traversal, creates any required
// parent directories, and writes the entry to disk.
//
// Parameters:
//   - f: zip entry to extract.
//   - destDir: destination directory into which the entry will be written.
//
// Returns an error if path sanitization fails, directory creation fails, or writing
// the file fails.
func extractZipFile(f *zip.File, destDir string) error {
	target, err := sanitizePath(destDir, f.Name)
	if err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, dirPerm)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(target), dirPerm); mkdirErr != nil {
		return fmt.Errorf("failed to create parent directory: %w", mkdirErr)
	}

	return writeZipFile(f, target)
}

// writeZipFile writes the zip file entry f to the given target path.
// f is the zip entry to extract; target is the destination file path on disk.
// It returns an error if creating or writing the destination fails, if the extracted
// data exceeds the configured maximum file size, if closing streams fails, or if
// setting executable permissions (when the entry has executable bits) fails.
func writeZipFile(f *zip.File, target string) error {
	outFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		_ = outFile.Close()
		return fmt.Errorf("failed to open file in zip: %w", err)
	}

	written, copyErr := io.CopyN(outFile, rc, maxFileSize+1)
	_ = rc.Close()
	closeErr := outFile.Close()

	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return fmt.Errorf("failed to extract file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close file: %w", closeErr)
	}
	if written > maxFileSize {
		return fmt.Errorf("file exceeds maximum size: %s", target)
	}

	// Set executable permission on Unix
	if f.Mode()&0111 != 0 {
		if chmodErr := os.Chmod(target, execPerm); chmodErr != nil {
			return fmt.Errorf("failed to set permissions: %w", chmodErr)
		}
	}
	return nil
}

// sanitizePath validates and sanitizes an archive entry path to ensure it resolves inside the provided destination directory.
// destDir is the extraction target directory and name is the archive entry path to join to destDir.
// It returns the joined, cleaned path that is guaranteed to lie within destDir, or an error if the resulting path would escape destDir.
func sanitizePath(destDir, name string) (string, error) {
	// Clean the destination directory first
	cleanDest := filepath.Clean(destDir)
	// Join with the name from the archive
	target := filepath.Join(cleanDest, name)
	// Ensure the target is within the destination
	if !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) && target != cleanDest {
		return "", fmt.Errorf("invalid file path in archive: %s", name)
	}
	return target, nil
}

// ValidateBinary verifies that the file at binaryPath exists and is a runnable binary for the current platform.
// On Windows this requires the path to end with the `.exe` extension (case-insensitive).
// On non-Windows platforms this requires at least one executable permission bit to be set.
// Returns an error if the path does not exist, points to a directory, lacks the required extension or executable bits, or if other file stat errors occur.
func ValidateBinary(binaryPath string) error {
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("expected file but found directory: %s", binaryPath)
	}

	// On Windows, check for .exe extension
	if runtime.GOOS == osWindows {
		if !strings.HasSuffix(strings.ToLower(binaryPath), ".exe") {
			return fmt.Errorf("windows binary must have .exe extension: %s", binaryPath)
		}
		return nil
	}

	// On Unix, check executable permission
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable: %s", binaryPath)
	}

	return nil
}
