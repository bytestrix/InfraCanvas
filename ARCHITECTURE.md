# InfraCanvas architecture

How the open-source binary is put together. Keep this in sync when you change the wire protocol or add a package.

## Processes

```
┌────────────────────────────────────────────────────────────┐
│ Browser: Next.js static export, React Flow canvas, Zustand │
└──────────────┬─────────────────────────────────────────────┘
               │ WebSocket /ws/canvas, HTTP /api/*
┌──────────────▼─────────────────────────────────────────────┐
│ infracanvas serve                                          │
│   pkg/server   relay: sessions, auth, embedded UI          │
│   pkg/agent    in-process agent for this machine           │
│   pkg/clustermgr  one virtual agent per connected cluster  │
└──────────────▲─────────────────────────────────────────────┘
               │ WebSocket /ws/agent (outbound from each VM)
┌──────────────┴─────────────────────────────────────────────┐
│ infracanvas start   on other VMs (join token)              │
└────────────────────────────────────────────────────────────┘
```

Everything, including the local machine's own agent and each Kubernetes cluster, talks to the relay through the same `/ws/agent` protocol. The local agent and cluster agents just dial `ws://127.0.0.1:<port>` from inside the process.

## Commands (`cmd/infracanvas/cmd/`)

| Command | Purpose |
|---|---|
| `serve` | relay + local agent + embedded UI in one process; optional Cloudflare tunnel |
| `start` | agent only, streams to a hub (`--backend`, `--token`) |
| `agent` | older agent-only entry point |
| `discover` | one-shot discovery, prints JSON |
| `get`, `export`, `logs` | query a machine from the CLI |
| `url`, `diagnose`, `version` | helpers |

## `serve` startup

1. Bind `127.0.0.1:7777` (tunnel or `--private`) or `0.0.0.0:7777` (`--no-tunnel`), falling back to a free port.
2. Mount the embedded dashboard behind the UI token.
3. Start `clustermgr` and reconnect clusters saved in the state directory.
4. Start the Cloudflare quick-tunnel unless disabled, and print the URL.
5. Run the local agent in a restart loop.

## Wire protocol

Every message is a JSON envelope `{"type": "...", "data": {...}}`.

| Direction | Type | Meaning |
|---|---|---|
| agent → relay | `HELLO` | hostname, scope, machine ID, resume secret |
| agent → relay | `GRAPH_SNAPSHOT` | full graph; cached for browsers that attach later |
| agent → relay | `GRAPH_DIFF` | changes since the last graph |
| agent → relay | `DISCOVERY_ERROR` | discovery failed before any snapshot; cached until one arrives |
| agent → relay | `ACTION_RESULT`, `ACTION_PROGRESS` | outcome of a browser action |
| agent → relay | `LOG_DATA`, `EXEC_DATA`, `EXEC_END` | log and terminal streams |
| agent → relay | `HEARTBEAT` | keepalive every 15s |
| relay → agent | `PAIR_CODE` | pair code, plus a resume secret on first connect |
| relay → agent | `ACTION_REQUEST`, `COMMAND` | browser-triggered work |
| relay → agent | `EXEC_START`, `EXEC_INPUT`, `EXEC_RESIZE`, `EXEC_END` | terminal sessions |
| browser → relay | `PAIR`, `BROWSER_ACTION` | attach to a machine, trigger an action |
| relay → browser | `AGENT_CONNECTED`, `AGENT_DISCONNECTED`, `ERROR` | machine state |

Sessions are keyed by machine ID. A reconnect that claims an existing machine ID must present that machine's resume secret or it's rejected.

## Agent (`pkg/agent/`)

- One goroutine reads from the relay; the main loop handles commands and heartbeats.
- Discovery runs in its own goroutine every `--refresh` seconds (default 30), so a slow pass never delays terminal input. Passes are serialized.
- The first successful pass is sent as `GRAPH_SNAPSHOT`, later ones as `GRAPH_DIFF`.
- Terminal input is written to each session's PTY by one goroutine per session, which keeps keystrokes in order.

## Discovery (`pkg/discovery/`, `pkg/orchestrator/`)

| Scope | Source | Produces |
|---|---|---|
| `host` | `/proc`, systemd, PM2 | host resources, services, processes, `CONNECTS_TO` edges from `/proc/net/tcp` |
| `docker` | Docker or Podman socket | containers, images, networks, volumes |
| `lxd` | LXD / Incus socket | containers |
| `kubernetes` | in-cluster config, `$KUBECONFIG`/`~/.kube/config`, or an uploaded kubeconfig | nodes, namespaces, workloads, pods, services, ingresses, storage, events |

The orchestrator runs the scopes in parallel. A failing scope is recorded in the snapshot's `errors` and the other scopes still ship. Inside Kubernetes, a resource kind the credential can't list (HTTP 403) is skipped and listed in `permission_issues`.

After discovery: `pkg/relationships` builds edges, `pkg/health` sets node health, `internal/redactor` masks secrets, and `pkg/output` turns it into the graph the UI draws.

## Actions (`pkg/actions/`)

Browser `BROWSER_ACTION` → relay → `ACTION_REQUEST` → agent.

- Docker / Podman / LXD: start, stop, restart, exec terminal, logs, update image
- Kubernetes: scale, rolling restart, update image, delete pod, exec, logs, port-forward
- Host: PTY shell, systemd start/stop, kill process, journal logs

The relay rejects actions, terminals and cluster changes in `--read-only` mode, and per cluster when that cluster is marked read-only. Every action is written to the audit log (`pkg/audit`).

## Frontend (`frontend/`)

- `lib/wsManager.ts`: one WebSocket per machine, reconnect with exponential backoff (1s to 30s, ±20% jitter)
- `store/vmStore.ts`: Zustand store, `applyVMDiff()` merges diffs and skips empty ones
- `lib/graphPreprocess.ts`, `lib/layout.ts`: graph to React Flow nodes and layout
- `components/canvas/`: canvas, node detail panel, terminal (xterm.js), logs

The dashboard is built with `next build` as a static export and embedded into the Go binary with the `embed_full` build tag (`pkg/webui`).

## State on disk

Under `INFRACANVAS_STATE_DIR` (default `/var/lib/infracanvas` or the user cache dir): machine ID, resume secret, connected clusters and their kubeconfigs (`0600`), audit log.
