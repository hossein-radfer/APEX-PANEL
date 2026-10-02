#!/usr/bin/env bash
# MWPanel: one-shot install script for a Linux server, wiring it up as a
# systemd service with a real (or generated-default) admin account.
#
# What this does, end to end:
#   1. Creates /opt/mwp, /opt/mwp/data, /opt/mwp/config
#   2. Installs the pre-built binary (must already exist at ./mwp next to
#      this script, or pass --binary PATH)
#   3. Writes /opt/mwp/config/.env from api/config/.env.example with the
#      values you pass filled in (DB dialect, license key, admin
#      credentials)
#   4. Installs and enables the systemd unit (deploy/mwp.service)
#   5. Starts the service
#
# This script does NOT set up nginx/TLS for MWPanel itself -- MWPanel is
# typically accessed directly by IP:port or through whatever reverse
# proxy your own infrastructure already uses. license-panel's own
# deploy/install.sh also runs plain HTTP with no reverse proxy (nginx/
# certbot were removed from that setup too -- see license-panel/deploy/
# README.md), so no TLS is involved on either side of this connection by
# default.
#
# Usage:
#   sudo ./install.sh \
#     --license-key MWP-XXXX-XXXX-XXXX-XXXX \
#     [--binary ./mwp] \
#     [--admin-username ApexPanel] [--admin-password change-me] \
#     [--db-dialect sqlite] \
#     [--license-server-url http://203.0.113.10:3131] [--dev-mode]
#
# --license-public-key is normally NOT needed: if omitted, MWPanel
# fetches the license server's Ed25519 public key automatically on first
# activation (from its unauthenticated /api/v1/license/public-key
# endpoint) and caches it, so there's nothing to copy/paste by hand
# anymore. Only pass --license-public-key if you want to pin a specific
# key without trusting the auto-fetch.
#
# If you don't have a license key yet, omit --license-key entirely -- the
# panel will start locked to the activation screen, where you (or your
# customer) can activate it via the UI instead (this is the normal flow;
# --license-key is only a first-boot convenience so you don't have to
# click through the UI yourself during a scripted install).

set -euo pipefail

BINARY="./mwp"
DB_DIALECT="sqlite"
ADMIN_USERNAME="ApexPanel"
ADMIN_PASSWORD="change-me"
LICENSE_KEY=""
LICENSE_PUBLIC_KEY=""
LICENSE_SERVER_URL=""
DEV_MODE=false

usage() {
  cat <<'EOF'
Usage: sudo ./install.sh [options]
  --binary PATH                Path to the pre-built mwp binary (default: ./mwp)
  --admin-username NAME        Default admin username (default: ApexPanel)
  --admin-password PASS        Default admin password (default: change-me)
  --db-dialect sqlite|postgres Database dialect (default: sqlite)
  --license-key KEY            License key to activate with on first boot (optional)
  --license-public-key KEY     Ed25519 public key from license-panel's keygen (optional)
  --license-server-url URL     Dev-only override for the license server address
  --dev-mode                   Set MODE=development (required for --license-server-url to take effect)
EOF
  exit 1
}

# require_value NAME "$2" "$#" -- fails with usage() if a --flag that
# takes a value was given with nothing after it, instead of crashing
# with a raw "unbound variable" error from `set -u`.
require_value() {
  local flag="$1" value="${2-}" remaining="$3"
  if [[ "$remaining" -lt 2 || "$value" == --* ]]; then
    echo "ERROR: $flag requires a value"
    usage
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary) require_value "$1" "${2-}" "$#"; BINARY="$2"; shift 2 ;;
    --admin-username) require_value "$1" "${2-}" "$#"; ADMIN_USERNAME="$2"; shift 2 ;;
    --admin-password) require_value "$1" "${2-}" "$#"; ADMIN_PASSWORD="$2"; shift 2 ;;
    --db-dialect) require_value "$1" "${2-}" "$#"; DB_DIALECT="$2"; shift 2 ;;
    --license-key) require_value "$1" "${2-}" "$#"; LICENSE_KEY="$2"; shift 2 ;;
    --license-public-key) require_value "$1" "${2-}" "$#"; LICENSE_PUBLIC_KEY="$2"; shift 2 ;;
    --license-server-url) require_value "$1" "${2-}" "$#"; LICENSE_SERVER_URL="$2"; shift 2 ;;
    --dev-mode) DEV_MODE=true; shift ;;
    -h|--help) usage ;;
    *) echo "Unknown argument: $1"; usage ;;
  esac
