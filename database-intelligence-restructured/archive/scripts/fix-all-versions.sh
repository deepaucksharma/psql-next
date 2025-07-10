#!/bin/bash

# Fix all OpenTelemetry version conflicts
# This script updates all modules to use consistent versions

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

echo -e "${BLUE}=== FIXING OPENTELEMETRY VERSION CONFLICTS ===${NC}\n"

# Target versions based on analysis
OTEL_VERSION="0.110.0"
CONFMAP_VERSION="1.16.0"  # confmap uses different versioning
PDATA_VERSION="1.16.0"
FEATUREGATE_VERSION="1.16.0"
SEMCONV_VERSION="0.110.0"
CLIENT_VERSION="1.16.0"

# Contrib version
CONTRIB_VERSION="0.110.0"

echo -e "${CYAN}Target versions:${NC}"
echo "- OpenTelemetry Collector: v${OTEL_VERSION}"
echo "- Confmap: v${CONFMAP_VERSION}"
echo "- Pdata: v${PDATA_VERSION}"
echo "- Contrib: v${CONTRIB_VERSION}"

# ==============================================================================
# Step 1: Update all processor modules
# ==============================================================================
echo -e "\n${CYAN}Step 1: Updating processor modules${NC}"

PROCESSORS=(
    "adaptivesampler"
    "circuitbreaker"
    "costcontrol"
    "nrerrormonitor"
    "planattributeextractor"
    "querycorrelator"
    "verification"
)

for processor in "${PROCESSORS[@]}"; do
    if [ -d "processors/$processor" ]; then
        echo -e "\n${YELLOW}Updating $processor...${NC}"
        cd "processors/$processor"
        
        # Update go.mod
        go get -u go.opentelemetry.io/collector/component@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/confmap@v${CONFMAP_VERSION}
        go get -u go.opentelemetry.io/collector/consumer@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/processor@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/pdata@v${PDATA_VERSION}
        go get -u go.opentelemetry.io/collector/semconv@v${SEMCONV_VERSION}
        
        # Run go mod tidy
        go mod tidy
        
        echo -e "${GREEN}[✓]${NC} $processor updated"
        cd "$PROJECT_ROOT"
    fi
done

# ==============================================================================
# Step 2: Update all receiver modules
# ==============================================================================
echo -e "\n${CYAN}Step 2: Updating receiver modules${NC}"

RECEIVERS=(
    "ash"
    "enhancedsql"
    "kernelmetrics"
)

for receiver in "${RECEIVERS[@]}"; do
    if [ -d "receivers/$receiver" ]; then
        echo -e "\n${YELLOW}Updating $receiver...${NC}"
        cd "receivers/$receiver"
        
        # Update go.mod
        go get -u go.opentelemetry.io/collector/component@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/confmap@v${CONFMAP_VERSION}
        go get -u go.opentelemetry.io/collector/consumer@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/receiver@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/pdata@v${PDATA_VERSION}
        go get -u go.opentelemetry.io/collector/semconv@v${SEMCONV_VERSION}
        
        # Run go mod tidy
        go mod tidy
        
        echo -e "${GREEN}[✓]${NC} $receiver updated"
        cd "$PROJECT_ROOT"
    fi
done

# ==============================================================================
# Step 3: Update exporter modules
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating exporter modules${NC}"

if [ -d "exporters/nri" ]; then
    echo -e "\n${YELLOW}Updating nri exporter...${NC}"
    cd "exporters/nri"
    
    go get -u go.opentelemetry.io/collector/component@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/confmap@v${CONFMAP_VERSION}
    go get -u go.opentelemetry.io/collector/exporter@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/pdata@v${PDATA_VERSION}
    
    go mod tidy
    
    echo -e "${GREEN}[✓]${NC} nri exporter updated"
    cd "$PROJECT_ROOT"
fi

# ==============================================================================
# Step 4: Update extension modules
# ==============================================================================
echo -e "\n${CYAN}Step 4: Updating extension modules${NC}"

if [ -d "extensions/healthcheck" ]; then
    echo -e "\n${YELLOW}Updating healthcheck extension...${NC}"
    cd "extensions/healthcheck"
    
    go get -u go.opentelemetry.io/collector/component@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/confmap@v${CONFMAP_VERSION}
    go get -u go.opentelemetry.io/collector/extension@v${OTEL_VERSION}
    
    go mod tidy
    
    echo -e "${GREEN}[✓]${NC} healthcheck extension updated"
    cd "$PROJECT_ROOT"
