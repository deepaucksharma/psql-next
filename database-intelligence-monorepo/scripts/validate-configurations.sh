#!/bin/bash

# Configuration Validation Script for Database Intelligence Modules
# This script validates OTTL syntax and configuration correctness

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

# Track results
TOTAL_MODULES=0
PASSED_MODULES=0
FAILED_MODULES=0
TOTAL_WARNINGS=0

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}Database Intelligence Configuration Validator${NC}"
echo -e "${BLUE}============================================${NC}"

# Function to validate a single module
validate_module() {
    local module_name=$1
    local module_path="$MODULES_DIR/$module_name"
    
    echo -e "\n${YELLOW}Validating module: $module_name${NC}"
    echo "----------------------------------------"
    
    if [ ! -d "$module_path" ]; then
        echo -e "${RED}✗ Module directory not found${NC}"
        return 1
    fi
    
    local errors=0
    local warnings=0
    
    # Check for required files
    echo "  Checking required files..."
    for file in "Dockerfile" "docker-compose.yaml" "config/collector.yaml"; do
        if [ ! -f "$module_path/$file" ]; then
            echo -e "    ${RED}✗ Missing: $file${NC}"
            ((errors++))
        else
            echo -e "    ${GREEN}✓ Found: $file${NC}"
        fi
    done
    
    # Validate collector configuration
    if [ -f "$module_path/config/collector.yaml" ]; then
        echo -e "\n  Validating OTTL syntax..."
        
        # Check for metric.name in datapoint context
        if grep -B2 -A2 "context: datapoint" "$module_path/config/collector.yaml" | grep -q "metric\.name"; then
            echo -e "    ${RED}✗ OTTL error: metric.name used in datapoint context${NC}"
            ((errors++))
        fi
        
        # Check for metric.unit/description in datapoint context
        if grep -B2 -A2 "context: datapoint" "$module_path/config/collector.yaml" | grep -q "metric\.\(unit\|description\)"; then
            echo -e "    ${RED}✗ OTTL error: metric properties used in datapoint context${NC}"
            ((errors++))
        fi
        
        # Check for datapoint.value in datapoint context (should be just 'value')
        if grep -q "datapoint\.value" "$module_path/config/collector.yaml"; then
            echo -e "    ${RED}✗ OTTL error: use 'value' instead of 'datapoint.value' in datapoint context${NC}"
            ((errors++))
        fi
        
        # Check for context: scope (should be context: metric for most cases)
        if grep -q "context: scope" "$module_path/config/collector.yaml"; then
            echo -e "    ${YELLOW}⚠ Warning: 'context: scope' found - verify if 'context: metric' is more appropriate${NC}"
            ((warnings++))
        fi
        
        echo -e "\n  Checking configuration best practices..."
        
        # Check for health check removal
        if grep -q "health_check" "$module_path/config/collector.yaml" | grep -v "WARNING" | grep -v "#" | grep -v "DO NOT ADD"; then
            echo -e "    ${YELLOW}⚠ Warning: health_check found in production config${NC}"
            ((warnings++))
        fi
        
        # Check for telemetry.metrics.address (invalid in newer versions)
        if grep -q "telemetry:" -A5 "$module_path/config/collector.yaml" | grep -q "metrics:" -A3 | grep -q "address:"; then
            echo -e "    ${RED}✗ Invalid telemetry.metrics.address configuration${NC}"
            ((errors++))
        fi
        
        # Check for proper New Relic configuration
        if ! grep -q "NEW_RELIC_OTLP_ENDPOINT" "$module_path/config/collector.yaml"; then
            echo -e "    ${YELLOW}⚠ Warning: No New Relic OTLP endpoint configured${NC}"
            ((warnings++))
        fi
        
        # Check for filter syntax with 'filter:' key in attributes processor
        if grep -q "filter:" "$module_path/config/collector.yaml" | grep -B5 "attributes/" | grep -q "filter:"; then
            echo -e "    ${RED}✗ Invalid 'filter:' key in attributes processor${NC}"
            ((errors++))
        fi
    fi
    
    # Check docker-compose.yaml
    echo -e "\n  Validating docker-compose.yaml..."
    
    # Check for collector-enterprise-working.yaml references
    if grep -q "collector-enterprise-working.yaml" "$module_path/docker-compose.yaml"; then
        echo -e "    ${RED}✗ References non-existent collector-enterprise-working.yaml${NC}"
        echo -e "    ${YELLOW}  → Should use collector.yaml or appropriate config file${NC}"
        ((errors++))
    fi
    
    # Check for health check warnings in docker-compose
    if grep -q "healthcheck:" "$module_path/docker-compose.yaml" | grep -B5 -A5 "healthcheck:" | grep -q "WARNING"; then
        echo -e "    ${GREEN}✓ Health check warnings present in docker-compose${NC}"
    fi
    
    # Test Docker build (quick syntax check only)
    echo -e "\n  Testing Docker configuration..."
    if cd "$module_path" && docker-compose config > /dev/null 2>&1; then
        echo -e "    ${GREEN}✓ docker-compose syntax valid${NC}"
    else
        echo -e "    ${RED}✗ docker-compose syntax invalid${NC}"
        ((errors++))
    fi
    
    # Summary for module
    TOTAL_WARNINGS=$((TOTAL_WARNINGS + warnings))
    
    if [ $errors -eq 0 ]; then
        echo -e "\n  ${GREEN}✓ Module validation PASSED${NC}"
        [ $warnings -gt 0 ] && echo -e "    ${YELLOW}with $warnings warnings${NC}"
        return 0
    else
        echo -e "\n  ${RED}✗ Module validation FAILED with $errors errors${NC}"
        [ $warnings -gt 0 ] && echo -e "    ${YELLOW}and $warnings warnings${NC}"
        return 1
    fi
}

