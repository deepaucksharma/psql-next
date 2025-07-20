#!/bin/bash

# NerdGraph Dashboard Validation Script
# Validates dashboard existence, data flow, and entity synthesis

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_DIR="$(cd "$SCRIPT_DIR/../dashboards" && pwd)"
NERDGRAPH_URL="https://api.newrelic.com/graphql"
VALIDATION_LOG="/tmp/dashboard-validation-$(date +%Y%m%d-%H%M%S).log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${1}" | tee -a "$VALIDATION_LOG"
}

# Check required environment variables
check_environment() {
    log "${BLUE}Checking environment variables...${NC}"
    
    if [[ -z "${NEW_RELIC_API_KEY:-}" ]]; then
        log "${RED}ERROR: NEW_RELIC_API_KEY environment variable is required${NC}"
        exit 1
    fi
    
    if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
        log "${RED}ERROR: NEW_RELIC_ACCOUNT_ID environment variable is required${NC}"
        exit 1
    fi
    
    log "${GREEN}✓ Environment variables validated${NC}"
}

# Execute NerdGraph query
execute_nerdgraph() {
    local query=$1
    local variables=${2:-"{}"}
    
    response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -d "{\"query\": \"$query\", \"variables\": $variables}")
    
    # Check for GraphQL errors
    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        log "${RED}NerdGraph Error:${NC}"
        echo "$response" | jq '.errors' | tee -a "$VALIDATION_LOG"
        return 1
    fi
    
    echo "$response"
}

# Get all dashboards for the account
get_dashboards() {
    log "${BLUE}Fetching dashboards from New Relic...${NC}"
    
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
    execute_nerdgraph "$query" "$variables"
}

# Validate specific dashboard exists
validate_dashboard_exists() {
    local dashboard_name="$1"
    local dashboards_response="$2"
    
    local dashboard_guid=$(echo "$dashboards_response" | jq -r \
        ".data.actor.account.dashboards[] | select(.name == \"$dashboard_name\") | .guid")
    
    if [[ "$dashboard_guid" == "null" || -z "$dashboard_guid" ]]; then
        log "${RED}✗ Dashboard not found: $dashboard_name${NC}"
        return 1
    else
        log "${GREEN}✓ Dashboard exists: $dashboard_name (GUID: $dashboard_guid)${NC}"
        echo "$dashboard_guid"
        return 0
    fi
}

