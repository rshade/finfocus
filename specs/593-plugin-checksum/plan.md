# Implementation Plan: Plugin Checksum Verification

**Branch**: `593-plugin-checksum` | **Date**: 2026-02-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/593-plugin-checksum/spec.md`

## Summary

Add SHA256 checksum verification to the plugin installation pipeline. When a GitHub
release includes a `checksums.txt` asset, the installer downloads it, parses the expected
hash for the downloaded binary, computes the actual SHA256 hash of the downloaded file,
and compares them. Mismatches block installation. Missing checksums files produce a warning
and allow installation to proceed (backward compatibility). A `--skip-checksum` CLI flag
bypasses verification entirely.

The implementation touches three layers: a new `checksum.go` module with pure verification
functions, modifications to `installRelease()` in `installer.go` to wire verification into
the download pipeline, and a new `--skip-checksum` flag in `plugin_install.go`.

## Technical Context

**Language/Version**: Go 1.25.8
**Primary Dependencies**: `crypto/sha256` (stdlib), `encoding/hex` (stdlib), existing
`internal/registry` package (`GitHubClient`, `Installer`, `GitHubRelease`)
**Storage**: N/A (verification is transient, no persistent state)
**Testing**: `go test` with testify (`require`/`assert`), `httptest.NewServer` for mock
HTTP, table-driven tests
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
**Project Type**: Single Go project (CLI tool)
**Performance Goals**: Checksum verification < 2 seconds for files up to 50 MB
**Constraints**: No new external dependencies (stdlib only for crypto); backward compatible
with releases lacking checksums.txt
**Scale/Scope**: 3 new/modified files, ~300 lines of new code, ~400 lines of new tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: This is orchestration logic in core (installer
  infrastructure). It does not add provider-specific cost data. Checksum verification
  is a core responsibility, not a plugin concern.
- [x] **Test-Driven Development**: Tests planned before implementation with 80%+ coverage
  target for checksum verification functions. Table-driven tests for parsing, hash
  comparison, and integration with install flow.
- [x] **Cross-Platform Compatibility**: Uses only `crypto/sha256`, `encoding/hex`,
  `os`, `io` from stdlib. No platform-specific code. SHA256 computation is identical
  across all platforms.
- [x] **Documentation Integrity**: CLAUDE.md will be updated with checksum verification
  details in the registry package section. Godoc comments on all exported functions.
- [x] **Protocol Stability**: No protocol buffer changes. This feature operates entirely
  within core's installer layer.
- [x] **Implementation Completeness**: Full implementation planned with no stubs or TODOs.
  All error paths tested. All edge cases from spec addressed.
- [x] **Quality Gates**: `make lint` and `make test` will be run. 80%+ coverage enforced.
- [x] **Multi-Repo Coordination**: No cross-repo changes needed. The checksums.txt file
  is published by plugin repositories as a release asset; core only consumes it.

**Violations Requiring Justification**: None

## Project Structure

### Documentation (this feature)

```text
specs/593-plugin-checksum/
├── plan.md              # This file
├── research.md          # Phase 0: research decisions
├── data-model.md        # Phase 1: data structures
├── quickstart.md        # Phase 1: developer quickstart
├── contracts/           # Phase 1: function contracts
│   └── checksum-api.md  # Checksum function signatures and behavior
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/registry/
├── checksum.go          # NEW: VerifyChecksum, ParseChecksumsFile, FindChecksumAsset
├── checksum_test.go     # NEW: Table-driven tests for all checksum functions
├── installer.go         # MODIFIED: Wire checksum verification into installRelease()
├── installer_test.go    # MODIFIED: Add checksum integration test cases
├── github.go            # EXISTING: GitHubClient, ReleaseAsset, DownloadAsset
└── ...

internal/cli/
├── plugin_install.go      # MODIFIED: Add --skip-checksum flag
├── plugin_install_test.go # MODIFIED: Test flag registration
├── plugin_update.go       # MODIFIED: Add --skip-checksum flag
└── plugin_update_test.go  # MODIFIED: Test flag registration
```

**Structure Decision**: Existing single-project Go structure. New checksum logic lives in
`internal/registry/` alongside the existing installer, following the established pattern
of one concern per file (`installer.go`, `archive.go`, `metadata.go`, `checksum.go`).

## Complexity Tracking

No constitution violations. No complexity justification needed.
