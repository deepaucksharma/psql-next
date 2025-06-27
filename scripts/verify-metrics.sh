#!/bin/bash
# Script to verify metrics are being received in New Relic

# Load environment variables
source .env

ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"
API_KEY="${NEW_RELIC_API_KEY}"
REGION="${NEW_RELIC_REGION:-US}"

# Query for PostgresSlowQueries
echo "Querying for PostgresSlowQueries events..."
curl -s -X POST "https://insights-api.newrelic.com/v1/accounts/$ACCOUNT_ID/query" \
  -H "Accept: application/json" \
  -H "X-Query-Key: $API_KEY" \
  -d "{ \"nrql\": \"SELECT count(*) FROM PostgresSlowQueries SINCE 5 minutes ago\" }" | jq .

echo -e "\nQuerying for recent PostgresSlowQueries..."
curl -s -X POST "https://insights-api.newrelic.com/v1/accounts/$ACCOUNT_ID/query" \
  -H "Accept: application/json" \
  -H "X-Query-Key: $API_KEY" \
  -d "{ \"nrql\": \"SELECT * FROM PostgresSlowQueries SINCE 5 minutes ago LIMIT 5\" }" | jq .