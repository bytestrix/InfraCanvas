# Deploy InfraCanvas Agent to Azure VM

## Quick Setup Guide

### Prerequisites
- Your frontend is running locally
- Your infracanvas-server is running on `localhost:8080`
- SSH access to Azure VM: `azureuser@4.240.90.150`

### Step 1: Find Your Local IP Address

You need your local machine's IP that the Azure VM can reach:

```bash
# On Linux
ip addr show | grep "inet " | grep -v 127.0.0.1

# Or check your router/network settings
```

### Step 2: Build the Agent for Linux

```bash
GOOS=linux GOARCH=amd64 go build -o bin/infracanvas-linux ./cmd/infracanvas
```

### Step 3: Copy Binary to VM

```bash
scp bin/infracanvas-linux azureuser@4.240.90.150:/tmp/infracanvas
```

### Step 4: Create Config File

Create a config file with your local IP:

```bash
cat > /tmp/agent-config.yaml <<EOF
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
```

Replace `YOUR_LOCAL_IP` with your actual local IP address.

### Step 5: Copy Config to VM

```bash
scp /tmp/agent-config.yaml azureuser@4.240.90.150:/tmp/agent-config.yaml
```

### Step 6: Install on VM

SSH into the VM and run:

```bash
ssh azureuser@4.240.90.150

# On the VM:
sudo mkdir -p /etc/infracanvas /usr/local/bin
sudo mv /tmp/infracanvas /usr/local/bin/infracanvas
sudo mv /tmp/agent-config.yaml /etc/infracanvas/agent.yaml
sudo chmod +x /usr/local/bin/infracanvas
```

### Step 7: Test the Agent

Run the agent manually first to test:

```bash
sudo /usr/local/bin/infracanvas agent --config /etc/infracanvas/agent.yaml
```

You should see output indicating it's connecting to your server.

### Step 8: Set Up as Service (Optional)

If the test works, set it up as a systemd service:

```bash
# Copy service file
scp examples/infracanvas-agent.service azureuser@4.240.90.150:/tmp/

# On the VM:
sudo mv /tmp/infracanvas-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl start infracanvas-agent
sudo systemctl enable infracanvas-agent
```

### Troubleshooting

Check agent logs:
```bash
sudo journalctl -u infracanvas-agent -f
```

Check agent status:
```bash
sudo systemctl status infracanvas-agent
```

Test connectivity from VM to your local server:
```bash
curl http://YOUR_LOCAL_IP:8080/api/health
```

## Important Notes

1. **Firewall**: Make sure your local firewall allows incoming connections on port 8080
2. **Network**: Your Azure VM must be able to reach your local IP (might need VPN or public IP)
3. **WebSocket**: The agent uses WebSocket protocol (`ws://`) to connect to the server
