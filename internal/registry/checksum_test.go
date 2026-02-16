package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty file",
			content: []byte{},
		},
		{
			name:    "simple content",
			content: []byte("hello world"),
		},
		{
			name:    "binary content",
			content: []byte{0x00, 0xFF, 0x42, 0xDE, 0xAD},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "testfile")
			require.NoError(t, os.WriteFile(tmpFile, tt.content, 0644))

			got, err := computeSHA256(context.Background(), tmpFile)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)

			// Verify against stdlib computation
			h := sha256.Sum256(tt.content)
			expected := hex.EncodeToString(h[:])
			assert.Equal(t, expected, got)
		})
	}
}

func TestComputeSHA256_FileNotFound(t *testing.T) {
	_, err := computeSHA256(context.Background(), "/nonexistent/path/file.bin")
	require.Error(t, err)
}

func TestComputeSHA256_Performance(t *testing.T) {
	// SC-006: Must complete in under 2 seconds for 50 MB file
	tmpFile := filepath.Join(t.TempDir(), "largefile")
	data := make([]byte, 50*1024*1024) // 50 MB
	require.NoError(t, os.WriteFile(tmpFile, data, 0644))

	start := time.Now()
	_, err := computeSHA256(context.Background(), tmpFile)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, duration, 2*time.Second, "computeSHA256 on 50 MB file took %v, exceeds 2s limit", duration)
}

func BenchmarkComputeSHA256_50MB(b *testing.B) {
	tmpFile := filepath.Join(b.TempDir(), "largefile")
	data := make([]byte, 50*1024*1024) // 50 MB
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := computeSHA256(context.Background(), tmpFile); err != nil {
			b.Fatalf("computeSHA256 failed: %v", err)
		}
	}
}

func TestVerifyChecksum(t *testing.T) {
	content := []byte("test file content")
	h := sha256.Sum256(content)
	correctHash := hex.EncodeToString(h[:])
	wrongHash := strings.Repeat("ab", 32) // 64-char hex but wrong

	tests := []struct {
		name         string
		expectedHash string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "matching hash lowercase",
			expectedHash: correctHash,
			wantErr:      false,
		},
		{
			name:         "matching hash uppercase",
			expectedHash: strings.ToUpper(correctHash),
			wantErr:      false,
		},
		{
			name:         "matching hash mixed case",
			expectedHash: strings.ToUpper(correctHash[:32]) + correctHash[32:],
			wantErr:      false,
		},
		{
			name:         "mismatching hash",
			expectedHash: wrongHash,
			wantErr:      true,
			errContains:  "checksum mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "testfile")
			require.NoError(t, os.WriteFile(tmpFile, content, 0644))

			err := VerifyChecksum(context.Background(), tmpFile, tt.expectedHash)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrChecksumMismatch)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestVerifyChecksum_FileNotFound(t *testing.T) {
	err := VerifyChecksum(context.Background(), "/nonexistent/file", strings.Repeat("ab", 32))
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrChecksumMismatch)
}

