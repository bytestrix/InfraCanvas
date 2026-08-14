#!/usr/bin/env bash
# InfraCanvas installer
# Usage:
#   curl -fsSL https://github.com/bytestrix/InfraCanvas/releases/latest/download/install.sh | bash
#
# Flags (pass after `bash -s --`):
#   --port <N>      Listen port (default 7777, auto-falls-back if taken)
#   --no-tunnel     Don't open a Cloudflare quick-tunnel; bind the port directly
#   --private       Imply --no-tunnel and bind 127.0.0.1 (SSH-tunnel only)
#   --read-only     Viewers can look but not touch: actions and terminals
#                   are blocked server-side (for public demos)
#   --run-user <U>  Run the systemd service as this user (default: auto-detect
#                   from $SUDO_USER, then any user with ~/.kube/config, then
#                   any user in the docker group; falls back to root)
#   --version <V>   Install a specific release tag (default: latest)
#   --join <URL>    Agent-only install: connect this VM to an existing
#                   InfraCanvas hub instead of running its own dashboard.
#                   Outbound connection only — opens no port on this VM.
#   --token <T>     Join token of the hub (printed by `infracanvas serve`
#                   on the hub VM). Required with --join.

set -euo pipefail

# ── defaults ──────────────────────────────────────────────────────────────────
GITHUB_REPO="bytestrix/InfraCanvas"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/infracanvas"
SERVICE_NAME="infracanvas"
LEGACY_SERVICE_NAME="infracanvas-agent"
PORT=7777
BIND_PRIVATE="false"
USE_TUNNEL="true"
READ_ONLY="false"
VERSION="latest"
RUN_USER_OVERRIDE=""
JOIN_URL=""
JOIN_TOKEN=""

# ── helpers ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[1;36m'; BOLD='\033[1m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

run_priv() {
  if [[ $EUID -eq 0 ]]; then "$@"; else sudo "$@"; fi
}

# ── parse args ────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)      PORT="$2";          shift 2 ;;
    --no-tunnel) USE_TUNNEL="false";  shift ;;
    --private)   BIND_PRIVATE="true"; USE_TUNNEL="false"; shift ;;
    --read-only) READ_ONLY="true";    shift ;;
    --run-user)  RUN_USER_OVERRIDE="$2"; shift 2 ;;
    --version)   VERSION="$2";        shift 2 ;;
    --join)      JOIN_URL="$2";       shift 2 ;;
    --token)     JOIN_TOKEN="$2";     shift 2 ;;
    -h|--help)
      sed -n '2,20p' "$0" | sed 's/^# //; s/^#//'
      exit 0
      ;;
    *) error "Unknown argument: $1 (try --help)" ;;
  esac
done

if [[ -n "$JOIN_URL" && -z "$JOIN_TOKEN" ]]; then
  error "--join requires --token <join-token> (printed by 'infracanvas serve' on the hub VM)"
fi
if [[ -z "$JOIN_URL" && -n "$JOIN_TOKEN" ]]; then
  error "--token only makes sense with --join <hub-url>"
fi

# ── platform check ────────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac
[[ "$OS" != "linux" ]] && error "This installer is Linux-only. On macOS, download the darwin binary directly: https://github.com/bytestrix/InfraCanvas/releases/latest"

info "Detected platform: $OS/$ARCH"

# ── dependencies ──────────────────────────────────────────────────────────────
for cmd in curl tar; do
  command -v "$cmd" >/dev/null || error "Required command not found: $cmd"
done

# ── stop any previous install FIRST (so port check sees real availability) ────
if command -v systemctl >/dev/null; then
  for svc in "$SERVICE_NAME" "$LEGACY_SERVICE_NAME"; do
    if systemctl list-unit-files 2>/dev/null | grep -q "^${svc}\.service"; then
      info "Stopping existing service: $svc"
      run_priv systemctl stop "$svc" 2>/dev/null || true
      sleep 1  # give the port a moment to free
    fi
  done
