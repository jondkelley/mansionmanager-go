#!/usr/bin/env bash
# deploy.sh — build and install palace-manager on this host
set -euo pipefail

BINARY=palace-manager
INSTALL_PATH=/usr/local/bin/palace-manager
SCRIPTS_DIR=/usr/local/lib/palace-manager/scripts
CONFIG_DIR=/etc/palace-manager
SERVICE_SRC="$(dirname "$0")/palace-manager.service"
SERVICE_DEST=/etc/systemd/system/palace-manager.service

echo "Building ${BINARY}..."
go build -o "${BINARY}" ./cmd/palace-manager

echo "Installing binary → ${INSTALL_PATH}"
install -m 0755 "${BINARY}" "${INSTALL_PATH}"

echo "Installing scripts → ${SCRIPTS_DIR}"
mkdir -p "${SCRIPTS_DIR}"
install -m 0755 scripts/provision-palace.sh "${SCRIPTS_DIR}/provision-palace.sh"
install -m 0755 scripts/update-pserver.sh   "${SCRIPTS_DIR}/update-pserver.sh"
install -m 0755 scripts/gen-media-nginx.sh  "/usr/local/bin/gen-media-nginx.sh"

echo "Ensuring config dir → ${CONFIG_DIR}"
mkdir -p "${CONFIG_DIR}"
if [[ ! -f "${CONFIG_DIR}/config.json" ]]; then
  ADMIN_PASS=$(tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 20)
  install -m 0600 /dev/null "${CONFIG_DIR}/config.json"
  cat > "${CONFIG_DIR}/config.json" <<EOF
{
  "manager": {
    "port": 3000,
    "host": "0.0.0.0",
    "username": "admin",
    "password": "${ADMIN_PASS}"
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
  echo "  Wrote ${CONFIG_DIR}/config.json"
else
  ADMIN_PASS="(existing — see ${CONFIG_DIR}/config.json)"
  echo "  Config exists — skipping"
fi

echo "Installing systemd unit → ${SERVICE_DEST}"
cp "${SERVICE_SRC}" "${SERVICE_DEST}"
systemctl daemon-reload

echo "Enabling and starting palace-manager..."
systemctl enable palace-manager
systemctl restart palace-manager

sleep 1
if systemctl is-active --quiet palace-manager; then
  echo "  Service is running."
else
  echo "  WARNING: service did not start — check: journalctl -u palace-manager -n 30"
fi

if command -v ufw &>/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
  echo "Opening port 3000 in ufw..."
  ufw allow 3000/tcp comment 'palace-manager' >/dev/null
elif command -v firewall-cmd &>/dev/null && firewall-cmd --state &>/dev/null; then
  echo "Opening port 3000 in firewalld..."
  firewall-cmd --permanent --add-port=3000/tcp >/dev/null
  firewall-cmd --reload >/dev/null
fi

SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")

echo ""
palace-manager version
echo ""
echo "========================================"
echo "  Web UI:   http://${SERVER_IP}:3000"
echo "  Username: admin"
echo "  Password: ${ADMIN_PASS}"
echo "========================================"
echo ""
echo "Open the URL above in your browser — no further setup needed."
echo "Credentials are stored in ${CONFIG_DIR}/users.json (hashed)."
echo "Use the UI or edit that file, then: systemctl restart palace-manager"
