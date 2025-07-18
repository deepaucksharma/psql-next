#!/bin/bash
# Verify NRQL Queries for MySQL Wait-Based Monitoring Dashboards
# This script tests all NRQL queries to ensure they return data

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

# Query results storage - using temp files instead of associative arrays
QUERY_RESULTS_FILE="/tmp/query_results.$$"
QUERY_ERRORS_FILE="/tmp/query_errors.$$"
total_queries=0
successful_queries=0
failed_queries=0
warning_queries=0

# Clean up temp files on exit
trap 'rm -f "$QUERY_RESULTS_FILE" "$QUERY_ERRORS_FILE"' EXIT

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

# Function to execute NRQL query
execute_nrql_query() {
    local nrql_query="$1"
    local widget_title="$2"
    local dashboard_name="$3"
    
    # Replace accountId placeholder
    nrql_query=$(echo "$nrql_query" | sed "s/accountId: *0/accountId: ${ACCOUNT_ID}/g")
    
    # Replace template variables with sample values
    nrql_query=$(echo "$nrql_query" | sed "s/{{query_hash}}/'sample_hash'/g")
    nrql_query=$(echo "$nrql_query" | sed "s/{{query_pattern}}/'%SELECT%'/g")
    
    # Build GraphQL query
    local graphql_query=$(cat <<EOF
{
  actor {
    account(id: ${ACCOUNT_ID}) {
      nrql(query: "${nrql_query}") {
        results
        totalResult
        metadata {
          eventTypes
          messages
          timeWindow {
            begin
            end
          }
        }
      }
    }
  }
}
EOF
)
    
    # Execute query
    local response=$(curl -s -X POST "${NERDGRAPH_ENDPOINT}" \
        -H "Content-Type: application/json" \
        -H "API-Key: ${API_KEY}" \
        -d "{\"query\": \"$(echo "$graphql_query" | tr '\n' ' ' | sed 's/"/\\"/g')\"}")
    
    # Check for errors
    local errors=$(echo "$response" | jq -r '.errors[]?.message // empty' 2>/dev/null)
    if [[ -n "$errors" ]]; then
        echo "${dashboard_name}:${widget_title}|$errors" >> "$QUERY_ERRORS_FILE"
        return 1
    fi
    
    # Check if results exist
    local results=$(echo "$response" | jq -r '.data.actor.account.nrql.results // empty' 2>/dev/null)
    local total_result=$(echo "$response" | jq -r '.data.actor.account.nrql.totalResult // empty' 2>/dev/null)
    local messages=$(echo "$response" | jq -r '.data.actor.account.nrql.metadata.messages[]? // empty' 2>/dev/null)
    
    if [[ -z "$results" ]] || [[ "$results" == "[]" ]]; then
        if [[ -n "$messages" ]]; then
            echo "${dashboard_name}:${widget_title}|No data (Messages: $messages)" >> "$QUERY_RESULTS_FILE"
            return 2
        else
            echo "${dashboard_name}:${widget_title}|No data returned" >> "$QUERY_RESULTS_FILE"
            return 2
        fi
    fi
    
    # Success
    local result_count=$(echo "$results" | jq 'length' 2>/dev/null || echo "0")
    echo "${dashboard_name}:${widget_title}|Success (${result_count} results)" >> "$QUERY_RESULTS_FILE"
    return 0
}

