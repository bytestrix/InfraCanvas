# Changelog

All notable changes to InfraCanvas are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [0.19.3] — 2026-08-15

### Fixed
- The v0.19.2 rate-limiter IP detection took the *first* entry of a spoofed `X-Forwarded-For` header, which a client could still forge ahead of whatever a third-party reverse proxy appends — removed the `X-Forwarded-For` fallback entirely. `Cf-Connecting-Ip` (set by Cloudflare's real edge network on the default bundled tunnel) remains the sole trusted signal; anything else falls back to the direct peer address.

---

## [0.19.2] — 2026-08-15

### Fixed
Regressions found by a follow-up re-audit of v0.19.1's own changes — this is why we test before shipping, and why we re-audit after.

- **Critical.** The v0.19.1 CSP (`default-src 'self'` with no `script-src`/`style-src` override) blocked the dashboard's own Next.js hydration scripts and inline styles outright — the UI failed to render in any CSP-enforcing browser. `script-src`/`style-src` now explicitly allow `'unsafe-inline'` (the static-export dashboard has no nonce infrastructure to do better without a larger rewrite); framing protection and same-origin resource restriction are unaffected.
- **Medium.** The v0.19.1 rate limiter bucketed every visitor behind the default bundled cloudflared tunnel into a single shared IP (`127.0.0.1`, since that's how requests arrive locally), meaning anyone with the tunnel URL could lock out the legitimate operator with ~20 bad requests. Now reads `Cf-Connecting-Ip` (only when the immediate peer is loopback, i.e. genuinely our own tunnel) so real visitors are rate-limited independently.
- Failed auth attempts (bad UI token, bad agent token, bad `?token=`) are now logged with the source IP and route — previously silent until the rate limiter tripped into a 429, with no signal an attack was in progress.
- Frontend Next.js bumped 14.2.29 → 14.2.35 for consistency with the hosted product (low practical risk here — OSS ships a static export with no server runtime, so most of what 14.2.30+ fixes doesn't apply, but no reason to stay behind).

---

## [0.19.1] — 2026-08-15

### Security
Hardening pass from a full OWASP Top 10 self-review (no external report this time).

- **Removed the unused, UI-unreferenced `host_run_command` action type** — arbitrary shell execution gated only by session auth, with no allowlist. Nothing in the dashboard used it.
- **UI/agent token generation and cluster-ID generation now fail closed.** Both previously fell back to a hardcoded value (`"infracanvas"`) or a predictable timestamp if `crypto/rand` ever errored — now they refuse to start instead.
- **Constant-time comparison** for every token/secret check (UI token, agent bearer token, hub resume secret) — closes a theoretical timing side-channel across 5 call sites.
- **WebSocket frame size limits added** (32MB agent / 4MB browser) — an unbounded frame could previously exhaust memory.
- **Security headers on every response** — CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, and HSTS-when-TLS.
- **Per-IP rate limiting on every auth-checking endpoint** (20 failed attempts/minute) — previously unlimited guesses were possible against the UI token, agent token, and resume secret.
- **`install-agent.sh` now checksum-verifies its first download** against the release's published `checksums.txt`, matching the `update_agent` in-app path fixed in v0.18.1.
- kubeconfig parse-error logging now routed through the existing secret redactor as defense-in-depth.

---

## [0.19.0] — 2026-08-14

### Added
- **Per-cluster read-only mode.** Independent of the global `--read-only` flag, each connected cluster now has its own read-only toggle — set it at connect time or flip it later — so some clusters can stay view-only while others remain fully interactive on the same dashboard.
- **Permission preview before connecting.** The Add Cluster dialog now shows what a kubeconfig can actually do (view, exec, restart/kill, scale/edit, read Secrets) before you commit to connecting it, via a set of `SelfSubjectAccessReview` checks that persist nothing.
- **Audit log.** Every write action, terminal session, and read-only-blocked attempt is recorded (append-only, local file) and viewable from a new Audit tab in the dashboard, or `GET /api/audit`. Attributed by session/machine, not by user — OSS has no per-user login.

---

## [0.18.1] — 2026-08-14

### Security
Fixes six vulnerabilities responsibly disclosed via r/selfhosted. Every one was independently verified against the code before fixing.

- **Critical.** `--read-only` demo mode was never enforced on `POST`/`DELETE /api/clusters`. A kubeconfig can contain an `exec:` credential plugin that client-go runs verbatim, so a read-only public demo could be turned into arbitrary command execution as the service user. Both routes now check read-only mode.
- **High.** Hub mode's shared join token let any connected VM claim any other machine's session — `/api/sessions` (which discloses every machine ID) also accepted that same token, making the attack practical end-to-end. Fixed with a per-machine resume secret required on every reconnect; a mismatch is rejected outright instead of silently swapped in over a session with browsers already attached. `/api/sessions` now requires the UI token only.
- **High.** Shell injection via the service/unit action parameters, string-interpolated into `sh -c`. Switched to direct argv execution (no shell involved).
- **High.** The cloudflared tunnel binary was trusted from a `/tmp` fallback cache directory with no ownership check and no checksum — reachable because the installer only set `HOME` for non-root run users, so the default root install silently fell back to a world-writable path any local user could plant a binary in ahead of time. Installer now always sets `HOME`; a cached binary is only trusted if this process actually owns its directory.
- **High.** `update_agent` downloaded and installed a binary from a caller-supplied URL with zero checksum verification, despite every release already publishing `checksums.txt`. Now requires `https://`, verifies sha256 against the release's own checksums, and refuses a custom URL without an explicit hash to check it against.
- **High.** The dashboard/join token file was written world-readable (0644 in a 0755 directory), unlike every other credential file in the codebase.

---

## [0.18.0] — 2026-08-14

### Fixed
- **Mobile-responsive dashboard.** The 200px sidebar had no responsive handling at all, consuming over half a 375px viewport with no scroll fallback — now an off-canvas drawer below 720px. The canvas toolbar, side panels, terminal/logs panels, and overview tile grid all had the same class of issue (fixed widths/heights and hard column minimums with no narrower fallback) and are fixed to match.

---

## [0.17.0] — 2026-08-12

### Added
- **Clusters — connect Kubernetes via kubeconfig, zero agent install.** Drop a kubeconfig and InfraCanvas talks to your cluster's API server directly, the same way `kubectl` does — nothing installed anywhere, nothing added to the cluster, and the kubeconfig never leaves this machine. Multiple contexts in one file get a picker so you connect exactly the cluster(s) you mean to. See the [Clusters](README.md#-clusters--kubernetes-with-zero-install) section.

### Fixed
- **Terminal output no longer garbles on connect.** A PTY resize race meant the terminal could send its first `exec_start` before the initial `fit()` ran, and every subsequent resize fired unconditionally with no debounce — both are now sequenced and debounced correctly.

---

## [0.16.0] — 2026-08-05

### Added
- **Progressive disclosure on the canvas.** Large graphs now drill down host → category → type → instance instead of dumping every node at once, keeping dense clusters and multi-host views readable.

---

## [0.15.0] — 2026-08-03

### Added
- **Add machine flow, without leaving the browser.** The dashboard now generates a ready-to-run join command inline, instead of sending you to the README for the install one-liner.

---

## [0.14.0] — 2026-07-31

### Added
- **Multi-VM hub mode.** One dashboard, many VMs — agents connect outbound to a hub instance, so you can watch a whole fleet from a single `infracanvas serve` without exposing anything inbound on the boxes themselves.
- **LXD/Incus wired into the continuous agent.** The cloud agent (`infracanvas start`) now keeps LXD/Incus instances live via a dedicated collection tick and auto-appends the `lxd` scope when a socket is present, matching the one-shot discovery shipped in v0.13.0.

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
