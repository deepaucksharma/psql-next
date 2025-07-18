# Database Intelligence Makefile

.PHONY: all build test clean docker help dev

# Variables
BINARY_NAME := dbintel
DOCKER_IMAGE := dbintel
DOCKER_TAG := latest
GO_VERSION := 1.23.0

# Scripts
BUILD_SCRIPT := ./scripts/build/build.sh
TEST_SCRIPT := ./scripts/test/run-tests.sh
DEPLOY_SCRIPT := ./scripts/deploy/docker.sh
DEV_SCRIPT := ./scripts/dev/fix-modules.sh
CLEAN_SCRIPT := ./scripts/maintain/cleanup.sh

# Color codes for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

# Default target
all: build

help: ## Show this help message
	@echo "Database Intelligence Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[32m%-15s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build production distribution
	@$(BUILD_SCRIPT) production

build-minimal: ## Build minimal distribution
	@$(BUILD_SCRIPT) minimal

build-enterprise: ## Build enterprise distribution
	@$(BUILD_SCRIPT) enterprise

build-all: ## Build all distributions
	@$(BUILD_SCRIPT) all

docker-build: ## Build Docker image
	@$(BUILD_SCRIPT) docker

# Test targets
test: ## Run all tests
	@$(TEST_SCRIPT) all

test-unit: ## Run unit tests only
	@$(TEST_SCRIPT) unit

test-integration: ## Run integration tests
	@$(TEST_SCRIPT) integration

test-e2e: ## Run end-to-end tests
	@$(TEST_SCRIPT) e2e

# Development targets
dev: ## Run development checks (format, lint, test)
	@echo "${YELLOW}Running development checks...${NC}"
	@$(DEV_SCRIPT) all
	@$(BUILD_SCRIPT) production
	@$(TEST_SCRIPT) unit
	@echo "${GREEN}Development checks complete${NC}"

dev-run: ## Run collector in development mode
	@$(DEPLOY_SCRIPT) up

fix-modules: ## Fix Go module issues
	@$(DEV_SCRIPT) all

# Deployment targets
deploy: ## Deploy with Docker
	@$(DEPLOY_SCRIPT) up production

deploy-status: ## Check deployment status
	@$(DEPLOY_SCRIPT) status

stop: ## Stop deployed services
	@$(DEPLOY_SCRIPT) down

# Maintenance targets
clean: ## Clean build artifacts and temp files
	@$(CLEAN_SCRIPT) build

clean-all: ## Clean everything (build, docker, logs, temp)
	@$(CLEAN_SCRIPT) all

clean-docker: ## Clean Docker resources
	@$(CLEAN_SCRIPT) docker

validate-config: ## Validate configurations
	@./scripts/maintain/validate.sh

# Quick commands
quick-start: ## Quick start (build and run)
	@$(BUILD_SCRIPT) production
	@$(DEPLOY_SCRIPT) up

test-short: ## Run short tests
	@echo "${YELLOW}Running short tests...${NC}"
	@go test ./... -short -v
	@echo "${GREEN}Short tests complete${NC}"

lint: ## Run linters
	@echo "${YELLOW}Running linters...${NC}"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "${RED}golangci-lint not installed. Install with: brew install golangci-lint${NC}"; \
	fi

fmt: ## Format code
	@echo "${YELLOW}Formatting code...${NC}"
	@go fmt ./...
	@echo "${GREEN}Formatting complete${NC}"

vet: ## Run go vet
	@echo "${YELLOW}Running go vet...${NC}"
	@go vet ./...
	@echo "${GREEN}Vet complete${NC}"

docker-build: ## Build Docker image
	@echo "${YELLOW}Building Docker image...${NC}"
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f deployments/docker/Dockerfile .
	@echo "${GREEN}Docker build complete: $(DOCKER_IMAGE):$(DOCKER_TAG)${NC}"

docker-push: ## Push Docker image to registry
	@echo "${YELLOW}Pushing Docker image...${NC}"
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "${GREEN}Docker push complete${NC}"

clean: ## Clean build artifacts
	@echo "${YELLOW}Cleaning build artifacts...${NC}"
	@rm -rf $(OUTPUT_DIR)
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "${GREEN}Clean complete${NC}"

deps: ## Download dependencies
	@echo "${YELLOW}Downloading dependencies...${NC}"
	@go mod download
	@echo "${GREEN}Dependencies downloaded${NC}"

update-deps: ## Update dependencies
	@echo "${YELLOW}Updating dependencies...${NC}"
	@go get -u ./...
	@go mod tidy
	@echo "${GREEN}Dependencies updated${NC}"

verify: ## Verify module dependencies
	@echo "${YELLOW}Verifying dependencies...${NC}"
	@go mod verify
	@echo "${GREEN}Verification complete${NC}"

run-minimal: build ## Run with minimal profile
	@echo "${YELLOW}Running with minimal profile...${NC}"
	@$(OUTPUT_DIR)/$(BINARY_NAME) --profile=minimal

run-standard: build ## Run with standard profile
	@echo "${YELLOW}Running with standard profile...${NC}"
	@$(OUTPUT_DIR)/$(BINARY_NAME) --profile=standard

run-enterprise: build ## Run with enterprise profile
	@echo "${YELLOW}Running with enterprise profile...${NC}"
	@$(OUTPUT_DIR)/$(BINARY_NAME) --profile=enterprise

dev-setup: ## Setup development environment
	@echo "${YELLOW}Setting up development environment...${NC}"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "${GREEN}Development setup complete${NC}"

check-go-version: ## Check Go version
	@echo "${YELLOW}Checking Go version...${NC}"
	@go version | grep -q "go$(GO_VERSION)" || (echo "${RED}Error: Go $(GO_VERSION) required${NC}" && exit 1)
	@echo "${GREEN}Go version OK${NC}"

# CI/CD targets
ci: check-go-version fmt vet lint test build ## Run CI pipeline
	@echo "${GREEN}CI pipeline complete${NC}"

release: clean test build-all ## Create a release build
	@echo "${YELLOW}Creating release archive...${NC}"
	@tar -czf $(OUTPUT_DIR)/$(BINARY_NAME)-$(shell git describe --tags --always).tar.gz -C $(OUTPUT_DIR) .
	@echo "${GREEN}Release created${NC}"