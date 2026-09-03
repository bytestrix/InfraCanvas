# InfraCanvas Architecture

> Living document. Update as you build. Replaces codebase re-analysis each session.

---

## Overview

InfraCanvas is a multi-VM infrastructure discovery and visualization platform. Two distinct products share this repo:

- **OSS** (`infracanvas` CLI): self-hosted, single-binary, Cloudflare tunnel
- **SaaS** (`saas-backend/` + SaaS frontend): multi-tenant, hosted on `bytestrix` VM

---

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      BROWSER                            │
│  Next.js 14 SPA: React Flow canvas, Zustand state       │
└──────────────┬──────────────────────────────────────────┘
               │ WebSocket /ws/canvas   HTTP /api/v1/*
               │
┌──────────────▼──────────────────────────────────────────┐
│              RELAY / API SERVER (bytestrix)              │
│                                                         │
│  OSS mode:  pkg/server  (:8080, in Docker container)    │
│  SaaS mode: saas-backend/pkg/api  (:8090, systemd)      │
│                                                         │
│  Postgres (port 5432)  Redis (port 6379)                │
└──────────────┬──────────────────────────────────────────┘
               │ WebSocket /ws/agent
               │
┌──────────────▼──────────────────────────────────────────┐
│              AGENT  (on each VM)                        │
│  pkg/agent: discovers host/docker/k8s, streams diffs    │
└─────────────────────────────────────────────────────────┘
```

---

## OSS Stack

### Binary: `infracanvas` (cmd/infracanvas/)

| Subcommand | Purpose |
|---|---|
| `serve` | All-in-one: relay + agent + embedded UI in one process |
| `agent` | Agent-only, connects to external relay |
| `discover` | One-shot discovery, prints JSON |
| `get` / `export` | Query running instance |
| `logs` | Stream container logs |
| `url` / `diagnose` | Operational helpers |

### Serve mode flow

1. Bind HTTP on `127.0.0.1:7777` (with tunnel) or `0.0.0.0:7777` (no-tunnel)
2. Start Cloudflare quick-tunnel → print public URL with `?token=<ui_token>`
3. Start in-process agent connecting to `ws://127.0.0.1:7777/ws/agent`
4. Browser auto-pairs (local mode, no pair code needed)

### OSS Relay Server (`pkg/server/`)

**Wire protocol**: all messages use `{"type": "...", "data": {...}}` JSON envelope.

| Direction | Message | Meaning |
|---|---|---|
| Agent → Server | `HELLO` | hostname + scope |
| Agent → Server | `GRAPH_SNAPSHOT` | full node/edge graph |
| Agent → Server | `GRAPH_DIFF` | incremental changes |
| Agent → Server | `ACTION_RESULT` | action completed |
| Agent → Server | `HEARTBEAT` | keepalive |
| Server → Agent | `PAIR_CODE` | assigned pair code |
| Server → Agent | `ACTION_REQUEST` | browser-triggered action |
| Browser → Server | `PAIR` | connect using pair code |
| Browser → Server | `BROWSER_ACTION` | trigger action on agent |
| Server → Browser | `AGENT_CONNECTED` | agent metadata |
| Server → Browser | `GRAPH_SNAPSHOT` / `GRAPH_DIFF` | relayed from agent |

**Session lifecycle:**
- Agent connects → server creates `Session` (UUID + pair code e.g. `DUSK-DEER-755033`)
- Browser sends `PAIR {code}` → bound to session
- Last snapshot cached for late-joining browsers
- Local mode: browser auto-binds, no pair code needed

### Agent (`pkg/agent/`)

- `WSAgent` maintains WebSocket connection to relay
- On connect: sends `HELLO`, gets `PAIR_CODE`
- Every `RefreshSeconds`: runs orchestrator → computes diff vs previous snapshot → sends `GRAPH_DIFF`
- On first run or reconnect: sends full `GRAPH_SNAPSHOT`
- Handles `ACTION_REQUEST` from browser (docker stop/start, k8s ops, exec sessions, log streaming)

### Discovery (`pkg/discovery/`)

Three scopes, discovered independently:

| Scope | Source | Key data |
|---|---|---|
| `host` | `/proc`, `sysinfo` | CPU, memory, disk, network interfaces |
| `docker` | Docker socket | containers, images, networks, volumes |
| `kubernetes` | `~/.kube/config` or in-cluster | pods, deployments, services, namespaces |

Output: `GraphOutput {nodes: GraphNode[], edges: GraphEdge[]}` from `pkg/output/`

### Actions (`pkg/actions/`)

Triggered by browser via `BROWSER_ACTION` → relay → `ACTION_REQUEST` on agent:

- **docker**: start/stop/restart container, pull image, exec into container
- **host**: exec shell command, PTY terminal session
- **kubernetes**: scale deployment, update image, rollout restart

Exec sessions: agent opens PTY or Docker exec, streams stdin/stdout via `EXEC_DATA` messages.

---

## SaaS Stack

### Backend: `saas-backend/` (binary: `infracanvas-api`, port 8090)

**Packages:**
- `pkg/api/auth/`: JWT auth, registration, login, forgot-password
- `pkg/api/agents/`: agent registry, list agents per org
- `pkg/api/websocket/`: SaaS relay (agent ↔ browser via org/agent ID)
- `pkg/api/organizations/`: org management, API keys
- `pkg/api/billing/`: billing integration
- `pkg/middleware/`: JWT validation, org context
- `pkg/database/`: Postgres connection (pg driver)
- `pkg/models/`: User, Org, Agent, APIKey structs

**Auth flow:**
- Register/login → JWT
- API keys for agent authentication (`/api/v1/org/api-keys`)
- Agent connects with API key → registered under org
- Browser connects with JWT → sees agents for their org

**SaaS relay flow (different from OSS):**
- Agent authenticates with org API key
- `[relay/agent] HELLO agent=ByteStrix-5bcfabe8 org=bytestrix-657a9cc0`
- Browser connects → sees all agents in org
- Relay bridges browser ↔ specific agent by agent ID (not pair code)

### SaaS Frontend (`~/infracanvas-saas-frontend/` on bytestrix, separate repo from OSS)

Next.js 14 App Router SPA. Deployed as dev server on `:3001`.

**Key routes** (`frontend/app/`):
- `/` → redirect to `/dashboard`
- `/auth/login`, `/auth/register`, `/auth/forgot-password`
- `/dashboard`: main canvas page

**State management** (`frontend/store/vmStore.ts`):
- Zustand store: `vms: Record<code, VMState>`
- VMState: `{status, hostname, scope, graph, error, lastUpdated}`
- `applyVMDiff()`: merges incremental diffs into graph state

**WebSocket client** (`frontend/lib/wsManager.ts`):
- Module-level singleton (survives React navigation)
- Exponential backoff reconnect (1s → 30s, ±20% jitter)
- Derives WS URL from `window.location` (same-origin `/ws/canvas`)
- OSS mode: sends `PAIR {code: "local"}` for auto-pair
- SaaS mode: sends auth token, then selects agent by ID

**Canvas rendering** (`frontend/components/canvas/`):
- React Flow for graph layout
- `frontend/lib/graphPreprocess.ts`: transforms `GraphOutput` → React Flow nodes/edges
- `frontend/lib/layout.ts`: hierarchical layout algorithm

---

## Infrastructure on `bytestrix` VM (13.200.198.166)

### Processes

| Process | Port | Binary |
|---|---|---|
| `infracanvas-server` (Docker) | 8080 | OSS relay server |
| `infracanvas-api` (systemd) | 8090 | SaaS backend |
| Next.js dev server | 3001 | SaaS frontend |
| Postgres | 5432 | DB for SaaS |
| Redis | 6379 | Sessions |
| nginx | 3000 | Proxy → 3001 + 8090 |

### Nginx routing (port 3000, working dev URL)

```
/ws/   → localhost:8090  (SaaS relay WebSocket)
/api/  → localhost:8090  (SaaS REST API)
/      → localhost:3001  (Next.js frontend)
```

**Port 80 is hijacked by k8s iptables DNAT** → do not use port 80 for InfraCanvas.

### Dev URL

`http://13.200.198.166:3000`

### Agent connection (SaaS)

Agent on bytestrix connects to `localhost:8090` (same VM). Registered as `ByteStrix-5bcfabe8` under org `bytestrix-657a9cc0`. Install script downloaded from `/api/v1/install/binary/linux-amd64`.

---

## Key Data Models

### GraphNode

```typescript
{
  id: string          // e.g. "/container:abc123"
  type: string        // "host" | "container" | "pod" | "service" | ...
  label: string
  layer: string       // "host" | "docker" | "kubernetes"
  metadata: Record<string, any>
}
```

### GraphEdge

```typescript
{
  id: string
  source: string      // node id
  target: string      // node id
  type: string        // "runs" | "connects" | "exposes" | ...
}
```

### GraphDiff

```typescript
{
  timestamp: string
  addedNodes: GraphNode[]
  modifiedNodes: GraphNode[]
  removedNodeIds: string[]
  addedEdges: GraphEdge[]
  removedEdgeIds: string[]
}
```

---

## OSS vs SaaS Comparison

| | OSS (`infracanvas serve`) | SaaS |
|---|---|---|
| Auth | UI token (random hex) | JWT + API keys |
| Multi-VM | No (one relay = one VM) | Yes (org → many agents) |
| Pairing | Pair code or auto (local) | Agent ID selection |
| Hosting | User's own machine | bytestrix VM |
| Tunnel | Cloudflare quick-tunnel | Direct (nginx proxy) |
| DB | None | Postgres + Redis |

---

## Development Workflow

```bash
# Local dev (OSS): run everything in one command
./bin/infracanvas serve

# SaaS frontend dev (separate repo on bytestrix)
# ssh bytestrix, then:
cd ~/infracanvas-saas-frontend && npm run dev -- --port 3001
# OR managed by systemd:
sudo systemctl restart infracanvas-frontend

# SaaS backend dev (on bytestrix)
sudo systemctl restart infracanvas-api
sudo journalctl -u infracanvas-api -f

# Check agent connected
curl http://localhost:8090/api/v1/agents -H "Authorization: Bearer <key>"
```
