#!/bin/bash
set -e

# Uninstallation script for infracanvas agent

echo "Uninstalling infracanvas agent..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root (use sudo)"
    exit 1
fi

# Variables
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/infracanvas"
SERVICE_FILE="/etc/systemd/system/infracanvas-agent.service"
BINARY_PATH="$INSTALL_DIR/infracanvas"

# Stop and disable service if systemd is available
if command -v systemctl &> /dev/null; then
    if systemctl is-active --quiet infracanvas-agent; then
        echo "Stopping infracanvas-agent service..."
        systemctl stop infracanvas-agent
    fi
    
    if systemctl is-enabled --quiet infracanvas-agent 2>/dev/null; then
        echo "Disabling infracanvas-agent service..."
        systemctl disable infracanvas-agent
    fi
    
    if [ -f "$SERVICE_FILE" ]; then
        echo "Removing systemd service file..."
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi
fi

# Remove binary
if [ -f "$BINARY_PATH" ]; then
    echo "Removing binary..."
    rm -f "$BINARY_PATH"
fi

# Ask about config removal
echo ""
read -p "Remove configuration directory $CONFIG_DIR? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Removing configuration directory..."
    rm -rf "$CONFIG_DIR"
else
    echo "Keeping configuration directory at $CONFIG_DIR"
fi

echo ""
echo "Uninstallation complete!"
