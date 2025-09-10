#!/usr/bin/env bash
set -euo pipefail

# Portable installer for sbsync release binaries.
#
# Usage examples:
#   bash scripts/install.sh                                # latest -> /usr/local/bin
#   bash scripts/install.sh -v v1.2.3                      # specific version
#   bash scripts/install.sh -b "$HOME/.local/bin"         # custom bin dir
#
# CI one-liner:
#   curl -fsSL https://raw.githubusercontent.com/RoqCode/storyblok-sync/main/scripts/install.sh \
#     | bash -s -- -b /usr/local/bin

REPO="RoqCode/storyblok-sync"
VERSION=""
BIN_DIR="/usr/local/bin"
NO_SUDO="0"
SKIP_CHECKSUM="0"

usage() {
  cat <<EOF
Install sbsync from GitHub Releases

Options:
  -v, --version <tag>       Version tag (default: latest), e.g. v1.2.3
  -b, --bin-dir <dir>       Install directory (default: /usr/local/bin)
      --no-sudo             Do not use sudo even if BIN_DIR requires it
      --skip-checksum       Skip checksum verification (not recommended)
  -h, --help                Show this help

Environment:
  GITHUB_TOKEN              Optional; used to increase GitHub API rate limits
EOF
}

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "error: missing required command '$1'" >&2; exit 1; }; }

detect_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    linux*) echo linux ;;
    darwin*) echo darwin ;;
    msys*|cygwin*|mingw*) echo windows ;;
    *) echo "error: unsupported OS" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
}

http_get() {
  # Adds auth header when GITHUB_TOKEN is present (optional)
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" "$1"
  else
    curl -fsSL "$1"
  fi
}

get_latest_tag() {
  # Avoid jq dependency; parse JSON lightly
  url="https://api.github.com/repos/${REPO}/releases/latest"
  http_get "$url" | sed -n 's/^  \{0,\}"tag_name": "\([^"]\+\)",$/\1/p' | head -n1
}

have_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then echo sha256sum; return; fi
  if command -v shasum >/dev/null 2>&1; then echo "shasum -a 256"; return; fi
  echo "";
}

verify_checksum() {
  local archive=$1 checksums=$2
  local tool
  tool=$(have_sha256)
  if [ -z "$tool" ]; then
    echo "warn: no sha256 tool found; skipping checksum verification" >&2
    return 0
  fi
  # Extract the expected line for the archive and verify
  if echo "$(grep "\s$(basename "$archive")$" "$checksums")" | $tool -c - >/dev/null 2>&1; then
    echo "checksum: OK"
  else
    echo "error: checksum verification failed for $(basename "$archive")" >&2
    exit 1
  fi
}

install_file() {
  local src=$1 dest_dir=$2 name=$3
  mkdir -p "$dest_dir"
  if [ "$NO_SUDO" = "0" ] && [ ! -w "$dest_dir" ]; then
    sudo install -m 0755 "$src" "$dest_dir/$name"
  else
    install -m 0755 "$src" "$dest_dir/$name"
  fi
  echo "installed: $dest_dir/$name"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      -v|--version) VERSION=${2:-}; shift 2 ;;
      -b|--bin-dir) BIN_DIR=${2:-}; shift 2 ;;
      --no-sudo) NO_SUDO=1; shift ;;
      --skip-checksum) SKIP_CHECKSUM=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) echo "error: unknown option: $1" >&2; usage; exit 1 ;;
    esac
  done
}

main() {
  need_cmd curl
  need_cmd tar
  need_cmd install

  parse_args "$@"

  OS=$(detect_os)
  ARCH=$(detect_arch)

  if [ -z "$VERSION" ]; then
    echo "resolving latest version from $REPO ..."
    VERSION=$(get_latest_tag)
    if [ -z "$VERSION" ]; then
      echo "error: could not resolve latest version" >&2
      exit 1
    fi
  fi

  EXT=tar.gz
  [ "$OS" = "windows" ] && EXT=zip
  ARCHIVE="sbsync_${VERSION}_${OS}_${ARCH}.${EXT}"

  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT
  echo "downloading: $ARCHIVE"
  http_get "${BASE_URL}/${ARCHIVE}" >"$tmpdir/$ARCHIVE"

  if [ "$SKIP_CHECKSUM" = "0" ]; then
    echo "downloading: checksums.txt"
    http_get "${BASE_URL}/checksums.txt" >"$tmpdir/checksums.txt"
    verify_checksum "$tmpdir/$ARCHIVE" "$tmpdir/checksums.txt"
  else
    echo "warning: skipping checksum verification" >&2
  fi

  echo "extracting..."
  if [ "$EXT" = "zip" ]; then
    need_cmd unzip
    unzip -q "$tmpdir/$ARCHIVE" -d "$tmpdir"
    BIN_SRC="$tmpdir/sbsync.exe"
    BIN_NAME="sbsync.exe"
  else
    tar -xzf "$tmpdir/$ARCHIVE" -C "$tmpdir"
    BIN_SRC="$tmpdir/sbsync"
    BIN_NAME="sbsync"
  fi

  if [ ! -f "$BIN_SRC" ]; then
    echo "error: extracted binary not found (expected $BIN_SRC)" >&2
    exit 1
  fi

  install_file "$BIN_SRC" "$BIN_DIR" "$BIN_NAME"
}

main "$@"