done

if [[ "$EUID" -ne 0 ]]; then
  echo "ERROR: run this script as root (sudo $0 ...)"
  exit 1
fi

if [[ ! -f "$BINARY" ]]; then
  echo "ERROR: binary not found at $BINARY -- build it first (see deploy/README.md) or pass --binary PATH"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR=/opt/mwp
DATA_DIR=/opt/mwp/data
CONFIG_DIR=/opt/mwp/config
ENV_FILE=/opt/mwp/config/.env

echo "==> Creating $INSTALL_DIR, $DATA_DIR, $CONFIG_DIR"
mkdir -p "$DATA_DIR" "$CONFIG_DIR"

echo "==> Installing binary to /usr/local/bin/mwp"
install -m 755 "$BINARY" /usr/local/bin/mwp

if [[ -f "$ENV_FILE" ]]; then
  echo "==> $ENV_FILE already exists -- leaving it untouched"
else
  echo "==> Writing $ENV_FILE"
  MODE_VALUE="production"
  if [[ "$DEV_MODE" == "true" ]]; then
    MODE_VALUE="development"
  fi
  # DB_NAME/DB_HOST/DB_PORT/DB_USERNAME/DB_PASSWORD are Postgres-only --
  # GetDBConfig() (api/config/config.go) derives sqlite's own file path
  # from DataDirPath automatically (<DATA_DIR>/mwp.db) UNLESS DB_NAME is
  # explicitly set, in which case that value wins even in sqlite mode and
  # is treated as a path RELATIVE TO THE PROCESS'S WORKING DIRECTORY
  # (/opt/mwp, per mwp.service), not DATA_DIR -- writing a bare "mwp_db"
  # here previously put the sqlite file somewhere other than
  # /opt/mwp/data, silently diverging from where backups/restores expect
  # it. Only write the Postgres block when --db-dialect postgres was
  # actually requested.
  DB_BLOCK=""
  if [[ "$DB_DIALECT" == "postgres" ]]; then
    DB_BLOCK="DB_HOST=127.0.0.1
DB_PORT=5432
DB_USERNAME=postgres
DB_PASSWORD=postgres
DB_NAME=mwp_db"
  fi

  cat > "$ENV_FILE" <<EOF
MODE=$MODE_VALUE
SERVER_HOST=0.0.0.0
SERVER_PORT=3000
TRAFFIC_JOB_INTERVAL=120

CONSOLE_LOG_FORMAT=plain

DB_DIALECT=$DB_DIALECT
$DB_BLOCK

ADMIN_USERNAME=$ADMIN_USERNAME
ADMIN_PASSWORD=$ADMIN_PASSWORD

AUTH_ACCESS_TOKEN_TTL=900
AUTH_REFRESH_TOKEN_TTL=86400

TELEGRAM_BOT_ENABLED=false

LICENSE_KEY=$LICENSE_KEY
LICENSE_PUBLIC_KEY=$LICENSE_PUBLIC_KEY
LICENSE_CHECK_INTERVAL_SECONDS=3600
LICENSE_SERVER_URL=$LICENSE_SERVER_URL
EOF
  chmod 600 "$ENV_FILE"
fi

echo "==> Installing systemd unit"
cp "$SCRIPT_DIR/mwp.service" /etc/systemd/system/mwp.service
systemctl daemon-reload
systemctl enable mwp

echo "==> Starting mwp"
systemctl restart mwp
sleep 1
systemctl status mwp --no-pager -l || true

echo ""
echo "============================================================"
echo " Done. MWPanel should now be reachable at:"
echo "   http://<server-ip>:3000"
echo ""
echo " Default admin credentials: $ADMIN_USERNAME / $ADMIN_PASSWORD"
echo " (only takes effect on a genuinely empty Admin table -- change"
echo " it from inside the panel after first login)"
echo ""
if [[ -z "$LICENSE_KEY" ]]; then
  echo " No license key was provided -- the panel will show the"
  echo " activation screen. Activate it from the web UI, or edit"
  echo " $ENV_FILE and restart (systemctl restart mwp)."
  echo ""
fi
echo " Useful commands:"
echo "   systemctl status mwp"
echo "   journalctl -u mwp -f"
echo "   tail -f $DATA_DIR/logs/mwp.log"
echo "   mwp menu"
echo "============================================================"
