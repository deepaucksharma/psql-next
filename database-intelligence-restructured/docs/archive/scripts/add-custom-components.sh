#!/bin/bash

echo "=== Adding Custom Components Back to Production Distribution ==="

cd distributions/production

# First, let's test which components compile successfully
echo "Testing component builds..."

# Test processors
WORKING_PROCESSORS=()
echo ""
echo "Testing processors..."
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    echo -n "  Testing $processor... "
    if cd ../../processors/$processor && go build . 2>/dev/null; then
        echo "✓"
        WORKING_PROCESSORS+=($processor)
    else
        echo "✗"
    fi
    cd - >/dev/null
done

# Test receivers  
WORKING_RECEIVERS=()
echo ""
echo "Testing receivers..."
for receiver in ash enhancedsql kernelmetrics; do
    echo -n "  Testing $receiver... "
    if cd ../../receivers/$receiver && go build . 2>/dev/null; then
        echo "✓"
        WORKING_RECEIVERS+=($receiver)
    else
        echo "✗"
    fi
    cd - >/dev/null
done

# Test exporters
WORKING_EXPORTERS=()
echo ""
echo "Testing exporters..."
for exporter in nri; do
    echo -n "  Testing $exporter... "
    if cd ../../exporters/$exporter && go build . 2>/dev/null; then
        echo "✓"
        WORKING_EXPORTERS+=($exporter)
    else
        echo "✗"
    fi
    cd - >/dev/null
done

echo ""
echo "Working components:"
echo "  Processors: ${WORKING_PROCESSORS[@]}"
echo "  Receivers: ${WORKING_RECEIVERS[@]}" 
echo "  Exporters: ${WORKING_EXPORTERS[@]}"

# Create components.go with working components
echo ""
echo "Creating components.go with working components..."

cat > components_with_custom.go << 'EOF'
// Package main provides component factories for the Database Intelligence Collector
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
	
	// Working custom processors
EOF

# Add working processor imports
for processor in "${WORKING_PROCESSORS[@]}"; do
    echo "	\"github.com/database-intelligence/processors/$processor\"" >> components_with_custom.go
done

# Add working receiver imports
echo "	" >> components_with_custom.go
echo "	// Working custom receivers" >> components_with_custom.go
for receiver in "${WORKING_RECEIVERS[@]}"; do
    echo "	\"github.com/database-intelligence/receivers/$receiver\"" >> components_with_custom.go
done

