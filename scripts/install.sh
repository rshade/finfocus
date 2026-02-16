#!/bin/sh
# FinFocus install script
# Usage: curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
set -eu

REPO="rshade/finfocus"
TMP_DIR=""

# cleanup removes the temporary directory referenced by TMP_DIR if it is set and exists.
cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

trap cleanup EXIT

# fail prints its arguments as an error message to stderr and exits the script with status 1.
fail() {
  printf '%s\n' "$@" >&2
  exit 1
}

# detect_os detects the current operating system and echoes a canonical identifier (`linux` or `macos`).
# On Windows-like environments or unknown platforms the function prints a user-facing error and exits with failure.
detect_os() {
  os="$(uname -s)"
  case "$os" in
    Linux)
      echo "linux"
      ;;
    Darwin)
      echo "macos"
      ;;
    MINGW*|MSYS*|CYGWIN*)
      fail "Windows is not supported by this install script." \
           "Please use one of these alternatives:" \
           "  go install github.com/${REPO}/cmd/finfocus@latest" \
           "  Manual download: https://github.com/${REPO}/releases"
      ;;
    *)
      fail "Unsupported operating system: ${os}" \
           "Supported platforms: Linux, macOS"
      ;;
  esac
}

# detect_arch detects the host CPU architecture and echoes "amd64" for x86_64/amd64 or "arm64" for aarch64/arm64; on other architectures it calls fail with an unsupported-architecture message.
detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      fail "Unsupported architecture: ${arch}" \
           "Supported architectures: amd64, arm64"
      ;;
  esac
}

# download downloads the resource at a URL to the specified output file using curl or wget with retry attempts; it fails if neither tool is available.
download() {
  url="$1"
  output="$2"
  if type curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --connect-timeout 10 --max-time 60 "$url" -o "$output"
  elif type wget >/dev/null 2>&1; then
    wget -q --tries=3 --connect-timeout=10 --timeout=60 -O "$output" "$url"
  else
    fail "Neither curl nor wget is available. Please install one and try again."
  fi
}

# get_latest_version fetches the latest release tag name for ${REPO} from the GitHub API and echoes it; on error it calls fail with instructions to set FINFOCUS_VERSION.
# Prefers jq for robust JSON parsing when available; falls back to grep/sed which
# may break on unexpected whitespace or formatting in the API response.
get_latest_version() {
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  release_file="${TMP_DIR}/latest-release.json"
  if ! download "$api_url" "$release_file"; then
    fail "Failed to fetch latest version from GitHub API." \
         "Set FINFOCUS_VERSION to bypass this check:" \
         "  FINFOCUS_VERSION=v0.1.0 sh install.sh"
  fi
  if command -v jq >/dev/null 2>&1; then
    version="$(jq -r '.tag_name // empty' "$release_file")"
  else
    version="$(grep '"tag_name"' "$release_file" | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  fi
  if [ -z "$version" ]; then
    fail "Failed to extract version from GitHub API response." \
         "Set FINFOCUS_VERSION to bypass this check:" \
         "  FINFOCUS_VERSION=v0.1.0 sh install.sh"
  fi
  echo "$version"
}

# resolve_install_dir selects and echoes the directory where the finfocus binary should be installed, using FINFOCUS_INSTALL_DIR if set (ensuring it exists and is writable), otherwise preferring /usr/local/bin if writable, or creating and returning $HOME/.local/bin.
resolve_install_dir() {
  if [ -n "${FINFOCUS_INSTALL_DIR:-}" ]; then
    dir="$FINFOCUS_INSTALL_DIR"
    if [ ! -d "$dir" ]; then
      mkdir -p "$dir" 2>/dev/null || fail "Install directory ${dir} is not writable"
    fi
    if [ ! -w "$dir" ]; then
      fail "Install directory ${dir} is not writable"
    fi
    echo "$dir"
    return
  fi
  if [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
  else
    local_bin="${HOME}/.local/bin"
    if ! mkdir -p "$local_bin" 2>/dev/null; then
      fail "Failed to create install directory ${local_bin}." \
           "Set FINFOCUS_INSTALL_DIR to specify an alternative directory."
    fi
    if [ ! -w "$local_bin" ]; then
      fail "Install directory ${local_bin} is not writable." \
           "Set FINFOCUS_INSTALL_DIR to specify an alternative directory."
    fi
    echo "$local_bin"
  fi
}

# install_binary extracts the provided tar.gz archive, locates the `finfocus` binary anywhere in the extracted tree, makes it executable, and moves it into the specified installation directory.
install_binary() {
  archive="$1"
  install_dir="$2"
  tar -xzf "$archive" -C "$TMP_DIR"
  binary="$(find "$TMP_DIR" -type f -name 'finfocus' -print -quit)"
  if [ -z "$binary" ] || [ ! -f "$binary" ]; then
    fail "No 'finfocus' binary found in extracted archive under ${TMP_DIR}."
  fi
  # Validate binary is a native executable
  if type file >/dev/null 2>&1; then
    file_type="$(file "$binary")"
    case "$file_type" in
      *ELF*|*Mach-O*)
        # Valid native executable
        ;;
      *)
        fail "Found '${binary}' but it is not a native executable:" \
             "  ${file_type}" \
             "The archive may be corrupt or contain an unsupported format."
        ;;
    esac
  fi
  chmod +x "$binary"
  mv "$binary" "${install_dir}/finfocus"
}

