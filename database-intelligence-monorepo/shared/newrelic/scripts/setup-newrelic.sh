#!/bin/bash
# Setup New Relic for MySQL Intelligence Monitoring
# This script creates dashboards, alerts, and workloads in New Relic

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QUERIES_DIR="${SCRIPT_DIR}/../nerdgraph"
DASHBOARDS_DIR="${SCRIPT_DIR}/../dashboards"

# Required environment variables
required_vars=("NEW_RELIC_API_KEY" "NEW_RELIC_ACCOUNT_ID" "NEW_RELIC_LICENSE_KEY")

# Function to check environment variables
check_environment() {
    echo "🔍 Checking environment variables..."
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            echo -e "${RED}❌ Error: $var is not set${NC}"
            exit 1
        fi
    done
    echo -e "${GREEN}✅ All required environment variables are set${NC}"
}

# New Relic API endpoints
if [ "${NEW_RELIC_REGION}" == "EU" ]; then
    NERDGRAPH_URL="https://api.eu.newrelic.com/graphql"
else
    NERDGRAPH_URL="https://api.newrelic.com/graphql"
fi

# Function to execute NerdGraph query
execute_nerdgraph() {
    local query=$1
    local variables=$2
    
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "{\"query\": \"$query\", \"variables\": $variables}")
    
    # Check for GraphQL errors
    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        echo -e "${RED}❌ NerdGraph Error:${NC}" >&2
        echo "$response" | jq '.errors' >&2
        return 1
    fi
    
    echo "$response"
}

# Function to validate dashboard exists and is working
validate_dashboards() {
    echo "🔍 Validating dashboards..."
    
    # Get all dashboards
    local query='
    query getDashboards($accountId: Int!) {
        actor {
            account(id: $accountId) {
                dashboards {
                    name
                    guid
                    createdAt
                    updatedAt
                    pages {
                        name
                        guid
                        widgets {
                            title
                            configuration
                        }
                    }
                }
            }
        }
    }'
    
    local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID}"
    local response=$(execute_nerdgraph "$query" "$variables")
    
    if [ $? -eq 0 ]; then
        local dashboard_count=$(echo "$response" | jq '.data.actor.account.dashboards | length')
        echo -e "${GREEN}✅ Found $dashboard_count dashboards in account${NC}"
        
        # Check for specific dashboards
        local target_dashboards=("MySQL Intelligence Dashboard" "Plan Explorer Dashboard" "Database Intelligence Executive Dashboard")
        
        for dashboard_name in "${target_dashboards[@]}"; do
            local dashboard_guid=$(echo "$response" | jq -r \
                ".data.actor.account.dashboards[] | select(.name == \"$dashboard_name\") | .guid")
            
            if [[ "$dashboard_guid" != "null" && -n "$dashboard_guid" ]]; then
                echo -e "${GREEN}  ✅ Found: $dashboard_name (GUID: $dashboard_guid)${NC}"
                validate_dashboard_data "$dashboard_guid" "$dashboard_name"
            else
                echo -e "${YELLOW}  ⚠️  Missing: $dashboard_name${NC}"
            fi
        done
    else
        echo -e "${RED}❌ Failed to fetch dashboards${NC}"
        return 1
    fi
}

