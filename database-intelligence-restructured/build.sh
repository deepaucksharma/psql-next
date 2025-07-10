#!/bin/bash
# Consolidated build script for Database Intelligence

set -e

# Build distributions
echo "Building Database Intelligence distributions..."

# Minimal distribution
echo "Building minimal distribution..."
cd distributions/minimal && go build -o ../../build/database-intelligence-minimal

# Standard distribution  
echo "Building standard distribution..."
cd ../standard && go build -o ../../build/database-intelligence-standard

# Enterprise distribution
echo "Building enterprise distribution..."
cd ../enterprise && go build -o ../../build/database-intelligence-enterprise

echo "Build complete!"
