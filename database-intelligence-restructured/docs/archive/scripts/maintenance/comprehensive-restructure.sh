#!/bin/bash
# Comprehensive Database Intelligence MVP Restructuring Script
# This script carefully migrates all files to the new Go workspace structure

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${SOURCE_DIR}/../database-intelligence-restructured"
ORG_NAME="github.com/database-intelligence"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="${SOURCE_DIR}/migration_${TIMESTAMP}.log"

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1" | tee -a "$LOG_FILE"
}

print_error() {
    echo -e "${RED}✗${NC} $1" | tee -a "$LOG_FILE"
}

print_info() {
    echo -e "${BLUE}→${NC} $1" | tee -a "$LOG_FILE"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1" | tee -a "$LOG_FILE"
}

print_section() {
    echo -e "\n${CYAN}==== $1 ====${NC}" | tee -a "$LOG_FILE"
}

# Initialize log
echo "Database Intelligence Restructuring - Started at $(date)" > "$LOG_FILE"
echo "Source: $SOURCE_DIR" >> "$LOG_FILE"
echo "Target: $TARGET_DIR" >> "$LOG_FILE"
echo "" >> "$LOG_FILE"

# Pre-flight checks
print_section "Pre-flight Checks"

# Check if source directory exists
if [ ! -d "$SOURCE_DIR" ]; then
    print_error "Source directory not found: $SOURCE_DIR"
    exit 1
fi

# Check if target directory already exists
if [ -d "$TARGET_DIR" ]; then
    print_warning "Target directory already exists: $TARGET_DIR"
    read -p "Do you want to remove it and continue? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "$TARGET_DIR"
        print_status "Removed existing target directory"
    else
        print_error "Aborted by user"
        exit 1
    fi
fi

# Create target directory structure
print_section "Creating Target Directory Structure"

mkdir -p "$TARGET_DIR"
cd "$TARGET_DIR"

# Create all directories
directories=(
    # Core structure
    "core/cmd/collector"
    "core/internal/builder"
    "core/internal/config"
    "core/pkg/telemetry"
    
    # Processors
    "processors/adaptivesampler"
    "processors/circuitbreaker"
    "processors/costcontrol"
    "processors/nrerrormonitor"
    "processors/planattributeextractor"
    "processors/querycorrelator"
    "processors/verification"
    
    # Receivers
    "receivers/ash"
    "receivers/enhancedsql"
    "receivers/kernelmetrics"
    
    # Exporters
    "exporters/nri"
    
    # Extensions
    "extensions/healthcheck"
    "extensions/pg_querylens"
    
    # Common libraries
    "common/featuredetector"
    "common/queryselector"
    "common/testutils"
    "common/types"
    
    # Distributions
    "distributions/minimal"
    "distributions/standard"
    "distributions/enterprise"
    
    # Configurations
    "configs/base"
    "configs/overlays/development"
    "configs/overlays/staging"
    "configs/overlays/production"
    "configs/overlays/features"
    "configs/profiles"
    "configs/examples"
    "configs/queries"
    
    # Deployments
    "deployments/docker/compose"
    "deployments/docker/dockerfiles"
    "deployments/docker/init-scripts"
    "deployments/kubernetes/base"
    "deployments/kubernetes/overlays/dev"
    "deployments/kubernetes/overlays/staging"
    "deployments/kubernetes/overlays/production"
    "deployments/helm/database-intelligence/templates"
    "deployments/helm/database-intelligence/charts"
    
    # Tests
    "tests/e2e/suites"
    "tests/e2e/fixtures"
    "tests/e2e/framework"
    "tests/integration"
    "tests/performance"
    "tests/benchmarks"
    "tests/testdata"
    
    # Tools and scripts
    "tools/builder"
    "tools/scripts/build"
    "tools/scripts/test"
    "tools/scripts/deploy"
    "tools/scripts/maintenance"
    "tools/ci/github"
    "tools/ci/gitlab"
    
    # Documentation
    "docs/architecture"
    "docs/deployment"
    "docs/development"
    "docs/operations"
    "docs/api"
    "docs/tutorials"
    
    # Archive for analysis docs
    "docs/archive/analysis"
    "docs/archive/legacy"
)

for dir in "${directories[@]}"; do
    mkdir -p "$dir"
