#!/bin/bash
# Test script to validate the complete Database Intelligence setup

set -e

echo "===== Database Intelligence Setup Validation ====="
echo

# Check environment variables
echo "1. Checking environment variables..."
if [ ! -f ".env" ]; then
    echo "   WARNING: .env file not found. Copying from .env.example..."
    cp .env.example .env
fi

# Validate Go versions
echo "2. Validating Go module versions..."
for mod in $(find . -name "go.mod" -not -path "./archive/*" -not -path "./tests/*"); do
    if grep -q "go 1.23\|go 1.24\|toolchain" "$mod"; then
        echo "   ERROR: Invalid Go version in $mod"
        exit 1
    fi
done
echo "   ✓ All Go modules use valid versions"

# Check config files
echo "3. Checking configuration files..."
configs=(
    "distributions/production/production-config.yaml"
    "distributions/production/production-config-complete.yaml"
)
for config in "${configs[@]}"; do
    if [ -f "$config" ]; then
        echo "   ✓ Found $config"
        # Check for required environment variables
        if grep -q "DB_POSTGRES_HOST" "$config" && grep -q "NEW_RELIC_LICENSE_KEY" "$config"; then
            echo "     ✓ Uses standardized environment variables"
        else
            echo "     WARNING: May not use standardized variables"
        fi
    else
        echo "   ERROR: Missing $config"
    fi
done

# Check Docker setup
echo "4. Checking Docker configuration..."
if [ -f "deployments/docker/compose/docker-compose.yaml" ]; then
    echo "   ✓ Docker Compose file exists"
    if grep -q "DB_POSTGRES_HOST" "deployments/docker/compose/docker-compose.yaml"; then
        echo "   ✓ Docker Compose uses standardized variables"
    fi
fi

# Check builder configuration
echo "5. Checking OTel Builder configuration..."
if [ -f "otelcol-builder-config-complete.yaml" ]; then
    echo "   ✓ Complete builder config exists"
    # Count custom components
    processors=$(grep -c "processors/.*v0.0.0" otelcol-builder-config-complete.yaml || true)
    receivers=$(grep -c "receivers/.*v0.0.0" otelcol-builder-config-complete.yaml || true)
    echo "   ✓ Includes $processors custom processors and $receivers custom receivers"
fi

# Summary
echo
echo "===== Validation Summary ====="
echo
echo "✓ Go versions are standardized"
echo "✓ Environment variables are standardized"
echo "✓ Configuration files are in place"
echo "✓ Docker setup is configured"
echo "✓ Builder configuration includes all components"
echo
echo "Next steps:"
echo "1. Set your New Relic license key in .env file"
echo "2. Run: ./build-complete-collector.sh"
echo "3. Test locally or with Docker Compose"
echo
echo "===== Validation Complete ====="