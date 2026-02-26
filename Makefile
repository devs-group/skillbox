APP_NAME    := skillbox
MODULE      := github.com/devs-group/skillbox
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME  := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS     := -w -s \
               -X $(MODULE)/internal/version.Version=$(VERSION) \
               -X $(MODULE)/internal/version.Commit=$(COMMIT) \
               -X $(MODULE)/internal/version.BuildTime=$(BUILD_TIME)

GO          := go
GOLINT      := golangci-lint
DOCKER      := docker

REGISTRY    ?= ghcr.io/devs-group
IMAGE       := $(REGISTRY)/$(APP_NAME)

.PHONY: all build build-cli run test test-cover lint fmt vet tidy \
        docker-build docker-push dev dev-down clean help

## all: lint, test, and build (default target)
all: lint test build

# ------------------------------------------------------------
# Build
# ------------------------------------------------------------

## build: Compile the server binary
build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/skillbox-server ./cmd/skillbox-server

## build-cli: Compile the CLI binary
build-cli:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/skillbox ./cmd/skillbox

## build-all: Build both server and CLI
build-all: build build-cli

## run: Build and run the server locally
run: build
	./bin/skillbox-server

# ------------------------------------------------------------
# Quality
# ------------------------------------------------------------

## test: Run all tests
test:
	$(GO) test -race -count=1 ./...

## test-cover: Run tests with coverage report
test-cover:
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out
	@echo "---"
	@echo "To view HTML report: go tool cover -html=coverage.out"

## test-integration: Run integration tests (requires Docker)
test-integration:
	$(GO) test -tags=integration -race -count=1 -timeout=5m ./...

## lint: Run golangci-lint
lint:
	$(GOLINT) run --timeout 5m ./...

## fmt: Format all Go files
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## tidy: Tidy and verify module dependencies
tidy:
	$(GO) mod tidy
	$(GO) mod verify

# ------------------------------------------------------------
# Docker
# ------------------------------------------------------------

## docker-build: Build the Docker image
docker-build:
	$(DOCKER) build \
		-f deploy/docker/Dockerfile \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

## docker-push: Push the Docker image to the registry
docker-push: docker-build
	$(DOCKER) push $(IMAGE):$(VERSION)
	$(DOCKER) push $(IMAGE):latest

# ------------------------------------------------------------
# Development
# ------------------------------------------------------------

## dev: Start the full development stack with Docker Compose
dev:
	$(DOCKER) compose -f deploy/docker/docker-compose.yml up --build

## dev-down: Tear down the development stack
dev-down:
	$(DOCKER) compose -f deploy/docker/docker-compose.yml down -v

## seed: Create an initial API key (run after dev stack is up)
seed:
	@bash scripts/seed-apikey.sh

# ------------------------------------------------------------
# Proto
# ------------------------------------------------------------

## proto: Generate gRPC code from proto definitions (requires protoc + plugins)
proto:
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/skillbox/v1/skillbox.proto

# ------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out
	$(GO) clean -cache -testcache

# ------------------------------------------------------------
# Help
# ------------------------------------------------------------

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'
