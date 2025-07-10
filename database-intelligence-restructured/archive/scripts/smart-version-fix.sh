#!/bin/bash

# Smart version fix for OpenTelemetry dependencies
# This script maps versions correctly based on what's available

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
cd "$PROJECT_ROOT"

echo -e "${BLUE}=== SMART VERSION FIX FOR OPENTELEMETRY ===${NC}\n"

# Version mapping based on actual availability
# For v0.110.0 collector components, we use:
# - confmap: v1.16.0 (different versioning)
# - pdata: v1.16.0 (different versioning)
# - featuregate: v1.16.0 (different versioning)
# - client: v1.16.0 (different versioning)
# - config/configtelemetry: v0.110.0
# - other components: v0.110.0

# ==============================================================================
# Function to update a single module
# ==============================================================================
update_module() {
    local module_path=$1
    local module_name=$(basename "$module_path")
    
    echo -e "\n${YELLOW}Updating $module_name...${NC}"
    cd "$module_path"
    
    # First, let's see what we currently have
    echo "Current dependencies:"
    grep "go.opentelemetry.io/collector" go.mod | grep -v "^module" | head -5
    
    # Update using explicit version replacement in go.mod
    # This is more reliable than using go get with mismatched versions
    
    # Create a temporary go.mod with correct versions
    cp go.mod go.mod.backup
    
    # Replace versions in go.mod
    sed -i.bak \
        -e 's|go.opentelemetry.io/collector/confmap v0\.[0-9.]*|go.opentelemetry.io/collector/confmap v1.16.0|g' \
        -e 's|go.opentelemetry.io/collector/pdata v0\.[0-9.]*|go.opentelemetry.io/collector/pdata v1.16.0|g' \
        -e 's|go.opentelemetry.io/collector/featuregate v0\.[0-9.]*|go.opentelemetry.io/collector/featuregate v1.16.0|g' \
        -e 's|go.opentelemetry.io/collector/client v0\.[0-9.]*|go.opentelemetry.io/collector/client v1.16.0|g' \
        -e 's|go.opentelemetry.io/collector/component v0\.[0-9.]*|go.opentelemetry.io/collector/component v0.110.0|g' \
        -e 's|go.opentelemetry.io/collector/consumer v0\.[0-9.]*|go.opentelemetry.io/collector/consumer v0.110.0|g' \
        -e 's|go.opentelemetry.io/collector/processor v0\.[0-9.]*|go.opentelemetry.io/collector/processor v0.110.0|g' \
        -e 's|go.opentelemetry.io/collector/receiver v0\.[0-9.]*|go.opentelemetry.io/collector/receiver v0.110.0|g' \
        -e 's|go.opentelemetry.io/collector/exporter v0\.[0-9.]*|go.opentelemetry.io/collector/exporter v0.110.0|g' \
        -e 's|go.opentelemetry.io/collector/extension v0\.[0-9.]*|go.opentelemetry.io/collector/extension v0.110.0|g' \
        -e 's|go.opentelemetry.io/collector/semconv v0\.[0-9.]*|go.opentelemetry.io/collector/semconv v0.110.0|g' \
        go.mod
    
    # Also update contrib packages to v0.110.0
    sed -i.bak \
        -e 's|github.com/open-telemetry/opentelemetry-collector-contrib/[^ ]* v0\.[0-9.]*|&|g' \
        -e 's|v0\.[0-9.]*$|v0.110.0|g' \
        go.mod
    
    # Clean up backup
    rm -f go.mod.bak
    
    # Now run go mod tidy to resolve dependencies
    echo "Running go mod tidy..."
    if go mod tidy; then
        echo -e "${GREEN}[✓]${NC} $module_name updated successfully"
    else
        echo -e "${RED}[✗]${NC} Failed to update $module_name"
        # Restore backup on failure
        mv go.mod.backup go.mod
        return 1
    fi
    
    # Remove backup if successful
    rm -f go.mod.backup
    
    cd "$PROJECT_ROOT"
}

# ==============================================================================
# Step 1: Update processors
# ==============================================================================
echo -e "${CYAN}Step 1: Updating processors${NC}"

for processor_dir in processors/*/; do
    if [ -f "$processor_dir/go.mod" ]; then
        update_module "$processor_dir"
    fi
done

# ==============================================================================
# Step 2: Update receivers
# ==============================================================================
echo -e "\n${CYAN}Step 2: Updating receivers${NC}"

for receiver_dir in receivers/*/; do
    if [ -f "$receiver_dir/go.mod" ]; then
        update_module "$receiver_dir"
    fi
done

# ==============================================================================
# Step 3: Update exporters
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating exporters${NC}"

if [ -d "exporters/nri" ]; then
    update_module "exporters/nri"
fi

# ==============================================================================
# Step 4: Update extensions
# ==============================================================================
echo -e "\n${CYAN}Step 4: Updating extensions${NC}"

if [ -d "extensions/healthcheck" ]; then
    update_module "extensions/healthcheck"
fi

# ==============================================================================
# Step 5: Update common modules
# ==============================================================================
echo -e "\n${CYAN}Step 5: Updating common modules${NC}"

for common_dir in common common/featuredetector common/queryselector; do
    if [ -f "$common_dir/go.mod" ]; then
        update_module "$common_dir"
    fi
done

# ==============================================================================
# Step 6: Sync workspace
# ==============================================================================
echo -e "\n${CYAN}Step 6: Syncing workspace${NC}"

go work sync

echo -e "${GREEN}[✓]${NC} Workspace synced"

# ==============================================================================
# Step 7: Test builds
# ==============================================================================
echo -e "\n${CYAN}Step 7: Testing builds${NC}"

# Test a few key modules
echo -e "\n${YELLOW}Testing adaptivesampler build...${NC}"
if (cd processors/adaptivesampler && go build ./...); then
    echo -e "${GREEN}[✓]${NC} adaptivesampler builds successfully"
else
    echo -e "${RED}[✗]${NC} adaptivesampler build failed"
fi

echo -e "\n${YELLOW}Testing ash receiver build...${NC}"
if (cd receivers/ash && go build ./...); then
    echo -e "${GREEN}[✓]${NC} ash receiver builds successfully"
else
    echo -e "${RED}[✗]${NC} ash receiver build failed"
fi

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== SMART VERSION FIX COMPLETE ===${NC}"

echo -e "\nVersion mapping applied:"
echo "- confmap: v0.110.0 → v1.16.0"
echo "- pdata: v0.110.0 → v1.16.0"
echo "- featuregate: v0.110.0 → v1.16.0"
echo "- Other components: v0.110.0"
echo "- Contrib packages: v0.110.0"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Run ./test-builds.sh to test all module builds"
echo "2. Build a working collector"
echo "3. Run E2E tests"