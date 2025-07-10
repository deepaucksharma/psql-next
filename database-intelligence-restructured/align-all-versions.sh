#!/bin/bash

# Comprehensive version alignment script
# Aligns all OpenTelemetry dependencies to compatible versions

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"

echo -e "${BLUE}=== Aligning All OpenTelemetry Versions ===${NC}"

cd "$PROJECT_ROOT"

# Define consistent versions
OTEL_VERSION="v0.112.0"  # Stable version that should work
OTEL_CONTRIB_VERSION="v0.112.0"

echo -e "${YELLOW}Using OpenTelemetry versions:${NC}"
echo -e "  Collector: ${OTEL_VERSION}"
echo -e "  Contrib: ${OTEL_CONTRIB_VERSION}"

# Function to update go.mod file
update_go_mod() {
    local dir=$1
    local modfile="$dir/go.mod"
    
    if [ ! -f "$modfile" ]; then
        return
    fi
    
    echo -e "${YELLOW}Updating $dir...${NC}"
    cd "$dir"
    
    # Remove existing require blocks to start fresh
    cp go.mod go.mod.bak
    
    # Extract module name and Go version
    MODULE_NAME=$(grep "^module " go.mod | awk '{print $2}')
    GO_VERSION=$(grep "^go " go.mod | awk '{print $2}')
    
    # Start fresh go.mod
    cat > go.mod << EOF
module $MODULE_NAME

go $GO_VERSION

require (
EOF
    
    # Add OpenTelemetry dependencies with consistent versions
    if grep -q "go.opentelemetry.io/collector" go.mod.bak; then
        cat >> go.mod << EOF
    go.opentelemetry.io/collector/component ${OTEL_VERSION}
    go.opentelemetry.io/collector/confmap ${OTEL_VERSION}
    go.opentelemetry.io/collector/consumer ${OTEL_VERSION}
    go.opentelemetry.io/collector/pdata ${OTEL_VERSION}
EOF
    fi
    
    if grep -q "go.opentelemetry.io/collector/processor" go.mod.bak; then
        cat >> go.mod << EOF
    go.opentelemetry.io/collector/processor ${OTEL_VERSION}
EOF
    fi
    
    if grep -q "go.opentelemetry.io/collector/exporter" go.mod.bak; then
        cat >> go.mod << EOF
    go.opentelemetry.io/collector/exporter ${OTEL_VERSION}
EOF
    fi
    
    if grep -q "go.opentelemetry.io/collector/receiver" go.mod.bak; then
        cat >> go.mod << EOF
    go.opentelemetry.io/collector/receiver ${OTEL_VERSION}
EOF
    fi
    
    # Add contrib dependencies
    if grep -q "github.com/open-telemetry/opentelemetry-collector-contrib" go.mod.bak; then
        cat >> go.mod << EOF
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchprocessor ${OTEL_CONTRIB_VERSION}
EOF
    fi
    
    # Add other dependencies
    cat >> go.mod << EOF
    go.uber.org/zap v1.27.0
    github.com/stretchr/testify v1.9.0
)
EOF
    
    # Add replace directives from backup if they exist
    if grep -q "^replace" go.mod.bak; then
        echo "" >> go.mod
        grep -A 1000 "^replace" go.mod.bak >> go.mod || true
    fi
    
    echo -e "${GREEN}[✓]${NC} Updated $dir"
}

# Update all processor modules
echo -e "\n${BLUE}Updating processor modules...${NC}"
for processor in processors/*; do
    if [ -d "$processor" ] && [ -f "$processor/go.mod" ]; then
        update_go_mod "$PROJECT_ROOT/$processor"
    fi
done

# Update common modules
echo -e "\n${BLUE}Updating common modules...${NC}"
update_go_mod "$PROJECT_ROOT/common"
update_go_mod "$PROJECT_ROOT/common/featuredetector"
update_go_mod "$PROJECT_ROOT/common/queryselector"

# Update core module
echo -e "\n${BLUE}Updating core module...${NC}"
update_go_mod "$PROJECT_ROOT/core"

# Update distribution modules
echo -e "\n${BLUE}Updating distribution modules...${NC}"
for dist in distributions/*; do
    if [ -d "$dist" ] && [ -f "$dist/go.mod" ]; then
        update_go_mod "$PROJECT_ROOT/$dist"
    fi
done

# Update exporter modules
echo -e "\n${BLUE}Updating exporter modules...${NC}"
update_go_mod "$PROJECT_ROOT/exporters/nri"

# Update extension modules
echo -e "\n${BLUE}Updating extension modules...${NC}"
update_go_mod "$PROJECT_ROOT/extensions/healthcheck"

# Now create a minimal working example
echo -e "\n${BLUE}Creating minimal working example...${NC}"
cd "$PROJECT_ROOT"

cat > test-build.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    fmt.Println("Database Intelligence Collector - Test Build")
    
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        fmt.Println("\nShutting down...")
        cancel()
    }()
    
    fmt.Println("Collector is running. Press Ctrl+C to stop.")
    <-ctx.Done()
    
    fmt.Println("Collector stopped.")
}
EOF

# Create a simple go.mod for testing
cat > go.mod << EOF
module github.com/database-intelligence/test

go 1.21
EOF

# Try to build the test
echo -e "\n${BLUE}Testing basic build...${NC}"
if go build -o test-collector test-build.go; then
    echo -e "${GREEN}[✓]${NC} Basic build successful"
    rm -f test-collector test-build.go
else
    echo -e "${RED}[✗]${NC} Basic build failed"
fi

echo -e "\n${BLUE}=== Version Alignment Complete ===${NC}"
echo -e "${YELLOW}Note: You may need to run 'go mod tidy' in each module directory${NC}"
echo -e "${YELLOW}Some modules may require manual intervention for complex dependencies${NC}"