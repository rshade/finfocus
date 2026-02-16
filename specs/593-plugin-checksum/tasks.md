# Tasks: Plugin Checksum Verification

**Input**: Design documents from `/specs/593-plugin-checksum/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II (Test-Driven Development), tests are MANDATORY and must be written BEFORE implementation. All code changes must maintain minimum 80% test coverage (95% for critical paths).

**Completeness**: Per Constitution Principle VI (Implementation Completeness), all tasks MUST be fully implemented. Stub functions, placeholders, and TODO comments are strictly forbidden.

**Documentation**: Per Constitution Principle IV (Documentation Integrity), documentation (README, docs/) MUST be updated concurrently with implementation and verified in CI to prevent drift.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Foundational (Core Checksum Module)

**Purpose**: Create the pure checksum verification functions that all user stories depend on. These functions have no dependencies on the installer and can be tested in isolation.

### Tests (TDD - Write First)

- [x] T001 [P] Write table-driven tests for `computeSHA256`, `VerifyChecksum`, `ParseChecksumsFile`, and `FindChecksumAsset` in `internal/registry/checksum_test.go`. Cover: matching hash, mismatching hash, file not found, valid SHA256SUMS format (GNU two-space, BSD single-space, binary mode `*` prefix), blank lines, comment lines, missing asset name, malformed content (no valid entries), 64-char hex validation, case-insensitive checksums.txt asset search, release with no checksums.txt asset. Include a timing assertion for `computeSHA256` on a 50 MB temp file to validate SC-006 (must complete in under 2 seconds). Use testify `require`/`assert`. Reference contracts in `specs/593-plugin-checksum/contracts/checksum-api.md` for exact function signatures and behavior.

### Implementation

- [x] T002 Implement all checksum functions in `internal/registry/checksum.go`: sentinel errors (`ErrAssetNotInChecksums`, `ErrMalformedChecksums`, `ErrChecksumMismatch`), `computeSHA256(filePath string) (string, error)` using streaming `crypto/sha256` + `io.Copy`, `VerifyChecksum(filePath, expectedHash string) error` with lowercase normalization, `ParseChecksumsFile(data []byte, assetName string) (string, error)` with tolerant whitespace splitting and `*` prefix stripping, `FindChecksumAsset(release *GitHubRelease) *ReleaseAsset` with case-insensitive name matching. All exported functions must have godoc comments.

- [x] T003 [P] Add `SkipChecksum bool` field to `InstallOptions` struct in `internal/registry/installer.go` (after the existing `Metadata` field). Add `SkipChecksum bool` field to `UpdateOptions` struct in the same file (after the existing `PluginDir` field). Add godoc comment: `// SkipChecksum bypasses SHA256 checksum verification during installation`.

**Checkpoint**: `go test ./internal/registry/... -run TestChecksum` passes. All checksum functions work in isolation.

---

## Phase 2: User Story 1 - Verified Plugin Installation (Priority: P1)

**Goal**: When a GitHub release includes `checksums.txt`, the installer automatically verifies the downloaded binary's SHA256 hash before extraction. Hash mismatches block installation.

**Independent Test**: Install a plugin from a mock release with `checksums.txt`. Verify the installation succeeds with a "Checksum verified" message when hashes match, and fails with a "checksum mismatch" error when they don't.

### US1 Tests (TDD - Write First)

- [x] T004 [US1] Write integration tests for checksum-verified installation in `internal/registry/installer_test.go`. Use `httptest.NewServer` to serve a mock GitHub release JSON containing both a platform asset and a `checksums.txt` asset. Test cases: (1) checksums.txt contains correct hash for the binary asset - verify installation completes and progress callback receives "Checksum verified" message. (2) checksums.txt contains wrong hash for the binary asset - verify installation returns error containing "checksum mismatch" and the install directory is NOT created. Use the existing mock archive helpers (`testAssetName`, `createMockArchive` patterns from `installer_api_test.go`).

