#!/bin/bash

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Testing MySQL Connections and OpenTelemetry Collector..."
echo "========================================================"

# Test MySQL Primary
echo -e "\n${YELLOW}Testing MySQL Primary...${NC}"
if docker compose exec -T mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SELECT 'Primary is accessible' as status;" 2>/dev/null; then
    echo -e "${GREEN}✓ MySQL Primary connection successful${NC}"
else
    echo -e "${RED}✗ MySQL Primary connection failed${NC}"
fi

# Test MySQL Replica
echo -e "\n${YELLOW}Testing MySQL Replica...${NC}"
if docker compose exec -T mysql-replica mysql -uotel_monitor -potelmonitorpass -e "SELECT 'Replica is accessible' as status;" 2>/dev/null; then
    echo -e "${GREEN}✓ MySQL Replica connection successful${NC}"
else
    echo -e "${RED}✗ MySQL Replica connection failed${NC}"
fi

# Check Replication Status
echo -e "\n${YELLOW}Checking Replication Status...${NC}"
SLAVE_IO=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep "Slave_IO_Running:" | awk '{print $2}')
SLAVE_SQL=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep "Slave_SQL_Running:" | awk '{print $2}')
LAG=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep "Seconds_Behind_Master:" | awk '{print $2}')

if [ "$SLAVE_IO" = "Yes" ] && [ "$SLAVE_SQL" = "Yes" ]; then
    echo -e "${GREEN}✓ Replication is running (Lag: ${LAG}s)${NC}"
else
    echo -e "${RED}✗ Replication is not running properly${NC}"
    echo "  Slave_IO_Running: $SLAVE_IO"
    echo "  Slave_SQL_Running: $SLAVE_SQL"
fi

# Test OTel Collector Health
echo -e "\n${YELLOW}Testing OpenTelemetry Collector...${NC}"
if curl -s http://localhost:13133/ | grep -q "Server available"; then
    echo -e "${GREEN}✓ OTel Collector is healthy${NC}"
else
    echo -e "${RED}✗ OTel Collector health check failed${NC}"
fi

# Check if metrics are being collected
echo -e "\n${YELLOW}Checking metric collection...${NC}"
COLLECTOR_LOGS=$(docker compose logs otel-collector --tail=20 2>&1)
if echo "$COLLECTOR_LOGS" | grep -q "MetricsExporter.*otlp/newrelic.*success"; then
    echo -e "${GREEN}✓ Metrics are being exported to New Relic${NC}"
elif echo "$COLLECTOR_LOGS" | grep -q "error"; then
    echo -e "${RED}✗ Errors found in collector logs${NC}"
    echo "Recent errors:"
    echo "$COLLECTOR_LOGS" | grep -i error | tail -5
else
    echo -e "${YELLOW}⚠ Cannot determine export status. Check logs with: docker compose logs otel-collector${NC}"
fi

# Performance Schema Status
echo -e "\n${YELLOW}Checking Performance Schema...${NC}"
PS_STATUS=$(docker compose exec -T mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SHOW VARIABLES LIKE 'performance_schema';" 2>/dev/null | grep performance_schema | awk '{print $2}')
if [ "$PS_STATUS" = "ON" ]; then
    echo -e "${GREEN}✓ Performance Schema is enabled${NC}"
else
    echo -e "${RED}✗ Performance Schema is disabled${NC}"
fi

echo -e "\n${YELLOW}Summary:${NC}"
echo "- MySQL endpoints: localhost:3306 (primary), localhost:3307 (replica)"
echo "- OTel Collector health: http://localhost:13133/"
echo "- OTel Collector zPages: http://localhost:55679/"
echo ""
echo "To view real-time logs:"
echo "  docker compose logs -f otel-collector"