#!/bin/bash

# Script to check if PostgreSQL metrics are being received in New Relic

set -e

# Source environment variables
source /Users/deepaksharma/syc/psql-next/.env

# Check if required variables are set
if [ -z "$NEW_RELIC_API_KEY" ] || [ -z "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo "ERROR: NEW_RELIC_API_KEY and NEW_RELIC_ACCOUNT_ID must be set"
    exit 1
fi

# Set the GraphQL endpoint based on region
if [ "$NEW_RELIC_REGION" = "EU" ]; then
    GRAPHQL_ENDPOINT="https://api.eu.newrelic.com/graphql"
else
    GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"
fi

echo "Checking New Relic for PostgreSQL metrics..."
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo "Region: ${NEW_RELIC_REGION:-US}"
echo ""

# Query for PostgreSQL metrics
QUERY='{ "query": "query { actor { account(id: '$NEW_RELIC_ACCOUNT_ID') { nrql(query: \"SELECT count(*) FROM PostgresqlInstanceSample SINCE 5 minutes ago\") { results } } } }" }'

echo "Querying for PostgresqlInstanceSample events..."
RESPONSE=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "$QUERY")

# Parse response
if echo "$RESPONSE" | grep -q "count"; then
    COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | cut -d: -f2)
    echo "✓ Found $COUNT PostgresqlInstanceSample events in the last 5 minutes"
else
    echo "✗ No PostgreSQL metrics found"
    echo "Response: $RESPONSE"
fi

# Query for slow queries
echo ""
echo "Querying for slow queries..."
QUERY='{ "query": "query { actor { account(id: '$NEW_RELIC_ACCOUNT_ID') { nrql(query: \"SELECT count(*) FROM PostgresSlowQueries SINCE 5 minutes ago\") { results } } } }" }'

RESPONSE=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "$QUERY")

if echo "$RESPONSE" | grep -q "count"; then
    COUNT=$(echo "$RESPONSE" | grep -o '"count":[0-9]*' | cut -d: -f2)
    echo "✓ Found $COUNT PostgresSlowQueries events in the last 5 minutes"
else
    echo "✗ No slow query metrics found"
fi

# Query for recent infrastructure entities
echo ""
echo "Querying for PostgreSQL entities..."
QUERY='{ "query": "query { actor { entitySearch(query: \"type = '\''POSTGRESQL_INSTANCE'\'' AND reporting = true\") { results { entities { name guid reporting tags { key values } } } } } }" }'

RESPONSE=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "$QUERY")

if echo "$RESPONSE" | grep -q "entities"; then
    echo "✓ Found PostgreSQL entities:"
    echo "$RESPONSE" | grep -o '"name":"[^"]*"' | cut -d'"' -f4 | while read -r entity; do
        echo "  - $entity"
    done
else
    echo "✗ No PostgreSQL entities found"
fi

echo ""
echo "Done checking New Relic data."