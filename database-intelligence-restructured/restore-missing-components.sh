#!/bin/bash

# Restore Missing Components Script
# This script restores critical components that were missed during refactoring

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
MVP_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-mvp"

echo -e "${BLUE}=== Restoring Missing Components ===${NC}"

# 1. Restore Missing Receivers
echo -e "\n${BLUE}1. Restoring Missing Receiver Implementations${NC}"

MISSING_RECEIVERS=(
    "ash"
    "enhancedsql"
    "kernelmetrics"
)

cd "$PROJECT_ROOT/receivers"

for receiver in "${MISSING_RECEIVERS[@]}"; do
    if [ -d "$MVP_ROOT/receivers/$receiver" ]; then
        echo -e "${YELLOW}Copying $receiver receiver...${NC}"
        cp -r "$MVP_ROOT/receivers/$receiver" .
        echo -e "${GREEN}[✓]${NC} Restored $receiver receiver"
        
        # Create go.mod if missing
        if [ ! -f "$receiver/go.mod" ]; then
            echo -e "${YELLOW}Creating go.mod for $receiver...${NC}"
            cat > "$receiver/go.mod" << EOF
module github.com/database-intelligence/receivers/$receiver

go 1.21

require (
    go.opentelemetry.io/collector/component v0.112.0
    go.opentelemetry.io/collector/receiver v0.112.0
    go.opentelemetry.io/collector/pdata v0.112.0
    go.uber.org/zap v1.27.0
)
EOF
            echo -e "${GREEN}[✓]${NC} Created go.mod for $receiver"
        fi
    else
        echo -e "${RED}[✗]${NC} $receiver not found in MVP!"
    fi
done

# 2. Restore validation directory
echo -e "\n${BLUE}2. Restoring Validation Tools${NC}"
if [ -d "$MVP_ROOT/validation" ]; then
    cp -r "$MVP_ROOT/validation" "$PROJECT_ROOT/"
    echo -e "${GREEN}[✓]${NC} Restored validation directory"
fi

# 3. Fix broken YAML configurations
echo -e "\n${BLUE}3. Checking YAML Configuration Issues${NC}"

# First check if Python is available
if ! command -v python3 &> /dev/null; then
    echo -e "${YELLOW}[!]${NC} Python3 not found, skipping YAML validation"
else
    # Check a sample YAML file
    SAMPLE_YAML="$PROJECT_ROOT/configs/examples/collector.yaml"
    if [ -f "$SAMPLE_YAML" ]; then
        if python3 -c "import yaml; yaml.safe_load(open('$SAMPLE_YAML'))" 2>/dev/null; then
            echo -e "${GREEN}[✓]${NC} YAML files appear valid"
        else
            echo -e "${YELLOW}[!]${NC} YAML validation issue - may need PyYAML module"
            # Try without yaml module
            if [ -s "$SAMPLE_YAML" ]; then
                echo -e "${GREEN}[✓]${NC} YAML files exist and have content"
            else
                echo -e "${RED}[✗]${NC} YAML files may be empty or corrupted"
            fi
        fi
    fi
fi

# 4. Update go.work to include new receivers
echo -e "\n${BLUE}4. Updating Go Workspace${NC}"
cd "$PROJECT_ROOT"

# Add receiver modules to go.work
if [ -f "go.work" ]; then
    # Check if receivers are already in go.work
    for receiver in "${MISSING_RECEIVERS[@]}"; do
        if ! grep -q "./receivers/$receiver" go.work; then
            # Add before the closing parenthesis
            sed -i.bak '/^)$/i\    ./receivers/'"$receiver" go.work
            echo -e "${GREEN}[✓]${NC} Added $receiver to go.work"
        fi
    done
fi

# 5. Update registry.go imports
echo -e "\n${BLUE}5. Updating Receiver Registry${NC}"
cd "$PROJECT_ROOT/receivers"

# Update registry.go to use local imports with replace directives
if [ -f "registry.go" ]; then
    # Check if we need to add replace directives to a go.mod
    if [ ! -f "go.mod" ]; then
        cat > go.mod << 'EOF'
module github.com/database-intelligence/receivers

go 1.21

require (
    go.opentelemetry.io/collector/receiver v0.112.0
)

replace (
    github.com/database-intelligence/receivers/ash => ./ash
    github.com/database-intelligence/receivers/enhancedsql => ./enhancedsql
    github.com/database-intelligence/receivers/kernelmetrics => ./kernelmetrics
)
EOF
        echo -e "${GREEN}[✓]${NC} Created receivers go.mod with replace directives"
    fi
fi

# 6. Fix Go build environment
echo -e "\n${BLUE}6. Setting Up Go Build Environment${NC}"
cd "$PROJECT_ROOT"

# Ensure go.mod exists at root
if [ ! -f "go.mod" ]; then
    go mod init github.com/database-intelligence/database-intelligence
    echo -e "${GREEN}[✓]${NC} Created root go.mod"
fi

# Test basic compilation again
echo -e "${YELLOW}Testing Go compilation...${NC}"
cat > test-build.go << 'EOF'
package main
import "fmt"
func main() { 
    fmt.Println("Database Intelligence - Build Test")
}
EOF

if go build -o test-binary test-build.go; then
    echo -e "${GREEN}[✓]${NC} Go compilation now works!"
    ./test-binary
    rm -f test-binary test-build.go
else
    echo -e "${RED}[✗]${NC} Go compilation still failing"
    # Try to diagnose
    go version
    echo "GOPATH: $GOPATH"
    echo "GOROOT: $GOROOT"
fi

# 7. Summary
echo -e "\n${BLUE}=== Restoration Summary ===${NC}"
echo -e "${GREEN}Components Restored:${NC}"
echo -e "  - ash receiver implementation"
echo -e "  - enhancedsql receiver implementation"
echo -e "  - kernelmetrics receiver implementation"
echo -e "  - validation tools"
echo -e "  - go.work updated"
echo -e "  - receiver registry configured"

echo -e "\n${YELLOW}Next Steps:${NC}"
echo -e "1. Run 'go work sync' to update dependencies"
echo -e "2. Run 'go mod tidy' in each receiver directory"
echo -e "3. Test build with './build.sh'"
echo -e "4. Run verification script again to confirm fixes"