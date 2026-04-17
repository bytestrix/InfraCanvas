#!/bin/bash
# Deploy InfraCanvas agent to any VM via SSH
#
# Usage:
#   ./deploy-agent-to-vm.sh --host 1.2.3.4 --user ubuntu --backend-url wss://relay.yourdomain.com
#
# Or set env vars:
#   VM_HOST=1.2.3.4 VM_USER=ubuntu BACKEND_URL=wss://relay.example.com ./deploy-agent-to-vm.sh

set -euo pipefail

# ── parse args / env ──────────────────────────────────────────────────────────
VM_HOST="${VM_HOST:-}"
VM_USER="${VM_USER:-ubuntu}"
BACKEND_URL="${BACKEND_URL:-}"
AGENT_NAME="${AGENT_NAME:-}"
AUTH_TOKEN="${INFRACANVAS_TOKEN:-}"
SSH_KEY="${SSH_KEY:-}"   # optional: path to private key

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)        VM_HOST="$2";     shift 2;;
    --user)        VM_USER="$2";     shift 2;;
    --backend-url) BACKEND_URL="$2"; shift 2;;
    --name)        AGENT_NAME="$2";  shift 2;;
    --token)       AUTH_TOKEN="$2";  shift 2;;
    --key)         SSH_KEY="$2";     shift 2;;
    *) echo "Unknown argument: $1"; exit 1;;
  esac
done

[[ -z "$VM_HOST" ]]     && { echo "Error: --host is required"; exit 1; }
[[ -z "$BACKEND_URL" ]] && { echo "Error: --backend-url is required (e.g. wss://relay.yourdomain.com)"; exit 1; }
[[ -z "$AGENT_NAME" ]]  && AGENT_NAME="vm-$(echo "$VM_HOST" | tr '.' '-')"

VM_SSH="$VM_USER@$VM_HOST"
SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10"
[[ -n "$SSH_KEY" ]] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

echo "=== InfraCanvas Agent Deployment ==="
echo "  Target: $VM_SSH"
echo "  Backend: $BACKEND_URL"
echo "  Agent name: $AGENT_NAME"
echo ""

# Step 1: Build
echo "[1/4] Building Linux amd64 binary..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/infracanvas-linux-amd64 ./cmd/infracanvas
echo "      Done."

# Step 2: Upload binary + installer
echo "[2/4] Uploading to $VM_SSH..."
scp $SSH_OPTS bin/infracanvas-linux-amd64 "$VM_SSH:/tmp/infracanvas"
scp $SSH_OPTS install-agent.sh "$VM_SSH:/tmp/install-agent.sh"

# Step 3: Run installer on remote VM
echo "[3/4] Running installer on remote VM..."
ssh $SSH_OPTS "$VM_SSH" "
  chmod +x /tmp/infracanvas /tmp/install-agent.sh
  # Use local binary instead of downloading
  mkdir -p /tmp/_ic_build/bin
  cp /tmp/infracanvas /tmp/_ic_build/bin/infracanvas-linux-amd64
  cd /tmp/_ic_build
  INFRACANVAS_BACKEND_URL='$BACKEND_URL' \
  INFRACANVAS_AGENT_NAME='$AGENT_NAME' \
  INFRACANVAS_TOKEN='$AUTH_TOKEN' \
  bash /tmp/install-agent.sh --backend-url '$BACKEND_URL' --name '$AGENT_NAME' ${AUTH_TOKEN:+--token '$AUTH_TOKEN'}
"

# Step 4: Show pair code
echo ""
echo "[4/4] Getting pair code..."
sleep 2
PAIR_CODE=$(ssh $SSH_OPTS "$VM_SSH" \
  "sudo journalctl -u infracanvas-agent -n 30 --no-pager 2>/dev/null | grep -oP 'Pair code: \K\S+' | tail -1 \
  || grep -oP 'Pair code: \K\S+' /var/log/infracanvas-agent.log 2>/dev/null | tail -1 \
  || echo '(check logs on VM)'")

echo ""
echo "=== Deployment Complete ==="
echo ""
echo "  VM:         $VM_SSH"
echo "  Pair code:  $PAIR_CODE"
echo ""
echo "  View logs:  ssh $VM_SSH 'sudo journalctl -u infracanvas-agent -f'"
echo ""
