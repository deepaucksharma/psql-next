# Database Intelligence MVP - Main Makefile
# Orchestrates build, test, and deployment operations

# Project metadata
PROJECT_NAME := database-intelligence-mvp
VERSION := mvp-1.0
BUILD_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

# Build configuration
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
BINARY_DIR := ./bin
DOCKER_REGISTRY := registry.newrelic.com
IMAGE_NAME := $(PROJECT_NAME)

# Test configuration
TEST_TIMEOUT := 30m
COVERAGE_FILE := coverage.out

# Default target
.PHONY: help
help: ## Show this help message
	@echo "Database Intelligence MVP - Build System"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Build Information:"
	@echo "  Project:     $(PROJECT_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Git Commit:  $(GIT_COMMIT)"
	@echo "  Git Branch:  $(GIT_BRANCH)"
	@echo "  Build Date:  $(BUILD_DATE)"
	@echo "  Target OS:   $(GOOS)/$(GOARCH)"

# ============================================================================
# Build Targets
# ============================================================================

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BINARY_DIR)
	@rm -f $(COVERAGE_FILE)
	@rm -rf ./tests/integration/test-results
	@rm -rf ./tests/load/results
	@go clean -cache
	@echo "Clean completed"



.PHONY: build
build: ## Build all custom processors
	@echo "Building custom processors..."
	@mkdir -p $(BINARY_DIR)
	
	@echo "Building plan attribute extractor..."
	@cd processors/planattributeextractor && \
		go build -ldflags="-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)" \
		-o ../../$(BINARY_DIR)/planattributeextractor .
	
	@echo "Building adaptive sampler..."
	@cd processors/adaptivesampler && \
		go build -ldflags="-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)" \
		-o ../../$(BINARY_DIR)/adaptivesampler .
	
	@echo "Building circuit breaker..."
	@cd processors/circuitbreaker && \
		go build -ldflags="-X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.gitCommit=$(GIT_COMMIT)" \
		-o ../../$(BINARY_DIR)/circuitbreaker .
	
	@echo "Build completed successfully"

