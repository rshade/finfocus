# Feature Specification: Install Script (curl | sh)

**Feature Branch**: `593-install-script`
**Created**: 2026-02-15
**Status**: Draft
**Input**: User description: "Provide a scripts/install.sh script that allows users to install FinFocus with a single command: curl -fsSL ... | sh"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One-Command Install on Linux (Priority: P1)

A developer working on a Linux machine wants to install FinFocus quickly without
cloning the repository or setting up Go. They run a single curl command and FinFocus
is installed and ready to use.

**Why this priority**: This is the primary use case. Linux is the most common
server and developer environment. A frictionless install command removes the
biggest barrier to adoption.

**Independent Test**: Can be fully tested by running the curl pipe command on a
clean Linux VM and verifying that `finfocus --version` succeeds afterward.

**Acceptance Scenarios**:

1. **Given** a Linux amd64 machine with curl installed, **When** the user runs
   `curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh`,
   **Then** the correct binary is downloaded, checksum-verified, installed to
   `/usr/local/bin/`, and `finfocus --version` prints the installed version.
2. **Given** a Linux arm64 machine, **When** the user runs the install command,
   **Then** the arm64 binary is detected and installed correctly.
3. **Given** a Linux machine where the user lacks write access to `/usr/local/bin/`,
   **When** the install command runs, **Then** the binary is installed to
   `$HOME/.local/bin/` instead, and the user is informed to add it to PATH if needed.

---

### User Story 2 - One-Command Install on macOS (Priority: P1)

A developer on macOS (Intel or Apple Silicon) wants the same frictionless install
experience.

**Why this priority**: macOS is the second most common developer environment.
Apple Silicon (arm64) adoption is widespread, making both architectures essential.

**Independent Test**: Can be tested by running the install command on macOS
(both Intel and Apple Silicon) and verifying `finfocus --version` succeeds.

**Acceptance Scenarios**:

1. **Given** a macOS amd64 (Intel) machine, **When** the user runs the install
   command, **Then** the correct macOS amd64 binary is downloaded and installed.
2. **Given** a macOS arm64 (Apple Silicon) machine, **When** the user runs the
   install command, **Then** the correct macOS arm64 binary is downloaded and installed.

---

### User Story 3 - Checksum Verification (Priority: P1)

A security-conscious user wants assurance that the downloaded binary has not been
tampered with. The script verifies the SHA256 checksum automatically before
installing.

**Why this priority**: Security is non-negotiable for a script that downloads and
installs executables. Checksum verification must be on by default.

**Independent Test**: Can be tested by downloading a binary, corrupting it, and
verifying the script refuses to install. Also tested by verifying a clean download
passes checksum verification.

**Acceptance Scenarios**:

1. **Given** a valid release with checksums.txt, **When** the user runs the install
   command, **Then** the script downloads checksums.txt and verifies the binary's
   SHA256 hash before installing.
2. **Given** a corrupted or tampered binary, **When** checksum verification runs,
   **Then** the script exits with a clear error message and does not install.
3. **Given** `FINFOCUS_NO_VERIFY` is set, **When** the user runs the install
   command, **Then** checksum verification is skipped with a warning.

---

### User Story 4 - Pinned Version Install (Priority: P2)

A DevOps engineer wants to install a specific version of FinFocus in a CI pipeline
or Dockerfile to ensure reproducible builds.

**Why this priority**: Version pinning is critical for CI/CD and infrastructure-as-code
environments but is secondary to the basic install flow.

