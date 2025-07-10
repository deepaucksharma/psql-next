#!/bin/bash

# Fix remaining confmap dependencies with correct versions

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

echo -e "${BLUE}=== FIXING REMAINING CONFMAP DEPENDENCIES ===${NC}"

# ==============================================================================
# Step 1: Fix exporters/nri with correct versions
# ==============================================================================
echo -e "\n${CYAN}Step 1: Fixing exporters/nri${NC}"

if [ -d "exporters/nri" ]; then
    cd "exporters/nri"
    echo -e "\n${YELLOW}Updating nri exporter with correct versions...${NC}"
    
    # Update to correct versions (exporter uses v0.129.0)
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/exporter@v0.129.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/consumer@v1.35.0
    go get go.opentelemetry.io/collector/processor@v1.35.0
    
    # Remove old dependencies
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Tidy
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} nri exporter fixed"
fi

# ==============================================================================
# Step 2: Fix receivers/enhancedsql
# ==============================================================================
echo -e "\n${CYAN}Step 2: Fixing receivers/enhancedsql${NC}"

if [ -d "receivers/enhancedsql" ]; then
    cd "receivers/enhancedsql"
    echo -e "\n${YELLOW}Updating enhancedsql receiver...${NC}"
    
    # Remove confmap dependency first
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Update to correct versions
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/consumer@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/receiver@v1.35.0
    
    # Tidy
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} enhancedsql receiver fixed"
fi

# ==============================================================================
# Step 3: Fix extensions/healthcheck
# ==============================================================================
echo -e "\n${CYAN}Step 3: Fixing extensions/healthcheck${NC}"

if [ -d "extensions/healthcheck" ]; then
    cd "extensions/healthcheck"
    echo -e "\n${YELLOW}Updating healthcheck extension...${NC}"
    
    # Remove confmap dependency
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Update to correct versions
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/extension@v1.35.0
    
    # Tidy
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} healthcheck extension fixed"
fi

# ==============================================================================
# Step 4: Now try to build production distribution
# ==============================================================================
echo -e "\n${CYAN}Step 4: Building production distribution${NC}"

cd "distributions/production"
echo -e "\n${YELLOW}Building collector binary...${NC}"

# Clean and build
rm -f otelcol-production
go build -o otelcol-production .

if [ -f "otelcol-production" ]; then
    echo -e "${GREEN}[✓]${NC} Production collector built successfully!"
    ls -la otelcol-production
else
    echo -e "${RED}[✗]${NC} Failed to build production collector"
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== BUILD COMPLETE ===${NC}"

if [ -f "distributions/production/otelcol-production" ]; then
    echo -e "\n${GREEN}Success! The production collector has been built.${NC}"
    echo -e "\nYou can now run E2E tests with the working collector."
else
    echo -e "\n${YELLOW}Build failed. Check the errors above.${NC}"
fi