#!/bin/bash

echo "=== Fixing Syntax Issues ==="

# 1. Fix go.work syntax
echo "Fixing go.work..."
cat > go.work << 'EOF'
go 1.23

use (
	.
	./internal/database
)
EOF

# 2. Fix production go.mod syntax
echo "Fixing production go.mod..."
cd distributions/production

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
	
	// Internal modules
	github.com/database-intelligence/internal/database => ../../internal/database
)
EOF

cd ../..

# 3. Fix NRI exporter properly by restoring from backup or original
echo "Restoring NRI exporter from original..."
cd exporters/nri

# Check if we have a backup
if [ -f exporter.go.bak ]; then
    echo "Restoring from backup..."
    cp exporter.go.bak exporter.go
    
    # Now properly remove rate limiter references
    sed -i.bak2 's|rateLimiter \*ratelimit\.DatabaseRateLimiter|// rateLimiter removed|g' exporter.go
    sed -i.bak2 's|e\.rateLimiter|nil /* rate limiter */|g' exporter.go
    sed -i.bak2 '/^[[:space:]]*\/\/ Import rate limiter$/d' exporter.go
else
    echo "No backup found. Creating minimal working version..."
    # We'll need to restore the full file - let me check what we have
    ls -la
fi

cd ../..

# 4. Build again
echo ""
echo "Building production collector..."
cd distributions/production

if go build -o otelcol-database-intelligence .; then
    echo "✓ Production collector built successfully!"
    ls -la otelcol-database-intelligence
else
    echo "⚠ Build failed. Checking errors..."
    go build . 2>&1 | head -30
fi

cd ../..

echo "=== Fix complete ==="