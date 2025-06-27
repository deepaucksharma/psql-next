#!/bin/bash
# Query New Relic NRDB for dual-mode metrics

# Load environment variables
source .env

API_KEY="${NEW_RELIC_API_KEY}"
ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"

echo "=== Querying PostgreSQL Metrics in NRDB ==="
echo ""

# 1. Query for PostgresSlowQueries (NRI format)
echo "1. PostgresSlowQueries events (NRI format):"
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM PostgresSlowQueries SINCE 10 minutes ago\\\") { results } } } }\"
  }" | jq -r '.data.actor.account.nrql.results'

echo -e "\n2. Sample PostgresSlowQueries data:"
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT entityName, databaseName, queryId, avgElapsedTimeMs, executionCount FROM PostgresSlowQueries SINCE 10 minutes ago LIMIT 3\\\") { results } } } }\"
  }" | jq -r '.data.actor.account.nrql.results'

# 2. Query for InfrastructureEvent
echo -e "\n3. InfrastructureEvent (simulated NRI):"
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM InfrastructureEvent WHERE category = 'PostgreSQL' SINCE 10 minutes ago\\\") { results } } } }\"
  }" | jq -r '.data.actor.account.nrql.results'

# 3. Query for OTLP metrics (if any made it through)
echo -e "\n4. OTLP Metrics (postgres.* namespace):"
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM Metric WHERE metricName LIKE 'postgres.%' SINCE 10 minutes ago\\\") { results } } } }\"
  }" | jq -r '.data.actor.account.nrql.results'

# 4. All events from postgres collector
echo -e "\n5. All events with postgres in entity name:"
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT uniques(eventType) FROM Log, InfrastructureEvent, PostgresSlowQueries WHERE entityName LIKE '%postgres%' OR entity.name LIKE '%postgres%' SINCE 10 minutes ago\\\") { results } } } }\"
  }" | jq -r '.data.actor.account.nrql.results'

echo -e "\n=== Query Complete ==="