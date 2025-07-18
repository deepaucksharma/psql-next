#!/bin/bash
# Common test utilities

# Colors
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export RED='\033[0;31m'
export BLUE='\033[0;34m'
export NC='\033[0m'

# Test assertion functions
assert_equals() {
    local expected=$1
    local actual=$2
    local message=${3:-"Assertion failed"}
    
    if [ "$expected" = "$actual" ]; then
        echo -e "${GREEN}✓ $message${NC}"
        return 0
    else
        echo -e "${RED}✗ $message${NC}"
        echo -e "  Expected: $expected"
        echo -e "  Actual: $actual"
        return 1
    fi
}

assert_file_exists() {
    local file=$1
    local message=${2:-"File should exist: $file"}
    
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓ $message${NC}"
        return 0
    else
        echo -e "${RED}✗ $message${NC}"
        return 1
    fi
}

assert_contains() {
    local file=$1
    local pattern=$2
    local message=${3:-"File should contain: $pattern"}
    
    if grep -q "$pattern" "$file"; then
        echo -e "${GREEN}✓ $message${NC}"
        return 0
    else
        echo -e "${RED}✗ $message${NC}"
        return 1
    fi
}

# Docker utilities
wait_for_container() {
    local container=$1
    local timeout=${2:-30}
    local elapsed=0
    
    echo -n "Waiting for container $container..."
    while ! docker ps | grep -q "$container"; do
        if [ $elapsed -ge $timeout ]; then
            echo -e " ${RED}timeout!${NC}"
            return 1
        fi
        echo -n "."
        sleep 1
        ((elapsed++))
    done
    echo -e " ${GREEN}ready!${NC}"
    return 0
}

# Metric validation
check_metrics_endpoint() {
    local endpoint=$1
    local metric_prefix=$2
    
    local response=$(curl -s "$endpoint/metrics")
    if echo "$response" | grep -q "$metric_prefix"; then
        echo -e "${GREEN}✓ Metrics found for $metric_prefix${NC}"
        return 0
    else
        echo -e "${RED}✗ No metrics found for $metric_prefix${NC}"
        return 1
    fi
}
