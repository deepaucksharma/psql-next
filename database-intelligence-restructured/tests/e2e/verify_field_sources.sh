#!/bin/bash

source ../../.env

echo "🔍 Field-Level Metric Source Verification"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
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
    
    echo "$response" | jq -r '.data.actor.account.nrql.results' 2>/dev/null || echo "Query failed"
}

echo -e "\n📊 OOTB POSTGRESQL RECEIVER METRICS"
echo "========================================"
echo "Source: OpenTelemetry Community postgresql receiver"
echo "Library: otelcol/postgresqlreceiver v0.91.0"

# Sample OOTB metric with all fields
run_query "SELECT 
  metricName,
  postgresql.backends as value,
  postgresql.database.name,
  db.system,
  otel.library.name,
  otel.library.version,
  instrumentation.provider,
  unit,
  description
FROM Metric 
WHERE metricName = 'postgresql.backends' 
SINCE 5 minutes ago 
LIMIT 1" "OOTB Metric Example: postgresql.backends"

echo -e "\n\n📊 CUSTOM SQLQUERY RECEIVER METRICS"
echo "========================================"
echo "Source: Custom SQL queries via sqlquery receiver"

# Sample custom metric with all fields
run_query "SELECT 
  metricName,
  postgres.slow_queries.elapsed_time as value,
  query_id as original_query_id,
  db.postgresql.query_id as transformed_query_id,
  query_text as original_query_text,
  db.statement as transformed_statement,
  statement_type as original_type,
  db.operation as transformed_operation,
  schema_name as original_schema,
  db.schema as transformed_schema,
  db.name,
  db.system,
  otel.library.name,
  unit
FROM Metric 
WHERE metricName = 'postgres.slow_queries.elapsed_time' 
SINCE 5 minutes ago 
LIMIT 1" "Custom Metric Example: postgres.slow_queries.elapsed_time"

echo -e "\n\n📊 FIELD MAPPING VERIFICATION"
echo "========================================"

echo -e "\n1. OOTB Fields (postgresql.* metrics):"
run_query "SELECT 
  uniques(metricName) as metric,
  uniques(postgresql.database.name) as databases,
  uniques(otel.library.name) as library
FROM Metric 
WHERE metricName LIKE 'postgresql.%' 
SINCE 10 minutes ago" "OOTB Metric Fields"

echo -e "\n2. Custom Fields (postgres.* metrics):"
run_query "SELECT 
  uniques(metricName) as metric,
  uniques(query_id) as original_ids,
  uniques(db.postgresql.query_id) as transformed_ids,
  uniques(statement_type) as original_types,
  uniques(db.operation) as transformed_types
FROM Metric 
WHERE metricName LIKE 'postgres.slow_queries%' 
SINCE 10 minutes ago" "Custom Metric Fields"

echo -e "\n3. Wait Event Fields:"
run_query "SELECT 
  uniques(wait_event_name) as original_events,
  uniques(db.wait_event.name) as transformed_events,
  uniques(wait_category) as original_categories,
  uniques(db.wait_event.category) as transformed_categories
FROM Metric 
WHERE metricName = 'postgres.wait_events' 
SINCE 10 minutes ago" "Wait Event Fields"

echo -e "\n\n📊 TRANSFORMATION VERIFICATION"
echo "========================================"

# Check if transformations are working
run_query "SELECT 
  count(*) as metrics_with_original_attrs
FROM Metric 
WHERE metricName LIKE 'postgres.%' 
  AND query_id IS NOT NULL 
SINCE 10 minutes ago" "Metrics with Original Attributes"

run_query "SELECT 
  count(*) as metrics_with_transformed_attrs
FROM Metric 
WHERE metricName LIKE 'postgres.%' 
  AND db.postgresql.query_id IS NOT NULL 
SINCE 10 minutes ago" "Metrics with Transformed Attributes"

echo -e "\n\n📊 SUMMARY TABLE"
echo "========================================"
cat << 'EOF'

| Metric Pattern | Source | Key Fields | Transformation |
|----------------|--------|------------|----------------|
| postgresql.* | OOTB Receiver | postgresql.database.name, postgresql.table.name, postgresql.index.name | None needed |
| postgres.slow_queries.* | Custom SQLQuery | query_id → db.postgresql.query_id, query_text → db.statement, statement_type → db.operation | Via transform processor |
| postgres.wait_events | Custom SQLQuery | wait_event_name → db.wait_event.name, wait_category → db.wait_event.category | Via transform processor |
| postgres.blocking_sessions | Custom SQLQuery | blocked_pid → db.blocking.blocked_pid, blocking_pid → db.blocking.blocking_pid | Via transform processor |
| postgres.execution_plan.* | Custom SQLQuery | node_type → db.plan.node_type, level_id → db.plan.level | Via transform processor |

EOF

echo "================================================================================"