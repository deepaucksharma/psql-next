#!/bin/bash

# Build a working collector with the latest stable versions
set -e

echo "=== Building Working Database Intelligence Collector ==="
echo

# Create a new distribution directory
mkdir -p distributions/working
cd distributions/working

# Create go.mod with latest stable versions
cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/working

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/confmap v0.105.0
    go.opentelemetry.io/collector/consumer v0.105.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
    go.opentelemetry.io/collector/extension v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/pdata v1.12.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
    go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.105.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.105.0
)

// Add custom processors
require (
    github.com/database-intelligence/processors/adaptivesampler v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/circuitbreaker v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/costcontrol v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/planattributeextractor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/querycorrelator v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/verification v0.0.0-00010101000000-000000000000
)

// Replace directives for local modules
replace (
    github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
    github.com/database-intelligence/processors/verification => ../../processors/verification
    github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
    github.com/database-intelligence/common/metrics => ../../common/metrics
    github.com/database-intelligence/common/newrelic => ../../common/newrelic
    github.com/database-intelligence/common/utils => ../../common/utils
    github.com/database-intelligence/common/telemetry => ../../common/telemetry
    github.com/database-intelligence/common/config => ../../common/config
    github.com/database-intelligence/core/piidetection => ../../core/piidetection
    github.com/database-intelligence/core/dataanonymizer => ../../core/dataanonymizer
    github.com/database-intelligence/core/querylens => ../../core/querylens
)
EOF

# Create main.go with all components
cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/exporter/otlpexporter"
    "go.opentelemetry.io/collector/exporter/otlphttpexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
    // Contrib components
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    
    // Custom processors
    "github.com/database-intelligence/processors/adaptivesampler"
    "github.com/database-intelligence/processors/circuitbreaker"
    "github.com/database-intelligence/processors/costcontrol"
    "github.com/database-intelligence/processors/nrerrormonitor"
    "github.com/database-intelligence/processors/planattributeextractor"
    "github.com/database-intelligence/processors/querycorrelator"
    "github.com/database-intelligence/processors/verification"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-collector",
        Description: "Database Intelligence Collector with custom processors",
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
    
    // Extensions
    factories.Extensions = map[component.Type]extension.Factory{
        healthcheckextension.NewFactory().Type(): healthcheckextension.NewFactory(),
    }
    
    // Receivers
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type():        otlpreceiver.NewFactory(),
        postgresqlreceiver.NewFactory().Type():  postgresqlreceiver.NewFactory(),
        mysqlreceiver.NewFactory().Type():       mysqlreceiver.NewFactory(),
    }
    
    // Processors
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():           batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory().Type():   memorylimiterprocessor.NewFactory(),
        // Custom processors
        adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
        costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
        nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
        querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
        verification.NewFactory().Type():           verification.NewFactory(),
    }
    
    // Exporters
    factories.Exporters = map[component.Type]exporter.Factory{
        debugexporter.NewFactory().Type():       debugexporter.NewFactory(),
        otlpexporter.NewFactory().Type():        otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():    otlphttpexporter.NewFactory(),
    }
    
    if err := factories.Validate(); err != nil {
        return otelcol.Factories{}, err
    }
    
    return factories, nil
}
EOF

# First, ensure all custom processors are using v0.105.0
echo "Updating custom processors to use v0.105.0..."
cd ../..
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    echo "Updating $processor..."
    cd processors/$processor
    go get go.opentelemetry.io/collector/component@v0.105.0
    go get go.opentelemetry.io/collector/consumer@v0.105.0
    go get go.opentelemetry.io/collector/processor@v0.105.0
    go get go.opentelemetry.io/collector/pdata@v1.12.0
    go mod tidy
    cd ../..
done

# Update common modules too
for module in featuredetector metrics newrelic utils telemetry config; do
    if [ -d "common/$module" ]; then
        echo "Updating common/$module..."
        cd common/$module
        go get go.opentelemetry.io/collector/pdata@v1.12.0
        go get go.opentelemetry.io/collector/component@v0.105.0
        go mod tidy
        cd ../..
    fi
done

# Now build the distribution
cd distributions/working
echo
echo "Building the collector..."
go mod tidy
go build -o database-intelligence-collector .

if [ -f database-intelligence-collector ]; then
    echo
    echo "=== SUCCESS! ==="
    echo "Collector built successfully!"
    echo
    echo "Binary: $(pwd)/database-intelligence-collector"
    echo
    echo "To run the collector:"
    echo "  ./database-intelligence-collector --config=../../configs/unified/database-intelligence-complete.yaml"
    echo
    
    # Create a simple test config
    cat > test-config.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
  adaptivesampler:
    initial_sampling_rate: 100
    min_sampling_rate: 10
    max_sampling_rate: 100

exporters:
  debug:
    verbosity: detailed

extensions:
  health_check:

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, adaptivesampler]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
EOF
    
    echo "Test configuration created: test-config.yaml"
    echo
    echo "To test with debug output:"
    echo "  ./database-intelligence-collector --config=test-config.yaml"
else
    echo "Build failed"
    exit 1
fi