### US1 Implementation

- [x] T005 [US1] Wire checksum verification into `installRelease()` method in `internal/registry/installer.go`. Insert verification block after `i.client.DownloadAsset()` succeeds (line ~417) and before `os.MkdirAll(installDir)` (line ~420). Implementation: (1) If `opts.SkipChecksum` is true, skip entirely. (2) Call `FindChecksumAsset(release)`. (3) If nil, emit progress warning and continue. (4) Download checksums asset to a second temp file using `i.client.DownloadAsset()`. (5) Read temp file content, call `ParseChecksumsFile(data, asset.Name)`. (6) If `ErrAssetNotInChecksums` or `ErrMalformedChecksums`, emit progress warning and continue. (7) If download/read error, emit progress warning and continue. (8) Call `VerifyChecksum(tmpPath, expectedHash)`. (9) If error (mismatch), return error (blocks installation). (10) Emit progress "Checksum verified" on success.

**Checkpoint**: `go test ./internal/registry/... -run TestInstall` passes. Installing from a release with valid checksums.txt verifies integrity. Mismatched hashes block installation.

---

## Phase 3: User Story 2 - Graceful Degradation (Priority: P2)

**Goal**: Releases without `checksums.txt` install successfully with a warning. Network errors, missing assets in the checksums file, and malformed files all produce warnings but never block installation.

**Independent Test**: Install a plugin from a mock release WITHOUT `checksums.txt`. Verify installation completes successfully and progress callback receives a warning about skipped verification.

### US2 Tests (TDD - Write First)

- [x] T006 [US2] Write degradation scenario tests in `internal/registry/installer_test.go`. Use `httptest.NewServer` mock releases. Test cases: (1) Release has NO `checksums.txt` asset - verify installation succeeds and progress receives warning containing "not found" or "skipping verification". (2) `checksums.txt` exists but does NOT list the downloaded asset name - verify installation succeeds with warning. (3) `checksums.txt` download returns HTTP 500 - verify installation succeeds with warning about network failure. (4) `checksums.txt` content is malformed (no valid hash entries) - verify installation succeeds with warning. Each test must assert installation completes (no error returned) and the plugin binary is present on disk.

**Checkpoint**: All degradation tests pass. Existing installer tests still pass (no regressions). `go test ./internal/registry/...` green.

---

## Phase 4: User Story 3 - Bypass Checksum Verification (Priority: P3)

**Goal**: Users can pass `--skip-checksum` to bypass all verification. No checksums file is downloaded and no hash computation occurs.

**Independent Test**: Install a plugin with `--skip-checksum` from a release with a mismatched checksums.txt. Verify installation succeeds without error.

### US3 Tests (TDD - Write First)

- [x] T007 [P] [US3] Write tests for `--skip-checksum` flag registration in `internal/cli/plugin_install_test.go` and `internal/cli/plugin_update_test.go`. Verify for both install and update commands: (1) The flag exists on the command. (2) The flag defaults to false. Follow the existing test pattern for flag checks (see `expectedFlags` pattern in existing tests).

- [x] T008 [US3] Write test in `internal/registry/installer_test.go` verifying that when `SkipChecksum: true` is set in `InstallOptions`, installation from a release with mismatched checksums.txt succeeds without error and no "Checksum verified" or "checksum mismatch" messages appear in progress output.

### US3 Implementation

- [x] T009 [P] [US3] Add `--skip-checksum` flag to the plugin install command in `internal/cli/plugin_install.go`. Add `cmd.Flags().BoolVar(&skipChecksum, "skip-checksum", false, "Skip SHA256 checksum verification during installation")` in the flags section (after existing `--metadata` flag). Wire the variable into the `InstallOptions` struct construction: `SkipChecksum: skipChecksum`.

