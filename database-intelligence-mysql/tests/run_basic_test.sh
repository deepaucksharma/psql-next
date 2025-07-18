#!/bin/bash

# Quick test runner for basic validation
set -e

echo "Loading environment variables..."
cd "$(dirname "$0")/.."

# Load .env file if it exists
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
    echo "✓ Environment variables loaded from .env"
else
    echo "⚠ Warning: .env file not found"
fi

# Run basic validation tests
echo ""
echo "Running basic validation tests..."
cd tests/e2e

# First, ensure we have the necessary Go modules
echo "Downloading dependencies..."
go mod download

# Run the basic validation test
echo ""
echo "Testing MySQL connectivity and New Relic credentials..."
go test -v -run TestBasicConnectivity ./basic_validation_test.go

echo ""
echo "Testing New Relic API connectivity..."
go test -v -run TestBasicNewRelicQuery ./basic_validation_test.go

echo ""
echo "✅ Basic validation complete!"
echo ""
echo "Next steps:"
echo "1. Ensure MySQL is running with Performance Schema enabled"
echo "2. Ensure OpenTelemetry collectors are running"
echo "3. Run full E2E tests with: ./run_tests_with_env.sh all"