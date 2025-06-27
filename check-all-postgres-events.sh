#!/bin/bash

# Load environment variables
source .env

# GraphQL endpoint
GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"

# Function to run NRQL query
run_query() {
    local nrql="$1"
    local description="$2"
    
    echo -e "\n\033[1;34m$description\033[0m"
    echo -e "\033[0;33mNRQL: $nrql\033[0m"
    
    local graphql_query=$(cat <<EOF
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$nrql\") { results } } } }"
}
EOF
)
    
    curl -s -X POST "$GRAPHQL_ENDPOINT" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "$graphql_query" | jq -r '.data.actor.account.nrql.results'
}

echo -e "\033[1;32m=== Checking All PostgreSQL Events in NRDB ===\033[0m"

# Check for any PostgresSlowQueries events
run_query "FROM PostgresSlowQueries SELECT count(*) SINCE 1 hour ago" \
    "Checking for ANY PostgresSlowQueries events"

# Check without WHERE clause
run_query "FROM PostgresSlowQueries SELECT * LIMIT 5 SINCE 1 hour ago" \
    "Recent PostgresSlowQueries (no filter)"

# Check for com.newrelic.postgresql integration
run_query "FROM IntegrationSample SELECT * WHERE displayName LIKE '%postgres%' LIMIT 5 SINCE 1 hour ago" \
    "Checking IntegrationSample for PostgreSQL"

# Check Infrastructure events
run_query "FROM InfrastructureEvent SELECT * WHERE category = 'integration' AND type LIKE '%postgres%' LIMIT 5 SINCE 1 hour ago" \
    "Checking InfrastructureEvent for PostgreSQL integrations"

# Check for any custom events with postgres in name
run_query "SELECT eventType() WHERE eventType() LIKE '%postgres%' SINCE 1 hour ago LIMIT 10" \
    "Checking for any event types with 'postgres' in name"

# Check SystemSample for collector process
run_query "FROM SystemSample SELECT * WHERE processDisplayName LIKE '%postgres-unified-collector%' LIMIT 1 SINCE 1 hour ago" \
    "Checking for collector process"

# Check for any events created in last 5 minutes
run_query "SELECT count(*) FROM Transaction, SystemSample, ProcessSample, NetworkSample, StorageSample, ContainerSample, InfrastructureEvent WHERE timestamp > \$(date +%s)000 - 300000" \
    "Recent events across all Infrastructure types"

echo -e "\n\033[1;32m=== Alternative: Check License Key Validity ===\033[0m"
echo "If no events are found, verify:"
echo "1. License key is valid and has correct permissions"
echo "2. Account ID matches the license key"
echo "3. Collector is running and outputting to stdout"
echo "4. Infrastructure Agent is properly configured to run the integration"