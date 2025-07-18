#!/bin/bash

# Fix featuredetector imports from common to internal
files=(
    "./internal/queryselector/selector.go"
    "./components/receivers/enhancedsql/config.go"
    "./components/receivers/enhancedsql/collect.go"
    "./components/receivers/enhancedsql/receiver.go"
    "./components/processors/circuitbreaker/feature_aware.go"
)

for file in "${files[@]}"; do
    echo "Fixing $file"
    sed -i '' 's|github.com/database-intelligence/db-intel/common/featuredetector|github.com/database-intelligence/db-intel/internal/featuredetector|g' "$file"
done

# Also fix the go.mod files
echo "Fixing go.mod files..."
find . -name "go.mod" -exec sed -i '' 's|github.com/database-intelligence/db-intel/common/featuredetector|github.com/database-intelligence/db-intel/internal/featuredetector|g' {} \;

echo "Done fixing featuredetector imports"