fi

# ── pick a free port (only if user didn't set one explicitly) ─────────────────
port_in_use() {
  local p="$1"
  if command -v ss >/dev/null; then
    ss -ltn "( sport = :$p )" 2>/dev/null | grep -q LISTEN
  elif command -v lsof >/dev/null; then
    lsof -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1
  else
    return 1  # can't tell — assume free
  fi
}

if [[ -z "$JOIN_URL" ]] && port_in_use "$PORT"; then
  ORIG_PORT="$PORT"
  for p in $(seq $((ORIG_PORT+1)) $((ORIG_PORT+15))); do
    if ! port_in_use "$p"; then
      warn "Port $ORIG_PORT is in use — falling back to $p"
      PORT="$p"
      break
    fi
  done
  if [[ "$PORT" == "$ORIG_PORT" ]]; then
    error "Ports $ORIG_PORT..$((ORIG_PORT+15)) are all in use — pass --port <free-port>"
  fi
fi

# ── download binary ───────────────────────────────────────────────────────────
BINARY_NAME="infracanvas-${OS}-${ARCH}"
if [[ "$VERSION" == "latest" ]]; then
  RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/latest/download"
else
  RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}"
fi
DOWNLOAD_URL="${RELEASE_BASE}/${BINARY_NAME}"
CHECKSUMS_URL="${RELEASE_BASE}/checksums.txt"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

info "Downloading InfraCanvas (${VERSION})..."
USED_LOCAL_BUILD=0
if ! curl -fsSL --progress-bar "$DOWNLOAD_URL" -o "$TMP_DIR/infracanvas"; then
  # Local-build fallback for `./install-agent.sh` from a clone with bin/ populated.
  LOCAL_BIN="$(dirname "$0")/bin/release/${BINARY_NAME}"
  [[ ! -x "$LOCAL_BIN" ]] && LOCAL_BIN="$(dirname "$0")/bin/${BINARY_NAME}"
  if [[ -x "$LOCAL_BIN" ]]; then
    info "Using local build at $LOCAL_BIN"
    cp "$LOCAL_BIN" "$TMP_DIR/infracanvas"
    USED_LOCAL_BUILD=1
  else
    error "Could not download from $DOWNLOAD_URL — check the release exists, or build locally: make release"
  fi
fi

# Verify integrity against the release's published checksums.txt. Skipped only
# for the local-build fallback above (nothing to verify against — it's already
# a binary the operator built themselves on this machine).
if [[ "$USED_LOCAL_BUILD" -eq 0 ]]; then
  info "Verifying checksum..."
  if ! curl -fsSL "$CHECKSUMS_URL" -o "$TMP_DIR/checksums.txt"; then
    error "Could not fetch $CHECKSUMS_URL to verify the download — refusing to install an unverified binary"
  fi
  EXPECTED_SHA256="$(grep -E "  ${BINARY_NAME}\$|  \./${BINARY_NAME}\$" "$TMP_DIR/checksums.txt" | awk '{print $1}' | head -n1)"
  if [[ -z "$EXPECTED_SHA256" ]]; then
    error "No checksum entry for ${BINARY_NAME} in checksums.txt — refusing to install an unverified binary"
  fi
  if command -v sha256sum >/dev/null; then
    ACTUAL_SHA256="$(sha256sum "$TMP_DIR/infracanvas" | awk '{print $1}')"
  elif command -v shasum >/dev/null; then
    ACTUAL_SHA256="$(shasum -a 256 "$TMP_DIR/infracanvas" | awk '{print $1}')"
  else
    error "Neither sha256sum nor shasum found — cannot verify download integrity"
  fi
  if [[ "${ACTUAL_SHA256,,}" != "${EXPECTED_SHA256,,}" ]]; then
    error "Checksum mismatch for ${BINARY_NAME} (got ${ACTUAL_SHA256}, want ${EXPECTED_SHA256}) — refusing to install"
  fi
