#!/bin/bash

# Fix Go module version conflicts across the workspace
set -e

echo "Fixing Go module version conflicts..."

# Target version for OpenTelemetry components
OTEL_VERSION="v0.105.0"

# List of modules to update
MODULES=(
    "common"
    "common/featuredetector"
    "common/queryselector"
    "core"
    "distributions/enterprise"
    "distributions/minimal"
    "distributions/production"
    "distributions/standard"
    "distributions/streamlined"
    "distributions/working"
    "exporters/nri"
    "extensions/healthcheck"
    "processors/adaptivesampler"
    "processors/circuitbreaker"
    "processors/costcontrol"
    "processors/nrerrormonitor"
    "processors/planattributeextractor"
    "processors/querycorrelator"
    "processors/verification"
    "tests"
    "tests/e2e"
    "tests/integration"
    "tests/test-collector"
)

# Function to update a module
update_module() {
    local module=$1
    echo "Updating $module..."
    
    if [ -f "$module/go.mod" ]; then
        cd "$module"
        
        # Remove go.sum to force fresh resolution
        rm -f go.sum
        
        # Update all OpenTelemetry dependencies to target version
        go get -u go.opentelemetry.io/collector/component@${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/confmap@${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/consumer@${OTEL_VERSION}
        go get -u go.opentelemetry.io/collector/pdata@v1.12.0
        go get -u go.opentelemetry.io/collector/semconv@${OTEL_VERSION}
        
        # Update contrib components if used
        if grep -q "opentelemetry-collector-contrib" go.mod; then
            go get -u github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension@${OTEL_VERSION}
            go get -u github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver@${OTEL_VERSION}
            go get -u github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver@${OTEL_VERSION}
        fi
        
        # Tidy the module
        go mod tidy
        
        cd - > /dev/null
        echo "✓ Updated $module"
    else
        echo "⚠ Skipping $module - no go.mod found"
    fi
}

# Update each module
for module in "${MODULES[@]}"; do
    update_module "$module"
done

echo ""
echo "Version update complete!"
echo "Now attempting to build a test collector..."

# Try building the production distribution
cd distributions/production
go build -o database-intelligence .
if [ $? -eq 0 ]; then
    echo "✓ Production distribution built successfully!"
else
    echo "✗ Build failed - manual intervention may be required"
fi