# Database Intelligence MVP Makefile

# Variables
BINARY_NAME=database-intelligence-collector
GOPATH=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Docker image naming
DOCKER_REGISTRY ?= 
DOCKER_ORG ?= newrelic
DOCKER_IMAGE_BASE = database-intelligence
DOCKER_IMAGE_STANDARD = $(DOCKER_IMAGE_BASE)
DOCKER_IMAGE_ENTERPRISE = $(DOCKER_IMAGE_BASE)-enterprise
DOCKER_TAG ?= latest

# Full image names
STANDARD_IMAGE = $(if $(DOCKER_REGISTRY),$(DOCKER_REGISTRY)/,)$(DOCKER_ORG)/$(DOCKER_IMAGE_STANDARD):$(DOCKER_TAG)
ENTERPRISE_IMAGE = $(if $(DOCKER_REGISTRY),$(DOCKER_REGISTRY)/,)$(DOCKER_ORG)/$(DOCKER_IMAGE_ENTERPRISE):$(DOCKER_TAG)

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint
GOVET=$(GOCMD) vet

# Directories
TESTS_DIR=./tests
PROCESSORS_DIR=./processors
DOCS_DIR=./docs
E2E_DIR=$(TESTS_DIR)/e2e
PERF_DIR=$(TESTS_DIR)/performance
OPTIMIZATION_DIR=$(TESTS_DIR)/optimization

# Default target
.DEFAULT_GOAL := help

# Help target
.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Install dependencies
.PHONY: install-tools
install-tools: ## Install required tools
	@echo "Installing tools..."
	go install go.opentelemetry.io/collector/cmd/builder@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install gotest.tools/gotestsum@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Tools installed successfully"

# OTLP and Dimensional Metrics Testing
.PHONY: test-otlp
test-otlp: ## Run OTLP compliance and dimensional metrics tests
	@echo "Running OTLP and dimensional metrics tests..."
	@cd $(E2E_DIR) && ./run_otlp_tests.sh
	@echo "OTLP tests completed. Check tests/e2e/reports/ for results."

.PHONY: test-otlp-dimensional
test-otlp-dimensional: ## Run dimensional metrics tests only
	@echo "Running dimensional metrics tests..."
	@cd $(E2E_DIR) && ./run_otlp_tests.sh dimensional

.PHONY: test-otlp-semantic
test-otlp-semantic: ## Run semantic conventions tests only
	@echo "Running semantic conventions tests..."
	@cd $(E2E_DIR) && ./run_otlp_tests.sh semantic

.PHONY: test-otlp-cardinality
test-otlp-cardinality: ## Run cardinality control tests only
	@echo "Running cardinality control tests..."
	@cd $(E2E_DIR) && ./run_otlp_tests.sh cardinality

.PHONY: test-otlp-pipeline
test-otlp-pipeline: ## Run processor pipeline tests only
	@echo "Running processor pipeline tests..."
	@cd $(E2E_DIR) && ./run_otlp_tests.sh pipeline

.PHONY: validate-otlp-export
validate-otlp-export: ## Validate OTLP export format
	@echo "Validating OTLP export format..."
	@cd $(E2E_DIR) && ./run_otlp_tests.sh validate

# Build targets
.PHONY: build-collector
build-collector: ## Build the custom OpenTelemetry collector
	@echo "Building custom OpenTelemetry collector..."
	@command -v builder >/dev/null 2>&1 || { echo "Installing builder..."; go install go.opentelemetry.io/collector/cmd/builder@v0.105.0; }
	builder --config=otelcol-builder-config-complete.yaml
	@echo "Collector build complete: distributions/production/$(BINARY_NAME)"

.PHONY: build-collector-minimal
build-collector-minimal: ## Build minimal collector
	@echo "Building minimal collector..."
	builder --config=otelcol-builder-config-minimal.yaml
	@echo "Minimal collector build complete"

.PHONY: build-collector-all
build-collector-all: ## Build all collector variants
	@echo "Building all collector variants..."
	$(MAKE) build-collector-minimal
	$(MAKE) build-collector
	@echo "All collectors built"

# Legacy build target
.PHONY: build
build: build-collector ## Build the binary (alias for build-collector)

.PHONY: build-debug
build-debug: ## Build with debug symbols
	@echo "Building debug binary..."
	$(GOBUILD) -gcflags="all=-N -l" $(LDFLAGS) -o bin/$(BINARY_NAME)-debug .
	@echo "Debug build complete: bin/$(BINARY_NAME)-debug"

.PHONY: build-all
build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Multi-platform build complete"

# Clean targets
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -rf test-results/
	rm -rf coverage/
	@echo "Clean complete"

# Dependency management
.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Managing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies updated"

