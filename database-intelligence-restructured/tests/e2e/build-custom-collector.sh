#!/bin/bash

# Build custom collector with all processors
cd ../..

echo "Building custom collector with all processors..."

# Ensure builder is installed
if ! command -v builder &> /dev/null; then
    echo "Installing OpenTelemetry Collector Builder..."
    go install go.opentelemetry.io/collector/cmd/builder@v0.109.0
fi

# Use existing builder config
if [ -f "builder-config.yaml" ]; then
    echo "Using builder-config.yaml"
    $HOME/go/bin/builder --config=builder-config.yaml
    
    if [ $? -eq 0 ]; then
        echo "Build successful!"
        echo "Collector binary: ./build/database-intelligence-collector"
        
        # Copy to e2e directory
        cp ./build/database-intelligence-collector ./tests/e2e/custom-collector
        chmod +x ./tests/e2e/custom-collector
        echo "Copied collector to ./tests/e2e/custom-collector"
    else
        echo "Build failed!"
        exit 1
    fi
else
    echo "builder-config.yaml not found!"
    exit 1
fi