# Add working exporter imports
if [ ${#WORKING_EXPORTERS[@]} -gt 0 ]; then
    echo "	" >> components_with_custom.go
    echo "	// Working custom exporters" >> components_with_custom.go
    for exporter in "${WORKING_EXPORTERS[@]}"; do
        echo "	\"github.com/database-intelligence/exporters/$exporter\"" >> components_with_custom.go
    done
fi

cat >> components_with_custom.go << 'EOF'
)

// componentsComplete provides all available components
var componentsComplete = otelcol.Factories{
	Receivers: map[component.Type]receiver.Factory{
		// Core receivers
		component.MustNewType("otlp"): otlpreceiver.NewFactory(),
		
		// Custom receivers
EOF

# Add working receivers to factory map
for receiver in "${WORKING_RECEIVERS[@]}"; do
    echo "		component.MustNewType(\"$receiver\"): $receiver.NewFactory()," >> components_with_custom.go
done

cat >> components_with_custom.go << 'EOF'
	},
	Processors: map[component.Type]processor.Factory{
		// Core processors
		component.MustNewType("batch"):          batchprocessor.NewFactory(),
		component.MustNewType("memory_limiter"): memorylimiterprocessor.NewFactory(),
		
		// Custom processors
EOF

# Add working processors to factory map
for processor in "${WORKING_PROCESSORS[@]}"; do
    echo "		component.MustNewType(\"$processor\"): $processor.NewFactory()," >> components_with_custom.go
done

cat >> components_with_custom.go << 'EOF'
	},
	Exporters: map[component.Type]exporter.Factory{
		// Core exporters
		component.MustNewType("debug"): debugexporter.NewFactory(),
		component.MustNewType("otlp"):  otlpexporter.NewFactory(),
		
		// Custom exporters
EOF

# Add working exporters to factory map
for exporter in "${WORKING_EXPORTERS[@]}"; do
    echo "		component.MustNewType(\"$exporter\"): $exporter.NewFactory()," >> components_with_custom.go
done

cat >> components_with_custom.go << 'EOF'
	},
	Extensions: map[component.Type]extension.Factory{},
	Connectors: map[component.Type]connector.Factory{},
}
EOF

# Update go.mod with working components
echo ""
echo "Updating go.mod with working components..."

# Backup current go.mod
cp go.mod go.mod.minimal

# Create new go.mod with working components
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
EOF

# Add working custom components to requirements
echo "	" >> go.mod
echo "	// Working custom components" >> go.mod
for processor in "${WORKING_PROCESSORS[@]}"; do
    echo "	github.com/database-intelligence/processors/$processor v0.0.0-00010101000000-000000000000" >> go.mod
done
for receiver in "${WORKING_RECEIVERS[@]}"; do
    echo "	github.com/database-intelligence/receivers/$receiver v0.0.0-00010101000000-000000000000" >> go.mod
done
for exporter in "${WORKING_EXPORTERS[@]}"; do
    echo "	github.com/database-intelligence/exporters/$exporter v0.0.0-00010101000000-000000000000" >> go.mod
done

# Also add common modules if working components need them
echo "	" >> go.mod
echo "	// Common modules" >> go.mod
echo "	github.com/database-intelligence/common/featuredetector v0.0.0-00010101000000-000000000000" >> go.mod
echo "	github.com/database-intelligence/common/queryselector v0.0.0-00010101000000-000000000000" >> go.mod

cat >> go.mod << 'EOF'
)

// Replace directives for local modules
replace (
	// Custom processors
EOF

for processor in "${WORKING_PROCESSORS[@]}"; do
    echo "	github.com/database-intelligence/processors/$processor => ../../processors/$processor" >> go.mod
done

echo "	" >> go.mod
echo "	// Custom receivers" >> go.mod
for receiver in "${WORKING_RECEIVERS[@]}"; do
    echo "	github.com/database-intelligence/receivers/$receiver => ../../receivers/$receiver" >> go.mod
done

if [ ${#WORKING_EXPORTERS[@]} -gt 0 ]; then
    echo "	" >> go.mod
    echo "	// Custom exporters" >> go.mod
    for exporter in "${WORKING_EXPORTERS[@]}"; do
        echo "	github.com/database-intelligence/exporters/$exporter => ../../exporters/$exporter" >> go.mod
    done
fi

cat >> go.mod << 'EOF'
	
	// Common modules
	github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
	github.com/database-intelligence/common/queryselector => ../../common/queryselector
	
	// Internal modules (if needed)
	github.com/database-intelligence/internal/database => ../../internal/database
)
EOF

# Test building with custom components
echo ""
echo "Building collector with custom components..."

# First backup minimal components
mv components.go components_minimal.go
mv components_with_custom.go components.go

# Clear go.sum and try to build
rm -f go.sum
if GOWORK=off go mod tidy && GOWORK=off go build -o otelcol-custom .; then
    echo "✓ Build successful with custom components!"
    ls -lh otelcol-custom
    
    # Test components
    echo ""
    echo "Available components:"
    ./otelcol-custom components 2>&1 | grep -E "(receivers:|processors:|exporters:)" -A 10
else
    echo "✗ Build failed with custom components"
    echo "Reverting to minimal configuration..."
    mv components.go components_with_custom.go
    mv components_minimal.go components.go
    cp go.mod.minimal go.mod
fi

cd ../..

echo ""
echo "=== Component addition complete ===">