# hash_sha256 computes the SHA-256 checksum of the given file and echoes the hex digest to stdout.
hash_sha256() {
  file="$1"
  if type sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | cut -d ' ' -f 1
  elif type shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | cut -d ' ' -f 1
  elif type openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$file" | awk '{print $NF}'
  else
    fail "No SHA256 tool available (sha256sum, shasum, or openssl required)."
  fi
}

# verify_checksum verifies an archive's SHA-256 checksum against the release's checksums.txt and exits with an error if the checksum is missing or does not match.
verify_checksum() {
  archive="$1"
  archive_name="$2"
  version="$3"
  checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
  checksums_file="${TMP_DIR}/checksums.txt"
  if ! download "$checksums_url" "$checksums_file"; then
    printf 'WARNING: Failed to download checksums.txt, skipping verification.\n' >&2
    return 0
  fi
  expected="$(grep -F -m1 " ${archive_name}" "$checksums_file" | cut -d ' ' -f 1)"
  if [ -z "$expected" ]; then
    printf 'WARNING: Checksum for %s not found in checksums.txt, skipping verification.\n' "$archive_name" >&2
    return 0
  fi
  actual="$(hash_sha256 "$archive")"
  if [ "$expected" != "$actual" ]; then
    fail "Checksum verification failed!" \
         "  Expected: ${expected}" \
         "  Actual:   ${actual}" \
         "The downloaded archive may be corrupted or tampered with."
  fi
}

# main orchestrates the installation of FinFocus: it detects OS and architecture, determines the release version (or uses FINFOCUS_VERSION), downloads the matching release archive, optionally verifies its SHA-256 checksum unless FINFOCUS_NO_VERIFY is set to "1" or "true" (case-insensitive), extracts and installs the finfocus binary into a resolved install directory (which can be overridden with FINFOCUS_INSTALL_DIR), and prints post-install instructions and PATH guidance.
main() {
  TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'finfocus-install')"

  printf 'Installing FinFocus...\n'

  OS="$(detect_os)"
  ARCH="$(detect_arch)"

  if [ -n "${FINFOCUS_VERSION:-}" ]; then
    VERSION="$FINFOCUS_VERSION"
    case "$VERSION" in
      v*) ;;
      *) VERSION="v${VERSION}" ;;
    esac
  else
    VERSION="$(get_latest_version)"
  fi

  ARCHIVE_NAME="finfocus-${VERSION}-${OS}-${ARCH}.tar.gz"
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
  ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE_NAME}"

  printf 'Downloading FinFocus %s (%s/%s)...\n' "$VERSION" "$OS" "$ARCH"
  if ! download "$DOWNLOAD_URL" "$ARCHIVE_PATH"; then
    fail "Failed to download FinFocus ${VERSION}." \
         "Version ${VERSION} may not exist." \
         "Check available versions at https://github.com/${REPO}/releases"
  fi

  no_verify="$(printf '%s' "${FINFOCUS_NO_VERIFY:-}" | tr '[:upper:]' '[:lower:]')"
  if [ "$no_verify" = "1" ] || [ "$no_verify" = "true" ]; then
    printf 'WARNING: Checksum verification disabled. This is not recommended.\n' >&2
  else
    printf 'Verifying checksum...\n'
    verify_checksum "$ARCHIVE_PATH" "$ARCHIVE_NAME" "$VERSION"
  fi

  INSTALL_DIR="$(resolve_install_dir)"

  printf 'Installing to %s...\n' "$INSTALL_DIR"
  install_binary "$ARCHIVE_PATH" "$INSTALL_DIR"

  printf '\nFinFocus %s installed successfully!\n' "$VERSION"
  printf '\nNext steps:\n'
  printf '  finfocus --version\n'
  printf '  finfocus --help\n'

  # Print PATH guidance if using fallback directory
  case "$INSTALL_DIR" in
    /usr/local/bin) ;;
    *)
      case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
          printf '\nNote: %s is not in your PATH.\n' "$INSTALL_DIR"
          printf 'Add it with:\n'
          # Intentional: show literal $PATH for user to copy
          # shellcheck disable=SC2016
          printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
          ;;
      esac
      ;;
  esac
}

main "$@"