#!/bin/bash

# Fix local imports to use replace directives in go.mod files
# This ensures all modules reference local paths instead of GitHub

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"

echo -e "${BLUE}=== Fixing Local Module Imports ===${NC}"

cd "$PROJECT_ROOT"

# First, ensure go.work is properly set up
echo -e "\n${BLUE}Setting up Go workspace...${NC}"
cat > go.work << 'EOF'
go 1.21

use (
    ./core
    ./common
    ./common/featuredetector
    ./common/queryselector
    ./processors/adaptivesampler
    ./processors/circuitbreaker
    ./processors/costcontrol
    ./processors/nrerrormonitor
    ./processors/planattributeextractor
    ./processors/querycorrelator
    ./processors/verification
    ./exporters/nri
    ./extensions/healthcheck
    ./distributions/minimal
    ./distributions/standard
    ./distributions/enterprise
    ./tests
    ./tests/integration
    ./tests/e2e
)
EOF

echo -e "${GREEN}[✓]${NC} Created go.work file"

# Fix core module dependencies
echo -e "\n${BLUE}Fixing core module...${NC}"
cd "$PROJECT_ROOT/core"

# Add replace directives for local modules
cat >> go.mod << 'EOF'

replace (
    github.com/database-intelligence/common => ../common
    github.com/database-intelligence/common/featuredetector => ../common/featuredetector
    github.com/database-intelligence/common/queryselector => ../common/queryselector
    github.com/database-intelligence/processors/adaptivesampler => ../processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => ../processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => ../processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => ../processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => ../processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => ../processors/querycorrelator
    github.com/database-intelligence/processors/verification => ../processors/verification
    github.com/database-intelligence/exporters/nri => ../exporters/nri
    github.com/database-intelligence/extensions/healthcheck => ../extensions/healthcheck
)
EOF

# Fix processor modules
echo -e "\n${BLUE}Fixing processor modules...${NC}"
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    if [ -d "$PROJECT_ROOT/processors/$processor" ]; then
        cd "$PROJECT_ROOT/processors/$processor"
        
        # Add replace directives if they reference common modules
        if grep -q "github.com/database-intelligence/common" go.mod 2>/dev/null; then
            cat >> go.mod << 'EOF'

replace (
    github.com/database-intelligence/common => ../../common
    github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
    github.com/database-intelligence/common/queryselector => ../../common/queryselector
)
EOF
            echo -e "${GREEN}[✓]${NC} Fixed $processor module"
        fi
    fi
done

# Fix distribution modules
echo -e "\n${BLUE}Fixing distribution modules...${NC}"
for dist in minimal standard enterprise; do
    if [ -d "$PROJECT_ROOT/distributions/$dist" ]; then
        cd "$PROJECT_ROOT/distributions/$dist"
        
        # Add comprehensive replace directives
        cat >> go.mod << 'EOF'

replace (
    github.com/database-intelligence/common => ../../common
    github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
    github.com/database-intelligence/common/queryselector => ../../common/queryselector
    github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
    github.com/database-intelligence/processors/verification => ../../processors/verification
    github.com/database-intelligence/exporters/nri => ../../exporters/nri
    github.com/database-intelligence/extensions/healthcheck => ../../extensions/healthcheck
)
EOF
        echo -e "${GREEN}[✓]${NC} Fixed $dist distribution"
    fi
done

# Fix test modules
echo -e "\n${BLUE}Fixing test modules...${NC}"
for testdir in tests tests/integration tests/e2e; do
    if [ -d "$PROJECT_ROOT/$testdir" ] && [ -f "$PROJECT_ROOT/$testdir/go.mod" ]; then
        cd "$PROJECT_ROOT/$testdir"
        
        # Calculate relative path to root
        RELATIVE_PATH=$(echo "$testdir" | sed 's/[^/]*/../g')
        
        cat >> go.mod << EOF

replace (
    github.com/database-intelligence/common => $RELATIVE_PATH/common
    github.com/database-intelligence/common/featuredetector => $RELATIVE_PATH/common/featuredetector
    github.com/database-intelligence/common/queryselector => $RELATIVE_PATH/common/queryselector
    github.com/database-intelligence/processors/adaptivesampler => $RELATIVE_PATH/processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => $RELATIVE_PATH/processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => $RELATIVE_PATH/processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => $RELATIVE_PATH/processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => $RELATIVE_PATH/processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => $RELATIVE_PATH/processors/querycorrelator
    github.com/database-intelligence/processors/verification => $RELATIVE_PATH/processors/verification
    github.com/database-intelligence/exporters/nri => $RELATIVE_PATH/exporters/nri
    github.com/database-intelligence/extensions/healthcheck => $RELATIVE_PATH/extensions/healthcheck
)
EOF
        echo -e "${GREEN}[✓]${NC} Fixed $testdir module"
    fi
done

# Now run go mod tidy on all modules
echo -e "\n${BLUE}Running go mod tidy on all modules...${NC}"
cd "$PROJECT_ROOT"

# Use go work sync to update dependencies
go work sync
echo -e "${GREEN}[✓]${NC} Synced workspace dependencies"

# Tidy individual modules
find . -name "go.mod" -not -path "./backup*" | while read -r modfile; do
    dir=$(dirname "$modfile")
    echo -e "${YELLOW}Tidying $dir...${NC}"
    cd "$PROJECT_ROOT/$dir"
    go mod tidy || echo -e "${YELLOW}[!] Warning: Failed to tidy $dir${NC}"
done

cd "$PROJECT_ROOT"

echo -e "\n${BLUE}=== Local Import Fix Complete ===${NC}"
echo -e "${GREEN}[✓]${NC} All modules now use local replace directives"
echo -e "${GREEN}[✓]${NC} Workspace file configured"
echo -e "${GREEN}[✓]${NC} Dependencies synchronized"