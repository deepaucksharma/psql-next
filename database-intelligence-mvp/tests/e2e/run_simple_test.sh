#!/bin/bash
# Simple E2E test runner

set -e

echo "Running E2E tests..."
echo "==================="

# Check if containers are running
if ! docker ps | grep -q e2e-postgres; then
    echo "Starting test containers..."
    docker-compose up -d
    echo "Waiting for containers to be ready..."
    sleep 10
fi

# Run the database to NRDB verification test
echo "Running database to NRDB verification tests..."
go test -v -timeout 5m -run TestDatabaseToNRDBVerification ./database_to_nrdb_verification_test.go

# Run processor tests if they exist
if [ -d "processor_tests" ]; then
    echo "Running processor tests..."
    go test -v -timeout 5m ./processor_tests/...
fi

echo "==================="
echo "E2E tests completed!"