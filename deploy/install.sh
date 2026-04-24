#!/usr/bin/env bash
# install.sh — run as root on the target server to install palace-manager
set -euo pipefail

BINARY_SRC="${1:-/tmp/palace-manager}"
# All other artifacts are expected alongside this script.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== palace-manager install ==="

# ── 1. Binary ────────────────────────────────────────────────────────────────
echo "  Installing binary..."
install -m 0755 "$BINARY_SRC" /usr/local/bin/palace-manager

# ── 2. Helper scripts ────────────────────────────────────────────────────────
echo "  Installing scripts..."
mkdir -p /usr/local/lib/palace-manager/scripts
install -m 0755 "${SCRIPT_DIR}/provision-palace.sh" /usr/local/lib/palace-manager/scripts/provision-palace.sh
install -m 0755 "${SCRIPT_DIR}/update-pserver.sh"   /usr/local/lib/palace-manager/scripts/update-pserver.sh
install -m 0755 "${SCRIPT_DIR}/gen-media-nginx.sh" /usr/local/bin/gen-media-nginx.sh

# ── 3. Config ────────────────────────────────────────────────────────────────
mkdir -p /etc/palace-manager
GITHUB_REPO="${PALACE_MANAGER_GITHUB_REPO:-}"

if [ ! -f /etc/palace-manager/config.json ]; then
  echo "  Generating config with random password..."
  ADMIN_PASS=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 20 || true)
  install -m 0600 /dev/null /etc/palace-manager/config.json
  cat > /etc/palace-manager/config.json <<EOF
{
  "manager": {
    "port": 3000,
    "host": "0.0.0.0",
    "username": "admin",
    "password": "${ADMIN_PASS}",
    "theme": "basic",
    "githubRepo": "${GITHUB_REPO}"
  },
  "scripts": {
    "provision": "/usr/local/lib/palace-manager/scripts/provision-palace.sh",
    "update": "/usr/local/lib/palace-manager/scripts/update-pserver.sh"
  },
  "pserver": {
    "templateDir": "/root/palace-template",
    "installPath": "/usr/local/bin/pserver",
    "sdistUrl": ""
  },
  "nginx": {
    "genScript": "/usr/local/bin/gen-media-nginx.sh",
    "regenInterval": "2m",
    "mediaHost": "media.thepalace.app",
    "certDir": "/etc/letsencrypt/live/media.thepalace.app",
    "edgeScheme": "https",
    "matchScheme": "both"
  }
}
EOF
else
  echo "  Config exists — keeping it (password unchanged)"
  # Migrate: host 127.0.0.1 → 0.0.0.0; add githubRepo if missing and we know it.
  GITHUB_REPO_ESC="${GITHUB_REPO}"
  python3 - <<PYEOF
import json
path = '/etc/palace-manager/config.json'
github_repo = '${GITHUB_REPO_ESC}'
with open(path) as f:
    d = json.load(f)
changed = False
if d.get('manager', {}).get('host') == '127.0.0.1':
    d['manager']['host'] = '0.0.0.0'
    changed = True
    print('  Migrated host: 127.0.0.1 \u2192 0.0.0.0')
if github_repo and not d.get('manager', {}).get('githubRepo'):
    d.setdefault('manager', {})['githubRepo'] = github_repo
    changed = True
    print(f'  Set githubRepo: {github_repo}')
if changed:
    with open(path, 'w') as f:
        json.dump(d, f, indent=2)
PYEOF
  ADMIN_PASS=$(python3 -c "import json,sys; d=json.load(open('/etc/palace-manager/config.json')); print(d.get('manager',{}).get('password','(see config.json)'))" 2>/dev/null || echo "(see /etc/palace-manager/config.json)")
fi

# ── 4. Systemd unit ──────────────────────────────────────────────────────────
SERVICE_SRC="${SCRIPT_DIR}/palace-manager.service"
if [[ ! -f "${SERVICE_SRC}" ]]; then
  echo "  ERROR: missing ${SERVICE_SRC}" >&2
  echo "  install.sh must run from a full release tree (same directory as palace-manager.service)." >&2
  echo "  Official install: extract the release .tar.gz and run: sudo bash install.sh ./palace-manager" >&2
  exit 1
fi
echo "  Installing systemd unit..."
cp "${SERVICE_SRC}" /etc/systemd/system/palace-manager.service
systemctl daemon-reload

# ── 5. Enable & start ────────────────────────────────────────────────────────
echo "  Enabling and starting palace-manager..."
systemctl enable palace-manager
systemctl restart palace-manager

# Give the service a moment to come up, then verify.
sleep 1
if systemctl is-active --quiet palace-manager; then
  echo "  Service is running."
else
  echo "  WARNING: service did not start — check: journalctl -u palace-manager -n 30"
fi

# ── 6. Firewall ──────────────────────────────────────────────────────────────
if command -v ufw &>/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
  echo "  Opening port 3000 in ufw..."
  ufw allow 3000/tcp comment 'palace-manager' >/dev/null
elif command -v firewall-cmd &>/dev/null && firewall-cmd --state &>/dev/null; then
  echo "  Opening port 3000 in firewalld..."
  firewall-cmd --permanent --add-port=3000/tcp >/dev/null
  firewall-cmd --reload >/dev/null
fi

# ── 7. Done ──────────────────────────────────────────────────────────────────
SERVER_IP=$(hostname -I | awk '{print $1}')

echo ""
palace-manager version
echo ""
echo "========================================"
echo "  Web UI:   http://${SERVER_IP}:3000"
echo "  Username: admin"
echo "  Password: ${ADMIN_PASS}"
echo "========================================"
echo ""
echo "Open the URL above in your browser to manage Palace servers."
echo "Credentials live in /etc/palace-manager/users.json (hashed)."
echo "Use the manager UI (Users tab) or edit that file, then: systemctl restart palace-manager"
