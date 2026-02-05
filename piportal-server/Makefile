# Makefile for PiPortal Server

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION)"

.PHONY: build
build:
	go build $(LDFLAGS) -o piportal-server .

.PHONY: run
run:
	go run . -dev -http :8000

.PHONY: deps
deps:
	go mod tidy

.PHONY: clean
clean:
	rm -f piportal-server piportal.db

# Build for Linux (for deployment)
.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o piportal-server-linux .

# Docker
.PHONY: docker
docker:
	docker build -t piportal-server .

.PHONY: docker-run
docker-run:
	docker run -p 8000:8000 -e PIPORTAL_DEV=1 piportal-server

.PHONY: help
help:
	@echo "PiPortal Server Build Targets:"
	@echo ""
	@echo "  make build       - Build server binary"
	@echo "  make run         - Run in development mode on :8000"
	@echo "  make deps        - Download dependencies"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make build-linux - Build for Linux deployment"
	@echo "  make docker      - Build Docker image"
