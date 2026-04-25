#!/usr/bin/env bash
# gen-media-nginx.sh
#
# Scans palace instance directories for mediaserverurl.txt + internalmediaserverurl.txt
# (written by pserver), then generates an nginx vhost with one proxy location per instance.
#
# Layout modes:
#   --palaces-dir DIR     Each DIR/<name>/mediaserverurl.txt (original behavior)
#   --scan-homes          Discover under each Unix home:
#                           /home/*/palace/mediaserverurl.txt   (flat, one palace per user)
#                           /home/*/palace/*/mediaserverurl.txt (nested extra tier)
#
# External URL matching:
#   --match-scheme https|http|both   Which scheme(s) in mediaserverurl.txt to accept (default: both)
#
# Nginx edge:
#   --edge-scheme https|http|dual    https = TLS + port 80→HTTPS redirect (default)
#                                    http  = proxy on port 80 only (lab / no TLS)
#                                    dual  = TLS + same paths on port 80 (HTTP or HTTPS clients)
#
# Usage:
#   ./gen-media-nginx.sh [--palaces-dir /srv/palace] [--dry-run] [--reload]
#   ./gen-media-nginx.sh --scan-homes --reload
#
# Requirements:
#   - nginx installed
#   - For --edge-scheme https|dual: TLS cert for MEDIA_HOST (see CERT_DIR)
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
PALACES_DIR="/srv/palace"
SCAN_HOMES=false
HOMES_PREFIX="/home"
NGINX_CONF="/etc/nginx/conf.d/100-palace-manager-media.conf"
MEDIA_HOST="media.thepalace.app"
CERT_DIR="/etc/letsencrypt/live/thepalace.app"
MATCH_SCHEME="both"
EDGE_SCHEME="https"
DRY_RUN=false
RELOAD=false

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --palaces-dir) PALACES_DIR="$2"; shift 2 ;;
    --scan-homes)  SCAN_HOMES=true; shift ;;
    --homes-prefix) HOMES_PREFIX="$2"; shift 2 ;;
    --nginx-conf)  NGINX_CONF="$2";  shift 2 ;;
    --media-host)  MEDIA_HOST="$2";  shift 2 ;;
    --cert-dir)    CERT_DIR="$2";    shift 2 ;;
    --match-scheme) MATCH_SCHEME="$2"; shift 2 ;;
    --edge-scheme) EDGE_SCHEME="$2"; shift 2 ;;
    --dry-run)     DRY_RUN=true;     shift   ;;
    --reload)      RELOAD=true;      shift   ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

case "$MATCH_SCHEME" in
  https|http|both) ;;
  *) echo "--match-scheme must be https, http, or both" >&2; exit 1 ;;
esac

case "$EDGE_SCHEME" in
  https|http|dual) ;;
  *) echo "--edge-scheme must be https, http, or dual" >&2; exit 1 ;;
esac

# ---------------------------------------------------------------------------
# Collect mediaserverurl.txt paths
# ---------------------------------------------------------------------------
declare -a MEDIA_FILES=()

if $SCAN_HOMES; then
  shopt -s nullglob
  declare -A media_seen
  collect() {
    local f
    for f in "$@"; do
      [[ -f "$f" ]] || continue
      [[ -n "${media_seen[$f]+x}" ]] && continue
      media_seen[$f]=1
      MEDIA_FILES+=("$f")
    done
  }
  collect "${HOMES_PREFIX}"/*/palace/mediaserverurl.txt
  collect "${HOMES_PREFIX}"/*/palace/*/mediaserverurl.txt
  shopt -u nullglob
else
  shopt -s nullglob
  for f in "${PALACES_DIR}"/*/mediaserverurl.txt; do
    [[ -f "$f" ]] && MEDIA_FILES+=("$f")
  done
  shopt -u nullglob
fi

# ---------------------------------------------------------------------------
# Scan palace directories → locations[path] = upstream
# ---------------------------------------------------------------------------
declare -A locations  # path → internal_upstream
LOCATION_COUNT=0

prefix_ok() {
  local ext="$1" want="$2"
  case "$want" in
    https) [[ "$ext" == "https://${MEDIA_HOST}/"* ]] ;;
    http)  [[ "$ext" == "http://${MEDIA_HOST}/"* ]] ;;
    both)  [[ "$ext" == "https://${MEDIA_HOST}/"* || "$ext" == "http://${MEDIA_HOST}/"* ]] ;;
  esac
}

extract_path_segment() {
  local external_url="$1"
  local seg=""
  if [[ "$external_url" == "https://${MEDIA_HOST}/"* ]]; then
    seg="${external_url#https://${MEDIA_HOST}/}"
  elif [[ "$external_url" == "http://${MEDIA_HOST}/"* ]]; then
    seg="${external_url#http://${MEDIA_HOST}/}"
  else
    echo ""
    return
  fi
  seg="${seg%/}"
  echo "$seg"
}

