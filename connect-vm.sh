#!/bin/bash
# connect-vm.sh — Connect any remote VM to InfraCanvas via SSH reverse tunnel.
#
# Usage:
#   ./connect-vm.sh                              # defaults: rishi@40.81.240.69
#   ./connect-vm.sh --host 1.2.3.4 --user ubuntu
#   ./connect-vm.sh --host 1.2.3.4 --user ubuntu --port 22 --name my-server
#
# What this does:
#   1. Starts the InfraCanvas backend locally (if not already running)
#   2. Uploads the agent binary to the remote VM via SCP
#   3. Opens an SSH reverse tunnel so the remote VM can reach your local backend
#   4. Runs the agent on the remote VM — it prints a pair code
#   5. You enter that pair code in the frontend at http://localhost:3001

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── defaults ──────────────────────────────────────────────────────────────────
VM_HOST="40.81.240.69"
VM_USER="rishi"
VM_PORT="22"
VM_NAME=""
BACKEND_PORT=8080
TUNNEL_PORT=19090  # port bound on the remote VM side of the reverse tunnel
AGENT_BINARY="$SCRIPT_DIR/bin/infracanvas-linux-amd64"
SERVER_BINARY="$SCRIPT_DIR/bin/infracanvas-server"

# ── arg parsing ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)   VM_HOST="$2";   shift 2;;
    --user)   VM_USER="$2";   shift 2;;
    --port)   VM_PORT="$2";   shift 2;;
    --name)   VM_NAME="$2";   shift 2;;
    --backend-port) BACKEND_PORT="$2"; shift 2;;
    -h|--help)
      sed -n '2,14p' "$0" | sed 's/^# \?//'
      exit 0;;
    *) echo "Unknown argument: $1"; exit 1;;
  esac
done

[[ -z "$VM_NAME" ]] && VM_NAME="$(echo "$VM_HOST" | tr '.' '-')"

SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=15 -p $VM_PORT"

# ── colors ────────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; RED='\033[0;31m'; NC='\033[0m'

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║      InfraCanvas — VM Connector          ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"
echo ""
echo -e "  VM:       ${YELLOW}${VM_USER}@${VM_HOST}:${VM_PORT}${NC}"
echo -e "  Agent:    ${YELLOW}${VM_NAME}${NC}"
echo -e "  Frontend: ${YELLOW}http://localhost:3001${NC}"
echo ""

# ── sanity checks ─────────────────────────────────────────────────────────────
if [[ ! -f "$AGENT_BINARY" ]]; then
  echo -e "${RED}ERROR: Agent binary not found: $AGENT_BINARY${NC}"
  echo "       Run: GOOS=linux GOARCH=amd64 go build -o bin/infracanvas-linux-amd64 ./cmd/infracanvas"
  exit 1
fi

if [[ ! -f "$SERVER_BINARY" ]]; then
  echo -e "${RED}ERROR: Server binary not found: $SERVER_BINARY${NC}"
  echo "       Run: go build -o bin/infracanvas-server ./cmd/infracanvas-server"
  exit 1
fi

# ── step 1: start backend if not already running ──────────────────────────────
echo -e "${GREEN}[1/3] Backend server...${NC}"
if curl -sf "http://localhost:${BACKEND_PORT}/api/health" >/dev/null 2>&1; then
  echo -e "      Already running on port ${BACKEND_PORT}."
else
  echo -e "      Starting on port ${BACKEND_PORT}..."
  "$SERVER_BINARY" &
  SERVER_PID=$!

  # Wait up to 5s for it to be healthy
  for i in $(seq 1 10); do
    sleep 0.5
    if curl -sf "http://localhost:${BACKEND_PORT}/api/health" >/dev/null 2>&1; then
      echo -e "      ${GREEN}Ready (PID $SERVER_PID).${NC}"
      break
    fi
    if [[ $i -eq 10 ]]; then
      echo -e "${RED}ERROR: Backend failed to start. Check port ${BACKEND_PORT} is free.${NC}"
      exit 1
    fi
  done

  # Kill backend when this script exits
  trap "echo ''; echo 'Shutting down backend (PID $SERVER_PID)...'; kill $SERVER_PID 2>/dev/null || true" EXIT
fi

# ── step 2: upload agent binary to remote VM ──────────────────────────────────
echo ""
echo -e "${GREEN}[2/3] Uploading agent to ${VM_USER}@${VM_HOST}...${NC}"
scp -o StrictHostKeyChecking=no -o ConnectTimeout=15 -P "$VM_PORT" "$AGENT_BINARY" "${VM_USER}@${VM_HOST}:/tmp/infracanvas-agent"
ssh $SSH_OPTS "${VM_USER}@${VM_HOST}" "chmod +x /tmp/infracanvas-agent"
echo -e "      Uploaded."

# ── step 3: SSH reverse tunnel + run agent ────────────────────────────────────
echo ""
echo -e "${GREEN}[3/3] Connecting via SSH reverse tunnel...${NC}"
echo ""
echo -e "  The backend relay is tunneled to the remote VM."
echo -e "  The agent will print a ${YELLOW}pair code${NC} below."
echo -e "  Enter it at ${YELLOW}http://localhost:3001${NC} → \"Connect VM\""
echo ""
echo -e "  (Press ${YELLOW}Ctrl+C${NC} to disconnect the VM)"
echo ""
echo "────────────────────────────────────────────────"

# -R tunnels remote VM's :BACKEND_PORT → local :BACKEND_PORT
# So the agent on the VM connects to ws://localhost:BACKEND_PORT
# and it actually reaches your local InfraCanvas backend.
# -R tunnels remote VM's :TUNNEL_PORT → local :BACKEND_PORT
# Avoids conflicts with whatever is already running on port 8080 on the VM.
ssh $SSH_OPTS \
    -o ExitOnForwardFailure=yes \
    -R "${TUNNEL_PORT}:localhost:${BACKEND_PORT}" \
    "${VM_USER}@${VM_HOST}" \
    "/tmp/infracanvas-agent start --backend ws://localhost:${TUNNEL_PORT}"
