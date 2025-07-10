#!/bin/bash

# Build a minimal working collector
set -e

echo "=== Building Minimal Working Collector ==="
echo

# Create a minimal distribution
mkdir -p distributions/minimal-working
cd distributions/minimal-working

# Create minimal main.go
cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Import only custom processors
    "github.com/database-intelligence/processors/adaptivesampler"
    "github.com/database-intelligence/processors/circuitbreaker"
    "github.com/database-intelligence/processors/costcontrol"
    "github.com/database-intelligence/processors/nrerrormonitor"
    "github.com/database-intelligence/processors/planattributeextractor"
    "github.com/database-intelligence/processors/querycorrelator"
    "github.com/database-intelligence/processors/verification"
)

func main() {
    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Minimal Collector",
        Version:     "1.0.0",
    }

    factories := otelcol.Factories{
        Processors: map[component.Type]component.Factory{
            adaptivesampler.NewFactory().Type():        adaptivesampler.NewFactory(),
            circuitbreaker.NewFactory().Type():         circuitbreaker.NewFactory(),
            costcontrol.NewFactory().Type():            costcontrol.NewFactory(),
            nrerrormonitor.NewFactory().Type():         nrerrormonitor.NewFactory(),
            planattributeextractor.NewFactory().Type(): planattributeextractor.NewFactory(),
            querycorrelator.NewFactory().Type():        querycorrelator.NewFactory(),
            verification.NewFactory().Type():           verification.NewFactory(),
        },
        Receivers:  map[component.Type]component.Factory{},
        Exporters:  map[component.Type]component.Factory{},
        Extensions: map[component.Type]component.Factory{},
        Connectors: map[component.Type]component.Factory{},
    }

    cmd := otelcol.NewCommand(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    })
    
    if err := cmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
EOF

# Create minimal go.mod
cat > go.mod << 'EOF'
module github.com/database-intelligence/distributions/minimal-working

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/otelcol v0.109.0
    
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
    
    github.com/database-intelligence/common/anonutils => ../../common/anonutils
    github.com/database-intelligence/common/detectutils => ../../common/detectutils
    github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
    github.com/database-intelligence/common/newrelicutils => ../../common/newrelicutils
    github.com/database-intelligence/common/piidetector => ../../common/piidetector
    github.com/database-intelligence/common/querylens => ../../common/querylens
    github.com/database-intelligence/common/queryparser => ../../common/queryparser
    github.com/database-intelligence/common/queryselector => ../../common/queryselector
    github.com/database-intelligence/common/sqltokenizer => ../../common/sqltokenizer
    github.com/database-intelligence/common/telemetry => ../../common/telemetry
    github.com/database-intelligence/common/utils => ../../common/utils
    
    github.com/database-intelligence/core/clientauth => ../../core/clientauth
    github.com/database-intelligence/core/costengine => ../../core/costengine
    github.com/database-intelligence/core/errorhandler => ../../core/errorhandler
    github.com/database-intelligence/core/errormonitor => ../../core/errormonitor
    github.com/database-intelligence/core/healthcheck => ../../core/healthcheck
    github.com/database-intelligence/core/multidb => ../../core/multidb
    github.com/database-intelligence/core/queryanalyzer => ../../core/queryanalyzer
    github.com/database-intelligence/core/ratelimiter => ../../core/ratelimiter
    github.com/database-intelligence/core/verification => ../../core/verification
)
EOF

echo "Building minimal collector..."
go mod tidy
go build -o database-intelligence-minimal ./main.go

if [ -f database-intelligence-minimal ]; then
    echo
    echo "=== Success! ==="
    echo "Minimal collector built successfully!"
    echo "Binary: $(pwd)/database-intelligence-minimal"
    echo
    echo "This proves that your custom processors can be compiled."
    echo "The version conflicts are with the contrib components."
else
    echo "Build failed"
    exit 1
fi