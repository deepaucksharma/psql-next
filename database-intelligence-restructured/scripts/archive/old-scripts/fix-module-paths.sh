#!/bin/bash

# Script to fix all module path references from github.com/deepaksharma/db-otel to github.com/newrelic/database-intelligence

echo "Starting module path fixes..."

# Define the old and new module paths
OLD_MODULE="github.com/deepaksharma/db-otel"
NEW_MODULE="github.com/newrelic/database-intelligence"

# Fix go.mod files
echo "Fixing go.mod files..."
find . -name "go.mod" -type f | while read -r file; do
    echo "Processing: $file"
    sed -i.bak "s|$OLD_MODULE|$NEW_MODULE|g" "$file"
done

# Fix go.work file
echo "Fixing go.work file..."
if [ -f "go.work" ]; then
    sed -i.bak "s|$OLD_MODULE|$NEW_MODULE|g" "go.work"
fi

# Fix all .go source files
echo "Fixing .go source files..."
find . -name "*.go" -type f | while read -r file; do
    if grep -q "$OLD_MODULE" "$file"; then
        echo "Processing: $file"
        sed -i.bak "s|$OLD_MODULE|$NEW_MODULE|g" "$file"
    fi
done

# Clean up backup files (optional - uncomment if you want to remove them)
# find . -name "*.bak" -type f -delete

echo "Module path fixes completed!"
echo ""
echo "Summary of changes:"
echo "- Replaced: $OLD_MODULE"
echo "- With: $NEW_MODULE"
echo ""
echo "Backup files created with .bak extension"
echo "Run 'find . -name \"*.bak\" -type f -delete' to remove backups after verification"