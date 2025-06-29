#!/bin/bash
# Comprehensive metric verification for Database Intelligence MVP

source "$(dirname "$0")/../.env"

echo "======================================"
echo "Database Intelligence MVP Verification"
echo "Account: $NEW_RELIC_ACCOUNT_ID"
echo "Time: $(date)"
echo "======================================"

# Function to run NRQL query
run_query() {
    local query="$1"
    local desc="$2"
    
    echo -e "\n📊 $desc"
    curl -s -X POST "https://api.newrelic.com/graphql" \
      -H "Content-Type: application/json" \
      -H "API-Key: ${NEW_RELIC_USER_KEY:-$NEW_RELIC_LICENSE_KEY}" \
      -d "{
        \"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"
      }" | jq '.data.actor.account.nrql.results'
}

# 1. Overall metrics count
run_query "SELECT count(*) as 'Total Metrics' FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 1 hour ago" \
          "Total OpenTelemetry Metrics"

# 2. PostgreSQL specific metrics
run_query "SELECT uniqueCount(metricName) as 'Unique Metrics' FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago" \
          "Unique PostgreSQL Metric Types"

# 3. Database tables being monitored
run_query "SELECT uniques(postgresql.table.name) FROM Metric WHERE postgresql.table.name IS NOT NULL SINCE 1 hour ago LIMIT 20" \
          "PostgreSQL Tables Being Monitored"

# 4. Metric breakdown by type
run_query "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql%' FACET metricName SINCE 1 hour ago LIMIT 10" \
          "Top 10 PostgreSQL Metrics"

# 5. Data freshness
run_query "SELECT latest(timestamp) as 'Last Data Point' FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 5 minutes ago" \
          "Data Freshness Check"

# 6. Collector verification
run_query "SELECT count(*) FROM Metric WHERE collector.name = 'database-intelligence' SINCE 1 hour ago" \
          "Database Intelligence Collector Metrics"

echo -e "\n✅ Verification Complete!"
echo "======================================"