#!/bin/bash

# Fix receiver versions with correct scraper package versions
# Scraper packages use v0.129.0 pattern, not v1.35.0

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

echo -e "${BLUE}=== FIXING RECEIVER VERSIONS ===${NC}"

# ==============================================================================
# Step 1: Fix ash receiver with correct scraper versions
# ==============================================================================
echo -e "\n${CYAN}Step 1: Fixing ash receiver${NC}"

cd "receivers/ash"
echo -e "\n${YELLOW}Updating ash receiver with correct versions...${NC}"

# Update base packages to v1.35.0
go get go.opentelemetry.io/collector/component@v1.35.0
go get go.opentelemetry.io/collector/consumer@v1.35.0
go get go.opentelemetry.io/collector/pdata@v1.35.0
go get go.opentelemetry.io/collector/receiver@v1.35.0
go get go.opentelemetry.io/collector/processor@v1.35.0

# Add scraper packages with v0.129.0 versioning
go get go.opentelemetry.io/collector/scraper@v0.129.0
go get go.opentelemetry.io/collector/scraper/scraperhelper@v0.129.0

# Remove direct confmap dependency if present
go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true

# Tidy up
go mod tidy

cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} ash receiver fixed"

# ==============================================================================
# Step 2: Fix kernelmetrics receiver (if it uses scraper)
# ==============================================================================
echo -e "\n${CYAN}Step 2: Checking kernelmetrics receiver${NC}"

# First check if kernelmetrics uses scraper packages
if grep -q "scraper" "receivers/kernelmetrics/"*.go 2>/dev/null; then
    cd "receivers/kernelmetrics"
    echo -e "\n${YELLOW}Updating kernelmetrics receiver with scraper packages...${NC}"
    
    # Update base packages to v1.35.0
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/consumer@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/receiver@v1.35.0
    
    # Add scraper packages with v0.129.0 versioning
    go get go.opentelemetry.io/collector/scraper@v0.129.0
    go get go.opentelemetry.io/collector/scraper/scraperhelper@v0.129.0
    
    # Remove direct confmap dependency if present
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Tidy up
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} kernelmetrics receiver fixed"
else
    echo -e "${YELLOW}kernelmetrics doesn't use scraper packages, updating normally...${NC}"
    cd "receivers/kernelmetrics"
    
    # Update base packages to v1.35.0
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/consumer@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/receiver@v1.35.0
    
    # Remove direct confmap dependency if present
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Tidy up
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} kernelmetrics receiver updated"
fi

# ==============================================================================
# Step 3: Fix enhancedsql receiver
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating enhancedsql receiver${NC}"

cd "receivers/enhancedsql"
echo -e "\n${YELLOW}Updating enhancedsql receiver...${NC}"

# Update base packages to v1.35.0
go get go.opentelemetry.io/collector/component@v1.35.0
go get go.opentelemetry.io/collector/consumer@v1.35.0
go get go.opentelemetry.io/collector/pdata@v1.35.0
go get go.opentelemetry.io/collector/receiver@v1.35.0

# Remove direct confmap dependency if present
go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true

# Tidy up
go mod tidy

cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} enhancedsql receiver updated"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== RECEIVER VERSION FIXES COMPLETE ===${NC}"

echo -e "\nReceivers have been updated with:"
echo "- Base packages: v1.35.0"
echo "- Scraper packages: v0.129.0 (where used)"

echo -e "\n${GREEN}Ready to continue with remaining updates!${NC}"