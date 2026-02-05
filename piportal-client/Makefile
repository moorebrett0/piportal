# Makefile for PiPortal Client
# Run 'make' to build for your current platform
# Run 'make all' to build for all Pi targets

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-s -w -X github.com/piportal/piportal-client/cmd.Version=$(VERSION)"

# Output directory for builds
DIST = dist

# Default target: build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o piportal .

# Build for all target platforms
.PHONY: all
all: build-linux-arm64 build-linux-arm build-linux-amd64

# Pi 4, Pi 5, Pi Zero 2 W (64-bit)
.PHONY: build-linux-arm64
build-linux-arm64:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/piportal-linux-arm64 .
	@echo "Built $(DIST)/piportal-linux-arm64"

# Pi Zero W, Pi 1, Pi 2 (32-bit ARM)
.PHONY: build-linux-arm
build-linux-arm:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=arm GOARM=6 go build $(LDFLAGS) -o $(DIST)/piportal-linux-arm .
	@echo "Built $(DIST)/piportal-linux-arm"

# x86-64 Linux (for testing on regular servers)
.PHONY: build-linux-amd64
build-linux-amd64:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/piportal-linux-amd64 .
	@echo "Built $(DIST)/piportal-linux-amd64"

# macOS (for local development)
.PHONY: build-darwin
build-darwin:
	@mkdir -p $(DIST)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/piportal-darwin-arm64 .
	@echo "Built $(DIST)/piportal-darwin-arm64"

# Install dependencies
.PHONY: deps
deps:
	go mod tidy

# Run locally (for development)
.PHONY: run
run:
	go run . start --token test_token --port 8080 --server ws://localhost:8000/tunnel

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(DIST) piportal
	go clean

# Check binary sizes
.PHONY: size
size: all
	@echo "\nBinary sizes:"
	@ls -lh $(DIST)/

# Run tests
.PHONY: test
test:
	go test -v ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

.PHONY: help
help:
	@echo "PiPortal Client Build Targets:"
	@echo ""
	@echo "  make build          - Build for current platform"
	@echo "  make all            - Build for all Pi targets (arm64, arm, amd64)"
	@echo "  make build-darwin   - Build for macOS (development)"
	@echo "  make deps           - Download and tidy dependencies"
	@echo "  make run            - Run locally with test settings"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make size           - Show binary sizes"
	@echo "  make test           - Run tests"
	@echo "  make fmt            - Format code"
