# Tasks: Install Script (curl | sh)

**Input**: Design documents from `/specs/593-install-script/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Tests**: This feature is a POSIX shell script, not Go code. ShellCheck (static
analysis) is the quality gate per the constitution check in plan.md. Go test
coverage metrics do not apply.

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all
tasks MUST be fully implemented. No stubs, placeholders, or TODO comments.

**Documentation**: Per Constitution Principle IV (Documentation Integrity),
`docs/getting-started/installation.md` and `docs/getting-started/quickstart.md`
MUST be updated to replace "Coming Soon" placeholders.

**Organization**: Tasks are grouped by user story. US1 (Linux) and US2 (macOS)
share the same code path and are combined. US6 (unsupported platform) is handled
by the error cases in detect_os/detect_arch and is included in the core phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create the script skeleton and foundational structure

- [x] T001 Create `scripts/install.sh` with POSIX shebang (`#!/bin/sh`), `set -eu`,
  trap-based cleanup function using `mktemp -d` for temporary directory
  (portable: `mktemp -d 2>/dev/null || mktemp -d -t 'finfocus-install'`),
  `fail()` error helper that prints to stderr and exits non-zero, and empty
  `main()` entry point. Make the file executable (`chmod +x`). Implements FR-016
  (cleanup on exit) and FR-011 (non-zero exit on failure). Per research R2, use
  `set -eu` only (no pipefail). Per research R8, use trap on EXIT signal.

**Checkpoint**: Script exists, is executable, runs without error, and cleans up
temp directory on exit.

---

## Phase 2: US1 + US2 + US6 - Core Install Flow (Priority: P1/P3) MVP

**Goal**: A working install script that detects OS/arch, downloads the correct
binary from GitHub Releases, extracts it, and installs to the target directory.
Covers Linux (US1), macOS (US2), and unsupported platform errors (US6).

**Independent Test**: Run the script on a Linux or macOS machine with a valid
GitHub Release available. Verify `finfocus --version` succeeds after install.

- [x] T002 [US1] [US2] [US6] Implement `detect_os()` function in `scripts/install.sh`
  that maps `uname -s` output: `Linux` to `linux`, `Darwin` to `macos` (per
  goreleaser naming in research R1 and design decision D2). For MINGW/MSYS/CYGWIN
  (Windows), exit with error suggesting `go install github.com/rshade/finfocus/cmd/finfocus@latest`
  or manual download. For any other OS, exit with error listing supported platforms.
  Implements FR-001.

- [x] T003 [US1] [US2] [US6] Implement `detect_arch()` function in `scripts/install.sh`
  that maps `uname -m` output: `x86_64` and `amd64` to `amd64`, `aarch64` and
  `arm64` to `arm64`. For any other architecture, exit with error listing
  supported architectures (amd64, arm64). Implements FR-002.

- [x] T004 [US1] [US2] Implement `download()` function in `scripts/install.sh`
  that abstracts HTTP downloads with curl/wget fallback. Per research R4: try
  `curl -fsSL --retry 3 "$url" -o "$output"` first, then fall back to
  `wget -q --tries=3 -O "$output" "$url"`. Use `type` command (not `which`) for
  POSIX-compliant existence checks (research R2). Exit with error if neither is
  available. Implements FR-013 and FR-015 (HTTPS only).

- [x] T005 [US1] [US2] Implement `get_latest_version()` function in `scripts/install.sh`
  that queries `https://api.github.com/repos/rshade/finfocus/releases/latest`
  using the `download()` function and extracts `tag_name` via
  `grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'` (no jq dependency, per
  design decision D4 and research R5). On failure, exit with error suggesting
  user set `FINFOCUS_VERSION` to bypass the API call. Implements FR-003.

- [x] T006 [US1] [US2] Implement `resolve_install_dir()` function in `scripts/install.sh`
  that determines the default install directory: (1) use `/usr/local/bin` if
  writable (`[ -w "/usr/local/bin" ]`), (2) fall back to `$HOME/.local/bin`
  creating it with `mkdir -p` if needed (FR-014). Print PATH guidance if using
  the fallback directory. Does NOT handle `FINFOCUS_INSTALL_DIR` override (that
  is added in T014/Phase 4). Implements FR-007.

- [x] T007 [US1] [US2] Implement `install_binary()` function in `scripts/install.sh`
  that extracts the finfocus binary from the tar.gz archive using
  `tar -xzf "$archive" -C "$TMP_DIR"`, then moves it to the resolved install
  directory with `chmod +x`. Implements FR-006.

