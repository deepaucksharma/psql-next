#!/bin/bash

# Use OpenTelemetry Collector Builder
set -e

echo "=== Using OpenTelemetry Collector Builder ==="
echo

# Check if builder is installed
if ! command -v builder &> /dev/null; then
    echo "Installing OpenTelemetry Collector Builder..."
    go install go.opentelemetry.io/collector/cmd/builder@v0.102.1
fi

# Update builder config to use compatible versions
cat > builder-config-v2.yaml << 'EOF'
dist:
  name: database-intelligence-collector
  description: Database Intelligence Collector with custom processors
  version: 2.0.0
  output_path: ./build
  otelcol_version: 0.102.1

extensions:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.102.1
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.102.1

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.102.1
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.102.1
  - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v0.102.1

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.102.1
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.102.1

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.102.1
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.102.1
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.102.1

replaces:
  - github.com/database-intelligence/processors/adaptivesampler => ./processors/adaptivesampler
  - github.com/database-intelligence/processors/circuitbreaker => ./processors/circuitbreaker
  - github.com/database-intelligence/processors/costcontrol => ./processors/costcontrol
  - github.com/database-intelligence/processors/nrerrormonitor => ./processors/nrerrormonitor
  - github.com/database-intelligence/processors/planattributeextractor => ./processors/planattributeextractor
  - github.com/database-intelligence/processors/querycorrelator => ./processors/querycorrelator
  - github.com/database-intelligence/processors/verification => ./processors/verification
EOF

echo "Running OpenTelemetry Collector Builder..."
if builder --config=builder-config-v2.yaml; then
    echo
    echo "=== Build Successful! ==="
    echo "Collector built in: ./build"
    echo
    cd build
    ls -la
    echo
    echo "To run the collector:"
    echo "  cd build"
    echo "  ./database-intelligence-collector --config=../configs/unified/database-intelligence-complete.yaml"
else
    echo "Builder failed. Let's try a different approach..."
    
    # Alternative: Create a working collector without custom processors first
    echo
    echo "=== Creating Standard Collector ==="
    
    mkdir -p distributions/standard-working
    cd distributions/standard-working
    
    cat > builder-standard.yaml << 'EOF'
dist:
  name: database-intelligence-standard
  description: Standard Database Intelligence Collector
  version: 1.0.0
  output_path: ./
  otelcol_version: 0.102.1

extensions:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.102.1

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.102.1
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.102.1
  - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v0.102.1

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.102.1

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.102.1
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.102.1
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.102.1
EOF
    
    if builder --config=builder-standard.yaml; then
        echo
        echo "=== Standard Collector Built Successfully! ==="
        echo "This proves the builder works."
        echo
        echo "Next step: Integrate custom processors using the builder's"
        echo "module replacement mechanism or by creating processor modules"
        echo "that match the expected version constraints."
    fi
fi