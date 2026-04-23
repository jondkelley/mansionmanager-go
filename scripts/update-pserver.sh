#!/usr/bin/env bash
#
# update-install-pserver.sh — download official Palace sdist bundle for the host's architecture and lay it down as a
# shared template tree on this host. Other tooling (notably deploy/host-provision-demo.sh with
# --from-template) copies selected artifacts from here into each /home/<user>/palace/ instance.
#
# Why /root/palace-template by default?
#   - Root-owned, one copy per machine, not world-readable unless you chmod it.
#   - Matches the mental model: "golden directory" refreshed from upstream before provisioning
#     or upgrading many palaces.
#
# What the tarball usually contains (sdist.thepalace.app layout):
#   - pserver          — main server binary (also installed to PSERVER_INSTALL_PATH, e.g. /usr/local/bin/pserver)
#   - ratbot           — optional bot helper binary
#   - pserver.pat      — starter / default preferences (copied per instance; you may customize per palace)
#   - media/           — default media tree
#   - scripts/         — helper scripts; kept ONLY under the template — never copied into each
#                         /home/<user>/palace/ by host-provision-demo.sh
#
# Usage:
#   sudo ./update-install-pserver.sh
#   sudo ./update-install-pserver.sh --restartall
#   sudo PALACE_TEMPLATE_DIR=/srv/palace-template ./update-install-pserver.sh --dry-run
#
# Flags:
#   --restartall   After updating the template (and installing the shared pserver binary),
#                  restart every enabled palman-*.service unit so running instances pick up the
#                  new binary. Data dirs are unchanged; this is the common in-place upgrade path.
#
# Recommended rollout (features ship often — stay current ~monthly):
#   1) Run without --restartall first; restart ONE test palace in isolation and verify.
#   2) When satisfied, run again with --restartall so every palman-*.service loads the new pserver.
#   Many hosts adopt a monthly habit: refresh the tarball, smoke-test one instance, then --restartall.
#   --dry-run      Print what would happen; no download, no writes (best-effort; still id -u root).
#   --no-install-binary
#                  Only refresh PALACE_TEMPLATE_DIR from the tarball; do not copy pserver to
#                  PSERVER_INSTALL_PATH (useful if you manage the binary elsewhere).
#
# Environment (optional overrides):
#   PALACE_SDIST_URL        — tarball URL (default: auto-detected from host arch on sdist.thepalace.app)
#   PALACE_TEMPLATE_DIR     — where the extracted tree lives (default: /root/palace-template)
#   PSERVER_INSTALL_PATH    — shared pserver location (default: /usr/local/bin/pserver)
#
set -euo pipefail

# Auto-detect host architecture to pick the right sdist bundle.
_detect_sdist_url() {
  local arch
  case "$(uname -m)" in
    x86_64)        arch=amd64  ;;
    aarch64|arm64) arch=arm64  ;;
    armv7l|armv6l) arch=arm    ;;
    i686|i386)     arch=i386   ;;
    *)             arch=amd64  ;;
  esac
  echo "https://sdist.thepalace.app/linux/latest-linux-${arch}.tar.gz"
}

PALACE_SDIST_URL="${PALACE_SDIST_URL:-$(_detect_sdist_url)}"
PALACE_TEMPLATE_DIR="${PALACE_TEMPLATE_DIR:-/root/palace-template}"
PSERVER_INSTALL_PATH="${PSERVER_INSTALL_PATH:-/usr/local/bin/pserver}"

DRY_RUN=false
RESTART_ALL=false
INSTALL_BINARY=true
JSON_OUTPUT=false

die() { echo "error: $*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Usage:
  sudo ./update-install-pserver.sh [options]

Fetches the latest sdist bundle for this host's architecture (or PALACE_SDIST_URL), extracts to PALACE_TEMPLATE_DIR,
and installs pserver to PSERVER_INSTALL_PATH.

Options:
  --restartall       Restart all palman-<user>.service units after a successful update
  --dry-run          Show actions only
  --no-install-binary  Skip copying pserver to PSERVER_INSTALL_PATH

Environment:
  PALACE_SDIST_URL, PALACE_TEMPLATE_DIR, PSERVER_INSTALL_PATH — see script header comments.

EOF
}

for _arg in "$@"; do
  case "$_arg" in
    -h|--help) usage; exit 0 ;;
  esac
done

while [[ $# -gt 0 ]]; do
  case "$1" in
    --restartall) RESTART_ALL=true; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    --no-install-binary) INSTALL_BINARY=false; shift ;;
    --json) JSON_OUTPUT=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || die "run as root (writes under ${PALACE_TEMPLATE_DIR} and optionally /usr/local/bin)"

run() {
  if $DRY_RUN; then
    printf '[dry-run] '; printf '%q ' "$@"; echo
  else
    "$@"
  fi
}

if $DRY_RUN; then
  echo "[dry-run] Would download:"
  echo "    ${PALACE_SDIST_URL}"
  echo "  Extract → resolve bundle root (directory containing ./pserver)"
  echo "  cp bundle → ${PALACE_TEMPLATE_DIR}/"
  if $INSTALL_BINARY; then
    echo "  install ${PALACE_TEMPLATE_DIR}/pserver → ${PSERVER_INSTALL_PATH}"
  fi
  if $RESTART_ALL; then
    echo "  systemctl restart each /etc/systemd/system/palman-*.service"
  fi
  exit 0
