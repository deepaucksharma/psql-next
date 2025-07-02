#!/bin/bash

# Simple Test Runner for Database Intelligence Collector
# This script runs tests in a straightforward manner

set -euo pipefail

echo "=== Database Intelligence Collector - Test Suite ==="
echo "Date: $(date)"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Results tracking
PASSED=0
FAILED=0

# Function to run a test
run_test() {
    local name="$1"
    local cmd="$2"
    
    echo -n "Testing $name... "
    if eval "$cmd" &>/dev/null; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++))
    else
        echo -e "${RED}FAIL${NC}"
        ((FAILED++))
        # Show error output
        echo "  Error output:"
        eval "$cmd" 2>&1 | head -20 | sed 's/^/    /'
    fi
}

# 1. Build Test
echo "=== Build Tests ==="
run_test "Main Build" "go build -o /tmp/test-collector ./main.go"

# 2. Unit Tests
echo ""
echo "=== Unit Tests ==="
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    run_test "$processor" "cd processors/$processor && go test -v -count=1 ."
done

# 3. Integration Tests
echo ""
echo "=== Integration Tests ==="
run_test "Integration" "cd tests/integration && go test -v -count=1 -short ."

# 4. E2E Tests (simplified)
echo ""
echo "=== E2E Tests ==="
run_test "Simplified E2E" "cd tests/e2e && go test -v -count=1 -run TestSimplified ./simplified_e2e_test.go ./package_test.go"

# Summary
echo ""
echo "=== Summary ==="
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo -e "Total: $((PASSED + FAILED))"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed${NC}"
    exit 1
fi