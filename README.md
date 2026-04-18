# InfraCanvas

**Real-time infrastructure topology canvas — containers, Kubernetes, and VMs visualized as a live graph.**

[![CI](https://github.com/bytestrix/InfraCanvas/actions/workflows/ci.yml/badge.svg)](https://github.com/bytestrix/InfraCanvas/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.21+](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/bytestrix/InfraCanvas)](https://goreportcard.com/report/github.com/bytestrix/InfraCanvas)

Install a lightweight agent on any Linux VM with one command. No inbound ports, no VPN, no cloud accounts. The agent phones home to your relay server over a single outbound WebSocket. You get a live visual map of everything running on your servers — containers, pods, networks, volumes, deployments — updating every 30 seconds and diffing to minimize bandwidth.

> **Security notice:** Pair codes grant read/exec access to your infrastructure. Treat them like passwords. Use a TLS-enabled relay (`wss://`) in production — see [Security](#security) below.

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

The agent on each VM connects **outbound** to the relay — no inbound firewall rules needed on the VMs. You pair a VM to your dashboard with a short code like `WOLF-BEAR-482917`. Multiple VMs, multiple browser tabs — all work simultaneously through the same relay.

---

## Quick Start

### 1. Run the relay + dashboard

You need Docker and Docker Compose installed.

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas
docker compose up -d
```

Open **http://localhost:3000** in your browser.

> The default `frontend/.env` points to the public demo relay at `ws://13.200.198.166:8080`.  
> To self-host, see [Self-hosting](#self-hosting) below.

### 2. Install the agent on any Linux VM

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

The agent connects, prints a **pair code**, and waits:

```
────────────────────────────────────────────────────────────────────
  InfraCanvas agent running

  Pair code:  WOLF-BEAR-482917

  Open the canvas and enter this code to connect.
────────────────────────────────────────────────────────────────────
```

If you missed the code:

```bash
sudo journalctl -u infracanvas-agent -n 50 | grep "Pair code"
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
| **Grouped view** | Nodes bucketed by type (Containers, K8s Workloads, Storage…) — one card per group, click to drill in |
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
| **Environment variables** | View all env vars with automatic secret masking; toggle to reveal |
| **Port mappings** | See host→container port bindings at a glance |
| **Mounts** | Volume and bind-mount paths with read/write mode |
| **Volumes & networks** | Visualized as nodes with mount/connect edges to containers |

### Kubernetes

| Feature | Details |
|---|---|
| **Full resource graph** | Cluster → Nodes → Namespaces → Deployments/StatefulSets/DaemonSets → Pods → Services → Ingress → PVCs |
| **Pod health** | Phase-driven health colors |
| **Rolling restart** | Trigger `kubectl rollout restart` for any Deployment/StatefulSet/DaemonSet |
| **Update image** | Change the container image for a Deployment via the UI |
| **Scale** | Set replica count for Deployments and StatefulSets |
| **Pod logs** | Fetch logs from any pod |
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
| **Cryptographic pair codes** | Codes are generated with `crypto/rand` — not guessable from time-based seeds |

---

## Security

### Pair codes

Pair codes (e.g. `WOLF-BEAR-482917`) are generated with `crypto/rand` and have ~1.44 billion possible values. Anyone who knows a pair code can:

- Read your full infrastructure topology (container names, images, env vars after redaction)
- Stream container and pod logs
- Open a shell into any container on that VM
- Execute Docker and Kubernetes actions (restart, scale, update image)

**Treat pair codes like API keys.** Do not share them in public channels or commit them to version control.

### Use TLS in production

The default setup runs over plain `ws://`. For any internet-facing deployment:

1. Put Caddy or nginx in front of the relay and dashboard (see [Self-hosting](#self-hosting))
2. Use `wss://` in `frontend/.env` and in the agent `--backend-url` flag
3. Set `INFRACANVAS_TOKEN` as a shared secret so only your agents can connect

### Auth token

```bash
# Set on the relay server (docker-compose.yml or environment)
INFRACANVAS_TOKEN=your-random-secret

# Set on each agent
INFRACANVAS_TOKEN=your-random-secret infracanvas start
# or in /etc/infracanvas/agent.env
```

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
| `PAIR_CODE` | relay → agent | Relay assigns a pair code like `WOLF-BEAR-482917` |
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

### Deploy your own relay

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Point everything at your server
NEXT_PUBLIC_WS_URL=ws://YOUR_IP:8080 \
NEXT_PUBLIC_API_URL=http://YOUR_IP:8080 \
docker compose up -d
```

Then update `DEFAULT_RELAY_URL` in `install-agent.sh` to `ws://YOUR_IP:8080` before distributing to your VMs.

> **Default relay:** The repo ships with `frontend/.env` pointing to `ws://13.200.198.166:8080` (the public demo relay). Override it by creating `frontend/.env.local` with your own URL — `.env.local` takes priority and is gitignored.

### With TLS / custom domain

Put Caddy or nginx in front. Caddy example:

```
canvas.example.com {
    reverse_proxy localhost:3000
}

relay.example.com {
    reverse_proxy localhost:8080
}
```

Then use `wss://relay.example.com` as the WebSocket URL and set `INFRACANVAS_TOKEN` for auth.

### Useful commands

```bash
docker compose logs -f
docker compose down
git pull && docker compose up --build -d
curl http://YOUR_IP:8080/api/health
```

---

## Agent management

```bash
# View live logs
sudo journalctl -u infracanvas-agent -f

# Status / restart / stop
sudo systemctl status infracanvas-agent
sudo systemctl restart infracanvas-agent
sudo systemctl stop infracanvas-agent

# Get pair code if you missed it
sudo journalctl -u infracanvas-agent -n 50 | grep "Pair code"
```

### Point agent at a custom relay

```bash
# Via environment variable at install time
INFRACANVAS_BACKEND_URL=ws://your-relay:8080 \
  curl -fsSL .../install.sh | bash

# Via flag
bash install.sh --backend-url ws://your-relay:8080

# Or edit the env file after install
sudo nano /etc/infracanvas/agent.env
sudo systemctl restart infracanvas-agent
```

---

## Building from source

**Requirements:** Go 1.21+, Node.js 20+

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Build agent binary
make build

# Build all release targets (linux/darwin × amd64/arm64)
make build-all

# Run all tests
make test

# Run dashboard + relay locally
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
│   ├── .env                  # Default env (points to public relay) — committed
│   ├── app/                  # Next.js 14 app router
│   ├── components/canvas/
│   │   ├── InfraCanvas.tsx   # Main canvas: ReactFlow + toolbar + export
│   │   ├── NodeDetailPanel.tsx
│   │   ├── LogsPanel.tsx
│   │   ├── TerminalPanel.tsx
│   │   ├── GroupNode.tsx
│   │   ├── InfraNode.tsx
│   │   └── GroupDrawer.tsx
│   ├── lib/
│   │   ├── wsManager.ts      # WebSocket singleton, reconnect, subscriptions
│   │   ├── layout.ts         # Dagre + zone layout algorithms
│   │   └── graphPreprocess.ts
│   ├── store/vmStore.ts      # Zustand global state
│   └── types/index.ts
├── install-agent.sh
├── uninstall-agent.sh
├── Dockerfile.server
├── frontend/Dockerfile
└── docker-compose.yml
```

---

## Roadmap

- [ ] Kubernetes pod exec (interactive shell inside pods via `kubectl exec`)
- [ ] Rate limiting on the relay (protect pair codes from enumeration)
- [ ] Multi-relay federation (one dashboard, multiple relay regions)
- [ ] Prometheus metrics endpoint on the relay
- [ ] Helm chart for the relay + dashboard
- [ ] Mobile-responsive canvas

---

## CI / CD

- **CI** (`ci.yml`) — runs `go build ./...` and `go test ./...` on every push/PR
- **Release** (`release.yml`) — triggered by `v*.*.*` tags; cross-compiles agent for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64; publishes GitHub Release with binaries + `install.sh` + `uninstall.sh`

```bash
git tag v1.2.0
git push origin v1.2.0
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide. The short version:

1. Open an issue before a large PR
2. Fork → branch from `main` → PR back to `main`
3. All of these must pass: `make test`, `make lint`, `cd frontend && npm run lint`

Good first issues are tagged [`good first issue`](https://github.com/bytestrix/InfraCanvas/issues?q=is%3Aopen+label%3A%22good+first+issue%22).

---

## License

MIT — see [LICENSE](LICENSE) for details.
