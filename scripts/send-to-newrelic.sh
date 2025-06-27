#!/bin/bash
# Script to send metrics to New Relic using the Metrics API

# Load environment variables
source .env

LICENSE_KEY="${NEW_RELIC_LICENSE_KEY}"
ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"
TIMESTAMP=$(date +%s)

# Create metric payload
PAYLOAD=$(cat <<EOF
[{
  "metrics": [{
    "name": "postgres.slow_queries.count",
    "type": "count",
    "value": 1,
    "timestamp": $TIMESTAMP,
    "attributes": {
      "database": "testdb",
      "query_id": "-2189511555843958798",
      "schema": "public",
      "statement_type": "SELECT",
      "entity.name": "postgres-collector-test",
      "entity.type": "PostgreSQL"
    }
  }, {
    "name": "postgres.slow_queries.duration",
    "type": "gauge",
    "value": 2971.05,
    "timestamp": $TIMESTAMP,
    "attributes": {
      "database": "testdb",
      "query_id": "-2189511555843958798",
      "schema": "public",
      "statement_type": "SELECT",
      "entity.name": "postgres-collector-test",
      "entity.type": "PostgreSQL"
    }
  }]
}]
EOF
)

echo "Sending metrics to New Relic..."
RESPONSE=$(curl -s -X POST "https://metric-api.newrelic.com/metric/v1" \
  -H "Content-Type: application/json" \
  -H "Api-Key: $LICENSE_KEY" \
  -d "$PAYLOAD")

echo "Response: $RESPONSE"

# Also send as custom event
EVENT_PAYLOAD=$(cat <<EOF
[{
  "eventType": "PostgresSlowQueriesTest",
  "database": "testdb",
  "queryId": "-2189511555843958798",
  "schema": "public",
  "statementType": "SELECT",
  "avgElapsedTimeMs": 2971.05,
  "executionCount": 1,
  "queryText": "SELECT * FROM generate_series(1,10) WHERE pg_sleep(3) IS NOT NULL",
  "timestamp": $TIMESTAMP
}]
EOF
)

echo -e "\nSending custom event to New Relic..."
EVENT_RESPONSE=$(curl -s -X POST "https://insights-collector.newrelic.com/v1/accounts/$ACCOUNT_ID/events" \
  -H "Content-Type: application/json" \
  -H "X-Insert-Key: $LICENSE_KEY" \
  -d "$EVENT_PAYLOAD")

echo "Event Response: $EVENT_RESPONSE"