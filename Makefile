# AgentFence Makefile
.PHONY: all build build-proxy build-wsgateway build-apiserver \
        build-web test test-go test-web lint clean docker-build \
        helm-package run-dev help

GO := go
NPM := npm

# Build all binaries
all: build

build: build-proxy build-wsgateway build-apiserver

build-proxy:
	$(GO) build -o bin/proxy ./cmd/proxy/

build-wsgateway:
	$(GO) build -o bin/wsgateway ./cmd/wsgateway/

build-apiserver:
	$(GO) build -o bin/apiserver ./cmd/apiserver/

build-web:
	cd web && $(NPM) run build

# Testing
test: test-go

test-go:
	$(GO) test -v -race -count=1 ./...

test-web:
	cd web && $(NPM) run typecheck

# Linting
lint:
	golangci-lint run ./...

# Docker
docker-build:
	docker build --target proxy -t ghcr.io/agentfence/proxy:latest .
	docker build --target wsgateway -t ghcr.io/agentfence/wsgateway:latest .
	docker build --target apiserver -t ghcr.io/agentfence/apiserver:latest .
	docker build --target dashboard -t ghcr.io/agentfence/dashboard:latest .

# Helm
helm-package:
	helm package charts/agentfence -d dist/

# Development
run-dev: build-proxy build-wsgateway build-apiserver
	@echo "Starting AgentFence development server..."
	@echo "Run each binary in separate terminals:"
	@echo "  ./bin/proxy"
	@echo "  ./bin/wsgateway"
	@echo "  ./bin/apiserver"
	@echo "  cd web && npm run dev"

# Clean
clean:
	rm -rf bin/ dist/
	cd web && rm -rf dist/ node_modules/

# Help
help:
	@echo "AgentFence Makefile"
	@echo "  make all           - Build all Go binaries"
	@echo "  make build         - Build all Go binaries"
	@echo "  make test          - Run Go tests"
	@echo "  make lint          - Run linter"
	@echo "  make docker-build  - Build Docker images"
	@echo "  make helm-package  - Package Helm chart"
	@echo "  make run-dev       - Start development servers"
	@echo "  make clean         - Remove build artifacts"
