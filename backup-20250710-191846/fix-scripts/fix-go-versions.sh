#!/bin/bash
# Fix Go version in all go.mod files

set -e

echo "Fixing Go versions in all go.mod files..."

# Find all go.mod files and replace go 1.24.3 with go 1.22
find . -name "go.mod" -type f | while read file; do
    if grep -q "go 1.24.3" "$file"; then
        echo "Fixing $file"
        sed -i.bak 's/go 1.24.3/go 1.22/g' "$file"
        rm "${file}.bak"
    fi
done

echo "Go versions fixed successfully!"