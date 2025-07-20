#!/bin/bash

# Deploy Validated Dashboards Script
# Deploys the verified dashboard configurations to New Relic

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_DIR="$SCRIPT_DIR/../dashboards"
NERDGRAPH_URL="https://api.newrelic.com/graphql"

echo -e "${BLUE}🚀 Deploying Validated MySQL Dashboards${NC}"
echo "==========================================="

# Check environment
if [[ -z "${NEW_RELIC_API_KEY:-}" ]] || [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo -e "${YELLOW}⚠️  Missing environment variables. Set:${NC}"
    echo "  export NEW_RELIC_API_KEY='your-user-api-key'"
    echo "  export NEW_RELIC_ACCOUNT_ID='your-account-id'"
    echo ""
    echo "Then run: $0"
    exit 1
fi

echo -e "${GREEN}✅ Environment variables found${NC}"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo ""

# Dashboard files to deploy
declare -a dashboard_files=(
    "validated-mysql-core-dashboard.json"
    "validated-sql-intelligence-dashboard.json"
    "validated-operational-dashboard.json"
)

declare -a dashboard_names=(
    "MySQL Core Intelligence - Validated"
    "SQL Intelligence - Validated Analytics"
    "MySQL Operations - Validated Monitoring"
)

total_dashboards=${#dashboard_files[@]}
deployed_count=0

echo -e "${BLUE}Deploying $total_dashboards validated dashboards...${NC}"
echo ""

for i in "${!dashboard_files[@]}"; do
    file="${dashboard_files[$i]}"
    name="${dashboard_names[$i]}"
    filepath="$DASHBOARD_DIR/$file"
    
    echo -e "${BLUE}Deploying $((i+1))/$total_dashboards: $name${NC}"
    
    # Check if file exists
    if [[ ! -f "$filepath" ]]; then
        echo -e "${RED}  ❌ Dashboard file not found: $filepath${NC}"
        echo ""
        continue
    fi
    
    # Read and validate JSON
    if ! dashboard_json=$(jq '.' "$filepath" 2>/dev/null); then
        echo -e "${RED}  ❌ Invalid JSON in dashboard file${NC}"
        echo ""
        continue
    fi
    
    # Substitute environment variables in the dashboard JSON
    dashboard_json=$(echo "$dashboard_json" | sed "s/\\\${NEW_RELIC_ACCOUNT_ID}/$NEW_RELIC_ACCOUNT_ID/g")
    
    # Build GraphQL mutation
    mutation='
    mutation($accountId: Int!, $dashboard: DashboardInput!) {
        dashboardCreate(accountId: $accountId, dashboard: $dashboard) {
            entityResult {
                guid
                name
            }
            errors {
                description
                type
            }
        }
    }'
    
    variables=$(jq -n \
        --argjson accountId "$NEW_RELIC_ACCOUNT_ID" \
        --argjson dashboard "$dashboard_json" \
        '{accountId: $accountId, dashboard: $dashboard}')
    
    # Execute mutation
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "{\"query\": \"$mutation\", \"variables\": $variables}")
    
    # Check for GraphQL errors
    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        echo -e "${RED}  ❌ GraphQL errors${NC}"
        echo "$response" | jq '.errors'
        echo ""
        continue
    fi
    
    # Check for dashboard creation errors
    if echo "$response" | jq -e '.data.dashboardCreate.errors[0]' > /dev/null 2>&1; then
        echo -e "${RED}  ❌ Dashboard creation errors${NC}"
        echo "$response" | jq '.data.dashboardCreate.errors'
        echo ""
        continue
    fi
    
    # Check for successful creation
    if echo "$response" | jq -e '.data.dashboardCreate.entityResult.guid' > /dev/null 2>&1; then
        guid=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.guid')
        dashboard_name=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.name')
        echo -e "${GREEN}  ✅ Deployed successfully${NC}"
        echo -e "${GREEN}     Dashboard: $dashboard_name${NC}"
        echo -e "${GREEN}     GUID: $guid${NC}"
        echo -e "${GREEN}     URL: https://one.newrelic.com/redirect/entity/$guid${NC}"
        ((deployed_count++))
    else
        echo -e "${RED}  ❌ Unknown deployment error${NC}"
        echo "$response" | jq '.'
    fi
    
    echo ""
done

echo "==========================================="
echo -e "${BLUE}Deployment Summary:${NC}"
echo -e "Deployed: ${GREEN}$deployed_count${NC}/$total_dashboards dashboards"

if [[ $deployed_count -eq $total_dashboards ]]; then
    echo -e "${GREEN}✅ All dashboards deployed successfully!${NC}"
    echo ""
    echo -e "${BLUE}🎯 Next Steps:${NC}"
    echo "1. Visit New Relic One: https://one.newrelic.com"
    echo "2. Navigate to Dashboards to view your new MySQL monitoring"
    echo "3. Run validation: ../../../scripts/validate-dashboard-queries.sh"
    echo "4. Monitor data flow and adjust collection intervals if needed"
    exit 0
elif [[ $deployed_count -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Some dashboards deployed. Check errors above for failed deployments.${NC}"
    exit 1
else
    echo -e "${RED}❌ No dashboards deployed. Check API permissions and account settings.${NC}"
    exit 2
fi