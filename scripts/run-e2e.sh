#!/bin/bash
# End-to-End PostgreSQL Unified Collector Test with New Relic Verification

set -e

# Add PostgreSQL to PATH if needed
export PATH="/opt/homebrew/opt/postgresql@16/bin:$PATH"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}PostgreSQL Unified Collector - End-to-End Test${NC}"
echo "=============================================="

# Check for .env file
if [ ! -f .env ]; then
    echo -e "${RED}Error: .env file not found!${NC}"
    echo "Creating .env from template..."
    cp .env.example .env
    echo -e "${YELLOW}Please edit .env with your New Relic credentials and run again${NC}"
    exit 1
fi

# Load environment
set -a
source .env
set +a

# Validate credentials
if [ -z "$NEW_RELIC_LICENSE_KEY" ] || [ "$NEW_RELIC_LICENSE_KEY" == "your_license_key_here" ]; then
    echo -e "${RED}Error: Please set NEW_RELIC_LICENSE_KEY in .env${NC}"
    exit 1
fi

if [ -z "$NEW_RELIC_API_KEY" ] || [ "$NEW_RELIC_API_KEY" == "your_nerdgraph_api_key_here" ]; then
    echo -e "${RED}Error: Please set NEW_RELIC_API_KEY in .env for metric verification${NC}"
    exit 1
fi

# Step 1: Clean up any existing containers
echo -e "\n${YELLOW}Step 1: Cleaning up existing containers...${NC}"
docker-compose down -v 2>/dev/null || true

# Step 2: Build the collector
echo -e "\n${YELLOW}Step 2: Building the collector...${NC}"
if [ ! -f "target/release/postgres-unified-collector" ] || [ "$1" == "--rebuild" ]; then
    cargo build --release
else
    echo "Using existing build. Use --rebuild to force rebuild."
fi

# Step 3: Start infrastructure
echo -e "\n${YELLOW}Step 3: Starting PostgreSQL and OTel Collector...${NC}"
docker-compose up -d postgres otel-collector

# Wait for PostgreSQL
echo -n "Waiting for PostgreSQL to be ready"
for i in {1..30}; do
    if PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d postgres -c "SELECT 1" > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

# Step 4: Verify PostgreSQL setup
echo -e "\n${YELLOW}Step 4: Verifying PostgreSQL configuration...${NC}"
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DATABASE << EOF
-- Check extensions
SELECT name, installed_version 
FROM pg_available_extensions 
WHERE name IN ('pg_stat_statements', 'pgcrypto') 
ORDER BY name;

-- Check pg_stat_statements
SELECT count(*) as statement_count FROM pg_stat_statements;
EOF

# Step 5: Generate initial load
echo -e "\n${YELLOW}Step 5: Generating initial PostgreSQL load...${NC}"
./scripts/run-load-test.sh > /tmp/load-test.log 2>&1 &
LOAD_PID=$!
echo "Load generator started (PID: $LOAD_PID)"

# Step 6: Start the collector
echo -e "\n${YELLOW}Step 6: Starting PostgreSQL Unified Collector in ${COLLECTOR_MODE} mode...${NC}"

# Create a collector log file
COLLECTOR_LOG="/tmp/postgres-collector-$(date +%Y%m%d-%H%M%S).log"
echo "Collector log: $COLLECTOR_LOG"

# Start collector in background
RUST_LOG=info ./target/release/postgres-unified-collector \
    --config configs/collector-config.toml \
    --mode ${COLLECTOR_MODE} \
    > "$COLLECTOR_LOG" 2>&1 &

COLLECTOR_PID=$!
echo "Collector started (PID: $COLLECTOR_PID)"

# Function to cleanup on exit
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    kill $COLLECTOR_PID 2>/dev/null || true
    kill $LOAD_PID 2>/dev/null || true
    wait $LOAD_PID 2>/dev/null || true
    echo -e "${GREEN}Cleanup complete${NC}"
}
trap cleanup EXIT

# Step 7: Wait for initial metrics
echo -e "\n${YELLOW}Step 7: Waiting for initial metrics collection...${NC}"
echo "Waiting 60 seconds for first collection cycle..."
for i in {1..60}; do
    printf "\r[%-60s] %d/60" "$(printf '#%.0s' $(seq 1 $i))" "$i"
    sleep 1
done
echo ""

# Step 8: Check collector health
echo -e "\n${YELLOW}Step 8: Checking collector health...${NC}"
if curl -s -f http://localhost:${HEALTH_CHECK_PORT:-8080}/health > /dev/null 2>&1; then
    echo -e "✅ Health endpoint: ${GREEN}OK${NC}"
    curl -s http://localhost:${HEALTH_CHECK_PORT:-8080}/health | jq '.'
else
    echo -e "❌ Health endpoint: ${RED}Not responding${NC}"
    echo "Recent collector logs:"
    tail -20 "$COLLECTOR_LOG"
fi

# Step 9: Generate more load
echo -e "\n${YELLOW}Step 9: Generating additional load patterns...${NC}"
# Run some specific queries to ensure all metric types are generated
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DATABASE << EOF
-- Slow query
SELECT test_schema.simulate_slow_query(2);

-- Create blocking scenario
BEGIN;
UPDATE test_schema.users SET updated_at = NOW() WHERE id = 1;
SELECT pg_sleep(5);
COMMIT;

-- Generate wait events
DO \$\$
DECLARE i INTEGER;
BEGIN
    FOR i IN 1..10 LOOP
        UPDATE test_schema.orders SET status = 'processing' WHERE id = i;
    END LOOP;
END \$\$;
EOF

# Wait for metrics to be sent
echo "Waiting 30 seconds for metrics to be sent to New Relic..."
sleep 30

# Step 10: Verify metrics in New Relic
echo -e "\n${YELLOW}Step 10: Verifying metrics in New Relic...${NC}"
./scripts/verify-metrics.sh

# Step 11: Show collector logs
echo -e "\n${YELLOW}Step 11: Recent collector logs:${NC}"
tail -50 "$COLLECTOR_LOG" | grep -E "(Collected|Sent|ERROR|WARN)" || true

# Step 12: Summary
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}End-to-End Test Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Resources:"
echo "- Collector PID: $COLLECTOR_PID"
echo "- Collector Log: $COLLECTOR_LOG"
echo "- Health Check: http://localhost:${HEALTH_CHECK_PORT:-8080}/health"
echo ""
echo "To continue monitoring:"
echo "1. View live logs: tail -f $COLLECTOR_LOG"
echo "2. Generate more load: ./scripts/run-load-test.sh"
echo "3. Check metrics again: ./scripts/verify-metrics.sh"
echo ""
echo "To stop everything:"
echo "- Press Ctrl+C or run: docker-compose down"

# Keep script running
echo -e "\n${YELLOW}Press Ctrl+C to stop...${NC}"
wait $COLLECTOR_PID