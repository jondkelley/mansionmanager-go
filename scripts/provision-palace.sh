#!/usr/bin/env bash
#
# Debian-style provisioning: one unprivileged Unix user per palace host under /home/<user>/,
# with all server data in /home/<user>/palace/ (pserver.pat, media/, logs, mediaserverurl.txt).
#
# Cron: one global job runs gen-media-nginx.sh --scan-homes (see HOST_IMPLEMENTATION_GUIDE.md).
#
# Prerequisites (operator):
#   - pserver binary (default /usr/local/bin/pserver; override PSERVER_BIN or --pserver-binary)
#   - gen-media-nginx.sh installed (GEN_MEDIA_NGINX or --gen-script)
#   - serverkey.txt + pserver.pat + media tree copied into the palace data dir before start
#     (or use --from-template after deploy/update-install-pserver.sh filled /root/palace-template)
#
# Usage:
#   sudo ./host-provision-demo.sh --user mypalace --tcp-port 9998 --http-port 8080
#   ./host-provision-demo.sh --help       # common options
#   ./host-provision-demo.sh --help-all    # every flag and env vars
#
set -euo pipefail

# Defaults; env vars apply unless overridden on the CLI (see --help-all).
PSERVER_BIN="${PSERVER_BIN:-/usr/local/bin/pserver}"
GEN_SCRIPT="${GEN_MEDIA_NGINX:-/usr/local/bin/gen-media-nginx.sh}"
CRON_SCHEDULE="${PALACE_CRON_SCHEDULE:-*/2 * * * *}"

PALACE_USER="palacedemo"
TCP_PORT="9998"
HTTP_PORT="8080"
VERBOSITY="2"
PROVIDER="MyPalaceNet"
# palace-manager sets PALACE_REVERSE_PROXY_MEDIA from nginx.edgeScheme + mediaHost (see config.json).
REVERSE_PROXY_MEDIA="${PALACE_REVERSE_PROXY_MEDIA:-https://media.thepalace.app}"
PALACE_DATA_DIR=""
CRON_TAG=""
# Default tree produced by deploy/update-install-pserver.sh (sdist tarball).
PALACE_TEMPLATE_DIR="${PALACE_TEMPLATE_DIR:-/root/palace-template}"
# Cron always uses gen-media-nginx --scan-homes with these unless advanced flags override:
MATCH_SCHEME="both"
EDGE_SCHEME="https"

DRY_RUN=false
FROM_TEMPLATE=false
FORCE_TEMPLATE_PAT=false
INSTALL_SYSTEMD=true
INSTALL_LOGROTATE=true
INSTALL_CRON=false   # palace-manager owns the nginx regen loop; no cron needed
JSON_OUTPUT=false
LOGROTATE_ONLY=false
# Non-default systemd unit for postrotate SIGHUP (legacy layouts); default palman-<user>.service
SYSTEMD_UNIT_CLI=""

die() { echo "error: $*" >&2; exit 1; }

# Shared by full provision and --logrotate-only.
# Args: palace_login_user log_file_path systemd_unit_for_hup
build_logrotate_content() {
  local lu="$1" lp="$2" unit="$3"
  printf '%s\n' "# palace ${lu} — rotate log, reopen via SIGHUP (also reloads pserver.pat if changed)
${lp} {
	su ${lu} ${lu}
	daily
	maxsize 500M
	rotate 14
	compress
	delaycompress
	missingok
	notifempty
	create 0644 ${lu} ${lu}
	sharedscripts
	postrotate
		systemctl kill -s HUP ${unit} >/dev/null 2>&1 || true
	endscript
}
"
}

require_rsync() {
  if command -v rsync >/dev/null 2>&1; then
    return
  fi
  echo "error: rsync is not installed, but is required for --from-template." >&2
  if command -v apt-get >/dev/null 2>&1; then
    echo "  Install it with:  apt-get install -y rsync" >&2
  elif command -v yum >/dev/null 2>&1; then
    echo "  Install it with:  yum install -y rsync" >&2
  elif command -v dnf >/dev/null 2>&1; then
    echo "  Install it with:  dnf install -y rsync" >&2
  else
    echo "  Install rsync using your system package manager." >&2
  fi
  exit 1
}

