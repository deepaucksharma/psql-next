#!/bin/bash

# Script to run go mod tidy in all module directories after module path fix

echo "Updating all Go modules..."
echo "========================="
echo ""

# Find all directories containing go.mod files
find . -name "go.mod" -type f ! -path "./archive/*" ! -path "./.module-path-backup*/*" | while read -r modfile; do
    dir=$(dirname "$modfile")
    echo "Processing module in: $dir"
    
    # Change to the module directory
    cd "$dir" || continue
    
    # Run go mod tidy
    echo "  Running: go mod tidy"
    go mod tidy
    
    # Check if successful
    if [ $? -eq 0 ]; then
        echo "  ✓ Success"
    else
        echo "  ✗ Failed - check errors above"
    fi
    
    # Return to original directory
    cd - > /dev/null
    echo ""
done

echo "Module updates complete!"
echo ""
echo "Next steps:"
echo "1. Check for any errors above"
echo "2. Run 'make build' to verify everything compiles"
echo "3. Commit the changes if successful"