# Function to validate data is flowing to specific dashboard
validate_dashboard_data() {
    local dashboard_guid=$1
    local dashboard_name=$2
    
    echo "    📊 Validating data for: $dashboard_name"
    
    # Get dashboard details and test widgets
    local query='
    query getDashboard($guid: EntityGuid!) {
        actor {
            entity(guid: $guid) {
                ... on DashboardEntity {
                    name
                    pages {
                        name
                        widgets {
                            title
                            configuration
                        }
                    }
                }
            }
        }
    }'
    
    local variables="{\"guid\": \"$dashboard_guid\"}"
    local response=$(execute_nerdgraph "$query" "$variables")
    
    if [ $? -eq 0 ]; then
        # Extract and test NRQL queries from widgets
        local nrql_queries=$(echo "$response" | jq -r '
            .data.actor.entity.pages[]?.widgets[]? | 
            select(.configuration.nrqlQueries?) | 
            .configuration.nrqlQueries[] | 
            select(.query?) | 
            .query
        ' 2>/dev/null || echo "")
        
        if [[ -n "$nrql_queries" ]]; then
            local query_count=0
            local successful_queries=0
            
            while IFS= read -r nrql_query; do
                if [[ -n "$nrql_query" && "$nrql_query" != "null" ]]; then
                    query_count=$((query_count + 1))
                    
                    # Test the NRQL query
                    if test_nrql_query "$nrql_query"; then
                        successful_queries=$((successful_queries + 1))
                    fi
                fi
            done <<< "$nrql_queries"
            
            if [[ $successful_queries -eq $query_count && $query_count -gt 0 ]]; then
                echo -e "${GREEN}    ✅ All $query_count widgets have data${NC}"
            elif [[ $successful_queries -gt 0 ]]; then
                echo -e "${YELLOW}    ⚠️  $successful_queries/$query_count widgets have data${NC}"
            else
                echo -e "${RED}    ❌ No widgets have data${NC}"
            fi
        else
            echo -e "${YELLOW}    ⚠️  No NRQL queries found in dashboard${NC}"
        fi
    else
        echo -e "${RED}    ❌ Failed to get dashboard details${NC}"
    fi
}

# Function to test individual NRQL query
test_nrql_query() {
    local nrql_query=$1
    
    local query='
    query testNRQL($accountId: Int!, $nrql: Nrql!) {
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
    
    local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"nrql\": \"$nrql_query\"}"
    local response=$(execute_nerdgraph "$query" "$variables" 2>/dev/null)
    
    if [ $? -eq 0 ]; then
        local result_count=$(echo "$response" | jq '.data.actor.account.nrql.results | length' 2>/dev/null || echo "0")
        
        if [[ "$result_count" -gt 0 ]]; then
            return 0  # Success
        fi
    fi
    
    return 1  # Failure
}

# Function to validate specific metrics are flowing
validate_metrics_flow() {
    echo "📈 Validating metrics data flow..."
    
    local critical_metrics=(
        "mysql.intelligence.comprehensive"
        "mysql.connections.current"
        "mysql.threads.running"
        "mysql.query.exec_count"
        "system.cpu.utilization"
        "system.memory.usage"
    )
    
    for metric in "${critical_metrics[@]}"; do
        echo "    Checking: $metric"
        
        local nrql="SELECT latest($metric) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 10 minutes ago"
        
        if test_nrql_query "$nrql"; then
            echo -e "${GREEN}    ✅ $metric: Data found${NC}"
        else
            echo -e "${RED}    ❌ $metric: No data${NC}"
        fi
    done
}

# Function to validate entity synthesis
validate_entities() {
    echo "🏗️  Validating entity synthesis..."
    
    local entity_types=("MYSQL_INSTANCE" "HOST" "SYNTHETIC_MONITOR" "APPLICATION")
    
    for entity_type in "${entity_types[@]}"; do
        echo "    Checking: $entity_type entities"
        
        local query='
        query validateEntities($accountId: Int!, $entityType: String!) {
            actor {
                entitySearch(query: $entityType) {
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
        
        local search_query="type = '$entity_type'"
        local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"entityType\": \"$search_query\"}"
        local response=$(execute_nerdgraph "$query" "$variables")
        
        if [ $? -eq 0 ]; then
            local entity_count=$(echo "$response" | jq '.data.actor.entitySearch.results.entities | length')
            local reporting_count=$(echo "$response" | jq '[.data.actor.entitySearch.results.entities[] | select(.reporting == true)] | length')
            
            if [[ $entity_count -gt 0 ]]; then
                echo -e "${GREEN}    ✅ $entity_type: $entity_count entities ($reporting_count reporting)${NC}"
            else
                echo -e "${YELLOW}    ⚠️  $entity_type: No entities found${NC}"
            fi
        else
            echo -e "${RED}    ❌ $entity_type: Failed to check entities${NC}"
        fi
    done
}

# Function to create dashboard
create_dashboard() {
    echo "📊 Creating MySQL Intelligence Dashboard..."
    
    # Read dashboard JSON
    dashboard_json=$(cat "${DASHBOARDS_DIR}/mysql-intelligence-dashboard.json")
    
    # Replace environment variables in dashboard JSON
    dashboard_json=$(echo "$dashboard_json" | sed "s/\${NEW_RELIC_ACCOUNT_ID}/$NEW_RELIC_ACCOUNT_ID/g")
    
    # Escape JSON for GraphQL
    escaped_dashboard=$(echo "$dashboard_json" | jq -c . | sed 's/"/\\"/g')
    
    # Create dashboard using NerdGraph
    mutation='mutation CreateDashboard($accountId: Int!, $dashboard: DashboardInput!) {
        dashboardCreate(accountId: $accountId, dashboard: $dashboard) {
            entityResult {
                guid
                name
                permalink
            }
            errors {
                description
                type
            }
        }
    }'
    
    variables="{
        \"accountId\": $NEW_RELIC_ACCOUNT_ID,
        \"dashboard\": $dashboard_json
    }"
    
    result=$(execute_nerdgraph "$mutation" "$variables")
    
    if echo "$result" | jq -e '.data.dashboardCreate.errors | length > 0' > /dev/null; then
        echo -e "${RED}❌ Error creating dashboard:${NC}"
        echo "$result" | jq '.data.dashboardCreate.errors'
        return 1
    else
        dashboard_url=$(echo "$result" | jq -r '.data.dashboardCreate.entityResult.permalink')
        echo -e "${GREEN}✅ Dashboard created successfully!${NC}"
        echo "   URL: $dashboard_url"
    fi
}

# Function to create alert policy
create_alert_policy() {
    echo "🚨 Creating Alert Policy..."
    
    mutation='mutation CreateAlertPolicy($accountId: Int!, $policy: AlertsPolicyInput!) {
        alertsPolicyCreate(accountId: $accountId, policy: $policy) {
            id
            name
            incidentPreference
        }
    }'
    
    variables="{
        \"accountId\": $NEW_RELIC_ACCOUNT_ID,
        \"policy\": {
            \"name\": \"MySQL Intelligence Alerts\",
            \"incidentPreference\": \"PER_CONDITION_AND_TARGET\"
        }
    }"
    
    result=$(execute_nerdgraph "$mutation" "$variables")
    policy_id=$(echo "$result" | jq -r '.data.alertsPolicyCreate.id')
    
    if [ "$policy_id" != "null" ]; then
        echo -e "${GREEN}✅ Alert policy created with ID: $policy_id${NC}"
        echo "$policy_id"
    else
        echo -e "${RED}❌ Error creating alert policy${NC}"
        return 1
    fi
}

