#!/bin/bash

# Build test collector with all required components
cd ../..

# Create builder config
cat > otelcol-builder-config.yaml <<EOF
dist:
  name: e2e-test-collector
  description: E2E Test Collector with all components
  output_path: ./tests/e2e
  otelcol_version: 0.92.0

extensions:
  - gomod: go.opentelemetry.io/collector/extension/zpagesextension v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.92.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.92.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.92.0
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.92.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/loggingexporter v0.92.0
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.92.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.92.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.92.0
EOF

# Install builder if not present
if ! command -v builder &> /dev/null; then
    go install go.opentelemetry.io/collector/cmd/builder@v0.92.0
fi

# Build the collector
builder --config=otelcol-builder-config.yaml

echo "Test collector built successfully at ./tests/e2e/e2e-test-collector"