#!/bin/bash

# Validate E2E Structure and Components
# This validates the refactored project structure

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
REPORT_FILE="$PROJECT_ROOT/e2e-validation-report.md"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== E2E STRUCTURE VALIDATION ===${NC}"

# Initialize report
cat > "$REPORT_FILE" << 'EOF'
# E2E Validation Report

**Date:** $(date)  
**Project:** Database Intelligence Restructured

## Project Structure Validation

EOF

# ==============================================================================
# Processors Validation
# ==============================================================================
echo -e "\n${CYAN}Validating Processors...${NC}"
echo -e "\n### Processors" >> "$REPORT_FILE"

PROCESSORS=(
    "adaptivesampler"
    "circuitbreaker"
    "costcontrol"
    "nrerrormonitor"
    "planattributeextractor"
    "querycorrelator"
    "verification"
)

PROCESSOR_COUNT=0
for processor in "${PROCESSORS[@]}"; do
    if [ -d "processors/$processor" ] && [ -f "processors/$processor/go.mod" ]; then
        echo -e "${GREEN}[✓]${NC} $processor"
        echo "- ✓ $processor" >> "$REPORT_FILE"
        ((PROCESSOR_COUNT++))
    else
        echo -e "${RED}[✗]${NC} $processor"
        echo "- ✗ $processor (missing)" >> "$REPORT_FILE"
    fi
done

echo -e "\nProcessors found: ${PROCESSOR_COUNT}/${#PROCESSORS[@]}" >> "$REPORT_FILE"

# ==============================================================================
# Receivers Validation
# ==============================================================================
echo -e "\n${CYAN}Validating Receivers...${NC}"
echo -e "\n### Receivers" >> "$REPORT_FILE"

RECEIVERS=(
    "ash"
    "enhancedsql"
    "kernelmetrics"
)

RECEIVER_COUNT=0
for receiver in "${RECEIVERS[@]}"; do
    if [ -d "receivers/$receiver" ] && [ -f "receivers/$receiver/go.mod" ]; then
        echo -e "${GREEN}[✓]${NC} $receiver"
        echo "- ✓ $receiver" >> "$REPORT_FILE"
        ((RECEIVER_COUNT++))
    else
        echo -e "${RED}[✗]${NC} $receiver"
        echo "- ✗ $receiver (missing)" >> "$REPORT_FILE"
    fi
done

echo -e "\nReceivers found: ${RECEIVER_COUNT}/${#RECEIVERS[@]}" >> "$REPORT_FILE"

# ==============================================================================
# Configuration Files
# ==============================================================================
echo -e "\n${CYAN}Validating Configurations...${NC}"
echo -e "\n### Configuration Files" >> "$REPORT_FILE"

CONFIGS=(
    "config/base/processors-base.yaml"
    "config/collector-simplified.yaml"
    "config/environments/development.yaml"
    "config/environments/production.yaml"
    "config/environments/staging.yaml"
)

CONFIG_COUNT=0
for config in "${CONFIGS[@]}"; do
    if [ -f "$config" ]; then
        echo -e "${GREEN}[✓]${NC} $config"
        echo "- ✓ $config" >> "$REPORT_FILE"
        ((CONFIG_COUNT++))
    else
        echo -e "${RED}[✗]${NC} $config"
        echo "- ✗ $config (missing)" >> "$REPORT_FILE"
    fi
done

echo -e "\nConfigs found: ${CONFIG_COUNT}/${#CONFIGS[@]}" >> "$REPORT_FILE"

# ==============================================================================
# Docker/Kubernetes Files
# ==============================================================================
echo -e "\n${CYAN}Validating Deployment Files...${NC}"
echo -e "\n### Deployment Files" >> "$REPORT_FILE"

DEPLOYMENT_FILES=(
    "deployments/docker/compose/docker-compose-databases.yaml"
    "deployments/docker/Dockerfile"
    "deployments/kubernetes/base/kustomization.yaml"
    "deployments/helm/charts/database-intelligence/Chart.yaml"
)

DEPLOYMENT_COUNT=0
for file in "${DEPLOYMENT_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${GREEN}[✓]${NC} $file"
        echo "- ✓ $file" >> "$REPORT_FILE"
        ((DEPLOYMENT_COUNT++))
    else
        echo -e "${RED}[✗]${NC} $file"
        echo "- ✗ $file (missing)" >> "$REPORT_FILE"
    fi
done

echo -e "\nDeployment files found: ${DEPLOYMENT_COUNT}/${#DEPLOYMENT_FILES[@]}" >> "$REPORT_FILE"

# ==============================================================================
# Test Database Connectivity
# ==============================================================================
echo -e "\n${CYAN}Testing Database Connectivity...${NC}"
echo -e "\n### Database Connectivity" >> "$REPORT_FILE"

