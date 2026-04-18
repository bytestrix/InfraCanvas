# Changelog

All notable changes to InfraCanvas are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

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
