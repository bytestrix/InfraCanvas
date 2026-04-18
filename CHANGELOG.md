# Changelog

All notable changes to InfraCanvas are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added
- Warm Claude/Anthropic-inspired UI redesign (coral accent, near-black backgrounds)
- `frontend/.env` committed with public relay URL so cloned repos work out of the box
- `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, `LICENSE`
- GitHub issue and PR templates

### Fixed
- WebSocket reconnect loop no longer masks "unknown pair code" error — UI now shows the error instead of spinning forever
- `install-agent.sh` pair code grep regex now matches `WORD-1234` format correctly
- `frontend/.env` default prevents agent↔browser relay mismatch for new users

### Changed
- Removed stale dev-only scripts (`deploy-*.sh`, `connect-vm.sh`, `test_e2e.sh`, `test_security.sh`)
- Removed redundant docs (`AZURE_VM_SETUP.md`, `DEVOPS_*.md`, `QUICKSTART.md`, etc.)
- CI now validates Go linting and frontend build

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
