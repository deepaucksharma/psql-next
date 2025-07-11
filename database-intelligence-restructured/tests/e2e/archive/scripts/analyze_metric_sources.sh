#!/bin/bash

source ../../.env

echo "🔍 Analyzing PostgreSQL Metrics Implementation Sources"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "Timestamp: $(date)"
echo "================================================================================"

# Function to run NRQL query
run_query() {
    local query="$1"
    local name="$2"
    
    cat > /tmp/query.json << EOF
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$query\") { results } } } }"
}
EOF
    
    echo -e "\n### $name ###"
    
    response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_USER_KEY" \
        -d @/tmp/query.json)
    
    if echo "$response" | jq -e '.data.actor.account.nrql.results' > /dev/null 2>&1; then
        echo "$response" | jq -r '.data.actor.account.nrql.results'
    else
        echo "Query failed"
    fi
}

# 1. Get all PostgreSQL metrics
echo -e "\n📊 SECTION 1: ALL POSTGRESQL METRICS"
echo "========================================"
run_query "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE '%postgres%' OR attributes.db.system = 'postgresql' SINCE 30 minutes ago" "All PostgreSQL Metrics"

# 2. Standard postgresql receiver metrics (OOTB)
echo -e "\n\n📊 SECTION 2: OOTB POSTGRESQL RECEIVER METRICS"
echo "========================================"
run_query "SELECT metricName, count(*) FROM Metric WHERE metricName LIKE 'postgresql.%' SINCE 30 minutes ago FACET metricName LIMIT 50" "OOTB PostgreSQL Receiver Metrics Count"

# 3. Custom postgres metrics (from sqlquery receiver)
echo -e "\n\n📊 SECTION 3: CUSTOM SQLQUERY RECEIVER METRICS"
echo "========================================"
run_query "SELECT metricName, count(*) FROM Metric WHERE metricName LIKE 'postgres.%' SINCE 30 minutes ago FACET metricName LIMIT 50" "Custom SQLQuery Metrics Count"

# 4. Sample OOTB metric with all attributes
echo -e "\n\n📊 SECTION 4: OOTB METRIC ATTRIBUTES SAMPLE"
echo "========================================"
echo "Metric: postgresql.backends"
run_query "SELECT * FROM Metric WHERE metricName = 'postgresql.backends' SINCE 30 minutes ago LIMIT 1" "postgresql.backends - Full Attributes"

echo -e "\nMetric: postgresql.db_size"
run_query "SELECT * FROM Metric WHERE metricName = 'postgresql.db_size' SINCE 30 minutes ago LIMIT 1" "postgresql.db_size - Full Attributes"

echo -e "\nMetric: postgresql.commits"
run_query "SELECT * FROM Metric WHERE metricName = 'postgresql.commits' SINCE 30 minutes ago LIMIT 1" "postgresql.commits - Full Attributes"

# 5. Sample custom metrics with all attributes
echo -e "\n\n📊 SECTION 5: CUSTOM METRIC ATTRIBUTES SAMPLE"
echo "========================================"
echo "Metric: postgres.slow_queries.elapsed_time"
run_query "SELECT * FROM Metric WHERE metricName = 'postgres.slow_queries.elapsed_time' SINCE 30 minutes ago LIMIT 1" "postgres.slow_queries.elapsed_time - Full Attributes"

echo -e "\nMetric: postgres.wait_events"
run_query "SELECT * FROM Metric WHERE metricName = 'postgres.wait_events' SINCE 30 minutes ago LIMIT 1" "postgres.wait_events - Full Attributes"

echo -e "\nMetric: postgres.execution_plan.cost"
run_query "SELECT * FROM Metric WHERE metricName = 'postgres.execution_plan.cost' SINCE 30 minutes ago LIMIT 1" "postgres.execution_plan.cost - Full Attributes"

# 6. List unique attribute keys per metric type
echo -e "\n\n📊 SECTION 6: UNIQUE ATTRIBUTES BY METRIC TYPE"
echo "========================================"
run_query "SELECT uniques(keyset()) FROM Metric WHERE metricName LIKE 'postgresql.%' SINCE 30 minutes ago" "OOTB Metric Attribute Keys"

run_query "SELECT uniques(keyset()) FROM Metric WHERE metricName LIKE 'postgres.%' SINCE 30 minutes ago" "Custom Metric Attribute Keys"

# 7. Check for custom processors
echo -e "\n\n📊 SECTION 7: TRANSFORMED ATTRIBUTES"
echo "========================================"
run_query "SELECT count(*) FROM Metric WHERE attributes.db.system = 'postgresql' AND attributes.db.statement IS NOT NULL SINCE 30 minutes ago" "Metrics with db.statement (transformed)"

run_query "SELECT count(*) FROM Metric WHERE attributes.db.system = 'postgresql' AND attributes.db.postgresql.query_id IS NOT NULL SINCE 30 minutes ago" "Metrics with db.postgresql.query_id (transformed)"

# 8. Resource attributes
echo -e "\n\n📊 SECTION 8: RESOURCE ATTRIBUTES"
echo "========================================"
run_query "SELECT uniques(attributes.environment), uniques(attributes.service.name) FROM Metric WHERE metricName LIKE '%postgres%' SINCE 30 minutes ago" "Resource Attributes"

echo -e "\n\n================================================================================"
echo "📋 ANALYSIS COMPLETE"
echo "================================================================================"