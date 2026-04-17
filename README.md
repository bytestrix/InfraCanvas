# InfraCanvas

**Real-time infrastructure topology canvas — containers, Kubernetes, and VMs visualized as a live graph.**

Install a lightweight agent on any Linux VM with one command. No inbound ports, no VPN, no cloud accounts. The agent phones home to your relay server over a single outbound WebSocket. You get a live visual map of everything running on your servers — containers, pods, networks, volumes, deployments — updating automatically every 30 seconds and diffing to minimize bandwidth.

![InfraCanvas Canvas](docs/canvas-preview.png)

---

## How it works

```
Your browser
  └─ Dashboard  (Next.js · port 3000)
          │
          │  WebSocket  /ws/canvas  (outbound only)
          ▼
    Relay Server  (Go · port 8080)
          ▲
          │  WebSocket  /ws/agent  (outbound only)
  VMs running the agent
```

The agent on each VM connects **outbound** to your relay server — no inbound firewall rules needed on the VMs. You pair a VM to your dashboard with a short human-readable code (e.g. `TIGER-APPLE-CLOUD`). Multiple VMs, multiple browser tabs — all work simultaneously through the same relay.

---

## Quick Start

### 1. Run the relay + dashboard

You need Docker and Docker Compose installed on a server (or your laptop).

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

NEXT_PUBLIC_WS_URL=ws://YOUR_SERVER_IP:8080 \
NEXT_PUBLIC_API_URL=http://YOUR_SERVER_IP:8080 \
docker compose up -d
```

Open **http://YOUR_SERVER_IP:3000** in your browser.

> Running locally? Replace `YOUR_SERVER_IP` with `localhost`.

### 2. Install the agent on any Linux VM

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

The agent connects, prints a **pair code**, and waits:

```
────────────────────────────────────────────────────
  InfraCanvas agent running

  Pair code:  TIGER-APPLE-CLOUD

  Open the canvas and enter this code to connect.
────────────────────────────────────────────────────
```

Enter the code in the dashboard — the VM appears on the canvas instantly.

### 3. Uninstall the agent

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash
```

---

## Features

### Canvas

| Feature | Details |
|---|---|
| **Live topology graph** | Every container, pod, service, volume, network, image drawn as a node with edges showing relationships |
| **Real-time updates** | Agent pushes a full snapshot on connect, then incremental diffs every 30 s — only changed nodes/edges are sent |
| **Grouped view** | Nodes buckered by type (Containers, K8s Workloads, Storage…) — one card per group, click to drill in |
| **Flat view** | Every individual node laid out by a Dagre hierarchy — zoom in for full detail |
| **Filter chips** | Toggle/spotlight Kubernetes · Docker · Host · Pods · Storage · Events groups; right-click to hide |
| **Health colors** | Healthy = green, degraded = amber, unhealthy = red, unknown = grey — driven by live container/pod state |
| **Critical alert banner** | Shown automatically when any group has degraded/unhealthy nodes — click to drill in |
| **Multi-VM** | Add as many VMs as you want from the same dashboard; each runs independently |
| **Export PNG** | Snapshot the current canvas as a high-res image |
| **Export JSON** | Download the full raw graph (nodes + edges + stats) as JSON |

### Container & Docker

| Feature | Details |
|---|---|
| **Container actions** | Restart / Stop / Start any container from the UI — sent to the agent via WebSocket, executed with Docker SDK |
| **Image tag update** | Pull a new image tag for a container directly from the panel |
| **Container logs** | View last 200 log lines with ERROR/WARN/INFO color-coding; download as `.txt` |
| **Container terminal** | Full interactive TTY shell inside any container (`docker exec`) with xterm.js |
| **Volumes & networks** | Visualized as nodes with mount/connect edges to containers |
| **Docker Compose projects** | Compose project membership shown via metadata |

### Kubernetes

| Feature | Details |
|---|---|
| **Full resource graph** | Cluster → Nodes → Namespaces → Deployments/StatefulSets/DaemonSets → Pods → Services → Ingress → PVCs |
| **Pod health** | Phase-driven health colors; pod terminal coming soon |
| **Rolling restart** | Trigger `kubectl rollout restart` equivalent for any Deployment/StatefulSet/DaemonSet |
| **Update image** | Change the container image for a Deployment via the UI |
| **Scale** | Set replica count for Deployments and StatefulSets |
| **Pod logs** | Fetch logs from any pod (`k8s_get_logs`) |
| **Events** | K8s events shown as nodes with links to affected resources |

### VM / Host

| Feature | Details |
|---|---|
| **Host info** | OS, kernel, CPU cores, memory, hostname |
| **VM terminal** | Full interactive PTY shell directly on the VM (`/bin/bash`) via xterm.js |
| **Cloud detection** | AWS / GCP / Azure / on-prem detected automatically |
| **Environment detection** | production / staging / QA / dev / test inferred from hostname/namespace patterns |

