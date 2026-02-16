# Research: Install Script (curl | sh)

**Feature Branch**: `593-install-script`
**Date**: 2026-02-15

## R1: GoReleaser Asset Naming Convention

**Decision**: Assets follow the pattern `finfocus-vX.Y.Z-{os}-{arch}.tar.gz`

**Rationale**: Confirmed from `.goreleaser.yaml` lines 26-31. The `name_template`
maps `darwin` to `macos` in archive names. Archives are `tar.gz` for Linux/macOS,
`zip` for Windows.

**Alternatives considered**:

- Raw binary downloads (no archive) - rejected; goreleaser produces archives by default
- Custom naming - rejected; goreleaser conventions are well-understood

**Concrete mapping**:

| `uname -s` | `uname -m` | Asset Name |
|-------------|------------|------------|
| Linux | x86_64 | `finfocus-vX.Y.Z-linux-amd64.tar.gz` |
| Linux | aarch64 | `finfocus-vX.Y.Z-linux-arm64.tar.gz` |
| Darwin | x86_64 | `finfocus-vX.Y.Z-macos-amd64.tar.gz` |
| Darwin | arm64 | `finfocus-vX.Y.Z-macos-arm64.tar.gz` |

**Checksums file**: `checksums.txt` (sha256, per `.goreleaser.yaml` line 39)

## R2: POSIX Shell Compatibility

**Decision**: Use `#!/bin/sh` with `set -eu` only; avoid bashisms.

**Rationale**: `set -o pipefail` is NOT POSIX-compliant (bash/zsh only). The
`local` keyword is widely supported but technically a bashism; use it anyway as
all target shells (dash, bash, zsh, ash) support it. Use `type` instead of
`which` for command existence checks (POSIX-compliant).

**Alternatives considered**:

- `#!/bin/bash` with `set -euo pipefail` - rejected; not available on all systems
  (e.g., Alpine uses ash, some minimal containers lack bash)
- `#!/bin/sh` with `set -euf` - rejected; `-f` disables globbing which may be needed

## R3: SHA256 Checksum Verification

**Decision**: Use `sha256sum` on Linux, `shasum -a 256` on macOS, with `openssl`
as a tertiary fallback.

**Rationale**: `sha256sum` is standard on Linux (coreutils). macOS ships `shasum`
(part of Perl) but not `sha256sum`. `openssl` is available on both as a fallback.

**Alternatives considered**:

- OpenSSL only - rejected; output format differs, more complex parsing
- Skip verification - rejected; security requirement (FR-005)

**Verification pattern**: Parse `checksums.txt` (goreleaser format: `<hash>  <filename>`)
and compare against computed hash.

## R4: curl/wget Fallback Strategy

**Decision**: Prefer `curl` with `-fsSL` flags; fall back to `wget -q -O`.

**Rationale**: `curl` is more widely available (ships with macOS). `wget` is
common on Linux servers. Both support HTTPS, redirects, and error handling.

**Key flags**:

- `curl -fsSL`: fail on HTTP errors, silent, show errors, follow redirects
- `wget -q -O`: quiet mode, output to file

## R5: GitHub API for Latest Version

**Decision**: Query `https://api.github.com/repos/rshade/finfocus/releases/latest`
and extract `tag_name` using text processing (no `jq` dependency).

**Rationale**: The GitHub API returns JSON. Rather than requiring `jq`, use
`grep` + `sed` to extract `tag_name`. Unauthenticated rate limit is 60 req/hr,
sufficient for install scripts.

**Alternatives considered**:

- Require `jq` - rejected; not installed by default on most systems
- Scrape HTML release page - rejected; fragile, subject to HTML changes
- GitHub redirect trick (`/releases/latest/download/`) - considered; works for
  downloads but doesn't give us the version string for checksum URL construction

## R6: ShellCheck CI Integration

**Decision**: Add ShellCheck step to the existing CI lint job using
`koalaman/shellcheck-action` or direct `shellcheck` invocation.

**Rationale**: The CI already has a lint job (`ci.yml` lines 86-113). Adding
ShellCheck as a step maintains the existing structure. The `ludeeus/action-shellcheck`
action is widely used but a direct `apt-get install shellcheck && shellcheck scripts/*.sh`
is simpler and avoids action version pinning issues.

**Alternatives considered**:

- Separate workflow - rejected; overkill for one script
- `ludeeus/action-shellcheck` action - viable but adds action dependency
- Make target (`make shellcheck`) - good for local dev, complementary to CI

## R7: Existing Documentation Impact

**Decision**: Update `docs/getting-started/installation.md` and
`docs/getting-started/quickstart.md` to replace "Coming Soon" placeholders.

**Rationale**: Both files have placeholder sections for binary download
(`installation.md` line 53: "Option 2: Download Prebuilt Binary (Coming Soon)",
`quickstart.md` line 27: "Option B: Download binary (coming soon)"). The install
script replaces these with the actual curl | sh command.

## R8: Temporary File Handling

**Decision**: Use `mktemp -d` for temporary directory with `trap` for cleanup.

**Rationale**: `mktemp -d` is POSIX-standard and available on all target platforms.
Trap on `EXIT` ensures cleanup even on errors. Use `-t` flag for template naming
on macOS compatibility (`mktemp -d` without template works on Linux but macOS
requires either `-t` or a template argument).

**Portable pattern**: `mktemp -d 2>/dev/null || mktemp -d -t 'finfocus-install'`
