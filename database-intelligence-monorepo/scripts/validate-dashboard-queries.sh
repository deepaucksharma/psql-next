#!/bin/bash

# Dashboard Query Validation Script
# Validates all NRQL queries in the new dashboard configurations

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NERDGRAPH_URL="https://api.newrelic.com/graphql"

echo -e "${BLUE}🔍 Dashboard Query Validation${NC}"
echo "====================================="

# Check environment
if [[ -z "${NEW_RELIC_API_KEY:-}" ]] || [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo -e "${YELLOW}⚠️  Missing environment variables. Set:${NC}"
    echo "  export NEW_RELIC_API_KEY='your-user-api-key'"
    echo "  export NEW_RELIC_ACCOUNT_ID='your-account-id'"
    echo ""
    echo -e "${BLUE}Sample validation queries you can test manually:${NC}"
    echo ""
    echo "# Test 1: Basic MySQL metrics availability"
    echo "SELECT count(*) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 1 hour ago"
    echo ""
    echo "# Test 2: MySQL uptime and connections"
    echo "SELECT latest(mysql.global.status.uptime) as 'Uptime (sec)', latest(mysql.global.status.threads_connected) as 'Connections' FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 5 minutes ago"
    echo ""
    echo "# Test 3: Query performance metrics"
    echo "SELECT average(mysql.query.latency_avg_ms) as 'Avg Latency' FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND metricName = 'mysql.query.latency_avg_ms' SINCE 1 hour ago"
    echo ""
    echo "# Test 4: Entity synthesis check"
    echo "SELECT uniqueCount(entity.guid) as 'MySQL Instances' FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 1 hour ago"
    echo ""
    echo "# Test 5: SQL intelligence metrics"
    echo "SELECT sum(mysql.query.exec_total) as 'Total Executions' FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND metricName = 'mysql.query.exec_total' SINCE 1 hour ago"
    echo ""
    exit 0
fi

echo -e "${GREEN}✅ Environment variables found${NC}"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo ""

# Test queries array
declare -a test_queries=(
    "SELECT count(*) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 1 hour ago"
    "SELECT latest(mysql.global.status.uptime) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 5 minutes ago"
    "SELECT average(mysql.query.latency_avg_ms) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND metricName = 'mysql.query.latency_avg_ms' SINCE 1 hour ago"
    "SELECT uniqueCount(entity.guid) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 1 hour ago"
    "SELECT sum(mysql.query.exec_total) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND metricName = 'mysql.query.exec_total' SINCE 1 hour ago"
    "SELECT rate(sum(mysql.global.status.queries), 1 minute) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND entity.type = 'MYSQL_INSTANCE' SINCE 10 minutes ago"
)

declare -a test_names=(
    "Basic MySQL Entity Check"
    "MySQL Uptime Metric"
    "Query Latency Metrics"
    "Entity Count Validation"
    "SQL Intelligence Metrics"
    "Query Rate Calculation"
)

total_tests=${#test_queries[@]}
passed_tests=0

echo -e "${BLUE}Running validation tests...${NC}"
echo ""

for i in "${!test_queries[@]}"; do
    test_name="${test_names[$i]}"
    query="${test_queries[$i]}"
    
    echo -e "${BLUE}Test $((i+1))/$total_tests: $test_name${NC}"
    
    # Build GraphQL query
    graphql_query='
    query($accountId: Int!, $nrql: Nrql!) {
        actor {
            account(id: $accountId) {
                nrql(query: $nrql) {
                    results
                    metadata {
                        timeWindow {
                            begin
                            end
                        }
                    }
                }
            }
        }
    }'
    
    variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"nrql\": \"$query\"}"
    
    # Execute query
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "{\"query\": \"$graphql_query\", \"variables\": $variables}")
    
    # Check for errors
    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        echo -e "${RED}  ❌ Query failed with errors${NC}"
        echo "$response" | jq '.errors'
        echo ""
        continue
    fi
    
    # Check for results
    if echo "$response" | jq -e '.data.actor.account.nrql.results[0]' > /dev/null 2>&1; then
        result=$(echo "$response" | jq -r '.data.actor.account.nrql.results[0] | to_entries | .[0].value // "null"')
        if [[ "$result" != "null" && "$result" != "0" ]]; then
            echo -e "${GREEN}  ✅ Passed (Result: $result)${NC}"
            ((passed_tests++))
        else
            echo -e "${YELLOW}  ⚠️  No data found${NC}"
        fi
    else
        echo -e "${RED}  ❌ No results returned${NC}"
    fi
    
    echo ""
done

echo "====================================="
echo -e "${BLUE}Validation Summary:${NC}"
echo -e "Passed: ${GREEN}$passed_tests${NC}/$total_tests tests"

if [[ $passed_tests -eq $total_tests ]]; then
    echo -e "${GREEN}✅ All dashboard queries validated successfully!${NC}"
    exit 0
elif [[ $passed_tests -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Some queries validated. Check data collection for missing metrics.${NC}"
    exit 1
else
    echo -e "${RED}❌ No queries validated. Check OpenTelemetry collector configuration.${NC}"
    exit 2
fi