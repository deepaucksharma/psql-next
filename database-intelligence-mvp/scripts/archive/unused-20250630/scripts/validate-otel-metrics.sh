#!/bin/bash
# Validate OTEL metrics that match OHI capabilities

source "$(dirname "$0")/../.env"

echo "======================================"
echo "OTEL Metrics Validation (OHI Capabilities)"
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
    local capability="$1"
    local description="$2"
    local query="$3"
    
    ((total_checks++))
    if run_query "$query" "$description"; then
        ((passed_checks++))
    fi
}

echo -e "\n${YELLOW}=== Core PostgreSQL Metrics (PostgreSQLSample equivalent) ===${NC}"

check_metric "Basic Metrics" "PostgreSQL infrastructure metrics" \
    "SELECT uniqueCount(metricName) as 'Metric Types', count(*) as 'Data Points' FROM Metric WHERE metricName LIKE 'postgresql.%' AND db.system = 'postgresql' SINCE 1 hour ago"

check_metric "Table Metrics" "Table-level metrics" \
    "SELECT uniques(postgresql.table.name) as 'Tables', sum(postgresql.table.size) as 'Total Size' FROM Metric WHERE metricName = 'postgresql.table.size' SINCE 1 hour ago"

check_metric "Database Size" "Database size tracking" \
    "SELECT latest(postgresql.database.size) as 'Size' FROM Metric WHERE metricName = 'postgresql.database.size' FACET database_name SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Query Performance Metrics (PostgresSlowQueries equivalent) ===${NC}"

check_metric "Query Count" "Query execution counts" \
    "SELECT sum(db.query.count) as 'Total Executions' FROM Metric WHERE metricName = 'db.query.count' AND db.system = 'postgresql' FACET statement_type SINCE 1 hour ago LIMIT 5"

check_metric "Query Duration" "Query duration metrics" \
    "SELECT average(db.query.mean_duration) as 'Avg Duration', max(db.query.mean_duration) as 'Max Duration' FROM Metric WHERE metricName = 'db.query.mean_duration' AND db.system = 'postgresql' SINCE 1 hour ago"

check_metric "Query IO" "Query I/O metrics" \
    "SELECT sum(db.io.disk_reads) as 'Disk Reads', sum(db.io.cache_hits) as 'Cache Hits' FROM Metric WHERE metricName IN ('db.io.disk_reads', 'db.io.cache_hits') AND db.system = 'postgresql' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Connection Metrics (PostgresBlockingSessions equivalent) ===${NC}"

check_metric "Active Connections" "Active connection count" \
    "SELECT latest(db.connections.active) as 'Active', latest(db.connections.idle) as 'Idle' FROM Metric WHERE metricName LIKE 'db.connections.%' AND db.system = 'postgresql' SINCE 1 hour ago"

check_metric "Blocked Connections" "Blocked connection tracking" \
    "SELECT latest(db.connections.blocked) as 'Blocked', latest(db.connections.waiting) as 'Waiting' FROM Metric WHERE metricName IN ('db.connections.blocked', 'db.connections.waiting') AND db.system = 'postgresql' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Wait Event Metrics (PostgresWaitEvents equivalent) ===${NC}"

check_metric "Wait Events" "Wait event distribution" \
    "SELECT sum(db.wait_events) as 'Count' FROM Metric WHERE metricName = 'db.wait_events' AND db.system = 'postgresql' FACET wait_event_type SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Replication Metrics (Additional) ===${NC}"

check_metric "Replication Lag" "Replication lag tracking" \
    "SELECT latest(db.replication.lag) as 'Lag Bytes', latest(db.replication.lag_time) as 'Lag Time' FROM Metric WHERE metricName LIKE 'db.replication.%' AND db.system = 'postgresql' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Dimensional Analysis ===${NC}"

check_metric "Cardinality Check" "Unique dimension values" \
    "SELECT uniqueCount(query_id) as 'Unique Queries', uniqueCount(statement_type) as 'Statement Types', uniqueCount(database_name) as 'Databases' FROM Metric WHERE metricName LIKE 'db.query.%' AND db.system = 'postgresql' SINCE 1 hour ago"

echo -e "\n${YELLOW}=== Summary ===${NC}"
echo "Total capability checks: $total_checks"
echo "Passed: $passed_checks"
echo "Failed: $((total_checks - passed_checks))"

if [ $passed_checks -eq $total_checks ]; then
    echo -e "\n${GREEN}✅ All OHI capabilities covered with OTEL metrics!${NC}"
    exit 0
else
    percentage=$((passed_checks * 100 / total_checks))
    echo -e "\n${YELLOW}📊 Coverage: ${percentage}%${NC}"
    echo -e "\nMissing capabilities may be due to:"
    echo "1. Insufficient database activity"
    echo "2. Metrics still being collected (wait 5 minutes)"
    echo "3. SQL query receiver needs time to execute"
    exit 1
fi