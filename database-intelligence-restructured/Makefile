# Database Intelligence - Unified Makefile
# Consolidates all build, test, and deployment operations

# Project metadata
PROJECT_NAME := database-intelligence
BINARY_NAME := database-intelligence-collector
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Docker configuration
DOCKER_REGISTRY ?= 
DOCKER_ORG ?= database-intelligence
DOCKER_IMAGE := $(if $(DOCKER_REGISTRY),$(DOCKER_REGISTRY)/,)$(DOCKER_ORG)/$(PROJECT_NAME)
DOCKER_TAG ?= $(VERSION)

# Directories
SCRIPTS_DIR := ./scripts
BUILD_DIR := ./distributions
CONFIGS_DIR := ./configs
CONFIG_DIR := ./configs
TESTS_DIR := ./tests

# Default shell
SHELL := /bin/bash

# Color output
NO_COLOR := \033[0m
OK_COLOR := \033[32;01m
ERROR_COLOR := \033[31;01m
WARN_COLOR := \033[33;01m

# Default target
.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "Database Intelligence Makefile\n\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: version
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

##@ Development

.PHONY: dev
dev: fmt lint test-unit ## Run all development checks
	@echo "$(OK_COLOR)Development checks passed!$(NO_COLOR)"

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -s -w .

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(WARN_COLOR)golangci-lint not installed, skipping$(NO_COLOR)"; \
	fi

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

.PHONY: security
security: ## Run security checks
	@echo "Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -quiet ./...; \
	else \
		echo "$(WARN_COLOR)gosec not installed, skipping$(NO_COLOR)"; \
	fi

##@ Build

.PHONY: build
build: ## Build production collector
	@echo "Building $(BINARY_NAME)..."
	@$(SCRIPTS_DIR)/build/build.sh production

.PHONY: build-all
build-all: ## Build all distributions (minimal, production, enterprise)
	@echo "Building all distributions..."
	@$(SCRIPTS_DIR)/build/build.sh all

.PHONY: build-minimal
build-minimal: ## Build minimal collector
	@echo "Building minimal collector..."
	@$(SCRIPTS_DIR)/build/build.sh minimal

.PHONY: build-debug
build-debug: ## Build with debug symbols
	@echo "Building debug binary..."
	@go build -gcflags="all=-N -l" $(LDFLAGS) -o $(BUILD_DIR)/debug/$(BINARY_NAME) ./distributions/production

.PHONY: build-multiplatform
build-multiplatform: ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@$(SCRIPTS_DIR)/build/build.sh multiplatform

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@$(SCRIPTS_DIR)/build/build.sh docker DOCKER_TAG=$(DOCKER_TAG)

.PHONY: docker-push
docker-push: docker-build ## Push Docker images
	@echo "Pushing Docker images..."
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@docker push $(DOCKER_IMAGE):latest

.PHONY: docker-up
docker-up: ## Start Docker environment
	@echo "Starting Docker environment..."
	@cd deployments/docker && docker compose up -d

.PHONY: docker-down
docker-down: ## Stop Docker environment
	@echo "Stopping Docker environment..."
	@cd deployments/docker && docker compose down

.PHONY: docker-logs
docker-logs: ## Show Docker logs
	@cd deployments/docker && docker compose logs -f

.PHONY: docker-clean
docker-clean: ## Clean Docker resources
	@echo "Cleaning Docker resources..."
	@cd deployments/docker && docker compose down -v
	@docker image prune -f

##@ Testing

.PHONY: test
test: test-unit test-integration ## Run all tests

.PHONY: test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	@$(SCRIPTS_DIR)/test/test.sh unit

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@$(SCRIPTS_DIR)/test/test.sh integration

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "Running E2E tests..."
	@$(SCRIPTS_DIR)/test/test.sh e2e

.PHONY: test-components
test-components: ## Test component builds
	@echo "Testing components..."
	@$(SCRIPTS_DIR)/test/test.sh components

.PHONY: test-otlp
test-otlp: ## Run OTLP compliance tests
	@echo "Running OTLP tests..."
	@$(SCRIPTS_DIR)/test/test.sh otlp

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@COVERAGE=true $(SCRIPTS_DIR)/test/test.sh all

.PHONY: test-performance
test-performance: ## Run performance tests
	@echo "Running performance tests..."
	@$(SCRIPTS_DIR)/test/test.sh performance

##@ Deployment

.PHONY: deploy
deploy: ## Deploy to production (Docker by default)
	@echo "Deploying to production..."
	@$(SCRIPTS_DIR)/deploy/deploy.sh docker production

.PHONY: deploy-staging
deploy-staging: ## Deploy to staging
	@echo "Deploying to staging..."
	@$(SCRIPTS_DIR)/deploy/deploy.sh docker staging

.PHONY: deploy-k8s
deploy-k8s: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	@$(SCRIPTS_DIR)/deploy/deploy.sh kubernetes production

.PHONY: deploy-binary
deploy-binary: ## Deploy as system service
	@echo "Deploying as system service..."
	@$(SCRIPTS_DIR)/deploy/deploy.sh binary production

.PHONY: deploy-parallel
deploy-parallel: ## Deploy parallel modes (config-only + enhanced)
	@echo "Deploying parallel modes..."
	@$(SCRIPTS_DIR)/deploy/deploy.sh parallel production

##@ Configuration

.PHONY: validate-config
validate-config: ## Validate all configuration files
	@echo "Validating configurations..."
	@for config in $(CONFIGS_DIR)/**/*.yaml; do \
		echo "Validating $$config..."; \
		$(BINARY_NAME) --config=$$config --dry-run || exit 1; \
	done

.PHONY: generate-config
generate-config: ## Generate configuration from templates
	@echo "Generating configuration..."
	@./scripts/utils/generate-config.sh

##@ Running

.PHONY: run
run: build ## Build and run collector
	@echo "Running collector..."
	@$(BUILD_DIR)/production/$(BINARY_NAME) --config=$(CONFIGS_DIR)/base.yaml

.PHONY: run-debug
run-debug: build-debug ## Run with debug logging
	@echo "Running in debug mode..."
	@LOG_LEVEL=debug $(BUILD_DIR)/debug/$(BINARY_NAME) --config=$(CONFIGS_DIR)/base.yaml

.PHONY: run-config-only
run-config-only: ## Run config-only mode
	@echo "Running config-only mode..."
	@docker run --rm -it \
		-v $(PWD)/configs/modes/config-only.yaml:/etc/otel/config.yaml \
		--env-file .env \
		otel/opentelemetry-collector-contrib:latest

.PHONY: dev-run
dev-run: build run ## Quick development cycle (build + run)

##@ Utilities

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)/*/bin
	@rm -rf $(BUILD_DIR)/*/_build
	@rm -rf test-results/ coverage/
	@go clean -cache

.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Managing dependencies..."
	@go mod download
	@go mod tidy

.PHONY: install-tools
install-tools: ## Install required development tools
	@echo "Installing tools..."
	@go install go.opentelemetry.io/collector/cmd/builder@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install gotest.tools/gotestsum@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest

.PHONY: generate
generate: ## Run code generation
	@echo "Running code generation..."
	@go generate ./...

.PHONY: verify
verify: fmt vet lint security ## Verify code quality
	@echo "$(OK_COLOR)Code verification passed!$(NO_COLOR)"

##@ Database Operations

.PHONY: db-start
db-start: ## Start test databases
	@echo "Starting test databases..."
	@cd deployments/docker && docker compose -f compose/docker-compose-databases.yaml up -d

.PHONY: db-stop
db-stop: ## Stop test databases
	@echo "Stopping test databases..."
	@cd deployments/docker && docker compose -f compose/docker-compose-databases.yaml down

.PHONY: db-reset
db-reset: db-stop db-start ## Reset test databases
	@echo "Databases reset"

.PHONY: db-shell-postgres
db-shell-postgres: ## Connect to PostgreSQL shell
	@docker exec -it postgres psql -U postgres

.PHONY: db-shell-mysql
db-shell-mysql: ## Connect to MySQL shell
	@docker exec -it mysql mysql -u root -proot

##@ Monitoring

.PHONY: metrics
metrics: ## Show collector metrics
	@curl -s http://localhost:8888/metrics | grep -E '^otelcol|^db_'

.PHONY: health
health: ## Check collector health
	@curl -s http://localhost:13133/health | jq .

.PHONY: status
status: ## Show deployment status
	@if pgrep -f $(BINARY_NAME) > /dev/null; then \
		echo "$(OK_COLOR)Collector is running$(NO_COLOR)"; \
		curl -s http://localhost:13133/health | jq .; \
	else \
		echo "$(ERROR_COLOR)Collector is not running$(NO_COLOR)"; \
	fi

##@ Documentation

.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	@go doc -all ./... > docs/API.md

.PHONY: serve-docs
serve-docs: ## Serve documentation locally
	@echo "Serving documentation at http://localhost:8000"
	@cd docs && python3 -m http.server 8000

##@ CI/CD

.PHONY: ci
ci: deps verify test ## Run CI pipeline
	@echo "$(OK_COLOR)CI pipeline passed!$(NO_COLOR)"

.PHONY: pre-commit
pre-commit: fmt lint test-unit ## Run pre-commit checks
	@echo "$(OK_COLOR)Pre-commit checks passed!$(NO_COLOR)"

.PHONY: release
release: clean verify test build docker-build ## Prepare release
	@echo "$(OK_COLOR)Release preparation complete!$(NO_COLOR)"
	@echo "Version: $(VERSION)"

# Include environment-specific targets if they exist
-include Makefile.local