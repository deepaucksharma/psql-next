#!/bin/bash

# Fix all OTEL versions to v0.109.0 which is known to work
set -e

echo "=== Fixing All OpenTelemetry Versions ==="
echo

# Standard OTEL version that works
OTEL_VERSION="v0.109.0"

# Function to update go.mod files
update_gomod() {
    local file=$1
    echo "Updating: $file"
    
    # Update all OTEL collector dependencies to v0.109.0
    sed -i '' -E 's|go\.opentelemetry\.io/collector/[a-zA-Z/]+ v[0-9]+\.[0-9]+\.[0-9]+|&|g' "$file" | grep -E "go\.opentelemetry\.io/collector" | while read -r line; do
        component=$(echo "$line" | sed -E 's|.*(go\.opentelemetry\.io/collector/[a-zA-Z/]+) v[0-9]+\.[0-9]+\.[0-9]+.*|\1|')
        sed -i '' -E "s|$component v[0-9]+\.[0-9]+\.[0-9]+|$component $OTEL_VERSION|g" "$file"
    done
    
    # Update contrib to matching version
    sed -i '' -E 's|github\.com/open-telemetry/opentelemetry-collector-contrib/[a-zA-Z/]+ v[0-9]+\.[0-9]+\.[0-9]+|&|g' "$file" | grep -E "github\.com/open-telemetry/opentelemetry-collector-contrib" | while read -r line; do
        component=$(echo "$line" | sed -E 's|.*(github\.com/open-telemetry/opentelemetry-collector-contrib/[a-zA-Z/]+) v[0-9]+\.[0-9]+\.[0-9]+.*|\1|')
        sed -i '' -E "s|$component v[0-9]+\.[0-9]+\.[0-9]+|$component $OTEL_VERSION|g" "$file"
    done
}

# Update all processor go.mod files
echo "Updating processor modules..."
for processor_dir in processors/*/; do
    if [ -f "$processor_dir/go.mod" ]; then
        update_gomod "$processor_dir/go.mod"
    fi
done

# Update distributions
echo -e "\nUpdating distribution modules..."
for dist_dir in distributions/*/; do
    if [ -f "$dist_dir/go.mod" ]; then
        update_gomod "$dist_dir/go.mod"
    fi
done

# Update common modules
echo -e "\nUpdating common modules..."
for common_dir in common/*/; do
    if [ -f "$common_dir/go.mod" ]; then
        update_gomod "$common_dir/go.mod"
    fi
done

# Update core modules
echo -e "\nUpdating core modules..."
for core_dir in core/*/; do
    if [ -f "$core_dir/go.mod" ]; then
        update_gomod "$core_dir/go.mod"
    fi
done

echo -e "\n=== Version Update Complete ==="
echo "All modules updated to use OTEL $OTEL_VERSION"