# PostgreSQL
if docker exec db-intel-postgres pg_isready -U postgres > /dev/null 2>&1; then
    echo -e "${GREEN}[✓]${NC} PostgreSQL connection successful"
    echo "- ✓ PostgreSQL: Connected" >> "$REPORT_FILE"
    
    # Check tables
    TABLES=$(docker exec db-intel-postgres psql -U postgres -d testdb -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public';")
    echo "  Tables in testdb: $TABLES" >> "$REPORT_FILE"
else
    echo -e "${RED}[✗]${NC} PostgreSQL connection failed"
    echo "- ✗ PostgreSQL: Failed" >> "$REPORT_FILE"
fi

# MySQL
if docker exec db-intel-mysql mysqladmin ping -h localhost -u root -ppassword > /dev/null 2>&1; then
    echo -e "${GREEN}[✓]${NC} MySQL connection successful"
    echo "- ✓ MySQL: Connected" >> "$REPORT_FILE"
    
    # Check tables
    TABLES=$(docker exec db-intel-mysql mysql -u root -ppassword testdb -sN -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='testdb';")
    echo "  Tables in testdb: $TABLES" >> "$REPORT_FILE"
else
    echo -e "${RED}[✗]${NC} MySQL connection failed"
    echo "- ✗ MySQL: Failed" >> "$REPORT_FILE"
fi

# ==============================================================================
# Go Module Validation
# ==============================================================================
echo -e "\n${CYAN}Validating Go Modules...${NC}"
echo -e "\n### Go Module Health" >> "$REPORT_FILE"

# Check go.work
if [ -f "go.work" ]; then
    echo -e "${GREEN}[✓]${NC} go.work exists"
    echo "- ✓ go.work exists" >> "$REPORT_FILE"
    
    # Count modules in workspace
    MODULE_COUNT=$(grep -c "^\s*\./" go.work || echo 0)
    echo "  Modules in workspace: $MODULE_COUNT" >> "$REPORT_FILE"
else
    echo -e "${RED}[✗]${NC} go.work missing"
    echo "- ✗ go.work missing" >> "$REPORT_FILE"
fi

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}Generating Summary...${NC}"
echo -e "\n## Summary" >> "$REPORT_FILE"

TOTAL_CHECKS=$((${#PROCESSORS[@]} + ${#RECEIVERS[@]} + ${#CONFIGS[@]} + ${#DEPLOYMENT_FILES[@]} + 3))
PASSED_CHECKS=$((PROCESSOR_COUNT + RECEIVER_COUNT + CONFIG_COUNT + DEPLOYMENT_COUNT))

# Add database checks
if docker exec db-intel-postgres pg_isready -U postgres > /dev/null 2>&1; then
    ((PASSED_CHECKS++))
fi
if docker exec db-intel-mysql mysqladmin ping -h localhost -u root -ppassword > /dev/null 2>&1; then
    ((PASSED_CHECKS++))
fi
if [ -f "go.work" ]; then
    ((PASSED_CHECKS++))
fi

echo -e "\n**Total Checks:** $TOTAL_CHECKS" >> "$REPORT_FILE"
echo -e "**Passed:** $PASSED_CHECKS" >> "$REPORT_FILE"
echo -e "**Failed:** $((TOTAL_CHECKS - PASSED_CHECKS))" >> "$REPORT_FILE"
echo -e "**Success Rate:** $((PASSED_CHECKS * 100 / TOTAL_CHECKS))%" >> "$REPORT_FILE"

# ==============================================================================
# Recommendations
# ==============================================================================
echo -e "\n## Recommendations" >> "$REPORT_FILE"

if [ $PROCESSOR_COUNT -lt ${#PROCESSORS[@]} ]; then
    echo -e "\n1. Some processors are missing. Consider restoring them from backup." >> "$REPORT_FILE"
fi

if [ $RECEIVER_COUNT -lt ${#RECEIVERS[@]} ]; then
    echo -e "\n2. Some receivers are missing. Consider restoring them from backup." >> "$REPORT_FILE"
fi

if [ $CONFIG_COUNT -lt ${#CONFIGS[@]} ]; then
    echo -e "\n3. Some configuration files are missing." >> "$REPORT_FILE"
fi

echo -e "\n${GREEN}=== VALIDATION COMPLETE ===${NC}"
echo -e "Report saved to: ${CYAN}$REPORT_FILE${NC}"
echo -e "\nSummary: ${GREEN}$PASSED_CHECKS${NC}/${TOTAL_CHECKS} checks passed (${GREEN}$((PASSED_CHECKS * 100 / TOTAL_CHECKS))%${NC})"

# Stop databases
echo -e "\n${YELLOW}Stopping test databases...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml down

exit 0