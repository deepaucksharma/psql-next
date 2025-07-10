#!/bin/bash

# Fix Implementation Based on Clean Reference Comparison
# This script identifies and shows the fixes needed without making changes

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

echo -e "${BLUE}=== ANALYSIS AND FIX RECOMMENDATIONS ===${NC}"

# ==============================================================================
# ISSUE 1: Module Path Inconsistency
# ==============================================================================
echo -e "\n${CYAN}ISSUE 1: Module Path Inconsistency${NC}"
echo -e "${RED}Problem:${NC} Mixed module paths with and without '-restructured'"

echo -e "\n${YELLOW}Current state:${NC}"
grep -h "^module" core/go.mod distributions/production/go.mod processors/adaptivesampler/go.mod | head -5

echo -e "\n${GREEN}Fix needed:${NC}"
echo "All modules should use: github.com/database-intelligence/ (without -restructured)"
echo "This requires updating:"
echo "1. Module declarations in all go.mod files"
echo "2. Import statements in all .go files"
echo "3. Replace directives to use consistent paths"

# ==============================================================================
# ISSUE 2: Version Misalignment
# ==============================================================================
echo -e "\n${CYAN}ISSUE 2: Version Misalignment${NC}"
echo -e "${RED}Problem:${NC} Three different version sets in use"

echo -e "\n${YELLOW}Current versions:${NC}"
echo "Core module: v1.35.0 + v0.129.0"
echo "Production dist: v0.105.0"
echo "Processors: v0.110.0 + v1.16.0"

echo -e "\n${YELLOW}Clean reference pattern:${NC}"
echo "Latest stable: v1.35.0 for base packages (component, confmap, consumer, pdata)"
echo "             v0.129.0 for implementations (specific receivers, processors, etc.)"

echo -e "\n${GREEN}Recommended fix - Option C (Latest):${NC}"
echo "Update all modules to use:"
echo "- go.opentelemetry.io/collector/component v1.35.0"
echo "- go.opentelemetry.io/collector/confmap v1.35.0"
echo "- go.opentelemetry.io/collector/consumer v1.35.0"
echo "- go.opentelemetry.io/collector/pdata v1.35.0"
echo "- go.opentelemetry.io/collector/processor v1.35.0"
echo "- go.opentelemetry.io/collector/receiver v1.35.0"
echo "- go.opentelemetry.io/collector/extension v1.35.0"
echo "- go.opentelemetry.io/collector/exporter v1.35.0"
echo ""
echo "For specific implementations:"
echo "- go.opentelemetry.io/collector/processor/batchprocessor v0.129.0"
echo "- go.opentelemetry.io/collector/receiver/otlpreceiver v0.129.0"
echo "- github.com/open-telemetry/opentelemetry-collector-contrib/* v0.129.0"

# ==============================================================================
# ISSUE 3: Direct confmap Imports
# ==============================================================================
echo -e "\n${CYAN}ISSUE 3: Direct confmap Imports${NC}"
echo -e "${RED}Problem:${NC} Some modules may import confmap directly"

