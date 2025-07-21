#!/bin/bash

# Test Dashboard Queries via NerdGraph
# This script tests individual widget queries to verify they return data

ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID:-3630072}"
API_KEY="${NEW_RELIC_API_KEY}"

if [ -z "$API_KEY" ]; then
    echo "Error: NEW_RELIC_API_KEY not set"
    exit 1
fi

echo "Testing Dashboard Queries via NerdGraph"
echo "======================================"
echo ""

# Function to test a single NRQL query
test_query() {
    local query_name="$1"
    local nrql_query="$2"
    
    echo "Testing: $query_name"
    echo "Query: $nrql_query"
    
    # Escape the query for JSON
    escaped_query=$(echo "$nrql_query" | sed 's/"/\\"/g')
    
    # Create the GraphQL query
    graphql_query='{
      "query": "{ actor { account(id: '$ACCOUNT_ID') { nrql(query: \"'$escaped_query'\") { results } } } }"
    }'
    
    # Execute the query
    response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Content-Type: application/json" \
        -H "API-Key: $API_KEY" \
        -d "$graphql_query")
    
    # Check if we got results
    if echo "$response" | grep -q '"results":\[\]'; then
        echo "❌ NO DATA RETURNED"
    elif echo "$response" | grep -q '"results"'; then
        echo "✅ Data returned successfully"
        # Show first result
        echo "$response" | jq -r '.data.actor.account.nrql.results[0]' 2>/dev/null || echo "Unable to parse results"
    else
        echo "❌ ERROR in query"
        echo "$response" | jq '.' 2>/dev/null || echo "$response"
    fi
    echo "---"
    echo ""
}

# Test queries from MySQL Intelligence Command Center dashboard

echo "=== MYSQL INTELLIGENCE COMMAND CENTER ==="
echo ""

# Original queries that likely don't work
test_query "System Health Score (Original)" \
    "SELECT average(mysql_mysql_query_cost_score) as 'Avg Query Cost' FROM Metric WHERE entity.type = 'MYSQL_QUERY_INTELLIGENCE' SINCE 30 minutes ago"

# Fixed queries based on actual metrics
test_query "System Health Score (Fixed - Check Service Names)" \
    "SELECT count(*) FROM Metric WHERE service.name IN ('sql-intelligence', 'core-metrics', 'wait-profiler') SINCE 30 minutes ago FACET service.name"

# Check what metrics are actually available
test_query "Available Metrics from sql-intelligence" \
    "SELECT uniques(metricName) FROM Metric WHERE service.name = 'sql-intelligence' SINCE 1 hour ago LIMIT 100"

test_query "Available Metrics from core-metrics" \
    "SELECT uniques(metricName) FROM Metric WHERE service.name = 'core-metrics' SINCE 1 hour ago LIMIT 100"

test_query "Available Metrics from wait-profiler" \
    "SELECT uniques(metricName) FROM Metric WHERE service.name = 'wait-profiler' SINCE 1 hour ago LIMIT 100"

# Test specific metric patterns
test_query "MySQL Buffer Pool Metrics" \
    "SELECT average(mysql_buffer_pool_pages) FROM Metric WHERE service.name = 'core-metrics' SINCE 30 minutes ago"

test_query "Wait Profiler Metrics" \
    "SELECT average(wait_profiler_wait_count) FROM Metric WHERE service.name = 'wait-profiler' SINCE 30 minutes ago"

test_query "Query Cost Score Variants" \
    "SELECT average(value) FROM Metric WHERE metricName LIKE '%query%cost%' SINCE 30 minutes ago FACET metricName"

echo "=== PERFORMANCE INTELLIGENCE EXECUTIVE ==="
echo ""

# Test executive dashboard queries
test_query "Performance Health Score" \
    "SELECT count(*) FROM Metric WHERE metricName LIKE '%cost%score%' SINCE 1 hour ago FACET metricName"

test_query "Business Impact Metrics" \
    "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE '%business%impact%' SINCE 1 hour ago"

echo "=== REAL-TIME OPERATIONS CENTER ==="
echo ""

# Test real-time monitoring queries
test_query "Active Connections Check" \
    "SELECT latest(mysql_threads) FROM Metric WHERE service.name = 'core-metrics' SINCE 5 minutes ago"

test_query "QPS Calculation" \
    "SELECT rate(sum(mysql_handlers), 1 second) FROM Metric WHERE service.name = 'core-metrics' SINCE 5 minutes ago"

echo "=== CHECKING ATTRIBUTE AVAILABILITY ==="
echo ""

# Check what attributes are available
test_query "Available Attributes" \
    "SELECT uniques(dimensions()) FROM Metric WHERE service.name = 'sql-intelligence' SINCE 1 hour ago LIMIT 50"

test_query "Check DIGEST attribute" \
    "SELECT count(*) FROM Metric WHERE service.name = 'sql-intelligence' AND DIGEST IS NOT NULL SINCE 1 hour ago"

test_query "Check recommendation_priority attribute" \
    "SELECT count(*) FROM Metric WHERE service.name = 'sql-intelligence' AND recommendation_priority IS NOT NULL SINCE 1 hour ago"

echo "=== TESTING ALTERNATIVE APPROACHES ==="
echo ""

# Test different ways to access the data
test_query "Using job label" \
    "SELECT count(*) FROM Metric WHERE job IN ('sql-intelligence', 'core-metrics', 'wait-profiler') SINCE 1 hour ago FACET job"

test_query "Using module label" \
    "SELECT count(*) FROM Metric WHERE module IN ('sql-intelligence', 'core-metrics', 'wait-profiler') SINCE 1 hour ago FACET module"

test_query "All metrics with module label" \
    "SELECT uniques(metricName) FROM Metric WHERE module IS NOT NULL SINCE 1 hour ago FACET module LIMIT 200"

echo ""
echo "Testing complete. Review the results above to determine which queries work."