fi

# ==============================================================================
# Step 5: Update common modules
# ==============================================================================
echo -e "\n${CYAN}Step 5: Updating common modules${NC}"

COMMON_MODULES=(
    "common"
    "common/featuredetector"
    "common/queryselector"
)

for module in "${COMMON_MODULES[@]}"; do
    if [ -d "$module" ]; then
        echo -e "\n${YELLOW}Updating $module...${NC}"
        cd "$module"
        
        go get -u go.opentelemetry.io/collector/component@v${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/confmap@v${CONFMAP_VERSION}
        go get -u go.opentelemetry.io/collector/pdata@v${PDATA_VERSION}
        
        go mod tidy
        
        echo -e "${GREEN}[✓]${NC} $module updated"
        cd "$PROJECT_ROOT"
    fi
done

# ==============================================================================
# Step 6: Update distribution modules
# ==============================================================================
echo -e "\n${CYAN}Step 6: Updating distribution modules${NC}"

# Update minimal distribution
if [ -d "distributions/minimal" ]; then
    echo -e "\n${YELLOW}Updating minimal distribution...${NC}"
    cd "distributions/minimal"
    
    go get -u go.opentelemetry.io/collector/component@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/confmap@v${CONFMAP_VERSION}
    go get -u go.opentelemetry.io/collector/otelcol@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/receiver@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/processor@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/exporter@v${OTEL_VERSION}
    go get -u go.opentelemetry.io/collector/extension@v${OTEL_VERSION}
    
    # Update contrib packages
    go get -u github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver@v${CONTRIB_VERSION}
    go get -u github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver@v${CONTRIB_VERSION}
    
    go mod tidy
    
    echo -e "${GREEN}[✓]${NC} minimal distribution updated"
    cd "$PROJECT_ROOT"
fi

# ==============================================================================
# Step 7: Sync workspace
# ==============================================================================
echo -e "\n${CYAN}Step 7: Syncing Go workspace${NC}"

go work sync

echo -e "${GREEN}[✓]${NC} Workspace synced"

# ==============================================================================
# Step 8: Verify updates
# ==============================================================================
echo -e "\n${CYAN}Step 8: Verifying updates${NC}"

# Check a sample of modules
echo -e "\n${YELLOW}Sample verification:${NC}"

# Check a processor
if [ -f "processors/adaptivesampler/go.mod" ]; then
    echo -e "\nAdaptive Sampler versions:"
    grep "go.opentelemetry.io/collector/component" processors/adaptivesampler/go.mod || true
    grep "go.opentelemetry.io/collector/confmap" processors/adaptivesampler/go.mod || true
fi

# Check a receiver
if [ -f "receivers/ash/go.mod" ]; then
    echo -e "\nASH Receiver versions:"
    grep "go.opentelemetry.io/collector/component" receivers/ash/go.mod || true
    grep "go.opentelemetry.io/collector/confmap" receivers/ash/go.mod || true
fi

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== VERSION FIX COMPLETE ===${NC}"
echo -e "\nAll modules have been updated to use:"
echo -e "- OpenTelemetry Collector: v${OTEL_VERSION}"
echo -e "- Confmap: v${CONFMAP_VERSION}"
echo -e "- Pdata: v${PDATA_VERSION}"
echo -e "- Contrib: v${CONTRIB_VERSION}"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Run tests for each module"
echo "2. Build a test collector"
echo "3. Run E2E tests"

# Create a build test script
cat > "$PROJECT_ROOT/test-builds.sh" << 'EOF'
#!/bin/bash

# Test building key modules after version updates

set -e

echo "Testing processor builds..."
for processor in processors/*/; do
    if [ -f "$processor/go.mod" ]; then
        echo "Building $(basename $processor)..."
        (cd "$processor" && go build ./...)
    fi
done

echo "Testing receiver builds..."
for receiver in receivers/*/; do
    if [ -f "$receiver/go.mod" ]; then
        echo "Building $(basename $receiver)..."
        (cd "$receiver" && go build ./...)
    fi
done

echo "All builds successful!"
EOF

chmod +x "$PROJECT_ROOT/test-builds.sh"

echo -e "\nRun ${GREEN}./test-builds.sh${NC} to test all builds"