.PHONY: deps-upgrade
deps-upgrade: ## Upgrade all dependencies
	@echo "Upgrading dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "Dependencies upgraded"

# Code quality targets
.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	goimports -w .
	@echo "Code formatted"

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	$(GOLINT) run ./...
	@echo "Linting complete"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet complete"

.PHONY: security
security: ## Run security checks
	@echo "Running security scan..."
	gosec -fmt json -out security-report.json ./...
	@echo "Security scan complete. Report: security-report.json"

# Test targets
.PHONY: test
test: ## Run unit tests
	@echo "Running unit tests..."
	@mkdir -p coverage
	$(GOTEST) -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	@echo "Unit tests complete"

.PHONY: test-short
test-short: ## Run short tests only
	@echo "Running short tests..."
	$(GOTEST) -v -short ./...
	@echo "Short tests complete"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration -timeout=20m $(E2E_DIR)/...
	@echo "Integration tests complete"

.PHONY: test-e2e
test-e2e: docker-up ## Run end-to-end tests
	@echo "Running E2E tests..."
	cd $(E2E_DIR) && $(GOTEST) -v -timeout=30m .
	@echo "E2E tests complete"

.PHONY: test-performance
test-performance: ## Run performance tests
	@echo "Running performance tests..."
	cd $(PERF_DIR) && $(GOTEST) -bench=. -benchmem -benchtime=30s -run=^$$ .
	@echo "Performance tests complete"

.PHONY: test-optimization
test-optimization: ## Run optimization tests
	@echo "Running optimization tests..."
	cd $(OPTIMIZATION_DIR) && $(GOTEST) -v -timeout=20m .
	@echo "Optimization tests complete"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	$(GOTEST) -v -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report: coverage/coverage.html"

.PHONY: test-all
test-all: test test-integration test-e2e test-performance ## Run all tests

# Benchmark targets
.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem -benchtime=30s -run=^$$ $(PROCESSORS_DIR)/...
	@echo "Benchmarks complete"

.PHONY: benchmark-compare
benchmark-compare: ## Compare benchmarks with main branch
	@echo "Comparing benchmarks..."
	git stash
	git checkout main
	$(GOTEST) -bench=. -benchmem -benchtime=10s -run=^$$ $(PROCESSORS_DIR)/... > benchmark-main.txt
	git checkout -
	git stash pop
	$(GOTEST) -bench=. -benchmem -benchtime=10s -run=^$$ $(PROCESSORS_DIR)/... > benchmark-current.txt
	benchstat benchmark-main.txt benchmark-current.txt
	@echo "Benchmark comparison complete"

# Docker targets
.PHONY: docker-build-standard
docker-build-standard: ## Build standard Docker image (config-only mode)
	@echo "Building standard Docker image..."
	docker build -t $(STANDARD_IMAGE) -f deployments/docker/Dockerfile.standard .
	@echo "Docker image built: $(STANDARD_IMAGE)"

.PHONY: docker-build-enterprise
docker-build-enterprise: build-collector ## Build enterprise Docker image (with custom components)
	@echo "Building enterprise Docker image..."
	docker build -t $(ENTERPRISE_IMAGE) -f deployments/docker/Dockerfile.enterprise .
	@echo "Docker image built: $(ENTERPRISE_IMAGE)"

.PHONY: docker-build
docker-build: docker-build-standard docker-build-enterprise ## Build all Docker images

.PHONY: docker-push-standard
docker-push-standard: docker-build-standard ## Push standard Docker image
	@echo "Pushing standard Docker image..."
	docker push $(STANDARD_IMAGE)
	@echo "Standard image pushed"

.PHONY: docker-push-enterprise
docker-push-enterprise: docker-build-enterprise ## Push enterprise Docker image
	@echo "Pushing enterprise Docker image..."
	docker push $(ENTERPRISE_IMAGE)
	@echo "Enterprise image pushed"

.PHONY: docker-push
docker-push: docker-push-standard docker-push-enterprise ## Push all Docker images

.PHONY: docker-up
docker-up: ## Start test environment with Docker Compose
	@echo "Starting Docker environment..."
	cd deployments/docker && docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Docker environment ready"

.PHONY: docker-down
docker-down: ## Stop Docker Compose environment
	@echo "Stopping Docker environment..."
	cd deployments/docker && docker-compose down -v
	@echo "Docker environment stopped"

.PHONY: docker-logs
docker-logs: ## Show Docker logs
	cd deployments/docker && docker-compose logs -f

# Run targets
.PHONY: run
run: build ## Run the collector with default config
	@echo "Running collector..."
	./bin/$(BINARY_NAME) --config=configs/collector-config.yaml

