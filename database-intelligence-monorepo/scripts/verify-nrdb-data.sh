#!/bin/bash

# Script to verify that all modules are sending data to New Relic
# Requires NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID to be set

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NR_API_KEY="${NEW_RELIC_API_KEY}"
NR_ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID:-3630072}"
GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}New Relic Database (NRDB) Data Verification${NC}"
echo -e "${BLUE}============================================${NC}"

# Check if API key is set
if [ -z "$NR_API_KEY" ]; then
    echo -e "${RED}ERROR: NEW_RELIC_API_KEY not set${NC}"
    echo "Please set: export NEW_RELIC_API_KEY=your-api-key"
    exit 1
fi

echo -e "\nAccount ID: ${GREEN}$NR_ACCOUNT_ID${NC}"
echo -e "Time Range: Last 30 minutes\n"

# Function to query NRDB
query_nrdb() {
    local nrql_query=$1
    local description=$2
    
    local query=$(cat <<EOF
{
  "query": "{ actor { account(id: $NR_ACCOUNT_ID) { nrql(query: \"$nrql_query\") { results } } } }"
}
EOF
)
    
    local response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
        -H 'Content-Type: application/json' \
        -H "API-Key: $NR_API_KEY" \
        -d "$query")
    
    echo "$response"
}

# Check overall metric ingestion
echo -e "${YELLOW}1. Overall Metric Ingestion (Last 30 minutes)${NC}"
echo "-------------------------------------------"

QUERY="SELECT count(*) FROM Metric WHERE module IS NOT NULL SINCE 30 minutes ago FACET module ORDER BY count DESC"
response=$(query_nrdb "$QUERY" "Module metrics")

echo "$response" | python3 -c "
import sys, json

try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results:
        print('${RED}✗ No metrics found${NC}')
    else:
        total = 0
        for result in results:
            module = result.get('module', 'unknown')
            count = result.get('count', 0)
            total += count
            
            if count > 1000:
                color = '${GREEN}'
            elif count > 100:
                color = '${YELLOW}'
            else:
                color = '${RED}'
            
            print(f'{color}✓{NC} {module}: {count:,} metrics')
        
        print(f'\nTotal metrics: {total:,}')
except Exception as e:
    print(f'${RED}Error parsing response: {e}${NC}')
    print('Response:', data if 'data' in locals() else 'No data')
"

# Check specific metric types per module
echo -e "\n${YELLOW}2. Module-Specific Metrics (Last 5 minutes)${NC}"
echo "--------------------------------------------"

MODULES=(
    "core-metrics"
    "sql-intelligence"
    "wait-profiler"
    "anomaly-detector"
    "business-impact"
    "replication-monitor"
    "performance-advisor"
    "resource-monitor"
    "alert-manager"
    "canary-tester"
    "cross-signal-correlator"
)

for module in "${MODULES[@]}"; do
    echo -e "\n${BLUE}$module:${NC}"
    
    # Get sample metrics
    QUERY="SELECT uniques(metricName) FROM Metric WHERE module = '$module' SINCE 5 minutes ago LIMIT 10"
    response=$(query_nrdb "$QUERY" "$module metrics")
    
    echo "$response" | python3 -c "
import sys, json

try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results or not results[0].get('uniques.metricName'):
        print('  ${RED}✗ No metrics found${NC}')
    else:
        metrics = results[0]['uniques.metricName'][:5]  # Show first 5
        for metric in metrics:
            print(f'  ${GREEN}✓${NC} {metric}')
        
        if len(results[0]['uniques.metricName']) > 5:
            print(f'  ... and {len(results[0][\"uniques.metricName\"]) - 5} more')
except Exception as e:
    print(f'  ${RED}✗ Error: {e}${NC}')
"
done

# Check MySQL connection metrics
echo -e "\n${YELLOW}3. MySQL Connection Status${NC}"
echo "-------------------------"