fi
chmod +x "$TMP_DIR/infracanvas"

if ! "$TMP_DIR/infracanvas" version >/dev/null 2>&1; then
  error "Downloaded binary failed to run — wrong arch or corrupted download"
fi

info "Installing to $INSTALL_DIR/infracanvas"
run_priv mv "$TMP_DIR/infracanvas" "$INSTALL_DIR/infracanvas"

# ── auth token ────────────────────────────────────────────────────────────────
TOKEN="$(head -c 12 /dev/urandom | od -An -tx1 | tr -d ' \n')"

info "Writing config to $CONFIG_DIR/config.env"
run_priv mkdir -p "$CONFIG_DIR"
SERVE_FLAGS=""
[[ "$USE_TUNNEL"   != "true" ]] && SERVE_FLAGS="$SERVE_FLAGS --no-tunnel"
[[ "$BIND_PRIVATE" == "true" ]] && SERVE_FLAGS="$SERVE_FLAGS --private"
[[ "$READ_ONLY"    == "true" ]] && SERVE_FLAGS="$SERVE_FLAGS --read-only"

if [[ -n "$JOIN_URL" ]]; then
  # Agent-only: this VM streams to an existing hub. Outbound only —
  # no port is opened, no tunnel is started, no UI token is needed.
  run_priv tee "$CONFIG_DIR/config.env" >/dev/null <<EOF
# InfraCanvas agent configuration
# Auto-generated by install.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
INFRACANVAS_BACKEND=$JOIN_URL
INFRACANVAS_TOKEN=$JOIN_TOKEN
EOF
else
  run_priv tee "$CONFIG_DIR/config.env" >/dev/null <<EOF
# InfraCanvas configuration
# Auto-generated by install.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
INFRACANVAS_UI_TOKEN=$TOKEN
INFRACANVAS_PORT=$PORT
INFRACANVAS_PRIVATE=$BIND_PRIVATE
INFRACANVAS_TUNNEL=$USE_TUNNEL
INFRACANVAS_READONLY=$READ_ONLY
EOF
fi
run_priv chmod 600 "$CONFIG_DIR/config.env"

# ── open the host firewall (UFW) if it's active and the port is closed ────────
# Skip for tunnel mode: cloudflared connects on loopback, no inbound rule needed.
# Skip for join mode: the agent only makes an outbound connection.
if [[ -z "$JOIN_URL" && "$BIND_PRIVATE" != "true" && "$USE_TUNNEL" != "true" ]] && command -v ufw >/dev/null; then
  if run_priv ufw status 2>/dev/null | grep -q "Status: active"; then
    if ! run_priv ufw status 2>/dev/null | grep -qE "^${PORT}/tcp\s+ALLOW"; then
      info "Opening port $PORT in UFW..."
      run_priv ufw allow "${PORT}/tcp" >/dev/null 2>&1 || warn "Could not add UFW rule for $PORT"
    fi
  fi
fi

# ── systemd unit ──────────────────────────────────────────────────────────────
if ! command -v systemctl >/dev/null || [[ ! -d /etc/systemd/system ]]; then
  warn "systemd not detected — start manually:"
  if [[ -n "$JOIN_URL" ]]; then
    warn "  $INSTALL_DIR/infracanvas start --backend $JOIN_URL --token $JOIN_TOKEN"
  else
    warn "  $INSTALL_DIR/infracanvas serve --port $PORT$SERVE_FLAGS"
  fi
  exit 0
fi