.PHONY: run-debug
run-debug: build-debug ## Run with debug logging
	@echo "Running collector in debug mode..."
	LOG_LEVEL=debug ./bin/$(BINARY_NAME)-debug --config=configs/collector-config.yaml

.PHONY: run-resilient
run-resilient: build ## Run with resilient config
	@echo "Running collector with resilient config..."
	./bin/$(BINARY_NAME) --config=configs/collector-resilient.yaml

.PHONY: run-gateway
run-gateway: build ## Run in gateway mode
	@echo "Running collector in gateway mode..."
	./bin/$(BINARY_NAME) --config=configs/collector-gateway.yaml

# Configuration validation
.PHONY: validate-config
validate-config: build ## Validate all configuration files
	@echo "Validating configurations..."
	@for config in configs/*.yaml; do \
		echo "Validating $$config..."; \
		./bin/$(BINARY_NAME) --config=$$config --dry-run || exit 1; \
	done
	@echo "All configurations valid"

# Documentation targets
.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	go doc -all > $(DOCS_DIR)/API.md
	@echo "Documentation generated"

.PHONY: docs-serve
docs-serve: ## Serve documentation locally
	@echo "Serving documentation..."
	cd $(DOCS_DIR) && python -m http.server 8000

# Development targets
.PHONY: dev
dev: deps fmt vet lint test ## Run all development checks

.PHONY: pre-commit
pre-commit: fmt vet lint test-short ## Run pre-commit checks

.PHONY: ci
ci: deps lint vet test test-integration ## Run CI pipeline

# Profiling targets
.PHONY: profile-cpu
profile-cpu: ## Run CPU profiling
	@echo "Running CPU profiling..."
	$(GOTEST) -bench=. -benchtime=30s -cpuprofile=cpu.prof $(PERF_DIR)
	go tool pprof -http=:8080 cpu.prof

.PHONY: profile-mem
profile-mem: ## Run memory profiling
	@echo "Running memory profiling..."
	$(GOTEST) -bench=. -benchtime=30s -memprofile=mem.prof $(PERF_DIR)
	go tool pprof -http=:8080 mem.prof

.PHONY: profile-trace
profile-trace: ## Run execution trace
	@echo "Running execution trace..."
	$(GOTEST) -bench=. -benchtime=10s -trace=trace.out $(PERF_DIR)
	go tool trace trace.out

# Release targets
.PHONY: release-dry-run
release-dry-run: ## Perform a dry-run release
	@echo "Performing release dry-run..."
	goreleaser release --snapshot --clean

.PHONY: release
release: ## Create a new release
	@echo "Creating release..."
	goreleaser release --clean

# Monitoring targets
.PHONY: monitor
monitor: ## Start monitoring stack (Prometheus + Grafana)
	@echo "Starting monitoring stack..."
	cd deployments/monitoring && docker-compose up -d
	@echo "Monitoring available at:"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Grafana: http://localhost:3000"

.PHONY: monitor-down
monitor-down: ## Stop monitoring stack
	@echo "Stopping monitoring stack..."
	cd deployments/monitoring && docker-compose down -v

# Utility targets
.PHONY: check-updates
check-updates: ## Check for dependency updates
	@echo "Checking for updates..."
	go list -u -m all

.PHONY: mod-graph
mod-graph: ## Generate module dependency graph
	@echo "Generating dependency graph..."
	go mod graph | modgraphviz | dot -Tpng -o dependency-graph.png
	@echo "Dependency graph saved to dependency-graph.png"

.PHONY: todo
todo: ## List all TODOs in the codebase
	@echo "Finding TODOs..."
	@grep -r "TODO\|FIXME\|XXX" --exclude-dir=vendor --exclude-dir=.git --exclude=Makefile .

.PHONY: stats
stats: ## Show code statistics
	@echo "Code statistics:"
	@echo "Lines of code:"
	@find . -name "*.go" -not -path "./vendor/*" | xargs wc -l | tail -1
	@echo ""
	@echo "Number of files:"
	@find . -name "*.go" -not -path "./vendor/*" | wc -l
	@echo ""
	@echo "Test coverage:"
	@go test -cover ./... | grep -E "^(ok|\\?)" | awk '{print $$2 "\t" $$5}' | column -t

# Quick commands for common workflows
.PHONY: quick-test
quick-test: fmt vet test-short ## Quick test run for development

.PHONY: full-test
full-test: fmt vet lint test test-integration ## Full test suite

.PHONY: ready
ready: clean deps fmt vet lint test build ## Ensure project is ready

.PHONY: install
install: build ## Install binary to GOPATH/bin
	@echo "Installing to $(GOPATH)/bin..."
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "Installed successfully"