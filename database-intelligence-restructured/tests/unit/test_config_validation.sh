#!/bin/bash
# Unit test for configuration validation

# Source test utilities
source "$(dirname "$0")/../utils/common.sh"

echo -e "${BLUE}=== Configuration Validation Tests ===${NC}"

# Test 1: Valid configuration should pass
echo -e "\n${YELLOW}Test 1: Valid configuration${NC}"
assert_file_exists "configs/postgresql-maximum-extraction.yaml" \
    "PostgreSQL config should exist"

# Test 2: Configuration should have required sections
echo -e "\n${YELLOW}Test 2: Required sections${NC}"
assert_contains "configs/postgresql-maximum-extraction.yaml" "receivers:" \
    "Config should have receivers section"
assert_contains "configs/postgresql-maximum-extraction.yaml" "exporters:" \
    "Config should have exporters section"
assert_contains "configs/postgresql-maximum-extraction.yaml" "service:" \
    "Config should have service section"

# Test 3: Environment variables should be used
echo -e "\n${YELLOW}Test 3: Environment variables${NC}"
assert_contains "configs/postgresql-maximum-extraction.yaml" '${env:' \
    "Config should use environment variables"

echo -e "\n${GREEN}✓ All configuration tests passed${NC}"
