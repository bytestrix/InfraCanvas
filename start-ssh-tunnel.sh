#!/bin/bash

# SSH tunnel to forward Azure VM's backend to your local machine
# This allows your frontend to connect to the backend through SSH

VM_USER="rishi"
VM_HOST="40.81.240.69"

echo "Starting SSH tunnel..."
echo "Local port 8080 -> Azure VM port 8080"
echo ""
echo "Keep this terminal open while using the frontend."
echo "Press Ctrl+C to stop the tunnel."
echo ""

# Forward local port 8080 to VM's port 8080
ssh -L 8080:localhost:8080 $VM_USER@$VM_HOST -N
