# Changelog

All notable changes to InfraCanvas are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [0.13.0] — 2026-07-07

### Added
- **LXC / LXD / Incus discovery.** Containers and VMs managed by LXD or Incus are auto-discovered from the local unix socket (snap LXD, deb LXD, and Incus paths) and drawn on the canvas alongside Docker and Kubernetes — name, status, type, and network, no config and no external dependencies. Instances render as container nodes carrying a `runtime` (`lxd`/`incus`) label.
  - Wired into the continuous agent: a new `lxd` collection tick keeps LXD instances live in the graph (previously LXD was only picked up by one-shot `discover`).
  - `infracanvas serve` includes `lxd` in its default discovery scope; the cloud agent (`infracanvas start`) auto-appends `lxd` when an LXD/Incus socket is present, so no scope change is needed at install time.

---

## [0.12.1] — 2026-06-11

### Fixed
- Service and process nodes from v0.12.0 arrived in the graph but were never displayed: the `services` filter key was missing from the default active set and the hardcoded filter-chip rows, and Overview had no tiles for them. Canvas now shows Services/Processes group cards and a Services chip; Overview gains "System Services" and "Processes" tiles.

---

## [0.12.0] — 2026-06-11

### Added
- **Systemd services and processes on the canvas.** Plain VMs — nginx + postgres + node via systemd or PM2, no Docker, no Kubernetes — now render their real workloads instead of an empty canvas:
  - Services with listening ports (postgres, nginx, redis…), failed units, and critical services appear as nodes; ports are aggregated from each service's process tree via a single-pass `/proc/net/tcp` + `/proc/<pid>/fd` socket scan.
  - Standalone listening processes (PM2 apps, dev servers) appear as their own nodes.
  - `CONNECTS_TO` edges drawn from established local TCP connections — e.g. `next-server → postgres :5432` — giving real service topology on a bare VM.
  - New actions: service start/stop/restart, process terminate (SIGTERM) and force-kill (SIGKILL, refuses pid ≤ 1 and the agent itself).
  - Service journalctl logs in the Logs panel (`host_logs` with unit scoping).
  - "Services" filter chip, dedicated icons, and detail-panel metadata (unit, ports, main PID, restart count).
- Zombie processes are marked degraded (state `Z` in `/proc/<pid>/stat`).

---

## [0.11.0] — 2026-06-10

### Added
- Read-only mode for public demos: `infracanvas serve --read-only`, installer flag `--read-only`, and `INFRACANVAS_READONLY=true` in `config.env`. The relay blocks every action and terminal request server-side before it reaches the agent; the topology graph and log viewing still work. Blocked actions surface as normal action errors in the UI, blocked terminals print a short notice and close cleanly, and the sidebar shows a "read-only demo" badge.
- Late-joining browsers now receive `AGENT_CONNECTED` (hostname, scope, read-only flag) immediately on pairing instead of waiting for the next agent event.

### Changed
- `README.md`: added a "How is this different from Portainer, Lazydocker, or Dozzle?" comparison; compressed the demo gif 7.2 MB → 3.0 MB.

---

## [0.4.2] — 2026-04-26

### Fixed
- Installer no longer silently runs the systemd service as `root` when `$SUDO_USER` is unset (e.g. when invoked from a root shell, `sudo -i`, `sudo su -`, cloud-init, or some piped `curl | sudo bash` configurations). Running as root meant `~/.kube/config` was empty and Kubernetes discovery was a no-op for users who hit this path.

### Changed
- `install-agent.sh` now picks the service user via a cascade: `--run-user` flag → `$SUDO_USER` → first user in `/home/*` with a readable `~/.kube/config` → first user in the `docker` group → first user with a real login shell → `root`. The chosen user is printed during install.
- New `--run-user <user>` flag for explicit override.

---

## [0.4.1] — 2026-04-26

### Removed
- "OSS vs hosted" section and table from `README.md`. The hosted SaaS doesn't exist yet; promising features that don't ship was misleading. The repo now describes only what's actually shipped.
- "brew install … (coming soon)" line from `README.md` quick start.
- Stale "legacy/SaaS" annotations in docs.

### Changed
- `uninstall-agent.sh` now cleans up the bundled `cloudflared` binary cached under `~/.cache/infracanvas/` (~30 MB) for `$SUDO_USER` and any other home directories.
- `README.md` `Uninstall` section expanded to list exactly what gets removed and document the local `./uninstall-agent.sh` path for repo clones.

---

## [0.4.0] — 2026-04-26

Major UX overhaul. The OSS flow is now **one binary on each VM**, exposed through a free Cloudflare quick-tunnel. The installer prints a public `https://*.trycloudflare.com` URL — no Docker, no laptop relay, no pair codes, no firewall change.

