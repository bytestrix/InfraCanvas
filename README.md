# InfraCanvas

**See everything running on your servers — in one live, visual map.**

[![CI](https://github.com/bytestrix/InfraCanvas/actions/workflows/ci.yml/badge.svg)](https://github.com/bytestrix/InfraCanvas/actions/workflows/ci.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
[![Go 1.21+](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/bytestrix/InfraCanvas)](https://goreportcard.com/report/github.com/bytestrix/InfraCanvas)

InfraCanvas runs a tiny agent on any Linux server. The agent discovers every container, pod, volume, network, and deployment on that machine and streams it to a visual canvas in your browser — live, updating every 30 seconds.

**No VPN. No inbound firewall rules. No cloud account needed.**

The agent connects *outward* to a relay server. The relay connects your browser to your VMs. Your servers never accept a connection from the outside world.

---

## What you get

- **Live topology graph** — every container, Kubernetes pod, service, volume, and network drawn as a connected graph. Relationships are shown as edges (this container mounts that volume, this pod belongs to that deployment).
- **Health at a glance** — nodes turn green, amber, or red based on real container/pod state. A banner appears automatically when something is unhealthy.
- **Container terminal** — open a real shell inside any running container, right in the browser. Full color, resize, everything.
- **VM shell** — open a terminal on the VM itself, not just the containers.
- **Container logs** — last 200 lines, color-coded by severity. Download as a file.
- **Kubernetes actions** — rolling restart, scale up/down, update the container image for any deployment — all from the UI.
- **Docker actions** — restart, stop, start, update image for any container.
- **Inspect everything** — click any node to see its environment variables (with secrets masked), port mappings, volume mounts, and image details.
- **Multi-VM** — connect as many servers as you want. Each one gets its own card in the dashboard.
- **Export** — save the canvas as a PNG screenshot or download the full graph as JSON.

---

## How it works

There are three pieces:

```
Your browser
  └── Dashboard (Next.js)
          │  WebSocket (outbound)
          ▼
    Relay Server (Go)
          ▲
          │  WebSocket (outbound from VM)
    Agent on each VM
```

**Relay server** — runs on any internet-accessible machine. Acts as a message broker. It never looks inside the data, just passes it through.

**Agent** — a single Go binary you install on each VM. It discovers what's running, builds a graph, and sends it to the relay. When you click "Restart container" in the browser, the relay forwards that instruction to the agent, which runs it.

**Dashboard** — a Next.js web app that connects to the relay. You enter a short **pair code** (like `WOLF-BEAR-482917`) and instantly see that VM's graph.

**Pair codes** are how VMs are identified. When the agent connects to the relay, the relay gives it a unique code. You type that code in the browser. The relay connects them. Nobody else can see your VM's data unless they know the code.

---

## Quick Start

### Step 1 — Run the relay and dashboard

You need Docker and Docker Compose.

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas
docker compose up -d
```

Open **http://localhost:3000** in your browser.

> By default the agent connects to the relay at `ws://localhost:8080` (the backend you started with `docker compose up -d`). For production deployments on separate hosts, [host your own relay](#self-hosting).

### Step 2 — Install the agent on a server

Run this on any Linux VM you want to monitor:

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

The agent starts, connects to the relay, and prints a pair code:

```
────────────────────────────────────────────────────────────
  InfraCanvas agent running

  Pair code:  WOLF-BEAR-482917

  Enter this code in the dashboard to connect.
────────────────────────────────────────────────────────────
```

### Step 3 — Enter the code in the dashboard

Type the pair code into the "Connect a VM" field in the browser. The VM appears on the canvas within a few seconds.

**Missed the code?**
```bash
sudo journalctl -u infracanvas-agent -n 300 | grep -v "deprecated" | grep "Pair code"
```

### Step 4 — Uninstall

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash
```

---

## Your data stays on your servers

Here is exactly what leaves each VM:

- Container names, IDs, status, image names, port mappings, restart counts
- Environment variables — **with secret values redacted**. Any variable whose name contains `SECRET`, `TOKEN`, `KEY`, `PASSWORD`, `CREDENTIAL`, `AUTH`, or `PASSWD` is replaced with `[REDACTED]` before it ever leaves the VM.
- Resource metadata: CPU, memory, network interfaces, OS info
- Kubernetes resource names and states (no secret values from ConfigMaps or Secrets)

What **never** leaves your VM:
- Actual secret values
- File contents
- Database contents
- Network traffic content
- Anything you did not explicitly stream (like logs or terminal output — those only go to your browser, not stored anywhere)

The relay server does not store snapshots to disk. If you disconnect, the data is gone.

---

## Security

### Pair codes protect your VMs

A pair code like `WOLF-BEAR-482917` is the only thing standing between your VM's data and the public internet (when using plain `ws://`). Treat it like a password:

- Do not post it in public Slack channels or GitHub issues
- Do not commit it to version control
- Regenerate it by restarting the agent (`sudo systemctl restart infracanvas-agent`)

Codes are generated with `crypto/rand` and have approximately 1.44 billion possible values — not guessable from time or hostname.

### Anyone with the pair code can

- See your full infrastructure topology
- View container logs
- Open a shell inside any container on that VM
- Run Docker and Kubernetes actions (restart, scale, update image)

### For production: use TLS and an auth token

Plain `ws://` is fine for local testing. For anything accessible from the internet:

**1. Put Caddy or nginx in front (automatic HTTPS):**

```
canvas.example.com {
    reverse_proxy localhost:3000
}
relay.example.com {
    reverse_proxy localhost:8080
}
```

**2. Set a shared secret so only your agents can connect:**

```bash
# On the relay server (add to docker-compose.yml environment)
INFRACANVAS_TOKEN=a-long-random-string

# On each agent (/etc/infracanvas/agent.env)
INFRACANVAS_TOKEN=a-long-random-string
```

**3. Use `wss://` in your dashboard `.env`:**

```
NEXT_PUBLIC_WS_URL=wss://relay.example.com
```

---

## All features

### Canvas

| Feature | What it does |
|---|---|
| Live topology graph | Every container, pod, service, volume, network drawn as nodes with edges showing relationships |
| Real-time updates | Full snapshot on first connect, then only changes every 30 s — minimal bandwidth |
| Grouped view | Nodes grouped by type (Containers, K8s Workloads, Storage…) — one card per group, click to expand |
| Flat view | Every node laid out individually by type and relationship — zoom in for full detail |
| Filter chips | Show/hide Kubernetes, Docker, Host, Pods, Storage, Events |
| Health colors | Green = healthy, amber = degraded, red = unhealthy, grey = unknown — from live state |
| Alert banner | Appears automatically when any group has unhealthy nodes |
| Multi-VM | Connect as many VMs as you want — each appears as a separate card |
| Export PNG | Save the canvas as a high-resolution image |
| Export JSON | Download the raw graph (all nodes, edges, metadata) |

### Containers and Docker

| Feature | What it does |
|---|---|
| Container terminal | Full interactive shell inside any container (`docker exec`) with color and resize |
| Container logs | Last 200 lines, color-coded ERROR/WARN/INFO, downloadable |
| Restart / Stop / Start | Run from the UI — executed by the agent on the VM |
| Update image | Set a new image tag and the agent pulls and recreates the container |
| Environment variables | All env vars shown with automatic secret masking — click to reveal |
| Port mappings | See which host ports map to which container ports |
| Volume mounts | See every bind mount and named volume, with source/destination paths |
| Image details | Registry, tag, size, digest, which containers are using it |

### Kubernetes

| Feature | What it does |
|---|---|
| Full resource graph | Cluster → Nodes → Namespaces → Deployments → Pods → Services → Ingress → PVCs |
| Health from pod phase | Running/Pending/Failed → green/amber/red |
| Rolling restart | Trigger `kubectl rollout restart` for Deployments, StatefulSets, DaemonSets |
| Update image | Change the container image for any Deployment |
| Scale | Change replica count for Deployments and StatefulSets |
| Pod logs | Fetch logs from any pod directly in the panel |
| K8s events | Events shown as nodes linked to the resources they affect |

### VM / Host

| Feature | What it does |
|---|---|
| VM terminal | Full interactive shell on the VM itself (not inside a container) |
| Host info | OS, kernel version, CPU cores, total memory, hostname |
| Cloud detection | Automatically identifies AWS, GCP, Azure, or on-prem |
| Environment detection | Infers production/staging/dev/test from hostname and namespace patterns |

---

## Self-hosting

### Requirements

- A Linux server with Docker and Docker Compose (for the relay + dashboard)
- Port **8080** open for agents to connect to the relay
- Port **3000** open for your browser to reach the dashboard
- Agents only need outbound access to port 8080 — no inbound ports on monitored VMs

### Deploy

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Tell the dashboard where your relay is
NEXT_PUBLIC_WS_URL=ws://YOUR_SERVER_IP:8080 \
NEXT_PUBLIC_API_URL=http://YOUR_SERVER_IP:8080 \
docker compose up -d
```

When distributing the agent to VMs, update `DEFAULT_RELAY_URL` in `install-agent.sh` to point at your relay.

### Useful commands

```bash
docker compose logs -f              # watch relay and dashboard logs
docker compose down                 # stop everything
git pull && docker compose up --build -d  # update to latest
curl http://YOUR_IP:8080/api/health # check relay is up
```

---

## Managing the agent

```bash
# Watch live logs
sudo journalctl -u infracanvas-agent -f

# Check status / restart / stop
sudo systemctl status infracanvas-agent
sudo systemctl restart infracanvas-agent
sudo systemctl stop infracanvas-agent

# Find the pair code if you missed it
sudo journalctl -u infracanvas-agent -n 300 | grep -v "deprecated" | grep "Pair code"

# Point the agent at a different relay
sudo nano /etc/infracanvas/agent.env
sudo systemctl restart infracanvas-agent
```

**Install with a custom relay URL:**
```bash
INFRACANVAS_BACKEND_URL=ws://your-relay:8080 \
  curl -fsSL .../install.sh | bash
```

---

## Building from source

**Requirements:** Go 1.21+, Node.js 20+

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

make build          # build the agent binary
make build-all      # cross-compile for linux/darwin × amd64/arm64
make test           # run all Go tests
docker compose up --build   # run the full stack locally
```

**Frontend only:**
```bash
cd frontend
npm install
npm run dev    # http://localhost:3000
```

### Project layout

```
InfraCanvas/
├── cmd/
│   ├── infracanvas/          # Agent CLI (infracanvas start / discover / logs …)
│   └── infracanvas-server/   # Relay server
├── pkg/
│   ├── agent/                # WebSocket agent: connect, discover, diff, exec, actions
│   ├── actions/              # Action runners: Docker, Kubernetes, Host
│   ├── discovery/
│   │   ├── docker/           # Container, image, volume, network discovery
│   │   ├── host/             # OS, CPU, memory, network interfaces
│   │   └── kubernetes/       # Full K8s resource discovery via client-go
│   ├── server/               # Relay: WebSocket broker, session store, pair codes
│   ├── orchestrator/         # Combines all discovery sources into one snapshot
│   ├── output/               # Graph builder (nodes + edges + metadata as JSON)
│   ├── relationships/        # Builds edges between entities (container→image, pod→node…)
│   ├── health/               # Health status calculator per node type
│   ├── environment/          # Detects prod/staging/dev from hostname patterns
│   └── redactor/             # Strips sensitive values from env vars before sending
├── internal/models/          # Core data models (snapshot, container, pod, host…)
├── frontend/
│   ├── app/                  # Next.js 14 app router
│   ├── components/canvas/
│   │   ├── InfraCanvas.tsx   # Main canvas: ReactFlow, toolbar, filters, export
│   │   ├── NodeDetailPanel.tsx   # Side panel: metadata, actions, env/ports/mounts
│   │   ├── LogsPanel.tsx         # Log streaming panel
│   │   ├── TerminalPanel.tsx     # xterm.js terminal
│   │   ├── GroupNode.tsx / InfraNode.tsx / GroupDrawer.tsx
│   ├── lib/
│   │   ├── wsManager.ts      # WebSocket singleton, reconnect, pub/sub
│   │   ├── layout.ts         # Dagre layout + zone grouping
│   │   └── graphPreprocess.ts
│   ├── store/vmStore.ts      # Zustand global state
│   └── types/index.ts
├── examples/                 # Example agent config and systemd service file
├── install-agent.sh          # One-command agent installer
├── uninstall-agent.sh        # Clean uninstall
├── Dockerfile.server         # Relay server image
├── frontend/Dockerfile       # Dashboard image
└── docker-compose.yml
```

---

## Roadmap

These are what we're building next. PRs welcome.

- [ ] **Kubernetes pod exec** — open a shell inside a K8s pod (the same way container exec works)
- [ ] **Rate limiting on the relay** — prevent brute-force enumeration of pair codes
- [ ] **Prometheus metrics** — expose relay and agent metrics for Grafana scraping
- [ ] **Helm chart** — deploy the relay + dashboard to a Kubernetes cluster
- [ ] **Multi-relay federation** — one dashboard, multiple relay regions
- [ ] **Mobile-responsive canvas** — usable on a phone for quick checks

---

## CI / CD

Every push runs `go build`, `go test`, and frontend lint. Releases are triggered by version tags:

```bash
git tag v0.3.0
git push origin v0.3.0
```

The release workflow cross-compiles the agent binary for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and publishes them as GitHub Release assets along with `install.sh` and `uninstall.sh`.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full guide. Quick version:

1. Open an issue before writing a large PR — saves everyone time
2. Fork → branch from `main` → PR to `main`
3. `make test` and `make lint` must pass, plus `cd frontend && npm run lint`

Issues tagged [`good first issue`](https://github.com/bytestrix/InfraCanvas/issues?q=is%3Aopen+label%3A%22good+first+issue%22) are a good place to start.

---

## License

GNU Affero General Public License v3.0 — see [LICENSE](LICENSE) for details.

**Plain English:**
- ✅ Free to use, modify, and self-host for any personal or internal company purpose
- ✅ Fork it, build on top of it, extend it — just keep your changes open source
- ❌ If you run this as a cloud service for paying customers, your modifications must be open source too

This protects the project from being taken and resold by large companies without contributing anything back to the community. Individual developers and companies using it internally are completely unaffected.
