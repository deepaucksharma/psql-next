#!/bin/bash

# Run Comprehensive E2E Tests
# This script runs the comprehensive E2E test suite that validates the complete flow
# from real databases to NRDB verification without shortcuts

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "================================================"
echo "Database Intelligence Collector - Comprehensive E2E Test"
echo "================================================"
echo ""
echo "This test validates the complete flow:"
echo "1. Real database connections (PostgreSQL/MySQL)"
echo "2. All 7 custom processors"
echo "3. pg_querylens integration"
echo "4. Full NRDB verification with actual queries"
echo "5. End-to-end latency measurement"
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# Check for required environment variables
if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]] || [[ -z "${NEW_RELIC_API_KEY:-}" ]]; then
    echo "⚠️  WARNING: NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY not set"
    echo "These are required for NRDB verification tests"
    echo ""
    echo "You can still run the test to validate local processing"
    read -p "Continue without NRDB verification? (y/n) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check PostgreSQL connection
echo -n "Checking PostgreSQL connection... "
if PGPASSWORD="${POSTGRES_PASSWORD:-postgres}" psql -h "${POSTGRES_HOST:-localhost}" -p "${POSTGRES_PORT:-5432}" -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-testdb}" -c "SELECT 1" &>/dev/null; then
    echo "✓"
else
    echo "✗"
    echo "ERROR: Cannot connect to PostgreSQL"
    echo "Please ensure PostgreSQL is running and accessible"
    exit 1
fi

# Check if collector is running
echo -n "Checking if collector is running... "
if curl -s http://localhost:8888/health &>/dev/null; then
    echo "✓"
else
    echo "✗"
    echo "WARNING: Collector not running at localhost:8888"
    echo "Some tests may fail without a running collector"
fi

echo ""
echo "Running comprehensive E2E tests..."
echo ""

# Create test output directory
mkdir -p output

# Run the comprehensive test with proper build tags and files
go test -v \
    -timeout 30m \
    -tags=e2e \
    -run "TestComprehensiveE2EFlow|TestRecentFeatures" \
    comprehensive_e2e_test.go \
    test_helpers.go \
    nrdb_validation_test.go \
    test_environment.go \
    2>&1 | tee output/comprehensive-e2e-$(date +%Y%m%d-%H%M%S).log

echo ""
echo "================================================"
echo "Test execution complete. Check output/ for logs"
echo "================================================"