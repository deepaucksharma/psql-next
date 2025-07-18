#!/bin/bash
# Script to validate metric naming conventions across all database configurations

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Validating Metric Naming Conventions ===${NC}"

# Function to check metric names in a config file
check_metrics() {
    local file=$1
    local db_type=$2
    local pattern=""
    
    # Set pattern based on database type
    case "$db_type" in
        "postgresql")
            pattern="^(postgresql|pg|db)\."
            ;;
        "mysql")
            pattern="^mysql\."
            ;;
        "mongodb")
            pattern="^(mongodb|mongodbatlas)\."
            ;;
        "mssql")
            pattern="^(mssql|sqlserver)\."
            ;;
        "oracle")
            pattern="^oracle\."
            ;;
    esac
    
    echo -e "${YELLOW}Checking $db_type metrics in $(basename $file)...${NC}"
    
    # Extract metric names from the file
    local metrics=$(grep -E "metric_name:" "$file" | sed 's/.*metric_name: *//' | sed 's/"//g' | sort | uniq)
    
    local total=0
    local valid=0
    local invalid=0
    
    while IFS= read -r metric; do
        if [ -n "$metric" ]; then
            ((total++))
            if echo "$metric" | grep -qE "$pattern"; then
                ((valid++))
            else
                ((invalid++))
                echo -e "  ${RED}✗ Invalid metric name: $metric${NC}"
            fi
        fi
    done <<< "$metrics"
    
    if [ $invalid -eq 0 ]; then
        echo -e "  ${GREEN}✓ All $total metrics follow naming convention${NC}"
    else
        echo -e "  ${RED}✗ $invalid/$total metrics violate naming convention${NC}"
    fi
    
    return $invalid
}

# Check each database configuration
TOTAL_ERRORS=0

for db in postgresql mysql mongodb mssql oracle; do
    config_file="configs/${db}-maximum-extraction.yaml"
    if [ -f "$config_file" ]; then
        check_metrics "$config_file" "$db" || ((TOTAL_ERRORS+=$?))
    else
        echo -e "${RED}Warning: $config_file not found${NC}"
    fi
done

# Summary
echo ""
echo -e "${BLUE}=== Metric Naming Validation Summary ===${NC}"
if [ $TOTAL_ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ All metrics follow naming conventions${NC}"
    exit 0
else
    echo -e "${RED}✗ Found $TOTAL_ERRORS metric naming issues${NC}"
    exit 1
fi