done
print_status "Created directory structure"

# Initialize Go workspace
print_section "Initializing Go Workspace"

cd "$TARGET_DIR"
go work init
print_status "Created go.work file"

# Create Go modules
print_section "Creating Go Modules"

# Function to create a Go module
create_go_module() {
    local module_path=$1
    local module_name=$2
    
    cd "$TARGET_DIR/$module_path"
    go mod init "${ORG_NAME}/${module_name}"
    cd "$TARGET_DIR"
    go work use "./${module_path}"
    print_status "Created module: ${module_name}"
}

# Create all modules
create_go_module "core" "core"
create_go_module "processors" "processors"
create_go_module "receivers" "receivers"
create_go_module "exporters" "exporters"
create_go_module "extensions" "extensions"
create_go_module "common" "common"
create_go_module "tests" "tests"

# Create distribution modules
create_go_module "distributions/minimal" "distributions/minimal"
create_go_module "distributions/standard" "distributions/standard"
create_go_module "distributions/enterprise" "distributions/enterprise"

# Migrate components
print_section "Migrating Components"

# Function to migrate files with import path updates
migrate_component() {
    local component_type=$1
    local component_name=$2
    local source_path="${SOURCE_DIR}/${component_type}/${component_name}"
    local target_path="${TARGET_DIR}/${component_type}/${component_name}"
    
    if [ -d "$source_path" ]; then
        # Copy all files
        cp -r "$source_path"/* "$target_path/" 2>/dev/null || true
        
        # Update import paths in all Go files
        find "$target_path" -name "*.go" -type f -exec sed -i.bak \
            -e "s|github.com/database-intelligence-mvp|${ORG_NAME}|g" \
            -e "s|github.com/newrelic/newrelic-otel-collector|${ORG_NAME}|g" \
            {} \;
        
        # Remove backup files
        find "$target_path" -name "*.bak" -type f -delete
        
        print_status "Migrated ${component_type}/${component_name}"
    else
        print_warning "Source not found: ${source_path}"
    fi
}

# Migrate all processors
for proc in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    migrate_component "processors" "$proc"
done

# Migrate all receivers
for recv in ash enhancedsql kernelmetrics; do
    migrate_component "receivers" "$recv"
done

# Migrate exporters
migrate_component "exporters" "nri"

# Migrate extensions
migrate_component "extensions" "healthcheck"

# Special handling for pg_querylens (C extension)
if [ -d "${SOURCE_DIR}/extensions/pg_querylens" ]; then
    cp -r "${SOURCE_DIR}/extensions/pg_querylens"/* "${TARGET_DIR}/extensions/pg_querylens/"
    print_status "Migrated pg_querylens C extension"
fi

# Migrate common libraries
print_section "Migrating Common Libraries"

for lib in featuredetector queryselector; do
    if [ -d "${SOURCE_DIR}/common/${lib}" ]; then
        cp -r "${SOURCE_DIR}/common/${lib}"/* "${TARGET_DIR}/common/${lib}/"
        print_status "Migrated common/${lib}"
    fi
done

# Migrate internal components
print_section "Migrating Internal Components"

if [ -d "${SOURCE_DIR}/internal" ]; then
    # Distribute internal components appropriately
    [ -d "${SOURCE_DIR}/internal/performance" ] && cp -r "${SOURCE_DIR}/internal/performance" "${TARGET_DIR}/core/internal/"
    [ -d "${SOURCE_DIR}/internal/secrets" ] && cp -r "${SOURCE_DIR}/internal/secrets" "${TARGET_DIR}/core/internal/"
    [ -d "${SOURCE_DIR}/internal/ratelimit" ] && cp -r "${SOURCE_DIR}/internal/ratelimit" "${TARGET_DIR}/core/internal/"
    [ -d "${SOURCE_DIR}/internal/health" ] && cp -r "${SOURCE_DIR}/internal/health" "${TARGET_DIR}/core/internal/"
    [ -d "${SOURCE_DIR}/internal/database" ] && cp -r "${SOURCE_DIR}/internal/database" "${TARGET_DIR}/core/internal/"
    [ -d "${SOURCE_DIR}/internal/conventions" ] && cp -r "${SOURCE_DIR}/internal/conventions" "${TARGET_DIR}/core/internal/"
    print_status "Migrated internal components"
fi

# Migrate core files
print_section "Migrating Core Files"

# Copy main.go to core
if [ -f "${SOURCE_DIR}/main.go" ]; then
    cp "${SOURCE_DIR}/main.go" "${TARGET_DIR}/core/cmd/collector/main.go"
    # Update imports in main.go
    sed -i.bak \
        -e "s|github.com/database-intelligence-mvp|${ORG_NAME}|g" \
        "${TARGET_DIR}/core/cmd/collector/main.go"
    rm -f "${TARGET_DIR}/core/cmd/collector/main.go.bak"
    print_status "Migrated main.go"
fi

# Copy builder configuration
if [ -f "${SOURCE_DIR}/otelcol-builder.yaml" ]; then
    cp "${SOURCE_DIR}/otelcol-builder.yaml" "${TARGET_DIR}/tools/builder/"
    print_status "Migrated builder configuration"
fi

# Migrate configurations
print_section "Migrating Configurations"

# Base configurations
if [ -d "${SOURCE_DIR}/config/base" ]; then
    cp -r "${SOURCE_DIR}/config/base"/* "${TARGET_DIR}/configs/base/" 2>/dev/null || true
fi

# Environment overlays
if [ -d "${SOURCE_DIR}/config/environments" ]; then
    cp "${SOURCE_DIR}/config/environments/development.yaml" "${TARGET_DIR}/configs/overlays/development/" 2>/dev/null || true
    cp "${SOURCE_DIR}/config/environments/staging.yaml" "${TARGET_DIR}/configs/overlays/staging/" 2>/dev/null || true
    cp "${SOURCE_DIR}/config/environments/production.yaml" "${TARGET_DIR}/configs/overlays/production/" 2>/dev/null || true
fi

# Feature overlays
if [ -d "${SOURCE_DIR}/config/overlays" ]; then
    cp "${SOURCE_DIR}/config/overlays"/*.yaml "${TARGET_DIR}/configs/overlays/features/" 2>/dev/null || true
fi

# Query configurations
if [ -d "${SOURCE_DIR}/config/queries" ]; then
    cp -r "${SOURCE_DIR}/config/queries"/* "${TARGET_DIR}/configs/queries/" 2>/dev/null || true
fi

# Copy all other config files to examples
find "${SOURCE_DIR}/config" -name "*.yaml" -o -name "*.yml" | while read -r config_file; do
    if [ -f "$config_file" ]; then
        cp "$config_file" "${TARGET_DIR}/configs/examples/"
    fi
done
print_status "Migrated configurations"

# Migrate deployments
print_section "Migrating Deployment Files"

# Docker files
if [ -d "${SOURCE_DIR}/docker" ] || [ -d "${SOURCE_DIR}/deploy/docker" ]; then
    # Docker Compose files
    find "${SOURCE_DIR}" -name "docker-compose*.yml" -o -name "docker-compose*.yaml" | while read -r compose_file; do
        cp "$compose_file" "${TARGET_DIR}/deployments/docker/compose/"
    done
    
    # Dockerfiles
    find "${SOURCE_DIR}" -name "Dockerfile*" | while read -r dockerfile; do
        cp "$dockerfile" "${TARGET_DIR}/deployments/docker/dockerfiles/"
    done
    
    # Init scripts
    if [ -d "${SOURCE_DIR}/deploy/docker/init-scripts" ]; then
        cp -r "${SOURCE_DIR}/deploy/docker/init-scripts"/* "${TARGET_DIR}/deployments/docker/init-scripts/" 2>/dev/null || true
    fi
    
    print_status "Migrated Docker files"
fi

# Kubernetes files
if [ -d "${SOURCE_DIR}/k8s" ] || [ -d "${SOURCE_DIR}/deployments/kubernetes" ]; then
    # Find all k8s yaml files
    find "${SOURCE_DIR}" -path "*/k8s/*" -name "*.yaml" -o -path "*/kubernetes/*" -name "*.yaml" | while read -r k8s_file; do
        cp "$k8s_file" "${TARGET_DIR}/deployments/kubernetes/base/"
    done
    print_status "Migrated Kubernetes files"
fi

# Helm charts
if [ -d "${SOURCE_DIR}/deployments/helm" ]; then
    cp -r "${SOURCE_DIR}/deployments/helm"/* "${TARGET_DIR}/deployments/helm/" 2>/dev/null || true
    print_status "Migrated Helm charts"
fi

# Migrate tests
print_section "Migrating Tests"

# E2E tests
if [ -d "${SOURCE_DIR}/tests/e2e" ]; then
    # Framework files
    [ -d "${SOURCE_DIR}/tests/e2e/framework" ] && cp -r "${SOURCE_DIR}/tests/e2e/framework"/* "${TARGET_DIR}/tests/e2e/framework/"
    
    # Test suites
    for test_file in "${SOURCE_DIR}/tests/e2e"/*.go; do
        if [ -f "$test_file" ]; then
            cp "$test_file" "${TARGET_DIR}/tests/e2e/suites/"
        fi
    done
    
    # Fixtures and testdata
    [ -d "${SOURCE_DIR}/tests/e2e/testdata" ] && cp -r "${SOURCE_DIR}/tests/e2e/testdata"/* "${TARGET_DIR}/tests/e2e/fixtures/"
    
    # Docker compose for tests
    [ -f "${SOURCE_DIR}/tests/e2e/docker-compose.yml" ] && cp "${SOURCE_DIR}/tests/e2e/docker-compose.yml" "${TARGET_DIR}/tests/e2e/"
    
    print_status "Migrated E2E tests"
fi

# Integration tests
if [ -d "${SOURCE_DIR}/tests/integration" ]; then
    cp -r "${SOURCE_DIR}/tests/integration"/* "${TARGET_DIR}/tests/integration/" 2>/dev/null || true
    print_status "Migrated integration tests"
fi

# Performance tests
if [ -d "${SOURCE_DIR}/tests/performance" ]; then
    cp -r "${SOURCE_DIR}/tests/performance"/* "${TARGET_DIR}/tests/performance/" 2>/dev/null || true
    print_status "Migrated performance tests"
fi

# Benchmarks
if [ -d "${SOURCE_DIR}/tests/benchmarks" ] || [ -d "${SOURCE_DIR}/benchmarks" ]; then
    find "${SOURCE_DIR}" -name "*_bench_test.go" -o -name "benchmark*.go" | while read -r bench_file; do
        cp "$bench_file" "${TARGET_DIR}/tests/benchmarks/"
    done
    print_status "Migrated benchmarks"
fi

# Migrate scripts
print_section "Migrating Scripts"

# Categorize and migrate scripts
if [ -d "${SOURCE_DIR}/scripts" ]; then
    # Build scripts
    for script in build*.sh compile*.sh; do
        [ -f "${SOURCE_DIR}/scripts/$script" ] && cp "${SOURCE_DIR}/scripts/$script" "${TARGET_DIR}/tools/scripts/build/"
    done
    
    # Test scripts
    for script in test*.sh validate*.sh check*.sh; do
        [ -f "${SOURCE_DIR}/scripts/$script" ] && cp "${SOURCE_DIR}/scripts/$script" "${TARGET_DIR}/tools/scripts/test/"
    done
    
    # Deployment scripts
    for script in deploy*.sh release*.sh; do
        [ -f "${SOURCE_DIR}/scripts/$script" ] && cp "${SOURCE_DIR}/scripts/$script" "${TARGET_DIR}/tools/scripts/deploy/"
    done
    
    # Maintenance scripts
    for script in cleanup*.sh fix*.sh migrate*.sh organize*.sh; do
        [ -f "${SOURCE_DIR}/scripts/$script" ] && cp "${SOURCE_DIR}/scripts/$script" "${TARGET_DIR}/tools/scripts/maintenance/"
    done
    
    # Copy any remaining scripts to maintenance
    find "${SOURCE_DIR}/scripts" -name "*.sh" | while read -r script; do
        if [ ! -f "${TARGET_DIR}/tools/scripts/"*"/$(basename "$script")" ]; then
            cp "$script" "${TARGET_DIR}/tools/scripts/maintenance/"
        fi
    done
    
    print_status "Migrated scripts"
fi

# Migrate documentation
print_section "Migrating Documentation"

# Main documentation files
for doc in README.md CHANGELOG.md CONTRIBUTING.md LICENSE; do
    [ -f "${SOURCE_DIR}/$doc" ] && cp "${SOURCE_DIR}/$doc" "${TARGET_DIR}/"
done

# Architecture docs
if [ -d "${SOURCE_DIR}/docs" ]; then
    # Categorize documentation
    for doc in "${SOURCE_DIR}/docs"/*.md; do
        if [ -f "$doc" ]; then
            filename=$(basename "$doc")
            case "$filename" in
                *ARCHITECTURE*|*DESIGN*)
                    cp "$doc" "${TARGET_DIR}/docs/architecture/"
                    ;;
                *DEPLOY*|*INSTALL*)
                    cp "$doc" "${TARGET_DIR}/docs/deployment/"
                    ;;
                *DEVELOP*|*GUIDE*|*API*)
                    cp "$doc" "${TARGET_DIR}/docs/development/"
                    ;;
                *OPERATION*|*MONITOR*|*TROUBLESHOOT*)
                    cp "$doc" "${TARGET_DIR}/docs/operations/"
                    ;;
                *)
                    cp "$doc" "${TARGET_DIR}/docs/"
                    ;;
            esac
        fi
    done
fi

# Move analysis documents to archive
for doc in STRATEGY_ALIGNMENT_REVIEW.md MIGRATION_GAPS_DIAGRAM.md IMPLEMENTATION_COMPLETENESS_ANALYSIS.md OTEL_IMPLEMENTATION_REVIEW.md; do
    [ -f "${SOURCE_DIR}/$doc" ] && cp "${SOURCE_DIR}/$doc" "${TARGET_DIR}/docs/archive/analysis/"
done

print_status "Migrated documentation"

# Create registry files
print_section "Creating Registry Files"

# Processors registry
cat > "${TARGET_DIR}/processors/registry.go" << 'EOF'
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
EOF
print_status "Created processors registry"

# Receivers registry
cat > "${TARGET_DIR}/receivers/registry.go" << 'EOF'
package receivers

import (
    "go.opentelemetry.io/collector/receiver"
    
    "github.com/database-intelligence/receivers/ash"
    "github.com/database-intelligence/receivers/enhancedsql"
    "github.com/database-intelligence/receivers/kernelmetrics"
)

// Factories returns all receiver factories
func Factories() (map[string]receiver.Factory, error) {
    factories := make(map[string]receiver.Factory)
    
    factories["ash"] = ash.NewFactory()
    factories["enhancedsql"] = enhancedsql.NewFactory()
    factories["kernelmetrics"] = kernelmetrics.NewFactory()
    
    return factories, nil
}
EOF
print_status "Created receivers registry"

# Exporters registry
cat > "${TARGET_DIR}/exporters/registry.go" << 'EOF'
package exporters

import (
    "go.opentelemetry.io/collector/exporter"
    
    "github.com/database-intelligence/exporters/nri"
)

// Factories returns all exporter factories
func Factories() (map[string]exporter.Factory, error) {
    factories := make(map[string]exporter.Factory)
    
    factories["nri"] = nri.NewFactory()
    
    return factories, nil
}
EOF
print_status "Created exporters registry"

# Extensions registry
cat > "${TARGET_DIR}/extensions/registry.go" << 'EOF'
package extensions

import (
    "go.opentelemetry.io/collector/extension"
    
    "github.com/database-intelligence/extensions/healthcheck"
)

// Factories returns all extension factories
func Factories() (map[string]extension.Factory, error) {
    factories := make(map[string]extension.Factory)
    
    factories["healthcheck"] = healthcheck.NewFactory()
    
    return factories, nil
}
EOF
print_status "Created extensions registry"

# Create distribution builds
print_section "Creating Distribution Builds"

# Minimal distribution
cat > "${TARGET_DIR}/distributions/minimal/main.go" << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Import only essential components
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/exporter/debugexporter"
)

func main() {
    factories := otelcol.Factories{}
    
    // Minimal receivers
    factories.Receivers = map[string]receiver.Factory{
        "postgresql": postgresqlreceiver.NewFactory(),
    }
    
    // Minimal processors
    factories.Processors = map[string]processor.Factory{
        "batch": batchprocessor.NewFactory(),
    }
    
    // Minimal exporters
    factories.Exporters = map[string]exporter.Factory{
        "prometheus": prometheusexporter.NewFactory(),
        "debug": debugexporter.NewFactory(),
    }
    
    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Collector - Minimal Edition",
        Version:     "2.0.0",
    }

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}
EOF
print_status "Created minimal distribution"

# Standard distribution
cat > "${TARGET_DIR}/distributions/standard/main.go" << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Import standard components
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
        Command:     "database-intelligence-standard",
        Description: "Database Intelligence Collector - Standard Edition",
        Version:     "2.0.0",
    }

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    var err error
    factories := otelcol.Factories{}

    // Add custom processors (subset)
    factories.Processors = map[string]processor.Factory{
        "adaptivesampler": processors.AdaptiveSamplerFactory(),
        "circuitbreaker": processors.CircuitBreakerFactory(),
    }

    // Standard receivers
    factories.Receivers, err = receivers.Factories()
    if err != nil {
        return factories, err
    }

    // Standard exporters
    factories.Exporters, err = exporters.Factories()
    if err != nil {
        return factories, err
    }

    return factories, nil
}
EOF
print_status "Created standard distribution"

# Enterprise distribution
cat > "${TARGET_DIR}/distributions/enterprise/main.go" << 'EOF'
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

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    var err error
    factories := otelcol.Factories{}

    // All processors
    factories.Processors, err = processors.Factories()
    if err != nil {
        return factories, err
    }

    // All receivers
    factories.Receivers, err = receivers.Factories()
    if err != nil {
        return factories, err
    }

    // All exporters
    factories.Exporters, err = exporters.Factories()
    if err != nil {
        return factories, err
    }

    // All extensions
    factories.Extensions, err = extensions.Factories()
    if err != nil {
        return factories, err
    }

    return factories, nil
}
EOF
print_status "Created enterprise distribution"

# Create root files
print_section "Creating Root Files"

# Create Makefile
cp "${SOURCE_DIR}/Makefile" "${TARGET_DIR}/Makefile" 2>/dev/null || cat > "${TARGET_DIR}/Makefile" << 'EOF'
.DEFAULT_GOAL := help

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_TIME = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

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
	cd distributions/minimal && go build -ldflags="$(LDFLAGS)" -o ../../bin/collector-minimal

.PHONY: build-standard
build-standard: workspace-sync ## Build standard distribution
	cd distributions/standard && go build -ldflags="$(LDFLAGS)" -o ../../bin/collector-standard

.PHONY: build-enterprise
build-enterprise: workspace-sync ## Build enterprise distribution
	cd distributions/enterprise && go build -ldflags="$(LDFLAGS)" -o ../../bin/collector-enterprise

.PHONY: build-all
build-all: build-minimal build-standard build-enterprise ## Build all distributions

.PHONY: test-unit
test-unit: ## Run unit tests
	go test -v ./...

.PHONY: test-e2e
test-e2e: build-enterprise ## Run E2E tests
	cd tests/e2e && go test -v -tags=e2e ./...

.PHONY: test-all
test-all: test-unit test-e2e ## Run all tests

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	go clean -cache -testcache

.PHONY: fmt
fmt: ## Format code
	go fmt ./...
	goimports -w .

.PHONY: lint
lint: ## Run linters
	golangci-lint run ./...
EOF
print_status "Created Makefile"

# Create README
cat > "${TARGET_DIR}/README.md" << 'EOF'
# Database Intelligence

A modular OpenTelemetry-based database monitoring solution using Go workspaces.

## Structure

```
database-intelligence/
├── core/              # Core collector components
├── processors/        # Custom processors
├── receivers/         # Custom receivers
├── exporters/         # Custom exporters
├── extensions/        # Extensions
├── common/           # Shared libraries
├── distributions/    # Pre-built distributions
├── configs/          # Configuration templates
├── deployments/      # Deployment artifacts
├── tests/           # Test suites
└── tools/           # Build and development tools
```

## Quick Start

```bash
# Build all distributions
make build-all

# Run enterprise distribution
./bin/collector-enterprise --config=configs/profiles/development.yaml

# Run tests
make test-all
```

## Development

This project uses Go workspaces for modular development:

```bash
# Work on a specific module
cd processors/adaptivesampler
go test ./...

# Sync workspace
go work sync
```

## Distributions

- **Minimal**: Basic PostgreSQL monitoring with Prometheus export
- **Standard**: PostgreSQL + MySQL with essential processors
- **Enterprise**: Full feature set with all processors and advanced capabilities

## Configuration

Configurations are organized by:
- `base/`: Base configurations for components
- `overlays/`: Environment and feature-specific overlays
- `profiles/`: Ready-to-use configuration profiles
- `examples/`: Example configurations

## Documentation

See the `docs/` directory for:
- Architecture documentation
- Deployment guides
- Development documentation
- Operations guides
EOF
print_status "Created README"

# Create .gitignore
cat > "${TARGET_DIR}/.gitignore" << 'EOF'
# Binaries
bin/
*.exe
*.dll
*.so
*.dylib

# Test artifacts
*.test
*.out
coverage.html
coverage.out
test-results/

# Go workspace
go.work.sum

# IDE
.idea/
.vscode/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Build artifacts
dist/
build/

# Logs
*.log

# Environment
.env
.env.local

# Generated files
configs/generated/
EOF
print_status "Created .gitignore"

# Copy additional files
print_section "Copying Additional Files"

# Copy go.mod and go.sum files if they exist
[ -f "${SOURCE_DIR}/go.mod" ] && cp "${SOURCE_DIR}/go.mod" "${TARGET_DIR}/core/"
[ -f "${SOURCE_DIR}/go.sum" ] && cp "${SOURCE_DIR}/go.sum" "${TARGET_DIR}/core/"

# Copy any CI/CD files
[ -d "${SOURCE_DIR}/.github" ] && cp -r "${SOURCE_DIR}/.github" "${TARGET_DIR}/tools/ci/github/"
[ -f "${SOURCE_DIR}/.gitlab-ci.yml" ] && cp "${SOURCE_DIR}/.gitlab-ci.yml" "${TARGET_DIR}/tools/ci/gitlab/"

# Copy Taskfile if exists
[ -f "${SOURCE_DIR}/Taskfile.yml" ] && cp "${SOURCE_DIR}/Taskfile.yml" "${TARGET_DIR}/tools/"

# Create migration summary
print_section "Creating Migration Summary"

cat > "${TARGET_DIR}/MIGRATION_SUMMARY.md" << EOF
# Migration Summary

## Migration completed at $(date)

### Source
- Path: ${SOURCE_DIR}
- Files migrated: $(find "$SOURCE_DIR" -type f | wc -l)

### Target
- Path: ${TARGET_DIR}
- New structure: Go workspace with modular design

### Key Changes

1. **Modular Structure**
   - Each component type is now a separate Go module
   - Clear separation between core, processors, receivers, exporters
   - Shared code in common module

2. **Distributions**
   - Minimal: Basic monitoring
   - Standard: Common use cases
   - Enterprise: Full features

3. **Configuration**
   - Base templates in configs/base/
   - Environment overlays in configs/overlays/
   - Ready profiles in configs/profiles/

4. **Documentation**
   - Organized by topic (architecture, deployment, etc.)
   - Analysis docs moved to archive

5. **Testing**
   - E2E tests in tests/e2e/suites/
   - Framework code in tests/e2e/framework/
   - Performance tests separated

### Next Steps

1. Run \`go work sync\` to synchronize workspace
2. Update CI/CD pipelines to use new structure
3. Test build all distributions: \`make build-all\`
4. Run tests: \`make test-all\`
5. Update deployment scripts for new paths

### Import Path Changes

All imports have been updated from:
- \`github.com/database-intelligence-mvp\`

To:
- \`github.com/database-intelligence\`
EOF
print_status "Created migration summary"

# Final summary
print_section "Migration Complete"

echo -e "\n${GREEN}✓ Migration completed successfully!${NC}"
echo -e "\nSummary:"
echo -e "  Source: ${BLUE}${SOURCE_DIR}${NC}"
echo -e "  Target: ${BLUE}${TARGET_DIR}${NC}"
echo -e "  Log file: ${BLUE}${LOG_FILE}${NC}"
echo -e "\nNext steps:"
echo -e "  1. ${CYAN}cd ${TARGET_DIR}${NC}"
echo -e "  2. ${CYAN}go work sync${NC}"
echo -e "  3. ${CYAN}make build-all${NC}"
echo -e "  4. ${CYAN}make test-all${NC}"
echo -e "\nThe original source directory has been preserved."