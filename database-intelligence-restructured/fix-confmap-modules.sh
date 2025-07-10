#!/bin/bash

# Fix modules with confmap v0.110.0 dependency

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

echo -e "${BLUE}=== FIXING CONFMAP V0.110.0 DEPENDENCIES ===${NC}"

# ==============================================================================
# Step 1: Fix exporters/nri
# ==============================================================================
echo -e "\n${CYAN}Step 1: Fixing exporters/nri${NC}"

if [ -d "exporters/nri" ]; then
    cd "exporters/nri"
    echo -e "\n${YELLOW}Removing confmap dependency from nri exporter...${NC}"
    
    # Remove confmap dependency
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Update to correct versions
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/exporter@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    
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
    echo -e "\n${YELLOW}Removing confmap dependency from enhancedsql receiver...${NC}"
    
    # Remove confmap dependency
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
    echo -e "\n${YELLOW}Removing confmap dependency from healthcheck extension...${NC}"
    
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
# Step 4: Sync workspace
# ==============================================================================
echo -e "\n${CYAN}Step 4: Syncing workspace${NC}"

cd "$PROJECT_ROOT"
go work sync || echo -e "${YELLOW}[!]${NC} Workspace sync had warnings"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== CONFMAP FIXES COMPLETE ===${NC}"

echo -e "\nAll confmap v0.110.0 dependencies have been removed"
echo -e "\n${GREEN}Ready to build!${NC}"