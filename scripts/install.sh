#!/bin/sh
# FinFocus install script
# Usage: curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/main/scripts/install.sh | sh
set -eu

REPO="rshade/finfocus"
TMP_DIR=""

cleanup() {
  if [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

trap cleanup EXIT

fail() {
  printf '%s\n' "$@" >&2
  exit 1
}

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

download() {
  url="$1"
  output="$2"
  if type curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 "$url" -o "$output"
  elif type wget >/dev/null 2>&1; then
    wget -q --tries=3 -O "$output" "$url"
  else
    fail "Neither curl nor wget is available. Please install one and try again."
  fi
}

get_latest_version() {
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  release_file="${TMP_DIR}/latest-release.json"
  if ! download "$api_url" "$release_file"; then
    fail "Failed to fetch latest version from GitHub API." \
         "Set FINFOCUS_VERSION to bypass this check:" \
         "  FINFOCUS_VERSION=v0.1.0 sh install.sh"
  fi
  version="$(grep '"tag_name"' "$release_file" | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  if [ -z "$version" ]; then
    fail "Failed to extract version from GitHub API response." \
         "Set FINFOCUS_VERSION to bypass this check:" \
         "  FINFOCUS_VERSION=v0.1.0 sh install.sh"
  fi
  echo "$version"
}

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
    mkdir -p "$local_bin"
    echo "$local_bin"
  fi
}

install_binary() {
  archive="$1"
  install_dir="$2"
  tar -xzf "$archive" -C "$TMP_DIR"
  binary="${TMP_DIR}/finfocus"
  if [ ! -f "$binary" ]; then
    fail "Archive did not contain a 'finfocus' binary."
  fi
  chmod +x "$binary"
  mv "$binary" "${install_dir}/finfocus"
}

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

verify_checksum() {
  archive="$1"
  archive_name="$2"
  version="$3"
  checksums_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
  checksums_file="${TMP_DIR}/checksums.txt"
  if ! download "$checksums_url" "$checksums_file"; then
    fail "Failed to download checksums.txt for verification." \
         "Set FINFOCUS_NO_VERIFY=1 to skip checksum verification (not recommended)."
  fi
  expected="$(grep "$archive_name" "$checksums_file" | cut -d ' ' -f 1)"
  if [ -z "$expected" ]; then
    fail "Checksum for ${archive_name} not found in checksums.txt."
  fi
  actual="$(hash_sha256 "$archive")"
  if [ "$expected" != "$actual" ]; then
    fail "Checksum verification failed!" \
         "  Expected: ${expected}" \
         "  Actual:   ${actual}" \
         "The downloaded archive may be corrupted or tampered with."
  fi
}

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

  if [ -n "${FINFOCUS_NO_VERIFY:-}" ]; then
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
    */usr/local/bin) ;;
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