- [x] T008 [US1] [US2] Wire the `main()` function in `scripts/install.sh` to
  orchestrate the full flow: call `detect_os`, `detect_arch`, `get_latest_version`,
  construct the download URL using the pattern
  `https://github.com/rshade/finfocus/releases/download/${VERSION}/finfocus-${VERSION}-${OS}-${ARCH}.tar.gz`,
  call `download` for the archive, call `install_binary`, then print the installed
  version and next steps (`finfocus --version`, `finfocus --help`). Implements
  FR-004, FR-010.

**Checkpoint**: Script can install FinFocus on Linux and macOS. Unsupported
platforms get clear error messages. This is the MVP.

---

## Phase 3: US3 - Checksum Verification (Priority: P1)

**Goal**: Add SHA256 checksum verification to the install flow, verifying the
downloaded archive against `checksums.txt` before installation.

**Independent Test**: Download a release archive, corrupt it (e.g., truncate),
run the script. Verify it refuses to install. Then run with a clean archive and
verify it passes.

- [x] T009 [US3] Implement `hash_sha256()` function in `scripts/install.sh` that
  computes the SHA256 hash of a file using the first available tool:
  `sha256sum "$file" | cut -d ' ' -f 1` (Linux), `shasum -a 256 "$file" | cut -d ' ' -f 1`
  (macOS), or `openssl dgst -sha256 "$file" | awk '{print $NF}'` (fallback).
  Exit with error if none available. Per research R3.

- [x] T010 [US3] Implement `verify_checksum()` function in `scripts/install.sh`
  that downloads `checksums.txt` from the same release, extracts the expected
  hash for the archive filename using `grep "$ARCHIVE_NAME" "$checksums_file" | cut -d ' ' -f 1`,
  computes the actual hash via `hash_sha256()`, and compares. Exit with error
  showing expected vs actual hash on mismatch. Implements FR-005.

- [x] T011 [US3] Add `FINFOCUS_NO_VERIFY` support in `scripts/install.sh` main()
  flow: if `FINFOCUS_NO_VERIFY` is set (non-empty), skip checksum verification
  with a warning printed to stderr: `"WARNING: Checksum verification disabled. This is not recommended."`.
  If the checksums.txt download fails and `FINFOCUS_NO_VERIFY` is not set, exit
  with error. Implements FR-017.

- [x] T012 [US3] Integrate checksum verification into `main()` in `scripts/install.sh`:
  after downloading the archive, call `verify_checksum()` before calling
  `install_binary()`. The verification step downloads checksums.txt to the temp
  directory and validates the archive hash.

**Checkpoint**: Install script now verifies binary integrity by default. Checksum
bypass via env var works with warning.

---

## Phase 4: US4 + US5 - Environment Configuration (Priority: P2)

**Goal**: Support version pinning via `FINFOCUS_VERSION` and install directory
override via `FINFOCUS_INSTALL_DIR`.

**Independent Test**: Set `FINFOCUS_VERSION=v0.1.0` and verify that specific
version installs. Set `FINFOCUS_INSTALL_DIR=/tmp/test-bin` and verify binary
appears there.

- [x] T013 [US4] Add `FINFOCUS_VERSION` support in `scripts/install.sh` main()
  flow: if `FINFOCUS_VERSION` is set, use it instead of calling
  `get_latest_version()`. Normalize the version by prepending `v` if missing
  (per design decision D3: `case "$VERSION" in v*) ;; *) VERSION="v${VERSION}" ;; esac`).
  If the specified version download fails (404), exit with error:
  `"Version ${VERSION} not found. Check available versions at https://github.com/rshade/finfocus/releases"`.
  Implements FR-009.

- [x] T014 [US5] Integrate `FINFOCUS_INSTALL_DIR` into `resolve_install_dir()` in
  `scripts/install.sh`: if set, validate it is writable (`[ -w "$dir" ]` or
  create with `mkdir -p`). If the directory cannot be created or written to,
  exit with error: `"Install directory ${dir} is not writable"`. This takes
  priority over the `/usr/local/bin` and `$HOME/.local/bin` fallback chain.
  Implements FR-008.

**Checkpoint**: Version pinning and custom install directory both work. All env
var overrides from the spec are implemented.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: CI integration, documentation updates, and final validation

- [x] T015 [P] Add ShellCheck step to `.github/workflows/ci.yml` in the lint job
  (after the markdownlint step). Use direct invocation:
  `sudo apt-get install -y shellcheck && shellcheck scripts/install.sh`. Per
  design decision D7 and research R6: add to existing lint job, not a separate
  workflow. Implements FR-012 (ShellCheck zero warnings).

