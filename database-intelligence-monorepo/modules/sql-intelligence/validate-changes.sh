#!/bin/bash

# SQL Intelligence Module - Change Validation Script
# This script validates that all required changes have been implemented

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
MODULE_DIR="$SCRIPT_DIR"
CONFIG_DIR="$MODULE_DIR/config"

echo "=========================================="
echo "SQL Intelligence Module Validation"
echo "=========================================="

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Validation counters
PASS=0
FAIL=0
WARN=0

# Function to check condition
check() {
    local description="$1"
    local condition="$2"
    
    echo -n "Checking: $description... "
    
    if eval "$condition"; then
        echo -e "${GREEN}PASS${NC}"
        ((PASS++))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        ((FAIL++))
        return 1
    fi
}

# Function to warn about condition
warn() {
    local description="$1"
    local condition="$2"
    
    echo -n "Warning: $description... "
    
    if eval "$condition"; then
        echo -e "${YELLOW}WARN${NC}"
        ((WARN++))
    else
        echo -e "${GREEN}OK${NC}"
    fi
}

echo ""
echo "1. Configuration Cleanup Validation"
echo "-----------------------------------"

# Check for backup files
warn "Backup files still exist" "ls $CONFIG_DIR/*.backup-* 2>/dev/null | grep -q backup"

# Check for redundant configs
check "No collector-enhanced.yaml" "! test -f $CONFIG_DIR/collector-enhanced.yaml"
check "No collector-enterprise.yaml" "! test -f $CONFIG_DIR/collector-enterprise.yaml"
check "No collector-enterprise-working.yaml" "! test -f $CONFIG_DIR/collector-enterprise-working.yaml"

# Check for main config
check "collector.yaml exists" "test -f $CONFIG_DIR/collector.yaml"

echo ""
echo "2. Architecture Validation"
echo "--------------------------"

# Check for single pipeline
if [ -f "$CONFIG_DIR/collector.yaml" ]; then
    check "No duplicate metrics/standard pipeline" \
        "! grep -q 'metrics/standard:' $CONFIG_DIR/collector.yaml"
    
    check "No duplicate metrics/critical pipeline" \
        "! grep -q 'metrics/critical:' $CONFIG_DIR/collector.yaml"
    
    check "Single metrics pipeline exists" \
        "grep -q 'metrics:' $CONFIG_DIR/collector.yaml"
    
    check "Routing processor configured" \
        "grep -q 'routing' $CONFIG_DIR/collector.yaml"
fi

echo ""
echo "3. Intelligence Features Validation"
echo "-----------------------------------"

if [ -f "$CONFIG_DIR/collector.yaml" ]; then
    check "Query intelligence transform exists" \
        "grep -q 'transform/query_intelligence' $CONFIG_DIR/collector.yaml || grep -q 'transform/query_analysis' $CONFIG_DIR/collector.yaml"
    
    check "Index efficiency scoring logic" \
        "grep -q 'index_efficiency' $CONFIG_DIR/collector.yaml || grep -q 'query_efficiency' $CONFIG_DIR/collector.yaml"
    
    check "Impact scoring implementation" \
        "grep -q 'impact_score\|severity' $CONFIG_DIR/collector.yaml"
    
    check "Recommendations processor" \
        "grep -q 'recommendation' $CONFIG_DIR/collector.yaml || grep -q 'needs_index' $CONFIG_DIR/collector.yaml"
fi

echo ""
echo "4. Metric Naming Validation"
echo "---------------------------"

if [ -f "$CONFIG_DIR/collector.yaml" ]; then
    # Check for old naming patterns that should be replaced
    warn "Old metric naming pattern (exec_total)" \
        "grep -q 'mysql\.query\.exec_total' $CONFIG_DIR/collector.yaml"
    
    warn "Inconsistent unit naming (_ms suffix)" \
        "grep -q '_ms\"' $CONFIG_DIR/collector.yaml"
    
    # Check for standardized patterns
    check "Metrictransform processor configured" \
        "grep -q 'metrictransform' $CONFIG_DIR/collector.yaml || grep -q 'consistent naming' $CONFIG_DIR/collector.yaml"
fi

echo ""
echo "5. Entity Synthesis Validation"
echo "------------------------------"

if [ -f "$CONFIG_DIR/collector.yaml" ]; then
    check "Dynamic entity GUID generation" \
        "grep -q 'Concat.*entity\.guid' $CONFIG_DIR/collector.yaml || ! grep -q 'MYSQL|\${env:CLUSTER_NAME}|\${env:MYSQL_ENDPOINT}' $CONFIG_DIR/collector.yaml"
    
    check "Entity type is MYSQL_QUERY_INTELLIGENCE or similar" \
        "grep -q 'MYSQL.*INTELLIGENCE\|MYSQL_INSTANCE' $CONFIG_DIR/collector.yaml"
fi

echo ""
echo "6. Query Optimization Validation"
echo "--------------------------------"

if [ -f "$CONFIG_DIR/collector.yaml" ]; then
    check "Impact-based query filtering" \
        "grep -q 'SUM_TIMER_WAIT.*>' $CONFIG_DIR/collector.yaml || grep -q 'WHERE.*TIMER.*>' $CONFIG_DIR/collector.yaml"
    
    warn "Still using arbitrary LIMIT 20" \
        "grep -q 'LIMIT 20' $CONFIG_DIR/collector.yaml"
fi

echo ""
echo "7. Docker Configuration Validation"
echo "----------------------------------"

if [ -f "$MODULE_DIR/docker-compose.yaml" ]; then
    check "Default config is collector.yaml" \
        "grep -q 'collector\.yaml' $MODULE_DIR/docker-compose.yaml || ! grep -q 'COLLECTOR_CONFIG' $MODULE_DIR/docker-compose.yaml"
    
    warn "init.sql still commented out" \
        "grep -q '#.*init\.sql' $MODULE_DIR/docker-compose.yaml"
fi

echo ""
echo "8. Testing Validation"
echo "--------------------"

if [ -f "$MODULE_DIR/Makefile" ]; then
    check "Integration test exists" \
        "grep -q 'test-integration:' $MODULE_DIR/Makefile"
    
    # Check if integration test generates load
    check "Integration test generates query load" \
        "grep -A10 'test-integration:' $MODULE_DIR/Makefile | grep -q 'mysql.*-e\|INSERT\|SELECT'"
fi

echo ""
echo "=========================================="
echo "Validation Summary"
echo "=========================================="
echo -e "Passed: ${GREEN}$PASS${NC}"
echo -e "Failed: ${RED}$FAIL${NC}"
echo -e "Warnings: ${YELLOW}$WARN${NC}"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}✓ All critical validations passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Critical validations failed. Please review the required changes.${NC}"
    exit 1
fi