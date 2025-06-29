#!/bin/bash
# Check for metrics in New Relic

source "$(dirname "$0")/../.env"

echo "Checking for PostgreSQL metrics in New Relic account $NEW_RELIC_ACCOUNT_ID..."

# Check for metrics
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: ${NEW_RELIC_USER_KEY:-$NEW_RELIC_LICENSE_KEY}" \
  -d "{
    \"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) as 'Metric Count' FROM Metric WHERE collector.name = 'database-intelligence' OR metricName LIKE 'postgresql%' SINCE 30 minutes ago\\\") { results } } } }\"
  }" | jq '.data.actor.account.nrql.results'

echo -e "\nChecking for any OpenTelemetry data..."
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: ${NEW_RELIC_USER_KEY:-$NEW_RELIC_LICENSE_KEY}" \
  -d "{
    \"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 30 minutes ago\\\") { results } } } }\"
  }" | jq '.data.actor.account.nrql.results'

echo -e "\nChecking for test log..."
curl -s -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: ${NEW_RELIC_USER_KEY:-$NEW_RELIC_LICENSE_KEY}" \
  -d "{
    \"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"SELECT count(*) FROM Log WHERE test = 'true' SINCE 1 hour ago\\\") { results } } } }\"
  }" | jq '.data.actor.account.nrql.results'