- [x] T010 [US3] Add `--skip-checksum` flag to the plugin update command in `internal/cli/plugin_update.go`. Add the flag definition and wire `SkipChecksum` from `UpdateOptions` through the `Update()` method to the `InstallOptions` passed to `installRelease()`. In the `Update()` method in `internal/registry/installer.go`, set `SkipChecksum: opts.SkipChecksum` in the `installOpts` struct (around line ~654).

**Checkpoint**: `finfocus plugin install --skip-checksum <plugin>` bypasses all verification. `finfocus plugin update --skip-checksum <plugin>` also bypasses. All tests pass.

---

## Phase 5: Polish and Cross-Cutting Concerns

**Purpose**: Documentation, quality gates, and final validation.

- [x] T011 [P] Update `internal/registry/CLAUDE.md` with a new "Checksum Verification" section documenting: the verification flow, functions in `checksum.go`, sentinel errors, integration point in `installRelease()`, and the `--skip-checksum` flag behavior.

- [x] T012 Run `make lint` and `make test` to verify all quality gates pass. Fix any linting errors or test failures. Verify no existing tests regressed.

- [x] T013 Verify test coverage for `internal/registry/checksum.go` meets 80% minimum: `go test -coverprofile=coverage.out ./internal/registry/... && go tool cover -func=coverage.out | grep checksum`. If below 80%, add additional test cases to `checksum_test.go` to cover missed branches.

---

## Dependencies and Execution Order

### Phase Dependencies

- **Phase 1 (Foundational)**: No dependencies - can start immediately
- **Phase 2 (US1)**: Depends on Phase 1 completion (T001-T003)
- **Phase 3 (US2)**: Depends on Phase 2 completion (T004-T005) since degradation tests validate installer wiring
- **Phase 4 (US3)**: Depends on Phase 2 completion (T004-T005) for installer wiring; CLI flag tasks (T009, T010) can start after T003
- **Phase 5 (Polish)**: Depends on all user stories complete

### Task Dependencies

```text
T001 ──→ T002 ──→ T004 ──→ T005 ──→ T006 ──→ T011
              │                  │              T012
T003 ─────────┘                  │              T013
                                 ├──→ T007 ──→ T009
                                 ├──→ T008
                                 └──→ T010
```

### Parallel Opportunities

- **Phase 1**: T001 and T003 can run in parallel (different files)
- **Phase 4**: T007 and T009 can run in parallel with T010 (different files)
- **Phase 5**: T011 can run in parallel with T012/T013

---

## Parallel Example: Phase 1

```text
# Parallel group 1 (different files):
T001: Write checksum tests in internal/registry/checksum_test.go
T003: Add SkipChecksum to InstallOptions/UpdateOptions in internal/registry/installer.go

# Sequential (same file, depends on T001):
T002: Implement checksum functions in internal/registry/checksum.go
```

## Parallel Example: Phase 4

```text
# Parallel group (different files, after T005):
T007: Flag tests in internal/cli/plugin_install_test.go + plugin_update_test.go
T009: Add --skip-checksum flag in internal/cli/plugin_install.go
T010: Add --skip-checksum flag in internal/cli/plugin_update.go + installer.go Update()
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Foundational checksum module (T001-T003)
2. Complete Phase 2: US1 verified installation (T004-T005)
3. **STOP and VALIDATE**: `go test ./internal/registry/...` all green
4. Checksum verification works for releases with `checksums.txt`

### Incremental Delivery

1. Phase 1 → Foundational module ready
2. Add US1 (Phase 2) → Verified installation works
3. Add US2 (Phase 3) → Degradation validated (tests only, no new implementation)
4. Add US3 (Phase 4) → `--skip-checksum` CLI flag available
5. Polish (Phase 5) → Documentation, coverage, quality gates

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US2 has no separate implementation tasks because degradation paths are built into the US1 wiring (T005). US2 phase is test-only validation.
- The `Update()` method in `installer.go` calls `installRelease()` internally, so checksum verification applies to updates automatically once T005 is complete.
- FR-009 (verification on updates) is satisfied by T005 + T010.
