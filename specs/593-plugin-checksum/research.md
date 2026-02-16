# Research: Plugin Checksum Verification

**Feature**: 593-plugin-checksum
**Date**: 2026-02-15

## R1: SHA256 Computation in Go

**Decision**: Use `crypto/sha256` from Go stdlib with streaming `io.Copy` into hasher.

**Rationale**: The stdlib `crypto/sha256` package is the idiomatic Go approach. It
implements `hash.Hash` which satisfies `io.Writer`, enabling streaming hash computation
without loading the entire file into memory. This is important for large plugin archives
(up to 500 MB per the existing `maxFileSize` constant in `archive.go`).

**Alternatives considered**:

- `crypto/sha512`: Stronger but SHA256 is the industry standard for checksums files and
  matches the SHA256SUMS format specified in the feature spec.
- Third-party libraries (e.g., `minio/sha256-simd`): Faster on some platforms but adds
  an external dependency. The performance difference is negligible for file sizes under
  500 MB given the <2s target.

**Implementation pattern**:

```go
f, _ := os.Open(filePath)
defer f.Close()
h := sha256.New()
io.Copy(h, f)
hex.EncodeToString(h.Sum(nil))
```

## R2: SHA256SUMS File Format Parsing

**Decision**: Line-by-line parsing with whitespace splitting (support both single and
double space separators).

**Rationale**: The SHA256SUMS format is `<64-hex-char-hash><whitespace><filename>` with
one entry per line. The GNU `sha256sum` utility uses two spaces, but BSD `shasum -a 256`
uses a single space. Some tools insert `*` before the filename for binary mode. Tolerant
parsing handles all common variants.

**Alternatives considered**:

- Strict two-space-only parsing: Would break for BSD-generated files.
- Regex-based parsing: Overkill for this simple format and slower.
- JSON-based checksums: Non-standard, would require plugin publishers to change tooling.

**Format variants handled**:

| Tool | Output format | Supported |
| ---- | ------------- | --------- |
| GNU sha256sum | `hash  filename` (2 spaces) | Yes |
| BSD shasum -a 256 | `hash filename` (1 space) | Yes |
| Binary mode | `hash *filename` | Yes (strip `*` prefix) |
| Comments/blank lines | `# comment` or empty | Yes (skip) |

## R3: Checksums File Discovery in GitHub Releases

**Decision**: Search `release.Assets` for an asset named `checksums.txt` (case-insensitive).

**Rationale**: The spec assumes `checksums.txt` as the filename. GitHub releases expose
assets as `[]ReleaseAsset` with a `Name` field and `BrowserDownloadURL`. We search the
existing release object (already fetched) for the checksums asset rather than making an
additional API call. The existing `DownloadAsset()` method handles the download.

**Alternatives considered**:

- GitHub API checksum headers: GitHub provides `Content-SHA256` headers for some assets
  but this is not universally available and doesn't cover the full SHA256SUMS use case.
- Separate API call for checksums: Unnecessary since the release object already contains
  all assets.
- Multiple filename patterns (`SHA256SUMS`, `sha256sums.txt`): Adds complexity. Starting
  with `checksums.txt` (case-insensitive) covers the primary use case and can be extended
  later.

## R4: Integration Point in Install Pipeline

**Decision**: Insert checksum verification after download, before extraction.

**Rationale**: The `installRelease()` method in `installer.go` follows this flow:

1. Find platform asset
2. Create temp file
3. Download asset to temp file
4. Create install directory
5. Extract archive
6. Find and validate binary

Checksum verification belongs between steps 3 and 4. At this point the downloaded file
exists as a temp file and can be hashed. If verification fails, cleanup is simple (the
temp file is already deferred for removal, and the install directory hasn't been created
yet).

**Alternatives considered**:

- During download (streaming hash): Would require modifying `DownloadAsset()` to accept
  an `io.Writer` tee. More efficient but more invasive. Can be optimized later.
- After extraction: Too late; potentially malicious code has already been written to disk.
- Before download (pre-fetch checksums): Adds latency by serializing two downloads. Better
  to download checksums file in parallel or after the binary download.

## R5: SkipChecksum Flag Flow

**Decision**: Add `SkipChecksum bool` field to `InstallOptions`. Wire from CLI flag
`--skip-checksum` in `plugin_install.go`.

**Rationale**: Follows the established pattern for install flags (`Force`, `NoSave`,
`FallbackToLatest`, etc.). The `InstallOptions` struct flows from CLI to `installRelease()`
where the checksum verification step checks `opts.SkipChecksum` to short-circuit.

The update flow (`Installer.Update()`) constructs its own `InstallOptions` when calling
`installRelease()`. To support FR-009 (verification on updates), the update command does
NOT need a separate flag since updates should always verify. However, for consistency and
edge cases, `SkipChecksum` should be plumbed through `UpdateOptions` as well.

**Alternatives considered**:

- Environment variable (`FINFOCUS_SKIP_CHECKSUM`): Less discoverable than a CLI flag.
  Could be added later if needed.
- Config file setting: Overkill for what should be an explicit per-invocation decision.
- No skip option: Spec requires FR-007 (`--skip-checksum` flag).
