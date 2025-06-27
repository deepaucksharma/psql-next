#!/bin/bash
# Check for New Relic integration errors

source .env

echo "Checking for NrIntegrationError events..."

# Query for integration errors
curl -X POST https://insights-api.newrelic.com/v1/accounts/4799862/query \
  -H "Accept: application/json" \
  -H "X-Query-Key: $NEW_RELIC_LICENSE_KEY" \
  -d "query=SELECT * FROM NrIntegrationError WHERE category = 'otlp' SINCE 1 hour ago LIMIT 10" 2>&1 | jq .

echo -e "\nChecking for any PostgreSQL metrics..."
curl -X POST https://insights-api.newrelic.com/v1/accounts/4799862/query \
  -H "Accept: application/json" \
  -H "X-Query-Key: $NEW_RELIC_LICENSE_KEY" \
  -d "query=SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 30 minutes ago" 2>&1 | jq .