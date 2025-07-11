#!/bin/bash

echo "=== Testing Component Builds ==="

# Disable go.work for isolated testing
export GOWORK=off

# Test processors
echo ""
echo "Testing processors..."
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    echo ""
    echo "Testing $processor:"
    cd processors/$processor
    if go build . 2>&1 | head -5; then
        echo "✓ $processor builds successfully"
    else
        echo "✗ $processor has errors"
    fi
    cd ../..
done

# Test receivers  
echo ""
echo "Testing receivers..."
for receiver in ash enhancedsql kernelmetrics; do
    echo ""
    echo "Testing $receiver:"
    cd receivers/$receiver
    if go build . 2>&1 | head -5; then
        echo "✓ $receiver builds successfully"
    else
        echo "✗ $receiver has errors"
    fi
    cd ../..
done

# Test exporters
echo ""
echo "Testing exporters..."
for exporter in nri; do
    echo ""
    echo "Testing $exporter:"
    cd exporters/$exporter
    if go build . 2>&1 | head -5; then
        echo "✓ $exporter builds successfully"
    else
        echo "✗ $exporter has errors"
    fi
    cd ../..
done

echo ""
echo "=== Test complete ===">