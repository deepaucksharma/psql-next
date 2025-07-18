#!/bin/bash
# Unified validation script that runs all validation checks

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATION_DIR="$SCRIPT_DIR/validation"

echo -e "${BLUE}=== Database Intelligence Unified Validation ===${NC}"
echo -e "Running all validation checks...\n"

# Track overall results
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

# Function to run a validation script
run_validation() {
    local script_name=$1
    local script_path="$VALIDATION_DIR/$script_name"
    local args="${2:-}"
    
    if [ ! -f "$script_path" ]; then
        echo -e "${RED}✗ Script not found: $script_name${NC}"
        ((FAILED_CHECKS++))
        ((TOTAL_CHECKS++))
        return 1
    fi
    
    echo -e "${BLUE}Running $script_name...${NC}"
    ((TOTAL_CHECKS++))
    
    # Handle scripts that need arguments
    case "$script_name" in
        "validate-config.sh")
            # Test with PostgreSQL config as default
            args="$SCRIPT_DIR/../configs/postgresql-maximum-extraction.yaml"
            ;;
        "validate-metrics.sh")
            # Skip if env vars not set
            if [ -z "${NEW_RELIC_ACCOUNT_ID:-}" ] || [ -z "${NEW_RELIC_API_KEY:-}" ]; then
                echo -e "${YELLOW}⚠ Skipping (requires NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY)${NC}"
                ((TOTAL_CHECKS--))  # Don't count as failure
                return 0
            fi
            ;;
    esac
    
    if bash "$script_path" $args > /tmp/validation_output_$$ 2>&1; then
        echo -e "${GREEN}✓ $script_name passed${NC}"
        ((PASSED_CHECKS++))
        rm -f /tmp/validation_output_$$
        return 0
    else
        echo -e "${RED}✗ $script_name failed${NC}"
        echo -e "${YELLOW}Error output:${NC}"
        tail -20 /tmp/validation_output_$$
        ((FAILED_CHECKS++))
        rm -f /tmp/validation_output_$$
        return 1
    fi
}

# Run all validations
echo -e "${YELLOW}[1/4] Configuration Validation${NC}"
run_validation "validate-config.sh" || true

echo -e "\n${YELLOW}[2/4] Metrics Validation${NC}"
run_validation "validate-metrics.sh" || true

echo -e "\n${YELLOW}[3/4] Metric Naming Conventions${NC}"
run_validation "validate-metric-naming.sh" || true

echo -e "\n${YELLOW}[4/4] End-to-End Validation${NC}"
run_validation "validate-e2e.sh" || true

# Summary
echo -e "\n${BLUE}=== Validation Summary ===${NC}"
echo -e "Total Checks: ${TOTAL_CHECKS}"
echo -e "Passed: ${GREEN}${PASSED_CHECKS}${NC}"
echo -e "Failed: ${RED}${FAILED_CHECKS}${NC}"

if [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "\n${GREEN}✓ All validations passed!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some validations failed!${NC}"
    echo -e "${YELLOW}Run individual validation scripts for detailed output.${NC}"
    exit 1
fi