# Function to create NRQL alert conditions
create_alert_conditions() {
    local policy_id=$1
    echo "📋 Creating Alert Conditions..."
    
    # Alert conditions to create
    conditions=(
        "Query Intelligence Score Critical|SELECT average(mysql.intelligence.comprehensive) FROM Metric WHERE intelligence_score > 150 FACET query_digest|150|300"
        "Replication Lag High|SELECT average(mysql.replica.time_behind_source) FROM Metric WHERE entity.name LIKE '%replica%'|300|180"
        "Business Revenue Impact|SELECT sum(business.revenue_impact) FROM Metric WHERE business_criticality = 'CRITICAL'|1000|60"
        "Anomaly Detection Rate|SELECT count(*) FROM Metric WHERE ml.is_anomaly = true|50|300"
        "Canary Test Failures|SELECT percentage(count(*), WHERE canary.health != 'healthy') FROM Metric WHERE metricName LIKE 'mysql.canary%'|20|180"
    )
    
    mutation='mutation CreateNrqlCondition($accountId: Int!, $policyId: ID!, $condition: AlertsNrqlConditionStaticInput!) {
        alertsNrqlConditionStaticCreate(accountId: $accountId, policyId: $policyId, condition: $condition) {
            id
            name
        }
    }'
    
    for condition in "${conditions[@]}"; do
        IFS='|' read -r name query threshold duration <<< "$condition"
        
        variables=$(cat <<EOF
{
    "accountId": $NEW_RELIC_ACCOUNT_ID,
    "policyId": "$policy_id",
    "condition": {
        "name": "$name",
        "enabled": true,
        "nrql": {
            "query": "$query"
        },
        "terms": [{
            "threshold": $threshold,
            "thresholdOccurrences": "ALL",
            "thresholdDuration": $duration,
            "operator": "ABOVE",
            "priority": "CRITICAL"
        }],
        "signal": {
            "aggregationWindow": 60,
            "aggregationMethod": "EVENT_FLOW",
            "aggregationDelay": 120
        },
        "expiration": {
            "closeViolationsOnExpiration": true,
            "expirationDuration": 3600,
            "openViolationOnExpiration": false
        },
        "violationTimeLimitSeconds": 259200
    }
}
EOF
)
        
        result=$(execute_nerdgraph "$mutation" "$variables")
        condition_id=$(echo "$result" | jq -r '.data.alertsNrqlConditionStaticCreate.id')
        
        if [ "$condition_id" != "null" ]; then
            echo -e "${GREEN}  ✅ Created condition: $name${NC}"
        else
            echo -e "${YELLOW}  ⚠️  Failed to create condition: $name${NC}"
        fi
    done
}

# Function to create workload
create_workload() {
    echo "🔧 Creating MySQL Intelligence Workload..."
    
    mutation='mutation CreateWorkload($accountId: Int!, $workload: WorkloadCreateInput!) {
        workloadCreate(accountId: $accountId, workload: $workload) {
            guid
            name
            permalink
        }
    }'
    
    variables=$(cat <<EOF
{
    "accountId": $NEW_RELIC_ACCOUNT_ID,
    "workload": {
        "name": "MySQL Intelligence Production",
        "entitySearchQueries": [{
            "query": "type = 'MYSQL_INSTANCE' AND tags.environment = 'production'"
        }],
        "scopeAccounts": {
            "accountIds": [$NEW_RELIC_ACCOUNT_ID]
        },
        "statusConfig": {
            "automatic": {
                "enabled": true,
                "remainingEntitiesRule": {
                    "rollupStrategy": "WORST_STATUS_WINS"
                },
                "rules": [{
                    "entitySearchQuery": "type = 'MYSQL_INSTANCE'",
                    "rollupStrategy": "WORST_STATUS_WINS"
                }]
            }
        }
    }
}
EOF
)
    
    result=$(execute_nerdgraph "$mutation" "$variables")
    workload_url=$(echo "$result" | jq -r '.data.workloadCreate.permalink')
    
    if [ "$workload_url" != "null" ]; then
        echo -e "${GREEN}✅ Workload created successfully!${NC}"
        echo "   URL: $workload_url"
    else
        echo -e "${YELLOW}⚠️  Failed to create workload${NC}"
    fi
}

# Function to create synthetic monitor
create_synthetic_monitor() {
    echo "🔍 Creating Synthetic Monitor for Canary Tests..."
    
    mutation='mutation CreateSyntheticMonitor($accountId: Int!, $monitor: SyntheticsCreateMonitorInput!) {
        syntheticsCreateMonitor(accountId: $accountId, monitor: $monitor) {
            monitor {
                guid
                name
                status
            }
            errors {
                description
                type
            }
        }
    }'
    
    # Base64 encode the script
    script_content=$(cat <<'EOF'
var assert = require("assert");
$http.get("http://canary-tester:8089/metrics", function(err, response, body) {
    assert.equal(response.statusCode, 200, "Expected 200 status code");
    assert.ok(body.includes("mysql.canary.latency"), "Canary metrics missing");
    
    // Parse metrics and check values
    var lines = body.split("\n");
    lines.forEach(function(line) {
        if (line.includes("mysql.canary.latency")) {
            var match = line.match(/mysql\.canary\.latency\s+(\d+)/);
            if (match) {
                var latency = parseInt(match[1]);
                assert.ok(latency < 1000, "Canary latency too high: " + latency + "ms");
            }
        }
    });
});
EOF
)
    
    script_base64=$(echo -n "$script_content" | base64 -w 0)
    
    variables=$(cat <<EOF
{
    "accountId": $NEW_RELIC_ACCOUNT_ID,
    "monitor": {
        "name": "MySQL Canary Tests",
        "type": "SCRIPT_API",
        "frequency": 5,
        "status": "ENABLED",
        "locations": ["AWS_US_EAST_1"],
        "script": "$script_base64",
        "runtime": {
            "runtimeType": "NODE_API",
            "runtimeTypeVersion": "16.10",
            "scriptLanguage": "JAVASCRIPT"
        }
    }
}
EOF
)
    
    result=$(execute_nerdgraph "$mutation" "$variables")
    
    if echo "$result" | jq -e '.data.syntheticsCreateMonitor.errors | length > 0' > /dev/null; then
        echo -e "${YELLOW}⚠️  Synthetic monitor creation had issues${NC}"
    else
        echo -e "${GREEN}✅ Synthetic monitor created successfully!${NC}"
    fi
}

