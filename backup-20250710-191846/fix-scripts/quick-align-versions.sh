#!/bin/bash

# Quick script to align OpenTelemetry versions
set -e

echo "=== Quick OpenTelemetry Version Alignment ==="
echo

# Find and update all go.mod files
find . -name "go.mod" -type f | while read -r file; do
    echo "Processing: $file"
    
    # Update core v1.x.x versions to v1.35.0
    sed -i '' -E 's|go\.opentelemetry\.io/collector/(component|processor|consumer|pdata|receiver|exporter|extension|featuregate|confmap|client) v1\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/\1 v1.35.0|g' "$file"
    
    # Update versioned v0.x.x versions to v0.130.0
    sed -i '' -E 's|go\.opentelemetry\.io/collector/[a-zA-Z/]+ v0\.[0-9]+\.[0-9]+|&|g' "$file" | grep -E "v0\.[0-9]+\.[0-9]+" | while read -r line; do
        component=$(echo "$line" | sed -E 's|.*(go\.opentelemetry\.io/collector/[a-zA-Z/]+) v0\.[0-9]+\.[0-9]+.*|\1|')
        sed -i '' -E "s|$component v0\.[0-9]+\.[0-9]+|$component v0.130.0|g" "$file"
    done
    
    # Update contrib versions to v0.130.0
    sed -i '' -E 's|github\.com/open-telemetry/opentelemetry-collector-contrib/[a-zA-Z/]+ v[0-9]+\.[0-9]+\.[0-9]+|&|g' "$file" | grep -E "v[0-9]+\.[0-9]+\.[0-9]+" | while read -r line; do
        component=$(echo "$line" | sed -E 's|.*(github\.com/open-telemetry/opentelemetry-collector-contrib/[a-zA-Z/]+) v[0-9]+\.[0-9]+\.[0-9]+.*|\1|')
        sed -i '' -E "s|$component v[0-9]+\.[0-9]+\.[0-9]+|$component v0.130.0|g" "$file"
    done
done

echo
echo "=== Version Alignment Complete ==="
echo
echo "Now running go mod tidy in all directories..."

# Run go mod tidy in each module directory
find . -name "go.mod" -type f | while read -r file; do
    dir=$(dirname "$file")
    echo "Running go mod tidy in: $dir"
    (cd "$dir" && go mod tidy) || echo "  Warning: go mod tidy failed in $dir"
done

echo
echo "Done!"