usage_minimal() {
  cat <<'EOF'
Usage:
  sudo ./host-provision-demo.sh [options]

Common options:
  --user NAME              Unprivileged account (default: palacedemo)
  --tcp-port N / --http-port N   Palace TCP / built-in -H HTTP (defaults: 9998 / 8080)
  --data-dir PATH          pserver WorkingDirectory (default: /home/<user>/palace). Alias: --palaces-dir
  --reverseproxymedia URL  Base for --reverseproxymedia (default: https://media.thepalace.app)
  --dry-run                Print actions only
  --no-cron                Skip cron (use when adding a 2nd+ palace user; one global cron is enough)
  --no-systemd             Skip systemd unit
  --no-logrotate           Skip logrotate snippet
  --logrotate-only         Write only /etc/logrotate.d/palace-<user> (needs existing Linux user & paths)
  --systemd-unit NAME.service   With --logrotate-only: unit for postrotate SIGHUP (default palman-<user>.service)

After install: copy pserver.pat + media, then:
  systemctl enable --now palman-<user>.service

  --from-template       Copy pserver binary, media/, ratbot, pserver.pat from PALACE_TEMPLATE_DIR
                        (default /root/palace-template). Does not copy scripts/ into the palace dir.
  --force-template-pat  With --from-template, overwrite an existing pserver.pat (default: keep existing)

Env (optional): PSERVER_BIN, GEN_MEDIA_NGINX, PALACE_CRON_SCHEDULE, PALACE_TEMPLATE_DIR — see --help-all

Full flag list:  --help-all
EOF
}

usage_full() {
  usage_minimal
  cat <<'EOF'

Advanced options (same script; for automation / tuning):
  --verbosity N            -v for pserver (default: 2)
  --provider TEXT          (default: MyPalaceNet)
  --pserver-binary PATH    Overrides PSERVER_BIN for this run
  --gen-script PATH        Overrides GEN_MEDIA_NGINX for this run
  --cron-schedule SPEC     Overrides PALACE_CRON_SCHEDULE for this run
  --cron-tag NAME          /etc/cron.d/<NAME> (default: palace-media-scan-homes)
  --match-scheme https|http|both   Passed to gen-media-nginx in cron (default: both)
  --edge-scheme https|http|dual    nginx edge (default: https — TLS + redirect on :80)
  --from-template        Install pserver, media, ratbot, pserver.pat from PALACE_TEMPLATE_DIR
  --force-template-pat   With --from-template, replace existing pserver.pat in the palace dir
  --lab                    Shorthand: --edge-scheme http for local dev (no TLS termination story)

Note: gen-media-nginx.sh --palaces-dir means “parent of each instance dir”; this script’s --data-dir
      is the palace folder itself. Cron always uses --scan-homes (all /home/*/palace/…).

Environment variables:
  PSERVER_BIN              Path to pserver (default /usr/local/bin/pserver)
  GEN_MEDIA_NGINX          Path to gen-media-nginx.sh
  PALACE_CRON_SCHEDULE     Standard cron schedule for the gen-media-nginx cron line (default */2 * * * *).
                           Lower frequency = less frequent nginx regen from mediaserverurl.txt; override with --cron-schedule.
  PALACE_TEMPLATE_DIR      Bundle tree from update-install-pserver.sh (default /root/palace-template)

EOF
}

# Pre-scan for help before mutating state
for _arg in "$@"; do
  case "$_arg" in
    -h|--help) usage_minimal; exit 0 ;;
    --help-all) usage_full; exit 0 ;;
  esac
done

