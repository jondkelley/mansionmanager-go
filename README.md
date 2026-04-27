# palace-manager

A management service for hosting multiple [pserver](https://thepalace.app/server.html) (Palace server) instances on a single Linux host. It wraps `systemd`, the provisioning shell scripts, and `gen-media-nginx.sh` behind a REST API and a web dashboard — so standing up a new palace, updating binaries, and configuring nginx + TLS can all be done from a browser or curl.

**This is a Linux-only tool.** It uses `systemd`, `certbot`, and nginx. It is tested on **Debian/Ubuntu** and supported on **RHEL-derived** distros (AlmaLinux, Rocky Linux, CentOS Stream, Fedora) with `dnf`/`yum` or `apt-get` for bootstrap dependencies.

---

## What it does

| Capability | How |
|-----------|-----|
| Provision a new palace | Creates a Linux user, data dir, systemd unit, and logrotate config via `scripts/provision-palace.sh` |
| Update pserver binary | Downloads latest official tarball from `sdist.thepalace.app` for your arch and refreshes the template via `scripts/update-pserver.sh` |
| Start / stop / restart | Wraps `systemctl` for each `palman-*.service` unit |
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

- **Debian 11+** or **Ubuntu 22.04+**, or **RHEL 8+–style** hosts (AlmaLinux, Rocky Linux, CentOS Stream, etc.) with EPEL available when using RPM certbot packages
- Root access (the provisioning scripts create Linux users, write systemd units, etc.)
- `rsync` — needed by `--from-template` in the provision script (e.g. `apt install rsync` / `dnf install rsync`)
- For TLS media proxy: `nginx`, `certbot`, `python3-certbot-nginx` — the `palace-manager bootstrap` **`deps`** step installs these via `apt-get`, `dnf`, or `yum`

### RHEL / AlmaLinux / Rocky / CentOS Stream notes

- Bootstrap runs **`dnf install epel-release`** or **`yum install epel-release`** best-effort before installing certbot packages; if `python3-certbot-nginx` is still missing, enable EPEL manually for your OS version, then re-run **`deps`**.
- **`deploy/install.sh`** opens port **3000** with **firewalld** when it is active, or **ufw** when that is active (typical on Ubuntu).

#### SELinux (Enforcing)

nginx may be blocked from proxying to palace HTTP backends until you allow network connects from the HTTP daemon domain, for example:

```bash
sudo setsebool -P httpd_can_network_connect 1
```

Use tighter policy (custom port labels, more specific Booleans) if your site requires it.

#### Nginx media vhost path (upgrade from older releases)

Releases **before** this layout used `/etc/nginx/sites-enabled/100-palace-manager-media.conf`. Current releases use **`/etc/nginx/conf.d/100-palace-manager-media.conf`**, which stock **Debian and RHEL** nginx packages load by default. After upgrading, **remove the old file** if it exists, then run **Regen Now** in the UI (or reload nginx) so `server_name` is not defined twice.

---

## Building from source

Go 1.22+ required.

```bash
git clone <this repo>
cd palaceserver-js   # or wherever palace-manager lives
go build -o palace-manager ./cmd/palace-manager
```

Release builds stamp the version with a link flag (what GitHub Actions use):

```bash
go build -trimpath -ldflags "-s -w -X main.version=1.2.3" -o palace-manager ./cmd/palace-manager
```

---

## GitHub Releases (tag → binaries)

Pushing a **semantic version tag** `v*` triggers [`.github/workflows/release.yml`](.github/workflows/release.yml). It cross-compiles for Linux **amd64**, **arm64**, **armv7** (`GOARM=7`), and **386**, uploads one tarball per architecture plus **`SHA256SUMS`**, and creates or updates the matching [GitHub Release](https://docs.github.com/en/repositories/releasing-projects-on-github/about-releases).

This repository is a **single Go module** at the root (`go.mod` and `cmd/palace-manager` here).

### Publish a version

```bash
git tag -a v1.2.3 -m "Release 1.2.3"
git push origin v1.2.3
```

Use `vMAJOR.MINOR.PATCH` so the workflow matches the `v*` tag filter.

### Release asset names

Each tarball is named `palace-manager_<version>_linux_<arch>.tar.gz` (for example `palace-manager_1.2.3_linux_arm64.tar.gz`). It contains the `palace-manager` binary, [`deploy/install.sh`](deploy/install.sh), [`deploy/palace-manager.service`](deploy/palace-manager.service), and the helper scripts under the same layout [`deploy/push.sh`](deploy/push.sh) expects.

---

## Install from a GitHub release

You need the GitHub **`owner/repo`** slug (for example `myorg/palaceserver-js`). Prebuilt artifacts are **Linux only**.

### Option A — `make install-release` (from your laptop)

Detects the remote CPU with `ssh user@host uname -m`, downloads the matching asset **on the server** (so your Mac never needs the tarball), and runs `install.sh`.

```bash
make install-release VERSION=1.2.3 HOST=root@your.server RELEASE_REPO=myorg/palaceserver-js
```

`VERSION` may be `1.2.3` or `v1.2.3`. If you omit `RELEASE_REPO` and this clone’s `origin` is a `github.com` remote, the script tries to infer `owner/repo` from `git remote get-url origin`.

The remote host needs **`curl` or `wget`** and outbound HTTPS to `github.com`.

### Option B — `deploy/setup.sh` on the server (self-install)

Copy or pipe the script, then pass **`owner/repo`** and optionally a version (default: **latest** GitHub release):

```bash
curl -fsSL https://raw.githubusercontent.com/myorg/palaceserver-js/main/deploy/setup.sh | sudo bash -s -- myorg/palaceserver-js
# pinned version:
curl -fsSL https://raw.githubusercontent.com/myorg/palaceserver-js/main/deploy/setup.sh | sudo bash -s -- myorg/palaceserver-js v1.2.3
```

Equivalent without args:

```bash
export PALACE_MANAGER_GITHUB_REPO=myorg/palaceserver-js
curl -fsSL https://raw.githubusercontent.com/myorg/palaceserver-js/main/deploy/setup.sh | sudo bash
```

If the release includes **`SHA256SUMS`**, the script verifies the tarball when possible.

You can also download **`setup.sh`** from the same release page (it is attached to each release) and run `sudo bash setup.sh myorg/palaceserver-js` on the server so you do not depend on a fixed branch name in `raw.githubusercontent.com`.

### Local release tarball (same layout as CI)

```bash
make dist TAG=v1.2.3
# optional cross-compile:
GOOS=linux GOARCH=arm64 make dist TAG=v1.2.3
# linux/arm with armv7 filename (matches published armv7 asset):
GOOS=linux GOARCH=arm GOARM=7 ASSET_ARCH=armv7 make dist TAG=v1.2.3
```

Artifacts appear under `dist/`.

---

## Cross-compile for a remote Linux host

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
| `deps` | Installs `nginx`, `certbot`, `python3-certbot-nginx`, `rsync` via **`apt-get`**, **`dnf`**, or **`yum`** (best-effort `epel-release` on RPM family) |
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
- **`palace-manager` always writes the media vhost to** `/etc/nginx/conf.d/100-palace-manager-media.conf` **(fixed name)** so it stays the same regardless of hostname; **`server_name`** inside the file still comes from **`nginx.mediaHost`**.
- **`certDir` must contain the PEMs for `mediaHost`.** Defaults now set `certDir` to `/etc/letsencrypt/live/<mediaHost>/`, matching what Certbot creates when you request a certificate for that hostname.
- **`config.json` is updated only when the `config` bootstrap step runs** (after the earlier steps succeed). If **`cert`** fails (e.g. DNS `NXDOMAIN`), the wizard stops and the on-disk config can still have old placeholders — nginx regen then points at certs that were never issued. Fix DNS (or use **`edgeScheme`: `http`** for local HTTP-only testing), complete Host Setup through **`config`**, then use **Regen Now** again.
- **Certificate renewal:** Certbot typically ships a **systemd timer** (`certbot.timer`) when installed from distro packages. The bootstrap **`hook`** step installs a **deploy hook** that runs `systemctl reload nginx` after a successful renew so new certs are picked up without manual restarts.

---

## Provisioning a palace

### Via the web UI

1. Open the **Palaces** tab, click **+ New Palace**
2. Enter Linux username, Palace Server Name, SYSOP, ports (optional Palace Domain Name for directory listing), and quota
3. Click **Provision** — live output streams in the modal

### Via the API

```bash
curl -X POST http://127.0.0.1:3000/api/palaces \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"mypalace","serverName":"My Mansion","sysop":"Joe Sysop","tcpPort":9998,"httpPort":8080}'
```

Output streams as SSE. After provisioning:

1. Copy `pserver.pat` and `serverkey.txt` (if using a registered directory key) into `/home/mypalace/palace/`
2. Enable the service:
   ```bash
   systemctl enable --now palman-mypalace.service
   ```
3. nginx will pick up the new palace's `mediaserverurl.txt` within the next regen interval (default 2 min)

### Second (and further) palaces

Use unique TCP and HTTP ports for each. The cron job is not needed — palace-manager runs nginx regen internally on a timer.

```bash
# Palace 2: different ports
curl -X POST http://127.0.0.1:3000/api/palaces \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"anotherpalace","serverName":"Palace Two","sysop":"Owner","tcpPort":9999,"httpPort":8081}'
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

Both stream output. The **Updates** tab in the web UI does the same.

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
| `DELETE` | `/api/palaces/:name` | Stop + disable + drop from registry; keeps `palman-*.service` (unregister-only) and appends a recovery snapshot to `/etc/palace-manager/unregistered-palaces.json`. `?purge=true` also removes the Linux user, deletes the unit file, and removes the home directory (and drops any snapshot). |
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

palace-manager needs root because provisioning creates Linux users, writes systemd units, and bootstrap installs packages (`apt-get`, `dnf`, or `yum`). The service unit (`deploy/palace-manager.service`) runs as root by default.

For a least-privilege setup, run `palace-manager bootstrap` once as root, then create `/etc/sudoers.d/palace-manager` granting the service user only the needed commands (`adduser`/`useradd`, `systemctl`, `install`, etc.).

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
| `/etc/nginx/conf.d/100-palace-manager-media.conf` | Generated media reverse-proxy vhost (`gen-media-nginx.sh`) |
| `/etc/systemd/system/palman-<name>.service` | Per-palace systemd unit |
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

---

## Responsive UI QA checklist

When changing dashboard UI, run this quick check before shipping:

- Validate widths: 320, 375, 414, 768, 1024, 1280+.
- Confirm header/nav works in both mobile menu mode and desktop horizontal mode.
- Verify no primary flow or modal requires horizontal page scrolling at 320px.
- Check `Palaces`, `Users`, `Audit log`, and `StaffPass` tables in mobile card mode and desktop table mode.
- Open/close each major modal with both mouse/touch and `Escape`.
- Confirm safe-area spacing on iOS devices (footer, modal edges, and header).
- Confirm reduced-motion mode still works (no required animation-only affordance).
