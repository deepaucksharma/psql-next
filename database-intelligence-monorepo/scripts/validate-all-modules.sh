#!/bin/bash

# Database Intelligence System - Comprehensive Validation Script
# This script validates all modules are running and sending data to New Relic

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NR_API_KEY="${NEW_RELIC_API_KEY}"
NR_ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"
GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}Database Intelligence Validation${NC}"
echo -e "${BLUE}================================${NC}\n"

# Function to check if a container is running
check_container() {
    local module=$1
    local container_name=$2
    
    if docker ps --format "{{.Names}}" | grep -q "$container_name"; then
        echo -e "${GREEN}✓${NC} $module container is running"
        return 0
    else
        echo -e "${RED}✗${NC} $module container is NOT running"
        return 1
    fi
}

# Function to query New Relic for module metrics
check_metrics() {
    local module=$1
    local query="{ actor { account(id: $NR_ACCOUNT_ID) { nrql(query: \"SELECT count(*) FROM Metric WHERE module = '$module' SINCE 2 minutes ago\") { results } } } }"
    
    local response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
        -H 'Content-Type: application/json' \
        -H "API-Key: $NR_API_KEY" \
        -d "{\"query\": \"$query\"}")
    
    local count=$(echo "$response" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['data']['actor']['account']['nrql']['results'][0]['count'] if data['data']['actor']['account']['nrql']['results'] else 0)")
    
    if [ "$count" -gt 0 ]; then
        echo -e "${GREEN}✓${NC} $module is sending metrics to New Relic: ${GREEN}$count${NC} in last 2 minutes"
        return 0
    else
        echo -e "${RED}✗${NC} $module is NOT sending metrics to New Relic"
        return 1
    fi
}

# Check Docker containers
echo -e "${YELLOW}Checking Docker Containers:${NC}"
echo "----------------------------"

MODULES=(
    "core-metrics:core-metrics-core-metrics-1"
    "sql-intelligence:sql-intelligence-sql-intelligence-1"
    "wait-profiler:wait-profiler-wait-profiler-1"
    "anomaly-detector:anomaly-detector-anomaly-detector-1"
    "business-impact:business-impact-business-impact-1"
    "performance-advisor:performance-advisor-performance-advisor-1"
    "resource-monitor:resource-monitor-collector"
    "replication-monitor:replication-monitor-replication-monitor-1"
)

running_count=0
for module_info in "${MODULES[@]}"; do
    IFS=':' read -r module container <<< "$module_info"
    if check_container "$module" "$container"; then
        ((running_count++))
    fi
done

echo -e "\nContainers running: ${GREEN}$running_count${NC}/8\n"

# Check New Relic metrics
echo -e "${YELLOW}Checking New Relic Data Flow:${NC}"
echo "-----------------------------"

# Get comprehensive metrics summary
query='{ actor { account(id: 3630072) { nrql(query: "SELECT count(*) FROM Metric WHERE module IS NOT NULL SINCE 5 minutes ago FACET module ORDER BY count DESC") { results } } } }'

response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
    -H 'Content-Type: application/json' \
    -H "API-Key: $NR_API_KEY" \
    -d "{\"query\": \"$query\"}")

echo "$response" | python3 -c "
import sys, json
data = json.load(sys.stdin)
results = data['data']['actor']['account']['nrql']['results']
total = 0
for result in results:
    module = result['module']
    count = result['count']
    total += count
    print(f'✓ {module}: {count} metrics')
print(f'\nTotal metrics in last 5 minutes: {total}')
"

# Check specific metric types
echo -e "\n${YELLOW}Checking Metric Types:${NC}"
echo "----------------------"

# Core metrics types
echo -e "\n${BLUE}Core Metrics:${NC}"
query='{ actor { account(id: 3630072) { nrql(query: "SELECT uniques(metricName) FROM Metric WHERE module = '\''core-metrics'\'' SINCE 5 minutes ago LIMIT 5") { results } } } }'
response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" -H 'Content-Type: application/json' -H "API-Key: $NR_API_KEY" -d "{\"query\": \"$query\"}")
echo "$response" | python3 -c "
import sys, json
data = json.load(sys.stdin)
metrics = data['data']['actor']['account']['nrql']['results'][0]['uniques.metricName'][:5] if data['data']['actor']['account']['nrql']['results'] else []
for metric in metrics:
    print(f'  - {metric}')
"

# Resource monitor types
echo -e "\n${BLUE}Resource Monitor:${NC}"
query='{ actor { account(id: 3630072) { nrql(query: "SELECT uniques(metricName) FROM Metric WHERE module = '\''resource-monitor'\'' SINCE 5 minutes ago LIMIT 5") { results } } } }'
response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" -H 'Content-Type: application/json' -H "API-Key: $NR_API_KEY" -d "{\"query\": \"$query\"}")
echo "$response" | python3 -c "
import sys, json
data = json.load(sys.stdin)
metrics = data['data']['actor']['account']['nrql']['results'][0]['uniques.metricName'][:5] if data['data']['actor']['account']['nrql']['results'] else []
for metric in metrics:
    print(f'  - {metric}')
"

# System health check
echo -e "\n${YELLOW}System Health Status:${NC}"
echo "--------------------"

# Check for anomalies
query='{ actor { account(id: 3630072) { nrql(query: "SELECT count(*) FROM Metric WHERE module = '\''anomaly-detector'\'' AND metricName LIKE '\''%anomaly%'\'' SINCE 10 minutes ago") { results } } } }'
response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" -H 'Content-Type: application/json' -H "API-Key: $NR_API_KEY" -d "{\"query\": \"$query\"}")
anomaly_count=$(echo "$response" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['data']['actor']['account']['nrql']['results'][0]['count'] if data['data']['actor']['account']['nrql']['results'] else 0)")

if [ "$anomaly_count" -gt 0 ]; then
    echo -e "${YELLOW}⚠${NC}  Anomaly detector has found $anomaly_count anomalies"
else
    echo -e "${GREEN}✓${NC} No anomalies detected"
fi

# Final summary
echo -e "\n${BLUE}================================${NC}"
echo -e "${BLUE}Validation Summary${NC}"
echo -e "${BLUE}================================${NC}"

if [ "$running_count" -ge 4 ] && [ "$total" -gt 0 ]; then
    echo -e "${GREEN}✓ System is operational${NC}"
    echo -e "  - $running_count/8 modules running"
    echo -e "  - Data flowing to New Relic"
    echo -e "  - Ready for production use"
else
    echo -e "${RED}✗ System needs attention${NC}"
    echo -e "  - Only $running_count/8 modules running"
    echo -e "  - Check container logs for errors"
fi

echo -e "\n${BLUE}Next Steps:${NC}"
echo "1. View dashboards at: https://one.newrelic.com"
echo "2. Check module logs: docker logs <container-name>"
echo "3. Run integration tests: make integration-test"