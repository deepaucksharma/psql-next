# Database Intelligence MVP - OTEL-First Makefile
# This Makefile supports building a custom OpenTelemetry collector with
# standard OTEL components and minimal custom processors for gaps.

.PHONY: all build test clean run install-tools help docker-up docker-down lint fmt vet deps
.PHONY: verify quickstart docker-simple docker-prod k8s-minimal k8s-prod test-integration
.PHONY: build-image push-image generate-config validate-config collector-debug
.PHONY: test-unit test-e2e test-benchmark test-ohi-validation migrate-ohi

# Variables
BINARY_NAME := otelcol-db-intelligence
DIST_DIR := dist
CONFIG_DIR := config
DOCKER_REGISTRY ?= docker.io
DOCKER_ORG ?= database-intelligence
DOCKER_TAG ?= latest
COLLECTOR_CONFIG ?= $(CONFIG_DIR)/collector.yaml

# Default target
all: deps build test

## Primary targets

build: ## Build the OTEL-first collector
	@echo "🔨 Building OTEL-first collector..."
	@if [ -f ~/go/bin/builder ]; then \
		echo "Using OpenTelemetry Collector Builder..."; \
		~/go/bin/builder --config=ocb-config.yaml; \
	elif command -v builder >/dev/null 2>&1; then \
		echo "Using builder from PATH..."; \
		builder --config=ocb-config.yaml; \
	else \
		echo "❌ Error: OCB/builder not found. Run 'make install-tools' first."; \
		exit 1; \
	fi

test: test-unit test-integration test-e2e ## Run all tests

test-unit: ## Run unit tests
	@echo "🧪 Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Unit tests completed"

test-integration: ## Run integration tests
	@echo "🧪 Running integration tests..."
	@go test -v -tags=integration ./tests/integration/...
	@echo "✅ Integration tests completed"

test-e2e: ## Run end-to-end tests (database to NRDB)
	@echo "🧪 Running E2E tests..."
	@go test -v -timeout 10m ./tests/e2e/...
	@echo "✅ E2E tests completed"

test-benchmark: ## Run performance benchmarks
	@echo "📏 Running performance benchmarks..."
	@go test -bench=. -benchmem ./tests/benchmark/...
	@echo "✅ Benchmarks completed"

test-ohi-validation: ## Run OHI compatibility validation
	@echo "🔍 Running OHI validation..."
	@cd validation && go run ohi-compatibility-validator.go
	@echo "✅ OHI validation completed"

clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(DIST_DIR) bin/ tmp/ coverage.out
	@go clean -cache
	@echo "✅ Clean completed"

run: build ## Build and run the collector
	@echo "🚀 Running OTEL collector..."
	@$(DIST_DIR)/$(BINARY_NAME) --config=$(COLLECTOR_CONFIG)

## Development targets

lint: ## Run linters
	@echo "🔍 Running linters..."
	@golangci-lint run --timeout=5m ./processors/...
	@echo "✅ Linting completed"

fmt: ## Format code
	@echo "📝 Formatting code..."
	@go fmt ./...
	@goimports -w ./processors/
	@echo "✅ Formatting completed"

vet: ## Run go vet
	@echo "🔍 Running go vet..."
	@go vet ./...
	@echo "✅ Vet completed"

## Installation targets

install-tools: ## Install required tools
	@echo "📦 Installing required tools..."
	@go install go.opentelemetry.io/collector/cmd/builder@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "✅ Tools installed"

## Docker targets

docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	@docker build -t $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$(BINARY_NAME):$(DOCKER_TAG) .
	@echo "✅ Docker image built"

docker-push: docker-build ## Push Docker image
	@echo "📤 Pushing Docker image..."
	@docker push $(DOCKER_REGISTRY)/$(DOCKER_ORG)/$(BINARY_NAME):$(DOCKER_TAG)
	@echo "✅ Docker image pushed"

docker-up: ## Start production services with Docker Compose
	@echo "🚀 Starting production services..."
	@docker-compose -f deploy/docker/docker-compose.yaml up -d
	@echo "✅ Services started. Check health: http://localhost:13133/"

