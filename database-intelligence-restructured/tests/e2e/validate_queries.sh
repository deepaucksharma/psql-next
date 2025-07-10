#!/bin/bash

# Load environment variables
source ../../.env

if [ -z "$NEW_RELIC_USER_KEY" ] || [ -z "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo "Error: NEW_RELIC_USER_KEY and NEW_RELIC_ACCOUNT_ID must be set"
    exit 1
fi

# Use the USER_KEY as API_KEY for the script
NEW_RELIC_API_KEY="$NEW_RELIC_USER_KEY"

echo "🔍 Validating OpenTelemetry PostgreSQL Queries"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "================================================================================"

# Function to execute NRQL query via NerdGraph
execute_nrql() {
    local query="$1"
    local name="$2"
    
    # Escape quotes in the query
    escaped_query=$(echo "$query" | sed 's/"/\\"/g')
    
    # Create GraphQL query
    graphql_query="{
        \"query\": \"{
            actor {
                account(id: $NEW_RELIC_ACCOUNT_ID) {
                    nrql(query: \\\"$escaped_query\\\") {
                        results
                    }
                }
            }
        }\"
    }"
    
    # Execute the query
    response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "$graphql_query")
    
    # Check for errors
    if echo "$response" | grep -q '"errors"'; then
        echo "❌ FAILED: $name"
        echo "Error: $(echo "$response" | jq -r '.errors[0].message' 2>/dev/null || echo "$response")"
        return 1
    fi
    
    # Extract results
    results=$(echo "$response" | jq -r '.data.actor.account.nrql.results' 2>/dev/null)
    
    if [ "$results" = "null" ] || [ -z "$results" ]; then
        echo "❌ FAILED: $name - No results in response"
        return 1
    fi
    
    # Count results
    result_count=$(echo "$results" | jq '. | length' 2>/dev/null || echo "0")
    
    if [ "$result_count" = "0" ]; then
        echo "⚠️  WARNING: $name - Query returned no results"
    else
        echo "✅ SUCCESS: $name - Query returned $result_count results"
        # Show first result
        echo "  Sample: $(echo "$results" | jq '.[0]' -c 2>/dev/null | head -c 100)..."
    fi
    
    return 0
}

# Test queries
success=0
total=0

# Query 1: Unique queries by database
((total++))
if execute_nrql "SELECT uniqueCount(attributes.db.postgresql.query_id) FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET attributes.db.name SINCE 1 hour ago" "Unique Queries by Database"; then
    ((success++))
fi
echo

# Query 2: Wait events
((total++))
if execute_nrql "SELECT sum(postgres.wait_events) FROM Metric WHERE metricName = 'postgres.wait_events' FACET attributes.db.wait_event.name SINCE 1 hour ago LIMIT 20" "Top Wait Events"; then
    ((success++))
fi
echo

# Query 3: PostgreSQL backends
((total++))
if execute_nrql "SELECT latest(postgresql.backends) FROM Metric WHERE metricName = 'postgresql.backends' FACET attributes.postgresql.database.name SINCE 1 hour ago" "Active Connections"; then
    ((success++))
fi
echo

# Query 4: Database size
((total++))
if execute_nrql "SELECT latest(postgresql.db_size) / 1024 / 1024 as 'Size (MB)' FROM Metric WHERE metricName = 'postgresql.db_size' FACET attributes.postgresql.database.name SINCE 1 hour ago" "Database Size"; then
    ((success++))
fi
echo

# Query 5: All PostgreSQL metrics
((total++))
if execute_nrql "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgres%' OR metricName LIKE 'postgresql%' SINCE 1 hour ago" "All PostgreSQL Metrics"; then
    ((success++))
fi
echo

# Query 6: Slow queries with attributes
((total++))
if execute_nrql "SELECT latest(postgres.slow_queries.elapsed_time), latest(attributes.db.name), latest(attributes.db.statement) FROM Metric WHERE metricName = 'postgres.slow_queries.elapsed_time' FACET attributes.db.postgresql.query_id SINCE 1 hour ago LIMIT 5" "Slow Queries Detail"; then
    ((success++))
fi
echo

# Query 7: Transaction rates
((total++))
if execute_nrql "SELECT sum(postgresql.commits) as 'Commits', sum(postgresql.rollbacks) as 'Rollbacks' FROM Metric WHERE metricName IN ('postgresql.commits', 'postgresql.rollbacks') TIMESERIES AUTO SINCE 1 hour ago" "Transaction Rates"; then
    ((success++))
fi
echo

# Query 8: Disk I/O
((total++))
if execute_nrql "SELECT average(postgres.slow_queries.disk_reads) FROM Metric WHERE metricName = 'postgres.slow_queries.disk_reads' FACET attributes.db.name SINCE 1 hour ago" "Disk Reads"; then
    ((success++))
fi

echo "================================================================================"
echo "📊 Summary:"
echo "✅ Successful queries: $success"
echo "❌ Failed queries: $((total - success))"
echo "Total queries tested: $total"

if [ $success -eq $total ]; then
    echo "🎉 All queries validated successfully!"
    exit 0
else
    echo "⚠️  Some queries failed validation"
    exit 1
fi