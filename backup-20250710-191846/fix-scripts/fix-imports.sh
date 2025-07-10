#!/bin/bash
# Fix import paths in source files

set -e

echo "Fixing import paths in source files..."

# Find all Go files and update import paths
find . -name "*.go" -type f | while read gofile; do
    if grep -q "github.com/database-intelligence-mvp" "$gofile"; then
        echo "Updating imports in $gofile"
        sed -i.bak \
            -e 's|github.com/database-intelligence-mvp|github.com/database-intelligence|g' \
            "$gofile"
        rm -f "${gofile}.bak"
    fi
done

echo "Updated import paths in source files"

# Now sync workspace
echo "Syncing workspace..."
go work sync || echo "Warning: workspace sync failed"

echo "Done!"