**Independent Test**: Can be tested by setting `FINFOCUS_VERSION=v0.1.0` and
verifying that exact version is installed rather than latest.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_VERSION` is set to a valid version tag, **When** the install
   command runs, **Then** that specific version is downloaded and installed.
2. **Given** `FINFOCUS_VERSION` is set to a nonexistent version, **When** the install
   command runs, **Then** the script exits with a clear error message.

---

### User Story 5 - Custom Install Directory (Priority: P2)

A user wants to install FinFocus to a non-default directory, perhaps because
they use a custom tools directory or need to install without elevated permissions.

**Why this priority**: Flexibility in install location supports diverse environments
but most users will use defaults.

**Independent Test**: Can be tested by setting `FINFOCUS_INSTALL_DIR=/tmp/test-bin`
and verifying the binary appears there.

**Acceptance Scenarios**:

1. **Given** `FINFOCUS_INSTALL_DIR` is set to a writable directory, **When** the
   install command runs, **Then** the binary is installed to that directory.
2. **Given** `FINFOCUS_INSTALL_DIR` is set to a non-writable directory, **When**
   the install command runs, **Then** the script exits with a clear error message.

---

### User Story 6 - Unsupported Platform Guidance (Priority: P3)

A Windows user or a user on an unsupported architecture attempts the install
script. They receive a helpful error message directing them to alternative
installation methods.

**Why this priority**: Good error messages reduce support burden, but this is an
edge case since the primary audience uses Linux or macOS.

**Independent Test**: Can be tested by mocking uname output for unsupported
platforms and verifying the error message.

**Acceptance Scenarios**:

1. **Given** the script runs on Windows (e.g., via Git Bash or WSL), **When** OS
   detection runs, **Then** the script exits with a message suggesting `go install`
   or manual download.
2. **Given** the script runs on an unsupported architecture (e.g., s390x), **When**
   architecture detection runs, **Then** the script exits with a clear error listing
   supported architectures.

---

### Edge Cases

- What happens when neither `curl` nor `wget` is available? The script exits with
  an error message listing the missing dependencies.
- What happens when the GitHub API is unreachable or rate-limited? The script exits
  with an error suggesting the user set `FINFOCUS_VERSION` to bypass the API call.
- What happens when the release exists but `checksums.txt` is missing? The script
  exits with an error and does not install unless `FINFOCUS_NO_VERIFY` is set.
- What happens when the binary is already installed at the target location? The
  script overwrites the existing binary with the new version.
- What happens when `$HOME/.local/bin/` does not exist? The script creates the
  directory before installing.
- What happens when the download is interrupted? Partial files are cleaned up via
  a trap handler on exit.
- What happens when disk space is insufficient? The script relies on standard OS
  errors and reports them clearly.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Script MUST detect the operating system using `uname -s` and map it
  to the correct release asset name (`linux`, `macos`).
- **FR-002**: Script MUST detect the CPU architecture using `uname -m` and map it
  to the correct release asset name (`amd64`, `arm64`).
- **FR-003**: Script MUST determine the latest release version by querying the
  GitHub API when `FINFOCUS_VERSION` is not set.
- **FR-004**: Script MUST download the appropriate release archive from GitHub
  Releases using HTTPS.
- **FR-005**: Script MUST download `checksums.txt` and verify the SHA256 checksum
  of the downloaded archive before installation (unless `FINFOCUS_NO_VERIFY` is set).
- **FR-006**: Script MUST extract the binary from the downloaded tar.gz archive
  and install it to the target directory.
- **FR-007**: Script MUST install to `/usr/local/bin/` by default, falling back
  to `$HOME/.local/bin/` if the default is not writable.
- **FR-008**: Script MUST allow overriding the install directory via
  `FINFOCUS_INSTALL_DIR`.
- **FR-009**: Script MUST allow pinning to a specific version via
  `FINFOCUS_VERSION`.
- **FR-010**: Script MUST print the installed version and usage next steps on
  success.
- **FR-011**: Script MUST exit with a non-zero code and clear error message on
  any failure.
- **FR-012**: Script MUST be POSIX-compatible (no bashisms) and pass ShellCheck
  with no warnings.
- **FR-013**: Script MUST work with either `curl` or `wget` for HTTP requests.
- **FR-014**: Script MUST create `$HOME/.local/bin/` if it does not exist when
  falling back to user-local install.
- **FR-015**: Script MUST use only HTTPS URLs for all downloads.
- **FR-016**: Script MUST clean up temporary files (downloaded archive, checksums)
  on exit, including on failure.
- **FR-017**: Script MUST support the `FINFOCUS_NO_VERIFY` environment variable
  to skip checksum verification, printing a warning when used.

### Key Entities

- **Release Asset**: A platform-specific archive (tar.gz) containing the finfocus
  binary, hosted on GitHub Releases. Named `finfocus-vX.Y.Z-{os}-{arch}.tar.gz`.
- **Checksums File**: A `checksums.txt` file containing SHA256 hashes of all
  release assets, published alongside each GitHub Release.
- **Install Target**: The filesystem directory where the finfocus binary is placed.
  Resolved via `FINFOCUS_INSTALL_DIR`, writable system path, or user-local path.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can install FinFocus on a fresh Linux or macOS machine in
  under 30 seconds with a single command.
- **SC-002**: The install script correctly identifies the platform and downloads
  the matching binary on all four supported platform/architecture combinations
  (Linux amd64, Linux arm64, macOS amd64, macOS arm64).
- **SC-003**: Binary integrity is verified via SHA256 checksum on every default
  install, preventing installation of corrupted or tampered binaries.
- **SC-004**: Users without root/sudo access can still install successfully via
  automatic fallback to a user-local directory.
- **SC-005**: All failure modes produce actionable error messages that tell the
  user exactly what went wrong and how to resolve it.
- **SC-006**: The install script passes ShellCheck with zero warnings, ensuring
  POSIX compatibility and shell scripting best practices.
- **SC-007**: Version pinning allows CI/CD pipelines to reproducibly install a
  specific FinFocus version.

## Assumptions

- GitHub Releases are the sole distribution channel for FinFocus binaries.
- The goreleaser configuration produces tar.gz archives for Linux and macOS with
  the naming pattern `finfocus-vX.Y.Z-{os}-{arch}.tar.gz` where `{os}` is `linux`
  or `macos` (not `darwin`), and `{arch}` is `amd64` or `arm64`.
- A `checksums.txt` file with SHA256 hashes is published with every release.
- The GitHub API endpoint `/repos/rshade/finfocus/releases/latest` is publicly
  accessible without authentication for determining the latest version.
- Either `curl` or `wget` is available on any system where this script would be used.
- Either `sha256sum` (Linux) or `shasum -a 256` (macOS) is available for checksum
  verification.
- The script does not need to support Windows natively (Windows users are directed
  to `go install` or manual download).
- The script does not handle upgrading an existing installation; it overwrites the
  binary at the target path.
