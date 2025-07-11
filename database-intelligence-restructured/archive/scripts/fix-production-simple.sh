#!/bin/bash

# Script to fix production distribution with just custom components (no contrib)

echo "=== Fixing Production Distribution (Simple Version) ==="

cd distributions/production

# First, remove any existing go.sum to start fresh
rm -f go.sum

# Create a new go.mod with only custom components
cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/production

go 1.23.0

require (
	// Core OpenTelemetry components
	go.opentelemetry.io/collector/component v1.35.0
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
	
	// Core module
	github.com/database-intelligence/core => ../../core
)
EOF

echo "Updated production go.mod with local module references (no contrib)"

# Update components.go without contrib components
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
var componentsComplete = func() (otelcol.Factories, error) {
	factories := otelcol.Factories{
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
		Extensions: map[component.Type]extension.Factory{
			// Add extensions as needed
		},
		Connectors: map[component.Type]connector.Factory{
			// Add connectors as needed
		},
	}
	
	return factories, nil
}()
EOF

echo "Updated components.go with custom components only"

# First, let's fix the NRI exporter to remove ratelimit import
echo "Fixing NRI exporter to remove ratelimit dependency..."
sed -i.bak '20,21d' ../../exporters/nri/exporter.go  # Remove the ratelimit import comment
sed -i.bak 's|// e.rateLimiter|// rateLimiter|g' ../../exporters/nri/exporter.go
sed -i.bak 's|rateLimiter \*ratelimit.DatabaseRateLimiter|// rateLimiter removed|g' ../../exporters/nri/exporter.go

# Try to download dependencies and tidy
echo "Downloading dependencies..."
go mod download 2>&1 | grep -v "no required module provides package" || true

# Run go mod tidy to clean up
echo "Running go mod tidy..."
go mod tidy -v 2>&1 || true

# Build the production collector
echo "Building production collector..."
if go build -o otelcol-database-intelligence .; then
    echo "✓ Production collector built successfully!"
    ls -la otelcol-database-intelligence
    
    # Test that it runs
    echo ""
    echo "Testing collector binary..."
    ./otelcol-database-intelligence --version || true
else
    echo "⚠ Build failed. Checking errors..."
    go build -v . 2>&1 | head -20
fi

# Go back to project root
cd ../..

echo ""
echo "=== Production distribution fix complete ==="
echo ""
echo "Next steps:"
echo "1. Fix any remaining import issues in custom components"
echo "2. Add contrib components later if needed"
echo "3. Create comprehensive test configurations"