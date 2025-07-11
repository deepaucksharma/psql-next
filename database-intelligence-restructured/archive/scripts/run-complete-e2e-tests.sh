#!/bin/bash

# Comprehensive E2E Testing Script
# This script runs all end-to-end tests and verifies complete flows

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
TEST_REPORT="$PROJECT_ROOT/E2E_TEST_REPORT_$(date +%Y%m%d-%H%M%S).md"
LOG_DIR="$PROJECT_ROOT/test-logs"

# Create log directory
mkdir -p "$LOG_DIR"

cd "$PROJECT_ROOT"

# Initialize report
cat > "$TEST_REPORT" << 'EOF'
# End-to-End Test Execution Report

Generated: DATE_PLACEHOLDER

## Overview
This report documents the execution of all end-to-end tests and the issues found.

EOF
sed -i.bak "s/DATE_PLACEHOLDER/$(date)/" "$TEST_REPORT" && rm -f "${TEST_REPORT}.bak"

echo -e "${BLUE}=== COMPREHENSIVE E2E TESTING ===${NC}"
echo "Test report: $TEST_REPORT"
echo "Log directory: $LOG_DIR"

# Function to add to report
report() {
    echo "$1" >> "$TEST_REPORT"
}

# Function to log test result
log_test() {
    local test_name=$1
    local status=$2
    local message=$3
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}[✓]${NC} $test_name: $message"
        report "- ✅ **$test_name**: $message"
    elif [ "$status" = "FAIL" ]; then
        echo -e "${RED}[✗]${NC} $test_name: $message"
        report "- ❌ **$test_name**: $message"
    else
        echo -e "${YELLOW}[!]${NC} $test_name: $message"
        report "- ⚠️ **$test_name**: $message"
    fi
}

# ==============================================================================
# STEP 1: Environment Setup
# ==============================================================================
echo -e "\n${CYAN}STEP 1: ENVIRONMENT SETUP${NC}"
report "## 1. Environment Setup"
report ""

# Check Docker
if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker --version | awk '{print $3}' | tr -d ',')
    log_test "Docker" "PASS" "Version $DOCKER_VERSION"
else
    log_test "Docker" "FAIL" "Not installed"
    exit 1
fi

# Check Docker Compose
if command -v docker-compose &> /dev/null; then
    COMPOSE_VERSION=$(docker-compose --version | awk '{print $4}' | tr -d ',')
    log_test "Docker Compose" "PASS" "Version $COMPOSE_VERSION"
else
    log_test "Docker Compose" "FAIL" "Not installed"
    exit 1
fi

# Check Go
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    log_test "Go" "PASS" "$GO_VERSION"
else
    log_test "Go" "FAIL" "Not installed"
    exit 1
fi

# Create .env file if missing
if [ ! -f ".env" ]; then
    echo -e "${YELLOW}Creating .env file...${NC}"
    cat > .env << 'EOF'
# Database Intelligence E2E Test Environment

# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=password
POSTGRES_DB=testdb

# MySQL
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=password
MYSQL_DATABASE=testdb

# New Relic (optional - set your own values)
NEW_RELIC_LICENSE_KEY=test-key
NEW_RELIC_ACCOUNT_ID=12345

# Collector
OTEL_LOG_LEVEL=debug
OTEL_RESOURCE_ATTRIBUTES=service.name=database-intelligence-e2e
EOF
    log_test "Environment File" "PASS" "Created .env"
else
    log_test "Environment File" "PASS" "Exists"
fi

# ==============================================================================
# STEP 2: Build Collector
# ==============================================================================
echo -e "\n${CYAN}STEP 2: BUILD COLLECTOR${NC}"
report ""
report "## 2. Build Collector"
report ""

# First, let's fix go.work sync
echo -e "${YELLOW}Syncing Go workspace...${NC}"
if go work sync > "$LOG_DIR/go-work-sync.log" 2>&1; then
    log_test "Go Workspace Sync" "PASS" "Workspace synchronized"
else
    log_test "Go Workspace Sync" "FAIL" "Check $LOG_DIR/go-work-sync.log"
    cat "$LOG_DIR/go-work-sync.log" | tail -20
fi

# Try to build a minimal test collector
echo -e "${YELLOW}Building test collector...${NC}"
cd "$PROJECT_ROOT/distributions/minimal"

if go build -o "$PROJECT_ROOT/test-collector" > "$LOG_DIR/build-minimal.log" 2>&1; then
    log_test "Minimal Build" "PASS" "Test collector built"
