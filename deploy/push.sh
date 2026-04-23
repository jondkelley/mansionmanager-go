#!/usr/bin/env bash
# push.sh — build locally and deploy to a remote server in one shot.
#
# Usage:
#   ./deploy/push.sh root@192.168.64.3
#   make push HOST=root@192.168.64.3
set -euo pipefail

HOST="${1:-${HOST:-}}"
if [[ -z "$HOST" ]]; then
  echo "Usage: $0 <user@host>" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BINARY=palace-manager

echo "=== palace-manager push → ${HOST} ==="

# ── 0. Shared SSH connection (one password prompt for the whole script) ───────
SSH_CTL="$(mktemp -d)/ssh-ctl"
SSH_OPTS=(-o ControlMaster=yes -o ControlPath="${SSH_CTL}" -o ControlPersist=60s)
echo "  Connecting..."
ssh "${SSH_OPTS[@]}" -fN "${HOST}"
trap 'ssh -o ControlPath="${SSH_CTL}" -O exit "${HOST}" 2>/dev/null; rm -f "${SSH_CTL}"' EXIT

SCP_OPTS=(-o ControlPath="${SSH_CTL}")

# ── 1. Detect remote architecture and cross-compile ──────────────────────────
REMOTE_UNAME=$(ssh -o ControlPath="${SSH_CTL}" "${HOST}" 'uname -m')
case "${REMOTE_UNAME}" in
  x86_64)       GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  armv7l|armv6l) GOARCH=arm ;;
  i686|i386)    GOARCH=386 ;;
  *)
    echo "  Unknown remote arch '${REMOTE_UNAME}' — defaulting to amd64" >&2
    GOARCH=amd64 ;;
esac

echo "  Remote arch: ${REMOTE_UNAME} → building linux/${GOARCH}..."
cd "${REPO_DIR}"
GOOS=linux GOARCH="${GOARCH}" go build -o "${BINARY}" ./cmd/palace-manager
echo "  Built: $(du -sh "${BINARY}" | cut -f1) binary"

# ── 2. Create remote staging directory and upload ────────────────────────────
REMOTE_TMP=$(ssh -o ControlPath="${SSH_CTL}" "${HOST}" 'mktemp -d /root/palace-deploy-XXXXXX')
echo "  Uploading to ${HOST}:${REMOTE_TMP}/ ..."
scp "${SCP_OPTS[@]}" -q \
  "${BINARY}" \
  "${SCRIPT_DIR}/install.sh" \
  "${SCRIPT_DIR}/palace-manager.service" \
  "${REPO_DIR}/scripts/provision-palace.sh" \
  "${REPO_DIR}/scripts/update-pserver.sh" \
  "${HOST}:${REMOTE_TMP}/"

# ── 3. Run install on the server ─────────────────────────────────────────────
echo "  Running install.sh on ${HOST}..."
echo ""
ssh -o ControlPath="${SSH_CTL}" -t "${HOST}" \
  "bash ${REMOTE_TMP}/install.sh ${REMOTE_TMP}/${BINARY} && rm -rf ${REMOTE_TMP}"

# Clean up local build artifact
rm -f "${BINARY}"