# Function to create Applied Intelligence workflow
create_ai_workflow() {
    echo "🤖 Creating Applied Intelligence Workflow..."
    
    # First create a destination (webhook)
    destination_mutation='mutation CreateDestination($accountId: Int!, $destination: AiNotificationsDestinationInput!) {
        aiNotificationsCreateDestination(accountId: $accountId, destination: $destination) {
            destination {
                id
                name
            }
        }
    }'
    
    destination_vars=$(cat <<EOF
{
    "accountId": $NEW_RELIC_ACCOUNT_ID,
    "destination": {
        "name": "MySQL Intelligence Webhook",
        "type": "WEBHOOK",
        "properties": [{
            "key": "url",
            "value": "${WEBHOOK_URL:-http://alert-manager:8080/alerts}"
        }]
    }
}
EOF
)
    
    dest_result=$(execute_nerdgraph "$destination_mutation" "$destination_vars")
    destination_id=$(echo "$dest_result" | jq -r '.data.aiNotificationsCreateDestination.destination.id')
    
    if [ "$destination_id" != "null" ]; then
        echo -e "${GREEN}  ✅ Created AI destination${NC}"
        
        # Create channel
        channel_mutation='mutation CreateChannel($accountId: Int!, $channel: AiNotificationsChannelInput!) {
            aiNotificationsCreateChannel(accountId: $accountId, channel: $channel) {
                channel {
                    id
                    name
                }
            }
        }'
        
        channel_vars=$(cat <<EOF
{
    "accountId": $NEW_RELIC_ACCOUNT_ID,
    "channel": {
        "name": "MySQL Intelligence Channel",
        "type": "WEBHOOK",
        "destinationId": "$destination_id",
        "product": "IINT",
        "properties": [{
            "key": "payload",
            "value": "{ \"alert\": {{ json issueTitle }}, \"severity\": {{ json priority }}, \"details\": {{ json annotations }} }"
        }]
    }
}
EOF
)
        
        channel_result=$(execute_nerdgraph "$channel_mutation" "$channel_vars")
        channel_id=$(echo "$channel_result" | jq -r '.data.aiNotificationsCreateChannel.channel.id')
        
        if [ "$channel_id" != "null" ]; then
            echo -e "${GREEN}  ✅ Created AI channel${NC}"
        fi
    fi
}

# Function to run validation only
validate_only() {
    echo "🔍 Validating New Relic MySQL Intelligence Setup"
    echo "================================================"
    
    # Validate dashboards exist and have data
    validate_dashboards
    
    # Validate metrics are flowing
    validate_metrics_flow
    
    # Validate entity synthesis
    validate_entities
    
    echo ""
    echo -e "${GREEN}✅ Validation completed!${NC}"
}

# Function to deploy all dashboards from files
deploy_all_dashboards() {
    echo "📊 Deploying all dashboards from files..."
    
    local dashboard_files=(
        "$DASHBOARDS_DIR/mysql-intelligence-dashboard.json"
        "$DASHBOARDS_DIR/plan-explorer-dashboard.json"
        "$DASHBOARDS_DIR/database-intelligence-executive-dashboard.json"
    )
    
    for dashboard_file in "${dashboard_files[@]}"; do
        if [[ -f "$dashboard_file" ]]; then
            deploy_dashboard_from_file "$dashboard_file"
        else
            echo -e "${YELLOW}⚠️  Dashboard file not found: $dashboard_file${NC}"
        fi
    done
}