# Validate data flow for specific metrics
validate_data_flow() {
    log "${BLUE}Validating data flow for key metrics...${NC}"
    
    local metrics_to_check=(
        "mysql.intelligence.comprehensive"
        "mysql.query.exec_count"
        "mysql.connections.current"
        "mysql.threads.running"
        "mysql.table.io.read_latency"
        "system.cpu.utilization"
        "system.memory.usage"
    )
    
    for metric in "${metrics_to_check[@]}"; do
        log "${YELLOW}Checking metric: $metric${NC}"
        
        local nrql="SELECT latest($metric) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 10 minutes ago"
        local query="
        query validateMetric(\$accountId: Int!, \$nrql: Nrql!) {
            actor {
                account(id: \$accountId) {
                    nrql(query: \$nrql) {
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
        }"
        
        local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"nrql\": \"$nrql\"}"
        local response=$(execute_nerdgraph "$query" "$variables")
        
        local result_count=$(echo "$response" | jq '.data.actor.account.nrql.results | length')
        
        if [[ "$result_count" -gt 0 ]]; then
            log "${GREEN}✓ Data found for metric: $metric${NC}"
        else
            log "${RED}✗ No data found for metric: $metric${NC}"
        fi
    done
}

# Validate entity synthesis
validate_entity_synthesis() {
    log "${BLUE}Validating entity synthesis...${NC}"
    
    local entity_types=(
        "MYSQL_INSTANCE"
        "HOST"
        "SYNTHETIC_MONITOR"
        "APPLICATION"
    )
    
    for entity_type in "${entity_types[@]}"; do
        log "${YELLOW}Checking entities of type: $entity_type${NC}"
        
        local query="
        query validateEntities(\$accountId: Int!) {
            actor {
                entitySearch(query: \"type = '$entity_type' AND tags.accountId = '$NEW_RELIC_ACCOUNT_ID'\") {
                    results {
                        entities {
                            guid
                            name
                            entityType
                            tags {
                                key
                                values
                            }
                        }
                    }
                }
            }
        }"
        
        local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID}"
        local response=$(execute_nerdgraph "$query" "$variables")
        
        local entity_count=$(echo "$response" | jq '.data.actor.entitySearch.results.entities | length')
        
        if [[ "$entity_count" -gt 0 ]]; then
            log "${GREEN}✓ Found $entity_count entities of type: $entity_type${NC}"
            
            # Show entity details
            echo "$response" | jq -r '.data.actor.entitySearch.results.entities[] | "  - \(.name) (\(.guid))"' | tee -a "$VALIDATION_LOG"
        else
            log "${RED}✗ No entities found of type: $entity_type${NC}"
        fi
    done
}

# Test dashboard widgets with real data
test_dashboard_widgets() {
    local dashboard_file="$1"
    local dashboard_name=$(basename "$dashboard_file" .json)
    
    log "${BLUE}Testing widgets in dashboard: $dashboard_name${NC}"
    
    if [[ ! -f "$dashboard_file" ]]; then
        log "${RED}✗ Dashboard file not found: $dashboard_file${NC}"
        return 1
    fi
    
    # Extract NRQL queries from dashboard
    local nrql_queries=$(jq -r '
        .pages[].widgets[]? | 
        select(.configuration.nrqlQueries?) | 
        .configuration.nrqlQueries[] | 
        select(.query?) | 
        .query
    ' "$dashboard_file" 2>/dev/null || echo "")
    
    if [[ -z "$nrql_queries" ]]; then
        log "${YELLOW}! No NRQL queries found in dashboard: $dashboard_name${NC}"
        return 0
    fi
    
    local query_count=0
    local successful_queries=0
    
    while IFS= read -r nrql_query; do
        if [[ -n "$nrql_query" && "$nrql_query" != "null" ]]; then
            query_count=$((query_count + 1))
            log "${YELLOW}Testing query $query_count: ${nrql_query:0:100}...${NC}"
            
            local query="
            query testWidget(\$accountId: Int!, \$nrql: Nrql!) {
                actor {
                    account(id: \$accountId) {
                        nrql(query: \$nrql) {
                            results
                            metadata {
                                messages
                                timeWindow {
                                    begin
                                    end
                                }
                            }
                        }
                    }
                }
            }"
            
            local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"nrql\": \"$nrql_query\"}"
            
            if response=$(execute_nerdgraph "$query" "$variables" 2>/dev/null); then
                local result_count=$(echo "$response" | jq '.data.actor.account.nrql.results | length' 2>/dev/null || echo "0")
                local messages=$(echo "$response" | jq -r '.data.actor.account.nrql.metadata.messages[]?' 2>/dev/null || echo "")
                
                if [[ "$result_count" -gt 0 ]]; then
                    log "${GREEN}✓ Query returned $result_count results${NC}"
                    successful_queries=$((successful_queries + 1))
                elif [[ -n "$messages" ]]; then
                    log "${YELLOW}! Query completed with messages: $messages${NC}"
                else
                    log "${YELLOW}! Query returned no results${NC}"
                fi
            else
                log "${RED}✗ Query failed${NC}"
            fi
        fi
    done <<< "$nrql_queries"
    
    log "${BLUE}Dashboard $dashboard_name: $successful_queries/$query_count queries successful${NC}"
}

# Create dashboard if it doesn't exist
create_dashboard_if_missing() {
    local dashboard_file="$1"
    local dashboard_name=$(basename "$dashboard_file" .json | sed 's/-/ /g' | sed 's/\b\w/\U&/g')
    
    if [[ ! -f "$dashboard_file" ]]; then
        log "${RED}✗ Dashboard file not found: $dashboard_file${NC}"
        return 1
    fi
    
    log "${BLUE}Creating dashboard if missing: $dashboard_name${NC}"
    
    # Read dashboard JSON
    local dashboard_json=$(cat "$dashboard_file")
    
    # Create dashboard mutation
    local mutation='
    mutation createDashboard($accountId: Int!, $dashboard: DashboardInput!) {
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
    
    # Prepare variables
    local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"dashboard\": $dashboard_json}"
    
    if response=$(execute_nerdgraph "$mutation" "$variables"); then
        local guid=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.guid')
        local errors=$(echo "$response" | jq -r '.data.dashboardCreate.errors[]?.description' 2>/dev/null || echo "")
        
        if [[ "$guid" != "null" && -n "$guid" ]]; then
            log "${GREEN}✓ Dashboard created successfully: $dashboard_name (GUID: $guid)${NC}"
        elif [[ -n "$errors" ]]; then
            log "${YELLOW}! Dashboard creation had issues: $errors${NC}"
        else
            log "${RED}✗ Dashboard creation failed${NC}"
        fi
    else
        log "${RED}✗ Failed to execute dashboard creation mutation${NC}"
    fi
}

# Generate validation report
generate_report() {
    log "${BLUE}Generating validation report...${NC}"
    
    local report_file="/tmp/dashboard-validation-report-$(date +%Y%m%d-%H%M%S).json"
    
    cat > "$report_file" << EOF
{
    "validation_timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "account_id": "$NEW_RELIC_ACCOUNT_ID",
    "validation_log": "$VALIDATION_LOG",
    "dashboards_checked": [
        "mysql-intelligence-dashboard",
        "plan-explorer-dashboard", 
        "database-intelligence-executive-dashboard"
    ],
    "metrics_validated": [
        "mysql.intelligence.comprehensive",
        "mysql.query.exec_count",
        "mysql.connections.current",
        "system.cpu.utilization"
    ],
    "entity_types_checked": [
        "MYSQL_INSTANCE",
        "HOST", 
        "SYNTHETIC_MONITOR",
        "APPLICATION"
    ]
}
EOF
    
    log "${GREEN}✓ Validation report generated: $report_file${NC}"
}

# Main validation function
main() {
    log "${BLUE}=== New Relic Dashboard Validation Started ===${NC}"
    log "${BLUE}Timestamp: $(date)${NC}"
    log "${BLUE}Log file: $VALIDATION_LOG${NC}"
    
    # Check environment
    check_environment
    
    # Get all dashboards
    log "${BLUE}=== Step 1: Fetching Dashboards ===${NC}"
    local dashboards_response
    if dashboards_response=$(get_dashboards); then
        local dashboard_count=$(echo "$dashboards_response" | jq '.data.actor.account.dashboards | length')
        log "${GREEN}✓ Found $dashboard_count dashboards in account${NC}"
    else
        log "${RED}✗ Failed to fetch dashboards${NC}"
        exit 1
    fi
    
    # Validate specific dashboards exist
    log "${BLUE}=== Step 2: Validating Dashboard Existence ===${NC}"
    local dashboard_names=(
        "MySQL Intelligence Dashboard"
        "Plan Explorer Dashboard"
        "Database Intelligence Executive Dashboard"
    )
    
    for dashboard_name in "${dashboard_names[@]}"; do
        validate_dashboard_exists "$dashboard_name" "$dashboards_response" || true
    done
    
    # Validate data flow
    log "${BLUE}=== Step 3: Validating Data Flow ===${NC}"
    validate_data_flow
    
    # Validate entity synthesis
    log "${BLUE}=== Step 4: Validating Entity Synthesis ===${NC}"
    validate_entity_synthesis
    
    # Test dashboard widgets
    log "${BLUE}=== Step 5: Testing Dashboard Widgets ===${NC}"
    for dashboard_file in "$DASHBOARD_DIR"/*.json; do
        if [[ -f "$dashboard_file" ]]; then
            test_dashboard_widgets "$dashboard_file"
        fi
    done
    
    # Generate report
    log "${BLUE}=== Step 6: Generating Report ===${NC}"
    generate_report
    
    log "${GREEN}=== Dashboard Validation Completed ===${NC}"
    log "${BLUE}Full log available at: $VALIDATION_LOG${NC}"
}

# Script execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi