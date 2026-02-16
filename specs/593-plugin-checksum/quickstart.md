# Developer Quickstart: Plugin Checksum Verification

**Feature**: 593-plugin-checksum

## What This Feature Does

Adds SHA256 checksum verification to the `finfocus plugin install` and
`finfocus plugin update` commands. When a GitHub release includes a `checksums.txt` file,
the installer automatically verifies the downloaded binary's integrity before installation.

## Files to Modify/Create

| File | Action | Purpose |
| ---- | ------ | ------- |
| `internal/registry/checksum.go` | Create | Checksum verification functions |
| `internal/registry/checksum_test.go` | Create | Tests for checksum functions |
| `internal/registry/installer.go` | Modify | Wire verification into install pipeline |
| `internal/cli/plugin_install.go` | Modify | Add `--skip-checksum` flag |

## Implementation Order

1. **checksum.go** - Pure functions first (no dependencies on installer)
2. **checksum_test.go** - Tests for all checksum functions (TDD)
3. **installer.go** - Wire verification into `installRelease()`
4. **plugin_install.go** - Add CLI flag and plumb through `InstallOptions`

## Key Integration Point

In `installer.go`, the `installRelease()` method currently follows this flow:

```text
FindPlatformAsset → Download → Extract → FindBinary → Validate → Save
```

Checksum verification inserts between Download and Extract:

```text
FindPlatformAsset → Download → [VERIFY CHECKSUM] → Extract → FindBinary → Validate → Save
```

## How to Test Locally

```bash
# Run checksum unit tests
go test -v ./internal/registry/... -run TestChecksum

# Run all registry tests
go test -v ./internal/registry/...

# Run linting
make lint

# Run full test suite
make test
```

## Checksum File Format

The `checksums.txt` file follows standard SHA256SUMS format:

```text
a1b2c3d4e5f6...  finfocus-plugin-aws-v1.0.0-linux-amd64.tar.gz
f6e5d4c3b2a1...  finfocus-plugin-aws-v1.0.0-darwin-arm64.tar.gz
```

## CLI Usage After Implementation

```bash
# Normal install (checksum verified automatically if available)
finfocus plugin install kubecost

# Skip checksum verification
finfocus plugin install --skip-checksum kubecost

# Update also verifies checksums
finfocus plugin update kubecost
```
