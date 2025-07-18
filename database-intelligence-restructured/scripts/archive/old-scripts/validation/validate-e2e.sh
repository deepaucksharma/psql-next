#!/bin/bash
# Comprehensive end-to-end validation script

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Database Intelligence E2E Validation ===${NC}"

# Track overall status
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

# Function to run a check
run_check() {
    local check_name=$1
    local check_command=$2
    
    ((TOTAL_CHECKS++))
    echo -n -e "${YELLOW}Checking $check_name... ${NC}"
    
    if eval "$check_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC}"
        ((PASSED_CHECKS++))
        return 0
    else
        echo -e "${RED}✗${NC}"
        ((FAILED_CHECKS++))
        return 1
    fi
}

# 1. Check directory structure
echo -e "\n${BLUE}[1/8] Directory Structure${NC}"
run_check "configs directory" "[ -d configs ]"
run_check "scripts directory" "[ -d scripts ]"
run_check "docs directory" "[ -d docs ]"
run_check "env templates" "[ -d configs/env-templates ]"

# 2. Check configuration files
echo -e "\n${BLUE}[2/8] Configuration Files${NC}"
for db in postgresql mysql mongodb mssql oracle; do
    run_check "$db config" "[ -f configs/${db}-maximum-extraction.yaml ]"
    run_check "$db env template" "[ -f configs/env-templates/${db}.env ]"
done

# 3. Check documentation
echo -e "\n${BLUE}[3/8] Documentation${NC}"
run_check "Main README" "[ -f README.md ]"
run_check "Deployment guide" "[ -f docs/guides/DEPLOYMENT.md ]"
run_check "Troubleshooting guide" "[ -f docs/guides/TROUBLESHOOTING.md ]"
run_check "Config guide" "[ -f docs/guides/CONFIG_ONLY_MAXIMUM_GUIDE.md ]"
for db in MYSQL MONGODB MSSQL ORACLE; do
    run_check "$db guide" "[ -f docs/guides/${db}_MAXIMUM_GUIDE.md ]"
done

# 4. Check scripts
echo -e "\n${BLUE}[4/8] Scripts${NC}"
run_check "validate-config.sh" "[ -x scripts/validate-config.sh ]"
run_check "validate-metrics.sh" "[ -x scripts/validate-metrics.sh ]"
run_check "test-database-config.sh" "[ -x scripts/test-database-config.sh ]"
run_check "start-all-databases.sh" "[ -x scripts/start-all-databases.sh ]"
run_check "stop-all-databases.sh" "[ -x scripts/stop-all-databases.sh ]"

# 5. Check Docker files
echo -e "\n${BLUE}[5/8] Docker Configuration${NC}"
run_check "docker-compose file" "[ -f docker-compose.databases.yml ]"

# 6. Validate YAML syntax
echo -e "\n${BLUE}[6/8] YAML Syntax Validation${NC}"
if command -v yq &> /dev/null; then
    for db in postgresql mysql mongodb mssql oracle; do
        run_check "$db YAML syntax" "yq eval '.' configs/${db}-maximum-extraction.yaml"
    done
else
    echo -e "${YELLOW}Warning: yq not found, skipping YAML validation${NC}"
fi

# 7. Check metric naming conventions
echo -e "\n${BLUE}[7/8] Metric Naming Conventions${NC}"
if [ -x scripts/validate-metric-naming.sh ]; then
    if ./scripts/validate-metric-naming.sh > /dev/null 2>&1; then
        echo -e "${GREEN}✓ All metrics follow naming conventions${NC}"
        ((PASSED_CHECKS++))
    else
        echo -e "${RED}✗ Some metrics violate naming conventions${NC}"
        ((FAILED_CHECKS++))
    fi
    ((TOTAL_CHECKS++))
fi

# 8. Check for common issues
echo -e "\n${BLUE}[8/8] Common Issues${NC}"
# Check for deployment.mode consistency
DEPLOYMENT_MODES=$(grep -h "deployment.mode" configs/*-maximum-extraction.yaml 2>/dev/null | grep -v "key:" | sort | uniq | wc -l)
if [ "$DEPLOYMENT_MODES" -eq 1 ]; then
    echo -e "${GREEN}✓ Deployment mode is consistent${NC}"
    ((PASSED_CHECKS++))
else
    echo -e "${RED}✗ Inconsistent deployment modes found${NC}"
    ((FAILED_CHECKS++))
fi
((TOTAL_CHECKS++))

# Check for consistent prometheus namespaces
run_check "Prometheus namespaces" "grep -q 'namespace: db_' configs/*-maximum-extraction.yaml"

# Summary
echo -e "\n${BLUE}=== Validation Summary ===${NC}"
echo -e "Total checks: ${TOTAL_CHECKS}"
echo -e "Passed: ${GREEN}${PASSED_CHECKS}${NC}"
echo -e "Failed: ${RED}${FAILED_CHECKS}${NC}"

if [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "\n${GREEN}✓ All validation checks passed!${NC}"
    echo -e "\nNext steps:"
    echo -e "1. Set up environment variables: cp configs/env-templates/postgresql.env .env"
    echo -e "2. Start services: ./scripts/start-all-databases.sh"
    echo -e "3. Test configuration: ./scripts/test-database-config.sh postgresql"
    exit 0
else
    echo -e "\n${RED}✗ Validation failed with ${FAILED_CHECKS} errors${NC}"
    echo -e "\nPlease fix the issues and run validation again."
    exit 1
fi