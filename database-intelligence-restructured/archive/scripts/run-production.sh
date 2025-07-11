#!/bin/bash

# Production Runner for Database Intelligence Platform
# Demonstrates all integrated features in action

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}     Database Intelligence Platform - Production Runner          ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo "This script demonstrates all integrated features:"
echo "✓ Connection Pooling      ✓ Health Monitoring"
echo "✓ Rate Limiting          ✓ Secrets Management"
echo "✓ Circuit Breakers       ✓ Adaptive Sampling"
echo ""

# Check environment
echo -e "${YELLOW}Checking environment...${NC}"

# Load .env if exists
if [ -f ".env" ]; then
    echo "Loading environment from .env file..."
    set -a
    source .env
    set +a
fi

# Required variables
REQUIRED_VARS=(
    "POSTGRES_HOST"
    "POSTGRES_PORT"
    "POSTGRES_USER"
    "POSTGRES_PASSWORD"
    "POSTGRES_DATABASE"
    "NEW_RELIC_API_KEY"
)

# Check required variables
MISSING=()
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING+=("$var")
    fi
done

if [ ${#MISSING[@]} -ne 0 ]; then
    echo -e "${RED}Missing required environment variables:${NC}"
    printf '%s\n' "${MISSING[@]}"
    echo ""
    echo "Please create a .env file with:"
    echo "POSTGRES_HOST=localhost"
    echo "POSTGRES_PORT=5432"
    echo "POSTGRES_USER=postgres"
    echo "POSTGRES_PASSWORD=your-password"
    echo "POSTGRES_DATABASE=postgres"
    echo "NEW_RELIC_API_KEY=NRAK-XXXXX"
    exit 1
fi

echo -e "${GREEN}✓ Environment configured${NC}"

# Set defaults
export ENVIRONMENT="${ENVIRONMENT:-production}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export MONITORING_ADDR="${MONITORING_ADDR:-:8080}"

# Display configuration
echo ""
echo -e "${BLUE}Configuration:${NC}"
echo "├─ Environment: $ENVIRONMENT"
echo "├─ Log Level: $LOG_LEVEL"
echo "├─ Monitoring: $MONITORING_ADDR"
echo "├─ Database: $POSTGRES_HOST:$POSTGRES_PORT/$POSTGRES_DATABASE"
echo "└─ New Relic: Configured"

# Test database connection
echo ""
echo -e "${YELLOW}Testing database connection...${NC}"
if PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DATABASE" -c "SELECT version();" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Database connection successful${NC}"
else
    echo -e "${RED}✗ Database connection failed${NC}"
    echo "Please check your PostgreSQL settings"
    exit 1
fi

# Check pg_stat_statements
if PGPASSWORD="$POSTGRES_PASSWORD" psql -h "$POSTGRES_HOST" -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DATABASE" -c "SELECT * FROM pg_stat_statements LIMIT 1;" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ pg_stat_statements enabled${NC}"
else
    echo -e "${YELLOW}! pg_stat_statements not enabled${NC}"
    echo "  Some features may be limited"
fi

# Build production distribution if needed
BINARY="distributions/production/database-intelligence-production"
if [ ! -f "$BINARY" ]; then
    echo ""
    echo -e "${YELLOW}Building production distribution...${NC}"
    (cd distributions/production && go build -o database-intelligence-production .)
fi

# Create runtime directory
RUNTIME_DIR="runtime/production"
mkdir -p "$RUNTIME_DIR/logs"

# Copy production config
CONFIG="configs/production.yaml"
if [ ! -f "$CONFIG" ]; then
    echo -e "${RED}Production config not found: $CONFIG${NC}"
    exit 1
fi

# Start collector
echo ""
echo -e "${BLUE}Starting Database Intelligence Collector...${NC}"
echo "═══════════════════════════════════════════════════════════════"

# Run with all features enabled
LOG_FILE="$RUNTIME_DIR/logs/collector-$(date +%Y%m%d-%H%M%S).log"

# Function to monitor endpoints
monitor_endpoints() {
    sleep 5  # Wait for startup
    
    echo ""
    echo -e "${BLUE}Monitoring Endpoints:${NC}"
    echo "├─ Health:  http://localhost:8080/health"
    echo "├─ Metrics: http://localhost:8080/metrics"
    echo "├─ Info:    http://localhost:8080/info"
    echo "├─ Internal: http://localhost:8888/metrics"
    echo "└─ Debug:   http://localhost:55679/debug/tracez"
    
    # Check health
    echo ""
    echo -e "${YELLOW}Checking health status...${NC}"
    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        HEALTH=$(curl -s http://localhost:8080/health | python3 -m json.tool 2>/dev/null || echo "{}")
        echo -e "${GREEN}✓ Health endpoint active${NC}"
        
        # Show component status
        if command -v jq >/dev/null 2>&1; then
            echo ""
            echo "Component Status:"
            echo "$HEALTH" | jq -r '.components | to_entries | .[] | "├─ \(.key): \(.value.healthy)"' 2>/dev/null || true
        fi
    fi
    
    # Show info
    echo ""
    if curl -s http://localhost:8080/info >/dev/null 2>&1; then
        INFO=$(curl -s http://localhost:8080/info | python3 -m json.tool 2>/dev/null || echo "{}")
        echo "Service Info:"
        echo "$INFO" | grep -E '"service"|"version"|"uptime"' || true
    fi
}

# Signal handling
trap 'echo ""; echo "Shutting down..."; kill $PID 2>/dev/null; exit' INT TERM

# Start collector
echo ""
echo "Starting collector (logs: $LOG_FILE)"
echo ""
"$BINARY" --config "$CONFIG" 2>&1 | tee "$LOG_FILE" &
PID=$!

# Monitor in background
monitor_endpoints &

# Show real-time logs with feature highlights
echo ""
echo -e "${BLUE}Watching for feature activation...${NC}"
echo "─────────────────────────────────────────────────"

tail -f "$LOG_FILE" | grep -E --color=always "(pool|health|rate.limit|secret|circuit|sampling|Starting|failed|error)" &
TAIL_PID=$!

# Wait for collector
wait $PID

# Cleanup
kill $TAIL_PID 2>/dev/null || true

echo ""
echo -e "${BLUE}Collector stopped${NC}"