#!/bin/bash

# Script to align OpenTelemetry versions across all modules
set -e

echo "=== Aligning OpenTelemetry Versions ==="
echo

# Define the target versions
OTEL_CORE_VERSION="v1.35.0"
OTEL_VERSIONED_VERSION="v0.130.0"
CONTRIB_VERSION="v0.130.0"

# Function to update a go.mod file
update_gomod() {
    local file=$1
    echo "Updating: $file"
    
    # Skip if file doesn't exist
    if [ ! -f "$file" ]; then
        echo "  File not found, skipping..."
        return
    fi
    
    # Update core component versions (v1.x.x)
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/component v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/component '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/processor v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/processor '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/consumer v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/consumer '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/pdata v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/pdata '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/receiver v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/receiver '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/exporter v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/exporter '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/extension v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/extension '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/featuregate v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/featuregate '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/confmap v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/confmap '"$OTEL_CORE_VERSION"'|g' "$file"
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/consumer/consumererror v[0-9]+\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/consumer/consumererror '"$OTEL_CORE_VERSION"'|g' "$file"
    
    # Update versioned components (v0.x.x)
    sed -i.bak -E 's|go\.opentelemetry\.io/collector/[a-z]+/[a-z]+ v0\.[0-9]+\.[0-9]+|&|g' "$file" | while read -r line; do
        if [[ $line =~ go\.opentelemetry\.io/collector/.*/.*\ v0\.[0-9]+\.[0-9]+ ]]; then
            component=$(echo "$line" | sed -E 's|.*go\.opentelemetry\.io/collector/([a-z]+/[a-z]+).*|\1|')
            sed -i.bak -E "s|go\.opentelemetry\.io/collector/$component v0\.[0-9]+\.[0-9]+|go.opentelemetry.io/collector/$component $OTEL_VERSIONED_VERSION|g" "$file"
        fi
    done
    
    # Update contrib versions
    sed -i.bak -E 's|github\.com/open-telemetry/opentelemetry-collector-contrib/[a-z]+/[a-z]+ v[0-9]+\.[0-9]+\.[0-9]+|&|g' "$file" | while read -r line; do
        if [[ $line =~ github\.com/open-telemetry/opentelemetry-collector-contrib/.*/.*\ v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
            component=$(echo "$line" | sed -E 's|.*github\.com/open-telemetry/opentelemetry-collector-contrib/([a-z]+/[a-z]+).*|\1|')
            sed -i.bak -E "s|github\.com/open-telemetry/opentelemetry-collector-contrib/$component v[0-9]+\.[0-9]+\.[0-9]+|github.com/open-telemetry/opentelemetry-collector-contrib/$component $CONTRIB_VERSION|g" "$file"
        fi
    done
    
    # Clean up backup files
    rm -f "${file}.bak"
}

# Update all processor go.mod files
echo "Updating processor modules..."
for processor_dir in processors/*/; do
    if [ -d "$processor_dir" ]; then
        update_gomod "$processor_dir/go.mod"
    fi
done

# Update all common go.mod files
echo -e "\nUpdating common modules..."
for common_dir in common/*/; do
    if [ -d "$common_dir" ]; then
        update_gomod "$common_dir/go.mod"
    fi
done

# Update all core go.mod files
echo -e "\nUpdating core modules..."
for core_dir in core/*/; do
    if [ -d "$core_dir" ]; then
        update_gomod "$core_dir/go.mod"
    fi
done

# Update distribution go.mod files
echo -e "\nUpdating distribution modules..."
for dist_dir in distributions/*/; do
    if [ -d "$dist_dir" ]; then
        update_gomod "$dist_dir/go.mod"
    fi
done

echo -e "\n=== Version Alignment Complete ==="
echo "All modules have been updated to use:"
echo "  - Core components: $OTEL_CORE_VERSION"
echo "  - Versioned components: $OTEL_VERSIONED_VERSION"
echo "  - Contrib components: $CONTRIB_VERSION"