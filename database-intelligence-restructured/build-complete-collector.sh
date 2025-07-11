#!/bin/bash
# Build complete Database Intelligence Collector with all custom components

set -e

echo "===== Database Intelligence Collector Build Script ====="
echo

# Check if builder is installed
if ! command -v builder &> /dev/null; then
    echo "Installing OpenTelemetry Collector Builder..."
    go install go.opentelemetry.io/collector/cmd/builder@v0.105.0
fi

# Clean previous builds
echo "Cleaning previous builds..."
rm -f distributions/production/database-intelligence
rm -f distributions/production/otelcol-*

# Run the builder
echo "Building collector with all components..."
builder --config=otelcol-builder-config-complete.yaml

# Check if build was successful
if [ -f "distributions/production/database-intelligence-collector" ]; then
    echo "Build successful!"
    mv distributions/production/database-intelligence-collector distributions/production/database-intelligence
    echo "Binary available at: distributions/production/database-intelligence"
    
    # Make it executable
    chmod +x distributions/production/database-intelligence
    
    # Show binary info
    echo
    echo "Binary information:"
    ls -lh distributions/production/database-intelligence
    
    # Test the binary
    echo
    echo "Testing binary..."
    distributions/production/database-intelligence --version
else
    echo "Build failed! Check error messages above."
    exit 1
fi

echo
echo "===== Build Complete ====="
echo
echo "Next steps:"
echo "1. Test locally: ./distributions/production/database-intelligence --config=distributions/production/production-config-complete.yaml"
echo "2. Build Docker image: cd distributions/production && docker build -t database-intelligence:latest ."
echo "3. Run with docker-compose: cd deployments/docker/compose && docker-compose up"