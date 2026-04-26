#!/usr/bin/env bash
# InfraCanvas uninstaller
# Usage:
#   curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash

set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

run_priv() {
  if [[ $EUID -eq 0 ]]; then "$@"; else sudo "$@"; fi
}

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/infracanvas"
SERVICES=("infracanvas" "infracanvas-agent")  # current + legacy

if command -v systemctl >/dev/null; then
  for svc in "${SERVICES[@]}"; do
    UNIT="/etc/systemd/system/${svc}.service"
    if [[ -f "$UNIT" ]] || systemctl list-unit-files 2>/dev/null | grep -q "^${svc}\.service"; then
      if systemctl is-active --quiet "$svc" 2>/dev/null; then
        info "Stopping $svc..."
        run_priv systemctl stop "$svc" || true
      fi
      if systemctl is-enabled --quiet "$svc" 2>/dev/null; then
        info "Disabling $svc..."
        run_priv systemctl disable "$svc" || true
      fi
      if [[ -f "$UNIT" ]]; then
        info "Removing $UNIT"
        run_priv rm -f "$UNIT"
      fi
    fi
  done
  run_priv systemctl daemon-reload
fi

if [[ -f "$INSTALL_DIR/infracanvas" ]]; then
  info "Removing binary..."
  run_priv rm -f "$INSTALL_DIR/infracanvas"
fi

if [[ -d "$CONFIG_DIR" ]]; then
  info "Removing config..."
  run_priv rm -rf "$CONFIG_DIR"
fi

# Old log files from pre-systemd installs
run_priv rm -f /var/log/infracanvas-agent.log /var/run/infracanvas-agent.pid 2>/dev/null || true

echo ""
echo -e "${GREEN}✓ InfraCanvas uninstalled${NC}"
