#!/bin/bash

# Final working solution - align everything to v0.105.0 which is known stable
set -e

echo "=== Final Working Solution ==="
echo "Aligning all components to OpenTelemetry v0.105.0"
echo

# Update all processor modules to v0.105.0
echo "Updating processor modules..."
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    cat > processors/$processor/go.mod << EOF
module github.com/database-intelligence/processors/$processor

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/consumer v0.105.0
    go.opentelemetry.io/collector/pdata v1.12.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.uber.org/zap v1.27.0
    github.com/stretchr/testify v1.9.0
)
EOF

    # Add specific requirements
    case $processor in
        "adaptivesampler")
            echo "" >> processors/$processor/go.mod
            echo "require (" >> processors/$processor/go.mod
            echo "    github.com/go-redis/redis/v8 v8.11.5" >> processors/$processor/go.mod
            echo "    github.com/hashicorp/golang-lru/v2 v2.0.7" >> processors/$processor/go.mod
            echo ")" >> processors/$processor/go.mod
            ;;
        "circuitbreaker")
            echo "" >> processors/$processor/go.mod
            echo "require github.com/database-intelligence/common/featuredetector v0.0.0-00010101000000-000000000000" >> processors/$processor/go.mod
            echo "" >> processors/$processor/go.mod
            echo "replace github.com/database-intelligence/common/featuredetector => ../../common/featuredetector" >> processors/$processor/go.mod
            ;;
        "planattributeextractor")
            echo "" >> processors/$processor/go.mod
            echo "require github.com/tidwall/gjson v1.17.0" >> processors/$processor/go.mod
            ;;
    esac
done

# Create working enterprise distribution
echo -e "\nCreating enterprise distribution..."
cat > distributions/enterprise/go.mod << 'EOF'
module github.com/database-intelligence/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.105.0
    go.opentelemetry.io/collector/confmap v1.12.0
    go.opentelemetry.io/collector/exporter v0.105.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.105.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.105.0
    go.opentelemetry.io/collector/extension v0.105.0
    go.opentelemetry.io/collector/otelcol v0.105.0
    go.opentelemetry.io/collector/processor v0.105.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.105.0
    go.opentelemetry.io/collector/receiver v0.105.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.105.0
    
    // Custom processors only - no contrib to avoid conflicts
    github.com/database-intelligence/processors/adaptivesampler v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/circuitbreaker v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/costcontrol v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/planattributeextractor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/querycorrelator v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/verification v0.0.0-00010101000000-000000000000
)

replace (
    github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
    github.com/database-intelligence/processors/verification => ../../processors/verification
    
    github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
)
EOF

# Update main.go to remove contrib dependencies
cat > distributions/enterprise/main.go << 'EOF'
package main

import (
    "fmt"
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
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
    // Import custom processors
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
        log.Fatalf("failed to build components: %v", err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-collector",
        Description: "Database Intelligence Collector with Custom Processors",
        Version:     "2.0.0",
    }

    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    factories := otelcol.Factories{}

    // Receivers - core only
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type(): otlpreceiver.NewFactory(),
    }

    // Processors - core + custom
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():         batchprocessor.NewFactory(),
        // Custom processors
        adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
        costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
        nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
        planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
        querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
        verification.NewFactory().Type():           verification.NewFactory(),
    }

    // Exporters - core only  
    factories.Exporters = map[component.Type]exporter.Factory{
        otlpexporter.NewFactory().Type():      otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():  otlphttpexporter.NewFactory(),
        debugexporter.NewFactory().Type():     debugexporter.NewFactory(),
    }

    // Extensions - empty for now
    factories.Extensions = map[component.Type]extension.Factory{}

    // Initialize empty connectors map
    factories.Connectors = make(map[component.Type]component.Factory)

    return factories, nil
}

func run(settings otelcol.CollectorSettings) error {
    cmd := otelcol.NewCommand(settings)
    if err := cmd.Execute(); err != nil {
        return fmt.Errorf("collector server run finished with error: %w", err)
    }
    
    return nil
}
EOF

# Build the collector
echo -e "\nBuilding collector..."
cd distributions/enterprise
go mod tidy
go build -o database-intelligence-collector ./main.go

if [ -f database-intelligence-collector ]; then
    echo
    echo "=== SUCCESS! ==="
    echo "Database Intelligence Collector built successfully!"
    echo
    echo "Binary: $(pwd)/database-intelligence-collector"
    echo
    echo "This collector includes:"
    echo "  - All 7 custom processors"
    echo "  - OTLP receiver"
    echo "  - OTLP/HTTP exporter for New Relic"
    echo "  - Debug exporter for testing"
    echo
    echo "To add database receivers (MySQL, PostgreSQL), use the OpenTelemetry Collector Builder"
    echo "with the builder-config.yaml provided."
else
    echo "Build failed"
fi