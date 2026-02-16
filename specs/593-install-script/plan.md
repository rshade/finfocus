# Implementation Plan: Install Script (curl | sh)

**Branch**: `593-install-script` | **Date**: 2026-02-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/593-install-script/spec.md`

## Summary

Create a POSIX-compatible `scripts/install.sh` that enables one-command FinFocus
installation via `curl -fsSL ... | sh`. The script detects OS/architecture, downloads
the correct release archive from GitHub Releases, verifies SHA256 checksums, and
installs the binary to `/usr/local/bin/` (or `$HOME/.local/bin/` as fallback).
Supports version pinning, custom install directories, and checksum bypass via
environment variables. Add ShellCheck validation to CI.

## Technical Context

**Language/Version**: POSIX sh (no bashisms)
**Primary Dependencies**: curl or wget, sha256sum or shasum, tar, mktemp
**Storage**: N/A (downloads to temp dir, installs binary to target dir)
**Testing**: ShellCheck (static analysis), manual platform testing, CI validation
**Target Platform**: Linux (amd64, arm64), macOS (amd64, arm64)
**Project Type**: Single script + CI integration + docs update
**Performance Goals**: Complete installation in under 30 seconds
**Constraints**: POSIX-compatible, zero warnings from ShellCheck, no runtime
dependencies beyond standard Unix tools
**Scale/Scope**: Single shell script (~200-250 lines), CI step addition, 2 doc updates

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with FinFocus Core Constitution (`.specify/memory/constitution.md`):

- [x] **Plugin-First Architecture**: N/A - this is a distribution/install tool, not a
  cost data source or plugin. No plugin or orchestration code is affected.
- [x] **Test-Driven Development**: ShellCheck provides static analysis coverage.
  The script is a standalone POSIX shell file with no Go code, so Go test coverage
  metrics do not apply. ShellCheck zero-warnings is the quality gate.
- [x] **Cross-Platform Compatibility**: Script supports Linux (amd64, arm64) and
  macOS (amd64, arm64). Windows is explicitly out of scope with guidance to use
  `go install`. The goreleaser already builds Windows binaries for manual download.
- [x] **Documentation Integrity**: `docs/getting-started/installation.md` and
  `docs/getting-started/quickstart.md` will be updated to replace "Coming Soon"
  placeholders with the actual install command.
- [x] **Protocol Stability**: N/A - no protocol buffer changes.
- [x] **Implementation Completeness**: Script will be fully functional with no
  stubs or TODOs. All edge cases from the spec will be handled.
- [x] **Quality Gates**: ShellCheck added to CI lint job. markdownlint for docs.
- [x] **Multi-Repo Coordination**: N/A - core-only change. No spec or plugin
  repo changes needed.

**Violations Requiring Justification**: None.

## Project Structure

### Documentation (this feature)

```text
specs/593-install-script/
├── plan.md              # This file
├── research.md          # Phase 0: research findings
├── quickstart.md        # Phase 1: usage guide
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
scripts/
└── install.sh                  # NEW: The install script (~200-250 lines)

.github/workflows/
└── ci.yml                      # MODIFIED: Add ShellCheck step to lint job

docs/getting-started/
├── installation.md             # MODIFIED: Replace "Coming Soon" with curl|sh
└── quickstart.md               # MODIFIED: Replace "coming soon" with curl|sh
```

**Structure Decision**: This feature adds a single new file (`scripts/install.sh`)
alongside the existing scripts directory. No new Go code, packages, or structural
changes. The CI and docs modifications are minimal, targeted edits.

## Design Decisions

### D1: Script Architecture

The script follows a function-based structure with a `main()` entry point:

1. `detect_os()` - Maps `uname -s` to goreleaser OS names
2. `detect_arch()` - Maps `uname -m` to goreleaser arch names
3. `get_latest_version()` - Queries GitHub API, extracts `tag_name`
4. `download()` - curl/wget abstraction with retry
5. `hash_sha256()` - Portable SHA256 computation
6. `verify_checksum()` - Validates archive against checksums.txt
7. `install_binary()` - Extracts and installs with permission handling
8. `main()` - Orchestrates the full flow

### D2: OS Name Mapping (goreleaser-specific)

The goreleaser config maps `darwin` to `macos` in archive names (`.goreleaser.yaml`
line 29). The script must map:

- `uname -s` = `Darwin` to asset OS = `macos`
- `uname -s` = `Linux` to asset OS = `linux`

### D3: Version Tag Normalization

`FINFOCUS_VERSION` accepts both `v0.1.0` and `0.1.0` formats. The script
normalizes by prepending `v` if missing, since GitHub tags use `v` prefix.

### D4: JSON Parsing Without jq

The GitHub API response is parsed with `grep` + `sed`/`cut` to extract `tag_name`.
Pattern: `grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'`

### D5: Temp Directory Portability

Use `mktemp -d 2>/dev/null || mktemp -d -t 'finfocus-install'` to handle both
Linux (`mktemp -d`) and macOS (`mktemp -d -t template`) conventions.

### D6: Install Directory Resolution

Priority order:

1. `FINFOCUS_INSTALL_DIR` environment variable (explicit override)
2. `/usr/local/bin` if writable (system default)
3. `$HOME/.local/bin` as fallback (user-local, created if missing)

### D7: CI Integration

Add a ShellCheck step to the existing lint job in `ci.yml` rather than creating
a separate workflow. This keeps all linting in one place and avoids workflow
proliferation.

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `scripts/install.sh` | CREATE | The POSIX install script |
| `.github/workflows/ci.yml` | MODIFY | Add ShellCheck step to lint job |
| `docs/getting-started/installation.md` | MODIFY | Replace "Coming Soon" with curl command |
| `docs/getting-started/quickstart.md` | MODIFY | Replace "coming soon" with curl command |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| goreleaser naming changes | Low | High | Pin to confirmed naming; test against real release |
| GitHub API rate limiting | Low | Medium | Script suggests `FINFOCUS_VERSION` as workaround |
| macOS shasum path changes | Very Low | Medium | Fallback chain: sha256sum -> shasum -> openssl |
| ShellCheck false positives | Low | Low | Use `# shellcheck disable=SCxxxx` with justification |
