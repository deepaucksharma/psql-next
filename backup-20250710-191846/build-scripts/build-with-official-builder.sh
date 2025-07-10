#!/bin/bash

# Build using the official OpenTelemetry Collector Builder
set -e

echo "=== Building with Official OpenTelemetry Collector Builder ==="
echo

# Install the latest builder if not already installed
BUILDER_VERSION="0.105.0"
if ! ~/go/bin/builder version 2>/dev/null | grep -q "$BUILDER_VERSION"; then
    echo "Installing OpenTelemetry Collector Builder v$BUILDER_VERSION..."
    go install go.opentelemetry.io/collector/cmd/builder@v$BUILDER_VERSION
fi

# Create builder configuration that works
cat > builder-config-official.yaml << 'EOF'
dist:
  name: database-intelligence
  description: Database Intelligence Collector with custom processors
  version: 2.0.0
  output_path: ./build-official
  otelcol_version: 0.105.0

extensions:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.105.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
  - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.105.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.105.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.105.0
EOF

echo "Building standard collector first..."
if ~/go/bin/builder --config=builder-config-official.yaml; then
    echo
    echo "=== Standard Collector Built Successfully! ==="
    echo
    
    # Now try adding custom processors one by one
    echo "Adding custom processors..."
    
    cat > builder-config-with-processors.yaml << 'EOF'
dist:
  name: database-intelligence-full
  description: Database Intelligence Collector with all custom processors
  version: 2.0.0
  output_path: ./build-full
  otelcol_version: 0.105.0

extensions:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.105.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
  - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.105.0
  # Add custom processors as local paths
  - path: ./processors/adaptivesampler
  - path: ./processors/circuitbreaker
  - path: ./processors/costcontrol
  - path: ./processors/nrerrormonitor
  - path: ./processors/planattributeextractor
  - path: ./processors/querycorrelator
  - path: ./processors/verification

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.105.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.105.0

replaces:
  - github.com/database-intelligence/common/featuredetector => ./common/featuredetector
  - github.com/database-intelligence/common/metrics => ./common/metrics
  - github.com/database-intelligence/common/newrelic => ./common/newrelic
  - github.com/database-intelligence/common/utils => ./common/utils
  - github.com/database-intelligence/common/telemetry => ./common/telemetry
  - github.com/database-intelligence/common/config => ./common/config
  - github.com/database-intelligence/core/piidetection => ./core/piidetection
  - github.com/database-intelligence/core/dataanonymizer => ./core/dataanonymizer
  - github.com/database-intelligence/core/querylens => ./core/querylens
EOF
    
    echo "Building full collector with custom processors..."
    if ~/go/bin/builder --config=builder-config-with-processors.yaml; then
        echo
        echo "=== FULL BUILD SUCCESSFUL! ==="
        echo "Standard collector: ./build-official/database-intelligence"
        echo "Full collector: ./build-full/database-intelligence-full"
        echo
        echo "To run the full collector:"
        echo "  ./build-full/database-intelligence-full --config=configs/unified/database-intelligence-complete.yaml"
    else
        echo "Failed to build with custom processors. The standard collector is available in ./build-official/"
        echo
        echo "Manual integration required for custom processors."
    fi
else
    echo "Failed to build standard collector"
    exit 1
fi