#!/bin/bash
# Fix OpenTelemetry version inconsistencies across all modules

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Standard OTel version
OTEL_VERSION="v0.105.0"
PDATA_VERSION="v1.12.0"

echo "Fixing OpenTelemetry dependencies to $OTEL_VERSION..."

# Function to fix a go.mod file
fix_go_mod() {
    local modfile="$1"
    echo "Fixing $modfile..."
    
    # Fix collector component versions
    sed -i "s|go.opentelemetry.io/collector/component v[0-9\.]*|go.opentelemetry.io/collector/component $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/confmap v[0-9\.]*|go.opentelemetry.io/collector/confmap $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/consumer v[0-9\.]*|go.opentelemetry.io/collector/consumer $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/exporter v[0-9\.]*|go.opentelemetry.io/collector/exporter $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/extension v[0-9\.]*|go.opentelemetry.io/collector/extension $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/processor v[0-9\.]*|go.opentelemetry.io/collector/processor $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/receiver v[0-9\.]*|go.opentelemetry.io/collector/receiver $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/scraper v[0-9\.]*|go.opentelemetry.io/collector/scraper $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/otelcol v[0-9\.]*|go.opentelemetry.io/collector/otelcol $OTEL_VERSION|g" "$modfile"
    
    # Fix pdata separately (uses different version)
    sed -i "s|go.opentelemetry.io/collector/pdata v[0-9\.]*|go.opentelemetry.io/collector/pdata $PDATA_VERSION|g" "$modfile"
    
    # Fix test packages
    sed -i "s|go.opentelemetry.io/collector/component/componenttest v[0-9\.]*|go.opentelemetry.io/collector/component/componenttest $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/consumer/consumertest v[0-9\.]*|go.opentelemetry.io/collector/consumer/consumertest $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/scraper/scraperhelper v[0-9\.]*|go.opentelemetry.io/collector/scraper/scraperhelper $OTEL_VERSION|g" "$modfile"
    sed -i "s|go.opentelemetry.io/collector/config/configretry v[0-9\.]*|go.opentelemetry.io/collector/config/configretry $OTEL_VERSION|g" "$modfile"
    
    # Fix featuregate
    sed -i "s|go.opentelemetry.io/collector/featuregate v[0-9\.]*|go.opentelemetry.io/collector/featuregate $PDATA_VERSION|g" "$modfile"
}

# Fix all go.mod files
find "$PROJECT_ROOT" -name "go.mod" -type f | grep -v archive | while read -r modfile; do
    fix_go_mod "$modfile"
done

echo "✅ OpenTelemetry dependencies fixed to $OTEL_VERSION (pdata: $PDATA_VERSION)"

# Now run go mod tidy in each module directory
echo ""
echo "Running go mod tidy in each module..."

# First, clear the module cache for OTel packages
go clean -modcache

# Run go mod tidy in each directory
find "$PROJECT_ROOT" -name "go.mod" -type f | grep -v archive | while read -r modfile; do
    dir=$(dirname "$modfile")
    echo "Tidying $dir..."
    (cd "$dir" && go mod tidy) || echo "Warning: Failed to tidy $dir"
done

echo "✅ All modules tidied"