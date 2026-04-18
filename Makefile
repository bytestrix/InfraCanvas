.PHONY: build test lint fmt install clean

# Build variables
BINARY_NAME=infracanvas
BUILD_DIR=bin
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X infracanvas/cmd/infracanvas/cmd.Version=$(VERSION) -X infracanvas/cmd/infracanvas/cmd.GitCommit=$(COMMIT) -X infracanvas/cmd/infracanvas/cmd.BuildDate=$(BUILD_DATE)"

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/infracanvas

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Install binary to /usr/local/bin
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete!"

# Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Format Go source
fmt:
	@echo "Formatting..."
	gofmt -w .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@echo "Clean complete!"

# Cross-compile for Linux and macOS
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/infracanvas
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/infracanvas
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/infracanvas
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/infracanvas
	@echo "Cross-compilation complete!"

# Build the server binary
build-server:
	@echo "Building infracanvas-server..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/infracanvas-server ./cmd/infracanvas-server

# Build everything (agent + server) for current platform
build-all-local: build build-server

# Cross-compile agent + server for release (produces binaries install-agent.sh can serve)
release:
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	GOOS=linux  GOARCH=amd64  go build $(LDFLAGS) -ldflags="-s -w" -o $(BUILD_DIR)/release/infracanvas-linux-amd64    ./cmd/infracanvas
	GOOS=linux  GOARCH=arm64  go build $(LDFLAGS) -ldflags="-s -w" -o $(BUILD_DIR)/release/infracanvas-linux-arm64    ./cmd/infracanvas
	GOOS=darwin GOARCH=amd64  go build $(LDFLAGS) -ldflags="-s -w" -o $(BUILD_DIR)/release/infracanvas-darwin-amd64   ./cmd/infracanvas
	GOOS=darwin GOARCH=arm64  go build $(LDFLAGS) -ldflags="-s -w" -o $(BUILD_DIR)/release/infracanvas-darwin-arm64   ./cmd/infracanvas
	GOOS=linux  GOARCH=amd64  go build $(LDFLAGS) -ldflags="-s -w" -o $(BUILD_DIR)/release/infracanvas-server-linux-amd64 ./cmd/infracanvas-server
	@echo "Release binaries in $(BUILD_DIR)/release/"

# Deploy relay server + frontend via Docker Compose
deploy-local:
	@cp -n .env.example .env 2>/dev/null || true
	docker compose up --build -d
	@echo ""
	@echo "InfraCanvas running at http://localhost:3000"
	@echo "Backend relay at      ws://localhost:8080"

# Help target
help:
	@echo "Available targets:"
	@echo "  build            - Build agent binary for current platform"
	@echo "  build-server     - Build server binary for current platform"
	@echo "  build-all-local  - Build agent + server for current platform"
	@echo "  build-all        - Cross-compile agent for Linux and macOS (amd64 and arm64)"
	@echo "  release          - Build all release binaries (agent + server, all platforms)"
	@echo "  deploy-local     - Run relay server + frontend via Docker Compose"
	@echo "  test             - Run all tests with race detection and coverage"
	@echo "  lint             - Run golangci-lint"
	@echo "  fmt              - Format Go source with gofmt"
	@echo "  install          - Install binary to /usr/local/bin (requires sudo)"
	@echo "  clean            - Remove build artifacts and coverage files"
	@echo "  help             - Show this help message"
