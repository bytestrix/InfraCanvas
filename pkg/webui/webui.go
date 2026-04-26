// Package webui exposes the dashboard's static assets to the relay server.
//
// Two build modes:
//   - default (no build tag): serves a placeholder page; `go build` works
//     out of the box without needing a Node toolchain.
//   - `-tags embed_full`: embeds the real Next.js export from pkg/webui/dist/.
//     Use this for release binaries — `make build-frontend` populates dist/
//     and `make build` / `make release` set the tag.
package webui
