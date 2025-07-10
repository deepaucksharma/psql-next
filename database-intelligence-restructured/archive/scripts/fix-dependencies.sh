#!/bin/bash
# Consolidated dependency fix script

set -e

echo "Fixing Go module dependencies..."

# Update all modules to use consistent OTEL versions
OTEL_VERSION="v0.129.0"

# Find all go.mod files and update dependencies
find . -name "go.mod" -not -path "./backup*" | while read -r modfile; do
    dir=$(dirname "$modfile")
    echo "Updating $dir..."
    cd "$dir"
    go mod tidy
    cd - > /dev/null
done

echo "Dependencies fixed!"
