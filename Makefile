.PHONY: build build-frontend build-all release test lint fmt install clean help

# Build variables
BINARY_NAME=infracanvas
BUILD_DIR=bin
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION_FLAGS=-X infracanvas/cmd/infracanvas/cmd.Version=$(VERSION) -X infracanvas/cmd/infracanvas/cmd.GitCommit=$(COMMIT) -X infracanvas/cmd/infracanvas/cmd.BuildDate=$(BUILD_DATE)
LDFLAGS=-ldflags "$(VERSION_FLAGS)"
RELEASE_LDFLAGS=-ldflags "$(VERSION_FLAGS) -s -w"

WEBUI_DIST=pkg/webui/dist
FRONTEND_OUT=frontend/out

# Build the dashboard and copy it into the embed directory.
# Re-run this whenever you change anything under frontend/.
build-frontend:
	@echo "Building frontend (Next.js static export)..."
	cd frontend && npm install --no-audit --no-fund && npm run build
	@echo "Copying export → $(WEBUI_DIST)/"
	@rm -rf $(WEBUI_DIST)
	@mkdir -p $(WEBUI_DIST)
	@cp -r $(FRONTEND_OUT)/. $(WEBUI_DIST)/
	@echo "Frontend embedded."

# Build the binary for the current platform with the embedded dashboard.
# Run `make build-frontend` first, or use `make all` which chains them.
build:
	@echo "Building $(BINARY_NAME) (with embedded UI)..."
	@mkdir -p $(BUILD_DIR)
	go build -tags embed_full $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/infracanvas

# Build a "stub" binary without the embedded dashboard — useful for quick
# iteration on backend code when you don't need the UI compiled in.
build-stub:
	@echo "Building $(BINARY_NAME) (stub UI, no frontend)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/infracanvas

# Full local build: dashboard + binary.
all: build-frontend build

# Run all tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Install binary to /usr/local/bin
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete!"

# Lint Go sources
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format Go source
fmt:
	gofmt -w .

# Clean build artifacts (keeps the embedded dist placeholder so the binary always builds)
clean:
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@rm -rf $(WEBUI_DIST) frontend/out frontend/.next
	@echo "Clean complete."

# Cross-compile release binaries for Linux + macOS (amd64 + arm64).
# build-frontend must run first so the embedded UI is up to date.
release: build-frontend
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	GOOS=linux  GOARCH=amd64 go build -tags embed_full $(RELEASE_LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64  ./cmd/infracanvas
	GOOS=linux  GOARCH=arm64 go build -tags embed_full $(RELEASE_LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-arm64  ./cmd/infracanvas
	GOOS=darwin GOARCH=amd64 go build -tags embed_full $(RELEASE_LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 ./cmd/infracanvas
	GOOS=darwin GOARCH=arm64 go build -tags embed_full $(RELEASE_LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 ./cmd/infracanvas
	@echo "Release binaries in $(BUILD_DIR)/release/"

help:
	@echo "Available targets:"
	@echo "  all            - Build frontend + binary for current platform (with embedded UI)"
	@echo "  build          - Build the binary (-tags embed_full, requires dist/)"
	@echo "  build-stub     - Build with placeholder UI (fast iteration on backend)"
	@echo "  build-frontend - Build the dashboard and embed it under pkg/webui/dist/"
	@echo "  release        - Cross-compile release binaries for Linux + macOS"
	@echo "  test           - Run tests with race detection and coverage"
	@echo "  install        - Install binary to /usr/local/bin (requires sudo)"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format Go source with gofmt"
	@echo "  clean          - Remove build artifacts and embedded dashboard"
	@echo "  help           - Show this help message"
