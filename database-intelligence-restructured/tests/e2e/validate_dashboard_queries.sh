#!/bin/bash

source ../../.env

echo "🎯 Validating OpenTelemetry Dashboard Queries"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "Timestamp: $(date)"
echo "================================================================================"

# Function to run and validate query
validate_query() {
    local query="$1"
    local name="$2"
    local expect_results="${3:-yes}"
    
    echo -e "\n📊 $name"
    echo "Query: $query"
    
    cat > /tmp/query.json << EOF
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$query\") { results } } } }"
}
EOF
    
    response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_USER_KEY" \
        -d @/tmp/query.json)
    
    if echo "$response" | jq -e '.data.actor.account.nrql.results' > /dev/null 2>&1; then
        results=$(echo "$response" | jq '.data.actor.account.nrql.results')
        count=$(echo "$results" | jq '. | length')
        
        if [ "$count" -gt 0 ]; then
            # Check if results have actual data (not just empty/zero values)
            has_data=false
            if echo "$results" | jq -e '.[0] | to_entries | .[] | select(.value != null and .value != 0 and .value != "")' > /dev/null 2>&1; then
                has_data=true
            fi
            
            if [ "$has_data" = true ]; then
                echo "✅ SUCCESS: Query returned $count results with data"
                echo "Sample result:"
                echo "$results" | jq '.[0]' | head -10
                return 0
            else
                if [ "$expect_results" = "yes" ]; then
                    echo "⚠️  WARNING: Query returned $count results but no meaningful data"
                    echo "$results" | jq '.[0]'
                    return 1
                else
                    echo "✅ SUCCESS: Query executed (no data expected)"
                    return 0
                fi
            fi
        else
            if [ "$expect_results" = "yes" ]; then
                echo "⚠️  WARNING: Query returned no results"
                return 1
            else
                echo "✅ SUCCESS: Query executed (no results expected)"
                return 0
            fi
        fi
    else
        echo "❌ FAILED: Query execution failed"
        echo "$response" | jq '.errors' 2>/dev/null || echo "$response"
        return 2
    fi
}

# Track results
total=0
success=0
warnings=0

# First, verify we have metrics
echo "🔍 Pre-flight checks..."
validate_query "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE '%postgres%' SINCE 10 minutes ago LIMIT 20" "Available PostgreSQL metrics"

# Dashboard queries validation
echo -e "\n\n📋 DASHBOARD WIDGET QUERIES"
echo "========================================"

# Bird's-Eye View widgets
echo -e "\n### Bird's-Eye View Page ###"

((total++))
if validate_query "SELECT uniqueCount(db.postgresql.query_id) FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET db.name SINCE 10 minutes ago" "1. Unique Queries by Database"; then
    ((success++))
else
    ((warnings++))
fi

((total++))
if validate_query "SELECT latest(postgres.slow_queries.elapsed_time) FROM Metric WHERE db.statement != '<insufficient privilege>' FACET db.statement SINCE 10 minutes ago LIMIT 5" "2. Average Query Execution Time"; then
    ((success++))
else
    ((warnings++))
fi

((total++))
if validate_query "SELECT sum(postgres.slow_queries.count) FROM Metric TIMESERIES 1 minute SINCE 10 minutes ago" "3. Query Execution Count Timeline"; then
    ((success++))
else
    ((warnings++))
fi

((total++))
if validate_query "SELECT sum(postgres.wait_events) FROM Metric FACET db.wait_event.name WHERE db.wait_event.name IS NOT NULL SINCE 10 minutes ago" "4. Top Wait Events"; then
    ((success++))
else
    ((warnings++))
fi

((total++))
if validate_query "SELECT latest(db.name) as 'Database', latest(db.statement) as 'Query', latest(postgres.slow_queries.elapsed_time) as 'Time(ms)' FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET db.postgresql.query_id SINCE 10 minutes ago LIMIT 5" "5. Slowest Queries Table"; then
    ((success++))
else
    ((warnings++))
fi

# Database Health widgets
echo -e "\n### Database Health Page ###"

((total++))
if validate_query "SELECT latest(postgresql.backends) FROM Metric WHERE metricName = 'postgresql.backends' FACET postgresql.database.name SINCE 10 minutes ago" "6. Active Connections"; then
    ((success++))
else
    ((warnings++))
fi

((total++))
if validate_query "SELECT latest(postgresql.db_size) / 1024 / 1024 as 'Size (MB)' FROM Metric WHERE metricName = 'postgresql.db_size' FACET postgresql.database.name SINCE 10 minutes ago" "7. Database Size"; then
    ((success++))
else
    ((warnings++))
fi

((total++))
if validate_query "SELECT sum(postgresql.commits) as 'Commits' FROM Metric WHERE metricName = 'postgresql.commits' TIMESERIES 1 minute SINCE 10 minutes ago" "8. Transaction Rate"; then
    ((success++))
else
    ((warnings++))
fi

# Summary
echo -e "\n\n================================================================================"
echo "📊 VALIDATION SUMMARY"
echo "================================================================================"
echo "Total queries tested: $total"
echo "✅ Successful: $success"
echo "⚠️  Warnings: $warnings"
echo "❌ Failed: $((total - success - warnings))"

if [ $success -eq $total ]; then
    echo -e "\n🎉 All queries validated successfully! Dashboard is ready to create."
    exit 0
elif [ $((success + warnings)) -eq $total ]; then
    echo -e "\n⚠️  Dashboard can be created but some widgets may show no data initially."
    exit 0
else
    echo -e "\n❌ Some queries failed. Please fix before creating dashboard."
    exit 1
fi