# Pick the user the systemd service should run as. Discovery sees that user's
# ~/.kube/config and (if they're in the docker group) /var/run/docker.sock, so
# this matters for whether Kubernetes / Docker show up in the dashboard.
#
# Cascade:
#   1. --run-user flag (explicit override)
#   2. $SUDO_USER (when run via plain `sudo …/install.sh`)
#   3. First non-root user in /home/* whose ~/.kube/config is readable
#   4. First non-root user in /home/* who is a member of the docker group
#   5. First non-root user with a real shell in /home/*
#   6. root (last resort — Kubernetes/Docker discovery may be empty)
pick_run_user() {
  local u
  if [[ -n "$RUN_USER_OVERRIDE" ]]; then
    if ! getent passwd "$RUN_USER_OVERRIDE" >/dev/null 2>&1; then
      error "--run-user $RUN_USER_OVERRIDE: no such user"
    fi
    echo "$RUN_USER_OVERRIDE"; return
  fi
  if [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" ]] \
       && getent passwd "$SUDO_USER" >/dev/null 2>&1; then
    echo "$SUDO_USER"; return
  fi
  # Scan /home for a candidate
  for h in /home/*/; do
    u="$(basename "$h")"
    [[ "$u" == "root" || "$u" == "*" ]] && continue
    getent passwd "$u" >/dev/null 2>&1 || continue
    if [[ -r "/home/${u}/.kube/config" ]]; then
      echo "$u"; return
    fi
  done
  for h in /home/*/; do
    u="$(basename "$h")"
    [[ "$u" == "root" || "$u" == "*" ]] && continue
    getent passwd "$u" >/dev/null 2>&1 || continue
    if id -nG "$u" 2>/dev/null | tr ' ' '\n' | grep -qx docker; then
      echo "$u"; return
    fi
  done
  for h in /home/*/; do
    u="$(basename "$h")"
    [[ "$u" == "root" || "$u" == "*" ]] && continue
    getent passwd "$u" >/dev/null 2>&1 || continue
    local sh
    sh="$(getent passwd "$u" | cut -d: -f7)"
    case "$sh" in
      */nologin|*/false|"") continue ;;
      *) echo "$u"; return ;;
    esac
  done
  echo "root"
}

RUN_USER="$(pick_run_user)"
RUN_HOME="$(getent passwd "$RUN_USER" 2>/dev/null | cut -d: -f6)"
[[ -z "$RUN_HOME" ]] && RUN_HOME="/root"
info "Service will run as: $RUN_USER (home: $RUN_HOME)"

UNIT_USER=""
UNIT_GROUP=""
UNIT_SUPP_GROUPS=""
UNIT_KUBECONFIG=""
# HOME must be set for every run user, not just non-root — a root service
# with no HOME makes os.UserCacheDir() fail in the agent, which falls back
# to /tmp/infracanvas: a world-writable path any local user can pre-create
# and plant a binary in before the service ever starts.
UNIT_HOME="Environment=HOME=${RUN_HOME}"
if [[ "$RUN_USER" != "root" ]]; then
  UNIT_USER="User=${RUN_USER}"
  UNIT_GROUP="Group=${RUN_USER}"
  if id -nG "$RUN_USER" 2>/dev/null | tr ' ' '\n' | grep -qx docker; then
    UNIT_SUPP_GROUPS="SupplementaryGroups=docker"
  else
    warn "User $RUN_USER not in 'docker' group — Docker discovery will be skipped."
    warn "  Run: sudo usermod -aG docker $RUN_USER  (then log out/in and reinstall)"
  fi
fi
if [[ -r "${RUN_HOME}/.kube/config" ]]; then
  UNIT_KUBECONFIG="Environment=KUBECONFIG=${RUN_HOME}/.kube/config"
  info "Detected kubeconfig at ${RUN_HOME}/.kube/config — Kubernetes discovery enabled."
fi

if [[ -n "$JOIN_URL" ]]; then
  EXEC_START="${INSTALL_DIR}/infracanvas start"
  UNIT_DESC="InfraCanvas agent (streaming to hub)"
else
  EXEC_START="${INSTALL_DIR}/infracanvas serve --port \${INFRACANVAS_PORT}${SERVE_FLAGS}"
  UNIT_DESC="InfraCanvas dashboard and agent"
fi

