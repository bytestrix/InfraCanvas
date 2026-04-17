#!/bin/bash
# Deploy infracanvas to remote VM

VM_USER="rishi"
VM_HOST="40.81.240.69"
BINARY="bin/infracanvas"

echo "Deploying infracanvas to ${VM_USER}@${VM_HOST}..."

# Copy binary
scp "$BINARY" "${VM_USER}@${VM_HOST}:~/"

echo ""
echo "Binary copied! Now SSH in and run:"
echo "  ssh ${VM_USER}@${VM_HOST}"
echo "  chmod +x ~/infracanvas"
echo "  sudo mv ~/infracanvas /usr/local/bin/"
echo "  infracanvas discover"
