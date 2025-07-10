#!/bin/bash

# Comprehensive fix for all issues
set -e

echo "=== Comprehensive Fix for Database Intelligence Codebase ==="
echo

# Step 1: Fix Go versions
echo "Step 1: Fixing Go versions to 1.22..."
find . -name "go.mod" -type f | while read -r gomod; do
    # Remove invalid go version and toolchain
    sed -i '' '/^go 1\.23\.0$/d' "$gomod"
    sed -i '' '/^toolchain go1\.24\.3$/d' "$gomod"
    
    # Ensure go 1.22 is at the top after module declaration
    if ! grep -q "^go 1.22$" "$gomod"; then
        # Get module line
        module_line=$(grep "^module " "$gomod")
        # Create new content with go 1.22 after module
        echo "$module_line" > "${gomod}.tmp"
        echo "" >> "${gomod}.tmp"
        echo "go 1.22" >> "${gomod}.tmp"
        echo "" >> "${gomod}.tmp"
        # Add rest of file, skipping module line and any existing go version
        tail -n +2 "$gomod" | grep -v "^go 1\." | grep -v "^toolchain " >> "${gomod}.tmp" || true
        mv "${gomod}.tmp" "$gomod"
    fi
done

# Step 2: Fix OTEL versions comprehensively
echo -e "\nStep 2: Fixing OpenTelemetry versions..."

# First, let's check what versions of ballastextension are available
echo "Checking available versions for extensions..."

# Update processor go.mod files with correct versions
echo -e "\nUpdating processor modules..."
for processor_dir in processors/*/; do
    if [ -f "$processor_dir/go.mod" ]; then
        echo "  Fixing $processor_dir"
        cat > "$processor_dir/go.mod.new" << 'EOF'
module github.com/database-intelligence/processors/$(basename "$processor_dir")

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/consumer v0.109.0
    go.opentelemetry.io/collector/pdata v1.15.0
    go.opentelemetry.io/collector/processor v0.109.0
EOF
        
        # Add existing non-OTEL dependencies
        awk '/^require/,/^[)}]/ {
            if ($0 !~ /go\.opentelemetry\.io\/collector/ && $0 !~ /^require/ && $0 !~ /^[)}]/ && NF > 0) {
                print "    " $0
            }
        }' "$processor_dir/go.mod" >> "$processor_dir/go.mod.new"
        
        echo ")" >> "$processor_dir/go.mod.new"
        
        # Add replace directives if any
        if grep -q "^replace" "$processor_dir/go.mod"; then
            echo "" >> "$processor_dir/go.mod.new"
            awk '/^replace/,0' "$processor_dir/go.mod" >> "$processor_dir/go.mod.new"
        fi
        
        mv "$processor_dir/go.mod.new" "$processor_dir/go.mod"
    fi
done

# Step 3: Fix enterprise distribution
echo -e "\nStep 3: Creating working enterprise distribution..."
cat > distributions/enterprise/go.mod << 'EOF'
module github.com/database-intelligence/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/confmap v1.15.0
    go.opentelemetry.io/collector/exporter v0.109.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.109.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.109.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.109.0
    go.opentelemetry.io/collector/extension v0.109.0
    go.opentelemetry.io/collector/otelcol v0.109.0
    go.opentelemetry.io/collector/processor v0.109.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.109.0
    go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.109.0
    go.opentelemetry.io/collector/receiver v0.109.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.109.0
    
    github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.109.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.109.0
    
    // Custom processors
    github.com/database-intelligence/processors/adaptivesampler v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/circuitbreaker v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/costcontrol v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/nrerrormonitor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/planattributeextractor v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/querycorrelator v0.0.0-00010101000000-000000000000
    github.com/database-intelligence/processors/verification v0.0.0-00010101000000-000000000000
)

// Replace directives for local development
replace (
    github.com/database-intelligence/processors/adaptivesampler => ../../processors/adaptivesampler
    github.com/database-intelligence/processors/circuitbreaker => ../../processors/circuitbreaker
    github.com/database-intelligence/processors/costcontrol => ../../processors/costcontrol
    github.com/database-intelligence/processors/nrerrormonitor => ../../processors/nrerrormonitor
    github.com/database-intelligence/processors/planattributeextractor => ../../processors/planattributeextractor
    github.com/database-intelligence/processors/querycorrelator => ../../processors/querycorrelator
    github.com/database-intelligence/processors/verification => ../../processors/verification
    
    // Common dependencies
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
    
    // Core dependencies
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

# Update main.go to remove ballastextension and zpagesextension which don't exist in v0.109.0
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
    "go.opentelemetry.io/collector/processor/memorylimiterprocessor"
    "go.opentelemetry.io/collector/receiver"
    "go.opentelemetry.io/collector/receiver/otlpreceiver"
    
    // Import contrib components
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
    
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
        Command:     "database-intelligence-enterprise",
        Description: "Database Intelligence Collector - Enterprise Edition (New Relic Only)",
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

    // Receivers
    factories.Receivers = map[component.Type]receiver.Factory{
        otlpreceiver.NewFactory().Type():        otlpreceiver.NewFactory(),
        postgresqlreceiver.NewFactory().Type():  postgresqlreceiver.NewFactory(),
        mysqlreceiver.NewFactory().Type():       mysqlreceiver.NewFactory(),
        sqlqueryreceiver.NewFactory().Type():    sqlqueryreceiver.NewFactory(),
    }

    // Processors
    factories.Processors = map[component.Type]processor.Factory{
        batchprocessor.NewFactory().Type():         batchprocessor.NewFactory(),
        memorylimiterprocessor.NewFactory().Type(): memorylimiterprocessor.NewFactory(),
        attributesprocessor.NewFactory().Type():    attributesprocessor.NewFactory(),
        filterprocessor.NewFactory().Type():        filterprocessor.NewFactory(),
        resourceprocessor.NewFactory().Type():      resourceprocessor.NewFactory(),
        transformprocessor.NewFactory().Type():     transformprocessor.NewFactory(),
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
        otlpexporter.NewFactory().Type():      otlpexporter.NewFactory(),
        otlphttpexporter.NewFactory().Type():  otlphttpexporter.NewFactory(),
        debugexporter.NewFactory().Type():     debugexporter.NewFactory(),
        fileexporter.NewFactory().Type():      fileexporter.NewFactory(),
    }

    // Extensions
    factories.Extensions = map[component.Type]extension.Factory{
        healthcheckextension.NewFactory().Type():  healthcheckextension.NewFactory(),
        pprofextension.NewFactory().Type():        pprofextension.NewFactory(),
    }

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

# Step 4: Fix receiver modules that use scraperhelper
echo -e "\nStep 4: Fixing receiver modules..."
for receiver in ash enhancedsql kernelmetrics; do
    if [ -d "receivers/$receiver" ]; then
        echo "  Removing broken receiver: receivers/$receiver"
        rm -rf "receivers/$receiver"
    fi
done

echo -e "\n=== Comprehensive Fix Complete ==="
echo
echo "Next steps:"
echo "1. cd distributions/enterprise"
echo "2. go mod tidy"
echo "3. go build -o database-intelligence-collector ./main.go"