info "Installing systemd service: $SERVICE_NAME (User=${RUN_USER})"
run_priv tee "/etc/systemd/system/${SERVICE_NAME}.service" >/dev/null <<EOF
[Unit]
Description=${UNIT_DESC}
Documentation=https://github.com/${GITHUB_REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
${UNIT_USER}
${UNIT_GROUP}
${UNIT_SUPP_GROUPS}
${UNIT_HOME}
${UNIT_KUBECONFIG}
EnvironmentFile=${CONFIG_DIR}/config.env
# StateDirectory= creates /var/lib/infracanvas, owned by the service user, and
# exposes it as \$STATE_DIRECTORY. The serve command writes the live tunnel
# URL there so \`infracanvas url\` can recover it after the install banner
# scrolls off-screen (or after cloudflared respawns with a new hostname).
StateDirectory=infracanvas
StateDirectoryMode=0755
ExecStart=${EXEC_START}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=infracanvas

[Install]
WantedBy=multi-user.target
EOF

# Remove legacy unit if present
if systemctl list-unit-files 2>/dev/null | grep -q "^${LEGACY_SERVICE_NAME}\.service"; then
  info "Removing legacy ${LEGACY_SERVICE_NAME} service"
  run_priv systemctl disable "$LEGACY_SERVICE_NAME" 2>/dev/null || true
  run_priv rm -f "/etc/systemd/system/${LEGACY_SERVICE_NAME}.service"
fi

run_priv systemctl daemon-reload
run_priv systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
# Capture cutoff just before restart so we only read the URL from this run.
RESTART_AT="$(date -u +%Y-%m-%d\ %H:%M:%S)"
run_priv systemctl restart "$SERVICE_NAME"

# Verify it actually started
sleep 2
if ! run_priv systemctl is-active --quiet "$SERVICE_NAME"; then
  warn "Service failed to start. Last log lines:"
  run_priv journalctl -u "$SERVICE_NAME" -n 20 --no-pager || true
  error "Aborting — fix the issue above and re-run install"
fi

# ── join mode: no tunnel, no URL — the hub has the dashboard ──────────────────
if [[ -n "$JOIN_URL" ]]; then
  echo ""
  echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
  echo -e "${BOLD}${GREEN}  ✓ InfraCanvas agent installed — joining your hub${NC}"
  echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
  echo ""
  echo -e "  Hub:  ${CYAN}${JOIN_URL}${NC}"
  echo ""
  echo -e "  This VM now streams its infrastructure to the hub over an"
  echo -e "  ${BOLD}outbound-only${NC} connection — no port was opened here."
  echo ""
  echo -e "  Open the hub dashboard: this machine appears in the"
  echo -e "  ${BOLD}Machines${NC} list in the sidebar within a few seconds."
  echo ""
  echo "  Manage the agent:"
  echo "    sudo systemctl status   $SERVICE_NAME"
  echo "    sudo systemctl restart  $SERVICE_NAME"
  echo "    sudo journalctl -u $SERVICE_NAME -f"
  echo ""
  echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
  exit 0
fi

