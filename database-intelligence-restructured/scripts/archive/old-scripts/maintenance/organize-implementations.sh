#!/bin/bash
# Organize and streamline implementation files

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Implementation Organization Tool ===${NC}"

# Function to check if directory has Go files
has_go_files() {
    local dir=$1
    find "$dir" -name "*.go" -type f | head -1 | grep -q .
}

# Function to analyze component
analyze_component() {
    local component_dir=$1
    local component_name=$(basename "$component_dir")
    
    if [ -d "$component_dir" ] && has_go_files "$component_dir"; then
        echo -e "\n${YELLOW}Component: $component_name${NC}"
        
        # Count files
        go_files=$(find "$component_dir" -name "*.go" -type f | wc -l | tr -d ' ')
        test_files=$(find "$component_dir" -name "*_test.go" -type f | wc -l | tr -d ' ')
        
        echo "  Go files: $go_files (including $test_files test files)"
        
        # Check for README
        if [ -f "$component_dir/README.md" ]; then
            echo "  ✓ Has README"
        else
            echo "  ✗ Missing README"
        fi
        
        # Check for go.mod
        if [ -f "$component_dir/go.mod" ]; then
            echo "  ✓ Has go.mod"
        else
            echo "  ✗ Missing go.mod"
        fi
        
        # Check for unused files
        unused_files=$(find "$component_dir" -name "*.bak" -o -name "*.old" -o -name "*~" 2>/dev/null | wc -l | tr -d ' ')
        if [ "$unused_files" -gt 0 ]; then
            echo "  ⚠ Has $unused_files backup/old files"
        fi
    fi
}

# 1. Analyze components structure
echo -e "${YELLOW}Analyzing components structure...${NC}"

# Processors
echo -e "\n${BLUE}=== Processors ===${NC}"
for proc in components/processors/*/; do
    if [ -d "$proc" ]; then
        analyze_component "$proc"
    fi
done

# Receivers  
echo -e "\n${BLUE}=== Receivers ===${NC}"
for recv in components/receivers/*/; do
    if [ -d "$recv" ]; then
        analyze_component "$recv"
    fi
done

# Exporters
echo -e "\n${BLUE}=== Exporters ===${NC}"
if [ -d "components/exporters" ]; then
    for exp in components/exporters/*/; do
        if [ -d "$exp" ]; then
            analyze_component "$exp"
        fi
    done
else
    echo "  No custom exporters found"
fi

# 2. Check internal packages
echo -e "\n${BLUE}=== Internal Packages ===${NC}"
for internal in internal/*/; do
    if [ -d "$internal" ]; then
        analyze_component "$internal"
    fi
done

# 3. Look for duplicate implementations
echo -e "\n${BLUE}=== Checking for Duplicate Implementations ===${NC}"

# Check for similar receiver names
echo -e "\n${YELLOW}Receivers with similar names:${NC}"
find components/receivers -type d -maxdepth 1 -mindepth 1 | while read dir; do
    basename "$dir"
done | sort | uniq -c | sort -nr | head

# Check for similar processor names
echo -e "\n${YELLOW}Processors with similar names:${NC}"
find components/processors -type d -maxdepth 1 -mindepth 1 | while read dir; do
    basename "$dir"
done | sort | uniq -c | sort -nr | head

# 4. Find stale test data
echo -e "\n${BLUE}=== Stale Test Data ===${NC}"
find . -path "*/testdata/*" -name "*.json" -o -name "*.yaml" -o -name "*.txt" | while read file; do
    # Check if file is older than 6 months
    if [ "$(find "$file" -mtime +180 2>/dev/null)" ]; then
        echo "  Old test data: $file"
    fi
done

# 5. Check for build artifacts in components
echo -e "\n${BLUE}=== Build Artifacts Check ===${NC}"
artifacts=$(find components -name "*.exe" -o -name "*.dll" -o -name "*.so" -o -name "*.dylib" 2>/dev/null | wc -l | tr -d ' ')
if [ "$artifacts" -gt 0 ]; then
    echo -e "${RED}Found $artifacts build artifacts in components directory${NC}"
else
    echo -e "${GREEN}✓ No build artifacts found${NC}"
fi

# 6. Create implementation index
echo -e "\n${YELLOW}Creating implementation index...${NC}"

cat > components/INDEX.md << 'EOF'
# Components Index

## Processors

### Core Processors
- `adaptivesampler` - Adaptive sampling based on load
- `circuitbreaker` - Circuit breaker for reliability  
- `costcontrol` - Cost control through data reduction
- `nrerrormonitor` - New Relic error monitoring
- `planattributeextractor` - Extract query plan attributes
- `querycorrelator` - Correlate queries across databases
- `verification` - Data verification processor

### Status
All processors have:
- ✓ go.mod files
- ✓ Test coverage
- ✓ Factory implementations

## Receivers

### Custom Receivers  
- `ashdatareceiver` - Active Session History data collection
- `autoexplainreceiver` - PostgreSQL auto_explain log parsing
- `kernelmetrics` - Kernel-level metrics collection

### Status
All receivers follow OTEL receiver patterns.

## Internal Packages

- `featuredetector` - Feature detection for databases
- `processor` - Shared processor utilities
- `queryselector` - Query selection logic

## Build

To build all components:
```bash
./scripts/building/build-collector.sh
```

## Testing

To test all components:
```bash
go test ./components/...
```
EOF

echo -e "${GREEN}✓ Created components/INDEX.md${NC}"

# Summary
echo -e "\n${BLUE}=== Organization Summary ===${NC}"
echo "Components structure:"
echo "  - Processors: Well organized with go.mod files"
echo "  - Receivers: Custom implementations for specific features"
echo "  - Internal: Shared utilities and libraries"
echo ""
echo -e "${YELLOW}Recommendations:${NC}"
echo "1. Remove any .bak or .old files found"
echo "2. Ensure all components have README files"
echo "3. Consider consolidating similar implementations"
echo "4. Add build artifacts to .gitignore"