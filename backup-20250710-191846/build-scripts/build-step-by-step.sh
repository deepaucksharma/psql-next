#!/bin/bash

# Build a working collector step by step
set -e

echo "=== Building Database Intelligence Collector Step by Step ==="
echo

# Create a clean distribution directory
rm -rf distributions/final
mkdir -p distributions/final
cd distributions/final

# Step 1: Create a minimal working collector
echo "Step 1: Creating minimal collector..."
cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/final

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
)
EOF

cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
)

func main() {
    info := component.BuildInfo{
        Command:     "database-intelligence",
        Description: "Database Intelligence Collector",
        Version:     "2.0.0",
    }

    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: func() (otelcol.Factories, error) {
            return otelcol.Factories{}, nil
        },
    }
    
    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
EOF

echo "Building minimal collector..."
go mod tidy
go build -o database-intelligence-minimal .

if [ ! -f database-intelligence-minimal ]; then
    echo "Failed to build minimal collector"
    exit 1
fi

echo "✓ Minimal collector built successfully"

# Step 2: Add OTLP receiver and debug exporter
echo
echo "Step 2: Adding OTLP receiver and debug exporter..."

cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/final

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
)
EOF

cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence",
        Description: "Database Intelligence Collector",
        Version:     "2.0.0",
    }

    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }
    
    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    factories := otelcol.Factories{}
    
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type(): otlpreceiver.NewFactory(),
    }
    
    factories.Exporters = map[component.Type]exporter.Factory{
        debugexporter.NewFactory().Type(): debugexporter.NewFactory(),
    }
    
    return factories, nil
}
EOF

go mod tidy
go build -o database-intelligence-basic .

if [ ! -f database-intelligence-basic ]; then
    echo "Failed to build basic collector"
    exit 1
fi

echo "✓ Basic collector built successfully"

# Step 3: Add New Relic exporter
echo
echo "Step 3: Adding New Relic OTLP exporters..."

cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/final

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
)
EOF

cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence",
        Description: "Database Intelligence Collector",
        Version:     "2.0.0",
    }

    set := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }
    
    cmd := otelcol.NewCommand(set)
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    factories := otelcol.Factories{}
    
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type(): otlpreceiver.NewFactory(),
    }
    
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type(): batchprocessor.NewFactory(),
    }
    
    factories.Exporters = map[component.Type]exporter.Factory{
        debugexporter.NewFactory().Type():      debugexporter.NewFactory(),
        otlpexporter.NewFactory().Type():       otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():   otlphttpexporter.NewFactory(),
    }
    
    return factories, nil
}
EOF

go mod tidy
go build -o database-intelligence-newrelic .

if [ ! -f database-intelligence-newrelic ]; then
    echo "Failed to build New Relic-ready collector"
    exit 1
fi

echo "✓ New Relic-ready collector built successfully"

# Create test configuration
cat > test-newrelic.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  debug:
    verbosity: detailed
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_API_KEY}

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, otlphttp]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, otlphttp]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, otlphttp]
EOF

echo
echo "=== BUILD SUCCESSFUL! ==="
echo
echo "Built collectors:"
echo "1. Minimal: ./database-intelligence-minimal"
echo "2. Basic: ./database-intelligence-basic"
echo "3. New Relic Ready: ./database-intelligence-newrelic"
echo
echo "Test configuration: test-newrelic.yaml"
echo
echo "To run with New Relic:"
echo "  export NEW_RELIC_API_KEY=your-api-key"
echo "  ./database-intelligence-newrelic --config=test-newrelic.yaml"
echo
echo "Next steps:"
echo "1. Test the basic functionality"
echo "2. Add database receivers (MySQL, PostgreSQL)"
echo "3. Integrate custom processors one by one"
echo "4. Create final production build"

cd ../../..