#!/bin/bash

# Verify fixes in current implementation

echo "=== MODULE PATH CHECK ==="
find . -name "go.mod" -exec grep -H "^module" {} \; | grep -v reference-distribution | head -10

echo -e "\n=== VERSION CONSISTENCY CHECK ==="
echo "Checking for mixed versions..."
for mod in processors/adaptivesampler core distributions/production; do
    if [ -f "$mod/go.mod" ]; then
        echo -e "\n$mod:"
        grep -E "go.opentelemetry.io/collector/[^ ]+ v" "$mod/go.mod" | head -5
    fi
done

echo -e "\n=== DIRECT CONFMAP IMPORTS CHECK ==="
echo "Checking for direct confmap imports in business logic..."
find processors receivers -name "*.go" -type f | xargs grep -l "confmap" | grep -v test | head -10
