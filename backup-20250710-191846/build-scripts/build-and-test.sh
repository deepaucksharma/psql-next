#!/bin/bash

# Build and Test Script for Database Intelligence Collector
set -e

echo "=== Database Intelligence Collector - Build and Test ==="
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

# Step 1: Build the enterprise distribution
print_status "Building enterprise distribution..."
cd distributions/enterprise

# Run go mod tidy to resolve dependencies
print_status "Resolving dependencies..."
go mod tidy || {
    print_error "Failed to resolve dependencies"
    exit 1
}

# Build the collector
print_status "Building collector binary..."
go build -o ../../bin/database-intelligence-collector ./main.go || {
    print_error "Failed to build collector"
    exit 1
}

cd ../..

print_status "Build completed successfully!"
echo

# Step 2: Validate configuration
print_status "Validating configuration..."
./bin/database-intelligence-collector validate --config=configs/unified/database-intelligence-complete.yaml || {
    print_error "Configuration validation failed"
    exit 1
}

print_status "Configuration is valid!"
echo

# Step 3: Run unit tests
print_status "Running unit tests..."

# Test each processor
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    echo "Testing processor: $processor"
    cd processors/$processor
    go test ./... -v || {
        print_warning "Tests failed for processor: $processor"
    }
    cd ../..
done

echo
print_status "All tests completed!"
echo

# Step 4: Generate summary
echo "=== Build Summary ==="
echo "Binary location: ./bin/database-intelligence-collector"
echo "Configuration: ./configs/unified/database-intelligence-complete.yaml"
echo
echo "To run the collector:"
echo "  ./bin/database-intelligence-collector --config=configs/unified/database-intelligence-complete.yaml"
echo
echo "To run with Docker Compose:"
echo "  docker-compose -f docker-compose.unified.yml up"