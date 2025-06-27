#!/bin/bash

# PostgreSQL Unified Collector - End-to-End Test Script
# This script runs the collector in hybrid mode with both NRI and OTLP outputs

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}PostgreSQL Unified Collector - End-to-End Test${NC}"
echo "================================================"

# Check for required environment variables
if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
    echo -e "${RED}Error: NEW_RELIC_LICENSE_KEY environment variable is not set${NC}"
    echo "Please set: export NEW_RELIC_LICENSE_KEY=your_license_key"
    exit 1
fi

# Update collector config with proper connection settings
echo -e "\n${YELLOW}1. Updating collector configuration...${NC}"
cat > collector-config.toml << EOF
# PostgreSQL Unified Collector Configuration

# Connection settings
connection_string = "postgresql://postgres:postgres@localhost:5432/testdb"
host = "localhost"
port = 5432
databases = ["testdb", "postgres"]
max_connections = 5
connect_timeout_secs = 30

# Collection settings
collection_interval_secs = 30
collection_mode = "hybrid"  # Send to both NRI and OTLP

# OHI compatibility settings
query_monitoring_count_threshold = 10
query_monitoring_response_time_threshold = 100
max_query_length = 4096

# Extended metrics
enable_extended_metrics = true
enable_ebpf = false
enable_ash = true
ash_sample_interval_secs = 1
ash_retention_hours = 1

# Output configuration
[outputs.nri]
enabled = true
entity_key = "localhost:5432"
integration_name = "com.newrelic.postgresql"

[outputs.otlp]
enabled = true
endpoint = "http://localhost:4317"
compression = "gzip"
timeout_secs = 30
headers = [
    ["service.name", "postgresql-unified"],
]

# Sampling configuration
[sampling]
mode = "fixed"
base_sample_rate = 1.0

[[sampling.rules]]
condition = "query_duration > 500"
sample_rate = 1.0
EOF

# Start infrastructure components
echo -e "\n${YELLOW}2. Starting infrastructure components...${NC}"
docker-compose up -d postgres otel-collector

# Wait for PostgreSQL to be ready
echo -e "\n${YELLOW}3. Waiting for PostgreSQL to be ready...${NC}"
for i in {1..30}; do
    if PGPASSWORD=postgres psql -h localhost -U postgres -d postgres -c "SELECT 1" > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

# Verify pg_stat_statements is enabled
echo -e "\n${YELLOW}4. Verifying PostgreSQL extensions...${NC}"
PGPASSWORD=postgres psql -h localhost -U postgres -d testdb << EOF
SELECT name, installed_version 
FROM pg_available_extensions 
WHERE name IN ('pg_stat_statements', 'pgcrypto') 
ORDER BY name;
EOF

# Build the collector if needed
echo -e "\n${YELLOW}5. Building the unified collector...${NC}"
if [ ! -f "target/release/postgres-unified-collector" ]; then
    cargo build --release
fi

# Generate initial load
echo -e "\n${YELLOW}6. Generating initial PostgreSQL load...${NC}"
./run-load-test.sh &
LOAD_PID=$!

# Start the unified collector
echo -e "\n${YELLOW}7. Starting PostgreSQL Unified Collector in hybrid mode...${NC}"
echo "Logs will be displayed below. Press Ctrl+C to stop."
echo "-------------------------------------------------------"

# Export OTLP endpoint for the collector
export OTLP_ENDPOINT="http://localhost:4317"

# Run the collector with debug logging
RUST_LOG=debug ./target/release/postgres-unified-collector \
    --config collector-config.toml \
    --mode hybrid \
    --debug &

COLLECTOR_PID=$!

# Function to cleanup on exit
cleanup() {
    echo -e "\n${YELLOW}Shutting down...${NC}"
    kill $COLLECTOR_PID 2>/dev/null || true
    kill $LOAD_PID 2>/dev/null || true
    echo -e "${GREEN}Collector stopped${NC}"
}

trap cleanup EXIT

# Monitor the collector
echo -e "\n${GREEN}Collector is running!${NC}"
echo "===================================="
echo "Metrics are being sent to:"
echo "- New Relic Infrastructure (NRI format)"
echo "- New Relic OTLP endpoint (via local OTel Collector)"
echo ""
echo "To view metrics in New Relic:"
echo "1. Infrastructure: https://one.newrelic.com/infrastructure"
echo "2. APM & Services: https://one.newrelic.com/services-map"
echo "3. Metrics Explorer: https://one.newrelic.com/metrics-explorer"
echo ""
echo "Health check endpoint: http://localhost:8080/health"
echo ""
echo "Press Ctrl+C to stop..."

# Keep the script running
wait $COLLECTOR_PID