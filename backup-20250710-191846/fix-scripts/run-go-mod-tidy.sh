#!/bin/bash

# Script to run go mod tidy in all modules
set -e

echo "=== Running go mod tidy in all modules ==="
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to run go mod tidy and report status
run_tidy() {
    local dir=$1
    local module_name=$(basename "$dir")
    
    echo -n "Processing $module_name... "
    cd "$dir"
    
    if go mod tidy 2>/tmp/gomod_${module_name}.err; then
        echo -e "${GREEN}✓${NC}"
        rm -f /tmp/gomod_${module_name}.err
    else
        echo -e "${RED}✗${NC}"
        echo "  Error output:"
        cat /tmp/gomod_${module_name}.err | sed 's/^/    /'
        rm -f /tmp/gomod_${module_name}.err
    fi
    
    cd - > /dev/null
}

# Start from the project root
cd /Users/deepaksharma/syc/db-otel/database-intelligence-restructured

# Process common modules first
echo "=== Processing common modules ==="
for dir in common/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing core modules ==="
for dir in core/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing processor modules ==="
for dir in processors/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing receiver modules ==="
for dir in receivers/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing exporter modules ==="
for dir in exporters/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing extension modules ==="
for dir in extensions/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing distribution modules ==="
for dir in distributions/*/; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Processing test modules ==="
for dir in tests tests/e2e tests/integration; do
    if [ -f "$dir/go.mod" ]; then
        run_tidy "$dir"
    fi
done

echo
echo "=== Summary ==="
echo "All modules have been processed."
echo
echo "Next step: Build the enterprise distribution"
echo "  cd distributions/enterprise"
echo "  go build -o database-intelligence-collector ./main.go"