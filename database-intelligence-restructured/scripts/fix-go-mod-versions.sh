#!/bin/bash
# Script to fix inconsistent OpenTelemetry versions across go.mod files

set -e

# Target version for OpenTelemetry components
OTEL_VERSION="v0.105.0"
PDATA_VERSION="v1.12.0"

echo "Fixing OpenTelemetry version inconsistencies..."
echo "Target OTel version: $OTEL_VERSION"
echo "Target pdata version: $PDATA_VERSION"

# Find all go.mod files excluding archive
GO_MOD_FILES=$(find . -name "go.mod" -type f | grep -v "/archive/" | grep -v "/vendor/")

for mod_file in $GO_MOD_FILES; do
    echo "Processing: $mod_file"
    
    # Backup original
    cp "$mod_file" "${mod_file}.backup"
    
    # Fix OpenTelemetry collector versions
    sed -i "s|go.opentelemetry.io/collector/component v[0-9.]*|go.opentelemetry.io/collector/component $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector/confmap v[0-9.]*|go.opentelemetry.io/collector/confmap $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector/consumer v[0-9.]*|go.opentelemetry.io/collector/consumer $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector/exporter v[0-9.]*|go.opentelemetry.io/collector/exporter $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector/extension v[0-9.]*|go.opentelemetry.io/collector/extension $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector/processor v[0-9.]*|go.opentelemetry.io/collector/processor $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector/receiver v[0-9.]*|go.opentelemetry.io/collector/receiver $OTEL_VERSION|g" "$mod_file"
    sed -i "s|go.opentelemetry.io/collector v[0-9.]*|go.opentelemetry.io/collector $OTEL_VERSION|g" "$mod_file"
    
    # Fix pdata version
    sed -i "s|go.opentelemetry.io/collector/pdata v[0-9.]*|go.opentelemetry.io/collector/pdata $PDATA_VERSION|g" "$mod_file"
    
    # Fix config/configretry version
    sed -i "s|go.opentelemetry.io/collector/config/configretry v[0-9.]*|go.opentelemetry.io/collector/config/configretry $OTEL_VERSION|g" "$mod_file"
    
    # Check if changes were made
    if ! diff -q "$mod_file" "${mod_file}.backup" > /dev/null; then
        echo "  - Updated versions in $mod_file"
        rm "${mod_file}.backup"
    else
        echo "  - No changes needed in $mod_file"
        rm "${mod_file}.backup"
    fi
done

echo ""
echo "Running go mod tidy on all modules..."

for mod_file in $GO_MOD_FILES; do
    mod_dir=$(dirname "$mod_file")
    echo "Running go mod tidy in $mod_dir"
    (cd "$mod_dir" && go mod tidy) || echo "  - Warning: go mod tidy failed in $mod_dir"
done

echo ""
echo "Version fix complete!"
echo "You may need to run 'go mod download' in each module directory."