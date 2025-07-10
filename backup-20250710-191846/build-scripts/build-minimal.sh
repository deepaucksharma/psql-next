#!/bin/bash

# Minimal build script to test custom processors
set -e

echo "=== Building Minimal Collector with Custom Processors ==="
echo

# Create a minimal test directory
mkdir -p test-build
cd test-build

# Create a minimal main.go
cat > main.go << 'EOF'
package main

import (
    "log"
    "os"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
)

func main() {
    info := component.BuildInfo{
        Command:     "test-collector",
        Description: "Test Database Intelligence Collector",
        Version:     "0.1.0",
    }

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: otelcol.Factories{},
    }); err != nil {
        log.Fatal(err)
    }
}

func run(settings otelcol.CollectorSettings) error {
    cmd := otelcol.NewCommand(settings)
    return cmd.Execute()
}
EOF

# Create a minimal go.mod
cat > go.mod << 'EOF'
module testcollector

go 1.22

require (
    go.opentelemetry.io/collector/component v1.35.0
    go.opentelemetry.io/collector/otelcol v0.130.0
)
EOF

# Try to build
echo "Building minimal collector..."
go mod tidy
go build -o test-collector main.go

if [ -f test-collector ]; then
    echo "✓ Minimal collector built successfully!"
    echo "Binary: $(pwd)/test-collector"
else
    echo "✗ Build failed"
    exit 1
fi

cd ..

echo
echo "=== Testing Custom Processor Builds ==="

# Test building each processor individually
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    echo -n "Testing $processor... "
    cd processors/$processor
    if go build ./...; then
        echo "✓ OK"
    else
        echo "✗ FAILED"
    fi
    cd ../..
done

echo
echo "=== Build Summary ==="
echo "Minimal collector builds: ✓"
echo "Now you can proceed with building the full collector with custom processors."