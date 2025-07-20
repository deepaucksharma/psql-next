#!/bin/bash

# Quick NerdGraph API Test
# Demonstrates direct NerdGraph usage for dashboard validation

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NERDGRAPH_URL="https://api.newrelic.com/graphql"

echo -e "${BLUE}🚀 Quick NerdGraph Dashboard Validation Test${NC}"
echo "============================================="

# Check environment
if [[ -z "${NEW_RELIC_API_KEY:-}" ]] || [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo -e "${YELLOW}⚠️  Missing environment variables. Set:${NC}"
    echo "  export NEW_RELIC_API_KEY='your-user-api-key'"
    echo "  export NEW_RELIC_ACCOUNT_ID='your-account-id'"
    echo ""
    echo -e "${BLUE}Here's how you would test directly with curl:${NC}"
    echo ""
    echo "# 1. Test API connectivity"
    echo "curl -X POST $NERDGRAPH_URL \\"
    echo "  -H 'Content-Type: application/json' \\"
    echo "  -H 'API-Key: \$NEW_RELIC_API_KEY' \\"
    echo "  -d '{\"query\": \"query { actor { user { name email } } }\"}'"
    echo ""
    echo "# 2. Check for MySQL metrics"
    echo "curl -X POST $NERDGRAPH_URL \\"
    echo "  -H 'Content-Type: application/json' \\"
    echo "  -H 'API-Key: \$NEW_RELIC_API_KEY' \\"
    echo "  -d '{\"query\": \"query(\\\$accountId: Int!, \\\$nrql: Nrql!) { actor { account(id: \\\$accountId) { nrql(query: \\\$nrql) { results } } } }\", \"variables\": {\"accountId\": '\$NEW_RELIC_ACCOUNT_ID', \"nrql\": \"SELECT count(*) FROM Metric WHERE metricName LIKE '\''mysql.%'\'' SINCE 1 hour ago\"}}'"
    echo ""
    echo "# 3. List all dashboards"
    echo "curl -X POST $NERDGRAPH_URL \\"
    echo "  -H 'Content-Type: application/json' \\"
    echo "  -H 'API-Key: \$NEW_RELIC_API_KEY' \\"
    echo "  -d '{\"query\": \"query(\\\$accountId: Int!) { actor { account(id: \\\$accountId) { dashboards { name guid } } } }\", \"variables\": {\"accountId\": '\$NEW_RELIC_ACCOUNT_ID'}}'"
    exit 0
fi

echo -e "${GREEN}✅ Environment variables found${NC}"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo ""

# Test 1: API Connectivity
echo -e "${BLUE}🔗 Test 1: API Connectivity${NC}"
api_response=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d '{"query": "query { actor { user { name email } } }"}')

if echo "$api_response" | jq -e '.data.actor.user.name' > /dev/null 2>&1; then
    user_name=$(echo "$api_response" | jq -r '.data.actor.user.name')
    echo -e "${GREEN}✅ API connection successful (User: $user_name)${NC}"
else
    echo -e "${RED}❌ API connection failed${NC}"
    echo "$api_response" | jq '.'
    exit 1
fi

# Test 2: MySQL Metrics Check
echo -e "${BLUE}📊 Test 2: MySQL Metrics Availability${NC}"
mysql_query='
query($accountId: Int!, $nrql: Nrql!) {
    actor {
        account(id: $accountId) {
            nrql(query: $nrql) {
                results
                metadata {
                    timeWindow {
                        begin
                        end
                    }
                }
            }
        }
    }
}'

mysql_variables="{
    \"accountId\": $NEW_RELIC_ACCOUNT_ID,
    \"nrql\": \"SELECT count(*) FROM Metric WHERE metricName LIKE 'mysql.%' SINCE 1 hour ago\"
}"

mysql_response=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "{\"query\": \"$mysql_query\", \"variables\": $mysql_variables}")

