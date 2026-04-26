# Contributing to InfraCanvas

Thank you for your interest in contributing. This document covers how to set up your environment, run tests, and submit changes.

## Development setup

**Requirements:** Go 1.21+, Node.js 20+

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Build dashboard + binary (with embedded UI)
make all

# Run all Go tests
make test

# Run the dashboard locally
./bin/infracanvas       # → http://localhost:7777/?token=…
```

Frontend-only iteration (against a separately running binary):

```bash
# Terminal 1 — run the binary in stub mode for fast rebuilds
make build-stub && ./bin/infracanvas serve --port 7777 --token dev

# Terminal 2 — Next dev server
cd frontend && npm install && npm run dev    # http://localhost:3000
```

## Project structure

```
cmd/infracanvas/cmd/    CLI commands (serve, start, discover, …)
pkg/agent/              WebSocket agent: discover, diff, exec, actions
pkg/server/             Relay: WebSocket broker, sessions, auth, static UI
pkg/webui/              Embedded dashboard (build-tagged)
pkg/discovery/          Host / Docker / Kubernetes discovery
pkg/actions/            Docker / K8s / Host action executors
frontend/               Next.js dashboard (statically exported)
```

## Making changes

1. Fork the repo and create a branch from `main`
2. Write or update tests for your change
3. Run `make test` and `make lint` — both must pass
4. Open a pull request against `main`

## Commit style

Use the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
feat: add container log streaming
fix: stop reconnect loop on server-rejected pair code
docs: update self-hosting instructions
refactor: extract session store into its own file
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `ci`, `chore`

## Testing

```bash
# Run all tests with race detector
make test

# Run a specific package
go test ./pkg/relationships/...

# Frontend lint
cd frontend && npm run lint
```

## Adding a new discovery source

1. Create a new package under `pkg/discovery/<name>/`
2. Implement the `Discoverer` interface in `pkg/orchestrator/orchestrator.go`
3. Add it to the orchestrator's scope list
4. Add tests in `<name>_test.go`

## Reporting bugs

Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md). Please include:
- InfraCanvas version (`infracanvas version`)
- OS and architecture of both the VM and the browser host
- Relevant log output (`sudo journalctl -u infracanvas -n 50`)

## Security vulnerabilities

Please **do not** open a public issue for security vulnerabilities. See [SECURITY.md](SECURITY.md) for responsible disclosure instructions.
