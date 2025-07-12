#!/bin/bash

# Start the Database Intelligence OpenTelemetry Collector
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="/Users/deepaksharma/syc/db-otel/.env"

echo "=== Starting Database Intelligence Collector ==="
echo ""

# Load environment variables
if [[ -f "$ENV_FILE" ]]; then
    echo "Loading environment from: $ENV_FILE"
    set -a
    source "$ENV_FILE"
    set +a
else
    echo "❌ Environment file not found: $ENV_FILE"
    exit 1
fi

# Verify required variables
if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
    echo "❌ NEW_RELIC_LICENSE_KEY is not set"
    exit 1
fi

echo "✅ Environment loaded"
echo "  Account ID: ${NEW_RELIC_ACCOUNT_ID}"
echo "  License Key: ${NEW_RELIC_LICENSE_KEY:0:10}..."
echo ""

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker Desktop."
    exit 1
fi

# Start PostgreSQL container if not running
echo "Checking PostgreSQL container..."
if ! docker ps | grep -q "db-intel-postgres"; then
    echo "Starting PostgreSQL container..."
    docker run -d \
        --name db-intel-postgres \
        -e POSTGRES_USER=postgres \
        -e POSTGRES_PASSWORD=devpassword123 \
        -e POSTGRES_DB=testdb \
        -p 5432:5432 \
        -v "$SCRIPT_DIR/deployments/docker/init-scripts/postgres-init.sql:/docker-entrypoint-initdb.d/init.sql:ro" \
        postgres:15 || true
    
    echo "Waiting for PostgreSQL to be ready..."
    sleep 10
fi

# Check PostgreSQL connectivity
echo "Testing PostgreSQL connection..."
if docker exec db-intel-postgres pg_isready -U postgres >/dev/null 2>&1; then
    echo "✅ PostgreSQL is ready"
else
    echo "⚠️  PostgreSQL is not ready yet"
fi

# Build the collector if not exists
COLLECTOR_BINARY="$SCRIPT_DIR/distributions/production/otelcol-complete"
if [[ ! -f "$COLLECTOR_BINARY" ]]; then
    echo ""
    echo "Building the collector..."
    cd "$SCRIPT_DIR/distributions/production"
    go build -o otelcol-complete .
    cd "$SCRIPT_DIR"
    echo "✅ Collector built successfully"
fi

# Create a runtime directory
RUNTIME_DIR="$SCRIPT_DIR/runtime"
mkdir -p "$RUNTIME_DIR"

# Copy config to runtime directory with environment substitution
CONFIG_FILE="$RUNTIME_DIR/collector-config.yaml"
# Use basic config for now to avoid component issues
envsubst < "$SCRIPT_DIR/configs/basic.yaml" > "$CONFIG_FILE"
echo "✅ Configuration prepared at: $CONFIG_FILE"

# Start the collector
echo ""
echo "Starting the collector..."
echo "============================="
echo ""
echo "Dashboard URL: https://one.newrelic.com/redirect/entity/MzYzMDA3MnxWSVp8REFTSEJPQVJEfGRhOjEwNDU1MDA5"
echo ""
echo "Collector endpoints:"
echo "  - OTLP gRPC: localhost:4317"
echo "  - OTLP HTTP: localhost:4318"
echo "  - Health check: http://localhost:13133/health"
echo "  - Metrics: http://localhost:8888/metrics"
echo "  - zPages: http://localhost:55679/debug/tracez"
echo ""
echo "Press Ctrl+C to stop the collector"
echo ""

# Run the collector
exec "$COLLECTOR_BINARY" --config="$CONFIG_FILE"