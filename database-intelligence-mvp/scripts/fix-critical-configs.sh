#\!/bin/bash
# Fix critical configuration issues

set -e

echo "=== Fixing Critical Configuration Issues ==="

# Function to add collector.name to resource processor
add_collector_name() {
    local file=$1
    echo "Adding collector.name to: $file"
    
    # Check if resource processor exists
    if grep -q "processors:" "$file" && grep -q "resource:" "$file"; then
        # Check if collector.name already exists
        if \! grep -q "collector\.name" "$file"; then
            # Add collector.name after the resource: line
            sed -i.bak '/resource:/,/attributes:/ {
                /attributes:/ a\
      - key: collector.name\
        value: otelcol\
        action: upsert
            }' "$file"
        fi
    else
        echo "  WARNING: No resource processor found in $file"
    fi
}

# Function to fix env vars and add logs to sqlquery
fix_comprehensive() {
    local file=$1
    echo "Comprehensive fix for: $file"
    
    # Fix all environment variable syntax (colon to colon-dash)
    sed -i.bak 's/${env:\([^:}]*\):\([^}]*\)}/${env:\1:-\2}/g' "$file"
    
    # Fix memory limiter
    sed -i.bak 's/limit_percentage: *[0-9]*/limit_mib: 1024/g' "$file"
    sed -i.bak 's/spike_limit_percentage: *[0-9]*/spike_limit_mib: 256/g' "$file"
    
    # Clean up backup
    rm -f "${file}.bak"
}

# Fix critical files
echo -e "\n1. Fixing collector-simplified.yaml"
fix_comprehensive "config/collector-simplified.yaml"
add_collector_name "config/collector-simplified.yaml"

echo -e "\n2. Fixing collector-resilient.yaml"
fix_comprehensive "config/collector-resilient.yaml"
add_collector_name "config/collector-resilient.yaml"

echo -e "\n3. Fixing production-newrelic.yaml"
if [ -f "config/production-newrelic.yaml" ]; then
    fix_comprehensive "config/production-newrelic.yaml"
    add_collector_name "config/production-newrelic.yaml"
fi

echo -e "\n✅ Critical fixes complete\!"