# ── detect the public IP for the printed URL ──────────────────────────────────
detect_public_ip() {
  local ip=""
  # Azure IMDS
  ip=$(curl -fsS -m 1 -H "Metadata: true" \
    "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2021-02-01&format=text" 2>/dev/null || true)
  [[ -n "$ip" ]] && { echo "$ip"; return; }
  # AWS IMDSv2
  local tok
  tok=$(curl -fsS -m 1 -X PUT -H "X-aws-ec2-metadata-token-ttl-seconds: 60" \
    "http://169.254.169.254/latest/api/token" 2>/dev/null || true)
  if [[ -n "$tok" ]]; then
    ip=$(curl -fsS -m 1 -H "X-aws-ec2-metadata-token: $tok" \
      "http://169.254.169.254/latest/meta-data/public-ipv4" 2>/dev/null || true)
    [[ -n "$ip" ]] && { echo "$ip"; return; }
  fi
  # GCP
  ip=$(curl -fsS -m 1 -H "Metadata-Flavor: Google" \
    "http://169.254.169.254/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip" 2>/dev/null || true)
  [[ -n "$ip" ]] && { echo "$ip"; return; }
  # Public echo
  ip=$(curl -fsS -m 2 https://api.ipify.org 2>/dev/null || true)
  [[ -n "$ip" ]] && { echo "$ip"; return; }
}

TUNNEL_URL=""
TUNNEL_REACHABLE="false"
STATE_FILE="/var/lib/infracanvas/state.json"
if [[ "$USE_TUNNEL" == "true" ]]; then
  info "Waiting for Cloudflare quick-tunnel to publish a URL (up to 60s)..."
  for _ in $(seq 1 60); do
    # The state file holds the live URL, but a stale one survives across
    # `systemctl restart` until the new serve process overwrites it. Only
    # trust the state file when its mtime is newer than RESTART_AT — the
    # journal (filtered with --since RESTART_AT) is the safe fallback.
    TUNNEL_URL=""
    if [[ -r "$STATE_FILE" ]] && run_priv find "$STATE_FILE" -newermt "$RESTART_AT" -print 2>/dev/null | grep -q .; then
      TUNNEL_URL=$(run_priv grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$STATE_FILE" 2>/dev/null | tail -1 || true)
    fi
    if [[ -z "$TUNNEL_URL" ]]; then
      TUNNEL_URL=$(run_priv journalctl -u "$SERVICE_NAME" --no-pager --since "$RESTART_AT" 2>/dev/null \
        | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -1 || true)
    fi
    [[ -n "$TUNNEL_URL" ]] && break
    sleep 1
  done
  if [[ -z "$TUNNEL_URL" ]]; then
    warn "Tunnel URL didn't appear within 60s — check: sudo journalctl -u $SERVICE_NAME -n 50"
  else
    # cloudflared only prints the trycloudflare.com URL on stderr *after* it
    # has registered with the Cloudflare edge, so seeing the URL in the journal
    # is itself proof the tunnel is live.
    #
    # However, free quick-tunnels can cycle within seconds: the watchdog
    # respawns cloudflared and writes a NEW URL to state.json. Rather than
    # sleeping a fixed amount and hoping the URL is still valid, we actively
    # probe the health endpoint. On each tick we also re-read state.json so
    # that if the tunnel cycled we automatically switch to the new URL.
    info "Verifying tunnel is reachable..."
    VERIFIED=false
    for _ in $(seq 1 24); do
      # Always pick up the latest URL in case the tunnel cycled during this loop.
      if [[ -r "$STATE_FILE" ]]; then
        LATEST=$(run_priv grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' "$STATE_FILE" 2>/dev/null | tail -1 || true)
        [[ -n "$LATEST" ]] && TUNNEL_URL="$LATEST"
      fi
      if curl -fsS --max-time 4 "${TUNNEL_URL}/api/health" >/dev/null 2>&1; then
        VERIFIED=true
        break
      fi
      sleep 2
    done
    if [[ "$VERIFIED" != "true" ]]; then
      warn "Tunnel didn't respond within 50s — the URL below may still need a moment."
      warn "If it doesn't load, run: sudo infracanvas url"
    fi
    TUNNEL_REACHABLE="true"
  fi
fi

PUBLIC_IP="$(detect_public_ip || true)"
INTERNAL_IP="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}' | head -1 || true)"
[[ -z "$INTERNAL_IP" ]] && INTERNAL_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"

REMOTE_USER="${SUDO_USER:-$USER}"

# ── final banner ──────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}${GREEN}  ✓ InfraCanvas installed and running${NC}"
echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
echo ""

if [[ -n "$TUNNEL_URL" ]]; then
  echo -e "  ${BOLD}Open in your browser:${NC}"
  echo -e "    ${CYAN}${TUNNEL_URL}/?token=${TOKEN}${NC}"
  echo ""
  echo -e "  ${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "  ${YELLOW}  URL not loading? The tunnel may have cycled. Run:${NC}"
  echo -e "  ${YELLOW}    ${BOLD}sudo infracanvas url${NC}"
  echo -e "  ${YELLOW}  to get the current live URL at any time.${NC}"
  echo -e "  ${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  echo -e "  Cloudflare free quick-tunnels need no firewall change, but each"
  echo -e "  cloudflared restart yields a new random hostname. The service"
  echo -e "  watchdog handles restarts automatically — ${BOLD}sudo infracanvas url${NC}"
  echo -e "  always prints the current URL."
  echo -e "  For a stable URL, reinstall with ${BOLD}--no-tunnel${NC} and open the port."
elif [[ "$BIND_PRIVATE" == "true" ]]; then
  echo -e "  Bound to ${BOLD}127.0.0.1:${PORT}${NC} — only this machine can reach it."
  echo ""
  echo -e "  ${BOLD}From your laptop, open an SSH tunnel:${NC}"
  if [[ -n "$INTERNAL_IP" ]]; then
    echo -e "    ${CYAN}ssh -L ${PORT}:localhost:${PORT} ${REMOTE_USER}@${INTERNAL_IP}${NC}"
  else
    echo -e "    ${CYAN}ssh -L ${PORT}:localhost:${PORT} ${REMOTE_USER}@<this-vm-ip>${NC}"
  fi
  echo ""
  echo -e "  Then open: ${CYAN}http://localhost:${PORT}/?token=${TOKEN}${NC}"
elif [[ -n "$PUBLIC_IP" ]]; then
  echo -e "  ${BOLD}Open in your browser:${NC}"
  echo -e "    ${CYAN}http://${PUBLIC_IP}:${PORT}/?token=${TOKEN}${NC}"
  echo ""
  echo -e "  ${YELLOW}Important:${NC} the URL only works if inbound TCP ${PORT} is allowed in your"
  echo -e "  cloud security group / firewall. If it doesn't load:"
  echo ""
  echo -e "    Azure  → Add inbound rule for TCP $PORT to this VM's NSG"
  echo -e "    AWS    → Authorize inbound TCP $PORT in the VM's security group"
  echo -e "    GCP    → Add a firewall rule allowing TCP $PORT"
  echo ""
  echo -e "  Alternative (no firewall change needed) — SSH tunnel from your laptop:"
  echo -e "    ${CYAN}ssh -L ${PORT}:localhost:${PORT} ${REMOTE_USER}@${PUBLIC_IP}${NC}"
  echo -e "    Then open: ${CYAN}http://localhost:${PORT}/?token=${TOKEN}${NC}"
else
  echo -e "  Bound to ${BOLD}0.0.0.0:${PORT}${NC} — no public IP detected"
  echo -e "  (this VM may be in a private subnet, on-prem, or behind NAT)."
  echo ""
  if [[ -n "$INTERNAL_IP" ]]; then
    echo -e "  From the same network: ${CYAN}http://${INTERNAL_IP}:${PORT}/?token=${TOKEN}${NC}"
    echo ""
  fi
  echo -e "  From your laptop, SSH-tunnel:"
  echo -e "    ${CYAN}ssh -L ${PORT}:localhost:${PORT} ${REMOTE_USER}@<reachable-host>${NC}"
  echo -e "    Then open: ${CYAN}http://localhost:${PORT}/?token=${TOKEN}${NC}"
fi

echo ""
echo "  Auth token:  $TOKEN"
echo "  Saved to:    $CONFIG_DIR/config.env"
echo ""
echo "  Manage the service:"
echo "    sudo infracanvas url                    # print the current public URL"
echo "    sudo systemctl status   $SERVICE_NAME"
echo "    sudo systemctl restart  $SERVICE_NAME"
echo "    sudo journalctl -u $SERVICE_NAME -f"
echo ""
echo -e "${BOLD}${GREEN}════════════════════════════════════════════════════════════${NC}"