else
    log_test "Minimal Build" "FAIL" "Check $LOG_DIR/build-minimal.log"
    echo -e "${RED}Build failed. Last 20 lines of log:${NC}"
    tail -20 "$LOG_DIR/build-minimal.log"
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# STEP 3: Start Test Infrastructure
# ==============================================================================
echo -e "\n${CYAN}STEP 3: START TEST INFRASTRUCTURE${NC}"
report ""
report "## 3. Test Infrastructure"
report ""

# Stop any existing containers
echo -e "${YELLOW}Stopping existing containers...${NC}"
docker-compose -f deployments/docker/compose/docker-compose.yaml down > /dev/null 2>&1 || true

# Start databases
echo -e "${YELLOW}Starting test databases...${NC}"
if docker-compose -f deployments/docker/compose/docker-compose-databases.yaml up -d > "$LOG_DIR/docker-databases.log" 2>&1; then
    log_test "Database Startup" "PASS" "PostgreSQL and MySQL started"
    
    # Wait for databases to be ready
    echo -e "${YELLOW}Waiting for databases to be ready...${NC}"
    sleep 10
    
    # Check PostgreSQL
    if docker exec db-intel-postgres pg_isready -U postgres > /dev/null 2>&1; then
        log_test "PostgreSQL Health" "PASS" "Database ready"
    else
        log_test "PostgreSQL Health" "FAIL" "Database not ready"
    fi
    
    # Check MySQL
    if docker exec db-intel-mysql mysqladmin ping -h localhost -u root -ppassword > /dev/null 2>&1; then
        log_test "MySQL Health" "PASS" "Database ready"
    else
        log_test "MySQL Health" "FAIL" "Database not ready"
    fi
else
    log_test "Database Startup" "FAIL" "Check $LOG_DIR/docker-databases.log"
fi

# ==============================================================================
# STEP 4: Test Configuration Files
# ==============================================================================
echo -e "\n${CYAN}STEP 4: TEST CONFIGURATION FILES${NC}"
report ""
report "## 4. Configuration Tests"
report ""

# Test each example configuration
CONFIG_DIR="$PROJECT_ROOT/configs/examples"
for config in "$CONFIG_DIR"/*.yaml; do
    if [ -f "$config" ]; then
        config_name=$(basename "$config")
        echo -e "\n${YELLOW}Testing config: $config_name${NC}"
        
        # Try to validate the config (basic syntax check)
        if [ -s "$config" ]; then
            # Check if it's a valid YAML by looking for key patterns
            if grep -q "receivers:\|processors:\|exporters:\|service:" "$config"; then
                log_test "Config: $config_name" "PASS" "Valid structure"
            else
                log_test "Config: $config_name" "WARN" "May be incomplete"
            fi
        else
            log_test "Config: $config_name" "FAIL" "Empty file"
        fi
    fi
done

# ==============================================================================
# STEP 5: Run Go Tests
# ==============================================================================
echo -e "\n${CYAN}STEP 5: RUN GO TESTS${NC}"
report ""
report "## 5. Go Test Execution"
report ""

# Run processor tests
echo -e "\n${YELLOW}Running processor tests...${NC}"
PROCESSOR_FAILED=0
for processor in processors/*/; do
    if [ -d "$processor" ] && [ -f "$processor/go.mod" ]; then
        proc_name=$(basename "$processor")
        cd "$processor"
        
        if go test -v -timeout 30s > "$LOG_DIR/test-$proc_name.log" 2>&1; then
            log_test "Processor Test: $proc_name" "PASS" "All tests passed"
        else
            log_test "Processor Test: $proc_name" "FAIL" "Check $LOG_DIR/test-$proc_name.log"
            ((PROCESSOR_FAILED++))
        fi
        
        cd "$PROJECT_ROOT"
    fi
done

# Run E2E tests
echo -e "\n${YELLOW}Running E2E tests...${NC}"
cd "$PROJECT_ROOT/tests/e2e"

# First check if we can build
if go build ./... > "$LOG_DIR/e2e-build.log" 2>&1; then
    log_test "E2E Build" "PASS" "E2E tests compiled"
    
    # Run specific test suites
    TEST_SUITES=(
        "database_to_nrdb_verification_test.go"
        "simple_db_test.go"
    )
    
    for suite in "${TEST_SUITES[@]}"; do
        if [ -f "suites/$suite" ]; then
            echo -e "\n${YELLOW}Running test suite: $suite${NC}"
            if go test -v -timeout 60s "./suites" -run "Test.*" > "$LOG_DIR/e2e-$suite.log" 2>&1; then
                log_test "E2E Suite: $suite" "PASS" "Tests passed"
            else
                log_test "E2E Suite: $suite" "FAIL" "Check $LOG_DIR/e2e-$suite.log"
            fi
        fi
    done
