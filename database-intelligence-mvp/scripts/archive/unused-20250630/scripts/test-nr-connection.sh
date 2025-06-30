#!/bin/bash
# Test New Relic API connectivity

source "$(dirname "$0")/../.env"

echo "Testing New Relic API connectivity..."
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "License Key: ${NEW_RELIC_LICENSE_KEY:0:10}..."

# Test with a simple NRQL query
echo -e "\nTesting NRQL API..."
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: ${NEW_RELIC_USER_KEY:-$NEW_RELIC_LICENSE_KEY}" \
  -d "{
    \"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { name } } }\"
  }")

http_status=$(echo "$response" | grep "HTTP_STATUS:" | cut -d: -f2)
body=$(echo "$response" | sed '/HTTP_STATUS:/d')

echo "HTTP Status: $http_status"
echo "Response: $body" | jq '.' 2>/dev/null || echo "$body"

if [[ "$http_status" == "200" ]]; then
    echo -e "\n✓ API connection successful"
else
    echo -e "\n✗ API connection failed"
fi