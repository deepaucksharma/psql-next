#!/bin/bash
# Deploy MySQL Wait-Based Monitoring Dashboards to New Relic
# This script uses the New Relic NerdGraph API to create or update dashboards

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_DIR="${SCRIPT_DIR}/../dashboards/newrelic"
API_KEY="${NEW_RELIC_API_KEY}"
ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID}"
NERDGRAPH_ENDPOINT="https://api.newrelic.com/graphql"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if [[ -z "${API_KEY}" ]]; then
        log_error "NEW_RELIC_API_KEY environment variable is not set"
        exit 1
    fi
    
    if [[ -z "${ACCOUNT_ID}" ]]; then
        log_error "NEW_RELIC_ACCOUNT_ID environment variable is not set"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_error "jq is not installed. Please install jq to continue."
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_error "curl is not installed. Please install curl to continue."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Function to escape JSON for GraphQL
escape_json_for_graphql() {
    # Read JSON and escape it properly for GraphQL
    jq -c . | sed 's/\\/\\\\/g' | sed 's/"/\\"/g'
}

# Function to create dashboard mutation
create_dashboard_mutation() {
    local dashboard_json="$1"
    local dashboard_name=$(echo "$dashboard_json" | jq -r '.name')
    
    # Extract dashboard configuration
    local description=$(echo "$dashboard_json" | jq -r '.description // ""')
    local permissions=$(echo "$dashboard_json" | jq -r '.permissions // "PUBLIC_READ_WRITE"')
    local pages=$(echo "$dashboard_json" | jq -c '.pages')
    
    # Build the mutation
    cat <<EOF
mutation {
  dashboardCreate(
    accountId: ${ACCOUNT_ID}
    dashboard: {
      name: "${dashboard_name}"
      description: "${description}"
      permissions: ${permissions}
      pages: $(echo "$pages" | sed 's/"accountId":[^,}]*/& /g' | sed "s/\"accountId\": *0/\"accountId\": ${ACCOUNT_ID}/g")
    }
  ) {
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
}
EOF
}

# Function to update dashboard mutation
create_update_mutation() {
    local guid="$1"
    local dashboard_json="$2"
    local dashboard_name=$(echo "$dashboard_json" | jq -r '.name')
    
    # Extract dashboard configuration
    local description=$(echo "$dashboard_json" | jq -r '.description // ""')
    local permissions=$(echo "$dashboard_json" | jq -r '.permissions // "PUBLIC_READ_WRITE"')
    local pages=$(echo "$dashboard_json" | jq -c '.pages')
    
    # Build the update mutation
    cat <<EOF
mutation {
  dashboardUpdate(
    guid: "${guid}"
    dashboard: {
      name: "${dashboard_name}"
      description: "${description}"
      permissions: ${permissions}
      pages: $(echo "$pages" | sed 's/"accountId":[^,}]*/& /g' | sed "s/\"accountId\": *0/\"accountId\": ${ACCOUNT_ID}/g")
    }
  ) {
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
}
EOF
}

# Function to check if dashboard exists
check_dashboard_exists() {
    local dashboard_name="$1"
    
    local query=$(cat <<EOF
{
  actor {
    entitySearch(query: "name = '${dashboard_name}' AND type = 'DASHBOARD' AND accountId = ${ACCOUNT_ID}") {
      results {
        entities {
          guid
          name
        }
      }
    }
  }
}
EOF
)
    
    local response=$(curl -s -X POST "${NERDGRAPH_ENDPOINT}" \
        -H "Content-Type: application/json" \
        -H "API-Key: ${API_KEY}" \
        -d "{\"query\": \"$(echo "$query" | tr '\n' ' ' | sed 's/"/\\"/g')\"}")
    
    local guid=$(echo "$response" | jq -r '.data.actor.entitySearch.results.entities[0].guid // empty')
    echo "$guid"
}

# Function to execute GraphQL mutation
execute_mutation() {
    local mutation="$1"
    
    # Create request body
    local request_body=$(jq -n --arg query "$mutation" '{query: $query}')
    
    # Execute the mutation
    local response=$(curl -s -X POST "${NERDGRAPH_ENDPOINT}" \
        -H "Content-Type: application/json" \
        -H "API-Key: ${API_KEY}" \
        -d "$request_body")
    
    echo "$response"
}

# Function to deploy a single dashboard
deploy_dashboard() {
    local dashboard_file="$1"
    local dashboard_name=$(basename "$dashboard_file" .json)
    
    log_info "Deploying dashboard: $dashboard_name"
    
    # Read dashboard JSON
    local dashboard_json=$(cat "$dashboard_file")
    local dashboard_display_name=$(echo "$dashboard_json" | jq -r '.name')
    
    # Check if dashboard already exists
    local existing_guid=$(check_dashboard_exists "$dashboard_display_name")
    
    if [[ -n "$existing_guid" ]]; then
        log_warning "Dashboard '$dashboard_display_name' already exists. Updating..."
        local mutation=$(create_update_mutation "$existing_guid" "$dashboard_json")
        local response=$(execute_mutation "$mutation")
        
        # Check for errors in update
        local errors=$(echo "$response" | jq -r '.data.dashboardUpdate.errors[]?.description // empty' 2>/dev/null)
        if [[ -n "$errors" ]]; then
            log_error "Failed to update dashboard: $errors"
            echo "$response" | jq '.'
            return 1
        fi
        
        local permalink=$(echo "$response" | jq -r '.data.dashboardUpdate.entityResult.permalink // empty')
        log_success "Dashboard updated successfully: $permalink"
    else
        log_info "Creating new dashboard '$dashboard_display_name'..."
        local mutation=$(create_dashboard_mutation "$dashboard_json")
        local response=$(execute_mutation "$mutation")
        
        # Check for errors in creation
        local errors=$(echo "$response" | jq -r '.data.dashboardCreate.errors[]?.description // empty' 2>/dev/null)
        if [[ -n "$errors" ]]; then
            log_error "Failed to create dashboard: $errors"
            echo "$response" | jq '.'
            return 1
        fi
        
        local permalink=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.permalink // empty')
        log_success "Dashboard created successfully: $permalink"
    fi
}

# Main deployment function
deploy_all_dashboards() {
    log_info "Starting dashboard deployment to New Relic..."
    
    # Get list of dashboard files
    local dashboard_files=(
        "${DASHBOARD_DIR}/mysql-dashboard-enhanced.json"
        "${DASHBOARD_DIR}/wait-analysis-dashboard-enhanced.json"
        "${DASHBOARD_DIR}/query-detail-dashboard-enhanced.json"
    )
    
    # Deploy each dashboard
    local success_count=0
    local total_count=${#dashboard_files[@]}
    
    for dashboard_file in "${dashboard_files[@]}"; do
        if [[ -f "$dashboard_file" ]]; then
            if deploy_dashboard "$dashboard_file"; then
                ((success_count++))
            fi
        else
            log_warning "Dashboard file not found: $dashboard_file"
        fi
    done
    
    log_info "Deployment complete. Successfully deployed $success_count out of $total_count dashboards."
    
    if [[ $success_count -eq $total_count ]]; then
        log_success "All dashboards deployed successfully!"
        return 0
    else
        log_warning "Some dashboards failed to deploy."
        return 1
    fi
}

# Function to list deployed dashboards
list_dashboards() {
    log_info "Listing deployed MySQL dashboards..."
    
    local query=$(cat <<EOF
{
  actor {
    entitySearch(query: "type = 'DASHBOARD' AND accountId = ${ACCOUNT_ID} AND name LIKE '%MySQL%'") {
      results {
        entities {
          guid
          name
          permalink
          tags {
            key
            values
          }
        }
      }
    }
  }
}
EOF
)
    
    local response=$(curl -s -X POST "${NERDGRAPH_ENDPOINT}" \
        -H "Content-Type: application/json" \
        -H "API-Key: ${API_KEY}" \
        -d "{\"query\": \"$(echo "$query" | tr '\n' ' ' | sed 's/"/\\"/g')\"}")
    
    echo "$response" | jq -r '.data.actor.entitySearch.results.entities[] | "\(.name): \(.permalink)"'
}

# Main script execution
main() {
    case "${1:-deploy}" in
        deploy)
            check_prerequisites
            deploy_all_dashboards
            ;;
        list)
            check_prerequisites
            list_dashboards
            ;;
        help|--help|-h)
            echo "Usage: $0 [deploy|list|help]"
            echo ""
            echo "Commands:"
            echo "  deploy  - Deploy all MySQL dashboards to New Relic (default)"
            echo "  list    - List all deployed MySQL dashboards"
            echo "  help    - Show this help message"
            echo ""
            echo "Environment variables required:"
            echo "  NEW_RELIC_API_KEY    - Your New Relic API key"
            echo "  NEW_RELIC_ACCOUNT_ID - Your New Relic account ID"
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"