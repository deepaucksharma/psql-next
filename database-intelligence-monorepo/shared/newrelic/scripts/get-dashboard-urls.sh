#!/bin/bash

# Get Dashboard URLs using NerdGraph
# Returns clickable URLs for all dashboards

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

NERDGRAPH_URL="https://api.newrelic.com/graphql"

# Check environment
if [[ -z "${NEW_RELIC_API_KEY:-}" ]] || [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo "❌ Missing environment variables. Set:"
    echo "  export NEW_RELIC_API_KEY='your-user-api-key'"
    echo "  export NEW_RELIC_ACCOUNT_ID='your-account-id'"
    exit 1
fi

echo -e "${BLUE}📊 Getting Dashboard URLs from New Relic...${NC}"
echo ""

# Query to get dashboards with permalinks
query='
query getDashboardURLs($accountId: Int!) {
    actor {
        account(id: $accountId) {
            dashboards {
                name
                guid
                permalink
                createdAt
                updatedAt
            }
        }
    }
}'

variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID}"

response=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "{\"query\": \"$query\", \"variables\": $variables}")

# Check for errors
if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
    echo "❌ NerdGraph Error:"
    echo "$response" | jq '.errors'
    exit 1
fi

# Extract and display dashboard URLs
echo "$response" | jq -r '.data.actor.account.dashboards[] | 
    "✅ \(.name)
   📋 GUID: \(.guid)
   🔗 URL:  \(.permalink)
   📅 Created: \(.createdAt)
   
"'

# Show MySQL-specific dashboards
mysql_dashboards=$(echo "$response" | jq -r '.data.actor.account.dashboards[] | select(.name | test("MySQL|Intelligence|Plan Explorer"; "i")) | .permalink')

if [[ -n "$mysql_dashboards" ]]; then
    echo -e "${GREEN}🎯 MySQL Intelligence Dashboard URLs:${NC}"
    echo "$mysql_dashboards" | while read -r url; do
        echo "   🔗 $url"
    done
fi