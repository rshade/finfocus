# Feature Specification: Plugin Checksum Verification

**Feature Branch**: `593-plugin-checksum`
**Created**: 2026-02-15
**Status**: Draft
**Input**: GitHub Issue #601 - Add SHA256 checksum verification to plugin installation to ensure binary integrity

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Verified Plugin Installation (Priority: P1)

A user installs a plugin from a GitHub release that includes a checksums file. The system
automatically downloads the checksums file, computes the hash of the downloaded binary, and
verifies they match before completing the installation. The user sees progress messages
confirming that integrity verification succeeded.

**Why this priority**: This is the core value proposition. Without verified installation,
there is no protection against corrupted or tampered binaries. This story delivers the
primary security benefit and is the foundation all other stories build upon.

**Independent Test**: Can be fully tested by installing a plugin from a release that
includes a checksums file. Verification success is confirmed when the installation
completes with a verification confirmation message.

**Acceptance Scenarios**:

1. **Given** a GitHub release with a checksums file and a matching plugin binary,
   **When** the user runs `finfocus plugin install <plugin>`,
   **Then** the system downloads the checksums file, verifies the binary hash matches,
   and completes the installation successfully with a verification confirmation message.

2. **Given** a GitHub release with a checksums file containing a hash that does NOT match
   the downloaded binary,
   **When** the user runs `finfocus plugin install <plugin>`,
   **Then** the system rejects the installation, removes any partially downloaded files,
   and displays an error message identifying the checksum mismatch.

3. **Given** a successful checksum verification,
   **When** the installation completes,
   **Then** the user sees a progress message indicating integrity verification passed.

---

### User Story 2 - Graceful Degradation for Releases Without Checksums (Priority: P2)

A user installs a plugin from a GitHub release that does not include a checksums file. The
system logs a warning that integrity verification was not possible but proceeds with the
installation. This ensures backward compatibility with existing plugin releases that have
not yet adopted checksum publishing.

**Why this priority**: Many existing plugin releases do not include checksums files.
Breaking installation for these plugins would be a regression. Graceful degradation
ensures the feature can be adopted incrementally by plugin publishers.

**Independent Test**: Can be fully tested by installing a plugin from a release that has
no checksums file. Success is confirmed when installation completes with a warning about
missing verification.

**Acceptance Scenarios**:

1. **Given** a GitHub release with NO checksums file,
   **When** the user runs `finfocus plugin install <plugin>`,
   **Then** the system logs a warning that checksum verification was skipped and completes
   the installation normally.

2. **Given** a GitHub release with a checksums file that does NOT list the specific
   downloaded asset,
   **When** the user runs `finfocus plugin install <plugin>`,
   **Then** the system logs a warning that the asset was not found in the checksums file
   and completes the installation normally.

3. **Given** a network error occurs when fetching the checksums file,
   **When** the user runs `finfocus plugin install <plugin>`,
   **Then** the system logs a warning about the network failure and completes the
   installation normally.

---

### User Story 3 - Bypass Checksum Verification (Priority: P3)

A user who wants to skip checksum verification (e.g., for local testing, air-gapped
environments, or known-good builds) can pass a CLI flag to bypass the verification step
entirely. No checksums file is downloaded and no hash computation occurs.

**Why this priority**: This is an escape hatch for edge cases. Most users should use
verified installation by default, but advanced users need the ability to opt out when
verification is impractical or unnecessary.

**Independent Test**: Can be fully tested by installing a plugin with the skip flag and
confirming no verification messages appear and installation completes without fetching the
checksums file.

**Acceptance Scenarios**:

1. **Given** any GitHub release (with or without a checksums file),
   **When** the user runs `finfocus plugin install --skip-checksum <plugin>`,
   **Then** the system skips all checksum verification and installs the plugin directly.

2. **Given** a release with a checksums file containing a mismatched hash,
   **When** the user runs `finfocus plugin install --skip-checksum <plugin>`,
   **Then** the system installs the plugin successfully without any verification error.

---

### Edge Cases

- What happens when the checksums file is malformed (not valid SHA256SUMS format)?
  The system logs a warning and continues installation without verification.
- What happens when the checksums file contains entries for other assets but not the one
  being installed? The system logs a warning and continues installation.
- What happens when plugin update (`finfocus plugin update`) is used?
  Checksum verification applies to updates in the same way as fresh installations.
- What happens when the checksums file uses a different hash algorithm (e.g., MD5)?
  Only SHA256 is supported. Non-SHA256 entries are ignored. If no SHA256 entry matches the
  asset name, a warning is logged and installation continues.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST compute the SHA256 hash of downloaded plugin binaries before
  completing installation.
- **FR-002**: System MUST download and parse checksums files from the same release as the
  plugin binary, using standard SHA256SUMS format (`<hash>  <filename>`).
- **FR-003**: System MUST reject installation and clean up downloaded files when a checksum
  mismatch is detected between the computed hash and the published hash.
- **FR-004**: System MUST continue installation with a logged warning when a checksums file
  is not available in the release.
- **FR-005**: System MUST continue installation with a logged warning when the downloaded
  asset name is not listed in the checksums file.
- **FR-006**: System MUST continue installation with a logged warning when a network error
  prevents fetching the checksums file.
- **FR-007**: System MUST support a CLI flag (`--skip-checksum`) that bypasses all checksum
  verification steps.
- **FR-008**: System MUST display progress messages to the user indicating verification
  status (verifying, verified, skipped, or failed).
- **FR-009**: System MUST apply checksum verification to both fresh installations and
  plugin updates.
- **FR-010**: System MUST parse checksums files tolerant of both two-space and single-space
  separators between hash and filename, and MUST strip leading `*` prefix from filenames
  (binary mode indicator used by some checksum tools).

### Key Entities

- **Checksums File**: A text file published alongside release assets containing SHA256 hash
  and filename pairs. Standard format: `<64-char-hex-hash>  <filename>` (one entry per
  line). Published by plugin maintainers as part of their release process.
- **Plugin Binary**: The platform-specific archive or executable downloaded from a GitHub
  release. Subject to integrity verification before installation completes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of plugin installations from releases with valid checksums files
  complete with verified integrity (no unverified installations when checksum data is
  available).
- **SC-002**: Users see clear verification status messages during every installation
  (verified, skipped, or warning).
- **SC-003**: Corrupted or tampered binaries are detected and rejected in 100% of cases
  where a valid checksums file is present.
- **SC-004**: Existing plugin releases without checksums files install successfully with
  zero regressions from current behavior.
- **SC-005**: All existing tests continue to pass, and new tests achieve 80%+ coverage of
  checksum verification logic.
- **SC-006**: Checksum verification adds less than 2 seconds to installation time for
  typical plugin sizes (under 50 MB).

## Assumptions

- Plugin publishers use the standard SHA256SUMS format (`<hash>  <filename>`) as generated
  by tools like `sha256sum` or `shasum -a 256`.
- The checksums file is named `checksums.txt` (matched case-insensitively) and published
  as a release asset alongside plugin binaries.
- SHA256 is the only hash algorithm supported; other algorithms (MD5, SHA1, SHA512) are
  out of scope.
- The checksums file is not cryptographically signed. Signature verification (GPG/cosign)
  is a potential future enhancement but is out of scope for this feature.
- Checksum verification applies to archive files (the downloaded asset), not to individual
  files extracted from archives.
