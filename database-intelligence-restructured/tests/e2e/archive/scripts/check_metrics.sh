#!/bin/bash

source ../../.env

echo "🔍 Checking for PostgreSQL metrics in New Relic"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "================================================================================"

run_query() {
    local query="$1"
    local name="$2"
    
    cat > /tmp/query.json << EOF
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$query\") { results } } } }"
}
EOF
    
    echo -e "\n📊 $name"
    echo "Query: $query"
    
    response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_USER_KEY" \
        -d @/tmp/query.json)
    
    if echo "$response" | jq -e '.data.actor.account.nrql.results' > /dev/null 2>&1; then
        results=$(echo "$response" | jq '.data.actor.account.nrql.results')
        count=$(echo "$results" | jq '. | length')
        
        if [ "$count" -gt 0 ]; then
            echo "✅ Found $count results"
            echo "$results" | jq . | head -20
        else
            echo "⚠️  No results found"
        fi
    else
        echo "❌ Query failed"
        echo "$response" | jq '.errors' 2>/dev/null || echo "$response"
    fi
}

# Check for any metrics in the last hour
run_query "SELECT count(*) FROM Metric SINCE 1 hour ago" "Total metrics in last hour"

# Check for PostgreSQL metrics
run_query "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE '%postgres%' OR metricName LIKE '%postgresql%' SINCE 1 hour ago" "PostgreSQL metric names"

# Check for metrics with db.system = postgresql
run_query "SELECT uniques(metricName) FROM Metric WHERE attributes.db.system = 'postgresql' SINCE 1 hour ago LIMIT 50" "Metrics with db.system=postgresql"

# Check wait events specifically
run_query "SELECT count(*) FROM Metric WHERE metricName = 'postgres.wait_events' SINCE 1 hour ago" "Wait events count"

# Check slow queries
run_query "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' SINCE 1 hour ago" "Slow queries count"

# Check standard postgresql receiver metrics
run_query "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql.%' SINCE 1 hour ago" "Standard PostgreSQL metrics"

# Show sample of any metric with postgres in name
run_query "SELECT metricName, attributes.db.system, attributes.db.name FROM Metric WHERE metricName LIKE '%postgres%' SINCE 1 hour ago LIMIT 10" "Sample PostgreSQL metrics with attributes"

# Check if collector is still running
echo -e "\n🔧 Checking collector status..."
if pgrep -f "otelcol-contrib.*collector-complete-ohi-parity.yaml" > /dev/null; then
    echo "✅ Collector is running"
else
    echo "❌ Collector is not running"
fi