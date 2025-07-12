#!/bin/bash

# Dashboard Migration Script - OHI to OpenTelemetry
# This script helps migrate New Relic dashboards from OHI to OTel format

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DASHBOARDS_DIR="$PROJECT_ROOT/dashboards"
OHI_DASHBOARD="$DASHBOARDS_DIR/newrelic/database-intelligence-dashboard.json"
OTEL_DASHBOARD="$DASHBOARDS_DIR/otel/database-intelligence-otel.json"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."
    
    local deps=("jq" "curl")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            log_error "$dep is required but not installed"
            exit 1
        fi
    done
    
    log_success "All dependencies are installed"
}

# Validate New Relic credentials
validate_credentials() {
    if [[ -z "${NEW_RELIC_API_KEY:-}" ]]; then
        log_error "NEW_RELIC_API_KEY environment variable is not set"
        exit 1
    fi
    
    if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
        log_error "NEW_RELIC_ACCOUNT_ID environment variable is not set"
        exit 1
    fi
    
    log_success "New Relic credentials validated"
}

# Backup existing dashboard
backup_dashboard() {
    local dashboard_guid=$1
    local backup_file="$DASHBOARDS_DIR/backups/dashboard_${dashboard_guid}_$(date +%Y%m%d_%H%M%S).json"
    
    mkdir -p "$DASHBOARDS_DIR/backups"
    
    log_info "Backing up dashboard $dashboard_guid..."
    
    curl -s -X POST https://api.newrelic.com/graphql \
        -H "Api-Key: $NEW_RELIC_API_KEY" \
        -H "Content-Type: application/json" \
        -d @- <<EOF | jq '.data.dashboardExport' > "$backup_file"
{
  "query": "query { dashboardExport(guid: \"$dashboard_guid\") { dashboardJson } }"
}
EOF
    
    if [[ -s "$backup_file" ]]; then
        log_success "Dashboard backed up to $backup_file"
    else
        log_error "Failed to backup dashboard"
        exit 1
    fi
}

# Translate OHI queries to OTel
translate_queries() {
    local input_file=$1
    local output_file=$2
    
    log_info "Translating queries from OHI to OTel format..."
    
    # Create a temporary file for processing
    local temp_file=$(mktemp)
    cp "$input_file" "$temp_file"
    
    # Translation rules
    declare -A translations=(
        ["FROM PostgresSlowQueries"]="FROM Metric WHERE metricName LIKE 'postgres.slow_queries%'"
        ["FROM PostgresWaitEvents"]="FROM Metric WHERE metricName LIKE 'postgres.wait_events%'"
        ["facet query_id"]="FACET attributes.db.postgresql.query_id"
        ["facet database_name"]="FACET attributes.db.name"
        ["facet wait_event_name"]="FACET attributes.db.wait_event.name"
        ["avg_elapsed_time_ms"]="postgres.slow_queries.elapsed_time"
        ["execution_count"]="postgres.slow_queries.count"
        ["avg_disk_reads"]="postgres.slow_queries.disk_reads"
        ["avg_disk_writes"]="postgres.slow_queries.disk_writes"
        ["total_wait_time_ms"]="db.ash.wait_events"
        ["query_text"]="attributes.db.statement"
        ["schema_name"]="attributes.db.schema"
        ["statement_type"]="attributes.db.operation"
    )
    
    # Apply translations
    for old in "${!translations[@]}"; do
        new="${translations[$old]}"
        sed -i "s|$old|$new|g" "$temp_file"
    done
    
    # Process with jq to ensure valid JSON
    jq '.' "$temp_file" > "$output_file"
    rm -f "$temp_file"
    
    log_success "Query translation completed"
}

# Deploy dashboard to New Relic
deploy_dashboard() {
    local dashboard_file=$1
    local dashboard_name=$2
    
    log_info "Deploying dashboard: $dashboard_name..."
    
    # Read dashboard JSON
    local dashboard_json=$(jq -c '.' "$dashboard_file")
    
    # Create GraphQL mutation
    local mutation=$(cat <<EOF
mutation {
  dashboardCreate(
    accountId: $NEW_RELIC_ACCOUNT_ID,
    dashboard: $dashboard_json
  ) {
    errors {
      description
    }
    entityResult {
      guid
      name
      permalink
    }
  }
}
EOF
)
    
    # Execute mutation
    local response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Api-Key: $NEW_RELIC_API_KEY" \
        -H "Content-Type: application/json" \
        -d "{\"query\": \"$mutation\"}")
    
    # Check for errors
    local errors=$(echo "$response" | jq -r '.data.dashboardCreate.errors[]?.description' 2>/dev/null)
    if [[ -n "$errors" ]]; then
        log_error "Failed to deploy dashboard: $errors"
        return 1
    fi
    
    # Extract dashboard info
    local guid=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.guid')
    local permalink=$(echo "$response" | jq -r '.data.dashboardCreate.entityResult.permalink')
    
    log_success "Dashboard deployed successfully!"
    log_info "GUID: $guid"
    log_info "URL: $permalink"
    
    echo "$guid"
}

# Validate dashboard data parity
validate_parity() {
    local ohi_guid=$1
    local otel_guid=$2
    
    log_info "Validating data parity between OHI and OTel dashboards..."
    
    # This would typically run validation queries
    # For now, we'll just log the action
    log_warning "Manual validation required - compare dashboards:"
    log_info "OHI Dashboard: https://one.newrelic.com/redirect/entity/$ohi_guid"
    log_info "OTel Dashboard: https://one.newrelic.com/redirect/entity/$otel_guid"
}

# Main migration flow
main() {
    log_info "Starting dashboard migration from OHI to OpenTelemetry..."
    
    # Parse arguments
    local mode="${1:-full}"
    local dashboard_guid="${2:-}"
    
    # Validate environment
    check_dependencies
    validate_credentials
    
    case "$mode" in
        "backup")
            if [[ -z "$dashboard_guid" ]]; then
                log_error "Dashboard GUID required for backup mode"
                exit 1
            fi
            backup_dashboard "$dashboard_guid"
            ;;
            
        "translate")
            translate_queries "$OHI_DASHBOARD" "$OTEL_DASHBOARD"
            log_success "Translation completed. Review: $OTEL_DASHBOARD"
            ;;
            
        "deploy")
            if [[ ! -f "$OTEL_DASHBOARD" ]]; then
                log_error "OTel dashboard not found. Run translate mode first."
                exit 1
            fi
            deploy_dashboard "$OTEL_DASHBOARD" "Database Intelligence - OpenTelemetry"
            ;;
            
        "full")
            # Full migration flow
            if [[ -z "$dashboard_guid" ]]; then
                log_warning "No dashboard GUID provided, skipping backup"
            else
                backup_dashboard "$dashboard_guid"
            fi
            
            translate_queries "$OHI_DASHBOARD" "$OTEL_DASHBOARD"
            local new_guid=$(deploy_dashboard "$OTEL_DASHBOARD" "Database Intelligence - OpenTelemetry")
            
            if [[ -n "$dashboard_guid" ]] && [[ -n "$new_guid" ]]; then
                validate_parity "$dashboard_guid" "$new_guid"
            fi
            ;;
            
        *)
            log_error "Invalid mode: $mode"
            echo "Usage: $0 [backup|translate|deploy|full] [dashboard_guid]"
            exit 1
            ;;
    esac
    
    log_success "Migration process completed!"
}

# Run main function
main "$@"