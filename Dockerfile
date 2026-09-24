# syntax=docker/dockerfile:1

# ── Dashboard (Next.js static export) ─────────────────────────────────────────
FROM node:20-alpine AS ui
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ── Binary with the dashboard embedded ────────────────────────────────────────
FROM golang:1.25-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/frontend/out/ pkg/webui/dist/
RUN CGO_ENABLED=0 go build -tags embed_full \
      -ldflags "-s -w -X infracanvas/cmd/infracanvas/cmd.Version=${VERSION} -X infracanvas/cmd/infracanvas/cmd.GitCommit=${COMMIT}" \
      -o /out/infracanvas ./cmd/infracanvas

# ── Runtime ───────────────────────────────────────────────────────────────────
# Alpine rather than distroless: the host terminal and log tailing need a shell.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates bash
COPY --from=build /out/infracanvas /usr/local/bin/infracanvas
ENV INFRACANVAS_STATE_DIR=/data
VOLUME /data
EXPOSE 7777
ENTRYPOINT ["infracanvas"]
CMD ["serve", "--no-tunnel"]
