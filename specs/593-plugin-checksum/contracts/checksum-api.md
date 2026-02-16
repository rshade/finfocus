# API Contract: Checksum Verification

**Package**: `internal/registry`
**File**: `checksum.go`

## Exported Functions

### VerifyChecksum

Computes the SHA256 hash of a file and compares it against an expected hash.

```go
func VerifyChecksum(filePath, expectedHash string) error
```

**Parameters**:

| Name | Type | Description |
| ---- | ---- | ----------- |
| filePath | string | Absolute path to the file to verify |
| expectedHash | string | Expected SHA256 hash (64-char lowercase hex) |

**Returns**:

| Condition | Return |
| --------- | ------ |
| Hash matches | `nil` |
| File cannot be opened | `error` wrapping OS error |
| Hash mismatch | `error` containing expected and actual hashes |

**Behavior**:

- Streams file content through `sha256.New()` via `io.Copy` (no full-file buffering)
- Normalizes `expectedHash` to lowercase before comparison
- Error message format for mismatch: `"checksum mismatch for %s: expected %s, got %s"`

### ParseChecksumsFile

Parses a SHA256SUMS-format file and returns the hash for a named asset.

```go
func ParseChecksumsFile(data []byte, assetName string) (string, error)
```

**Parameters**:

| Name | Type | Description |
| ---- | ---- | ----------- |
| data | []byte | Raw content of the checksums file |
| assetName | string | Name of the asset to find (exact match) |

**Returns**:

| Condition | Return |
| --------- | ------ |
| Asset found | `(hash string, nil)` |
| Asset not found | `("", ErrAssetNotInChecksums)` |
| No valid entries | `("", ErrMalformedChecksums)` |

**Behavior**:

- Splits content by newlines, processes each line
- Skips blank lines and lines starting with `#`
- Splits each line on whitespace (tolerates 1 or 2+ spaces)
- Strips leading `*` from filename (binary mode indicator)
- Hash must be exactly 64 hex characters; lines with invalid hashes are skipped
- First matching entry wins (if duplicates exist)

### FindChecksumAsset

Searches a release's assets for a checksums file.

```go
func FindChecksumAsset(release *GitHubRelease) *ReleaseAsset
```

**Parameters**:

| Name | Type | Description |
| ---- | ---- | ----------- |
| release | *GitHubRelease | The release to search |

**Returns**:

| Condition | Return |
| --------- | ------ |
| checksums.txt found | `*ReleaseAsset` pointer |
| Not found | `nil` |

**Behavior**:

- Iterates `release.Assets`
- Case-insensitive match for asset name `checksums.txt`
- Returns first match

## Sentinel Errors

```go
var ErrAssetNotInChecksums = errors.New("asset not listed in checksums file")
var ErrMalformedChecksums = errors.New("checksums file contains no valid entries")
var ErrChecksumMismatch = errors.New("checksum mismatch")
```

## Internal (Unexported) Functions

### computeSHA256

Computes SHA256 hash of a file, returning lowercase hex string.

```go
func computeSHA256(filePath string) (string, error)
```

Used internally by `VerifyChecksum`. Separated for testability.

## Integration Contract: installRelease Modification

The existing `installRelease()` method gains checksum verification between the download
and extraction steps.

**Insertion point**: After `i.client.DownloadAsset()` succeeds, before `os.MkdirAll()`.

**Pseudocode**:

```text
IF NOT opts.SkipChecksum:
    checksumAsset = FindChecksumAsset(release)
    IF checksumAsset is nil:
        progress("Warning: checksums.txt not found, skipping verification")
    ELSE:
        download checksumAsset to temp file
        parse checksums file for downloaded asset name
        IF asset not found in checksums:
            progress("Warning: asset not listed in checksums file")
        ELSE:
            verify checksum of downloaded binary
            IF mismatch:
                RETURN error (installation blocked)
            progress("Checksum verified")
```

**Error handling**: Network errors fetching checksums.txt produce a warning, not a fatal
error. Only a confirmed hash mismatch is fatal.
