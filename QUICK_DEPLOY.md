# Quick Deploy Commands

## Option 1: Automated Script (Recommended)

1. Edit `deploy-agent-to-azure.sh` and set your local IP:
   ```bash
   # Find your local IP first
   ip addr show | grep "inet " | grep -v 127.0.0.1
   
   # Edit the script and replace YOUR_LOCAL_IP
   nano deploy-agent-to-azure.sh
   ```

2. Run the script:
   ```bash
   ./deploy-agent-to-azure.sh
   ```

## Option 2: Manual Commands

```bash
# 1. Build for Linux
GOOS=linux GOARCH=amd64 go build -o bin/infracanvas-linux ./cmd/infracanvas

# 2. Copy to VM
scp bin/infracanvas-linux azureuser@4.240.90.150:/tmp/infracanvas

# 3. SSH and install
ssh azureuser@4.240.90.150
sudo mkdir -p /etc/infracanvas /usr/local/bin
sudo mv /tmp/infracanvas /usr/local/bin/infracanvas
sudo chmod +x /usr/local/bin/infracanvas

# 4. Create config (replace YOUR_LOCAL_IP)
sudo tee /etc/infracanvas/agent.yaml > /dev/null <<EOF
backend_url: "ws://YOUR_LOCAL_IP:8080"
auth_token: "dev-token"
tls_insecure: true
host_interval: 10
docker_interval: 15
kubernetes_interval: 20
heartbeat_interval: 30
scope:
  - host
  - docker
agent_id: ""
agent_name: "azure-vm-agent"
enable_redaction: true
enable_watchers: true
EOF

# 5. Test run
sudo /usr/local/bin/infracanvas agent --config /etc/infracanvas/agent.yaml
```

## Verify Connection

On your local machine, check server logs for agent connection:
```bash
# Your server should show agent connection messages
```

On the VM, check agent is running:
```bash
ssh azureuser@4.240.90.150 'ps aux | grep infracanvas'
```

## Important: Network Setup

Your Azure VM needs to reach your local machine. Options:

1. **Public IP**: Expose your local server on public IP (use with caution)
2. **VPN**: Connect VM to your network via VPN
3. **Reverse Tunnel**: Use SSH reverse tunnel
4. **Cloud Server**: Deploy infracanvas-server to a cloud instance

### Quick SSH Tunnel (for testing)
```bash
# On your local machine, create reverse tunnel
ssh -R 8080:localhost:8080 azureuser@4.240.90.150

# Then on VM, use localhost:8080 in config
backend_url: "ws://localhost:8080"
```
