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
if [ ! -f /etc/palace-manager/config.json ]; then
  echo "  Generating config with random password..."
  ADMIN_PASS=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 20)
  install -m 0600 /dev/null /etc/palace-manager/config.json
  cat > /etc/palace-manager/config.json <<EOF
{
  "manager": {
    "port": 3000,
    "host": "0.0.0.0",
    "username": "admin",
    "password": "${ADMIN_PASS}",
    "theme": "basic"
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
  # Migrate host from 127.0.0.1 to 0.0.0.0 if still set to the old default.
  python3 - <<'PYEOF'
import json, sys
path = '/etc/palace-manager/config.json'
with open(path) as f:
    d = json.load(f)
if d.get('manager', {}).get('host') == '127.0.0.1':
    d['manager']['host'] = '0.0.0.0'
    with open(path, 'w') as f:
        json.dump(d, f, indent=2)
    print('  Migrated host: 127.0.0.1 → 0.0.0.0')
PYEOF
  ADMIN_PASS=$(python3 -c "import json,sys; d=json.load(open('/etc/palace-manager/config.json')); print(d.get('manager',{}).get('password','(see config.json)'))" 2>/dev/null || echo "(see /etc/palace-manager/config.json)")
fi

# ── 4. Systemd unit ──────────────────────────────────────────────────────────
echo "  Installing systemd unit..."
cp "${SCRIPT_DIR}/palace-manager.service" /etc/systemd/system/palace-manager.service
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