docker-down: ## Stop all Docker Compose services
	@echo "🛑 Stopping Docker Compose services..."
	@docker-compose -f deploy/docker/docker-compose.yaml down
	@echo "✅ Services stopped"

docker-simple: ## Start simple development setup
	@echo "🚀 Starting simple development setup..."
	@cd deploy/examples && docker-compose -f docker-compose-simple.yaml up -d
	@echo "✅ Simple setup started. Dashboard: http://localhost:3000"

docker-prod: ## Start full production setup
	@echo "🚀 Starting production setup..."
	@cd deploy/examples && docker-compose -f docker-compose-production.yaml up -d
	@echo "✅ Production setup started. Grafana: http://localhost:3000"

## Kubernetes targets

k8s-minimal: ## Deploy minimal Kubernetes setup
	@echo "☸️  Deploying minimal Kubernetes setup..."
	@kubectl apply -f deploy/examples/kubernetes-minimal.yaml
	@echo "✅ Minimal K8s deployment complete"

k8s-prod: ## Deploy production Kubernetes setup
	@echo "☸️  Deploying production Kubernetes setup..."
	@kubectl apply -f deploy/examples/kubernetes-production.yaml
	@echo "✅ Production K8s deployment complete"

## Verification targets

verify: ## Run verification checks
	@echo "✅ Running verification..."
	@./scripts/verify-metrics.sh
	@echo "✅ Verification complete"

quickstart: ## Run quickstart setup
	@echo "🚀 Running quickstart..."
	@./scripts/init-env.sh
	@make build
	@make docker-simple
	@echo "✅ Quickstart complete"

## Configuration targets

generate-config: ## Generate collector configuration
	@echo "📝 Generating collector configuration..."
	@./scripts/generate-config.sh > $(CONFIG_DIR)/collector-generated.yaml
	@echo "✅ Configuration generated"

validate-config: ## Validate collector configuration
	@echo "🔍 Validating collector configuration..."
	@$(DIST_DIR)/$(BINARY_NAME) validate --config=$(COLLECTOR_CONFIG)
	@echo "✅ Configuration valid"

## Debug targets

collector-debug: build ## Run collector with debug logging
	@echo "🐛 Running collector in debug mode..."
	@$(DIST_DIR)/$(BINARY_NAME) --config=$(COLLECTOR_CONFIG) --log-level=debug

## Dependency management

deps: ## Download and tidy dependencies
	@echo "📦 Managing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies updated"

deps-upgrade: ## Upgrade all dependencies
	@echo "⬆️  Upgrading dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "✅ Dependencies upgraded"

## Documentation

docs: ## Generate documentation
	@echo "📚 Generating documentation..."
	@go doc -all ./processors/... > docs/API.md
	@echo "✅ Documentation generated"

## Metrics and monitoring

metrics: ## Show current metrics
	@echo "📊 Current metrics:"
	@curl -s http://localhost:8888/metrics | grep -E "^(# HELP|# TYPE|db_|postgresql_|mysql_)"

health: ## Check collector health
	@echo "🏥 Checking collector health..."
	@curl -s http://localhost:13133/ | jq '.' || echo "Collector not running"

## OHI Migration targets

migrate-ohi: build ## Run OHI migration collector
	@echo "🔄 Starting OHI migration collector..."
	@$(DIST_DIR)/$(BINARY_NAME) --config=config/collector-ohi-migration.yaml

validate-ohi: build ## Validate OHI compatibility configuration
	@echo "✔️ Validating OHI configuration..."
	@$(DIST_DIR)/$(BINARY_NAME) validate --config=config/collector-ohi-migration.yaml

## Help

help: ## Display this help message
	@echo "Database Intelligence MVP - v2.0.0 with OHI Migration Support"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Primary targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '^(build|test|run|clean):' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Docker targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '^docker-' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Kubernetes targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '^k8s-' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Development targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '^(lint|fmt|vet|deps):' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Other targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -vE '^(build|test|run|clean|docker-|k8s-|lint|fmt|vet|deps):' | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
