#!/bin/bash

# Update versions carefully following the v1.35.0 + v0.129.0 pattern
# This updates modules one by one to match the core module pattern

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

echo -e "${BLUE}=== UPDATING VERSIONS TO v1.35.0 + v0.129.0 PATTERN ===${NC}"

# ==============================================================================
# Step 1: Update processors to match core pattern
# ==============================================================================
echo -e "\n${CYAN}Step 1: Updating processor modules${NC}"

update_processor() {
    local proc_name=$1
    local proc_path="processors/$proc_name"
    
    if [ -d "$proc_path" ] && [ -f "$proc_path/go.mod" ]; then
        echo -e "\n${YELLOW}Updating $proc_name processor...${NC}"
        cd "$proc_path"
        
        # Update base packages to v1.35.0
        go get go.opentelemetry.io/collector/component@v1.35.0
        go get go.opentelemetry.io/collector/consumer@v1.35.0
        go get go.opentelemetry.io/collector/pdata@v1.35.0
        go get go.opentelemetry.io/collector/processor@v1.35.0
        
        # Remove direct confmap dependency if present
        go mod edit -droprequire=go.opentelemetry.io/collector/confmap
        
        # Tidy up
        go mod tidy
        
        cd "$PROJECT_ROOT"
        echo -e "${GREEN}[✓]${NC} $proc_name updated"
    fi
}

# Update each processor
for proc in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    update_processor "$proc"
done

# ==============================================================================
# Step 2: Update receivers
# ==============================================================================
echo -e "\n${CYAN}Step 2: Updating receiver modules${NC}"

update_receiver() {
    local recv_name=$1
    local recv_path="receivers/$recv_name"
    
    if [ -d "$recv_path" ] && [ -f "$recv_path/go.mod" ]; then
        echo -e "\n${YELLOW}Updating $recv_name receiver...${NC}"
        cd "$recv_path"
        
        # Update base packages to v1.35.0
        go get go.opentelemetry.io/collector/component@v1.35.0
        go get go.opentelemetry.io/collector/consumer@v1.35.0
        go get go.opentelemetry.io/collector/pdata@v1.35.0
        go get go.opentelemetry.io/collector/receiver@v1.35.0
        
        # Remove direct confmap dependency if present
        go mod edit -droprequire=go.opentelemetry.io/collector/confmap
        
        # Tidy up
        go mod tidy
        
        cd "$PROJECT_ROOT"
        echo -e "${GREEN}[✓]${NC} $recv_name updated"
    fi
}

# Update each receiver
for recv in ash enhancedsql kernelmetrics; do
    update_receiver "$recv"
done

# ==============================================================================
# Step 3: Update common modules
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating common modules${NC}"

update_common() {
    local common_path=$1
    
    if [ -d "$common_path" ] && [ -f "$common_path/go.mod" ]; then
        echo -e "\n${YELLOW}Updating $common_path...${NC}"
        cd "$common_path"
        
        # Update base packages if used
        go get go.opentelemetry.io/collector/pdata@v1.35.0 2>/dev/null || true
        go get go.opentelemetry.io/collector/component@v1.35.0 2>/dev/null || true
        
        # Remove direct confmap dependency if present
        go mod edit -droprequire=go.opentelemetry.io/collector/confmap 2>/dev/null || true
        
        # Tidy up
        go mod tidy
        
        cd "$PROJECT_ROOT"
        echo -e "${GREEN}[✓]${NC} $common_path updated"
    fi
}

# Update common modules
update_common "common"
update_common "common/featuredetector"
update_common "common/queryselector"

# ==============================================================================
# Step 4: Update exporters
# ==============================================================================
echo -e "\n${CYAN}Step 4: Updating exporter modules${NC}"

if [ -d "exporters/nri" ] && [ -f "exporters/nri/go.mod" ]; then
    echo -e "\n${YELLOW}Updating nri exporter...${NC}"
    cd "exporters/nri"
    
    # Update base packages to v1.35.0
    go get go.opentelemetry.io/collector/component@v1.35.0
    go get go.opentelemetry.io/collector/pdata@v1.35.0
    go get go.opentelemetry.io/collector/exporter@v1.35.0
    
    # Remove direct confmap dependency if present
    go mod edit -droprequire=go.opentelemetry.io/collector/confmap
    
    # Tidy up
    go mod tidy
    
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}[✓]${NC} nri exporter updated"
fi

# ==============================================================================
# Step 5: Update production distribution
# ==============================================================================
echo -e "\n${CYAN}Step 5: Updating production distribution${NC}"

if [ -d "distributions/production" ] && [ -f "distributions/production/go.mod" ]; then
    echo -e "\n${YELLOW}Updating production distribution...${NC}"
    cd "distributions/production"
    
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
# Step 6: Sync workspace
# ==============================================================================
echo -e "\n${CYAN}Step 6: Syncing workspace${NC}"

go work sync || echo -e "${YELLOW}[!]${NC} Workspace sync had warnings (this is expected)"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== VERSION UPDATE COMPLETE ===${NC}"

echo -e "\nAll modules updated to:"
echo "- Base packages: v1.35.0"
echo "- Implementation packages: v0.129.0"
echo "- Contrib packages: v0.129.0"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Test building each module"
echo "2. Run unit tests"
echo "3. Build complete collector"

echo -e "\n${GREEN}Versions are now aligned with the core module pattern!${NC}"