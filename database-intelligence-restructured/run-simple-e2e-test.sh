#!/bin/bash

# Simple E2E Test Runner

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== SIMPLE E2E TEST ===${NC}"

# Check if we have a working collector
COLLECTOR=""
if [ -f "./e2e-collector" ]; then
    COLLECTOR="./e2e-collector"
elif [ -f "./simple-e2e-collector/database-intelligence-e2e" ]; then
    COLLECTOR="./simple-e2e-collector/database-intelligence-e2e"
else
    echo -e "${YELLOW}[!]${NC} No collector found, using test mode"
fi

# Start databases
echo -e "\n${YELLOW}Starting databases...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml up -d

# Wait for databases
echo -e "${YELLOW}Waiting for databases...${NC}"
sleep 15

# Test database connectivity
echo -e "\n${BLUE}Testing Database Connectivity${NC}"

# PostgreSQL
if docker exec db-intel-postgres pg_isready -U postgres; then
    echo -e "${GREEN}[✓]${NC} PostgreSQL is ready"
    
    # Run test query
    docker exec db-intel-postgres psql -U postgres -c "SELECT version();"
else
    echo -e "${RED}[✗]${NC} PostgreSQL not ready"
fi

# MySQL
if docker exec db-intel-mysql mysqladmin ping -h localhost -u root -ppassword 2>/dev/null; then
    echo -e "${GREEN}[✓]${NC} MySQL is ready"
    
    # Run test query
    docker exec db-intel-mysql mysql -u root -ppassword -e "SELECT VERSION();"
else
    echo -e "${RED}[✗]${NC} MySQL not ready"
fi

# If we have a collector, try to run it
if [ -n "$COLLECTOR" ]; then
    echo -e "\n${YELLOW}Starting collector...${NC}"
    
    # Create simple config
    cat > simple-test-config.yaml << 'CONFIG'
extensions:
  health_check:

receivers:
  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
      memory:

processors:
  batch:

exporters:
  debug:
    verbosity: detailed

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [batch]
      exporters: [debug]
CONFIG
    
    # Run collector for 30 seconds
    timeout 30s "$COLLECTOR" --config=simple-test-config.yaml || true
    
    rm -f simple-test-config.yaml
fi

# Stop databases
echo -e "\n${YELLOW}Stopping databases...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml down

echo -e "\n${GREEN}Simple E2E test completed!${NC}"