while [[ $# -gt 0 ]]; do
  case "$1" in
    --user) PALACE_USER="$2"; shift 2 ;;
    --tcp-port) TCP_PORT="$2"; shift 2 ;;
    --http-port) HTTP_PORT="$2"; shift 2 ;;
    --verbosity|-v) VERBOSITY="$2"; shift 2 ;;
    --provider) PROVIDER="$2"; shift 2 ;;
    --reverseproxymedia) REVERSE_PROXY_MEDIA="$2"; shift 2 ;;
    --data-dir|--palaces-dir) PALACE_DATA_DIR="$2"; shift 2 ;;
    --pserver-binary) PSERVER_BIN="$2"; shift 2 ;;
    --gen-script) GEN_SCRIPT="$2"; shift 2 ;;
    --cron-schedule) CRON_SCHEDULE="$2"; shift 2 ;;
    --match-scheme) MATCH_SCHEME="$2"; shift 2 ;;
    --edge-scheme) EDGE_SCHEME="$2"; shift 2 ;;
    --cron-tag) CRON_TAG="$2"; shift 2 ;;
    --lab) EDGE_SCHEME="http"; shift ;;
    --cron-mode)
      case "${2:-}" in
        global)
          echo "warning: --cron-mode global is obsolete (ignored); cron always uses --scan-homes" >&2
          ;;
        instance)
          die "obsolete --cron-mode instance removed; use manual cron from HOST_IMPLEMENTATION_GUIDE.md (--palaces-dir …)"
          ;;
        *) die "obsolete --cron-mode: use HOST_IMPLEMENTATION_GUIDE.md for manual cron"
          ;;
      esac
      shift 2
      ;;
    --from-template) FROM_TEMPLATE=true; shift ;;
    --force-template-pat) FORCE_TEMPLATE_PAT=true; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    --no-systemd) INSTALL_SYSTEMD=false; shift ;;
    --no-logrotate) INSTALL_LOGROTATE=false; shift ;;
    --logrotate-only) LOGROTATE_ONLY=true; shift ;;
    --systemd-unit) SYSTEMD_UNIT_CLI="$2"; shift 2 ;;
    --no-cron) INSTALL_CRON=false; shift ;;
    --cron) INSTALL_CRON=true; shift ;;
    --json) JSON_OUTPUT=true; shift ;;
    -h|--help|--help-all) die "internal: help should have been handled before the option loop" ;;
    *) die "unknown option: $1 (try --help or --help-all)" ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || die "run as root (creates user, systemd, logrotate, cron)"

if $LOGROTATE_ONLY; then
  id -u "$PALACE_USER" &>/dev/null || die "Linux user ${PALACE_USER} does not exist (create the palace first)"
  [[ -z "$PALACE_DATA_DIR" ]] && PALACE_DATA_DIR="/home/${PALACE_USER}/palace"
  UNIT_NAME="${SYSTEMD_UNIT_CLI:-palman-${PALACE_USER}.service}"
  LOG_PATH="${PALACE_DATA_DIR}/pserver.log"
  LOGROTATE_PATH="/etc/logrotate.d/palace-${PALACE_USER}"
  LOGROTATE_CONTENT="$(build_logrotate_content "$PALACE_USER" "$LOG_PATH" "$UNIT_NAME")"
  echo "Writing ${LOGROTATE_PATH} for ${LOG_PATH} (postrotate → ${UNIT_NAME}) ..."
  if $DRY_RUN; then
    echo "[dry-run] would write logrotate (${#LOGROTATE_CONTENT} bytes)"
  else
    printf '%s\n' "$LOGROTATE_CONTENT" > "$LOGROTATE_PATH"
  fi
  if $JSON_OUTPUT; then
    printf '{"ok":true,"user":"%s","logrotatePath":"%s","logPath":"%s","systemdUnit":"%s"}\n' \
      "$PALACE_USER" "$LOGROTATE_PATH" "$LOG_PATH" "$UNIT_NAME"
  fi
  exit 0
fi

# Validate template early — before touching the system — so a retry doesn't
# fail with "user already exists" after a previous partial run.
if $FROM_TEMPLATE && ! $DRY_RUN; then
  require_rsync
  [[ -d "$PALACE_TEMPLATE_DIR" ]]         || die "PALACE_TEMPLATE_DIR not found: ${PALACE_TEMPLATE_DIR} — run update-pserver.sh first"
  [[ -f "${PALACE_TEMPLATE_DIR}/pserver" ]]    || die "template missing pserver: ${PALACE_TEMPLATE_DIR}/pserver"
  [[ -f "${PALACE_TEMPLATE_DIR}/pserver.pat" ]] || die "template missing pserver.pat: ${PALACE_TEMPLATE_DIR}/pserver.pat"
  [[ -d "${PALACE_TEMPLATE_DIR}/media" ]]  || die "template missing media/: ${PALACE_TEMPLATE_DIR}/media"
fi

case "$MATCH_SCHEME" in
  https|http|both) ;;
  *) die "--match-scheme must be https, http, or both" ;;
esac

case "$EDGE_SCHEME" in
  https|http|dual) ;;
  *) die "--edge-scheme must be https, http, or dual" ;;
esac

[[ -z "$CRON_TAG" ]] && CRON_TAG="palace-media-scan-homes"

