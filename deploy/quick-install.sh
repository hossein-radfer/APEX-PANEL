#!/usr/bin/env bash
# ApexPanel: true one-command bootstrap for a fresh Ubuntu server.
#
# Unlike install.sh (which assumes a pre-built ./mwp binary already
# exists next to it), this script starts from nothing: it clones the
# repository, installs Go/Node.js if missing, builds the frontend, builds
# the Go binary (which embeds that frontend build via ui/assets.go's
# //go:embed directive -- see deploy/README.md for why the build order
# matters), and then hands off to install.sh for the actual systemd
# install. Intended to be run exactly like this, as root, on a fresh
# Ubuntu 22.04/24.04 server:
#
#   curl -fsSL https://raw.githubusercontent.com/hossein-radfer/APEX-PANEL/main/deploy/quick-install.sh | sudo bash
#
# Every install.sh option (--license-key, --admin-username, --db-dialect,
# etc.) can be appended after a `--` separator and is passed straight
# through, e.g.:
#
#   curl -fsSL .../quick-install.sh | sudo bash -s -- --license-key MWP-XXXX-XXXX-XXXX-XXXX
#
# This script does NOT set up nginx/TLS -- see install.sh's own header
# comment for why (ApexPanel is typically accessed directly by IP:port or
# through whatever reverse proxy your own infrastructure already uses).

set -euo pipefail

REPO_URL="https://github.com/hossein-radfer/APEX-PANEL.git"
CLONE_DIR="/opt/apexpanel-src"
GO_VERSION="1.24.3"
NODE_MAJOR="20"

if [[ "$EUID" -ne 0 ]]; then
  echo "ERROR: run this script as root (sudo bash quick-install.sh ...)"
  exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
  echo "ERROR: this script is for Debian/Ubuntu (apt-get) hosts only."
  exit 1
fi

echo "==> Installing base build dependencies (git, curl, build-essential)"
apt-get update -qq
apt-get install -y -qq git curl build-essential >/dev/null

if ! command -v go >/dev/null 2>&1; then
  echo "==> Installing Go $GO_VERSION"
  ARCH="$(dpkg --print-architecture)"
  case "$ARCH" in
    amd64) GO_ARCH="amd64" ;;
    arm64) GO_ARCH="arm64" ;;
    *) echo "ERROR: unsupported architecture for automatic Go install: $ARCH -- install Go manually and re-run."; exit 1 ;;
  esac
  curl -fsSLo /tmp/go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
else
  echo "==> Go already installed ($(go version))"
fi

if ! command -v node >/dev/null 2>&1; then
  echo "==> Installing Node.js $NODE_MAJOR.x"
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash - >/dev/null 2>&1
  apt-get install -y -qq nodejs >/dev/null
else
  echo "==> Node.js already installed ($(node --version))"
fi

echo "==> Cloning repository into $CLONE_DIR"
rm -rf "$CLONE_DIR"
git clone --depth 1 "$REPO_URL" "$CLONE_DIR"
cd "$CLONE_DIR"

echo "==> Building the frontend (ui/dist, embedded into the Go binary)"
cd ui
npm install --silent
npm run build --silent
cd ..

echo "==> Building the Go binary (this embeds ui/dist via ui/assets.go)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mwp -ldflags="-s -w" ./api/cmd

echo "==> Handing off to install.sh for the systemd install"
chmod +x deploy/install.sh
exec deploy/install.sh --binary ./mwp "$@"