echo -e "\n${YELLOW}Checking for direct imports:${NC}"
# Check if any processors import confmap directly
for proc in processors/*; do
    if [ -d "$proc" ] && [ -f "$proc/go.mod" ]; then
        if grep -q "confmap" "$proc/go.mod" 2>/dev/null; then
            echo "$(basename $proc) imports confmap in go.mod"
        fi
    fi
done

echo -e "\n${GREEN}Fix needed:${NC}"
echo "Processors/receivers should not import confmap directly"
echo "Use component.Config interface instead"

# ==============================================================================
# ISSUE 4: Go Version in go.work
# ==============================================================================
echo -e "\n${CYAN}ISSUE 4: Go Version${NC}"
echo -e "${YELLOW}Current go.work version:${NC}"
head -1 go.work

echo -e "\n${GREEN}Fix needed:${NC}"
echo "Update go.work to use 'go 1.23' (not go 1.24.3)"
echo "Individual modules can specify toolchain go1.24.3"

# ==============================================================================
# Create Detailed Fix Plan
# ==============================================================================
echo -e "\n${CYAN}=== CREATING DETAILED FIX PLAN ===${NC}"

cat > fix-plan.md << 'EOF'
# Detailed Fix Plan

## Phase 1: Module Path Alignment
1. Update all go.mod files to use `github.com/database-intelligence/` (remove -restructured)
2. Update all import statements in .go files
3. Update replace directives

## Phase 2: Version Alignment (Option C - Latest)
### Base Packages (v1.35.0)
- component, confmap, consumer, pdata, processor, receiver, extension, exporter

### Implementation Packages (v0.129.0)
- Specific processors (batchprocessor, memorylimiterprocessor)
- Specific receivers (otlpreceiver, postgresqlreceiver)
- Contrib packages

### Update Order:
1. Common modules first
2. Processors/Receivers
3. Core module
4. Distributions

## Phase 3: Import Cleanup
1. Remove direct confmap imports from processors/receivers
2. Use component.Config interface
3. Let the framework handle configuration

## Phase 4: Testing
1. Build each module individually
2. Run unit tests
3. Build complete collector
4. Run E2E tests

## Version Mapping Table
| Package Type | Current | Target |
|-------------|---------|---------|
| component   | mixed   | v1.35.0 |
| confmap     | mixed   | v1.35.0 |
| pdata       | v1.16.0 | v1.35.0 |
| processor   | mixed   | v1.35.0 |
| batchprocessor | mixed | v0.129.0 |
| contrib     | mixed   | v0.129.0 |
EOF

echo -e "${GREEN}[✓]${NC} Fix plan created: fix-plan.md"

# ==============================================================================
# Create Version Update Script
# ==============================================================================
echo -e "\n${CYAN}Creating version update script...${NC}"

cat > apply-version-fixes.sh << 'SCRIPT'
#!/bin/bash

# Apply version fixes based on clean reference pattern
# THIS SCRIPT SHOWS WHAT WOULD BE CHANGED - RUN WITH 'apply' TO MAKE CHANGES

set -e

ACTION="${1:-show}"

if [ "$ACTION" = "show" ]; then
    echo "=== DRY RUN MODE ==="
    echo "This will show what changes would be made."
    echo "Run with './apply-version-fixes.sh apply' to make changes"
    echo ""
fi

# Function to update a module
update_module() {
    local module_path=$1
    local module_name=$(basename "$module_path")
    
    echo "Module: $module_name"
    
    if [ "$ACTION" = "apply" ]; then
        cd "$module_path"
        
        # Update to v1.35.0 pattern
        go get -u go.opentelemetry.io/collector/component@v1.35.0
        go get -u go.opentelemetry.io/collector/confmap@v1.35.0
        go get -u go.opentelemetry.io/collector/consumer@v1.35.0
        go get -u go.opentelemetry.io/collector/pdata@v1.35.0
        
        # Update specific components based on module type
        if [[ "$module_path" == *"processor"* ]]; then
            go get -u go.opentelemetry.io/collector/processor@v1.35.0
        elif [[ "$module_path" == *"receiver"* ]]; then
            go get -u go.opentelemetry.io/collector/receiver@v1.35.0
        elif [[ "$module_path" == *"exporter"* ]]; then
            go get -u go.opentelemetry.io/collector/exporter@v1.35.0
        fi
        
        go mod tidy
        cd - > /dev/null
        
        echo "  ✓ Updated"
    else
        echo "  Would update to v1.35.0/v0.129.0 pattern"
    fi
    echo ""
}

# Update processors
echo "=== PROCESSORS ==="
for proc in processors/*; do
    if [ -d "$proc" ] && [ -f "$proc/go.mod" ]; then
        update_module "$proc"
    fi
done

# Update receivers
echo "=== RECEIVERS ==="
for recv in receivers/*; do
    if [ -d "$recv" ] && [ -f "$recv/go.mod" ]; then
        update_module "$recv"
    fi
done

# Update common
echo "=== COMMON MODULES ==="
for common in common common/featuredetector common/queryselector; do
    if [ -d "$common" ] && [ -f "$common/go.mod" ]; then
        update_module "$common"
    fi
done

if [ "$ACTION" = "show" ]; then
    echo "To apply these changes, run: ./apply-version-fixes.sh apply"
fi
SCRIPT

chmod +x apply-version-fixes.sh

echo -e "${GREEN}[✓]${NC} Created apply-version-fixes.sh"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== SUMMARY ===${NC}"

echo -e "\n${YELLOW}Key Issues Identified:${NC}"
echo "1. ❌ Module paths inconsistent (with/without -restructured)"
echo "2. ❌ Three different version sets (v0.105.0, v0.110.0, v1.35.0)"
echo "3. ❌ Version mismatch between core and other modules"
echo "4. ⚠️  Possible direct confmap imports"

echo -e "\n${YELLOW}Recommended Solution:${NC}"
echo "1. Align all module paths (remove -restructured)"
echo "2. Update to latest versions (v1.35.0 + v0.129.0 pattern)"
echo "3. Remove direct confmap imports"
echo "4. Test incrementally"

echo -e "\n${YELLOW}Next Steps:${NC}"
echo "1. Review fix-plan.md for detailed plan"
echo "2. Run ./apply-version-fixes.sh to see what would change"
echo "3. Run ./apply-version-fixes.sh apply to make changes"
echo "4. Update module paths manually (search/replace)"

echo -e "\n${GREEN}Analysis complete!${NC}"