else
    log_test "E2E Build" "FAIL" "Check $LOG_DIR/e2e-build.log"
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# STEP 6: Integration Flow Tests
# ==============================================================================
echo -e "\n${CYAN}STEP 6: INTEGRATION FLOW TESTS${NC}"
report ""
report "## 6. Integration Flow Tests"
report ""

# Test 1: Database → Collector → Export flow
echo -e "\n${YELLOW}Testing complete data flow...${NC}"

# Create a test collector config
cat > "$PROJECT_ROOT/test-flow-config.yaml" << 'EOF'
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: password
    databases:
      - testdb
    collection_interval: 10s

processors:
  batch:
    timeout: 10s

exporters:
  debug:
    verbosity: detailed
  file:
    path: /tmp/metrics.json

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [debug, file]
EOF

# Try to run the collector with test config
if [ -f "$PROJECT_ROOT/test-collector" ]; then
    echo -e "${YELLOW}Starting test collector...${NC}"
    
    # Run collector for 30 seconds
    timeout 30s "$PROJECT_ROOT/test-collector" --config="$PROJECT_ROOT/test-flow-config.yaml" > "$LOG_DIR/collector-flow.log" 2>&1 || true
    
    # Check if metrics were generated
    if [ -f "/tmp/metrics.json" ]; then
        log_test "Data Flow Test" "PASS" "Metrics exported successfully"
        # Show sample metrics
        echo -e "${GREEN}Sample metrics:${NC}"
        head -5 /tmp/metrics.json
    else
        log_test "Data Flow Test" "FAIL" "No metrics exported"
    fi
else
    log_test "Data Flow Test" "SKIP" "Test collector not built"
fi

# ==============================================================================
# STEP 7: Issue Analysis
# ==============================================================================
echo -e "\n${CYAN}STEP 7: ISSUE ANALYSIS${NC}"
report ""
report "## 7. Issues Found and Fixes Needed"
report ""

# Count failures
TOTAL_FAILURES=$(grep -c "❌" "$TEST_REPORT" || echo 0)
TOTAL_WARNINGS=$(grep -c "⚠️" "$TEST_REPORT" || echo 0)
TOTAL_PASSES=$(grep -c "✅" "$TEST_REPORT" || echo 0)

report ""
report "### Test Summary"
report "- Total Passed: $TOTAL_PASSES ✅"
report "- Total Warnings: $TOTAL_WARNINGS ⚠️"
report "- Total Failed: $TOTAL_FAILURES ❌"

# Analyze common issues
if [ $TOTAL_FAILURES -gt 0 ]; then
    report ""
    report "### Common Issues Detected"
    
    # Check for import errors
    if grep -q "cannot find module" "$LOG_DIR"/*.log 2>/dev/null; then
        report "- **Import Path Issues**: Some modules cannot be resolved"
    fi
    
    # Check for compilation errors
    if grep -q "undefined:" "$LOG_DIR"/*.log 2>/dev/null; then
        report "- **Undefined References**: Some functions or types are not found"
    fi
    
    # Check for connection errors
    if grep -q "connection refused\|timeout" "$LOG_DIR"/*.log 2>/dev/null; then
        report "- **Connection Issues**: Cannot connect to databases or services"
    fi
fi

# ==============================================================================
# STEP 8: Cleanup
# ==============================================================================
echo -e "\n${CYAN}STEP 8: CLEANUP${NC}"
report ""
report "## 8. Cleanup"
report ""

# Stop test containers
echo -e "${YELLOW}Stopping test containers...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml down > /dev/null 2>&1

# Remove test files
rm -f "$PROJECT_ROOT/test-flow-config.yaml"
rm -f "$PROJECT_ROOT/test-collector"
rm -f /tmp/metrics.json

log_test "Cleanup" "PASS" "Test environment cleaned"

# Final summary
echo -e "\n${CYAN}=== E2E TEST SUMMARY ===${NC}"
echo -e "Total Tests Run: $((TOTAL_PASSES + TOTAL_WARNINGS + TOTAL_FAILURES))"
echo -e "Passed: ${GREEN}$TOTAL_PASSES${NC}"
echo -e "Warnings: ${YELLOW}$TOTAL_WARNINGS${NC}"
echo -e "Failed: ${RED}$TOTAL_FAILURES${NC}"
echo -e "\nFull report: ${BLUE}$TEST_REPORT${NC}"
echo -e "Logs directory: ${BLUE}$LOG_DIR${NC}"

# Provide next steps
if [ $TOTAL_FAILURES -gt 0 ]; then
    echo -e "\n${YELLOW}Next Steps:${NC}"
    echo "1. Review the test report: $TEST_REPORT"
    echo "2. Check detailed logs in: $LOG_DIR"
    echo "3. Run fix-e2e-issues.sh to address common problems"
fi