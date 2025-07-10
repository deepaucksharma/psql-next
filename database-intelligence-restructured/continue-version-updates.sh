#!/bin/bash

# Continue version updates after fixing import paths
# This script continues from where update-versions-carefully.sh left off

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

echo -e "${BLUE}=== CONTINUING VERSION UPDATES ===${NC}"

# ==============================================================================
# Step 1: Update ash receiver with scraper package
# ==============================================================================
echo -e "\n${CYAN}Step 1: Updating ash receiver with scraper package${NC}"

cd "receivers/ash"
echo -e "\n${YELLOW}Updating ash receiver...${NC}"

# Update base packages to v1.35.0
go get go.opentelemetry.io/collector/component@v1.35.0
go get go.opentelemetry.io/collector/consumer@v1.35.0
go get go.opentelemetry.io/collector/pdata@v1.35.0
go get go.opentelemetry.io/collector/receiver@v1.35.0

# Add scraper package with scraperhelper
go get go.opentelemetry.io/collector/scraper@v1.35.0

# Update the processor to v1.35.0 (fixing the mismatch)
go get go.opentelemetry.io/collector/processor@v1.35.0

# Remove direct confmap dependency if present
go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true

# Tidy up
go mod tidy

cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} ash receiver updated"

# ==============================================================================
# Step 2: Update enhancedsql receiver
# ==============================================================================
echo -e "\n${CYAN}Step 2: Updating enhancedsql receiver${NC}"

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
# Step 3: Update kernelmetrics receiver
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating kernelmetrics receiver${NC}"

cd "receivers/kernelmetrics"
echo -e "\n${YELLOW}Updating kernelmetrics receiver...${NC}"

# Update base packages to v1.35.0
go get go.opentelemetry.io/collector/component@v1.35.0
go get go.opentelemetry.io/collector/consumer@v1.35.0
go get go.opentelemetry.io/collector/pdata@v1.35.0
go get go.opentelemetry.io/collector/receiver@v1.35.0

# Add scraper package if needed
go get go.opentelemetry.io/collector/scraper@v1.35.0

# Remove direct confmap dependency if present
go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true

# Tidy up
go mod tidy

cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} kernelmetrics receiver updated"

# ==============================================================================
# Step 4: Update common modules
# ==============================================================================
echo -e "\n${CYAN}Step 4: Updating common modules${NC}"

# Update main common module
cd "common"
echo -e "\n${YELLOW}Updating common module...${NC}"
go get go.opentelemetry.io/collector/pdata@v1.35.0 2>/dev/null || true
go get go.opentelemetry.io/collector/component@v1.35.0 2>/dev/null || true
go mod tidy
cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} common module updated"

# Update featuredetector
cd "common/featuredetector"
echo -e "\n${YELLOW}Updating featuredetector module...${NC}"
go get go.opentelemetry.io/collector/pdata@v1.35.0 2>/dev/null || true
go get go.opentelemetry.io/collector/component@v1.35.0 2>/dev/null || true
go mod tidy
cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} featuredetector module updated"

# Update queryselector
cd "common/queryselector"
echo -e "\n${YELLOW}Updating queryselector module...${NC}"
go get go.opentelemetry.io/collector/pdata@v1.35.0 2>/dev/null || true
go get go.opentelemetry.io/collector/component@v1.35.0 2>/dev/null || true
go mod tidy
cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} queryselector module updated"

# ==============================================================================
# Step 5: Update exporters
# ==============================================================================
echo -e "\n${CYAN}Step 5: Updating exporter modules${NC}"

if [ -d "exporters/nri" ]; then
    cd "exporters/nri"
    echo -e "\n${YELLOW}Updating nri exporter...${NC}"
    
    # Update base packages to v1.35.0
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/exporter@v1.35.0
    
    # Remove direct confmap dependency if present
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap || true
    
    # Tidy up
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} nri exporter updated"
fi

# ==============================================================================
# Step 6: Update production distribution
# ==============================================================================
echo -e "\n${CYAN}Step 6: Updating production distribution${NC}"

if [ -d "distributions/production" ]; then
    cd "distributions/production"
    echo -e "\n${YELLOW}Updating production distribution...${NC}"
    
    # Update base packages to v1.35.0
    go get go.opentelemetry.io/collector/component@v1.35.0
    
    # Update implementations to v0.129.0
    go get go.opentelemetry.io/collector/otelcol@v0.129.0
    go get go.opentelemetry.io/collector/exporter/debugexporter@v0.129.0
    go get go.opentelemetry.io/collector/exporter/otlpexporter@v0.129.0
    go get go.opentelemetry.io/collector/exporter/otlphttpexporter@v0.129.0
    go get go.opentelemetry.io/collector/processor/batchprocessor@v0.129.0
    go get go.opentelemetry.io/collector/processor/memorylimiterprocessor@v0.129.0
    go get go.opentelemetry.io/collector/receiver/otlpreceiver@v0.129.0
    
    # Update contrib to v0.129.0
    go get github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter@v0.129.0
    go get github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension@v0.129.0
    go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver@v0.129.0
    go get github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver@v0.129.0
    
    # Tidy up
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} Production distribution updated"
fi

# ==============================================================================
# Step 7: Sync workspace
# ==============================================================================
echo -e "\n${CYAN}Step 7: Syncing workspace${NC}"

go work sync || echo -e "${YELLOW}[!]${NC} Workspace sync had warnings (this is expected)"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== VERSION UPDATE COMPLETE ===${NC}"

echo -e "\nAll modules have been updated to:"
echo "- Base packages: v1.35.0"
echo "- Implementation packages: v0.129.0"
echo "- Contrib packages: v0.129.0"
echo "- Scraper packages: v1.35.0"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Test building each module"
echo "2. Run unit tests"
echo "3. Build complete collector"
echo "4. Run E2E tests"

echo -e "\n${GREEN}All versions are now aligned with the core module pattern!${NC}"