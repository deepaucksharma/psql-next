# Database Intelligence - Restructuring Implementation Guide

## Quick Start: Setting Up the New Structure

### Step 1: Create the Go Workspace Structure

```bash
#!/bin/bash
# setup-workspace.sh

# Create new project root
mkdir -p database-intelligence
cd database-intelligence

# Initialize Go workspace
go work init

# Create module directories
mkdir -p {core,processors,receivers,exporters,extensions,common,distributions,configs,deployments,tests,tools}

# Create sub-directories
mkdir -p core/{cmd/collector,internal/builder,config}
mkdir -p distributions/{minimal,standard,enterprise}
mkdir -p configs/{base,overlays,profiles,examples}
mkdir -p deployments/{docker,kubernetes,helm}
mkdir -p tests/{e2e,integration,performance,benchmarks}
mkdir -p tools/{builder,scripts,ci}
```

### Step 2: Initialize Go Modules

```bash
# Initialize each module with proper import paths
cd core
go mod init github.com/database-intelligence/core

cd ../processors
go mod init github.com/database-intelligence/processors

cd ../receivers
go mod init github.com/database-intelligence/receivers

cd ../exporters
go mod init github.com/database-intelligence/exporters

cd ../extensions
go mod init github.com/database-intelligence/extensions

cd ../common
go mod init github.com/database-intelligence/common

cd ../tests
go mod init github.com/database-intelligence/tests

# Add all modules to workspace
cd ..
go work use ./core ./processors ./receivers ./exporters ./extensions ./common ./tests
```

### Step 3: Common Module Structure

```go
// common/go.mod
module github.com/database-intelligence/common

go 1.21

require (
    go.opentelemetry.io/collector/pdata v1.35.0
    go.uber.org/zap v1.27.0
)
```

```go
// common/types/interfaces.go
package types

import (
    "context"
    "go.opentelemetry.io/collector/component"
)

// SharedConfig represents common configuration
type SharedConfig struct {
    Enabled  bool              `mapstructure:"enabled"`
    Settings map[string]string `mapstructure:"settings"`
}

// FeatureDetector interface for database capability detection
type FeatureDetector interface {
    DetectFeatures(ctx context.Context, dsn string) (Features, error)
}

// Features represents detected database capabilities
type Features struct {
    Version         string
    HasPgStatements bool
    HasAutoExplain  bool
    HasPgQueryLens  bool
    CustomFeatures  map[string]bool
}
```

### Step 4: Processor Module Example

```go
// processors/go.mod
module github.com/database-intelligence/processors

go 1.21

require (
    github.com/database-intelligence/common v0.1.0
    go.opentelemetry.io/collector/component v1.35.0
    go.opentelemetry.io/collector/processor v1.35.0
    go.uber.org/zap v1.27.0
)

replace github.com/database-intelligence/common => ../common
```

```go
// processors/registry.go
package processors

import (
    "go.opentelemetry.io/collector/processor"
    
    "github.com/database-intelligence/processors/adaptivesampler"
    "github.com/database-intelligence/processors/circuitbreaker"
    "github.com/database-intelligence/processors/costcontrol"
    "github.com/database-intelligence/processors/nrerrormonitor"
    "github.com/database-intelligence/processors/planattributeextractor"
    "github.com/database-intelligence/processors/querycorrelator"
    "github.com/database-intelligence/processors/verification"
)

// Factories returns all processor factories
func Factories() (map[string]processor.Factory, error) {
    factories := make(map[string]processor.Factory)
    
    factories["adaptivesampler"] = adaptivesampler.NewFactory()
    factories["circuitbreaker"] = circuitbreaker.NewFactory()
    factories["costcontrol"] = costcontrol.NewFactory()
    factories["nrerrormonitor"] = nrerrormonitor.NewFactory()
    factories["planattributeextractor"] = planattributeextractor.NewFactory()
    factories["querycorrelator"] = querycorrelator.NewFactory()
    factories["verification"] = verification.NewFactory()
    
    return factories, nil
}
```

### Step 5: Distribution Builds

```go
// distributions/enterprise/main.go
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    "github.com/database-intelligence/processors"
    "github.com/database-intelligence/receivers"
    "github.com/database-intelligence/exporters"
    "github.com/database-intelligence/extensions"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatalf("failed to build components: %v", err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-enterprise",
        Description: "Database Intelligence Collector - Enterprise Edition",
        Version:     "2.0.0",
    }

    params := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }

    if err := run(params); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    var err error
    factories := otelcol.Factories{}

    // Add all processors
    factories.Processors, err = processors.Factories()
    if err != nil {
        return factories, err
    }

    // Add all receivers
    factories.Receivers, err = receivers.Factories()
    if err != nil {
        return factories, err
    }

    // Add all exporters
    factories.Exporters, err = exporters.Factories()
    if err != nil {
        return factories, err
    }

    // Add extensions
    factories.Extensions, err = extensions.Factories()
    if err != nil {
        return factories, err
    }

    return factories, nil
}
```

### Step 6: Root Makefile