### Added
- `infracanvas serve` (default command) — boots relay, embedded dashboard, and in-process agent on a single port.
- **Cloudflare quick-tunnel by default**: `pkg/tunnel` manages a `cloudflared` child process and prints a public HTTPS URL. The binary downloads `cloudflared` on first run (Linux); on macOS install via `brew install cloudflared`. `--no-tunnel` disables it and binds the port directly. `--private` implies `--no-tunnel` and binds `127.0.0.1`.
- Random per-install UI auth token, stored in `/etc/infracanvas/config.env`. Token is required via `?token=` query param on first load; subsequent requests use an HTTP-only cookie.
- Static-export Next.js dashboard embedded into the Go binary via `go:embed` under build tag `embed_full`. Default builds embed a placeholder so plain `go build` works without a Node toolchain.
- Install-script port preflight, binary self-test, systemd unit verification, and tunnel-URL extraction from `journalctl` (filtered by restart timestamp) for the final banner.
- Installer auto-detects `$SUDO_USER` and writes `User=`/`Group=` into the systemd unit so the agent runs as the invoking user — Kubernetes discovery picks up `~/.kube/config` automatically, and `SupplementaryGroups=docker` is added when the user is in the `docker` group.
- `make all`, `make build-frontend`, `make build-stub` targets.

### Changed
- Frontend simplified to a single auto-connecting dashboard (no `Connect VM` modal, no per-VM cards) — one VM per dashboard.
- Relay supports `LocalMode`: browser WS auto-binds to the in-process agent without a `PAIR` exchange.
- `agent.env` → `config.env`; service unit renamed from `infracanvas-agent` to `infracanvas` (the installer migrates the legacy unit).
- `install-agent.sh` rewritten: drops the relay-URL config, adds `--port`, `--no-tunnel`, `--private`, `--version`.

### Removed
- `cmd/infracanvas-server/` (standalone relay binary — folded into `serve`).
- `Dockerfile.server`, `docker-compose.yml`, `docker-compose.prod.yml`, `frontend/Dockerfile`, `Caddyfile`, `Caddyfile.prod`, `.dockerignore`, `.env.example` — all artifacts of the old laptop-relay model.
- `frontend/components/ConnectModal.tsx`, `frontend/components/VMCard.tsx`, `frontend/app/vm/[code]/page.tsx`.
- `start.sh`, `examples/agent-config.yaml`, `examples/infracanvas-agent.service`.

### Migration
The installer detects the legacy `infracanvas-agent` systemd unit and removes it before installing `infracanvas`. Re-run the install one-liner to upgrade.

---

## [0.3.0] — 2026-04-19

### Added
- Bytestrix purple/fuchsia UI palette with Catppuccin Mocha terminal theme
- Container detail panel: environment variables with secret masking toggle, port mappings, volume mounts, image metadata
- `frontend/.env` committed with public relay URL so cloned repos work out of the box
- `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, `LICENSE`
- GitHub issue and PR templates

### Fixed
- Container terminal: blank screen fixed by importing `@xterm/xterm/css/xterm.css` in layout
- Container terminal: double output fixed by disabling React StrictMode
- Container terminal: colors and readline fixed by setting `TERM=xterm-256color` in exec env
- Container terminal: real PTY resize via ResizeObserver wired correctly
- Container exec: `No such container: container:XXXX` fixed by stripping `container:` prefix in `normalizeEntityID`
- Port mappings: camelCase field names (`hostPort`, `containerPort`) now match backend output
- Kubernetes discovery: `KUBECONFIG` propagated to systemd service via agent env file
- Pair codes now generated with `crypto/rand` — not time-seeded or guessable
- Pair code entropy increased: `WORD-WORD-NNNNNN` format (~1.44B combinations vs ~4.5M before)

### Changed
- Removed stale dev-only scripts (`deploy-*.sh`, `connect-vm.sh`, `test_e2e.sh`, `test_security.sh`) — gitignored
- Removed redundant docs (`AZURE_VM_SETUP.md`, `DEVOPS_*.md`, `QUICKSTART.md`, etc.) — gitignored
- Removed internal implementation notes from tracked files
- CI now validates Go linting and frontend build
- README: added Security section, Roadmap, badges, known limitations

---

## [1.0.0] — 2026-04-17

### Added
- Real-time WebSocket relay: agent → relay → browser
- One-command agent installer (`install.sh`) with systemd integration
- Grouped and flat canvas views using ReactFlow
- Filter chips with spotlight mode and right-click to hide
- Container actions: restart, stop, start, update image
- Container logs viewer with ERROR/WARN/INFO color-coding
- Interactive container terminal (`docker exec` via xterm.js)
- VM shell terminal (host PTY via xterm.js)
- Kubernetes full resource graph (cluster → pods → services → ingress → PVCs)
- Kubernetes actions: rollout restart, scale, update image, get logs
- Host discovery: OS, CPU, memory, cloud provider detection
- Health colors: healthy/degraded/unhealthy/unknown per node
- Critical alert banner for degraded groups
- Export canvas as PNG or JSON
- Multi-VM dashboard with per-VM status cards
- Secret redaction before data leaves the VM
- Optional shared auth token between agent and relay
- Docker Compose setup for relay + dashboard
- Production Docker Compose with Caddy reverse proxy and TLS
- Cross-platform agent builds: linux/darwin × amd64/arm64
- GitHub Actions CI (build + test) and release workflow