QUERY="SELECT latest(mysql.connection.count) FROM Metric SINCE 5 minutes ago FACET module"
response=$(query_nrdb "$QUERY" "MySQL connections")

echo "$response" | python3 -c "
import sys, json

try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results:
        print('${RED}✗ No MySQL connection metrics found${NC}')
    else:
        for result in results:
            module = result.get('module', 'unknown')
            value = result.get('latest.mysql.connection.count', 0)
            
            if value > 0:
                print(f'${GREEN}✓${NC} {module}: {value} connections')
            else:
                print(f'${RED}✗${NC} {module}: No connections')
except Exception as e:
    print(f'${RED}Error: {e}${NC}')
"

# Check for errors or warnings
echo -e "\n${YELLOW}4. Recent Errors/Warnings${NC}"
echo "------------------------"

QUERY="SELECT count(*) FROM Metric WHERE attributes.alert.severity IS NOT NULL SINCE 10 minutes ago FACET attributes.alert.severity, module"
response=$(query_nrdb "$QUERY" "Alerts")

echo "$response" | python3 -c "
import sys, json

try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results:
        print('${GREEN}✓ No alerts found${NC}')
    else:
        for result in results:
            severity = result.get('attributes.alert.severity', 'unknown')
            module = result.get('module', 'unknown')
            count = result.get('count', 0)
            
            if severity == 'critical':
                color = '${RED}'
            elif severity == 'warning':
                color = '${YELLOW}'
            else:
                color = '${GREEN}'
            
            print(f'{color}⚠{NC} {module}: {count} {severity} alerts')
except Exception as e:
    print(f'${RED}Error: {e}${NC}')
"

# Check New Relic entity synthesis
echo -e "\n${YELLOW}5. Entity Synthesis Status${NC}"
echo "-------------------------"

QUERY="SELECT count(*) FROM Metric WHERE entity.guid IS NOT NULL SINCE 5 minutes ago FACET entity.type"
response=$(query_nrdb "$QUERY" "Entity synthesis")

echo "$response" | python3 -c "
import sys, json

try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results:
        print('${RED}✗ No entities synthesized${NC}')
    else:
        for result in results:
            entity_type = result.get('entity.type', 'unknown')
            count = result.get('count', 0)
            print(f'${GREEN}✓${NC} {entity_type}: {count} metrics')
except Exception as e:
    print(f'${RED}Error: {e}${NC}')
"

# Summary and recommendations
echo -e "\n${BLUE}============================================${NC}"
echo -e "${BLUE}Summary and Recommendations${NC}"
echo -e "${BLUE}============================================${NC}"

# Check which modules are NOT sending data
echo -e "\n${YELLOW}Modules Status:${NC}"

for module in "${MODULES[@]}"; do
    QUERY="SELECT count(*) FROM Metric WHERE module = '$module' SINCE 2 minutes ago"
    response=$(query_nrdb "$QUERY" "$module check")
    
    count=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(results[0]['count'] if results else 0)
except:
    print(0)
")
    
    if [ "$count" -gt 0 ]; then
        echo -e "${GREEN}✓${NC} $module: Active (${count} metrics)"
    else
        echo -e "${RED}✗${NC} $module: No recent data"
    fi
done

echo -e "\n${YELLOW}Next Steps:${NC}"
echo "1. For modules with no data:"
echo "   - Check if container is running: docker ps | grep <module>"
echo "   - Check logs: docker logs <module-container>"
echo "   - Verify New Relic API key is set correctly"
echo ""
echo "2. View in New Relic UI:"
echo "   - Metrics Explorer: https://one.newrelic.com/metrics-explorer"
echo "   - Query Builder: https://one.newrelic.com/data-exploration"
echo "   - Use query: FROM Metric SELECT * WHERE module IS NOT NULL"
echo ""
echo "3. Check specific module data:"
echo "   - Example: FROM Metric SELECT * WHERE module = 'core-metrics' SINCE 30 minutes ago"