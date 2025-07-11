#!/bin/bash

echo "=== Building Minimal Collector First ==="

cd distributions/production

# Create a minimal components.go with just core components
echo "Creating minimal components.go..."
cat > components_minimal.go << 'EOF'
// Package main provides minimal component factories for testing
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
)

// componentsComplete provides minimal components for testing
var componentsComplete = func() (otelcol.Factories, error) {
	factories := otelcol.Factories{
		Receivers: map[component.Type]receiver.Factory{
			component.MustNewType("otlp"): otlpreceiver.NewFactory(),
		},
		Processors: map[component.Type]processor.Factory{
			component.MustNewType("batch"):          batchprocessor.NewFactory(),
			component.MustNewType("memory_limiter"): memorylimiterprocessor.NewFactory(),
		},
		Exporters: map[component.Type]exporter.Factory{
			component.MustNewType("debug"): debugexporter.NewFactory(),
			component.MustNewType("otlp"):  otlpexporter.NewFactory(),
		},
		Extensions: map[component.Type]extension.Factory{},
		Connectors: map[component.Type]connector.Factory{},
	}
	
	return factories, nil
}()
EOF

# Backup original components.go
mv components.go components_full.go

# Use minimal version
mv components_minimal.go components.go

# Update go.mod to only require core components
echo "Creating minimal go.mod..."
cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/production

go 1.23.0

require (
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
)
EOF

# Clear go.sum
rm -f go.sum

# Try building minimal version
echo ""
echo "Building minimal collector..."
if GOWORK=off go mod tidy && GOWORK=off go build -o otelcol-minimal .; then
    echo "✓ Minimal collector built successfully!"
    ls -la otelcol-minimal
    
    echo ""
    echo "Testing minimal collector..."
    ./otelcol-minimal --version
    
    echo ""
    echo "Available components in minimal build:"
    ./otelcol-minimal components 2>&1 | head -20
else
    echo "⚠ Even minimal build failed. Checking errors..."
    GOWORK=off go build . 2>&1 | head -20
fi

cd ../..

echo ""
echo "=== Minimal build complete ==="
echo ""
echo "If successful, we can gradually add custom components back"