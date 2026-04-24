#!/usr/bin/env bash
# package-release.sh — cross-compile palace-manager and produce a release tarball.
#
# Usage:
#   TAG=v1.2.3 GOOS=linux GOARCH=amd64 ./scripts/package-release.sh
#   TAG=v1.2.3 GOOS=linux GOARCH=arm GOARM=7 ASSET_ARCH=armv7 ./scripts/package-release.sh
#
# Env:
#   TAG         Required. Git tag (v1.2.3); used for -ldflags version and archive name.
#   GOOS        Default: linux
#   GOARCH      Required for non-default path.
#   GOARM       Optional (e.g. 7 when GOARCH=arm).
#   ASSET_ARCH  Optional label in tarball filename (default: GOARCH), e.g. armv7 for linux/arm.
set -euo pipefail

TAG="${TAG:?set TAG=v1.2.3}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:?set GOARCH=amd64}"
ASSET_ARCH="${ASSET_ARCH:-${GOARCH}}"
VERSION="${TAG#v}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Module root: parent of scripts/, or immediate child (monorepo: repo/palaceserver-js/...).
resolve_root() {
  if [[ -n "${PALACE_MANAGER_ROOT:-}" ]]; then
    local forced="${PALACE_MANAGER_ROOT}"
    if [[ "${forced}" != /* ]]; then
      local top
      top="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel 2>/dev/null || true)"
      if [[ -n "${top}" ]]; then
        forced="$(cd "${top}/${forced}" && pwd)"
      else
        forced="$(cd "${SCRIPT_DIR}/../${forced}" && pwd)"
      fi
    else
      forced="$(cd "${forced}" && pwd)"
    fi
    if [[ ! -f "${forced}/go.mod" || ! -d "${forced}/cmd/palace-manager" ]]; then
      echo "PALACE_MANAGER_ROOT must contain go.mod and cmd/palace-manager" >&2
      exit 1
    fi
    echo "${forced}"
    return
  fi
  local up
  up="$(cd "${SCRIPT_DIR}/.." && pwd)"
  if [[ -f "${up}/go.mod" && -d "${up}/cmd/palace-manager" ]]; then
    echo "${up}"
    return
  fi
  local d
  for d in "${up}"/*; do
    if [[ -f "${d}/go.mod" && -d "${d}/cmd/palace-manager" ]]; then
      echo "$(cd "${d}" && pwd)"
      return
    fi
  done
  echo ""
}

ROOT="$(resolve_root)"
if [[ -z "${ROOT}" ]]; then
  echo "Could not find go.mod + cmd/palace-manager. Run from the module tree, or set PALACE_MANAGER_ROOT." >&2
  exit 1
fi

STAGE="$(mktemp -d)"
BINARY_NAME="palace-manager"
ARCHIVE="palace-manager_${VERSION}_${GOOS}_${ASSET_ARCH}.tar.gz"

mkdir -p "${ROOT}/dist"

export GOOS GOARCH
if [[ -n "${GOARM:-}" ]]; then
  export GOARM
fi

echo "=== packaging ${BINARY_NAME} ${VERSION} (${GOOS}/${GOARCH}${GOARM:+ GOARM=${GOARM}}) → dist/${ARCHIVE} ==="

cd "${ROOT}"
go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "${STAGE}/${BINARY_NAME}" ./cmd/palace-manager

cp "${ROOT}/deploy/install.sh" "${STAGE}/"
cp "${ROOT}/deploy/palace-manager.service" "${STAGE}/"
cp "${ROOT}/scripts/provision-palace.sh" "${STAGE}/"
cp "${ROOT}/scripts/update-pserver.sh" "${STAGE}/"
cp "${ROOT}/scripts/gen-media-nginx.sh" "${STAGE}/"

tar -czf "${ROOT}/dist/${ARCHIVE}" -C "${STAGE}" .
rm -rf "${STAGE}"

ls -lh "${ROOT}/dist/${ARCHIVE}"
