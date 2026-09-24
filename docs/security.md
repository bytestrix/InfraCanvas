# Security model

InfraCanvas gives whoever opens the dashboard terminals and write actions on your machines. This page describes what protects that, so you can decide how to expose it. To report a vulnerability, see [SECURITY.md](../SECURITY.md).

## Where your data goes

Discovery, the relay and the dashboard run in one process on your machine. There is no InfraCanvas backend, account, telemetry or update check in the self-hosted binary. The only outbound connections are:

- the Cloudflare quick-tunnel, if you leave it on (`--no-tunnel` or `--private` turn it off)
- Kubernetes API servers of clusters you connect
- the hub, on machines installed with `--join`

## Network exposure

| Mode | Listens on | Reachable from |
|---|---|---|
| installer default | `127.0.0.1:7777` + Cloudflare tunnel | the internet, via the `trycloudflare.com` URL |
| `--no-tunnel` | `0.0.0.0:7777` | anything that can reach the port |
| `--private` | `127.0.0.1:7777` | this machine only; use an SSH tunnel |

The tunnel is outbound-only from your machine and HTTPS-terminated at Cloudflare's edge. It still makes the dashboard public to anyone who has the URL and token.

## Authentication

- Every install generates a random 24-character hex token (12 random bytes) and saves it in `/etc/infracanvas/config.env`.
- The dashboard needs it on first visit (`?token=…`), then keeps it in an HTTP-only cookie. Requests without it get `401`.
- Failed attempts are rate-limited per client IP.
- There is no per-user login in the self-hosted version. Anyone with the token has full access, so treat the URL plus token like an SSH key.

## Joining machines

- In hub mode the relay issues a separate join token. An agent without it is rejected at the WebSocket handshake.
- Joined machines connect outbound to the hub; the hub never dials them and they open no port.
- Each machine ID gets its own resume secret on first connect. A reconnect that claims an existing machine ID without that secret is rejected, so one joined machine can't take over another's session.

## Read-only mode

`--read-only` makes the relay reject every action, terminal request, cluster preview and cluster connect/disconnect on the server side. Topology and logs keep working. Use it for public demos or a status screen. Each connected cluster also has its own read-only toggle.

## Kubernetes clusters

- A kubeconfig you upload is parsed in memory by the same process, saved at `0600` under the state directory, and not sent anywhere else.
- Connecting runs the kubeconfig's authentication, including any `exec:` credential plugin (`aws`, `gcloud`, `az`), on the machine running `infracanvas serve`. Only connect kubeconfigs you trust.
- Before you connect, the dialog runs `SelfSubjectAccessReview` checks and shows what the credential can do: view, exec, restart/kill, scale/edit, read Secrets.
- Resource kinds the credential can't list are skipped rather than failing the connection.

## Secret redaction

Before data leaves the discovery layer:

- env vars whose names match `password`, `passwd`, `pwd`, `secret`, `token`, `key`, `credential`, `auth`, `api_key`, `access_key`, `private_key`, `jwt` or `bearer` (case-insensitive) are replaced with `[REDACTED]`
- values that look like JWTs, AWS secret keys, PEM private keys or long base64 strings are replaced too
- `--password=…`-style arguments in process command lines are masked

Kubernetes Secret values are not put into the graph; only the Secret name, its key names and metadata are.

## Audit log

Every write action, terminal session and read-only-blocked attempt is recorded locally and shown in the dashboard's Audit tab. Entries are attributed by machine, not by person.

## Which user it runs as

The systemd unit runs as the user who ran the installer (`$SUDO_USER` when run with `sudo`). If you run the installer directly as root, it picks a user with `~/.kube/config`, then a user in the `docker` group, and falls back to root. Override with `--run-user`. The service has the access that user has: docker group membership gives it root-equivalent control of containers, and its kubeconfig decides what it can do in Kubernetes.
