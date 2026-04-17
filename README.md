# InfraCanvas

**Real-time infrastructure topology canvas for your VMs, containers, and Kubernetes clusters.**

InfraCanvas gives you a live visual map of everything running on your servers — containers, pods, networks, volumes, services — updating in real time. Install a lightweight agent on any Linux VM with one command. No inbound ports, no VPN, no complex setup.

![InfraCanvas Canvas](docs/canvas-preview.png)

---

## How it works

```
Your laptop / server
  └─ Dashboard (localhost:3000)
          │
          │  WebSocket (outbound only)
          ▼
    Relay Server (self-hosted)
          ▲
          │  WebSocket (outbound only)
  VMs running the agent
```

The agent on each VM connects **outbound** to your relay server — no inbound ports needed on your VMs. You pair a VM to your dashboard using a short code (like `TIGER-APPLE-CLOUD`).

---

## Quick Start

### 1. Run the dashboard + relay

You need Docker and Docker Compose installed.

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

NEXT_PUBLIC_WS_URL=ws://YOUR_SERVER_IP:8080 \
NEXT_PUBLIC_API_URL=http://YOUR_SERVER_IP:8080 \
docker compose up -d
```

Open **http://YOUR_SERVER_IP:3000** in your browser.

> To run locally on your own laptop, replace `YOUR_SERVER_IP` with `localhost`.

### 2. Install the agent on a VM

On any Linux VM you want to monitor:

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

The agent will print a **pair code**:

```
════════════════════════════════════════
  Your pair code:  TIGER-APPLE-CLOUD
  Enter this in the InfraCanvas dashboard
════════════════════════════════════════
```

Enter it in the dashboard — the VM appears on the canvas instantly.

### 3. Uninstall the agent

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash
```

---

## Features

- **Live topology canvas** — containers, pods, services, networks, volumes visualized as a graph
- **Real-time resource stats** — CPU, memory, disk per node and container
- **Multi-VM support** — connect as many VMs as you want, each paired with a code
- **Docker & Docker Compose** — full container visibility including compose projects
- **Kubernetes** — pods, deployments, namespaces, services, ingress
- **WebSocket pairing** — agents connect outbound only, no firewall rules needed on VMs
- **Sensitive data redaction** — env vars with secrets/tokens are automatically redacted

---

## Self-hosting

The relay server and dashboard run as Docker containers. You can host them on any VPS.

### Requirements

- Any Linux server with Docker + Docker Compose
- Ports **3000** (dashboard) and **8080** (relay) open in your firewall

### Deploy

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Replace with your server's IP or domain
NEXT_PUBLIC_WS_URL=ws://YOUR_IP:8080 \
NEXT_PUBLIC_API_URL=http://YOUR_IP:8080 \
docker compose up -d
```

### Useful commands

```bash
# View logs
docker compose logs -f

# Stop everything
docker compose down

# Update to latest
git pull && docker compose up --build -d
```

---

## Agent management

```bash
# View live logs
sudo journalctl -u infracanvas-agent -f

# Check status
sudo systemctl status infracanvas-agent

# Restart
sudo systemctl restart infracanvas-agent

# Stop
sudo systemctl stop infracanvas-agent
```

---

## Building from source

**Requirements:** Go 1.25+, Node.js 20+

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas

# Build agent binary
make build

# Build all release binaries (linux + macOS, amd64 + arm64)
make build-all

# Run tests
make test

# Run dashboard + relay locally
make deploy-local
```

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Dashboard                      │
│              (Next.js, port 3000)                │
└───────────────────┬─────────────────────────────┘
                    │ WebSocket /ws/canvas
┌───────────────────▼─────────────────────────────┐
│                Relay Server                      │
│             (Go, port 8080)                      │
│  /ws/agent  ◄──── agents push snapshots         │
│  /ws/canvas ◄──── dashboard subscribes          │
│  /api/*          REST endpoints                  │
└───────────────────▲─────────────────────────────┘
                    │ WebSocket /ws/agent (outbound)
        ┌───────────┴───────────┐
        │                       │
   ┌────▼────┐             ┌────▼────┐
   │  VM 1   │             │  VM 2   │
   │  agent  │             │  agent  │
   └─────────┘             └─────────┘
```

---

## Contributing

Contributions are welcome. Please open an issue before submitting a large PR so we can discuss the approach.

```bash
# Run tests
go test ./...

# Run a specific package
go test ./pkg/relationships/...
```

---

## License

MIT — see [LICENSE](LICENSE) for details.