func TestParseChecksumsFile(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		assetName   string
		wantHash    string
		wantErr     error
		errContains string
	}{
		{
			name:      "GNU format two spaces",
			data:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  my-plugin-linux-amd64.tar.gz\n",
			assetName: "my-plugin-linux-amd64.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:      "BSD format single space",
			data:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890 my-plugin-linux-amd64.tar.gz\n",
			assetName: "my-plugin-linux-amd64.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:      "binary mode with star prefix",
			data:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890 *my-plugin-linux-amd64.tar.gz\n",
			assetName: "my-plugin-linux-amd64.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name: "multiple entries find correct one",
			data: "1111111111111111111111111111111111111111111111111111111111111111  other-file.tar.gz\n" +
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  target-file.tar.gz\n" +
				"2222222222222222222222222222222222222222222222222222222222222222  another-file.tar.gz\n",
			assetName: "target-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name: "skips blank lines and comments",
			data: "# SHA256 checksums\n" +
				"\n" +
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  my-file.tar.gz\n" +
				"\n" +
				"# end of file\n",
			assetName: "my-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name: "skips lines with invalid hash length",
			data: "shorthash  bad-file.tar.gz\n" +
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  good-file.tar.gz\n",
			assetName: "good-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name: "skips lines with non-hex hash",
			data: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  bad-file.tar.gz\n" +
				"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  good-file.tar.gz\n",
			assetName: "good-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:      "uppercase hash is returned lowercase",
			data:      "ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890  my-file.tar.gz\n",
			assetName: "my-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:      "asset not found",
			data:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  other-file.tar.gz\n",
			assetName: "missing-file.tar.gz",
			wantErr:   ErrAssetNotInChecksums,
		},
		{
			name:      "malformed content no valid entries",
			data:      "this is not a checksums file\njust some random text\n",
			assetName: "any-file.tar.gz",
			wantErr:   ErrMalformedChecksums,
		},
		{
			name:      "empty content",
			data:      "",
			assetName: "any-file.tar.gz",
			wantErr:   ErrMalformedChecksums,
		},
		{
			name:      "only blank lines and comments",
			data:      "# comment\n\n# another comment\n\n",
			assetName: "any-file.tar.gz",
			wantErr:   ErrMalformedChecksums,
		},
		{
			name: "first matching entry wins",
			data: "1111111111111111111111111111111111111111111111111111111111111111  dup-file.tar.gz\n" +
				"2222222222222222222222222222222222222222222222222222222222222222  dup-file.tar.gz\n",
			assetName: "dup-file.tar.gz",
			wantHash:  "1111111111111111111111111111111111111111111111111111111111111111",
		},
		{
			name:      "tab-separated",
			data:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890\tmy-file.tar.gz\n",
			assetName: "my-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
		{
			name:      "windows line endings",
			data:      "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  my-file.tar.gz\r\n",
			assetName: "my-file.tar.gz",
			wantHash:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChecksumsFile([]byte(tt.data), tt.assetName)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHash, got)
		})
	}
}

func TestFindChecksumAsset(t *testing.T) {
	tests := []struct {
		name      string
		release   *GitHubRelease
		wantFound bool
		wantName  string
	}{
		{
			name: "finds checksums.txt lowercase",
			release: &GitHubRelease{
				Assets: []ReleaseAsset{
					{Name: "plugin-linux-amd64.tar.gz"},
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
					{Name: "plugin-darwin-arm64.tar.gz"},
				},
			},
			wantFound: true,
			wantName:  "checksums.txt",
		},
		{
			name: "finds CHECKSUMS.TXT uppercase",
			release: &GitHubRelease{
				Assets: []ReleaseAsset{
					{Name: "plugin-linux-amd64.tar.gz"},
					{Name: "CHECKSUMS.TXT", BrowserDownloadURL: "https://example.com/CHECKSUMS.TXT"},
				},
			},
			wantFound: true,
			wantName:  "CHECKSUMS.TXT",
		},
		{
			name: "finds Checksums.txt mixed case",
			release: &GitHubRelease{
				Assets: []ReleaseAsset{
					{Name: "Checksums.txt", BrowserDownloadURL: "https://example.com/Checksums.txt"},
				},
			},
			wantFound: true,
			wantName:  "Checksums.txt",
		},
		{
			name: "no checksums asset",
			release: &GitHubRelease{
				Assets: []ReleaseAsset{
					{Name: "plugin-linux-amd64.tar.gz"},
					{Name: "plugin-darwin-arm64.tar.gz"},
				},
			},
			wantFound: false,
		},
		{
			name: "empty assets",
			release: &GitHubRelease{
				Assets: []ReleaseAsset{},
			},
			wantFound: false,
		},
		{
			name:      "nil release",
			release:   nil,
			wantFound: false,
		},
		{
			name:      "nil assets slice",
			release:   &GitHubRelease{},
			wantFound: false,
		},
		{
			name: "returns first match",
			release: &GitHubRelease{
				Assets: []ReleaseAsset{
					{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/first"},
					{Name: "CHECKSUMS.TXT", BrowserDownloadURL: "https://example.com/second"},
				},
			},
			wantFound: true,
			wantName:  "checksums.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindChecksumAsset(tt.release)
			if tt.wantFound {
				require.NotNil(t, result)
				assert.Equal(t, tt.wantName, result.Name)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}
