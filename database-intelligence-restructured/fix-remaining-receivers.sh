#!/bin/bash

# Fix remaining receiver issues

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

echo -e "${BLUE}=== FIXING REMAINING RECEIVERS ===${NC}"

# ==============================================================================
# Step 1: Fix kernelmetrics receiver
# ==============================================================================
echo -e "\n${CYAN}Step 1: Fixing kernelmetrics receiver${NC}"

cd "receivers/kernelmetrics"
echo -e "\n${YELLOW}Removing confmap dependency and updating...${NC}"

# First remove the confmap dependency
go mod edit -droprequire=go.opentelemetry.io/collector/confmap

# Update base packages to v1.35.0
go get go.opentelemetry.io/collector/component@v1.35.0
go get go.opentelemetry.io/collector/consumer@v1.35.0
go get go.opentelemetry.io/collector/pdata@v1.35.0
go get go.opentelemetry.io/collector/receiver@v1.35.0
go get go.opentelemetry.io/collector/processor@v1.35.0

# Add scraper packages with v0.129.0 versioning if needed
go get go.opentelemetry.io/collector/scraper@v0.129.0
go get go.opentelemetry.io/collector/scraper/scraperhelper@v0.129.0

# Update go version
go mod edit -go=1.23.0

# Tidy up
go mod tidy

cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} kernelmetrics receiver fixed"

# ==============================================================================
# Step 2: Fix enhancedsql receiver
# ==============================================================================
echo -e "\n${CYAN}Step 2: Fixing enhancedsql receiver${NC}"

cd "receivers/enhancedsql"
echo -e "\n${YELLOW}Updating enhancedsql receiver...${NC}"

# Update go version first
go mod edit -go=1.23.0

# Update base packages to v1.35.0
go get go.opentelemetry.io/collector/component@v1.35.0
go get go.opentelemetry.io/collector/consumer@v1.35.0
go get go.opentelemetry.io/collector/pdata@v1.35.0
go get go.opentelemetry.io/collector/receiver@v1.35.0

# Tidy up
go mod tidy

cd "$PROJECT_ROOT"
echo -e "${GREEN}[✓]${NC} enhancedsql receiver updated"

# ==============================================================================
# Step 3: Update remaining modules
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating remaining modules${NC}"

# Update common modules
for module in common common/featuredetector common/queryselector; do
    if [ -d "$module" ] && [ -f "$module/go.mod" ]; then
        echo -e "\n${YELLOW}Updating $module...${NC}"
        cd "$module"
        
        # Update go version
        go mod edit -go=1.23.0 || true
        
        # Update packages if they exist
        go get go.opentelemetry.io/collector/pdata@v1.35.0 2>/dev/null || true
        go get go.opentelemetry.io/collector/component@v1.35.0 2>/dev/null || true
        
        # Tidy up
        go mod tidy
        
        cd "$PROJECT_ROOT"
        echo -e "${GREEN}[✓]${NC} $module updated"
    fi
done

# Update nri exporter
if [ -d "exporters/nri" ] && [ -f "exporters/nri/go.mod" ]; then
    echo -e "\n${YELLOW}Updating nri exporter...${NC}"
    cd "exporters/nri"
    
    # Update go version
    go mod edit -go=1.23.0 || true
    
    # Update base packages to v1.35.0
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/exporter@v1.35.0
    
    # Tidy up
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} nri exporter updated"
fi

# Update production distribution
if [ -d "distributions/production" ] && [ -f "distributions/production/go.mod" ]; then
    echo -e "\n${YELLOW}Updating production distribution...${NC}"
    cd "distributions/production"
    
    # Update go version
    go mod edit -go=1.23.0 || true
    
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
# Step 4: Sync workspace
# ==============================================================================
echo -e "\n${CYAN}Step 4: Syncing workspace${NC}"

cd "$PROJECT_ROOT"
go work sync || echo -e "${YELLOW}[!]${NC} Workspace sync had warnings (this is expected)"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== ALL MODULES UPDATED ===${NC}"

echo -e "\nAll modules have been updated to:"
echo "- Base packages: v1.35.0"
echo "- Implementation packages: v0.129.0"
echo "- Scraper packages: v0.129.0"
echo "- Go version: 1.23.0"

echo -e "\n${YELLOW}Next step:${NC}"
echo "Build and test all components"

echo -e "\n${GREEN}Version alignment complete!${NC}"