#\!/bin/bash
source .env

GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"

run_query() {
    local nrql="$1"
    local desc="$2"
    
    echo -e "\n\033[1;34m$desc\033[0m"
    echo -e "\033[0;33m$nrql\033[0m"
    
    local graphql_query=$(cat <<QUERY
{
  "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$nrql\") { results } } } }"
}
QUERY
)
    
    curl -s -X POST "$GRAPHQL_ENDPOINT" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "$graphql_query" | jq -r '.data.actor.account.nrql.results'
}

echo -e "\033[1;32m=== Verifying Metrics in NRDB ===\033[0m"

run_query "FROM Metric SELECT * WHERE metricName LIKE 'postgres%' SINCE 5 minutes ago" \
    "Checking for PostgreSQL metrics"

run_query "FROM Metric SELECT count(*) WHERE metricName = 'postgres.slow_queries.count' SINCE 10 minutes ago" \
    "Count of slow query metrics"

run_query "FROM Metric SELECT * WHERE source = 'docker-collector-test' SINCE 10 minutes ago" \
    "Metrics from our test source"
