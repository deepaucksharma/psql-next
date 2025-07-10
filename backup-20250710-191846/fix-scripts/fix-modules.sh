#!/bin/bash
# Fix all go.mod files to use correct module paths

set -e

echo "Fixing all go.mod files..."

# Find all go.mod files and update module paths
find . -name "go.mod" -type f | while read gomod; do
    echo "Updating $gomod"
    
    # Update module declarations
    sed -i.bak \
        -e 's|github.com/database-intelligence-mvp|github.com/database-intelligence|g' \
        "$gomod"
    
    # Remove backup files
    rm -f "${gomod}.bak"
done

echo "Updated all go.mod files"

# Now run go mod tidy for each module
echo "Running go mod tidy for all modules..."

find . -name "go.mod" -type f | while read gomod; do
    dir=$(dirname "$gomod")
    echo "Tidying $dir"
    (cd "$dir" && go mod tidy) || echo "Warning: failed to tidy $dir"
done

echo "Done!"