#!/usr/bin/env bash
# install-release.sh — install a published GitHub release onto a remote Linux host.
#
# Usage (via Makefile recommended):
#   make install-release VERSION=1.2.3 HOST=root@server RELEASE_REPO=owner/palaceserver-js
#
# Env:
#   VERSION                  Release version (1.2.3 or v1.2.3).
#   HOST                     ssh target user@host (same as ssh(1)).
#   RELEASE_REPO / PALACE_MANAGER_GITHUB_REPO   GitHub "owner/repo" (no https://).
#
# The remote host must be Linux, have curl or wget, and reach github.com.
set -euo pipefail

VERSION_RAW="${VERSION:-}"
HOST="${HOST:-}"
REPO="${RELEASE_REPO:-${PALACE_MANAGER_GITHUB_REPO:-}}"

if [[ -z "${VERSION_RAW}" || -z "${HOST}" ]]; then
  echo "Usage: VERSION=1.2.3 HOST=user@host [RELEASE_REPO=owner/repo] $0" >&2
  echo "   or: make install-release VERSION=1.2.3 HOST=user@host RELEASE_REPO=owner/repo" >&2
  exit 1
fi

if [[ -z "${REPO}" ]]; then
  if ROOT="$(git -C "$(dirname "$0")/.." rev-parse --show-toplevel 2>/dev/null)"; then
    URL="$(git -C "${ROOT}" remote get-url origin 2>/dev/null || true)"
    case "${URL}" in
      *github.com:*/*)
        REPO="${URL#*github.com:}"
        REPO="${REPO%.git}"
        ;;
      *github.com/*/*)
        REPO="${URL#*github.com/}"
        REPO="${REPO%.git}"
        ;;
    esac
  fi
fi

if [[ -z "${REPO}" ]]; then
  echo "Set RELEASE_REPO or PALACE_MANAGER_GITHUB_REPO to your GitHub owner/repo (e.g. myorg/palaceserver-js)." >&2
  exit 1
fi

TAG="v${VERSION_RAW#v}"
VER_FILE="${TAG#v}"

REMOTE_UNAME="$(ssh -o ConnectTimeout=15 "${HOST}" 'uname -m' </dev/null)"
case "${REMOTE_UNAME}" in
  x86_64) ASSET_ARCH=amd64 ;;
  aarch64|arm64) ASSET_ARCH=arm64 ;;
  armv7l|armv6l) ASSET_ARCH=armv7 ;;
  i686|i386) ASSET_ARCH=386 ;;
  *)
    echo "Unknown remote uname -m '${REMOTE_UNAME}'. Supported: x86_64, aarch64, armv7l, i686." >&2
    exit 1
    ;;
esac

ASSET="palace-manager_${VER_FILE}_linux_${ASSET_ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

echo "=== install-release → ${HOST} (${REMOTE_UNAME} → ${ASSET}) ==="
echo "    ${DOWNLOAD_URL}"

REMOTE_TMP="$(ssh "${HOST}" 'mktemp -d /tmp/palace-manager-release-XXXXXX')"
trap '[[ -n "${REMOTE_TMP}" ]] && ssh -o ConnectTimeout=10 "${HOST}" "rm -rf ${REMOTE_TMP}" 2>/dev/null || true' EXIT

ssh "${HOST}" bash -s -- "${DOWNLOAD_URL}" "${REMOTE_TMP}" "${ASSET}" <<'REMOTE'
set -euo pipefail
DOWNLOAD_URL="$1"
REMOTE_TMP="$2"
ASSET="$3"
cd "${REMOTE_TMP}"

download() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "${out}" "${url}"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "${out}" "${url}"
  else
    echo "Remote needs curl or wget to fetch the release." >&2
    exit 1
  fi
}

download "${DOWNLOAD_URL}" "${REMOTE_TMP}/${ASSET}"
tar -xzf "${REMOTE_TMP}/${ASSET}" -C "${REMOTE_TMP}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Re-running install as root via sudo..." >&2
  exec sudo bash "${REMOTE_TMP}/install.sh" "${REMOTE_TMP}/palace-manager"
fi

bash "${REMOTE_TMP}/install.sh" "${REMOTE_TMP}/palace-manager"
rm -rf "${REMOTE_TMP}"
REMOTE

trap - EXIT

echo "=== done ==="
