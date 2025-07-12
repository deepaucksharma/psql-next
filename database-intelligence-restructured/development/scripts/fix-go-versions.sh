#!/bin/bash
# Fix Go versions across all go.mod and go.work files

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Standard Go version to use
GO_VERSION="1.22"

echo "Fixing Go versions to $GO_VERSION across all modules..."

# Fix go.work file
if [[ -f "$PROJECT_ROOT/go.work" ]]; then
    echo "Fixing go.work..."
    sed -i "s/^go .*/go $GO_VERSION/" "$PROJECT_ROOT/go.work"
fi

# Fix all go.mod files
find "$PROJECT_ROOT" -name "go.mod" -type f | grep -v archive | while read -r modfile; do
    echo "Fixing $modfile..."
    sed -i "s/^go .*/go $GO_VERSION/" "$modfile"
done

echo "✅ Go versions fixed to $GO_VERSION"

# Verify the changes
echo ""
echo "Verification:"
find "$PROJECT_ROOT" -name "go.mod" -type f | grep -v archive | xargs grep "^go " | sort | uniq