[[ -z "$PALACE_DATA_DIR" ]] && PALACE_DATA_DIR="/home/${PALACE_USER}/palace"
UNIT_NAME="palman-${PALACE_USER}.service"
UNIT_PATH="/etc/systemd/system/${UNIT_NAME}"
LOGROTATE_PATH="/etc/logrotate.d/palace-${PALACE_USER}"
CRON_PATH="/etc/cron.d/${CRON_TAG}"

run() {
  if $DRY_RUN; then
    printf '[dry-run] '; printf '%q ' "$@"; echo
  else
    "$@"
  fi
}

if ! id -u "$PALACE_USER" &>/dev/null; then
  echo "Creating user ${PALACE_USER} (unprivileged, no login password set) ..."
  run adduser --disabled-password --gecos "" "$PALACE_USER"
else
  echo "User ${PALACE_USER} already exists — skipping adduser."
fi

echo "Creating palace directory ${PALACE_DATA_DIR} ..."
run mkdir -p "${PALACE_DATA_DIR}/media"
run chown -R "${PALACE_USER}:${PALACE_USER}" "$PALACE_DATA_DIR"

# Optional: populate from shared sdist template (see deploy/update-install-pserver.sh).
# Copies only pserver → PSERVER_BIN, media/, ratbot, and pserver.pat — never scripts/.
if $FROM_TEMPLATE; then
  echo ""
  echo "Populating from template ${PALACE_TEMPLATE_DIR} ..."
  if $DRY_RUN; then
    echo "[dry-run] would verify template + install files into ${PALACE_DATA_DIR}"
  fi
  echo "  → shared binary: ${PALACE_TEMPLATE_DIR}/pserver → ${PSERVER_BIN}"
  run install -m 0755 "${PALACE_TEMPLATE_DIR}/pserver" "${PSERVER_BIN}"
  if [[ -f "${PALACE_TEMPLATE_DIR}/ratbot" ]]; then
    echo "  → ratbot → ${PALACE_DATA_DIR}/ratbot"
    run install -m 0755 -o "${PALACE_USER}" -g "${PALACE_USER}" "${PALACE_TEMPLATE_DIR}/ratbot" "${PALACE_DATA_DIR}/ratbot"
  else
    echo "  (no ratbot in template — skipped)"
  fi
  echo "  → media/ → ${PALACE_DATA_DIR}/media/"
  run rsync -a "${PALACE_TEMPLATE_DIR}/media/" "${PALACE_DATA_DIR}/media/"
  run chown -R "${PALACE_USER}:${PALACE_USER}" "${PALACE_DATA_DIR}/media"
  if [[ ! -f "${PALACE_DATA_DIR}/pserver.pat" ]] || $FORCE_TEMPLATE_PAT; then
    echo "  → pserver.pat → ${PALACE_DATA_DIR}/pserver.pat"
    run cp -a "${PALACE_TEMPLATE_DIR}/pserver.pat" "${PALACE_DATA_DIR}/pserver.pat"
    run chown "${PALACE_USER}:${PALACE_USER}" "${PALACE_DATA_DIR}/pserver.pat"
  else
    echo "  → keeping existing ${PALACE_DATA_DIR}/pserver.pat (use --force-template-pat to replace)"
  fi
  echo "Template scripts/ left under ${PALACE_TEMPLATE_DIR}/scripts (not copied into palace dir)."
  echo ""
fi

LOG_PATH="${PALACE_DATA_DIR}/pserver.log"
PAT_PATH="${PALACE_DATA_DIR}/pserver.pat"

if [[ ! -f "$PAT_PATH" ]]; then
  echo ""
  echo "NOTE: Place ${PAT_PATH} (and serverkey.txt beside it if needed), plus media assets under ${PALACE_DATA_DIR}/media/"
  echo "      before starting the service. Binary expected at: ${PSERVER_BIN}"
  echo "      Or run this script with --from-template after update-install-pserver.sh."
  echo ""
fi

EXEC_START="${PSERVER_BIN} -p ${TCP_PORT} -l ${LOG_PATH} -x ${PAT_PATH} -m ${PALACE_DATA_DIR}/media/ -nofork -H ${HTTP_PORT} -v ${VERBOSITY} --provider ${PROVIDER} --reverseproxymedia ${REVERSE_PROXY_MEDIA}"

