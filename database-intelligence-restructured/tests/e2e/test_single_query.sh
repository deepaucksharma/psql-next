#!/bin/bash

source ../../.env

echo "Testing single NRQL query..."
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"

# Simple test query
QUERY="SELECT count(*) FROM Metric WHERE metricName LIKE 'postgres%' SINCE 5 minutes ago"

# Create the GraphQL query with proper escaping
cat > /tmp/nerdgraph_query.json << EOF
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$QUERY\") { results } } } }"
}
EOF

echo "Query JSON:"
cat /tmp/nerdgraph_query.json

echo -e "\nExecuting query..."
response=$(curl -s -X POST https://api.newrelic.com/graphql \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_USER_KEY" \
    -d @/tmp/nerdgraph_query.json)

echo -e "\nRaw response:"
echo "$response" | jq . || echo "$response"

# Check if we got results
if echo "$response" | jq -e '.data.actor.account.nrql.results' > /dev/null 2>&1; then
    echo -e "\n✅ Query successful!"
    echo "Results:"
    echo "$response" | jq '.data.actor.account.nrql.results'
else
    echo -e "\n❌ Query failed"
    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        echo "Errors:"
        echo "$response" | jq '.errors'
    fi
fi