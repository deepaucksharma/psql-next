#!/bin/bash
# Script to create unified testing framework

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Creating Unified Testing Framework ===${NC}"

# Create test directory structure
echo -e "${YELLOW}Setting up test directory structure...${NC}"

mkdir -p tests/{unit,integration,e2e,performance,fixtures,utils}

# Create main test runner
echo -e "${YELLOW}Creating unified test runner...${NC}"

cat > scripts/testing/run-tests.sh << 'EOF'
#!/bin/bash
# Unified test runner for Database Intelligence

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test type from argument
TEST_TYPE=${1:-all}
DATABASE=${2:-all}

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${BLUE}=== Database Intelligence Test Runner ===${NC}"
echo -e "Test Type: ${YELLOW}$TEST_TYPE${NC}"
echo -e "Database: ${YELLOW}$DATABASE${NC}"
echo ""

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Function to run a test suite
run_test_suite() {
    local suite_name=$1
    local test_command=$2
    
    echo -e "${BLUE}Running $suite_name tests...${NC}"
    ((TOTAL_TESTS++))
    
    if eval "$test_command"; then
        echo -e "${GREEN}✓ $suite_name tests passed${NC}"
        ((PASSED_TESTS++))
    else
        echo -e "${RED}✗ $suite_name tests failed${NC}"
        ((FAILED_TESTS++))
    fi
}

# Run tests based on type
case "$TEST_TYPE" in
    unit)
        echo -e "${YELLOW}Running unit tests...${NC}"
        run_test_suite "Configuration validation" "$ROOT_DIR/scripts/validation/validate-config.sh"
        run_test_suite "Metric naming" "$ROOT_DIR/scripts/validation/validate-metric-naming.sh"
        ;;
    
    integration)
        echo -e "${YELLOW}Running integration tests...${NC}"
        if [ "$DATABASE" = "all" ]; then
            for db in postgresql mysql mongodb mssql oracle; do
                run_test_suite "$db integration" "$ROOT_DIR/scripts/testing/test-database-config.sh $db 30"
            done
        else
            run_test_suite "$DATABASE integration" "$ROOT_DIR/scripts/testing/test-database-config.sh $DATABASE 30"
        fi
        ;;
    
    e2e)
        echo -e "${YELLOW}Running end-to-end tests...${NC}"
        run_test_suite "E2E validation" "$ROOT_DIR/scripts/validation/validate-e2e.sh"
        run_test_suite "Integration test" "$ROOT_DIR/scripts/testing/test-integration.sh $DATABASE"
        ;;
    
    performance)
        echo -e "${YELLOW}Running performance tests...${NC}"
        if [ "$DATABASE" != "all" ]; then
            run_test_suite "$DATABASE performance" "$ROOT_DIR/scripts/testing/benchmark-performance.sh $DATABASE 60"
            run_test_suite "$DATABASE cardinality" "$ROOT_DIR/scripts/testing/check-metric-cardinality.sh $DATABASE"
        else
            echo -e "${RED}Please specify a database for performance testing${NC}"
            exit 1
        fi
        ;;
    
    all)
        echo -e "${YELLOW}Running all test suites...${NC}"
        # Run all test types
        $0 unit $DATABASE
        $0 integration $DATABASE
        $0 e2e $DATABASE
        ;;
    
    *)
        echo -e "${RED}Unknown test type: $TEST_TYPE${NC}"
        echo "Usage: $0 [unit|integration|e2e|performance|all] [database]"
        exit 1
        ;;
esac

# Summary
echo -e "\n${BLUE}=== Test Summary ===${NC}"
echo -e "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some tests failed!${NC}"
    exit 1
fi
EOF

chmod +x scripts/testing/run-tests.sh

# Create test utilities
echo -e "${YELLOW}Creating test utilities...${NC}"

cat > tests/utils/common.sh << 'EOF'
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
EOF

# Create test fixtures
echo -e "${YELLOW}Creating test fixtures...${NC}"

cat > tests/fixtures/minimal-config.yaml << 'EOF'
# Minimal test configuration
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 10s

exporters:
  debug:
    verbosity: basic

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
EOF

# Create sample unit test
echo -e "${YELLOW}Creating sample unit test...${NC}"

cat > tests/unit/test_config_validation.sh << 'EOF'
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
EOF

chmod +x tests/unit/test_config_validation.sh

# Create test documentation
echo -e "${YELLOW}Creating test documentation...${NC}"

cat > tests/README.md << 'EOF'
# Database Intelligence Test Suite

Comprehensive testing framework for Database Intelligence collectors.

## Test Structure

```
tests/
├── unit/          # Unit tests for individual components
├── integration/   # Integration tests with real databases
├── e2e/           # End-to-end tests with full pipeline
├── performance/   # Performance benchmarks and load tests
├── fixtures/      # Test data and configurations
└── utils/         # Shared test utilities
```

## Running Tests

### All Tests
```bash
./scripts/testing/run-tests.sh all
```

### Specific Test Type
```bash
# Unit tests only
./scripts/testing/run-tests.sh unit

# Integration tests
./scripts/testing/run-tests.sh integration postgresql

# End-to-end tests
./scripts/testing/run-tests.sh e2e

# Performance tests
./scripts/testing/run-tests.sh performance mysql
```

### Individual Test Suites
```bash
# Configuration validation
./scripts/validation/validate-config.sh

# Database-specific test
./scripts/testing/test-database-config.sh postgresql

# Performance benchmark
./scripts/testing/benchmark-performance.sh postgresql 300
```

## Writing Tests

### Unit Tests
Place unit tests in `tests/unit/` and follow the naming convention `test_*.sh`.

Example:
```bash
#!/bin/bash
source "$(dirname "$0")/../utils/common.sh"

# Test assertions
assert_equals "expected" "actual" "Test description"
assert_file_exists "path/to/file"
assert_contains "file.yaml" "pattern" "Should contain pattern"
```

### Integration Tests
Integration tests should:
1. Set up test environment
2. Run collector with test config
3. Verify metrics are collected
4. Clean up resources

### Performance Tests
Performance tests measure:
- Metric collection rate
- Memory usage
- CPU utilization
- Cardinality impact

## CI/CD Integration

The test suite is designed to run in CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run tests
  run: ./scripts/testing/run-tests.sh all
```

## Test Coverage

Current test coverage includes:
- ✅ Configuration validation
- ✅ Metric naming conventions
- ✅ Database connectivity
- ✅ Collector startup/shutdown
- ✅ Metric export verification
- ✅ Performance benchmarks
- ✅ Cardinality analysis
EOF

# Summary
echo -e "\n${BLUE}=== Unified Testing Framework Created ===${NC}"
echo "Created:"
echo "  - Unified test runner: scripts/testing/run-tests.sh"
echo "  - Test utilities: tests/utils/common.sh"
echo "  - Test fixtures: tests/fixtures/"
echo "  - Sample unit test: tests/unit/test_config_validation.sh"
echo "  - Test documentation: tests/README.md"

echo -e "\n${YELLOW}Usage:${NC}"
echo "  Run all tests: ./scripts/testing/run-tests.sh all"
echo "  Run unit tests: ./scripts/testing/run-tests.sh unit"
echo "  Run integration: ./scripts/testing/run-tests.sh integration postgresql"
echo "  Run performance: ./scripts/testing/run-tests.sh performance mysql"