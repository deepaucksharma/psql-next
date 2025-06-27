#!/bin/bash
# Query New Relic for our metrics

# Load environment variables
source .env

API_KEY="${NEW_RELIC_API_KEY}"
ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"

echo "Querying for custom metrics..."
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM Metric WHERE metricName LIKE 'postgres.%' SINCE 5 minutes ago\\\") { results } } } }\"
  }" | jq .

echo -e "\nQuerying for custom events..."
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $API_KEY" \
  -d "{
    \"query\": \"{ actor { account(id: $ACCOUNT_ID) { nrql(query: \\\"SELECT * FROM PostgresSlowQueriesTest SINCE 5 minutes ago LIMIT 5\\\") { results } } } }\"
  }" | jq .