#!/bin/bash

# Quick NRQL query tool
QUERY="$1"
NR_API_KEY="${NEW_RELIC_API_KEY}"
NR_ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID:-3630072}"

if [ -z "$QUERY" ]; then
    echo "Usage: $0 'NRQL_QUERY'"
    exit 1
fi

curl -s -X POST "https://api.newrelic.com/graphql" \
    -H 'Content-Type: application/json' \
    -H "API-Key: $NR_API_KEY" \
    -d "{\"query\": \"{ actor { account(id: $NR_ACCOUNT_ID) { nrql(query: \\\"$QUERY\\\") { results } } } }\"}" | \
    python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(json.dumps(results, indent=2))
except Exception as e:
    print(f'Error: {e}')
    print('Response:', sys.stdin.read())
"