UNIT_CONTENT="[Unit]
Description=Palace pserver (${PALACE_USER})
Documentation=https://thepalace.app/server.html
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${PALACE_USER}
Group=${PALACE_USER}
WorkingDirectory=${PALACE_DATA_DIR}
ExecStart=${EXEC_START}
Restart=on-failure
RestartSec=5

# Advanced hardening (uncomment & tune paths):
# NoNewPrivileges=true
# PrivateTmp=true
# ProtectSystem=strict
# ReadWritePaths=${PALACE_DATA_DIR}

[Install]
WantedBy=multi-user.target
"

if $INSTALL_SYSTEMD; then
  echo "Writing ${UNIT_PATH} ..."
  if $DRY_RUN; then
    echo "[dry-run] would write systemd unit (${#UNIT_CONTENT} bytes)"
  else
    printf '%s\n' "$UNIT_CONTENT" > "$UNIT_PATH"
  fi
  run systemctl daemon-reload
  echo "Systemd unit installed. Enable with: systemctl enable --now ${UNIT_NAME}"
else
  echo "Skipping systemd (--no-systemd)."
fi

LOGROTATE_CONTENT="$(build_logrotate_content "$PALACE_USER" "$LOG_PATH" "$UNIT_NAME")"

if $INSTALL_LOGROTATE; then
  echo "Writing ${LOGROTATE_PATH} ..."
  if $DRY_RUN; then
    echo "[dry-run] would write logrotate (${#LOGROTATE_CONTENT} bytes)"
  else
    printf '%s\n' "$LOGROTATE_CONTENT" > "$LOGROTATE_PATH"
  fi
else
  echo "Skipping logrotate (--no-logrotate)."
fi

CRON_BODY="# Auto-maintain nginx media map — scans /home/*/palace/…
# Script: ${GEN_SCRIPT}
${CRON_SCHEDULE} root /bin/sh ${GEN_SCRIPT} --scan-homes --match-scheme ${MATCH_SCHEME} --edge-scheme ${EDGE_SCHEME} --reload
"

if $INSTALL_CRON; then
  if [[ ! -x "$GEN_SCRIPT" ]]; then
    echo "WARNING: ${GEN_SCRIPT} is missing or not executable — cron job will fail until you install gen-media-nginx.sh" >&2
  fi
  if [[ -f "$CRON_PATH" ]] && ! $DRY_RUN; then
    echo "WARNING: ${CRON_PATH} already exists — overwriting. Use --cron-tag or merge by hand on shared hosts." >&2
  fi
  echo "Writing ${CRON_PATH} ..."
  if $DRY_RUN; then
    echo "[dry-run] would write cron.d"
  else
    printf '%s\n' "$CRON_BODY" > "$CRON_PATH"
    chmod 0644 "$CRON_PATH"
  fi
  echo "Cron installed (global --scan-homes). Further palace users on this host: re-run with --no-cron."
else
  echo "Skipping cron (--no-cron)."
fi

echo ""
echo "=== Summary ==="
echo "User:           ${PALACE_USER}"
echo "Data dir:       ${PALACE_DATA_DIR}"
echo "TCP / HTTP:     ${TCP_PORT} / ${HTTP_PORT}"
echo "External media: ${REVERSE_PROXY_MEDIA}/<directorykey-sha1>/ (serverkey + directory key)"
echo "Unit:           ${UNIT_NAME}"
echo ""
echo "Host password file (optional): /etc/palacehostpass — must be readable by User=${PALACE_USER}"
echo "  bcrypt hash per line and/or username:hash; chmod 640 /etc/palacehostpass && chgrp ${PALACE_USER} /etc/palacehostpass"
echo ""
echo "Next: copy pserver.pat + media, ensure ${PSERVER_BIN} exists, then:"
echo "  systemctl enable --now ${UNIT_NAME}"
echo "  ${GEN_SCRIPT} --scan-homes --match-scheme ${MATCH_SCHEME} --edge-scheme ${EDGE_SCHEME} --dry-run"
echo "  ${GEN_SCRIPT} --scan-homes --match-scheme ${MATCH_SCHEME} --edge-scheme ${EDGE_SCHEME} --reload"

if $JSON_OUTPUT; then
  printf '{"ok":true,"user":"%s","tcpPort":%s,"httpPort":%s,"dataDir":"%s"}\n' \
    "$PALACE_USER" "$TCP_PORT" "$HTTP_PORT" "$PALACE_DATA_DIR"
fi
