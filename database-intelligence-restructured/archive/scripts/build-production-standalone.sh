#!/bin/bash

echo "=== Building Production Collector Standalone ==="

# Navigate to production directory
cd distributions/production

# Remove go.work dependency by setting GOWORK=off
export GOWORK=off

# Clear module cache for this build
rm -f go.sum

# First, let's see what's missing
echo "Checking missing dependencies..."
go mod download 2>&1 | grep -E "(no required module|cannot find)" | head -10 || true

# Run go mod tidy to resolve dependencies
echo ""
echo "Running go mod tidy..."
go mod tidy -v 2>&1 || true

# Check if we need to add internal/database to requirements
if ! grep -q "github.com/database-intelligence/internal/database" go.mod; then
    echo "Adding internal/database to requirements..."
    # Add it before the closing parenthesis of require section
    sed -i.bak '/^)$/i\
	\
	// Internal modules\
	github.com/database-intelligence/internal/database v0.0.0-00010101000000-000000000000' go.mod
fi

# Try building with verbose output
echo ""
echo "Building collector..."
if go build -v -o otelcol-database-intelligence . 2>&1; then
    echo ""
    echo "✓ Production collector built successfully!"
    ls -la otelcol-database-intelligence
    
    # Test the binary
    echo ""
    echo "Testing collector binary..."
    ./otelcol-database-intelligence --version || true
    
    echo ""
    echo "Checking available components..."
    ./otelcol-database-intelligence components || true
else
    echo ""
    echo "⚠ Build failed. Let's check specific issues..."
    
    # Check for missing packages
    echo ""
    echo "Missing packages:"
    go list -m all 2>&1 | grep -E "no required module" | head -10
    
    # Check for compilation errors
    echo ""
    echo "Compilation errors:"
    go build . 2>&1 | grep -E "undefined|cannot|error" | head -20
fi

# Go back to project root
cd ../..

echo ""
echo "=== Build attempt complete ==="
echo ""
echo "To run the collector:"
echo "  cd distributions/production"
echo "  ./otelcol-database-intelligence --config=../../config/collector-simple.yaml"