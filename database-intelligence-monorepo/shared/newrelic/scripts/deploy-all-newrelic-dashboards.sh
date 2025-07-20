#!/bin/bash

# Deploy All New Relic Dashboards Script
# Deploys the complete suite of MySQL monitoring dashboards to New Relic

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_DIR="$SCRIPT_DIR/../dashboards/shared/newrelic/dashboards"
NERDGRAPH_URL="https://api.newrelic.com/graphql"

echo -e "${BLUE}🚀 Deploying Complete MySQL Monitoring Suite to New Relic${NC}"
echo "================================================================"

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

# Complete dashboard suite for MySQL monitoring
declare -a dashboard_files=(
    "validated-mysql-core-dashboard.json"
    "validated-sql-intelligence-dashboard.json"
    "validated-operational-dashboard.json"
    "core-metrics-newrelic-dashboard.json"
    "sql-intelligence-newrelic-dashboard.json"
    "replication-monitor-newrelic-dashboard.json"
    "performance-advisor-newrelic-dashboard.json"
    "mysql-intelligence-dashboard.json"
    "database-intelligence-executive-dashboard.json"
    "plan-explorer-dashboard.json"
    "simple-test-dashboard.json"
)

declare -a dashboard_descriptions=(
    "MySQL Core Intelligence - Validated patterns"
    "SQL Intelligence Analytics - Validated patterns"
    "MySQL Operations - Validated patterns"
    "Core Metrics Module - New Relic native"
    "SQL Intelligence Module - New Relic native"
    "Replication Monitor - New Relic native"
    "Performance Advisor - New Relic native"
    "Complete MySQL Intelligence - Updated patterns"
    "Executive Dashboard - Updated patterns"
    "Plan Explorer Dashboard - Query analysis"
    "Simple Test Dashboard - Basic connectivity"
)

total_dashboards=${#dashboard_files[@]}
deployed_count=0
failed_count=0

echo -e "${BLUE}Deploying $total_dashboards MySQL monitoring dashboards...${NC}"
echo ""

# Function to deploy a single dashboard
deploy_dashboard() {
    local file="$1"
    local description="$2"
    local index="$3"
    local filepath="$DASHBOARD_DIR/$file"
    
    echo -e "${BLUE}[$index/$total_dashboards] Deploying: $description${NC}"
    
    # Check if file exists
    if [[ ! -f "$filepath" ]]; then
        echo -e "${RED}  ❌ Dashboard file not found: $filepath${NC}"
        ((failed_count++))
        return 1
    fi
    
    # Read and validate JSON
    if ! dashboard_json=$(jq '.' "$filepath" 2>/dev/null); then
        echo -e "${RED}  ❌ Invalid JSON in dashboard file${NC}"
        ((failed_count++))
        return 1
    fi
    
    # Substitute environment variables
    dashboard_json=$(echo "$dashboard_json" | sed "s/\${NEW_RELIC_ACCOUNT_ID}/$NEW_RELIC_ACCOUNT_ID/g")
    
    # Build inline GraphQL mutation (avoids variable parsing issues)
    query="mutation { dashboardCreate(accountId: $NEW_RELIC_ACCOUNT_ID, dashboard: $dashboard_json) { entityResult { guid name } errors { description type } } }"
    
    # Execute mutation
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "{\"query\": \"$query\"}")
    
    # Check for errors and success
    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        echo -e "${RED}  ❌ GraphQL errors${NC}"
        echo "$response" | jq '.errors' | sed 's/^/     /'
        ((failed_count++))
        return 1
    elif echo "$response" | jq -e '.data.dashboardCreate.errors[0]' > /dev/null 2>&1; then
        echo -e "${RED}  ❌ Dashboard creation errors${NC}"
        echo "$response" | jq '.data.dashboardCreate.errors' | sed 's/^/     /'
        ((failed_count++))
        return 1
    elif echo "$response" | jq -e '.data.dashboardCreate.entityResult.guid' > /dev/null 2>&1; then
        guid=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.guid')
        dashboard_name=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.name')
        echo -e "${GREEN}  ✅ Deployed successfully${NC}"
        echo -e "${GREEN}     Name: $dashboard_name${NC}"
        echo -e "${GREEN}     GUID: $guid${NC}"
        echo -e "${GREEN}     URL: https://one.newrelic.com/redirect/entity/$guid${NC}"
        ((deployed_count++))
        return 0
    else
        echo -e "${RED}  ❌ Unknown deployment error${NC}"
        echo "$response" | jq '.' | sed 's/^/     /'
        ((failed_count++))
        return 1
    fi
}

# Deploy all dashboards
for i in "${!dashboard_files[@]}"; do
    file="${dashboard_files[$i]}"
    description="${dashboard_descriptions[$i]}"
    deploy_dashboard "$file" "$description" "$((i+1))"
    echo ""
done

echo "================================================================"
echo -e "${BLUE}MySQL Monitoring Suite Deployment Summary:${NC}"
echo -e "✅ Successfully deployed: ${GREEN}$deployed_count${NC} dashboards"
echo -e "❌ Failed deployments: ${RED}$failed_count${NC} dashboards"
echo -e "📊 Total dashboards: $total_dashboards"

if [[ $deployed_count -eq $total_dashboards ]]; then
    echo -e "${GREEN}🎉 Complete MySQL monitoring suite deployed successfully!${NC}"
    echo ""
    echo -e "${BLUE}🎯 Your New Relic MySQL Monitoring Suite Includes:${NC}"
    echo "   📈 Executive Overview - Business-level MySQL health"
    echo "   🔍 Core Metrics - Foundation MySQL monitoring"
    echo "   💡 SQL Intelligence - Query performance analysis"
    echo "   ⚡ Operations Dashboard - Real-time operational health"
    echo "   🔄 Replication Monitor - Master-replica health"
    echo "   🎯 Performance Advisor - Optimization recommendations"
    echo "   📊 Maximum Value Dashboard - Comprehensive analytics"
    echo ""
    echo -e "${BLUE}🚀 Next Steps:${NC}"
    echo "   1. Visit New Relic One: https://one.newrelic.com"
    echo "   2. Navigate to Dashboards to explore your MySQL monitoring"
    echo "   3. Set up alerts based on the dashboard metrics"
    echo "   4. Monitor and optimize your MySQL performance!"
    
    exit 0
elif [[ $deployed_count -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Partial deployment completed. Check errors above for failed dashboards.${NC}"
    echo ""
    echo -e "${BLUE}💡 Tip: Re-run this script to retry failed deployments${NC}"
    exit 1
else
    echo -e "${RED}❌ No dashboards deployed successfully.${NC}"
    echo ""
    echo -e "${BLUE}🔧 Troubleshooting:${NC}"
    echo "   1. Verify API key permissions (NerdGraph access required)"
    echo "   2. Check account ID is correct"
    echo "   3. Ensure JSON files are valid"
    echo "   4. Verify network connectivity to New Relic"
    exit 2
fi