# List of all modules
MODULES=(
    "core-metrics"
    "sql-intelligence"
    "wait-profiler"
    "anomaly-detector"
    "business-impact"
    "replication-monitor"
    "performance-advisor"
    "resource-monitor"
    "alert-manager"
    "canary-tester"
    "cross-signal-correlator"
)

# Validate each module
for module in "${MODULES[@]}"; do
    ((TOTAL_MODULES++))
    if validate_module "$module"; then
        ((PASSED_MODULES++))
    else
        ((FAILED_MODULES++))
    fi
done

# Overall summary
echo -e "\n${BLUE}============================================${NC}"
echo -e "${BLUE}Validation Summary${NC}"
echo -e "${BLUE}============================================${NC}"
echo -e "Total modules:    $TOTAL_MODULES"
echo -e "${GREEN}Passed:          $PASSED_MODULES${NC}"
echo -e "${RED}Failed:          $FAILED_MODULES${NC}"
echo -e "${YELLOW}Total warnings:  $TOTAL_WARNINGS${NC}"

# Provide recommendations
echo -e "\n${BLUE}Common Issues Found:${NC}"
echo "1. OTTL Context Mismatches:"
echo "   - Use 'value' not 'datapoint.value' in datapoint context"
echo "   - Use 'name' not 'metric.name' in metric context"
echo "   - metric.name, metric.unit, metric.description only available in metric context"
echo ""
echo "2. Configuration Issues:"
echo "   - Remove telemetry.metrics.address (use telemetry.metrics.level instead)"
echo "   - Use collector.yaml not collector-enterprise-working.yaml"
echo "   - Remove health_check from production configs"
echo ""
echo "3. Best Practices:"
echo "   - Use 'context: metric' for metric-level transformations"
echo "   - Use 'context: datapoint' only for value-based operations"
echo "   - Always include New Relic OTLP configuration"

if [ $FAILED_MODULES -eq 0 ]; then
    echo -e "\n${GREEN}✓ All modules validated successfully!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Validation failed for $FAILED_MODULES modules${NC}"
    echo -e "\nRun individual module fixes or use the automated fix script"
    exit 1
fi