# Function to deploy dashboard from JSON file
deploy_dashboard_from_file() {
    local dashboard_file=$1
    local dashboard_name=$(basename "$dashboard_file" .json | sed 's/-/ /g' | sed 's/\b\w/\U&/g')
    
    echo "📊 Deploying dashboard: $dashboard_name"
    
    # Read and process dashboard JSON
    local dashboard_json=$(cat "$dashboard_file")
    
    # Replace environment variables
    dashboard_json=$(echo "$dashboard_json" | sed "s/\${NEW_RELIC_ACCOUNT_ID}/$NEW_RELIC_ACCOUNT_ID/g")
    
    # Create dashboard mutation
    local mutation='
    mutation CreateDashboard($accountId: Int!, $dashboard: DashboardInput!) {
        dashboardCreate(accountId: $accountId, dashboard: $dashboard) {
            entityResult {
                guid
                name
                permalink
            }
            errors {
                description
                type
            }
        }
    }'
    
    local variables="{
        \"accountId\": $NEW_RELIC_ACCOUNT_ID,
        \"dashboard\": $dashboard_json
    }"
    
    local result=$(execute_nerdgraph "$mutation" "$variables")
    
    if [ $? -eq 0 ]; then
        local errors=$(echo "$result" | jq -r '.data.dashboardCreate.errors[]?.description' 2>/dev/null || echo "")
        
        if [[ -z "$errors" ]]; then
            local dashboard_url=$(echo "$result" | jq -r '.data.dashboardCreate.entityResult.permalink')
            local dashboard_guid=$(echo "$result" | jq -r '.data.dashboardCreate.entityResult.guid')
            echo -e "${GREEN}✅ Dashboard deployed: $dashboard_name${NC}"
            echo "   GUID: $dashboard_guid"
            echo "   URL: $dashboard_url"
        else
            echo -e "${RED}❌ Dashboard deployment failed: $dashboard_name${NC}"
            echo "   Errors: $errors"
        fi
    else
        echo -e "${RED}❌ Failed to deploy dashboard: $dashboard_name${NC}"
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  setup     - Full New Relic setup (dashboards, alerts, workloads)"
    echo "  validate  - Validate existing dashboards and data flow"
    echo "  deploy    - Deploy all dashboards from JSON files"
    echo "  help      - Show this help message"
    echo ""
    echo "Environment variables required:"
    echo "  NEW_RELIC_API_KEY     - New Relic User API Key"
    echo "  NEW_RELIC_ACCOUNT_ID  - New Relic Account ID"
    echo "  NEW_RELIC_LICENSE_KEY - New Relic License Key"
    echo ""
    echo "Optional environment variables:"
    echo "  NEW_RELIC_REGION      - Set to 'EU' for EU region (default: US)"
    echo "  WEBHOOK_URL           - Webhook URL for alerts"
}

# Main execution
main() {
    local command=${1:-setup}
    
    case "$command" in
        "setup")
            check_environment
            echo "🚀 Setting up New Relic for MySQL Intelligence Monitoring"
            echo "=================================================="
            
            # Create dashboard
            create_dashboard
            
            # Create alert policy and conditions
            policy_id=$(create_alert_policy)
            if [ -n "$policy_id" ] && [ "$policy_id" != "1" ]; then
                create_alert_conditions "$policy_id"
            fi
            
            # Create workload
            create_workload
            
            # Create synthetic monitor
            create_synthetic_monitor
            
            # Create AI workflow
            create_ai_workflow
            
            echo ""
            echo -e "${GREEN}✅ New Relic setup completed!${NC}"
            echo ""
            echo "Next steps:"
            echo "1. Deploy the MySQL Intelligence collectors"
            echo "2. Run '$0 validate' to verify data is flowing"
            echo "3. Customize dashboards and alerts as needed"
            echo "4. Configure notification channels in New Relic One"
            ;;
        "validate")
            check_environment
            validate_only
            ;;
        "deploy")
            check_environment
            deploy_all_dashboards
            ;;
        "help"|"-h"|"--help")
            show_usage
            ;;
        *)
            echo -e "${RED}❌ Unknown command: $command${NC}"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# Run main function with arguments
main "$@"