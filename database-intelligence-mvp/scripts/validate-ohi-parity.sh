#!/bin/bash
# Validate OHI parity for PostgreSQL metrics

source "$(dirname "$0")/../.env"

echo "======================================"
echo "OHI Parity Validation for PostgreSQL"
echo "Account: $NEW_RELIC_ACCOUNT_ID"
echo "Time: $(date)"
echo "======================================"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Function to run NRQL query
run_query() {
    local query="$1"
    local desc="$2"
    
    echo -e "\n📊 $desc"
    result=$(curl -s -X POST "https://api.newrelic.com/graphql" \
      -H "Content-Type: application/json" \
      -H "API-Key: ${NEW_RELIC_USER_KEY:-$NEW_RELIC_LICENSE_KEY}" \
      -d "{
        \"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"
      }" | jq '.data.actor.account.nrql.results')
    
    if [ "$result" != "null" ] && [ "$result" != "[]" ]; then
        echo -e "${GREEN}✓ Found${NC}"
        echo "$result" | jq '.'
        return 0
    else
        echo -e "${RED}✗ Not found${NC}"
        return 1
    fi
}

# Counters
total_checks=0
passed_checks=0

# Function to check metric
check_metric() {
    local event_type="$1"
    local description="$2"
    local query="$3"
    
    ((total_checks++))
    if run_query "$query" "$description"; then
        ((passed_checks++))
    fi
}

echo -e "\n${YELLOW}=== Checking OHI Event Types ===${NC}"

# 1. PostgreSQLSample equivalent (basic metrics)
check_metric "PostgreSQLSample" "PostgreSQL Basic Metrics" \
    "SELECT count(*) as 'Metric Count', uniques(metricName) as 'Unique Metrics' FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago"

# 2. PostgresSlowQueries
check_metric "PostgresSlowQueries" "Slow Query Events" \
    "SELECT count(*) as 'Event Count' FROM Log WHERE eventType = 'PostgresSlowQueries' SINCE 1 hour ago"

# 3. PostgresWaitEvents
check_metric "PostgresWaitEvents" "Wait Event Data" \
    "SELECT count(*) as 'Event Count' FROM Log WHERE eventType = 'PostgresWaitEvents' SINCE 1 hour ago"

# 4. PostgresBlockingSessions
check_metric "PostgresBlockingSessions" "Blocking Session Events" \
    "SELECT count(*) as 'Event Count' FROM Log WHERE eventType = 'PostgresBlockingSessions' SINCE 1 hour ago"

# 5. PostgresExecutionPlans
check_metric "PostgresExecutionPlans" "Execution Plan Events" \
    "SELECT count(*) as 'Event Count' FROM Log WHERE eventType = 'PostgresExecutionPlans' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Checking Key OHI Metrics ===${NC}"

# Check specific metrics that OHI collects
check_metric "Blocks Read" "postgresql.blocks_read metric" \
    "SELECT sum(postgresql.blocks_read) FROM Metric WHERE metricName = 'postgresql.blocks_read' SINCE 1 hour ago"

check_metric "Connection Count" "postgresql.connection.count metric" \
    "SELECT latest(postgresql.connection.count) FROM Metric WHERE metricName = 'postgresql.connection.count' SINCE 1 hour ago"

check_metric "Database Size" "postgresql.database.size metric" \
    "SELECT latest(postgresql.database.size) FROM Metric WHERE metricName = 'postgresql.database.size' SINCE 1 hour ago"

check_metric "Table Size" "postgresql.table.size metric" \
    "SELECT sum(postgresql.table.size) FROM Metric WHERE metricName = 'postgresql.table.size' FACET postgresql.table.name SINCE 1 hour ago LIMIT 5"

check_metric "BGWriter Stats" "postgresql.bgwriter metrics" \
    "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql.bgwriter%' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Checking Slow Query Attributes ===${NC}"

# Check if slow query events have all required attributes
check_metric "Slow Query Attributes" "Required slow query attributes" \
    "SELECT uniques(query_id), uniques(database_name), uniques(statement_type), average(avg_elapsed_time_ms) FROM Log WHERE eventType = 'PostgresSlowQueries' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Entity Synthesis Check ===${NC}"

# Check entity creation
check_metric "PostgreSQL Entities" "Database entity synthesis" \
    "SELECT uniques(entity.name), uniques(entity.type), uniques(entity.guid) FROM Log WHERE entity.type = 'POSTGRESQL_DATABASE' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Summary ===${NC}"
echo "Total checks: $total_checks"
echo "Passed: $passed_checks"
echo "Failed: $((total_checks - passed_checks))"

if [ $passed_checks -eq $total_checks ]; then
    echo -e "\n${GREEN}✅ Full OHI parity achieved!${NC}"
    exit 0
else
    echo -e "\n${YELLOW}⚠️  Some OHI metrics missing. This may be due to:${NC}"
    echo "1. Extensions not installed (pg_stat_statements, pg_wait_sampling)"
    echo "2. Insufficient database activity"
    echo "3. Collector configuration issues"
    echo "4. Permissions missing for monitoring user"
    exit 1
fi