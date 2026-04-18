---
name: Bug report
about: Something isn't working
labels: bug
---

## Description

<!-- A clear description of what the bug is. -->

## Steps to reproduce

1. 
2. 
3. 

## Expected behaviour

<!-- What you expected to happen. -->

## Actual behaviour

<!-- What actually happened. -->

## Environment

- **InfraCanvas version:** <!-- run `infracanvas version` -->
- **Agent OS/arch:** <!-- e.g. Ubuntu 24.04 / linux/amd64 -->
- **Browser:** <!-- e.g. Chrome 124 -->
- **Deployment:** <!-- Docker Compose / local binary / self-hosted relay -->

## Logs

```
# Agent logs:
sudo journalctl -u infracanvas-agent -n 50

# Server logs (if self-hosting):
docker compose logs server
```

<!-- Paste relevant output here -->
