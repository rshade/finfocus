package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Sentinel errors for checksum verification.
var (
	// ErrAssetNotInChecksums is returned when the asset name is not listed in the checksums file.
	ErrAssetNotInChecksums = errors.New("asset not listed in checksums file")

	// ErrMalformedChecksums is returned when the checksums file contains no valid entries.
	ErrMalformedChecksums = errors.New("checksums file contains no valid entries")

	// ErrChecksumMismatch is returned when the computed hash does not match the expected hash.
	ErrChecksumMismatch = errors.New("checksum mismatch")
)

// sha256HexLen is the expected length of a SHA256 hash in lowercase hexadecimal.
const sha256HexLen = 64

// computeSHA256 computes the SHA256 hash of the file at filePath and returns
// it as a lowercase hexadecimal string. It streams the file content through
// the hasher via io.Copy to avoid loading the entire file into memory.
func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, copyErr := io.Copy(h, f); copyErr != nil {
		return "", fmt.Errorf("computing SHA256: %w", copyErr)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum computes the SHA256 hash of the file at filePath and compares
// it against expectedHash. The expectedHash is normalized to lowercase before
// comparison. Returns nil if the hashes match, or an error wrapping
// ErrChecksumMismatch if they differ.
func VerifyChecksum(filePath, expectedHash string) error {
	actual, err := computeSHA256(filePath)
	if err != nil {
		return err
	}

	expected := strings.ToLower(expectedHash)
	if actual != expected {
		return fmt.Errorf("%w for %s: expected %s, got %s",
			ErrChecksumMismatch, filePath, expected, actual)
	}

	return nil
}

// ParseChecksumsFile parses a SHA256SUMS-format byte slice and returns the hash
// for the named asset. It supports GNU (two-space), BSD (single-space), and
// binary-mode (*-prefixed filename) formats. Blank lines and lines starting
// with '#' are skipped. Lines with invalid hash lengths or non-hex characters
// are also skipped. Returns ErrMalformedChecksums if no valid entries are found,
// or ErrAssetNotInChecksums if the asset name is not present.
func ParseChecksumsFile(data []byte, assetName string) (string, error) {
	lines := strings.Split(string(data), "\n")
	validEntries := 0

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on whitespace (handles 1 space, 2 spaces, tabs)
		fields := strings.Fields(line)
		if len(fields) < 2 { //nolint:mnd // need hash + filename
			continue
		}

		hash := fields[0]
		filename := fields[1]

		// Validate hash: must be exactly 64 hex characters
		if len(hash) != sha256HexLen {
			continue
		}
		if _, err := hex.DecodeString(hash); err != nil {
			continue
		}

		validEntries++

		// Strip leading '*' from filename (binary mode indicator)
		filename = strings.TrimPrefix(filename, "*")

		if filename == assetName {
			return strings.ToLower(hash), nil
		}
	}

	if validEntries == 0 {
		return "", ErrMalformedChecksums
	}

	return "", ErrAssetNotInChecksums
}

// FindChecksumAsset searches a release's assets for a checksums file.
// It performs a case-insensitive match for the asset name "checksums.txt".
// Returns the first matching asset, or nil if no checksums asset is found.
func FindChecksumAsset(release *GitHubRelease) *ReleaseAsset {
	if release == nil || len(release.Assets) == 0 {
		return nil
	}
	for i := range release.Assets {
		if strings.EqualFold(release.Assets[i].Name, "checksums.txt") {
			return &release.Assets[i]
		}
	}
	return nil
}
