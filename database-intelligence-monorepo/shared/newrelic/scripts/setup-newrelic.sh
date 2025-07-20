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

# Check required environment variables
echo "🔍 Checking environment variables..."
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo -e "${RED}❌ Error: $var is not set${NC}"
        exit 1
    fi
done
echo -e "${GREEN}✅ All required environment variables are set${NC}"

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
    
    echo "$response"
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

# Main execution
main() {
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
    echo "2. Verify data is flowing to New Relic"
    echo "3. Customize dashboards and alerts as needed"
    echo "4. Configure notification channels in New Relic One"
}

# Run main function
main