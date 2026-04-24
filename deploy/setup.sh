#!/usr/bin/env bash
# setup.sh — self-install palace-manager from GitHub Releases on the current Linux host.
# Run as root (or it will re-exec with sudo).
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/OWNER/REPO/main/deploy/setup.sh | sudo bash -s -- OWNER/REPO
#   curl -fsSL https://raw.githubusercontent.com/OWNER/REPO/main/deploy/setup.sh | sudo bash -s -- OWNER/REPO v1.2.3
#   curl -fsSL https://raw.githubusercontent.com/jondkelley/mansionmanager-go/main/deploy/setup.sh \| sudo PALACE_MANAGER_GITHUB_REPO=jondkelley/mansionmanager-go bash -s
# Args:
#   $1  GitHub owner/repo (required unless PALACE_MANAGER_GITHUB_REPO is set).
#   $2  Version tag or bare semver (optional). Default: latest GitHub release.
#
# Env:
#   PALACE_MANAGER_GITHUB_REPO   Same as arg 1 if you prefer not to pass repo on the command line.
#   PALACE_MANAGER_VERSION        Override version (e.g. v1.2.3 or 1.2.3); same as arg 2.
set -euo pipefail

REPO="${PALACE_MANAGER_GITHUB_REPO:-${1:-}}"
VERSION_ARG="${PALACE_MANAGER_VERSION:-${2:-}}"

if [[ -z "${REPO}" ]]; then
  echo "Usage: sudo bash $0 <owner/repo> [version]" >&2
  echo "   or: PALACE_MANAGER_GITHUB_REPO=owner/repo sudo bash $0 [version]" >&2
  echo "Example: sudo bash $0 myorg/palaceserver-js" >&2
  exit 1
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "palace-manager releases are built for Linux only." >&2
  exit 1
fi

if [[ "$(id -u)" -ne 0 ]]; then
  exec sudo PALACE_MANAGER_GITHUB_REPO="${REPO}" PALACE_MANAGER_VERSION="${VERSION_ARG}" bash "$0" "${REPO}" "${VERSION_ARG}"
fi

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) ASSET_ARCH=amd64 ;;
  aarch64|arm64) ASSET_ARCH=arm64 ;;
  armv7l|armv6l) ASSET_ARCH=armv7 ;;
  i686|i386) ASSET_ARCH=386 ;;
  *)
    echo "Unsupported machine $(uname -m). Prebuilt assets exist for amd64, arm64, armv7, 386." >&2
    exit 1
    ;;
esac

download() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "${out}" "${url}"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "${out}" "${url}"
  else
    echo "Install curl or wget, then re-run this script." >&2
    exit 1
  fi
}

api_latest_tag() {
  python3 - <<PY
import json, ssl, urllib.request
ctx = ssl.create_default_context()
req = urllib.request.Request(
    "https://api.github.com/repos/${REPO}/releases/latest",
    headers={"Accept": "application/vnd.github+json", "User-Agent": "palace-manager-setup"},
)
with urllib.request.urlopen(req, context=ctx, timeout=60) as r:
    data = json.load(r)
print(data["tag_name"])
PY
}

if [[ -n "${VERSION_ARG}" ]]; then
  TAG="v${VERSION_ARG#v}"
else
  TAG="$(api_latest_tag)"
fi

VER_FILE="${TAG#v}"
ASSET="palace-manager_${VER_FILE}_linux_${ASSET_ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${TAG}"
TMP="$(mktemp -d /tmp/palace-manager-setup-XXXXXX)"
trap 'rm -rf "${TMP}"' EXIT

echo "=== palace-manager setup ==="
echo "    repo:    ${REPO}"
echo "    tag:     ${TAG}"
echo "    asset:   ${ASSET}"

download "${BASE}/${ASSET}" "${TMP}/${ASSET}"

SUMS_OK=0
if command -v curl >/dev/null 2>&1 && curl -fsSL -o "${TMP}/SHA256SUMS" "${BASE}/SHA256SUMS" 2>/dev/null; then
  SUMS_OK=1
elif command -v wget >/dev/null 2>&1 && wget -q -O "${TMP}/SHA256SUMS" "${BASE}/SHA256SUMS" 2>/dev/null; then
  SUMS_OK=1
fi

if [[ "${SUMS_OK}" -eq 1 ]]; then
  (
    cd "${TMP}"
    if grep -qF "${ASSET}" SHA256SUMS 2>/dev/null; then
      grep -F "${ASSET}" SHA256SUMS | sha256sum -c || {
        echo "SHA256 checksum verification failed." >&2
        exit 1
      }
    fi
  )
fi

tar -xzf "${TMP}/${ASSET}" -C "${TMP}"
PALACE_MANAGER_GITHUB_REPO="${REPO}" bash "${TMP}/install.sh" "${TMP}/palace-manager"
rm -rf "${TMP}"
trap - EXIT

echo "=== setup complete ==="
