#!/usr/bin/env bash
# InfraCanvas uninstaller
# Usage:
#   curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash

set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*"; }

run_priv() {
  if [[ $EUID -eq 0 ]]; then "$@"; else sudo "$@"; fi
}

# Returns true if a systemd unit file belongs to the InfraCanvas Cloud (SaaS)
# agent — identified by the presence of INFRACANVAS_TOKEN or INFRACANVAS_BACKEND
# env vars, which are only written by the cloud installer, not the OSS installer.
is_saas_unit() {
  local unit_file="$1"
  [[ -f "$unit_file" ]] && grep -qE 'INFRACANVAS_TOKEN|INFRACANVAS_BACKEND' "$unit_file" 2>/dev/null
}

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/infracanvas"
# Only remove the OSS service name ("infracanvas").
# "infracanvas-agent" was historically a legacy OSS name but is now also used
# by InfraCanvas Cloud — we skip it here to avoid breaking cloud installs.
# The cloud installer migrates that name to "ic-agent" going forward.
SERVICES=("infracanvas")

if command -v systemctl >/dev/null; then
  for svc in "${SERVICES[@]}"; do
    UNIT="/etc/systemd/system/${svc}.service"
    if [[ -f "$UNIT" ]] || systemctl list-unit-files 2>/dev/null | grep -q "^${svc}\.service"; then
      # Safety check: skip if this unit is actually an InfraCanvas Cloud agent.
      if is_saas_unit "$UNIT"; then
        warn "Skipping $svc — looks like an InfraCanvas Cloud agent (contains INFRACANVAS_TOKEN)."
        warn "To uninstall the cloud agent, use the InfraCanvas Cloud dashboard instead."
        continue
      fi
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

# Bundled cloudflared cache (~30 MB) for the user the service ran as.
# Best-effort: SUDO_USER is set when invoked via `sudo`; otherwise scan /home.
CACHE_USERS=()
[[ -n "${SUDO_USER:-}" ]] && CACHE_USERS+=("$SUDO_USER")
for h in /home/*/; do
  u="$(basename "$h")"
  [[ "$u" == "${SUDO_USER:-}" ]] && continue
  CACHE_USERS+=("$u")
done
[[ -d "/root/.cache/infracanvas" ]] && CACHE_USERS+=("root")

for u in "${CACHE_USERS[@]}"; do
  if [[ "$u" == "root" ]]; then
    cache_dir="/root/.cache/infracanvas"
  else
    cache_dir="/home/${u}/.cache/infracanvas"
  fi
  if [[ -d "$cache_dir" ]]; then
    info "Removing cloudflared cache for $u: $cache_dir"
    run_priv rm -rf "$cache_dir"
  fi
done

echo ""
echo -e "${GREEN}✓ InfraCanvas uninstalled${NC}"
