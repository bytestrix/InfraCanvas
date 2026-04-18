# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| Latest release | ✅ |
| Older releases | ❌ |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report security issues by emailing **security@bytestrix.com** with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested fix (optional)

You will receive an acknowledgement within 48 hours and a full response within 7 days. We will coordinate a fix and disclosure timeline with you.

## Scope

The following are in scope:

- Authentication bypass in the relay server
- Arbitrary command execution via the agent API
- Sensitive data leakage (secrets not redacted before leaving the VM)
- WebSocket message injection or spoofing

The following are out of scope:

- Attacks that require physical access to the VM
- Social engineering
- Vulnerabilities in third-party dependencies (report those upstream)

## Security model

- **Agents connect outbound only** — no inbound ports needed on monitored VMs
- **Secret redaction** — environment variables matching common secret patterns are redacted before the agent sends data
- **Auth token** — set `INFRACANVAS_TOKEN` on the relay and in `agent.env` to authenticate agents; without it the relay runs in dev mode (no auth)
- **Pair codes** — codes are single-use session identifiers generated server-side; they expire when the agent disconnects
