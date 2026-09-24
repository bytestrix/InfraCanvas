# Installing and running InfraCanvas

Every method below runs the same binary. Pick by where you want the dashboard to live.

- [Local, on your own machine](#local-on-your-own-machine)
- [Docker](#docker)
- [On a Linux VM with the installer](#on-a-linux-vm-with-the-installer)
- [From source](#from-source)
- [Installer flags](#installer-flags)
- [Serve flags and environment variables](#serve-flags-and-environment-variables)
- [Putting it behind your own domain](#putting-it-behind-your-own-domain)
- [Managing the service](#managing-the-service)
- [Uninstall](#uninstall)

## Local, on your own machine

Download the release binary and run it bound to `127.0.0.1`, with no tunnel. Nothing is installed and nothing leaves the machine.

```bash
os=$(uname -s | tr '[:upper:]' '[:lower:]'); arch=$(uname -m)
[ "$arch" = x86_64 ] && arch=amd64; [ "$arch" = aarch64 ] && arch=arm64
curl -fsSLo infracanvas "https://github.com/bytestrix/InfraCanvas/releases/latest/download/infracanvas-$os-$arch"
chmod +x infracanvas
./infracanvas serve --no-tunnel --private
# → http://localhost:7777/?token=…
```

Builds exist for linux and darwin, amd64 and arm64. It picks up Docker, Podman and your current kubeconfig context automatically.

## Docker

```bash
docker run -d --name infracanvas --pid=host --network=host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v infracanvas:/data \
  ghcr.io/bytestrix/infracanvas:latest serve --no-tunnel --private

docker logs infracanvas   # prints the URL with its token
```

Why the flags:

- `--pid=host` lets it read the host's processes from `/proc`. Without it you only see the container's own processes.
- `--network=host` lets it read the host's listening ports and connections, and makes `--private` bind the host's `127.0.0.1:7777`. Drop `--private` to listen on all interfaces.
- `/var/run/docker.sock` is how it reads and controls containers. Mount it `:ro` if you only want to look; actions and container terminals will then fail.
- `/data` holds the machine ID, connected clusters and the audit log.

Limits compared to a native install: the "VM shell" terminal opens a shell inside the InfraCanvas container, not on the host, and systemd log tailing needs `journalctl`, which the image doesn't include. To see a Kubernetes cluster, connect it from the dashboard with a kubeconfig (see [Clusters](multi-machine.md#kubernetes-clusters)).

## On a Linux VM with the installer

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
```

The script ([read it first](../install-agent.sh), it's one file of bash):

1. downloads the binary to `/usr/local/bin/infracanvas`
2. writes `/etc/infracanvas/config.env` with a random 24-character auth token
3. installs a systemd unit that runs as the user who invoked it (see `--run-user`)
4. by default opens a Cloudflare quick-tunnel and prints an `https://….trycloudflare.com/?token=…` URL

The quick-tunnel gives you HTTPS with no domain or open port, but it also makes the dashboard reachable from the internet by anyone with the URL and token. The dashboard includes terminals and write actions, so treat that URL like an SSH key. If you don't want that:

```bash
# localhost only; reach it with `ssh -L 7777:127.0.0.1:7777 user@vm`
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash -s -- --private

# bind 0.0.0.0:7777 and put your own reverse proxy in front
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash -s -- --no-tunnel
```

Example output:

```
✓ InfraCanvas installed and running

  Open in your browser:
    https://shy-pine-2f1a.trycloudflare.com/?token=a8f3e2b1c9d4f02e7b61c5d0

  Auth token:  a8f3e2b1c9d4f02e7b61c5d0  (saved in /etc/infracanvas/config.env)
```

## From source

Requires Go 1.25+ (see `go.mod`) and Node.js 20+.

```bash
git clone https://github.com/bytestrix/InfraCanvas.git
cd InfraCanvas && make all
./bin/infracanvas serve --no-tunnel --private
```

## Installer flags

Pass them after `bash -s --`:

| Flag | Effect |
|---|---|
| `--private` | Bind `127.0.0.1` only. Implies `--no-tunnel`. |
| `--no-tunnel` | Don't start the Cloudflare tunnel; bind `0.0.0.0` on the port. |
| `--port <N>` | Listen port, default 7777. Falls back to a free port if taken. |
| `--read-only` | Viewers can look but not act: actions, terminals and cluster changes are rejected server-side. |
| `--run-user <U>` | User the service runs as. Default: the user running the installer (`$SUDO_USER` under sudo). If run directly as root: a user with `~/.kube/config`, then one in the `docker` group, then root. |
| `--version <V>` | Install a specific release tag, e.g. `v0.19.4`. |
| `--join <URL> --token <T>` | Agent-only install that streams to an existing hub. See [Multiple machines](multi-machine.md). |

## Serve flags and environment variables

`infracanvas serve --help` lists everything. The ones you'll use:

| Flag | Env var | Default |
|---|---|---|
| `--port` | `INFRACANVAS_PORT` (read by the systemd unit) | `7777` |
| `--no-tunnel` | `INFRACANVAS_TUNNEL=false` | tunnel on |
| `--private` | `INFRACANVAS_PRIVATE=true` | off; implies `--no-tunnel` |
| `--read-only` | `INFRACANVAS_READONLY=true` | off |
| `--token` | `INFRACANVAS_UI_TOKEN` | random |
| `--agent-token` | `INFRACANVAS_AGENT_TOKEN` | random |
| `--discover` | `INFRACANVAS_SCOPE` | `host,docker,lxd,kubernetes` |
| `--refresh` | | `30` seconds |
| `--discover-local-kubeconfig` | `INFRACANVAS_DISCOVER_LOCAL_KUBECONFIG` | `true` |
| | `INFRACANVAS_STATE_DIR` | `/var/lib/infracanvas` if it exists, else the user cache dir |

## Putting it behind your own domain

Install with `--no-tunnel`, then point a reverse proxy at `127.0.0.1:7777`. The dashboard uses WebSockets, so the proxy has to pass `Upgrade` headers.

**Caddy** (gets a certificate automatically):

```caddy
infra.example.com {
    reverse_proxy localhost:7777
}
```

**Nginx + Let's Encrypt:**

```nginx
server {
    listen 80;
    server_name infra.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name infra.example.com;

    ssl_certificate     /etc/letsencrypt/live/infra.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/infra.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:7777;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 300s;
    }
}
```

Get the certificate with `sudo certbot --nginx -d infra.example.com`.

**SSH tunnel** (no domain, nothing public): install with `--private`, then from your laptop run `ssh -L 7777:127.0.0.1:7777 user@your-server` and open `http://localhost:7777/?token=<token>`.

## Managing the service

```bash
sudo systemctl status   infracanvas
sudo systemctl restart  infracanvas
sudo journalctl -u infracanvas -f
```

`/etc/infracanvas/config.env` on a hub:

```bash
INFRACANVAS_UI_TOKEN=a8f3e2b1c9d4f02e7b61c5d0
INFRACANVAS_PORT=7777
INFRACANVAS_TUNNEL=true
INFRACANVAS_PRIVATE=false
INFRACANVAS_READONLY=false
```

On a machine installed with `--join`:

```bash
INFRACANVAS_BACKEND=https://hub.example.com
INFRACANVAS_TOKEN=<join-token>
```

Edit it, then `sudo systemctl restart infracanvas`. Installs from v0.19.4 or earlier bake the tunnel and private settings into the systemd unit; re-run the installer once to pick up edits to those two.

## Uninstall

```bash
curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/uninstall.sh | sudo bash
```

Removes the binary, the systemd unit, `/etc/infracanvas/` and the cached `cloudflared` binary (about 30 MB). From a clone: `sudo ./uninstall-agent.sh`.
