#!/bin/bash
# Migrate Database Intelligence MVP to Go Workspace Structure

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SOURCE_DIR=$(pwd)
TARGET_DIR="../database-intelligence"
ORG_NAME="github.com/database-intelligence"

echo -e "${BLUE}Database Intelligence MVP - Migration to Go Workspace${NC}"
echo -e "${BLUE}=====================================================${NC}"
echo ""

# Function to print status
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${BLUE}→${NC} $1"
}

# Step 1: Create target directory structure
echo -e "${YELLOW}Step 1: Creating workspace structure${NC}"
mkdir -p "$TARGET_DIR"
cd "$TARGET_DIR"

# Create all directories
directories=(
    "core/cmd/collector"
    "core/internal/builder"
    "core/config"
    "processors"
    "receivers"
    "exporters"
    "extensions"
    "common/featuredetector"
    "common/queryselector"
    "common/testutils"
    "common/types"
    "distributions/minimal"
    "distributions/standard"
    "distributions/enterprise"
    "configs/base"
    "configs/overlays"
    "configs/profiles"
    "configs/examples"
    "deployments/docker"
    "deployments/kubernetes/base"
    "deployments/kubernetes/overlays"
    "deployments/helm"
    "tests/e2e"
    "tests/integration"
    "tests/performance"
    "tests/benchmarks"
    "tools/builder"
    "tools/scripts"
    "tools/ci"
)

for dir in "${directories[@]}"; do
    mkdir -p "$dir"
    print_status "Created $dir"
done

# Step 2: Initialize Go workspace
echo -e "\n${YELLOW}Step 2: Initializing Go workspace${NC}"
go work init
print_status "Created go.work file"

# Step 3: Create Go modules
echo -e "\n${YELLOW}Step 3: Creating Go modules${NC}"
modules=(
    "core"
    "processors"
    "receivers"
    "exporters"
    "extensions"
    "common"
    "tests"
)

for module in "${modules[@]}"; do
    cd "$module"
    go mod init "${ORG_NAME}/${module}"
    cd ..
    go work use "./${module}"
    print_status "Initialized module: ${module}"
done

# Create distribution modules
for dist in minimal standard enterprise; do
    cd "distributions/$dist"
    go mod init "${ORG_NAME}/distributions/${dist}"
    cd ../..
    go work use "./distributions/${dist}"
    print_status "Initialized distribution: ${dist}"
done

# Step 4: Copy and migrate components
echo -e "\n${YELLOW}Step 4: Migrating components${NC}"

# Function to migrate a component
migrate_component() {
    local component_type=$1
    local component_name=$2
    local source_path="${SOURCE_DIR}/${component_type}/${component_name}"
    local target_path="${TARGET_DIR}/${component_type}/${component_name}"
    
    if [ -d "$source_path" ]; then
        cp -r "$source_path" "$target_path"
        
        # Update import paths
        find "$target_path" -name "*.go" -type f -exec sed -i.bak \
            -e "s|github.com/database-intelligence-mvp|${ORG_NAME}|g" \
            -e "s|github.com/newrelic/newrelic-otel-collector|${ORG_NAME}|g" \
            {} \;
        
        # Remove backup files
        find "$target_path" -name "*.bak" -type f -delete
        
        print_status "Migrated ${component_type}/${component_name}"
    else
        print_error "Source not found: ${source_path}"
    fi
}

# Migrate processors
processors=(
    "adaptivesampler"
    "circuitbreaker"
    "costcontrol"
    "nrerrormonitor"
    "planattributeextractor"
    "querycorrelator"
    "verification"
)

for proc in "${processors[@]}"; do
    migrate_component "processors" "$proc"
done

# Migrate receivers
receivers=(
    "ash"
    "enhancedsql"
    "kernelmetrics"
)

for recv in "${receivers[@]}"; do
    migrate_component "receivers" "$recv"
done

# Migrate exporters
migrate_component "exporters" "nri"

# Migrate extensions
migrate_component "extensions" "healthcheck"
migrate_component "extensions" "pg_querylens"

