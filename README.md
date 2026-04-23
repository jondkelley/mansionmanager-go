# palace-manager

A management service for hosting multiple [pserver](https://thepalace.app/server.html) (Palace server) instances on a single Linux host. It wraps `systemd`, the provisioning shell scripts, and `gen-media-nginx.sh` behind a REST API and a web dashboard — so standing up a new palace, updating binaries, and configuring nginx + TLS can all be done from a browser or curl.

**This is a Linux-only tool.** It uses `systemd`, `adduser`, `certbot`, and nginx — it is designed for Debian/Ubuntu VPS operators.

---

## What it does

| Capability | How |
|-----------|-----|
| Provision a new palace | Creates a Linux user, data dir, systemd unit, and logrotate config via `scripts/provision-palace.sh` |
| Update pserver binary | Downloads latest official tarball from `sdist.thepalace.app` for your arch and refreshes the template via `scripts/update-pserver.sh` |
| Start / stop / restart | Wraps `systemctl` for each `palace-*.service` unit |
| Nginx media regen | Runs `gen-media-nginx.sh --scan-homes --reload` on a built-in ticker (default 2 min) — **no cron job needed** |
| Let's Encrypt bootstrap | One-shot `palace-manager bootstrap` command installs deps, obtains a cert, writes the renewal hook, and generates the initial nginx config |
| Web UI | Single-page dashboard embedded in the binary; served at `http://127.0.0.1:3000/` |
| REST API | Same port; all endpoints require `Authorization: Bearer <apiKey>` |

---

## Architecture / URL scheme

Downloads are auto-detected for your host at runtime:

| OS | Arch | sdist URL |
|----|------|-----------|
| Linux | amd64 | `…/linux/latest-linux-amd64.tar.gz` |
| Linux | arm64 | `…/linux/latest-linux-arm64.tar.gz` |
| Linux | i386 | `…/linux/latest-linux-i386.tar.gz` |
| macOS | arm64 | `…/mac/latest-darwin-arm64.tar.gz` |
| FreeBSD | amd64 | `…/freebsd/latest-freebsd-amd64.tar.gz` |

Override by setting `sdistUrl` in `config.json` if you want to cross-fetch a different arch.

---

## Prerequisites

- Debian 11+ or Ubuntu 22.04+ (tested on Debian 13 arm64 / amd64)
- Root access (the provisioning scripts create Linux users, write systemd units, etc.)
- `rsync` — needed by `--from-template` in the provision script: `apt install rsync`
- For TLS media proxy: `nginx`, `certbot`, `python3-certbot-nginx` — the bootstrap command installs these for you

---

## Building from source

Go 1.22+ required.

```bash
git clone <this repo>
cd palaceserver-js   # or wherever palace-manager lives
go build -o palace-manager ./cmd/palace-manager
```

### Cross-compile for a remote Linux host

If your build machine is macOS (common for local dev):

```bash
# For a Debian arm64 VPS (e.g. Raspberry Pi, Ampere, Hetzner CAX):
GOOS=linux GOARCH=arm64 go build -o palace-manager-linux-arm64 ./cmd/palace-manager

# For a Debian amd64 VPS:
GOOS=linux GOARCH=amd64 go build -o palace-manager-linux-amd64 ./cmd/palace-manager
```

---

## Installing on a server

Run all of the following **as root**.

### 1. Copy files to the server

```bash
# From your build machine:
scp palace-manager-linux-arm64 root@YOUR_HOST:/tmp/palace-manager
scp scripts/provision-palace.sh scripts/update-pserver.sh scripts/gen-media-nginx.sh root@YOUR_HOST:/tmp/
scp config/config.json root@YOUR_HOST:/tmp/palace-manager-config.json
scp deploy/palace-manager.service root@YOUR_HOST:/tmp/
```

Or use `make install` directly on the server if you have the source there.

### 2. Install the binary, scripts, and systemd unit

```bash
# Install binary
install -m 0755 /tmp/palace-manager /usr/local/bin/palace-manager

# Install provisioning scripts
mkdir -p /usr/local/lib/palace-manager/scripts
install -m 0755 /tmp/provision-palace.sh /usr/local/lib/palace-manager/scripts/provision-palace.sh
install -m 0755 /tmp/update-pserver.sh   /usr/local/lib/palace-manager/scripts/update-pserver.sh

# Nginx media generator (referenced by config nginx.genScript)
install -m 0755 /tmp/gen-media-nginx.sh /usr/local/bin/gen-media-nginx.sh

# Write default config (only if one doesn't exist)
mkdir -p /etc/palace-manager
[ ! -f /etc/palace-manager/config.json ] && \
  install -m 0600 /tmp/palace-manager-config.json /etc/palace-manager/config.json

# Install and reload systemd unit
cp /tmp/palace-manager.service /etc/systemd/system/palace-manager.service
systemctl daemon-reload
```

Alternatively, with the `Makefile` on the server:

```bash
make install
```

### 3. Configure

Edit `/etc/palace-manager/config.json`:

```json
{
  "manager": {
    "port": 3000,
    "host": "127.0.0.1",
    "apiKey": "your-secret-key-here"
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
    "mediaHost": "media.yourhost.com",
    "certDir": "/etc/letsencrypt/live/yourhost.com",
    "edgeScheme": "https",
    "matchScheme": "both"
  }
}
```

Key things to change:
- **`apiKey`** — set to a strong random secret; this protects your entire management API
- **`mediaHost`** — the public HTTPS hostname for your media proxy (must match your DNS + TLS cert)
- **`certDir`** — path to the Let's Encrypt `live/` directory certbot creates for that hostname
- **`sdistUrl`** — leave empty to auto-detect for your server's architecture, or set explicitly

### 4. Enable and start

```bash
systemctl enable --now palace-manager
systemctl status palace-manager
```

The web dashboard is now at **`http://127.0.0.1:3000/`** — access it via an SSH tunnel:

```bash
# On your local machine:
ssh -L 3000:127.0.0.1:3000 user@YOUR_HOST
# Then open http://localhost:3000 in your browser
```

---

## First-time host bootstrap (TLS + nginx)

If this is a fresh host with no nginx or TLS certificate yet, run the bootstrap command **as root**:

```bash
palace-manager bootstrap \
  --media-host media.yourhost.com \
  --email you@example.com
```

This runs 7 idempotent steps:

| Step | What happens |
|------|-------------|
| `deps` | `apt install nginx certbot python3-certbot-nginx rsync` |
| `dns` | Resolves `--media-host` and compares to this server's public IP (advisory) |
| `cert` | `certbot certonly --nginx -d <media-host>` — skipped if cert already exists |
| `dhparam` | Generates `/etc/letsencrypt/ssl-dhparams.pem` if missing |
| `hook` | Writes `/etc/letsencrypt/renewal-hooks/deploy/nginx-reload.sh` for auto-renewal |
| `nginx` | Runs `gen-media-nginx.sh --scan-homes --reload` to generate the initial nginx config |
| `config` | Saves `certDir` and `mediaHost` back to `/etc/palace-manager/config.json` |

Use `--staging` during testing to avoid Let's Encrypt rate limits:

```bash
palace-manager bootstrap --media-host media.yourhost.com --email you@example.com --staging
```

Run individual steps only:

```bash
palace-manager bootstrap --steps deps,cert
```

The same flow is available through the web UI under **Host Setup**.

### How TLS paths relate to palace URLs

- **`gen-media-nginx.sh` does not read certificate paths from `mediaserverurl.txt`.** Those files tell nginx how to proxy path segments to each palace’s internal HTTP URL. TLS is configured separately via **`nginx.mediaHost`** and **`nginx.certDir`** in `/etc/palace-manager/config.json`.
- **`palace-manager` always writes the media vhost to** `/etc/nginx/sites-enabled/100-palace-manager-media.conf` **(fixed name)** so it stays the same regardless of hostname; **`server_name`** inside the file still comes from **`nginx.mediaHost`**.
- **`certDir` must contain the PEMs for `mediaHost`.** Defaults now set `certDir` to `/etc/letsencrypt/live/<mediaHost>/`, matching what Certbot creates when you request a certificate for that hostname.
- **`config.json` is updated only when the `config` bootstrap step runs** (after the earlier steps succeed). If **`cert`** fails (e.g. DNS `NXDOMAIN`), the wizard stops and the on-disk config can still have old placeholders — nginx regen then points at certs that were never issued. Fix DNS (or use **`edgeScheme`: `http`** for local HTTP-only testing), complete Host Setup through **`config`**, then use **Regen Now** again.
- **Certificate renewal:** Certbot ships a **systemd timer** on Debian/Ubuntu (`certbot.timer`) — not a cron line. The bootstrap **`hook`** step installs a **deploy hook** that runs `systemctl reload nginx` after a successful renew so new certs are picked up without manual restarts.

---

## Provisioning a palace

### Via the web UI

1. Open the **Palaces** tab, click **+ New Palace**
2. Enter a name (used as the Linux username), TCP port, and HTTP port
3. Click **Provision** — live output streams in the modal

### Via the API

```bash
curl -X POST http://127.0.0.1:3000/api/palaces \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"mypalace","tcpPort":9998,"httpPort":8080}'
```

Output streams as SSE. After provisioning:

1. Copy `pserver.pat` and `serverkey.txt` (if using a registered directory key) into `/home/mypalace/palace/`
2. Enable the service:
   ```bash
   systemctl enable --now palace-mypalace.service
   ```
3. nginx will pick up the new palace's `mediaserverurl.txt` within the next regen interval (default 2 min)

### Second (and further) palaces

Use unique TCP and HTTP ports for each. The cron job is not needed — palace-manager runs nginx regen internally on a timer.

```bash
# Palace 2: different ports
curl -X POST http://127.0.0.1:3000/api/palaces \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"anotherpalace","tcpPort":9999,"httpPort":8081}'
```

---

## Updating pserver

```bash
# Via API (update binary only):
curl -X POST "http://127.0.0.1:3000/api/update" \
  -H "Authorization: Bearer YOUR_API_KEY"

# Update binary AND restart all palace services:
curl -X POST "http://127.0.0.1:3000/api/update?restartAll=true" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Both stream output. The **Update Binary** tab in the web UI does the same.

---

## REST API reference

All endpoints require `Authorization: Bearer <apiKey>`.

### Palaces

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/palaces` | List all palace instances with status |
| `POST` | `/api/palaces` | Provision a new palace (streams SSE) |
| `GET` | `/api/palaces/:name` | Get a single palace |
| `POST` | `/api/palaces/:name/start` | Start |
| `POST` | `/api/palaces/:name/stop` | Stop |
| `POST` | `/api/palaces/:name/restart` | Restart |
| `DELETE` | `/api/palaces/:name` | Disable unit (`?purge=true` also removes the Linux user) |
| `GET` | `/api/palaces/:name/logs` | Tail log (`?lines=N`, default 100) |

### Binary update

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/update` | Update pserver binary (`?restartAll=true` to also restart all units) |

### Nginx

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/nginx/status` | Last regen time, exit code, next scheduled run |
| `POST` | `/api/nginx/regen` | Trigger an immediate regen (streams SSE) |

### Bootstrap

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/bootstrap/status` | Current state of each setup step |
| `POST` | `/api/bootstrap/run` | Run bootstrap (body: `{"mediaHost","email","staging","steps":[…]}`) — streams SSE |

---

## Nginx regen — no cron needed

palace-manager runs `gen-media-nginx.sh --scan-homes --reload` on an internal ticker (configurable via `nginx.regenInterval`, default `2m`). This replaces the `/etc/cron.d/palace-media-scan-homes` entry that `host-provision-demo.sh` would normally install.

A regen is also triggered automatically whenever a new palace is provisioned or removed.

---

## Config reference

`/etc/palace-manager/config.json`:

```json
{
  "manager": {
    "port": 3000,          // HTTP port for the management API + web UI
    "host": "127.0.0.1",  // bind address — 127.0.0.1 means SSH-tunnel-only access
    "apiKey": "…"          // Bearer token required on all API calls
  },
  "scripts": {
    "provision": "…/provision-palace.sh",
    "update": "…/update-pserver.sh"
  },
  "pserver": {
    "templateDir": "/root/palace-template",   // populated by update-pserver.sh
    "installPath": "/usr/local/bin/pserver",
    "sdistUrl": ""                             // empty = auto-detect from runtime arch
  },
  "nginx": {
    "genScript": "/usr/local/bin/gen-media-nginx.sh",
    "regenInterval": "2m",                    // how often to run nginx regen (Go duration string)
    "mediaHost": "media.yourhost.com",
    "certDir": "/etc/letsencrypt/live/…",
    "edgeScheme": "https",                    // https | http | dual
    "matchScheme": "both"                     // both | https | http
  }
}
```

Override the config path with `-config /path/to/config.json` or `PALACE_MANAGER_CONFIG` env var.

---

## Permissions model

palace-manager needs root because provisioning creates Linux users, writes systemd units, and runs `apt`. The service unit (`deploy/palace-manager.service`) runs as root by default.

For a least-privilege setup, run `palace-manager bootstrap` once as root, then create `/etc/sudoers.d/palace-manager` granting the service user only the needed commands (adduser, systemctl, install).

---

## Files on disk

| Path | Purpose |
|------|---------|
| `/usr/local/bin/palace-manager` | Manager binary |
| `/etc/palace-manager/config.json` | Manager config |
| `/etc/palace-manager/registry.json` | Per-palace port + metadata registry (auto-managed) |
| `/usr/local/lib/palace-manager/scripts/` | Shell scripts for provisioning and updates |
| `/etc/systemd/system/palace-manager.service` | Systemd unit for the manager itself |
| `/root/palace-template/` | Shared pserver template tree (populated by update-pserver.sh) |
| `/usr/local/bin/pserver` | Shared pserver binary (installed by update-pserver.sh) |
| `/home/<name>/palace/` | Per-palace data: `pserver.pat`, `media/`, logs, `mediaserverurl.txt` |
| `/etc/systemd/system/palace-<name>.service` | Per-palace systemd unit |
| `/etc/logrotate.d/palace-<name>` | Per-palace log rotation |

---

## Maintenance

```bash
# Restart the manager after config changes:
systemctl restart palace-manager

# Tail manager logs:
journalctl -u palace-manager -f

# Manually trigger nginx regen:
curl -X POST http://127.0.0.1:3000/api/nginx/regen \
  -H "Authorization: Bearer YOUR_API_KEY"

# Check bootstrap step states:
curl http://127.0.0.1:3000/api/bootstrap/status \
  -H "Authorization: Bearer YOUR_API_KEY"
```
# mansionmanager-go
# mansionmanager-go