### Security

| Feature | Details |
|---|---|
| **Secret redaction** | Environment variables containing `SECRET`, `TOKEN`, `KEY`, `PASSWORD`, `CREDENTIAL` etc. are automatically redacted before leaving the VM |
| **Auth token** | Optional shared secret between agent and relay (`INFRACANVAS_TOKEN`); relay rejects connections without it |
| **Outbound-only agents** | No inbound ports needed on monitored VMs — agents initiate the WebSocket connection |

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     Browser                              │
│           Dashboard  (Next.js · port 3000)               │
│   ReactFlow canvas · Zustand state · xterm.js terminal   │
└───────────────────────┬─────────────────────────────────┘
                        │  WS /ws/canvas
                        │  PAIR → GRAPH_SNAPSHOT → GRAPH_DIFF
                        │  BROWSER_ACTION → ACTION_REQUEST
                        │  ACTION_RESULT ← ACTION_PROGRESS
                        │  LOG_DATA · EXEC_DATA · EXEC_INPUT
┌───────────────────────▼─────────────────────────────────┐
│                  Relay Server  (Go · port 8080)          │
│  /ws/agent  ←── agents connect outbound                  │
│  /ws/canvas ←── browsers subscribe                       │
│  /api/health · /api/sessions                             │
│  Transparent relay: agent msgs → all paired browsers     │
│  Transparent relay: browser msgs → paired agent          │
└───────────────────────┬─────────────────────────────────┘
                        │  WS /ws/agent
                        │  HELLO · PAIR_CODE · GRAPH_SNAPSHOT
                        │  ACTION_REQUEST → ACTION_RESULT
                        │  EXEC_START / EXEC_INPUT / EXEC_DATA
┌───────────────────────▼─────────────────────────────────┐
│                  Agent Binary  (infracanvas start)        │
│                                                          │
│  Discovery:                                              │
│    ├── Host (OS, CPU, memory, network interfaces)        │
│    ├── Docker (containers, images, volumes, networks)    │
│    └── Kubernetes (pods, deployments, services, ingress) │
│                                                          │
│  Actions:                                                │
│    ├── Docker: restart / stop / start / logs / exec      │
│    └── Kubernetes: rollout restart / scale / update img  │
│                                                          │
│  Terminals:                                              │
│    ├── Container exec  (Docker exec API + PTY)           │
│    └── VM shell        (creack/pty + /bin/bash)          │
└──────────────────────────────────────────────────────────┘
```

### WebSocket message protocol

| Message | Direction | Purpose |
|---|---|---|
| `HELLO` | agent → relay | Agent identifies itself (hostname, scope, version) |
| `PAIR_CODE` | relay → agent | Relay assigns a human-readable pair code |
| `PAIRED` | relay → agent | A browser has connected to this session |
| `GRAPH_SNAPSHOT` | agent → relay → browser | Full graph on first connect |
| `GRAPH_DIFF` | agent → relay → browser | Incremental changes every 30 s |
| `HEARTBEAT` | agent → relay | Keep-alive every 15 s |
| `BROWSER_ACTION` | browser → relay | UI action (renamed to `ACTION_REQUEST` before forwarding) |
| `ACTION_REQUEST` | relay → agent | Execute a docker/k8s/host action |
| `ACTION_RESULT` | agent → relay → browser | Action outcome (success/failure + details) |
| `ACTION_PROGRESS` | agent → relay → browser | In-progress updates (0–100%) |
| `LOG_DATA` | agent → relay → browser | Streaming container log lines |
| `EXEC_START` | browser → relay → agent | Open a PTY/exec terminal session |
| `EXEC_INPUT` | browser → relay → agent | Keystrokes from xterm.js (base64) |
| `EXEC_RESIZE` | browser → relay → agent | Terminal window resize event |
| `EXEC_DATA` | agent → relay → browser | Terminal output to xterm.js (base64) |
| `EXEC_END` | both directions | Session terminated |

---

## Self-hosting

### Requirements

- Linux server with Docker + Docker Compose
- Ports **3000** (dashboard) and **8080** (relay) open in your firewall
- Agents only need outbound internet to reach your relay on port 8080

### Deploy

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Replace with your server IP or domain
NEXT_PUBLIC_WS_URL=ws://YOUR_IP:8080 \
NEXT_PUBLIC_API_URL=http://YOUR_IP:8080 \
docker compose up -d
```

### With TLS / custom domain

Put Caddy or nginx in front as a reverse proxy. Caddy example (`Caddyfile`):

```
canvas.example.com {
    reverse_proxy localhost:3000
}

relay.example.com {
    reverse_proxy localhost:8080
}
```

Then use `wss://relay.example.com` as the WebSocket URL and update `DEFAULT_RELAY_URL` in `install-agent.sh`.

### Useful commands