# Migrate common libraries
cp -r "${SOURCE_DIR}/common/featuredetector"/* "${TARGET_DIR}/common/featuredetector/"
cp -r "${SOURCE_DIR}/common/queryselector"/* "${TARGET_DIR}/common/queryselector/"
print_status "Migrated common libraries"

# Step 5: Create registry files
echo -e "\n${YELLOW}Step 5: Creating registry files${NC}"

# Create processors registry
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

# Create similar registries for receivers and exporters
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

# Step 6: Copy configurations
echo -e "\n${YELLOW}Step 6: Copying configurations${NC}"
cp -r "${SOURCE_DIR}/config/"* "${TARGET_DIR}/configs/"
print_status "Copied configuration files"

# Step 7: Copy deployment files
echo -e "\n${YELLOW}Step 7: Copying deployment files${NC}"
cp -r "${SOURCE_DIR}/deploy/docker/"* "${TARGET_DIR}/deployments/docker/" 2>/dev/null || true
cp -r "${SOURCE_DIR}/deployments/kubernetes/"* "${TARGET_DIR}/deployments/kubernetes/" 2>/dev/null || true
cp -r "${SOURCE_DIR}/deployments/helm/"* "${TARGET_DIR}/deployments/helm/" 2>/dev/null || true
print_status "Copied deployment files"

# Step 8: Create main files for distributions
echo -e "\n${YELLOW}Step 8: Creating distribution builds${NC}"

# Create minimal distribution
cat > "${TARGET_DIR}/distributions/minimal/main.go" << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
)

func main() {
    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Collector - Minimal Edition",
        Version:     "2.0.0",
    }

    if err := run(otelcol.CollectorSettings{BuildInfo: info}); err != nil {
        log.Fatal(err)
    }
}
EOF
print_status "Created minimal distribution"

# Step 9: Create root Makefile
echo -e "\n${YELLOW}Step 9: Creating root Makefile${NC}"
cp "${SOURCE_DIR}/scripts/maintenance/Makefile.workspace" "${TARGET_DIR}/Makefile" 2>/dev/null || \
cat > "${TARGET_DIR}/Makefile" << 'EOF'
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@echo '  build-all       Build all distributions'
	@echo '  test-all        Run all tests'
	@echo '  lint            Run linters'
	@echo '  fmt             Format code'

.PHONY: build-all
build-all:
	@echo "Building all distributions..."
	cd distributions/minimal && go build
	cd distributions/standard && go build
	cd distributions/enterprise && go build

.PHONY: test-all
test-all:
	@echo "Running all tests..."
	go test ./...
EOF
print_status "Created Makefile"

# Step 10: Create README
echo -e "\n${YELLOW}Step 10: Creating README${NC}"
cat > "${TARGET_DIR}/README.md" << 'EOF'
# Database Intelligence

A modular OpenTelemetry-based database monitoring solution.

## Structure

- `core/` - Core collector components
- `processors/` - Custom processors
- `receivers/` - Custom receivers
- `exporters/` - Custom exporters
- `extensions/` - Extensions
- `common/` - Shared libraries
- `distributions/` - Pre-built distributions
- `configs/` - Configuration templates
- `deployments/` - Deployment artifacts
- `tests/` - Test suites
- `tools/` - Build and development tools

## Quick Start

```bash
# Build all distributions
make build-all

# Run tests
make test-all

# Run enterprise distribution
./bin/collector-enterprise --config=configs/profiles/development.yaml
```

## Development

This project uses Go workspaces. To work on a specific module:

```bash
cd processors/adaptivesampler
go test ./...
```
EOF
print_status "Created README"

# Final summary
echo -e "\n${GREEN}Migration completed successfully!${NC}"
echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. cd ${TARGET_DIR}"
echo "2. go work sync"
echo "3. make build-all"
echo "4. make test-all"
echo ""
echo -e "${BLUE}The new workspace structure is ready at: ${TARGET_DIR}${NC}"