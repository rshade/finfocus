package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		destDir string
		path    string
		wantErr bool
	}{
		{
			name:    "valid path",
			destDir: tmpDir,
			path:    "subdir/file.txt",
			wantErr: false,
		},
		{
			name:    "simple filename",
			destDir: tmpDir,
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "zip-slip attempt",
			destDir: tmpDir,
			path:    "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "absolute path is made relative",
			destDir: tmpDir,
			path:    "/etc/passwd",
			wantErr: false, // filepath.Join makes absolute paths relative
		},
		{
			name:    "hidden path traversal",
			destDir: tmpDir,
			path:    "foo/../../../etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sanitizePath(tt.destDir, tt.path)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create executable file
	execPath := filepath.Join(tmpDir, "executable")
	require.NoError(t, os.WriteFile(execPath, []byte("binary"), 0750))

	// Create non-executable file
	nonExecPath := filepath.Join(tmpDir, "nonexec")
	require.NoError(t, os.WriteFile(nonExecPath, []byte("data"), 0644))

	// Create directory
	dirPath := filepath.Join(tmpDir, "directory")
	require.NoError(t, os.MkdirAll(dirPath, 0750))

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid executable",
			path:    execPath,
			wantErr: runtime.GOOS == "windows", // On Windows, needs .exe
		},
		{
			name:    "directory",
			path:    dirPath,
			wantErr: true,
		},
		{
			name:    "non-existent",
			path:    filepath.Join(tmpDir, "nonexistent"),
			wantErr: true,
		},
	}

	// On Unix, non-executable should fail
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name    string
			path    string
			wantErr bool
		}{
			name:    "non-executable file",
			path:    nonExecPath,
			wantErr: true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBinary(tt.path)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractArchive(t *testing.T) {
	// Create test tar.gz archive
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "test.tar.gz")
	destDir := filepath.Join(tmpDir, "extracted")

	// Create a simple tar.gz file
	createTestTarGz(t, tarPath, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	})

	require.NoError(t, os.MkdirAll(destDir, 0750))

	err := ExtractArchive(tarPath, destDir)
	require.NoError(t, err)

	// Verify files extracted
	assert.FileExists(t, filepath.Join(destDir, "file1.txt"))
}

func TestExtractArchiveZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "extracted")

	// Create a simple zip file
	createTestZip(t, zipPath, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	})

	require.NoError(t, os.MkdirAll(destDir, 0750))

	err := ExtractArchive(zipPath, destDir)
	require.NoError(t, err)

	// Verify files extracted
	assert.FileExists(t, filepath.Join(destDir, "file1.txt"))
}

func TestExtractArchiveUnsupported(t *testing.T) {
	tmpDir := t.TempDir()
	unsupportedPath := filepath.Join(tmpDir, "test.rar")
	destDir := filepath.Join(tmpDir, "extracted")

	require.NoError(t, os.WriteFile(unsupportedPath, []byte("fake"), 0644))

	err := ExtractArchive(unsupportedPath, destDir)
	require.Error(t, err, "expected error for unsupported archive format")
}

func TestExtractArchiveNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	err := ExtractArchive(filepath.Join(tmpDir, "nonexistent.tar.gz"), tmpDir)
	require.Error(t, err, "expected error for non-existent archive")
}

func TestMaxFileSizeBoundary(t *testing.T) {
	// Test that maxFileSize constant is set to 500MB
	expectedSize := 500 * 1024 * 1024
	assert.Equal(t, expectedSize, maxFileSize, "maxFileSize should be 500MB")

	// Test that the boundary is reasonable (greater than 100MB, less than 1GB)
	assert.Greater(t, maxFileSize, 100*1024*1024, "maxFileSize should be greater than 100MB for plugin compatibility")
	assert.Less(t, maxFileSize, 1024*1024*1024, "maxFileSize should be less than 1GB to prevent excessive memory usage")
}

// Helper to create test tar.gz archives.
func createTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
}

// Helper to create test zip archives.
func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
}
