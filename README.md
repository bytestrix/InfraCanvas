# InfraCanvas

**A live, visual map of everything running on a server — installed with one command.**

[![CI](https://github.com/bytestrix/InfraCanvas/actions/workflows/ci.yml/badge.svg)](https://github.com/bytestrix/InfraCanvas/actions/workflows/ci.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
[![Go 1.21+](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org/)

InfraCanvas is a single Go binary you run on any Linux machine. It discovers every container, pod, volume, network, and deployment on that host and serves a live visual dashboard you open in your browser. No Docker required, no extra services to host, no setup.

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

That's the whole installation. The installer opens a free Cloudflare [quick-tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/do-more-with-tunnels/trycloudflare/) and prints a public `https://*.trycloudflare.com` URL — paste it into any browser and you see the dashboard. No firewall change, no signup, no SSH.

---

## How it works

```
       ┌─────────────────────────────────────────┐
       │  your-vm                                │
       │                                         │
       │   ┌────────────────────────────────┐    │
       │   │  infracanvas (single binary)   │    │
       │   │   ├── discovery agent          │    │
       │   │   ├── WebSocket relay          │    │
       │   │   └── embedded dashboard UI    │    │
       │   └────────────────────────────────┘    │
       │            ▲ 127.0.0.1:7777             │
       │            │                            │
       │   ┌────────┴───────┐                    │
       │   │  cloudflared   │  outbound only     │
       │   └────────┬───────┘                    │
       └────────────┼────────────────────────────┘
                    │  Cloudflare quick-tunnel
                    ▼
              ┌──────────┐
              │  laptop  │  →  https://xyz.trycloudflare.com
              └──────────┘
```

One binary, one URL. The dashboard, relay, and agent all run in the same process on the machine you're inspecting. A bundled `cloudflared` opens an outbound-only tunnel to Cloudflare's edge, which gives you a public HTTPS URL with no inbound firewall rule. Your laptop is just a browser.

Prefer to expose the port directly? Pass `--no-tunnel` to bind `0.0.0.0:7777` (you'll need to allow inbound TCP in your cloud security group). Add `--private` to bind `127.0.0.1` instead and reach the dashboard via SSH tunnel.

---

## Quick start (any Linux VM)

```bash
ssh user@your-vm
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

Output ends with something like this:

```
✓ InfraCanvas installed and running

  Open in your browser:
    https://shy-pine-2f1a.trycloudflare.com/?token=a8f3e2b1c9d4f02e

  This URL works from anywhere — Cloudflare's free quick-tunnel needs no
  firewall rule. The URL is ephemeral; it changes whenever the service
  restarts. Run with --no-tunnel for a stable URL on your own port.

  Auth token:  a8f3e2b1c9d4f02e  (saved in /etc/infracanvas/config.env)
```

Open the URL, and you see your VM's infrastructure live.

### Run multiple VMs

Each VM is independent. Install on each, open the printed URL for each in a separate tab — no tunnel coordination needed. The binary is intentionally one-VM-per-dashboard.

### Install options

```bash
# Custom port (default 7777) — only matters with --no-tunnel
curl -fsSL .../install.sh | bash -s -- --port 8888

# Skip Cloudflare tunnel; bind 0.0.0.0:7777 directly (open the port in your SG)
curl -fsSL .../install.sh | bash -s -- --no-tunnel

# Bind 127.0.0.1 only; reach via SSH tunnel (implies --no-tunnel)
curl -fsSL .../install.sh | bash -s -- --private

# Pin a specific release
curl -fsSL .../install.sh | bash -s -- --version v0.4.0
```

### Run on your laptop too

The same binary works locally — build from source (see [Building from source](#building-from-source)), then:

```bash
infracanvas serve
# → https://*.trycloudflare.com/?token=…   (or pass --no-tunnel for http://localhost:7777)
```

You'll see your laptop's Docker containers and Kubernetes context (if any) on the canvas. Useful for development and demos.

---

## What you get

- **Live topology graph** — every container, pod, service, volume, network drawn as connected nodes; updates every 30s.
- **Health at a glance** — green/amber/red based on real container/pod state, alert banner when something is unhealthy.
- **Container terminal** — open a shell inside any running container, in the browser.
- **VM shell** — terminal on the host itself.
- **Container logs** — color-coded, downloadable.
- **Kubernetes actions** — rolling restart, scale, update image, all from the UI.
- **Docker actions** — restart, stop, start, update image.
- **Inspect everything** — env vars (with secrets masked), port mappings, volume mounts, image details.
- **Export** — PNG screenshot or full JSON of the graph.

See the [features section](#all-features) for the full list.

---

## Security model

The dashboard, relay, and agent all run on the same machine, so there's no remote agent ↔ relay channel to secure. The two surfaces that matter:

**1. The exposed URL.** Default mode binds `127.0.0.1:7777` and exposes it through a Cloudflare quick-tunnel — outbound-only from your VM, HTTPS-terminated at Cloudflare's edge. Anyone with the URL+token hits the dashboard. The URL is unguessable (random subdomain) but not secret — pair it with the auth token below. With `--no-tunnel` the binary binds `0.0.0.0` directly and anyone who reaches the IP/port hits the dashboard. With `--private`, it binds `127.0.0.1` only and you reach it via SSH tunnel.

**2. The auth token.** Every install generates a random 24-character token (printed once, saved to `/etc/infracanvas/config.env`). The dashboard requires it on first visit (`?token=…`); after that it lives in an HTTP-only cookie. WebSocket calls also require the token. Without the token, every request returns `401`.

**What the dashboard can do once authenticated:**
- See the full topology of this machine
- View container logs
- Open a shell inside any container, or on the host
- Run Docker and Kubernetes actions (restart, scale, update image)

Treat the URL+token like an SSH key for the box. Anyone with both has the same effective power.

**Secret redaction.** Environment variables whose names contain `SECRET`, `TOKEN`, `KEY`, `PASSWORD`, `CREDENTIAL`, `AUTH`, or `PASSWD` are replaced with `[REDACTED]` before they leave the discovery layer. File contents, database contents, and network traffic are never touched by InfraCanvas.

**Service runs as you, not root.** When you install via `sudo …/install.sh`, the systemd unit is written with `User=$SUDO_USER` (and `Group=$SUDO_USER`). The agent inherits *your* `~/.kube/config` automatically — Kubernetes discovery just works for whatever cluster you can already `kubectl` against. If you're a member of the `docker` group, `SupplementaryGroups=docker` is added so Docker discovery works without sudo. Falling back to `root` only happens when there's no invoking user (rare). Net effect: no privilege escalation beyond what you can already do at the shell.

---

## Managing the service

```bash
sudo systemctl status   infracanvas
sudo systemctl restart  infracanvas
sudo systemctl stop     infracanvas
sudo journalctl -u infracanvas -f
```

Config lives in `/etc/infracanvas/config.env`:

```bash
INFRACANVAS_UI_TOKEN=a8f3e2b1c9d4f02e
INFRACANVAS_PORT=7777
INFRACANVAS_TUNNEL=true
INFRACANVAS_PRIVATE=false
```

Edit, then `sudo systemctl restart infracanvas`.

### Uninstall

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash
```

The uninstaller stops and disables the systemd service, then removes:

- `/usr/local/bin/infracanvas` — the binary
- `/etc/systemd/system/infracanvas.service` — the unit
- `/etc/infracanvas/` — config and auth token
- `~/.cache/infracanvas/` — the bundled `cloudflared` binary (~30 MB), for the user the service ran as

If you cloned this repo, you can also run it locally:

```bash
sudo ./uninstall-agent.sh
```

---

## All features

### Canvas
| Feature | What it does |
|---|---|
| Live topology graph | Every container, pod, service, volume, network drawn as nodes with edges showing relationships |
| Real-time updates | Full snapshot on first connect, then only changes every 30s |
| Grouped view | Nodes grouped by type (Containers, K8s Workloads, Storage…) — one card per group, click to expand |
| Flat view | Every node laid out individually by type and relationship |
| Filter chips | Show/hide Kubernetes, Docker, Host, Pods, Storage, Events |
| Health colors | Green = healthy, amber = degraded, red = unhealthy |
| Alert banner | Appears automatically when any group has unhealthy nodes |
| Export PNG | Save the canvas as a high-res image |
| Export JSON | Download the raw graph (nodes, edges, metadata) |

### Containers and Docker
| Feature | What it does |
|---|---|
| Container terminal | Full interactive shell inside any container |
| Container logs | Last 200 lines, color-coded, downloadable |
| Restart / Stop / Start | Run from the UI — executed by the in-process agent |
| Update image | Set a new image tag and the agent pulls and recreates |
| Environment variables | Shown with automatic secret masking |
| Port mappings | Host ↔ container port pairs |
| Volume mounts | Bind mounts and named volumes with paths |
| Image details | Registry, tag, size, digest, which containers use it |

### Kubernetes
| Feature | What it does |
|---|---|
| Full resource graph | Cluster → Nodes → Namespaces → Deployments → Pods → Services → Ingress → PVCs |
| Health from pod phase | Running/Pending/Failed → green/amber/red |
| Rolling restart | `kubectl rollout restart` for Deployments, StatefulSets, DaemonSets |
| Update image | Change the image for any Deployment |
| Scale | Change replica count for Deployments and StatefulSets |
| Pod logs | Fetch logs from any pod |
| K8s events | Shown as nodes linked to the resources they affect |

### Host
| Feature | What it does |
|---|---|
| VM terminal | Interactive shell on the host (not inside a container) |
| Host info | OS, kernel, CPU cores, memory, hostname |
| Cloud detection | Identifies AWS / GCP / Azure / on-prem |
| Environment detection | Infers prod/staging/dev from hostname patterns |

---

## Building from source

**Requirements:** Go 1.21+, Node.js 20+

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

make all                # build dashboard + binary (with embedded UI)
./bin/infracanvas       # → http://localhost:7777/?token=…
```

Other useful targets:

```bash
make build-frontend     # build the dashboard, embed under pkg/webui/dist/
make build              # build binary with embedded dashboard (requires dist/)
make build-stub         # build with placeholder UI — fastest, for backend iteration
make release            # cross-compile for linux/darwin × amd64/arm64
make test               # run all Go tests
make clean              # remove bin/ and embedded dashboard
```

### Project layout

```
InfraCanvas/
├── cmd/infracanvas/cmd/
│   ├── serve.go              # `infracanvas serve` — boots relay + UI + agent
│   ├── start.go              # `infracanvas start` — agent-only mode
│   ├── discover.go           # one-shot CLI discovery
│   └── …
├── pkg/
│   ├── agent/                # WebSocket agent: discover, diff, exec, actions
│   ├── server/               # Relay: WebSocket broker, sessions, auth, static UI
│   ├── webui/                # Embedded dashboard (build-tagged)
│   │   ├── embed_full.go     # `-tags embed_full` → embeds dist/
│   │   ├── embed_stub.go     # default → embeds placeholder/
│   │   ├── placeholder/      # tracked: dev stub
│   │   └── dist/             # gitignored: generated by `make build-frontend`
│   ├── actions/              # Docker / K8s / Host action runners
│   ├── discovery/            # docker, host, kubernetes
│   ├── orchestrator/         # combines discovery sources into one snapshot
│   ├── output/               # graph builder
│   ├── relationships/        # edges between entities
│   ├── health/               # health status calculation
│   └── redactor/             # strips sensitive env vars
├── frontend/
│   ├── app/page.tsx          # single-VM dashboard, auto-connects on mount
│   ├── components/canvas/    # ReactFlow canvas, node detail panel, terminal, logs
│   ├── lib/wsManager.ts      # WS client, same-origin
│   └── store/vmStore.ts      # Zustand state
├── install-agent.sh          # one-command installer
└── uninstall-agent.sh
```

### Releasing

Tag a version; the workflow cross-compiles, embeds the dashboard, and publishes binaries.

```bash
git tag v0.5.0
git push origin v0.5.0
```

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Open an issue before a large PR. `make test` and `make lint` must pass, plus `cd frontend && npm run lint`.

Good first issues: [`good first issue`](https://github.com/bytestrix/InfraCanvas/issues?q=is%3Aopen+label%3A%22good+first+issue%22).

---

## License

GNU Affero General Public License v3.0 — see [LICENSE](LICENSE).

- ✅ Free for any personal or internal company use
- ✅ Fork, modify, redistribute — just keep changes open source
- ❌ If you run this as a paid cloud service for customers, your modifications must be open source too

This protects against large companies repackaging the project without contributing back. Individual developers and internal company use are unaffected.