.PHONY: build-release
build-release: ## Build release binaries for multiple platforms
	@echo "Building release binaries..."
	@mkdir -p $(BINARY_DIR)/linux-amd64 $(BINARY_DIR)/linux-arm64 $(BINARY_DIR)/darwin-amd64 $(BINARY_DIR)/darwin-arm64
	
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 $(MAKE) build
	@mv $(BINARY_DIR)/* $(BINARY_DIR)/linux-amd64/
	
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 $(MAKE) build
	@mv $(BINARY_DIR)/* $(BINARY_DIR)/linux-arm64/
	
	# Darwin AMD64
	@GOOS=darwin GOARCH=amd64 $(MAKE) build
	@mv $(BINARY_DIR)/* $(BINARY_DIR)/darwin-amd64/
	
	# Darwin ARM64
	@GOOS=darwin GOARCH=arm64 $(MAKE) build
	@mv $(BINARY_DIR)/* $(BINARY_DIR)/darwin-arm64/
	
	@echo "Release binaries built for all platforms"

# ============================================================================
# Test Targets
# ============================================================================

.PHONY: test
test: ## Run all unit tests
	@echo "Running unit tests..."
	@cd processors/planattributeextractor && go test -v -timeout $(TEST_TIMEOUT) ./...
	@cd processors/adaptivesampler && go test -v -timeout $(TEST_TIMEOUT) ./...
	@cd processors/circuitbreaker && go test -v -timeout $(TEST_TIMEOUT) ./...
	@cd processors/verification && go test -v -timeout $(TEST_TIMEOUT) ./...
	@cd tests/unit && go test -v -timeout $(TEST_TIMEOUT) ./...
	@echo "Unit tests completed"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	
	@cd processors/planattributeextractor && \
		go test -coverprofile=../../coverage/planattributeextractor.out -covermode=atomic ./...
	
	@cd processors/adaptivesampler && \
		go test -coverprofile=../../coverage/adaptivesampler.out -covermode=atomic ./...
	
	@cd processors/circuitbreaker && \
		go test -coverprofile=../../coverage/circuitbreaker.out -covermode=atomic ./...
	
	# Merge coverage files
	@echo "mode: atomic" > $(COVERAGE_FILE)
	@grep -h -v "mode: atomic" coverage/*.out >> $(COVERAGE_FILE) || true
	
	@go tool cover -html=$(COVERAGE_FILE) -o coverage/coverage.html
	@go tool cover -func=$(COVERAGE_FILE)
	@echo "Coverage report generated: coverage/coverage.html"

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@./tests/integration/test-postgresql.sh
	@echo "Integration tests completed"

.PHONY: test-load
test-load: ## Run load tests
	@echo "Running load tests..."
	@./tests/load/load-test.sh --duration 120 --concurrent-users 5
	@echo "Load tests completed"

.PHONY: test-all
test-all: test test-coverage test-integration ## Run all tests

# ============================================================================
# Code Quality Targets
# ============================================================================

.PHONY: lint
lint: ## Run code linting
	@echo "Running code linting..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd processors/planattributeextractor && golangci-lint run; \
		cd ../adaptivesampler && golangci-lint run; \
		cd ../circuitbreaker && golangci-lint run; \
		cd ../verification && golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping lint"; \
	fi

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	@cd processors/planattributeextractor && go fmt ./...
	@cd processors/adaptivesampler && go fmt ./...
	@cd processors/circuitbreaker && go fmt ./...
	@cd processors/verification && go fmt ./...
	@echo "Code formatting completed"

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@cd processors/planattributeextractor && go vet ./...
	@cd processors/adaptivesampler && go vet ./...
	@cd processors/circuitbreaker && go vet ./...
	@cd processors/verification && go vet ./...
	@echo "Vet completed"

.PHONY: security-scan
security-scan: ## Run security scanning
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		cd processors/planattributeextractor && gosec ./...; \
		cd ../adaptivesampler && gosec ./...; \
		cd ../circuitbreaker && gosec ./...; \
		cd ../verification && gosec ./...; \
	else \
		echo "gosec not installed, skipping security scan"; \
	fi

.PHONY: quality
quality: fmt vet lint security-scan ## Run all code quality checks

# ============================================================================
# Docker Targets
# ============================================================================

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@cd deployments/docker && $(MAKE) build
	@echo "Docker image built successfully"

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image..."
	@cd deployments/docker && $(MAKE) push
	@echo "Docker image pushed successfully"

.PHONY: docker-run
docker-run: ## Run collector in Docker
	@echo "Starting collector in Docker..."
	@cd deployments/docker && $(MAKE) up
	@echo "Collector started - use 'make docker-logs' to view logs"

.PHONY: docker-stop
docker-stop: ## Stop Docker containers
	@echo "Stopping Docker containers..."
	@cd deployments/docker && $(MAKE) down
	@echo "Docker containers stopped"

.PHONY: docker-logs
docker-logs: ## Show Docker container logs
	@cd deployments/docker && $(MAKE) logs

.PHONY: docker-clean
docker-clean: ## Clean Docker resources
	@echo "Cleaning Docker resources..."
	@cd deployments/docker && $(MAKE) clean
	@echo "Docker cleanup completed"

# ============================================================================
# Kubernetes Targets
# ============================================================================

.PHONY: k8s-deploy
k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	@cd deployments/kubernetes && ./deploy.sh
	@echo "Kubernetes deployment completed"

.PHONY: k8s-status
k8s-status: ## Check Kubernetes deployment status
	@echo "Checking Kubernetes status..."
	@kubectl get statefulset -n monitoring nr-db-intelligence-collector
	@kubectl get pods -n monitoring -l app=nr-db-intelligence
	@kubectl get services -n monitoring -l app=nr-db-intelligence

.PHONY: k8s-logs
k8s-logs: ## Show Kubernetes pod logs
	@kubectl logs -n monitoring -l app=nr-db-intelligence -f

.PHONY: k8s-delete
k8s-delete: ## Delete Kubernetes deployment
	@echo "Deleting Kubernetes deployment..."
	@kubectl delete -f deploy/k8s/statefulset.yaml || true
	@kubectl delete -f deploy/k8s/configmap.yaml || true
	@kubectl delete -f deploy/k8s/secrets.yaml || true
	@kubectl delete -f deploy/k8s/rbac.yaml || true
	@kubectl delete -f deploy/k8s/network-policy.yaml || true
	@echo "Kubernetes resources deleted"

# ============================================================================
# Validation Targets
# ============================================================================

.PHONY: validate-prerequisites
validate-prerequisites: ## Validate database prerequisites
	@echo "Validating database prerequisites..."
	@./scripts/validate-prerequisites.sh
	@echo "Prerequisites validation completed"

.PHONY: validate-config
validate-config: ## Validate collector configuration
	@echo "Validating collector configuration..."
	@if command -v otelcol-contrib >/dev/null 2>&1; then \
		otelcol-contrib validate --config=config/collector.yaml; \
	else \
		echo "otelcol-contrib not available, skipping config validation"; \
	fi

.PHONY: validate-deployment
validate-deployment: validate-prerequisites validate-config ## Validate deployment readiness
	@echo "Deployment validation completed"

# ============================================================================
# Documentation Targets
# ============================================================================

.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	@mkdir -p docs/generated
	
	# Generate processor documentation
	@echo "# Plan Attribute Extractor" > docs/generated/planattributeextractor.md
	@echo "" >> docs/generated/planattributeextractor.md
	@cat processors/planattributeextractor/README.md >> docs/generated/planattributeextractor.md
	
	@echo "# Adaptive Sampler" > docs/generated/adaptivesampler.md
	@echo "" >> docs/generated/adaptivesampler.md
	@cat processors/adaptivesampler/README.md >> docs/generated/adaptivesampler.md
	
	@echo "Documentation generated in docs/generated/"

.PHONY: docs-serve
docs-serve: docs ## Serve documentation locally
	@echo "Serving documentation at http://localhost:8080"
	@if command -v python3 >/dev/null 2>&1; then \
		cd docs && python3 -m http.server 8080; \
	elif command -v python >/dev/null 2>&1; then \
		cd docs && python -m SimpleHTTPServer 8080; \
	else \
		echo "Python not available for serving docs"; \
	fi

# ============================================================================
# Release Targets
# ============================================================================

.PHONY: pre-release
pre-release: clean quality test-all validate-deployment ## Run pre-release checks
	@echo "Pre-release checks completed successfully"
	@echo "Ready for release $(VERSION)"

.PHONY: release
release: pre-release build-release docker-build ## Create a release
	@echo "Creating release $(VERSION)..."
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "Release $(VERSION) created"
	@echo "To publish: git push origin $(VERSION)"

.PHONY: release-notes
release-notes: ## Generate release notes
	@echo "Generating release notes for $(VERSION)..."
	@echo "# Release $(VERSION)" > RELEASE_NOTES.md
	@echo "" >> RELEASE_NOTES.md
	@echo "## Changes" >> RELEASE_NOTES.md
	@git log --oneline $(shell git describe --tags --abbrev=0)..HEAD >> RELEASE_NOTES.md || echo "- Initial release" >> RELEASE_NOTES.md
	@echo "Release notes generated: RELEASE_NOTES.md"

# ============================================================================
# Development Targets
# ============================================================================

.PHONY: dev-setup
dev-setup: ## Set up development environment
	@echo "Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/goat/cmd/gosec@latest
	@mkdir -p .git/hooks
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Development environment setup completed"

.PHONY: dev-run
dev-run: build ## Run collector in development mode
	@echo "Running collector in development mode..."
	@cd deployments/docker && $(MAKE) dev-setup && $(MAKE) up
	@echo "Development environment started"

.PHONY: dev-test
dev-test: ## Run tests in development mode
	@echo "Running development tests..."
	@$(MAKE) test-coverage
	@$(MAKE) test-integration
	@echo "Development tests completed"

# ============================================================================
# CI/CD Targets
# ============================================================================

.PHONY: ci
ci: clean deps quality test-coverage ## Run CI pipeline
	@echo "CI pipeline completed successfully"

.PHONY: cd
cd: ci docker-build docker-push ## Run CD pipeline
	@echo "CD pipeline completed successfully"

# ============================================================================
# Utility Targets
# ============================================================================

.PHONY: version
version: ## Show version information
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Branch: $(GIT_BRANCH)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Target: $(GOOS)/$(GOARCH)"

.PHONY: env
env: ## Show environment information
	@echo "Environment Information:"
	@echo "  Go Version: $(shell go version)"
	@echo "  Docker: $(shell docker --version 2>/dev/null || echo 'Not available')"
	@echo "  Kubernetes: $(shell kubectl version --client --short 2>/dev/null || echo 'Not available')"
	@echo "  Git: $(shell git --version 2>/dev/null || echo 'Not available')"

.PHONY: install-tools
install-tools: ## Install required development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/goat/cmd/gosec@latest
	@echo "Development tools installed"

# ============================================================================
# Monitoring Targets
# ============================================================================

.PHONY: health-check
health-check: ## Check collector health
	@echo "Checking collector health..."
	@curl -f http://localhost:13133/ && echo "Health check: PASS" || echo "Health check: FAIL"

.PHONY: metrics
metrics: ## Show collector metrics
	@echo "Fetching collector metrics..."
	@curl -s http://localhost:8888/metrics | head -20

.PHONY: debug
debug: ## Show debug information
	@echo "Debug information:"
	@curl -s http://localhost:55679/debug/servicez || echo "Debug endpoint not available"

# Set default goal
.DEFAULT_GOAL := help