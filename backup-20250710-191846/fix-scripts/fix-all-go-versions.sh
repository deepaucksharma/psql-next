#!/bin/bash

# Fix all Go version issues comprehensively
set -e

echo "=== Fixing All Go Version Issues ==="
echo

# Function to fix go.mod file
fix_go_mod() {
    local file=$1
    echo "Fixing $file..."
    
    # Create a temporary file with the fixed content
    awk '
    /^go / { print "go 1.22"; next }
    /^toolchain / { next }
    { print }
    ' "$file" > "$file.tmp"
    
    # Replace the original file
    mv "$file.tmp" "$file"
}

# Find and fix all go.mod files
find . -name "go.mod" -not -path "./vendor/*" -not -path "./build/*" | while read -r gomod; do
    fix_go_mod "$gomod"
done

echo
echo "All go.mod files have been fixed to use Go 1.22"
echo

# Now run go mod tidy on all modules to ensure consistency
echo "Running go mod tidy on all modules..."
find . -name "go.mod" -not -path "./vendor/*" -not -path "./build/*" | while read -r gomod; do
    dir=$(dirname "$gomod")
    echo "Tidying $dir..."
    (cd "$dir" && go mod tidy) || echo "Warning: Failed to tidy $dir"
done

echo
echo "=== Go Version Fix Complete ==="