- [x] T016 [P] Update `docs/getting-started/installation.md`: replace the
  "Option 2: Download Prebuilt Binary (Coming Soon)" section (lines 53-65) with
  the actual curl | sh install command and environment variable documentation.
  Keep the section title as "Option 2: Install Script (Recommended)" and include
  examples for version pinning and custom directory. Per research R7.

- [x] T017 [P] Update `docs/getting-started/quickstart.md`: replace the
  "Option B: Download binary (coming soon)" section (lines 27-32) with the
  actual curl | sh command:
  `curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh`.
  Mark it as "(recommended)" instead of "(coming soon)". Per research R7.

- [x] T018 Run ShellCheck locally on `scripts/install.sh` and fix any warnings.
  The script MUST pass with zero warnings per FR-012 and SC-006. Address any
  SC-prefixed warnings by fixing the code (preferred) or adding targeted
  `# shellcheck disable=SCxxxx` directives with a comment justifying the
  suppression.

- [x] T019 Run markdownlint on `docs/getting-started/installation.md` and
  `docs/getting-started/quickstart.md` to verify no formatting issues were
  introduced.

- [x] T020 Final validation: review the complete `scripts/install.sh` against all
  17 functional requirements (FR-001 through FR-017) and 7 success criteria
  (SC-001 through SC-007) from the spec. Verify every edge case from the spec
  has corresponding handling in the script. Ensure no TODO comments or stub
  functions remain (Constitution Principle VI).

**Checkpoint**: All files pass linting. Documentation is updated. Script is ready
for PR.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Core Install (Phase 2)**: Depends on Phase 1 - creates the working install flow
- **Checksum (Phase 3)**: Depends on Phase 2 - adds security to existing flow
- **Env Config (Phase 4)**: Depends on Phase 2 - adds env var overrides
- **Polish (Phase 5)**: Depends on Phases 2, 3, and 4

### User Story Dependencies

- **US1 + US2 (Linux + macOS)**: Combined in Phase 2 - same code path with OS mapping
- **US3 (Checksum)**: Phase 3 - independent, adds to existing download flow
- **US4 + US5 (Version pin + Custom dir)**: Phase 4 - independent, adds env var handling
- **US6 (Unsupported platform)**: Combined in Phase 2 - error cases in detect functions

### Within Each Phase

- T002 and T003 can run in parallel (different functions)
- T004 and T005 can run in parallel (different functions)
- T006 and T007 depend on T004 (use download function)
- T008 depends on all T002-T007 (orchestrates them)
- T009 and T010 are sequential (T010 uses T009)
- T015, T016, T017 can all run in parallel (different files)

### Parallel Opportunities

```text
Phase 2 parallel groups:
  Group A: T002 (detect_os) + T003 (detect_arch)
  Group B: T004 (download) + T005 (get_latest_version)
  Then: T006, T007, T008 sequentially

Phase 3: Sequential (T009 → T010 → T011 → T012)

Phase 4: T013 and T014 are independent (different functions)

Phase 5 parallel group: T015 (CI) + T016 (install docs) + T017 (quickstart docs)
  Then: T018, T019, T020 sequentially
```

---

## Implementation Strategy

### MVP First (Phase 1 + Phase 2)

1. Complete Phase 1: Script skeleton with cleanup
2. Complete Phase 2: Working install for Linux + macOS
3. **STOP and VALIDATE**: Test on Linux, verify `finfocus --version` works
4. This delivers a functional installer without checksum verification

### Full Feature (Phase 1 + 2 + 3 + 4 + 5)

1. Phase 1: Setup
2. Phase 2: Core install flow (MVP)
3. Phase 3: Add checksum verification (security)
4. Phase 4: Add env var overrides (flexibility)
5. Phase 5: CI, docs, validation (polish)
6. Each phase is an increment that can be tested independently

---

## Notes

- All tasks operate on a single file (`scripts/install.sh`) except T015-T017
- US1 and US2 share identical code paths; the only difference is the OS name
  mapping in `detect_os()` (Linux → linux, Darwin → macos)
- US6 is handled by the error/fallthrough cases in `detect_os()` and `detect_arch()`
- The goreleaser asset naming uses `macos` (not `darwin`) per `.goreleaser.yaml`
- POSIX compliance means: no bashisms, `#!/bin/sh`, `set -eu`, `type` not `which`
- The script has no Go code, so `make test` and `make lint` are not directly
  applicable; ShellCheck and markdownlint are the quality gates
