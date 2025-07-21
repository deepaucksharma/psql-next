#!/bin/bash

# Automated Fix Script for Common OpenTelemetry Configuration Issues
# This script automatically fixes common OTTL and configuration issues

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MODULES_DIR="$PROJECT_ROOT/modules"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Automated Configuration Fix Script${NC}"
echo -e "${BLUE}========================================${NC}"

# Track fixes
TOTAL_FIXES=0

# Function to fix OTTL context issues
fix_ottl_context() {
    local file=$1
    local module_name=$(basename $(dirname $(dirname "$file")))
    
    echo -e "\n${YELLOW}Checking OTTL in $module_name...${NC}"
    
    # Fix metric.name in datapoint context
    if grep -q "context: datapoint" "$file" && grep -A10 "context: datapoint" "$file" | grep -q "metric\.name"; then
        echo -e "  ${GREEN}✓${NC} Fixing metric.name in datapoint context"
        sed -i.bak '/context: datapoint/,/context: metric\|processors:\|exporters:/{
            s/where metric\.name == /where name == /g
            s/metric\.name/name/g
        }' "$file"
        ((TOTAL_FIXES++))
    fi
    
    # Fix metric.value in datapoint context (should be just 'value')
    if grep -q "metric\.value" "$file"; then
        echo -e "  ${GREEN}✓${NC} Fixing metric.value to value"
        sed -i.bak 's/metric\.value/value/g' "$file"
        ((TOTAL_FIXES++))
    fi
    
    # Fix datapoint.value in datapoint context (should be just 'value')
    if grep -q "datapoint\.value" "$file"; then
        echo -e "  ${GREEN}✓${NC} Fixing datapoint.value to value"
        sed -i.bak 's/datapoint\.value/value/g' "$file"
        ((TOTAL_FIXES++))
    fi
    
    # Fix context: scope to context: metric
    if grep -q "context: scope" "$file"; then
        echo -e "  ${GREEN}✓${NC} Changing context: scope to context: metric"
        sed -i.bak 's/context: scope/context: metric/g' "$file"
        ((TOTAL_FIXES++))
    fi
}

# Function to fix telemetry configuration
fix_telemetry_config() {
    local file=$1
    local module_name=$(basename $(dirname $(dirname "$file")))
    
    if grep -A10 "^telemetry:" "$file" | grep -A5 "metrics:" | grep -q "address:"; then
        echo -e "\n${YELLOW}Fixing telemetry config in $module_name...${NC}"
        echo -e "  ${GREEN}✓${NC} Removing invalid telemetry.metrics.address"
        
        # Remove the entire metrics section under telemetry
        perl -i.bak -0pe 's/telemetry:\s*\n(\s+)metrics:\s*\n\s+address:.*?\n(?=\s*\w|\z)/telemetry:\n/gms' "$file"
        ((TOTAL_FIXES++))
    fi
}

# Function to fix docker-compose references
fix_docker_compose() {
    local file=$1
    local module_name=$(basename $(dirname "$file")))
    
    if grep -q "collector-enterprise-working.yaml" "$file"; then
        echo -e "\n${YELLOW}Fixing docker-compose in $module_name...${NC}"
        echo -e "  ${GREEN}✓${NC} Updating collector config reference"
        sed -i.bak 's/collector-enterprise-working.yaml/collector.yaml/g' "$file"
        ((TOTAL_FIXES++))
    fi
}

# Function to fix attributes processor filters
fix_attributes_filters() {
    local file=$1
    local module_name=$(basename $(dirname $(dirname "$file")))
    
    if grep -B5 "attributes/" "$file" | grep -A10 "actions:" | grep -q "filter:"; then
        echo -e "\n${YELLOW}Fixing attributes processor in $module_name...${NC}"
        echo -e "  ${GREEN}✓${NC} Removing invalid filter keys"
        
        # Remove filter: lines within attributes processor actions
        perl -i.bak -0pe 's/(attributes[^:]*:\s*\n\s*actions:\s*\n(?:\s*-[^\n]*\n)*?)(\s*filter:[^\n]*\n)/$1/gms' "$file"
        ((TOTAL_FIXES++))
    fi
}

# Process all modules
echo -e "\n${BLUE}Processing all modules...${NC}"

for module_dir in "$MODULES_DIR"/*; do
    if [ -d "$module_dir" ]; then
        module_name=$(basename "$module_dir")
        
        # Check collector configs
        for config in "$module_dir/config/collector"*.yaml; do
            if [ -f "$config" ]; then
                fix_ottl_context "$config"
                fix_telemetry_config "$config"
                fix_attributes_filters "$config"
            fi
        done
        
        # Check docker-compose
        if [ -f "$module_dir/docker-compose.yaml" ]; then
            fix_docker_compose "$module_dir/docker-compose.yaml"
        fi
    fi
done

# Clean up backup files
echo -e "\n${BLUE}Cleaning up backup files...${NC}"
find "$MODULES_DIR" -name "*.bak" -delete

# Summary
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}Fix Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "Total fixes applied: ${GREEN}$TOTAL_FIXES${NC}"

if [ $TOTAL_FIXES -gt 0 ]; then
    echo -e "\n${GREEN}✓ Configuration issues have been fixed!${NC}"
    echo -e "\nNext steps:"
    echo -e "1. Run validation script: ./scripts/validate-configurations.sh"
    echo -e "2. Test modules: make test"
    echo -e "3. Commit changes: git add -A && git commit -m 'Fix OTTL and configuration issues'"
else
    echo -e "\n${GREEN}✓ No issues found - configurations are clean!${NC}"
fi