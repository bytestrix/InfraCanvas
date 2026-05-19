# InfraCanvas — Claude Code Instructions

## Release Policy

### Semver rules (strict)
- **Patch** (`0.0.X`): bug fixes only — no new features, no UI changes
- **Minor** (`0.X.0`): new features, UI changes, refactors, dependency bumps
- **Major** (`X.0.0`): breaking changes to API, install flow, or config format

### One release per logical unit
**Never** create multiple releases for changes that belong together. If you are implementing a feature across 10 commits, that is **one release** when complete — not 10 patch releases.

Bad: v0.6.7, v0.6.8, v0.6.9, v0.6.10 each adding one small piece of the same feature  
Good: v0.7.0 when the feature is done

### Release checklist
1. All related changes committed and tested
2. `cd frontend && npm run build` passes with no errors
3. `make release VERSION=vX.Y.Z` builds all 4 binaries successfully
4. `git tag vX.Y.Z && git push origin vX.Y.Z`
5. Create GitHub release and upload binaries from `bin/release/`
6. Update release notes with full changelog

### GitHub API (no `gh` CLI on this machine)
Use Python urllib for all GitHub operations:
```python
import urllib.request, json

TOKEN = 'ghp_...'  # from user
REPO  = 'bytestrix/InfraCanvas'
HEADERS = {'Authorization': f'token {TOKEN}', 'Accept': 'application/vnd.github.v3+json'}

# Create release
data = json.dumps({'tag_name': 'vX.Y.Z', 'name': 'vX.Y.Z', 'body': '...'}).encode()
req  = urllib.request.Request(f'https://api.github.com/repos/{REPO}/releases', data=data, headers={**HEADERS, 'Content-Type': 'application/json'})
with urllib.request.urlopen(req) as r:
    release = json.loads(r.read())
upload_url = release['upload_url'].split('{')[0]

# Upload asset
with open('bin/release/infracanvas-linux-amd64', 'rb') as f:
    body = f.read()
req = urllib.request.Request(f'{upload_url}?name=infracanvas-linux-amd64', data=body,
      headers={**HEADERS, 'Content-Type': 'application/octet-stream'}, method='POST')
with urllib.request.urlopen(req) as r:
    print(r.status, r.read()[:80])
```

### Squashing bad releases
If patch releases were created prematurely:
```bash
git log --oneline vPREV..HEAD   # see what to squash
git reset --soft vPREV          # unstage commits (keep changes)
git commit -m "feat(...): ..."  # single commit
git push --force origin main

# delete old tags + releases via API, then re-tag
git tag -d vBAD && git push origin :refs/tags/vBAD
git tag vNEW && git push origin vNEW
```

## Commit conventions
- `feat(scope): ...` — new feature (triggers minor bump)
- `fix(scope): ...` — bug fix (triggers patch bump)
- `chore(...): ...` — tooling, deps, CI (no release needed unless bundled)
- No `Co-Authored-By` credits in any commit

## Build commands
```bash
make build-frontend          # Next.js static export → pkg/webui/dist/
make build                   # Go binary with embedded UI (requires dist/)
make build-stub              # Go binary with placeholder UI (fast, backend-only)
make release VERSION=vX.Y.Z  # Cross-compile linux/darwin × amd64/arm64 → bin/release/
make test                    # Go tests
```

## Theming
All colors use CSS variables (`var(--bg)`, `var(--ink)`, etc.). Never use hardcoded hex in inline styles. `[data-theme="dark"|"light"]` on `<html>` switches the entire UI. See `frontend/app/globals.css` for the full token list.
