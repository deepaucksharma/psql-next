#!/bin/bash
# Script to start all database containers with their OpenTelemetry collectors

set -e

# Check if NEW_RELIC_LICENSE_KEY is set
if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
    echo "Error: NEW_RELIC_LICENSE_KEY environment variable is not set"
    echo "Please export NEW_RELIC_LICENSE_KEY=your_license_key"
    exit 1
fi

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting all database containers...${NC}"

# Start databases first
echo -e "${YELLOW}Starting databases...${NC}"
docker-compose -f docker-compose.databases.yml up -d \
    postgres mysql mongodb mssql oracle

# Wait for databases to be healthy
echo -e "${YELLOW}Waiting for databases to be healthy...${NC}"
sleep 30

# Check health status
echo -e "${YELLOW}Checking database health...${NC}"
docker-compose -f docker-compose.databases.yml ps

# Start collectors
echo -e "${YELLOW}Starting OpenTelemetry collectors...${NC}"
docker-compose -f docker-compose.databases.yml up -d \
    otel-collector-postgres \
    otel-collector-mysql \
    otel-collector-mongodb \
    otel-collector-mssql \
    otel-collector-oracle

# Wait for collectors to start
sleep 10

# Show status
echo -e "${GREEN}All services started!${NC}"
echo ""
echo "Database endpoints:"
echo "  PostgreSQL: localhost:5432 (user: postgres, pass: postgres123)"
echo "  MySQL:      localhost:3306 (user: root, pass: mysql123)"
echo "  MongoDB:    localhost:27017 (user: admin, pass: mongo123)"
echo "  MSSQL:      localhost:1433 (user: sa, pass: MsSql!123)"
echo "  Oracle:     localhost:1521 (user: system, pass: oracle123)"
echo ""
echo "Prometheus metrics endpoints:"
echo "  PostgreSQL: http://localhost:8888/metrics"
echo "  MySQL:      http://localhost:8889/metrics"
echo "  MongoDB:    http://localhost:8890/metrics"
echo "  MSSQL:      http://localhost:8893/metrics"
echo "  Oracle:     http://localhost:8894/metrics"
echo ""
echo "Health check endpoints:"
echo "  PostgreSQL: http://localhost:13133/health"
echo "  MySQL:      http://localhost:13134/health"
echo "  MongoDB:    http://localhost:13135/health"
echo "  MSSQL:      http://localhost:13136/health"
echo "  Oracle:     http://localhost:13137/health"