```bash
# View logs
docker compose logs -f

# Stop everything
docker compose down

# Update to latest image
git pull && docker compose up --build -d

# Check relay health
curl http://YOUR_IP:8080/api/health
```

---

## Agent management

```bash
# View live logs (systemd)
sudo journalctl -u infracanvas-agent -f

# Status
sudo systemctl status infracanvas-agent

# Restart
sudo systemctl restart infracanvas-agent

# Stop
sudo systemctl stop infracanvas-agent

# Get pair code if you missed it
sudo journalctl -u infracanvas-agent -n 50 | grep "Pair code"
```

### Custom relay URL

By default `install.sh` connects to `ws://13.49.41.61:8080` (the public demo relay). To point to your own:

```bash
# Via environment variable
INFRACANVAS_BACKEND_URL=ws://your-relay:8080 \
  curl -fsSL .../install.sh | bash

# Via flag
bash install.sh --backend-url ws://your-relay:8080

# Or edit /etc/infracanvas/agent.env after install
sudo systemctl restart infracanvas-agent
```

---

## Building from source

**Requirements:** Go 1.25+, Node.js 20+

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Build agent binary
make build

# Build all release targets (linux/darwin × amd64/arm64)
make build-all

# Run all tests
make test

# Run dashboard + relay locally (requires docker compose)
NEXT_PUBLIC_WS_URL=ws://localhost:8080 \
NEXT_PUBLIC_API_URL=http://localhost:8080 \
docker compose up --build
```

### Project layout

```
InfraCanvas/
├── cmd/
│   ├── infracanvas/          # Agent CLI  (infracanvas start / discover / logs …)
│   └── infracanvas-server/   # Relay server entry point
├── pkg/
│   ├── agent/                # WebSocket agent: connect, discover, diff, actions, exec
│   ├── actions/              # Action executors: Docker, Kubernetes, Host
│   ├── discovery/
│   │   ├── docker/           # Container, image, volume, network discovery
│   │   ├── host/             # OS, CPU, memory, process discovery
│   │   └── kubernetes/       # Full K8s resource discovery
│   ├── server/               # Relay server: WebSocket broker, session store
│   ├── orchestrator/         # Combines all discovery sources into one snapshot
│   ├── output/               # Graph formatter (nodes + edges JSON)
│   ├── relationships/        # Builds edges between entities
│   ├── health/               # Health status calculator per node
│   ├── environment/          # Environment detection (prod/staging/dev)
│   └── redactor/             # Sensitive value redaction
├── internal/
│   └── models/               # Core data models (snapshot, container, pod…)
├── frontend/
│   ├── app/                  # Next.js 14 app router
│   ├── components/canvas/
│   │   ├── InfraCanvas.tsx   # Main canvas: ReactFlow + toolbar + export
│   │   ├── NodeDetailPanel.tsx # Right panel: metadata, actions, logs/terminal buttons
│   │   ├── LogsPanel.tsx     # Bottom panel: streaming container logs
│   │   ├── TerminalPanel.tsx # Bottom panel: xterm.js PTY terminal
│   │   ├── GroupNode.tsx     # Grouped card node
│   │   ├── InfraNode.tsx     # Individual entity node
│   │   ├── GroupDrawer.tsx   # Slide-out drawer when clicking a group
│   │   └── NamespaceGroupNode.tsx
│   ├── lib/
│   │   ├── wsManager.ts      # WebSocket singleton, reconnect, all subscriptions
│   │   ├── layout.ts         # Dagre + zone layout algorithms
│   │   └── graphPreprocess.ts # Group builder, health rollup
│   ├── store/
│   │   └── vmStore.ts        # Zustand global state (VMs, graphs, diffs)
│   └── types/index.ts        # All TypeScript types
├── install-agent.sh          # One-liner installer (released as install.sh)
├── uninstall-agent.sh        # One-liner uninstaller (released as uninstall.sh)
├── Dockerfile.server         # Relay server Docker image
├── frontend/Dockerfile       # Dashboard Docker image
├── docker-compose.yml        # Relay + dashboard together
└── .github/workflows/
    ├── ci.yml                # Go build + test on every push/PR
    └── release.yml           # Cross-compile agent binaries on git tag
```

---

## CI / CD

- **CI** (`ci.yml`) — runs `go build ./...` and `go test ./...` on every push to `main` and every PR
- **Release** (`release.yml`) — triggered by `v*.*.*` tags; cross-compiles agent for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64; publishes GitHub Release with binaries + `install.sh` + `uninstall.sh`

To release a new version:

```bash
git tag v1.2.0
git push origin v1.2.0
```

GitHub Actions builds everything and creates the release automatically.

---

## Contributing

Contributions are welcome. Please open an issue before submitting a large PR so we can discuss the approach.

```bash
# Run tests
go test ./...

# Run a specific package
go test ./pkg/relationships/...

# Lint
golangci-lint run ./...
```

---

## License

MIT — see [LICENSE](LICENSE) for details.
