# Database Intelligence MVP - Docker Makefile
# Provides convenient commands for building and managing Docker containers

# Configuration
IMAGE_NAME := newrelic/database-intelligence-collector
VERSION := mvp-1.0
DOCKER_REGISTRY := registry.newrelic.com
NAMESPACE := monitoring

# Build metadata
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
VCS_REF := $(shell git rev-parse --short HEAD)
VCS_URL := $(shell git config --get remote.origin.url)

# Docker build arguments
BUILD_ARGS := \
	--build-arg BUILD_DATE=$(BUILD_DATE) \
	--build-arg VCS_REF=$(VCS_REF) \
	--build-arg VCS_URL=$(VCS_URL) \
	--build-arg VERSION=$(VERSION)

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Database Intelligence MVP - Docker Commands"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the Docker image
	@echo "Building Database Intelligence collector image..."
	@docker build \
		$(BUILD_ARGS) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		-f Dockerfile \
		../../
	@echo "Build completed: $(IMAGE_NAME):$(VERSION)"

.PHONY: build-no-cache
build-no-cache: ## Build the Docker image without cache
	@echo "Building Database Intelligence collector image (no cache)..."
	@docker build \
		$(BUILD_ARGS) \
		--no-cache \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		-f Dockerfile \
		../../
	@echo "Build completed: $(IMAGE_NAME):$(VERSION)"

.PHONY: security-scan
security-scan: ## Run security scan on the image
	@echo "Running security scan..."
	@docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy:latest image $(IMAGE_NAME):$(VERSION)

.PHONY: validate-env
validate-env: ## Validate environment configuration
	@echo "Validating environment configuration..."
	@if [ ! -f .env ]; then \
		echo "ERROR: .env file not found. Copy .env.template to .env and configure."; \
		exit 1; \
	fi
	@echo "Environment file found and validated"

.PHONY: setup-data-dirs
setup-data-dirs: ## Create required data directories
	@echo "Creating data directories..."
	@mkdir -p data/state data/logs data/certs
	@chmod 755 data/state data/logs data/certs
	@echo "Data directories created"

.PHONY: up
up: validate-env setup-data-dirs ## Start the collector with docker-compose
	@echo "Starting Database Intelligence collector..."
	@docker-compose up -d
	@echo "Collector started. Use 'make logs' to view logs."

.PHONY: up-with-monitoring
up-with-monitoring: validate-env setup-data-dirs ## Start collector with monitoring stack
	@echo "Starting Database Intelligence collector with monitoring..."
	@docker-compose --profile monitoring up -d
	@echo "Collector and monitoring stack started."

.PHONY: down
down: ## Stop and remove containers
	@echo "Stopping Database Intelligence collector..."
	@docker-compose down
	@echo "Collector stopped"

.PHONY: restart
restart: down up ## Restart the collector

.PHONY: logs
logs: ## Show collector logs
	@docker-compose logs -f db-intelligence-collector

.PHONY: logs-all
logs-all: ## Show all container logs
	@docker-compose logs -f

.PHONY: status
status: ## Show container status
	@docker-compose ps

.PHONY: health
health: ## Check collector health
	@echo "Checking collector health..."
	@curl -f http://localhost:13133/ || echo "Health check failed"

.PHONY: metrics
metrics: ## Show collector metrics
	@echo "Fetching collector metrics..."
	@curl -s http://localhost:8888/metrics | head -20

.PHONY: exec
exec: ## Execute shell in collector container
	@docker-compose exec db-intelligence-collector sh

.PHONY: test-db-connection
test-db-connection: ## Test database connectivity
	@echo "Testing database connectivity..."
	@docker-compose exec db-intelligence-collector sh -c 'psql "$$PG_REPLICA_DSN" -c "SELECT version();"'

.PHONY: backup-state
backup-state: ## Backup collector state
	@echo "Backing up collector state..."
	@mkdir -p backups
	@docker run --rm -v $$(pwd)/data/state:/source:ro -v $$(pwd)/backups:/backup \
		alpine:latest tar czf /backup/state-backup-$$(date +%Y%m%d-%H%M%S).tar.gz -C /source .
	@echo "State backup completed"

.PHONY: restore-state
restore-state: ## Restore collector state from backup (requires BACKUP_FILE)
	@if [ -z "$(BACKUP_FILE)" ]; then \
		echo "ERROR: BACKUP_FILE not specified. Use: make restore-state BACKUP_FILE=backup.tar.gz"; \
		exit 1; \
	fi
	@echo "Restoring collector state from $(BACKUP_FILE)..."
	@docker-compose down
	@docker run --rm -v $$(pwd)/data/state:/target -v $$(pwd)/backups:/backup \
		alpine:latest tar xzf /backup/$(BACKUP_FILE) -C /target
	@echo "State restored. Use 'make up' to start collector."

