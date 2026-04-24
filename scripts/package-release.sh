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
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
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
