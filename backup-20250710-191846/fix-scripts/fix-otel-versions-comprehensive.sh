#!/bin/bash

# Comprehensive script to fix OpenTelemetry versions across entire codebase
set -e

echo "=== Comprehensive OpenTelemetry Version Fix ==="
echo
echo "This script will update all modules to use the correct OpenTelemetry versions:"
echo "- Implementation components: v0.109.0"
echo "- Core data/config components: v1.15.0"
echo

# Define version mappings
declare -A VERSION_MAP=(
    # v0.109.0 components (implementation)
    ["go.opentelemetry.io/collector/component"]="v0.109.0"
    ["go.opentelemetry.io/collector/processor"]="v0.109.0"
    ["go.opentelemetry.io/collector/consumer"]="v0.109.0"
    ["go.opentelemetry.io/collector/receiver"]="v0.109.0"
    ["go.opentelemetry.io/collector/exporter"]="v0.109.0"
    ["go.opentelemetry.io/collector/extension"]="v0.109.0"
    ["go.opentelemetry.io/collector/connector"]="v0.109.0"
    ["go.opentelemetry.io/collector/otelcol"]="v0.109.0"
    
    # Specific components
    ["go.opentelemetry.io/collector/exporter/debugexporter"]="v0.109.0"
    ["go.opentelemetry.io/collector/exporter/otlpexporter"]="v0.109.0"
    ["go.opentelemetry.io/collector/exporter/otlphttpexporter"]="v0.109.0"
    ["go.opentelemetry.io/collector/extension/ballastextension"]="v0.109.0"
    ["go.opentelemetry.io/collector/extension/zpagesextension"]="v0.109.0"
    ["go.opentelemetry.io/collector/processor/batchprocessor"]="v0.109.0"
    ["go.opentelemetry.io/collector/processor/memorylimiterprocessor"]="v0.109.0"
    ["go.opentelemetry.io/collector/receiver/otlpreceiver"]="v0.109.0"
    
    # Test components
    ["go.opentelemetry.io/collector/component/componenttest"]="v0.109.0"
    ["go.opentelemetry.io/collector/processor/processortest"]="v0.109.0"
    ["go.opentelemetry.io/collector/processor/processorhelper"]="v0.109.0"
    ["go.opentelemetry.io/collector/consumer/consumertest"]="v0.109.0"
    
    # v1.15.0 components (core data/config)
    ["go.opentelemetry.io/collector/pdata"]="v1.15.0"
    ["go.opentelemetry.io/collector/featuregate"]="v1.15.0"
    ["go.opentelemetry.io/collector/confmap"]="v1.15.0"
    ["go.opentelemetry.io/collector/config/configopaque"]="v1.15.0"
    ["go.opentelemetry.io/collector/config/configretry"]="v1.15.0"
    ["go.opentelemetry.io/collector/config/configtls"]="v1.15.0"
    ["go.opentelemetry.io/collector/config/configcompression"]="v1.15.0"
    ["go.opentelemetry.io/collector/client"]="v1.15.0"
    
    # Config providers
    ["go.opentelemetry.io/collector/confmap/provider/envprovider"]="v1.15.0"
    ["go.opentelemetry.io/collector/confmap/provider/fileprovider"]="v1.15.0"
    ["go.opentelemetry.io/collector/confmap/provider/httpprovider"]="v0.109.0"
    ["go.opentelemetry.io/collector/confmap/provider/httpsprovider"]="v0.109.0"
    ["go.opentelemetry.io/collector/confmap/provider/yamlprovider"]="v0.109.0"
)

# Function to update a specific go.mod file
update_gomod() {
    local file=$1
    local temp_file="${file}.tmp"
    
    echo "Processing: $file"
    
    # Create a copy to work with
    cp "$file" "$temp_file"
    
    # Update each known component to its correct version
    for component in "${!VERSION_MAP[@]}"; do
        version="${VERSION_MAP[$component]}"
        # Update both direct and indirect dependencies
        sed -i '' -E "s|${component} v[0-9]+\.[0-9]+\.[0-9]+|${component} ${version}|g" "$temp_file"
    done
    
    # Update contrib components to v0.109.0
    sed -i '' -E 's|github\.com/open-telemetry/opentelemetry-collector-contrib/[a-zA-Z/]+ v[0-9]+\.[0-9]+\.[0-9]+|&|g' "$temp_file" | while read -r line; do
        if [[ $line =~ github\.com/open-telemetry/opentelemetry-collector-contrib/.* ]]; then
            sed -i '' -E "s|github\.com/open-telemetry/opentelemetry-collector-contrib/([a-zA-Z/]+) v[0-9]+\.[0-9]+\.[0-9]+|github.com/open-telemetry/opentelemetry-collector-contrib/\1 v0.109.0|g" "$temp_file"
        fi
    done
    
    # Move the updated file back
    mv "$temp_file" "$file"
}

# Update all go.mod files
echo "Finding all go.mod files..."
find . -name "go.mod" -type f | while read -r gomod; do
    # Skip vendor and other generated directories
    if [[ "$gomod" == *"/vendor/"* ]] || [[ "$gomod" == *"/.git/"* ]]; then
        continue
    fi
    update_gomod "$gomod"
done

echo
echo "=== Creating updated enterprise distribution go.mod ==="

# Create the corrected enterprise distribution go.mod
cat > distributions/enterprise/go.mod << 'EOF'
module github.com/database-intelligence/distributions/enterprise

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/exporter v0.109.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.109.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.109.0
    go.opentelemetry.io/collector/exporter/otlphttpexporter v0.109.0
    go.opentelemetry.io/collector/extension v0.109.0
    go.opentelemetry.io/collector/extension/ballastextension v0.109.0
    go.opentelemetry.io/collector/extension/zpagesextension v0.109.0
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
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.109.0
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

echo
echo "=== Restoring full enterprise main.go ==="

# Restore the full main.go with all components
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
    "go.opentelemetry.io/collector/extension/ballastextension"
    "go.opentelemetry.io/collector/extension/zpagesextension"
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
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
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
        filelogreceiver.NewFactory().Type():     filelogreceiver.NewFactory(),
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
        ballastextension.NewFactory().Type():      ballastextension.NewFactory(),
        zpagesextension.NewFactory().Type():       zpagesextension.NewFactory(),
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

echo
echo "=== Version Fix Complete ==="
echo
echo "All modules have been updated to use the correct OpenTelemetry versions."
echo "Next steps:"
echo "1. Run go mod tidy in each directory"
echo "2. Build the enterprise distribution"
echo "3. Test the collector"