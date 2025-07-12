#!/bin/bash

echo "=== Building Production Collector with All Custom Components ==="

cd distributions/production

# Clean up duplicate files
echo "Cleaning up duplicate files..."
rm -f components_minimal.go components_with_custom.go

# Create complete components.go with all custom components
echo "Creating components.go with all custom components..."

cat > components.go << 'EOF'
// Package main provides complete component factories for the Database Intelligence Collector
package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	
	// Custom processors
	"github.com/database-intelligence/processors/adaptivesampler"
	"github.com/database-intelligence/processors/circuitbreaker"
	"github.com/database-intelligence/processors/costcontrol"
	"github.com/database-intelligence/processors/nrerrormonitor"
	"github.com/database-intelligence/processors/planattributeextractor"
	"github.com/database-intelligence/processors/querycorrelator"
	"github.com/database-intelligence/processors/verification"
	
	// Custom receivers
	"github.com/database-intelligence/receivers/ash"
	"github.com/database-intelligence/receivers/enhancedsql"
	"github.com/database-intelligence/receivers/kernelmetrics"
	
	// Custom exporters
	"github.com/database-intelligence/exporters/nri"
)

// componentsComplete provides all available components
var componentsComplete = otelcol.Factories{
	Receivers: map[component.Type]receiver.Factory{
		// Core receivers
		component.MustNewType("otlp"): otlpreceiver.NewFactory(),
		
		// Custom receivers
		component.MustNewType("ash"):           ash.NewFactory(),
		component.MustNewType("enhancedsql"):   enhancedsql.NewFactory(),
		component.MustNewType("kernelmetrics"): kernelmetrics.NewFactory(),
	},
	Processors: map[component.Type]processor.Factory{
		// Core processors
		component.MustNewType("batch"):          batchprocessor.NewFactory(),
		component.MustNewType("memory_limiter"): memorylimiterprocessor.NewFactory(),
		
		// Custom processors
		component.MustNewType("adaptivesampler"):        adaptivesampler.NewFactory(),
		component.MustNewType("circuitbreaker"):         circuitbreaker.NewFactory(),
		component.MustNewType("costcontrol"):            costcontrol.NewFactory(),
		component.MustNewType("nrerrormonitor"):         nrerrormonitor.NewFactory(),
		component.MustNewType("planattributeextractor"): planattributeextractor.NewFactory(),
		component.MustNewType("querycorrelator"):        querycorrelator.NewFactory(),
		component.MustNewType("verification"):           verification.NewFactory(),
	},
	Exporters: map[component.Type]exporter.Factory{
		// Core exporters
		component.MustNewType("debug"): debugexporter.NewFactory(),
		component.MustNewType("otlp"):  otlpexporter.NewFactory(),
		
		// Custom exporters
		component.MustNewType("nri"): nri.NewFactory(),
	},
	Extensions: map[component.Type]extension.Factory{},
	Connectors: map[component.Type]connector.Factory{},
}
EOF

# Create go.mod with all components
echo "Creating go.mod with all components..."

cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/production

go 1.23.0

require (
	// Core OpenTelemetry components
	go.opentelemetry.io/collector/component v1.35.0
	go.opentelemetry.io/collector/confmap v1.35.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.35.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.35.0
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.35.0
	go.opentelemetry.io/collector/confmap/provider/httpsprovider v1.35.0
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.35.0
	go.opentelemetry.io/collector/connector v0.129.0
	go.opentelemetry.io/collector/exporter v0.129.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.129.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.129.0
	go.opentelemetry.io/collector/extension v1.35.0
	go.opentelemetry.io/collector/otelcol v0.129.0
	go.opentelemetry.io/collector/processor v1.35.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.129.0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.129.0
	go.opentelemetry.io/collector/receiver v1.35.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.129.0
	
	// Custom processors
	github.com/database-intelligence/processors/adaptivesampler v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/circuitbreaker v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/costcontrol v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/planattributeextractor v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/querycorrelator v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/processors/verification v0.0.0-00010101000000-000000000000
	
	// Custom receivers
	github.com/database-intelligence/receivers/ash v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/receivers/enhancedsql v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/receivers/kernelmetrics v0.0.0-00010101000000-000000000000
	
	// Custom exporters
	github.com/database-intelligence/exporters/nri v0.0.0-00010101000000-000000000000
	
	// Common modules
	github.com/database-intelligence/common/featuredetector v0.0.0-00010101000000-000000000000
	github.com/database-intelligence/common/queryselector v0.0.0-00010101000000-000000000000
)

// Replace directives for local modules
replace (
	// Custom processors
	github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
	github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
	github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
	github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
	github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
	github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
	github.com/database-intelligence/processors/verification => ../../processors/verification
	
	// Custom receivers
	github.com/database-intelligence/receivers/ash => ../../receivers/ash
	github.com/database-intelligence/receivers/enhancedsql => ../../receivers/enhancedsql
	github.com/database-intelligence/receivers/kernelmetrics => ../../receivers/kernelmetrics
	
	// Custom exporters
	github.com/database-intelligence/exporters/nri => ../../exporters/nri
	
	// Common modules
	github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
	github.com/database-intelligence/common/queryselector => ../../common/queryselector
	
	// Internal modules
	github.com/database-intelligence/internal/database => ../../internal/database
)
EOF

# Clear go.sum and build
echo ""
echo "Building complete collector..."
rm -f go.sum
export GOWORK=off

if go mod tidy && go build -o otelcol-complete .; then
    echo ""
    echo "✓ Build successful!"
    ls -lh otelcol-complete
    
    # Show components
    echo ""
    echo "Available components:"
    ./otelcol-complete components 2>&1 | grep -E "(receivers:|processors:|exporters:)" -A 15 | head -50
    
    # Test with config
    echo ""
    echo "Validating with test config..."
    ./otelcol-complete validate --config=test-minimal.yaml
else
    echo "✗ Build failed"
    echo "Errors:"
    go build . 2>&1 | head -20
fi

cd ../..

echo ""
echo "=== Build complete ==="