```makefile
# Makefile
.DEFAULT_GOAL := help

# Build variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# Directories
DIST_DIR = ./dist
BIN_DIR = ./bin

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: workspace-sync
workspace-sync: ## Sync Go workspace
	go work sync

.PHONY: build-minimal
build-minimal: workspace-sync ## Build minimal distribution
	@echo "Building minimal distribution..."
	@mkdir -p $(BIN_DIR)
	cd distributions/minimal && go build -ldflags="$(LDFLAGS)" -o ../../$(BIN_DIR)/collector-minimal

.PHONY: build-standard
build-standard: workspace-sync ## Build standard distribution
	@echo "Building standard distribution..."
	@mkdir -p $(BIN_DIR)
	cd distributions/standard && go build -ldflags="$(LDFLAGS)" -o ../../$(BIN_DIR)/collector-standard

.PHONY: build-enterprise
build-enterprise: workspace-sync ## Build enterprise distribution
	@echo "Building enterprise distribution..."
	@mkdir -p $(BIN_DIR)
	cd distributions/enterprise && go build -ldflags="$(LDFLAGS)" -o ../../$(BIN_DIR)/collector-enterprise

.PHONY: build-all
build-all: build-minimal build-standard build-enterprise ## Build all distributions

.PHONY: test-unit
test-unit: ## Run unit tests
	@echo "Running unit tests..."
	cd common && go test -v ./...
	cd processors && go test -v ./...
	cd receivers && go test -v ./...
	cd exporters && go test -v ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	cd tests/integration && go test -v -tags=integration ./...

.PHONY: test-e2e
test-e2e: build-enterprise ## Run E2E tests
	@echo "Running E2E tests..."
	cd tests/e2e && go test -v -tags=e2e ./...

.PHONY: test-all
test-all: test-unit test-integration test-e2e ## Run all tests

.PHONY: docker-build
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -f distributions/minimal/Dockerfile -t database-intelligence:minimal .
	docker build -f distributions/standard/Dockerfile -t database-intelligence:standard .
	docker build -f distributions/enterprise/Dockerfile -t database-intelligence:enterprise .

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BIN_DIR) $(DIST_DIR)
	go clean -cache -testcache

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
```

### Step 7: Docker Compose for Development

```yaml
# docker-compose.yml
version: '3.8'

services:
  # Test databases
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    volumes:
      - ./deployments/docker/init-scripts/postgres-init.sql:/docker-entrypoint-initdb.d/init.sql

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: mysql
    ports:
      - "3306:3306"
    volumes:
      - ./deployments/docker/init-scripts/mysql-init.sql:/docker-entrypoint-initdb.d/init.sql

  # Monitoring stack
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./configs/examples/prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin

  # Collector (development mode)
  collector:
    build:
      context: .
      dockerfile: distributions/enterprise/Dockerfile
    ports:
      - "4317:4317"  # OTLP gRPC
      - "4318:4318"  # OTLP HTTP
      - "8888:8888"  # Prometheus metrics
      - "13133:13133" # Health check
    volumes:
      - ./configs/profiles/development.yaml:/etc/collector/config.yaml
    environment:
      - POSTGRES_HOST=postgres
      - MYSQL_HOST=mysql
    depends_on:
      - postgres
      - mysql
      - prometheus
```

### Step 8: GitHub Actions Workflow

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Install tools
        run: make install-tools
      - name: Run linters
        run: make lint

  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        module: [common, processors, receivers, exporters, extensions]
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Test ${{ matrix.module }}
        run: |
          cd ${{ matrix.module }}
          go test -v -race -coverprofile=coverage.out ./...
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./${{ matrix.module }}/coverage.out
          flags: ${{ matrix.module }}

  build:
    runs-on: ubuntu-latest
    needs: [lint, test]
    strategy:
      matrix:
        distribution: [minimal, standard, enterprise]
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Build ${{ matrix.distribution }}
        run: make build-${{ matrix.distribution }}
      - name: Upload artifact
        uses: actions/upload-artifact@v3
        with:
          name: collector-${{ matrix.distribution }}
          path: bin/collector-${{ matrix.distribution }}

  e2e-test:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Download enterprise build
        uses: actions/download-artifact@v3
        with:
          name: collector-enterprise
          path: bin/
      - name: Make binary executable
        run: chmod +x bin/collector-enterprise
      - name: Start test environment
        run: docker-compose up -d
      - name: Run E2E tests
        run: make test-e2e
      - name: Stop test environment
        if: always()
        run: docker-compose down
```

## Benefits of This Structure

### 1. **Independent Development**
- Teams can work on specific modules without affecting others
- Clear ownership boundaries
- Faster CI/CD per module

### 2. **Flexible Deployment**
```bash
# Deploy only what you need
./bin/collector-minimal    # Just PostgreSQL monitoring
./bin/collector-standard   # PostgreSQL + MySQL with basic processors
./bin/collector-enterprise # Full feature set with all processors
```

### 3. **Easy Testing**
```bash
# Test specific component
cd processors/adaptivesampler && go test ./...

# Test integration between components
cd tests/integration && go test ./...

# Full E2E testing
make test-e2e
```

### 4. **Version Management**
```go
// Each module can have its own version
// processors/version.go
package processors

const Version = "v1.2.0"

// receivers/version.go  
package receivers

const Version = "v1.1.0"
```

### 5. **Custom Builds**
```go
// Create your own distribution
// distributions/custom/main.go
package main

import (
    // Import only what you need
    _ "github.com/database-intelligence/processors/adaptivesampler"
    _ "github.com/database-intelligence/receivers/postgresql"
    _ "github.com/database-intelligence/exporters/prometheus"
)
```

This structure provides maximum flexibility while maintaining simplicity and testability.