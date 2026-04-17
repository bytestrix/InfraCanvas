#!/bin/bash
set -e

# Configuration
VM_USER="rishi"
VM_HOST="40.81.240.69"
VM_SSH="$VM_USER@$VM_HOST"

echo "=== InfraCanvas Backend Deployment to Azure VM ==="
echo ""

# Step 1: Build the backend binary for Linux
echo "[1/4] Building backend binary for Linux..."
GOOS=linux GOARCH=amd64 go build -o bin/infracanvas-server-linux ./cmd/infracanvas-server

# Step 2: Copy backend to VM
echo "[2/4] Copying backend to VM..."
ssh $VM_SSH 'rm -rf /tmp/infracanvas-server-binary'
scp bin/infracanvas-server-linux $VM_SSH:/tmp/infracanvas-server-binary

# Step 3: Install backend on VM
echo "[3/4] Installing backend on VM..."
ssh $VM_SSH 'bash -s' <<'ENDSSH'
# Kill old backend if running
pkill -f 'infracanvas-server' || true

# Create directory
mkdir -p ~/infracanvas/backend

# Move binary
mv /tmp/infracanvas-server-binary ~/infracanvas/backend/infracanvas-server
chmod +x ~/infracanvas/backend/infracanvas-server

echo "Backend installed successfully!"
ENDSSH

# Step 4: Start the backend
echo "[4/4] Starting backend..."
ssh $VM_SSH 'bash -s' <<'ENDSSH'
cd ~/infracanvas/backend
nohup ./infracanvas-server > backend.log 2>&1 < /dev/null &
disown
echo "Backend started on port 8080"
ENDSSH

sleep 2

echo ""
echo "=== Deployment Complete! ==="
echo ""
echo "Backend is running at: http://$VM_HOST:8080"
echo ""
echo "Check backend logs:"
echo "  ssh $VM_SSH 'cat ~/infracanvas/backend/backend.log'"
echo ""
echo "Update your frontend .env.local:"
echo "  NEXT_PUBLIC_WS_URL=ws://$VM_HOST:8080"
