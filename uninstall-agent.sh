#!/bin/bash
# InfraCanvas Agent Uninstaller
# Usage: curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash

set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/infracanvas"
SERVICE_NAME="infracanvas-agent"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
LOG_FILE="/var/log/infracanvas-agent.log"
PID_FILE="/var/run/infracanvas-agent.pid"

# Stop and disable systemd service
if command -v systemctl &>/dev/null; then
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    info "Stopping $SERVICE_NAME..."
    systemctl stop "$SERVICE_NAME"
  fi
  if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
    info "Disabling $SERVICE_NAME..."
    systemctl disable "$SERVICE_NAME"
  fi
  if [[ -f "$SERVICE_FILE" ]]; then
    info "Removing systemd service..."
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
  fi
else
  # No systemd — kill background process
  if [[ -f "$PID_FILE" ]]; then
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
      info "Stopping agent (PID $PID)..."
      kill "$PID"
    fi
    rm -f "$PID_FILE"
  fi
fi

# Remove binary
if [[ -f "$INSTALL_DIR/infracanvas" ]]; then
  info "Removing binary..."
  rm -f "$INSTALL_DIR/infracanvas"
fi

# Remove config and logs
info "Removing config and logs..."
rm -rf "$CONFIG_DIR"
rm -f "$LOG_FILE"

echo ""
echo -e "${GREEN}✓ InfraCanvas agent uninstalled${NC}"
