#!/bin/bash

echo "Fixing module path inconsistencies..."

# Fix YAML files
find . -name "*.yaml" -type f -exec sed -i '' \
  's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' {} \;

# Fix Go files (if any exist)  
find . -name "*.go" -type f -exec sed -i '' \
  's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' {} \;

echo "Checking for remaining inconsistencies..."
INCONSISTENCIES=$(grep -r "github.com/newrelic" . --include="*.yaml" --include="*.go" 2>/dev/null || true)

if [ -z "$INCONSISTENCIES" ]; then
  echo "✅ All module paths fixed successfully"
else
  echo "⚠️  Some inconsistencies remain:"
  echo "$INCONSISTENCIES"
fi