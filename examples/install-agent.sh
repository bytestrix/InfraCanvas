#!/bin/bash
set -e

# Installation script for infracanvas agent

echo "Installing infracanvas agent..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root (use sudo)"
    exit 1
fi

# Variables
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/infracanvas"
SERVICE_FILE="/etc/systemd/system/infracanvas-agent.service"
BINARY_URL="${BINARY_URL:-https://github.com/yourusername/infracanvas/releases/latest/download/infracanvas}"

# Create config directory
echo "Creating configuration directory..."
mkdir -p "$CONFIG_DIR"

# Download binary (or copy from local build)
if [ -f "./bin/infracanvas" ]; then
    echo "Installing from local build..."
    cp ./bin/infracanvas "$INSTALL_DIR/infracanvas"
else
    echo "Downloading infracanvas binary..."
    curl -fsSL "$BINARY_URL" -o "$INSTALL_DIR/infracanvas"
fi

# Make binary executable
chmod +x "$INSTALL_DIR/infracanvas"

# Create default config if it doesn't exist
if [ ! -f "$CONFIG_DIR/agent.yaml" ]; then
    echo "Creating default configuration..."
    cat > "$CONFIG_DIR/agent.yaml" <<EOF
# Agent Configuration
backend_url: "http://localhost:8080"
auth_token: "CHANGE_ME"
tls_insecure: false

host_interval: 10
docker_interval: 15
kubernetes_interval: 20
heartbeat_interval: 30

scope:
  - host
  - docker
  - kubernetes

agent_id: ""
agent_name: "$(hostname)"

enable_redaction: true
enable_watchers: true
EOF
    echo "Configuration created at $CONFIG_DIR/agent.yaml"
    echo "Please edit this file and set your backend_url and auth_token"
fi

# Install systemd service
if command -v systemctl &> /dev/null; then
    echo "Installing systemd service..."
    
    if [ -f "./examples/infracanvas-agent.service" ]; then
        cp ./examples/infracanvas-agent.service "$SERVICE_FILE"
    else
        cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Infrastructure Discovery Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=$INSTALL_DIR/infracanvas agent --config $CONFIG_DIR/agent.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    fi
    
    # Reload systemd
    systemctl daemon-reload
    
    echo "Systemd service installed"
    echo ""
    echo "To start the agent:"
    echo "  sudo systemctl start infracanvas-agent"
    echo ""
    echo "To enable on boot:"
    echo "  sudo systemctl enable infracanvas-agent"
    echo ""
    echo "To view logs:"
    echo "  sudo journalctl -u infracanvas-agent -f"
else
    echo "systemd not found, skipping service installation"
fi

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit $CONFIG_DIR/agent.yaml and configure your backend URL and auth token"
echo "2. Start the agent: sudo systemctl start infracanvas-agent"
echo "3. Check status: sudo systemctl status infracanvas-agent"