fi

# -----------------------------------------------------------------------------
# Download tarball to a temporary file (cleaned up on exit).
# -----------------------------------------------------------------------------
TMP_DIR=""
cleanup() {
  if [[ -n "${TMP_DIR:-}" && -d "${TMP_DIR:-}" ]]; then
    rm -rf "$TMP_DIR"
  fi
}
trap cleanup EXIT
TMP_DIR="$(mktemp -d -t palace-sdist.XXXXXX)"
ARCHIVE="${TMP_DIR}/bundle.tar.gz"

echo "Downloading:"
echo "  ${PALACE_SDIST_URL}"
echo "→ ${ARCHIVE}"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$PALACE_SDIST_URL" -o "$ARCHIVE"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$ARCHIVE" "$PALACE_SDIST_URL"
else
  die "need curl or wget to download the bundle"
fi

# -----------------------------------------------------------------------------
# Extract and detect the bundle root directory.
# Upstream archives may be flat (pserver at top level) or wrapped in one folder.
# -----------------------------------------------------------------------------
STAGE="${TMP_DIR}/extract"
mkdir -p "$STAGE"
tar -xzf "$ARCHIVE" -C "$STAGE"

# ROOT = directory that directly contains `pserver`.
resolve_bundle_root() {
  local stage="$1"
  local found
  found="$(find "$stage" -type f \( -name pserver -o -name pserver.exe \) 2>/dev/null | head -1 || true)"
  if [[ -z "$found" ]]; then
    die "archive does not contain a file named pserver — unexpected sdist layout"
  fi
  dirname "$found"
}

BUNDLE_ROOT="$(resolve_bundle_root "$STAGE")"

echo "Bundle root resolved to:"
echo "  ${BUNDLE_ROOT}"

# -----------------------------------------------------------------------------
# Publish into PALACE_TEMPLATE_DIR: full tree from the tarball (includes scripts/, media/, …).
# This is the shared "golden" copy operators and host-provision-demo.sh read from.
# We replace the template directory contents to match upstream (backup first if you keep local edits).
# -----------------------------------------------------------------------------
echo "Installing template tree to:"
echo "  ${PALACE_TEMPLATE_DIR}"

rm -rf "$PALACE_TEMPLATE_DIR"
mkdir -p "$PALACE_TEMPLATE_DIR"
cp -a "${BUNDLE_ROOT}/." "${PALACE_TEMPLATE_DIR}/"

# -----------------------------------------------------------------------------
# Executable bits: pserver and ratbot should be runnable when copied or invoked from template.
# -----------------------------------------------------------------------------
for bin in pserver ratbot; do
  if [[ -f "${PALACE_TEMPLATE_DIR}/${bin}" ]]; then
    chmod +x "${PALACE_TEMPLATE_DIR}/${bin}"
  else
    echo "note: ${PALACE_TEMPLATE_DIR}/${bin} not present in this bundle (skipped chmod)" >&2
  fi
done

# -----------------------------------------------------------------------------
# Bundle version stamp (upstream sdist often includes version.txt next to pserver).
# -----------------------------------------------------------------------------
echo ""
if [[ -f "${PALACE_TEMPLATE_DIR}/version.txt" ]]; then
  echo "version.txt (from tarball):"
  sed 's/^/  /' "${PALACE_TEMPLATE_DIR}/version.txt"
else
  echo "note: no version.txt in this bundle — release stamp unknown"
fi

# -----------------------------------------------------------------------------
# Shared system-wide binary: most systemd units expect /usr/local/bin/pserver (see host-provision-demo).
# -----------------------------------------------------------------------------
if $INSTALL_BINARY; then
  echo "Installing shared binary:"
  echo "  ${PALACE_TEMPLATE_DIR}/pserver → ${PSERVER_INSTALL_PATH}"
  if [[ ! -f "${PALACE_TEMPLATE_DIR}/pserver" ]]; then
    die "template missing pserver after extract — aborting binary install"
  fi
  install -m 0755 "${PALACE_TEMPLATE_DIR}/pserver" "$PSERVER_INSTALL_PATH"
else
  echo "Skipping PSERVER_INSTALL_PATH (--no-install-binary)."
fi

echo ""
echo "=== Template ready ==="
echo "  ${PALACE_TEMPLATE_DIR}"
echo ""
echo "Next steps:"
echo "  • New palace user: sudo ./host-provision-demo.sh --user <name> ... --from-template"
echo "  • Or copy media / ratbot / pserver.pat manually from the template path above."
echo ""

# -----------------------------------------------------------------------------
# --restartall: bounce every palman-*.service so ExecStart picks up the new /usr/local/bin/pserver.
# -----------------------------------------------------------------------------
if $RESTART_ALL; then
  echo "=== Restarting palace systemd units (--restartall) ==="
  shopt -s nullglob
  matches=(/etc/systemd/system/palman-*.service)
  shopt -u nullglob
  if [[ ${#matches[@]} -eq 0 ]]; then
    echo "No /etc/systemd/system/palman-*.service files found — nothing to restart."
  else
    n=0
    for path in "${matches[@]}"; do
      unit=$(basename "$path")
      echo "Restarting ${unit} ..."
      systemctl restart "$unit"
      n=$((n + 1))
    done
    echo "Done (${n} unit(s))."
  fi
fi

if $JSON_OUTPUT; then
  printf '{"ok":true}\n'
fi

exit 0