if echo "$mysql_response" | jq -e '.data.actor.account.nrql.results[0]' > /dev/null 2>&1; then
    mysql_count=$(echo "$mysql_response" | jq -r '.data.actor.account.nrql.results[0].count')
    if [[ "$mysql_count" -gt 0 ]]; then
        echo -e "${GREEN}✅ MySQL metrics found: $mysql_count data points${NC}"
    else
        echo -e "${YELLOW}⚠️  No MySQL metrics found in the last hour${NC}"
    fi
else
    echo -e "${RED}❌ Failed to query MySQL metrics${NC}"
fi

# Test 3: Dashboard List
echo -e "${BLUE}📋 Test 3: Dashboard Inventory${NC}"
dashboard_query='
query($accountId: Int!) {
    actor {
        account(id: $accountId) {
            dashboards {
                name
                guid
                createdAt
                pages {
                    name
                }
            }
        }
    }
}'

dashboard_variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID}"

dashboard_response=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "{\"query\": \"$dashboard_query\", \"variables\": $dashboard_variables}")

if echo "$dashboard_response" | jq -e '.data.actor.account.dashboards' > /dev/null 2>&1; then
    dashboard_count=$(echo "$dashboard_response" | jq '.data.actor.account.dashboards | length')
    echo -e "${GREEN}✅ Found $dashboard_count dashboards in account${NC}"
    
    # Check for specific dashboards
    mysql_dashboard_guid=$(echo "$dashboard_response" | jq -r '.data.actor.account.dashboards[] | select(.name | contains("MySQL")) | .guid' | head -1)
    
    if [[ "$mysql_dashboard_guid" != "" && "$mysql_dashboard_guid" != "null" ]]; then
        echo -e "${GREEN}  ✅ MySQL dashboard found (GUID: $mysql_dashboard_guid)${NC}"
    else
        echo -e "${YELLOW}  ⚠️  No MySQL-related dashboards found${NC}"
        echo -e "${BLUE}     Available dashboards:${NC}"
        echo "$dashboard_response" | jq -r '.data.actor.account.dashboards[].name' | sed 's/^/       - /'
    fi
else
    echo -e "${RED}❌ Failed to list dashboards${NC}"
fi

# Test 4: Entity Check
echo -e "${BLUE}🏗️  Test 4: Entity Synthesis Check${NC}"
entity_query='
query {
    actor {
        entitySearch(query: "type = '\''MYSQL_INSTANCE'\''") {
            results {
                entities {
                    guid
                    name
                    entityType
                    reporting
                }
            }
        }
    }
}'

entity_response=$(curl -s -X POST "$NERDGRAPH_URL" \
    -H "Content-Type: application/json" \
    -H "API-Key: $NEW_RELIC_API_KEY" \
    -d "{\"query\": \"$entity_query\"}")

if echo "$entity_response" | jq -e '.data.actor.entitySearch.results.entities' > /dev/null 2>&1; then
    entity_count=$(echo "$entity_response" | jq '.data.actor.entitySearch.results.entities | length')
    reporting_count=$(echo "$entity_response" | jq '[.data.actor.entitySearch.results.entities[] | select(.reporting == true)] | length')
    
    if [[ $entity_count -gt 0 ]]; then
        echo -e "${GREEN}✅ MySQL entities found: $entity_count total ($reporting_count reporting)${NC}"
        echo "$entity_response" | jq -r '.data.actor.entitySearch.results.entities[] | "  - \(.name) (\(.guid))"'
    else
        echo -e "${YELLOW}⚠️  No MySQL entities found - check entity synthesis configuration${NC}"
    fi
else
    echo -e "${RED}❌ Failed to search entities${NC}"
fi

echo ""
echo -e "${GREEN}✅ NerdGraph validation test completed!${NC}"
echo ""
echo -e "${BLUE}💡 Next steps:${NC}"
echo "   1. If no MySQL metrics found, verify OpenTelemetry collectors are running"
echo "   2. If no dashboards found, run: ./setup-newrelic.sh deploy"
echo "   3. If no entities found, check entity synthesis attributes in collectors"
echo "   4. For full validation, run: ./setup-newrelic.sh validate"