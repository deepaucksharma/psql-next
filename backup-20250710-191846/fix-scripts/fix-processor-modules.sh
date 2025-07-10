#!/bin/bash

# Fix processor module names and dependencies
set -e

echo "=== Fixing Processor Modules ==="
echo

# Fix each processor individually
fix_processor() {
    local name=$1
    local dir="processors/$name"
    
    echo "Fixing $name..."
    
    # Create proper go.mod
    cat > "$dir/go.mod" << EOF
module github.com/database-intelligence/processors/$name

go 1.22

require (
    go.opentelemetry.io/collector/component v0.109.0
    go.opentelemetry.io/collector/consumer v0.109.0
    go.opentelemetry.io/collector/pdata v1.15.0
    go.opentelemetry.io/collector/processor v0.109.0
    go.uber.org/zap v1.27.0
    github.com/stretchr/testify v1.9.0
)
EOF

    # Add specific dependencies for each processor
    case $name in
        "adaptivesampler")
            cat >> "$dir/go.mod" << 'EOF'

require (
    github.com/go-redis/redis/v8 v8.11.5
    github.com/hashicorp/golang-lru/v2 v2.0.7
)
EOF
            ;;
        "circuitbreaker")
            cat >> "$dir/go.mod" << 'EOF'

require (
    github.com/database-intelligence/common/featuredetector v0.0.0-00010101000000-000000000000
)

replace github.com/database-intelligence/common/featuredetector => ../../common/featuredetector
EOF
            ;;
        "planattributeextractor")
            cat >> "$dir/go.mod" << 'EOF'

require (
    github.com/tidwall/gjson v1.17.0
)
EOF
            ;;
    esac
}

# Fix all processors
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    fix_processor "$processor"
done

echo
echo "=== Running go mod tidy for all processors ==="
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    echo -n "  $processor... "
    cd "processors/$processor"
    if go mod tidy 2>/dev/null; then
        echo "✓"
    else
        echo "✗"
    fi
    cd ../..
done

echo
echo "=== Processor Modules Fixed ==="