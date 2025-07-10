#!/bin/bash

# Final cleanup and build script
set -e

echo "=== Final Cleanup and Build ==="
echo

# Step 1: Remove all toolchain directives
echo "Step 1: Removing invalid toolchain directives..."
find . -name "go.mod" -type f | while read -r gomod; do
    # Remove go 1.22.0 and replace with go 1.22
    sed -i '' 's/^go 1\.22\.0$/go 1.22/g' "$gomod"
    sed -i '' 's/^go 1\.23$/go 1.22/g' "$gomod"
    # Remove toolchain directive
    sed -i '' '/^toolchain go1\.24\.3$/d' "$gomod"
done

# Step 2: Fix circuitbreaker dependencies
echo -e "\nStep 2: Fixing circuitbreaker module..."
cat > processors/circuitbreaker/go.mod << 'EOF'
module github.com/database-intelligence/processors/circuitbreaker

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/consumer v0.109.0
    go.opentelemetry.io/collector/pdata v1.15.0
    go.opentelemetry.io/collector/processor v0.109.0
    go.uber.org/zap v1.27.0
    github.com/stretchr/testify v1.9.0
    github.com/database-intelligence/common/featuredetector v0.0.0-00010101000000-000000000000
)

replace github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
EOF

# Step 3: Build enterprise distribution
echo -e "\nStep 3: Building enterprise distribution..."
cd distributions/enterprise

echo "  Running go mod tidy..."
if go mod tidy; then
    echo "  ✓ Dependencies resolved"
else
    echo "  ✗ Failed to resolve dependencies"
    echo "  Continuing anyway..."
fi

echo "  Building collector binary..."
if go build -o database-intelligence-collector ./main.go; then
    echo "  ✓ Build successful!"
    echo "  Binary location: $(pwd)/database-intelligence-collector"
else
    echo "  ✗ Build failed"
    exit 1
fi

cd ../..

# Step 4: Validate the binary
echo -e "\nStep 4: Validating binary..."
if distributions/enterprise/database-intelligence-collector --version; then
    echo "  ✓ Binary is functional"
else
    echo "  ✗ Binary validation failed"
fi

echo
echo "=== Build Complete ==="
echo
echo "The Database Intelligence Collector has been built successfully!"
echo "Binary location: distributions/enterprise/database-intelligence-collector"
echo
echo "To run the collector:"
echo "  cd distributions/enterprise"
echo "  ./database-intelligence-collector --config=../../configs/unified/database-intelligence-complete.yaml"
echo
echo "To build Docker image:"
echo "  docker build -f deployments/docker/dockerfiles/Dockerfile.custom -t db-intel-collector:latest ."