# Function to extract and test queries from dashboard
test_dashboard_queries() {
    local dashboard_file="$1"
    local dashboard_name=$(basename "$dashboard_file" .json)
    
    log_info "Testing queries in dashboard: $dashboard_name"
    
    # Read dashboard JSON
    local dashboard_json=$(cat "$dashboard_file")
    local dashboard_display_name=$(echo "$dashboard_json" | jq -r '.name')
    
    # Extract all widgets and their queries
    local pages=$(echo "$dashboard_json" | jq -c '.pages[]')
    
    while IFS= read -r page; do
        local page_name=$(echo "$page" | jq -r '.name')
        local widgets=$(echo "$page" | jq -c '.widgets[]')
        
        while IFS= read -r widget; do
            local widget_title=$(echo "$widget" | jq -r '.title // "Untitled"')
            
            # Extract NRQL queries from different widget types
            local nrql_queries=$(echo "$widget" | jq -r '
                .configuration | 
                to_entries | 
                .[].value.nrqlQueries[]?.query // empty
            ' 2>/dev/null)
            
            if [[ -n "$nrql_queries" ]]; then
                while IFS= read -r query; do
                    if [[ -n "$query" ]]; then
                        ((total_queries++))
                        
                        log_info "Testing: ${page_name} > ${widget_title}"
                        
                        if execute_nrql_query "$query" "$widget_title" "$dashboard_display_name"; then
                            ((successful_queries++))
                            log_success "Query successful"
                        elif [[ $? -eq 2 ]]; then
                            ((warning_queries++))
                            log_warning "Query returned no data"
                        else
                            ((failed_queries++))
                            log_error "Query failed"
                        fi
                    fi
                done <<< "$nrql_queries"
            fi
        done <<< "$widgets"
    done <<< "$pages"
}

# Function to generate summary report
generate_summary_report() {
    local report_file="${SCRIPT_DIR}/../dashboards/newrelic/query-verification-report.md"
    
    cat > "$report_file" <<EOF
# NRQL Query Verification Report

**Generated:** $(date)  
**Account ID:** ${ACCOUNT_ID}

## Summary

- **Total Queries Tested:** ${total_queries}
- **Successful Queries:** ${successful_queries} ($(awk "BEGIN {printf \"%.1f\", ${successful_queries}*100/${total_queries}}")%)
- **Failed Queries:** ${failed_queries} ($(awk "BEGIN {printf \"%.1f\", ${failed_queries}*100/${total_queries}}")%)
- **Queries with No Data:** ${warning_queries} ($(awk "BEGIN {printf \"%.1f\", ${warning_queries}*100/${total_queries}}")%)

## Query Results

### Successful Queries
EOF
    
    # Add successful queries
    if [[ -f "$QUERY_RESULTS_FILE" ]]; then
        while IFS='|' read -r key result; do
            if [[ "$result" == *"Success"* ]]; then
                echo "- **${key}**: ${result}" >> "$report_file"
            fi
        done < "$QUERY_RESULTS_FILE"
    fi
    
    echo "" >> "$report_file"
    echo "### Queries with Warnings" >> "$report_file"
    
    # Add warning queries
    if [[ -f "$QUERY_RESULTS_FILE" ]]; then
        while IFS='|' read -r key result; do
            if [[ "$result" == *"No data"* ]]; then
                echo "- **${key}**: ${result}" >> "$report_file"
            fi
        done < "$QUERY_RESULTS_FILE"
    fi
    
    echo "" >> "$report_file"
    echo "### Failed Queries" >> "$report_file"
    
    # Add failed queries
    if [[ -f "$QUERY_ERRORS_FILE" ]]; then
        while IFS='|' read -r key error; do
            echo "- **${key}**: ${error}" >> "$report_file"
        done < "$QUERY_ERRORS_FILE"
    fi
    
    echo "" >> "$report_file"
    echo "## Recommendations" >> "$report_file"
    
    if [[ $warning_queries -gt 0 ]]; then
        cat >> "$report_file" <<EOF

### For Queries with No Data:
1. Ensure the OpenTelemetry collector is sending data with the expected attributes
2. Verify that the metric names and attribute names match your collector configuration
3. Check if data exists for the specified time range
4. Confirm that the instrumentation.provider attribute is set to 'opentelemetry'
EOF
    fi
    
    if [[ $failed_queries -gt 0 ]]; then
        cat >> "$report_file" <<EOF

### For Failed Queries:
1. Check the NRQL syntax for errors
2. Verify that all referenced attributes exist in your data
3. Ensure function calls are properly formatted
4. Check for any deprecated NRQL features
EOF
    fi
    
    log_success "Verification report generated: $report_file"
}

# Function to test sample data availability
test_data_availability() {
    log_info "Testing basic data availability..."
    
    local test_queries=(
        "SELECT count(*) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 1 hour ago"
        "SELECT uniqueCount(metricName) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 1 hour ago"
        "SELECT uniqueCount(mysql.instance.endpoint) FROM Metric WHERE instrumentation.provider = 'opentelemetry' AND mysql.instance.endpoint IS NOT NULL SINCE 1 hour ago"
    )
    
    for query in "${test_queries[@]}"; do
        log_info "Testing: $query"
        if execute_nrql_query "$query" "Data Availability Test" "System"; then
            log_success "Data is available"
        else
            log_error "No OpenTelemetry data found"
            log_warning "Make sure the OpenTelemetry collector is running and sending data to New Relic"
            return 1
        fi
    done
    
    return 0
}

# Main verification function
verify_all_queries() {
    log_info "Starting NRQL query verification..."
    
    # First test if any data is available
    if ! test_data_availability; then
        log_error "No OpenTelemetry data available. Please ensure data is being sent to New Relic."
        exit 1
    fi
    
    # Get list of dashboard files
    local dashboard_files=(
        "${DASHBOARD_DIR}/mysql-dashboard-enhanced.json"
        "${DASHBOARD_DIR}/wait-analysis-dashboard-enhanced.json"
        "${DASHBOARD_DIR}/query-detail-dashboard-enhanced.json"
    )
    
    # Test each dashboard
    for dashboard_file in "${dashboard_files[@]}"; do
        if [[ -f "$dashboard_file" ]]; then
            test_dashboard_queries "$dashboard_file"
        else
            log_warning "Dashboard file not found: $dashboard_file"
        fi
    done
    
    # Generate summary report
    generate_summary_report
    
    log_info "Verification complete!"
    log_info "Total: $total_queries, Success: $successful_queries, Failed: $failed_queries, No Data: $warning_queries"
    
    if [[ $failed_queries -eq 0 ]]; then
        log_success "All queries executed successfully!"
        return 0
    else
        log_warning "Some queries failed. Check the report for details."
        return 1
    fi
}

# Function to test specific widget
test_specific_widget() {
    local dashboard_name="$1"
    local widget_title="$2"
    
    log_info "Testing specific widget: $dashboard_name > $widget_title"
    
    # Find dashboard file
    local dashboard_file=""
    for file in "${DASHBOARD_DIR}"/*.json; do
        local name=$(jq -r '.name' "$file" 2>/dev/null)
        if [[ "$name" == *"$dashboard_name"* ]]; then
            dashboard_file="$file"
            break
        fi
    done
    
    if [[ -z "$dashboard_file" ]]; then
        log_error "Dashboard not found: $dashboard_name"
        return 1
    fi
    
    # Find and test widget
    local found=false
    local dashboard_json=$(cat "$dashboard_file")
    local pages=$(echo "$dashboard_json" | jq -c '.pages[]')
    
    while IFS= read -r page; do
        local widgets=$(echo "$page" | jq -c '.widgets[]')
        
        while IFS= read -r widget; do
            local title=$(echo "$widget" | jq -r '.title // "Untitled"')
            
            if [[ "$title" == *"$widget_title"* ]]; then
                found=true
                local nrql_queries=$(echo "$widget" | jq -r '
                    .configuration | 
                    to_entries | 
                    .[].value.nrqlQueries[]?.query // empty
                ' 2>/dev/null)
                
                if [[ -n "$nrql_queries" ]]; then
                    while IFS= read -r query; do
                        if [[ -n "$query" ]]; then
                            log_info "Query: $query"
                            if execute_nrql_query "$query" "$title" "$dashboard_name"; then
                                log_success "Query successful"
                            else
                                log_error "Query failed"
                            fi
                        fi
                    done <<< "$nrql_queries"
                fi
            fi
        done <<< "$widgets"
    done <<< "$pages"
    
    if [[ "$found" == "false" ]]; then
        log_error "Widget not found: $widget_title"
        return 1
    fi
}

# Main script execution
main() {
    case "${1:-verify}" in
        verify)
            check_prerequisites
            verify_all_queries
            ;;
        test)
            check_prerequisites
            if [[ $# -lt 3 ]]; then
                log_error "Usage: $0 test <dashboard_name> <widget_title>"
                exit 1
            fi
            test_specific_widget "$2" "$3"
            ;;
        help|--help|-h)
            echo "Usage: $0 [verify|test|help]"
            echo ""
            echo "Commands:"
            echo "  verify  - Verify all NRQL queries in all dashboards (default)"
            echo "  test    - Test a specific widget's queries"
            echo "  help    - Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0 verify"
            echo "  $0 test 'MySQL Performance' 'Connection Utilization'"
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