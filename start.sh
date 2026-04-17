#!/bin/bash
set -euo pipefail

# Start InfraCanvas Platform (local dev)
echo "Starting InfraCanvas Platform..."
echo ""

# Check prerequisites
if [[ ! -f "./bin/infracanvas-server" ]]; then
  echo "ERROR: ./bin/infracanvas-server not found."
  echo "       Run: make build-server"
  exit 1
fi

if [[ ! -d "./frontend/.next" ]]; then
  echo "ERROR: Frontend not built."
  echo "       Run: cd frontend && npm run build"
  exit 1
fi

# Warn if auth is disabled
if [[ -z "${INFRACANVAS_TOKEN:-}" ]]; then
  echo "WARNING: INFRACANVAS_TOKEN is not set — running without auth (dev mode)"
  echo "         Set it in production: export INFRACANVAS_TOKEN=your-secret"
  echo ""
fi

# Start backend server
echo "Starting WebSocket server on :8080..."
./bin/infracanvas-server &
SERVER_PID=$!

# Give it a moment to bind
sleep 1

# Verify it started
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
  echo "ERROR: Server failed to start. Check for port conflicts on :8080"
  exit 1
fi

# Start frontend
echo "Starting frontend on :3000..."
cd frontend
npm start &
FRONTEND_PID=$!
cd ..

echo ""
echo "InfraCanvas is running!"
echo ""
echo "   Frontend: http://localhost:3000"
echo "   Backend:  ws://localhost:8080"
echo ""
echo "To connect an agent on this machine:"
echo "   ./bin/infracanvas start --backend ws://localhost:8080"
echo ""
echo "To connect an agent on a remote VM:"
echo "   bash install-agent.sh --backend-url ws://<this-machine-ip>:8080"
echo ""
echo "Press Ctrl+C to stop all services"
echo ""

cleanup() {
  echo ""
  echo "Stopping services..."
  kill "$SERVER_PID" "$FRONTEND_PID" 2>/dev/null || true
  exit 0
}
trap cleanup INT TERM

wait
