#!/bin/bash
# Script to fix cross-references in documentation

set -e

echo "=== Fixing Cross-References in Documentation ==="

# Fix references in docs/reference/ARCHITECTURE.md
echo "Fixing references in ARCHITECTURE.md..."
sed -i '' 's|\[Quick Start\](QUICK_START.md)|[Quick Start](../guides/QUICK_START.md)|g' docs/reference/ARCHITECTURE.md
sed -i '' 's|\[Configuration Guide\](CONFIGURATION.md)|[Configuration Guide](../guides/CONFIGURATION.md)|g' docs/reference/ARCHITECTURE.md
sed -i '' 's|\[Deployment Guide\](DEPLOYMENT.md)|[Deployment Guide](../guides/DEPLOYMENT.md)|g' docs/reference/ARCHITECTURE.md
sed -i '' 's|\[Testing Guide\](TESTING.md)|[Testing Guide](../development/TESTING.md)|g' docs/reference/ARCHITECTURE.md
sed -i '' 's|\[Troubleshooting\](TROUBLESHOOTING.md)|[Troubleshooting](../guides/TROUBLESHOOTING.md)|g' docs/reference/ARCHITECTURE.md

# Fix references in archive docs that point to moved files
echo "Fixing references in archive docs..."

# Fix config file references
find docs/archive -name "*.md" -type f | while read file; do
    # Update config/ references to configs/
    sed -i '' 's|config/otelcol\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|config/collector-dev\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|config/collector-production\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|config/collector-minimal\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|configs/collector-with-secrets\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|configs/collector-test\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
done

# Fix documentation references in main guides
echo "Fixing references in main guides..."

# Update README.md references
if [ -f "README.md" ]; then
    sed -i '' 's|docs/QUICK_START\.md|docs/guides/QUICK_START.md|g' README.md
    sed -i '' 's|docs/ARCHITECTURE\.md|docs/reference/ARCHITECTURE.md|g' README.md
    sed -i '' 's|docs/CONFIGURATION\.md|docs/guides/CONFIGURATION.md|g' README.md
    sed -i '' 's|docs/DEPLOYMENT\.md|docs/guides/DEPLOYMENT.md|g' README.md
    sed -i '' 's|docs/TESTING\.md|docs/development/TESTING.md|g' README.md
    sed -i '' 's|docs/TROUBLESHOOTING\.md|docs/guides/TROUBLESHOOTING.md|g' README.md
fi

# Fix references between guide documents
find docs/guides -name "*.md" -type f | while read file; do
    # Fix internal guide references
    sed -i '' 's|\./CONFIGURATION\.md|./CONFIGURATION.md|g' "$file"
    sed -i '' 's|\./DEPLOYMENT\.md|./DEPLOYMENT.md|g' "$file"
    sed -i '' 's|\./TROUBLESHOOTING\.md|./TROUBLESHOOTING.md|g' "$file"
    
    # Fix references to reference docs
    sed -i '' 's|\.\./ARCHITECTURE\.md|../reference/ARCHITECTURE.md|g' "$file"
    sed -i '' 's|\.\./METRICS\.md|../reference/METRICS.md|g' "$file"
done

# Update config file references to use new standardized names
echo "Updating config file references..."
find docs -name "*.md" -type f | while read file; do
    # Update to new config names
    sed -i '' 's|configs/postgresql-config\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|configs/mysql-config\.yaml|configs/mysql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|configs/mongodb-config\.yaml|configs/mongodb-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|configs/mssql-config\.yaml|configs/mssql-maximum-extraction.yaml|g' "$file"
    sed -i '' 's|configs/oracle-config\.yaml|configs/oracle-maximum-extraction.yaml|g' "$file"
done

echo "=== Cross-reference fixes completed ==="