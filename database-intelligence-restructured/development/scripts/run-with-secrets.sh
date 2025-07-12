#!/bin/bash

# Run OpenTelemetry Collector with Secrets Management
# This script demonstrates how to run the collector with secrets properly configured

set -e

echo "🔐 Database Intelligence Collector - Secure Run"
echo "============================================="

# Check if .env file exists
if [ -f ".env" ]; then
    echo "Loading environment variables from .env file..."
    set -a
    source .env
    set +a
else
    echo "No .env file found. Using system environment variables."
fi

# Required environment variables
REQUIRED_VARS=(
    "POSTGRES_HOST"
    "POSTGRES_PORT"
    "POSTGRES_USER"
    "POSTGRES_PASSWORD"
    "POSTGRES_DATABASE"
    "NEW_RELIC_API_KEY"
)

# Check for required variables
echo ""
echo "Checking required environment variables..."
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING_VARS+=("$var")
        echo "❌ Missing: $var"
    else
        # Show masked value
        value="${!var}"
        if [[ "$var" == *"PASSWORD"* ]] || [[ "$var" == *"KEY"* ]] || [[ "$var" == *"TOKEN"* ]]; then
            masked="${value:0:3}***${value: -3}"
            echo "✅ Found: $var = $masked"
        else
            echo "✅ Found: $var = $value"
        fi
    fi
done

if [ ${#MISSING_VARS[@]} -ne 0 ]; then
    echo ""
    echo "ERROR: Missing required environment variables:"
    printf '%s\n' "${MISSING_VARS[@]}"
    echo ""
    echo "Please set the missing variables and try again."
    echo "You can create a .env file with the following format:"
    echo ""
    echo "POSTGRES_HOST=localhost"
    echo "POSTGRES_PORT=5432"
    echo "POSTGRES_USER=postgres"
    echo "POSTGRES_PASSWORD=your-password"
    echo "POSTGRES_DATABASE=postgres"
    echo "NEW_RELIC_API_KEY=NRAK-XXXXXXXXXXXXX"
    echo ""
    exit 1
fi

# Optional environment variables with defaults
export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_DATABASE="${POSTGRES_DATABASE:-postgres}"
export OTLP_ENDPOINT="${OTLP_ENDPOINT:-localhost:4317}"
export COLLECTOR_LOG_LEVEL="${COLLECTOR_LOG_LEVEL:-info}"

# Determine which distribution to use
DISTRIBUTION="${1:-enterprise}"
CONFIG_FILE="${2:-configs/collector-with-secrets.yaml}"

echo ""
echo "Configuration:"
echo "- Distribution: $DISTRIBUTION"
echo "- Config file: $CONFIG_FILE"
echo "- Log level: $COLLECTOR_LOG_LEVEL"

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "ERROR: Configuration file not found: $CONFIG_FILE"
    exit 1
fi

# Build the collector if needed
COLLECTOR_BINARY="distributions/$DISTRIBUTION/database-intelligence-$DISTRIBUTION"

if [ ! -f "$COLLECTOR_BINARY" ]; then
    echo ""
    echo "Building $DISTRIBUTION distribution..."
    make build-$DISTRIBUTION
fi

# Run pre-flight checks
echo ""
echo "Running pre-flight checks..."

# Test PostgreSQL connection
echo -n "Testing PostgreSQL connection... "
if PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DATABASE" -c "SELECT 1" >/dev/null 2>&1; then
    echo "✅ Success"
else
    echo "❌ Failed"
    echo "Could not connect to PostgreSQL. Please check your connection settings."
fi

# Test New Relic API key (if endpoint is set)
if [ -n "$NEW_RELIC_API_KEY" ] && [ "$NEW_RELIC_API_KEY" != "dummy" ]; then
    echo -n "Testing New Relic API key... "
    if curl -s -H "Api-Key: $NEW_RELIC_API_KEY" https://api.newrelic.com/v2/applications.json >/dev/null 2>&1; then
        echo "✅ Valid"
    else
        echo "⚠️  Could not validate (may still work)"
    fi
fi

# Create runtime directory
RUNTIME_DIR="runtime/$DISTRIBUTION"
mkdir -p "$RUNTIME_DIR"

# Copy config to runtime directory with timestamp
RUNTIME_CONFIG="$RUNTIME_DIR/config-$(date +%Y%m%d-%H%M%S).yaml"
cp "$CONFIG_FILE" "$RUNTIME_CONFIG"
echo ""
echo "Runtime config: $RUNTIME_CONFIG"

# Set up signal handling
trap 'echo ""; echo "Shutting down collector..."; kill $COLLECTOR_PID 2>/dev/null; exit' INT TERM

# Start the collector
echo ""
echo "Starting Database Intelligence Collector..."
echo "=========================================="
echo ""

# Run collector with secure configuration
"$COLLECTOR_BINARY" --config "$RUNTIME_CONFIG" &
COLLECTOR_PID=$!

# Wait a moment for startup
sleep 2

# Check if collector is still running
if kill -0 $COLLECTOR_PID 2>/dev/null; then
    echo ""
    echo "✅ Collector started successfully (PID: $COLLECTOR_PID)"
    echo ""
    echo "Monitoring endpoints:"
    echo "- Health check: http://localhost:13133/health"
    echo "- Metrics:      http://localhost:8888/metrics"
    echo "- zPages:       http://localhost:55679/debug/tracez"
    echo ""
    echo "Press Ctrl+C to stop the collector"
    echo ""
    
    # Monitor health endpoint
    if command -v curl >/dev/null 2>&1; then
        echo "Waiting for health endpoint..."
        for i in {1..30}; do
            if curl -s http://localhost:13133/health >/dev/null 2>&1; then
                echo "✅ Health endpoint is ready"
                
                # Show health status
                echo ""
                echo "Health Status:"
                curl -s http://localhost:13133/health | python3 -m json.tool 2>/dev/null || echo "Could not format health response"
                break
            fi
            sleep 1
        done
    fi
    
    # Wait for collector to exit
    wait $COLLECTOR_PID
else
    echo ""
    echo "❌ Collector failed to start. Check the logs above for errors."
    exit 1
fi