for media_file in "${MEDIA_FILES[@]}"; do
  instance_dir="$(dirname "$media_file")"
  internal_file="$instance_dir/internalmediaserverurl.txt"

  [[ -f "$internal_file" ]] || continue

  external_url="$(tr -d '[:space:]' < "$media_file")"
  internal_url="$(tr -d '[:space:]' < "$internal_file")"

  [[ -z "$external_url" ]] && continue
  [[ -z "$internal_url" ]] && continue

  if ! prefix_ok "$external_url" "$MATCH_SCHEME"; then
    echo "SKIP ${media_file}: external URL $external_url does not match ${MATCH_SCHEME}://${MEDIA_HOST}/…" >&2
    continue
  fi

  path_segment="$(extract_path_segment "$external_url")"

  if [[ -z "$path_segment" ]]; then
    echo "SKIP ${media_file}: no path segment in external URL" >&2
    continue
  fi

  upstream="${internal_url%/}/"

  echo "FOUND ${media_file}: /$path_segment/ → $upstream"
  locations["/$path_segment/"]="$upstream"
  LOCATION_COUNT=$((LOCATION_COUNT + 1))
done

SCAN_DESC=$(
  if $SCAN_HOMES; then
    echo "${HOMES_PREFIX}/*/palace/… and ${HOMES_PREFIX}/*/palace/*/…"
  else
    echo "${PALACES_DIR}/*"
  fi
)

if [[ $LOCATION_COUNT -eq 0 ]]; then
  echo "No matching palace instances found under ${SCAN_DESC} — nothing to generate." >&2
  exit 0
fi

# ---------------------------------------------------------------------------
# Shared location blocks (nginx config fragment)
# ---------------------------------------------------------------------------
LOCATION_BLOCKS=""
mapfile -t _sorted_paths < <(printf '%s\n' "${!locations[@]}" | LC_ALL=C sort)
for path_segment in "${_sorted_paths[@]}"; do
  upstream="${locations[$path_segment]}"
  LOCATION_BLOCKS+="$(cat <<NGINX_LOC
    # Palace instance: ${path_segment}
    location ${path_segment} {
        proxy_pass         ${upstream};
        proxy_set_header   Host \$host;
        proxy_set_header   X-Real-IP \$remote_addr;
        proxy_set_header   X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto \$scheme;
        proxy_read_timeout 30s;
        proxy_buffering    off;
    }

NGINX_LOC
)"
done

# ---------------------------------------------------------------------------
# Generate nginx config
# ---------------------------------------------------------------------------
conf="# Auto-generated by gen-media-nginx.sh — do not edit by hand.
# Re-run the script to regenerate after adding or changing palace instances.
# Generated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
#
# scan: ${SCAN_DESC}
# match-scheme: ${MATCH_SCHEME}  edge-scheme: ${EDGE_SCHEME}
#

"

if [[ "$EDGE_SCHEME" == "https" || "$EDGE_SCHEME" == "dual" ]]; then
  conf+="$(cat <<NGINX_TLS
server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name ${MEDIA_HOST};

    ssl_certificate     ${CERT_DIR}/fullchain.pem;
    ssl_certificate_key ${CERT_DIR}/privkey.pem;
    include             /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam         /etc/letsencrypt/ssl-dhparams.pem;

    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options SAMEORIGIN;

    location / {
        return 404;
    }

${LOCATION_BLOCKS}}
NGINX_TLS
)"
fi

if [[ "$EDGE_SCHEME" == "https" ]]; then
  conf+="$(cat <<NGINX_REDIRECT


server {
    listen 80;
    listen [::]:80;
    server_name ${MEDIA_HOST};
    return 301 https://\$host\$request_uri;
}
NGINX_REDIRECT
)"
fi

if [[ "$EDGE_SCHEME" == "http" || "$EDGE_SCHEME" == "dual" ]]; then
  conf+="$(cat <<NGINX_HTTP


server {
    listen 80;
    listen [::]:80;
    server_name ${MEDIA_HOST};

    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options SAMEORIGIN;

    location / {
        return 404;
    }

${LOCATION_BLOCKS}}
NGINX_HTTP
)"
fi

# ---------------------------------------------------------------------------
# Output / apply
# ---------------------------------------------------------------------------
if $DRY_RUN; then
  echo ""
  echo "=== DRY RUN: would write to $NGINX_CONF ==="
  echo "$conf"
else
  echo "$conf" > "$NGINX_CONF"
  echo "Written: $NGINX_CONF"

  if nginx -t 2>&1; then
    echo "nginx config OK"
    if $RELOAD; then
      systemctl reload nginx
      echo "nginx reloaded"
    else
      echo "Run with --reload to apply, or: systemctl reload nginx"
    fi
  else
    echo "ERROR: nginx config test failed — reverting" >&2
    rm -f "$NGINX_CONF"
    exit 1
  fi
fi
