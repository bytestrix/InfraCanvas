# Quick Setup Guide

## 1. Build and Run Frontend

```bash
cd frontend

# Build the Next.js app
npm run build

# Start the production server
npm start
# OR for development with hot reload:
npm run dev
```

The frontend will be available at: http://localhost:3000

## 2. Build and Run Backend Server

```bash
# From the root directory
cd ..

# Build the Go binaries
make build

# Start the WebSocket server
./bin/infracanvas-server
# Default: listens on :8080
```

## 3. Run Agent on VM

On your Azure VM (40.81.240.69):

```bash
# SSH to the VM
ssh rishi@40.81.240.69

# Navigate to the project
cd /path/to/InfraCanvas

# Build the agent
make build

# Run the agent (connects to your server)
./bin/infracanvas start --backend-url ws://YOUR_SERVER_IP:8080

# Example if server is on localhost:
./bin/infracanvas start --backend-url ws://localhost:8080

# Example if server is on another machine:
./bin/infracanvas start --backend-url ws://192.168.1.100:8080
```

The agent will print a PAIR CODE like: `ABC123`

## 4. Connect in Browser

1. Open http://localhost:3000
2. Click "Connect VM"
3. Enter the pair code from step 3
4. View your infrastructure!

## 5. Use DevOps Operations

Once connected:
1. Click on a VM card to view its canvas
2. Look for the "Operations" button (we'll add it next)
3. Select operations like "Update Image"
4. Execute across multiple VMs!

---

## Troubleshooting

### Frontend won't build
```bash
cd frontend
rm -rf .next node_modules
npm install
npm run build
```

### Backend won't compile
```bash
# Make sure you have Go 1.21+
go version

# Clean and rebuild
make clean
make build
```

### Agent can't connect
- Check firewall allows port 8080
- Verify backend-url is correct
- Check server is running: `curl http://localhost:8080/api/health`

### No Kubernetes data
- Agent needs kubeconfig access
- Check: `kubectl cluster-info`
- Ensure RBAC permissions

---

## Architecture

```
Browser (localhost:3000)
    ↓ WebSocket
Server (localhost:8080)
    ↓ WebSocket  
Agent (on VM - discovers infrastructure)
```

---

## Next: Add Operations Button

See the integration code below to add the operations panel to your canvas!