.PHONY: clean
clean: ## Remove stopped containers and unused images
	@echo "Cleaning up Docker resources..."
	@docker-compose down -v
	@docker system prune -f
	@echo "Cleanup completed"

.PHONY: clean-all
clean-all: ## Remove all containers, images, and volumes
	@echo "WARNING: This will remove ALL Docker resources including volumes!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose down -v; \
		docker rmi $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME):latest 2>/dev/null || true; \
		docker system prune -af; \
		rm -rf data/; \
		echo "Complete cleanup finished"; \
	else \
		echo "Cleanup cancelled"; \
	fi

.PHONY: push
push: ## Push image to registry
	@echo "Pushing image to registry..."
	@docker tag $(IMAGE_NAME):$(VERSION) $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)
	@docker tag $(IMAGE_NAME):$(VERSION) $(DOCKER_REGISTRY)/$(IMAGE_NAME):latest
	@docker push $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION)
	@docker push $(DOCKER_REGISTRY)/$(IMAGE_NAME):latest
	@echo "Image pushed to registry"

.PHONY: release
release: build security-scan push ## Build, scan, and push image

.PHONY: debug
debug: ## Start collector in debug mode
	@echo "Starting collector in debug mode..."
	@docker-compose -f docker-compose.yml -f docker-compose.debug.yml up -d
	@echo "Debug mode started. zpages available at http://localhost:55679"

.PHONY: generate-env
generate-env: ## Generate .env file from template
	@if [ -f .env ]; then \
		echo "ERROR: .env file already exists. Remove it first if you want to regenerate."; \
		exit 1; \
	fi
	@cp .env.template .env
	@echo ".env file generated from template. Please edit it with your actual values."

.PHONY: validate-config
validate-config: ## Validate OpenTelemetry configuration
	@echo "Validating collector configuration..."
	@docker run --rm -v $$(pwd)/config:/etc/otel:ro $(IMAGE_NAME):$(VERSION) \
		otelcol-contrib validate --config=/etc/otel/collector.yaml

.PHONY: performance-test
performance-test: ## Run basic performance test
	@echo "Running performance test..."
	@docker run --rm --network container:nr-db-intelligence-collector \
		appropriate/curl:latest \
		-w "@/dev/stdin" <<< "time_total: %{time_total}\nsize_download: %{size_download}\n" \
		http://localhost:8888/metrics

.PHONY: monitoring-urls
monitoring-urls: ## Show monitoring URLs
	@echo "Monitoring URLs:"
	@echo "  Health Check:   http://localhost:13133/"
	@echo "  Metrics:        http://localhost:8888/metrics"
	@echo "  Debug Pages:    http://localhost:55679/debug/"
	@echo "  Prometheus:     http://localhost:9090 (if monitoring profile enabled)"
	@echo "  Grafana:        http://localhost:3000 (if monitoring profile enabled)"

# Development targets
.PHONY: dev-setup
dev-setup: generate-env setup-data-dirs ## Set up development environment
	@echo "Development environment setup completed"
	@echo "1. Edit .env file with your credentials"
	@echo "2. Run 'make up' to start the collector"

.PHONY: dev-logs
dev-logs: ## Show detailed logs for development
	@docker-compose logs -f --tail=100 db-intelligence-collector

.PHONY: dev-shell
dev-shell: ## Start development shell with mounted source
	@docker run --rm -it \
		-v $$(pwd)/../../:/workspace \
		-w /workspace \
		golang:1.21-alpine sh

# CI/CD targets
.PHONY: ci-build
ci-build: ## Build for CI/CD pipeline
	@echo "Building for CI/CD..."
	@docker build \
		$(BUILD_ARGS) \
		--label "org.opencontainers.image.created=$(BUILD_DATE)" \
		--label "org.opencontainers.image.revision=$(VCS_REF)" \
		--label "org.opencontainers.image.source=$(VCS_URL)" \
		--label "org.opencontainers.image.version=$(VERSION)" \
		-t $(IMAGE_NAME):$(VERSION) \
		-f Dockerfile \
		../../

.PHONY: ci-test
ci-test: ci-build ## Run tests in CI/CD
	@echo "Running CI tests..."
	@docker run --rm $(IMAGE_NAME):$(VERSION) otelcol-contrib validate --config=/etc/otel/config.yaml

# Display build info
.PHONY: info
info: ## Show build information
	@echo "Build Information:"
	@echo "  Image:       $(IMAGE_NAME):$(VERSION)"
	@echo "  Build Date:  $(BUILD_DATE)"
	@echo "  VCS Ref:     $(VCS_REF)"
	@echo "  VCS URL:     $(VCS_URL)"