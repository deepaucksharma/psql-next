#!/bin/bash

# PostgreSQL Unified Collector - Run Script
# This script provides a streamlined way to run the collector with proper configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Load environment variables
if [ -f .env ]; then
    set -a
    source .env
    set +a
else
    echo -e "${RED}Error: .env file not found!${NC}"
    echo "Please copy .env.example to .env and configure it with your credentials."
    exit 1
fi

# Validate required environment variables
required_vars=("POSTGRES_CONNECTION_STRING" "NEW_RELIC_LICENSE_KEY")
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo -e "${RED}Error: $var is not set in .env file!${NC}"
        exit 1
    fi
done

# Set DATABASE_URL from POSTGRES_CONNECTION_STRING for backward compatibility
export DATABASE_URL="${POSTGRES_CONNECTION_STRING}"

# Functions
build_collector() {
    echo -e "${YELLOW}Building PostgreSQL Unified Collector...${NC}"
    cargo build --release
    echo -e "${GREEN}Build completed successfully!${NC}"
}

run_collector() {
    local mode=${1:-$COLLECTOR_MODE}
    local config_file=${2:-config-local.toml}
    
    echo -e "${YELLOW}Running collector in $mode mode...${NC}"
    ./target/release/postgres-unified-collector -c "$config_file" -m "$mode"
}

run_with_infra_agent() {
    echo -e "${YELLOW}Running with Infrastructure Agent...${NC}"
    
    # Check if Infrastructure Agent is installed
    if ! command -v newrelic-infra &> /dev/null; then
        echo -e "${RED}Error: New Relic Infrastructure Agent is not installed!${NC}"
        echo "Please install it from: https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent/"
        exit 1
    fi
    
    # Run Infrastructure Agent with our configuration
    newrelic-infra -config newrelic-infra.yml
}

generate_activity() {
    echo -e "${YELLOW}Generating PostgreSQL test activity...${NC}"
    
    # Create test schema and data if needed
    psql "$POSTGRES_CONNECTION_STRING" <<EOF
-- Create test schema if not exists
CREATE SCHEMA IF NOT EXISTS test_schema;

-- Create test table if not exists
CREATE TABLE IF NOT EXISTS test_schema.users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create slow query function
CREATE OR REPLACE FUNCTION test_schema.simulate_slow_query(delay_seconds NUMERIC)
RETURNS void AS \$\$
BEGIN
    PERFORM pg_sleep(delay_seconds);
END;
\$\$ LANGUAGE plpgsql;

-- Generate slow queries
SELECT test_schema.simulate_slow_query(2.5);
SELECT pg_sleep(3), 'Testing slow query';
UPDATE test_schema.users SET email = 'test@example.com' WHERE id = 1;

-- Generate some activity
INSERT INTO test_schema.users (email) 
SELECT 'user' || i || '@example.com' 
FROM generate_series(1, 100) i
ON CONFLICT DO NOTHING;

SELECT COUNT(*) FROM test_schema.users;
EOF
    
    echo -e "${GREEN}Test activity generated!${NC}"
}

test_health() {
    echo -e "${YELLOW}Testing health endpoint...${NC}"
    curl -s http://localhost:${COLLECTOR_PORT:-8080}/health | jq '.' || echo -e "${RED}Health check failed!${NC}"
}

show_help() {
    cat << EOF
PostgreSQL Unified Collector - Run Script

Usage: ./run.sh [command] [options]

Commands:
    build           Build the collector
    run             Run the collector (default: NRI mode)
    run-otlp        Run the collector in OTLP mode
    run-agent       Run with Infrastructure Agent
    generate        Generate test PostgreSQL activity
    test            Run collector and test health endpoint
    help            Show this help message

Options:
    -m, --mode      Collector mode (nri or otlp)
    -c, --config    Config file path (default: config-local.toml)

Examples:
    ./run.sh build
    ./run.sh run
    ./run.sh run -m otlp
    ./run.sh generate
    ./run.sh run-agent

Environment:
    Configure your credentials in .env file (see .env.example)
EOF
}

# Main script logic
case "${1:-run}" in
    build)
        build_collector
        ;;
    run)
        shift
        run_collector "$@"
        ;;
    run-otlp)
        run_collector "otlp" "${2:-config-local.toml}"
        ;;
    run-agent)
        run_with_infra_agent
        ;;
    generate)
        generate_activity
        ;;
    test)
        # Run collector in background
        run_collector &
        COLLECTOR_PID=$!
        sleep 5
        test_health
        kill $COLLECTOR_PID 2>/dev/null || true
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        show_help
        exit 1
        ;;
esac