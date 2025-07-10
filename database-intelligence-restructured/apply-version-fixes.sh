#!/bin/bash

# Apply version fixes based on clean reference pattern
# THIS SCRIPT SHOWS WHAT WOULD BE CHANGED - RUN WITH 'apply' TO MAKE CHANGES

set -e

ACTION="${1:-show}"

if [ "$ACTION" = "show" ]; then
    echo "=== DRY RUN MODE ==="
    echo "This will show what changes would be made."
    echo "Run with './apply-version-fixes.sh apply' to make changes"
    echo ""
fi

# Function to update a module
update_module() {
    local module_path=$1
    local module_name=$(basename "$module_path")
    
    echo "Module: $module_name"
    
    if [ "$ACTION" = "apply" ]; then
        cd "$module_path"
        
        # Update to v1.35.0 pattern
        go get -u go.opentelemetry.io/collector/component@v1.35.0
        go get -u go.opentelemetry.io/collector/confmap@v1.35.0
        go get -u go.opentelemetry.io/collector/consumer@v1.35.0
        go get -u go.opentelemetry.io/collector/pdata@v1.35.0
        
        # Update specific components based on module type
        if [[ "$module_path" == *"processor"* ]]; then
            go get -u go.opentelemetry.io/collector/processor@v1.35.0
        elif [[ "$module_path" == *"receiver"* ]]; then
            go get -u go.opentelemetry.io/collector/receiver@v1.35.0
        elif [[ "$module_path" == *"exporter"* ]]; then
            go get -u go.opentelemetry.io/collector/exporter@v1.35.0
        fi
        
        go mod tidy
        cd - > /dev/null
        
        echo "  ✓ Updated"
    else
        echo "  Would update to v1.35.0/v0.129.0 pattern"
    fi
    echo ""
}

# Update processors
echo "=== PROCESSORS ==="
for proc in processors/*; do
    if [ -d "$proc" ] && [ -f "$proc/go.mod" ]; then
        update_module "$proc"
    fi
done

# Update receivers
echo "=== RECEIVERS ==="
for recv in receivers/*; do
    if [ -d "$recv" ] && [ -f "$recv/go.mod" ]; then
        update_module "$recv"
    fi
done

# Update common
echo "=== COMMON MODULES ==="
for common in common common/featuredetector common/queryselector; do
    if [ -d "$common" ] && [ -f "$common/go.mod" ]; then
        update_module "$common"
    fi
done

if [ "$ACTION" = "show" ]; then
    echo "To apply these changes, run: ./apply-version-fixes.sh apply"
fi
