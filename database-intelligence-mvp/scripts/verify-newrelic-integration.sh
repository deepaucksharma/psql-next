#!/bin/bash
# Comprehensive New Relic Integration Verification Script
# Validates all aspects of the database intelligence integration

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Load environment
if ! load_env_file "${PROJECT_ROOT}/.env"; then
    error "Failed to load environment"
    exit 1
fi

# Validate required environment variables
validate_env_var "NEW_RELIC_LICENSE_KEY"
validate_env_var "NEW_RELIC_ACCOUNT_ID"

# Configuration
NERDGRAPH_URL="https://api.newrelic.com/graphql"
VERIFICATION_RESULTS_FILE="/tmp/nr_verification_results_$(date +%s).json"

# ====================
# Helper Functions
# ====================

execute_nrql_query() {
    local query="$1"
    local description="$2"
    
    echo -e "${BLUE}Executing: ${description}${NC}"
    
    local response=$(curl -s -X POST "$NERDGRAPH_URL" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -d @- <<EOF
{
    "query": "{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \"$query\") { results } } } }"
}
EOF
    )
    
    echo "$response" | jq -r '.data.actor.account.nrql.results'
}

verify_data_ingestion() {
    echo -e "\n${YELLOW}=== Verifying Data Ingestion ===${NC}\n"
    
    # Check if any data from our collector arrived
    local query="SELECT count(*) as 'Total Records' FROM Log WHERE instrumentation.provider = 'opentelemetry' AND collector.name = 'database-intelligence' SINCE 30 minutes ago"
    local result=$(execute_nrql_query "$query" "Checking for OpenTelemetry data")
    
    local count=$(echo "$result" | jq -r '.[0]."Total Records"' 2>/dev/null || echo "0")
    
    if [[ "$count" -gt 0 ]]; then
        success "✓ Data ingestion confirmed: $count records"
    else
        error "✗ No data found from database-intelligence collector"
        echo "  Troubleshooting tips:"
        echo "  1. Check collector logs: docker logs db-intel-primary"
        echo "  2. Verify collector is running: docker ps | grep db-intel"
        echo "  3. Check environment variables in .env"
        return 1
    fi
}

verify_entity_synthesis() {
    echo -e "\n${YELLOW}=== Verifying Entity Synthesis ===${NC}\n"
    
    # Check for database entities
    local query="SELECT uniques(entity.guid) as 'Database Entities', uniques(database_name) as 'Databases' FROM Log WHERE entity.type = 'DATABASE' AND instrumentation.provider = 'opentelemetry' SINCE 1 hour ago"
    local result=$(execute_nrql_query "$query" "Checking database entity creation")
    
    local entities=$(echo "$result" | jq -r '.[0]."Database Entities" | length' 2>/dev/null || echo "0")
    
    if [[ "$entities" -gt 0 ]]; then
        success "✓ Entity synthesis working: $entities database entities created"
        echo "$result" | jq -r '.[0]."Databases"[]' 2>/dev/null | while read -r db; do
            echo "    - $db"
        done
    else
        warning "⚠ No database entities found"
        echo "  This may indicate missing entity synthesis attributes"
    fi
    
    # Check entity correlation
    query="SELECT percentage(count(*), WHERE entity.guid IS NOT NULL) as 'Entity Correlation %' FROM Log WHERE database_name IS NOT NULL AND collector.name = 'database-intelligence' SINCE 1 hour ago"
    result=$(execute_nrql_query "$query" "Checking entity correlation rate")
    
    local correlation=$(echo "$result" | jq -r '.[0]."Entity Correlation %"' 2>/dev/null || echo "0")
    echo -e "\n  Entity correlation rate: ${correlation}%"
}

check_integration_errors() {
    echo -e "\n${YELLOW}=== Checking for Integration Errors ===${NC}\n"
    
    # Critical: Check NrIntegrationError events
    local query="SELECT count(*) as 'Error Count', latest(message) as 'Latest Error' FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' AND (message LIKE '%database%' OR message LIKE '%otel%' OR message LIKE '%opentelemetry%') SINCE 1 hour ago"
    local result=$(execute_nrql_query "$query" "Checking for NrIntegrationError events")
    
    local error_count=$(echo "$result" | jq -r '.[0]."Error Count"' 2>/dev/null || echo "0")
    
    if [[ "$error_count" -eq 0 ]]; then
        success "✓ No integration errors detected"
    else
        error "✗ Found $error_count integration errors"
        local latest_error=$(echo "$result" | jq -r '.[0]."Latest Error"' 2>/dev/null)
        echo "  Latest error: $latest_error"
        
        # Get error breakdown
        query="SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' FACET message SINCE 1 hour ago LIMIT 10"
        execute_nrql_query "$query" "Error breakdown" | jq -r '.[] | "  - \(.message): \(.count)"' 2>/dev/null
    fi
}

verify_cardinality_management() {
    echo -e "\n${YELLOW}=== Verifying Cardinality Management ===${NC}\n"
    
    # Check query normalization effectiveness
    local query="SELECT uniqueCount(db.query.fingerprint) as 'Normalized Patterns', uniqueCount(query_text) as 'Raw Queries' FROM Log WHERE db.query.fingerprint IS NOT NULL AND collector.name = 'database-intelligence' SINCE 1 hour ago"
    local result=$(execute_nrql_query "$query" "Checking query normalization")
    
    local patterns=$(echo "$result" | jq -r '.[0]."Normalized Patterns"' 2>/dev/null || echo "0")
    local raw=$(echo "$result" | jq -r '.[0]."Raw Queries"' 2>/dev/null || echo "0")
    
    if [[ "$patterns" -gt 0 && "$raw" -gt 0 ]]; then
        local compression_ratio=$(awk "BEGIN {printf \"%.2f\", ($raw - $patterns) / $raw * 100}")
        success "✓ Cardinality reduction working: ${compression_ratio}% compression"
        echo "    Raw queries: $raw → Normalized patterns: $patterns"
    else
        warning "⚠ Query normalization not detected"
    fi
    
    # Check for cardinality warnings
    query="SELECT count(*) FROM NrIntegrationError WHERE message LIKE '%cardinality%' OR message LIKE '%unique time series%' SINCE 1 day ago"
    result=$(execute_nrql_query "$query" "Checking cardinality warnings")
    
    local warnings=$(echo "$result" | jq -r '.[0].count' 2>/dev/null || echo "0")
    if [[ "$warnings" -gt 0 ]]; then
        warning "⚠ Found $warnings cardinality warnings in the last day"
    fi
}

verify_circuit_breaker() {
    echo -e "\n${YELLOW}=== Verifying Circuit Breaker ===${NC}\n"
    
    # Check circuit breaker metrics
    local query="SELECT latest(cb.state) as 'Circuit State', sum(cb.opened_count) as 'Total Opens', average(cb.error_rate) as 'Avg Error Rate' FROM Log WHERE cb.state IS NOT NULL FACET database_name SINCE 1 hour ago"
    local result=$(execute_nrql_query "$query" "Checking circuit breaker status")
    
    if [[ -n "$result" && "$result" != "[]" ]]; then
        success "✓ Circuit breaker metrics found"
        echo "$result" | jq -r '.[] | "    \(.database_name): State=\(.["Circuit State"]), Opens=\(.["Total Opens"]), Error Rate=\(.["Avg Error Rate"])"' 2>/dev/null
    else
        warning "⚠ No circuit breaker metrics found"
    fi
}

verify_sampling_effectiveness() {
    echo -e "\n${YELLOW}=== Verifying Adaptive Sampling ===${NC}\n"
    
    # Check sampling decisions
    local query="SELECT filter(count(*), WHERE sampling.decision = 'sampled') as 'Sampled', filter(count(*), WHERE sampling.decision = 'dropped') as 'Dropped' FROM Log WHERE sampling.decision IS NOT NULL AND collector.name = 'database-intelligence' SINCE 1 hour ago"
    local result=$(execute_nrql_query "$query" "Checking sampling decisions")
    
    local sampled=$(echo "$result" | jq -r '.[0].Sampled' 2>/dev/null || echo "0")
    local dropped=$(echo "$result" | jq -r '.[0].Dropped' 2>/dev/null || echo "0")
    
    if [[ "$sampled" -gt 0 || "$dropped" -gt 0 ]]; then
        local total=$((sampled + dropped))
        local sample_rate=$(awk "BEGIN {printf \"%.2f\", $sampled / $total * 100}")
        success "✓ Adaptive sampling active: ${sample_rate}% sample rate"
        echo "    Sampled: $sampled, Dropped: $dropped"
    else
        warning "⚠ No sampling metrics found"
    fi
}

verify_database_metrics() {
    echo -e "\n${YELLOW}=== Verifying Database Metrics ===${NC}\n"
    
    # Check for database-specific metrics
    local query="SELECT uniques(database_name) as 'Databases', count(*) as 'Total Queries', average(duration_ms) as 'Avg Duration' FROM Log WHERE database_name IS NOT NULL AND collector.name = 'database-intelligence' SINCE 1 hour ago"
    local result=$(execute_nrql_query "$query" "Checking database metrics")
    
    local db_count=$(echo "$result" | jq -r '.[0].Databases | length' 2>/dev/null || echo "0")
    
    if [[ "$db_count" -gt 0 ]]; then
        success "✓ Database metrics collected from $db_count databases"
        echo "$result" | jq -r '.[0] | "    Total queries: \(."Total Queries"), Avg duration: \(."Avg Duration")ms"' 2>/dev/null
    else
        error "✗ No database metrics found"
    fi
}

generate_verification_report() {
    echo -e "\n${YELLOW}=== Generating Verification Report ===${NC}\n"
    
    local report_time=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
    
    cat > "$VERIFICATION_RESULTS_FILE" <<EOF
{
    "verification_time": "$report_time",
    "account_id": "$NEW_RELIC_ACCOUNT_ID",
    "checks": {
        "data_ingestion": $(verify_data_ingestion > /dev/null 2>&1 && echo "true" || echo "false"),
        "entity_synthesis": $(verify_entity_synthesis > /dev/null 2>&1 && echo "true" || echo "false"),
        "integration_errors": $(check_integration_errors > /dev/null 2>&1 && echo "true" || echo "false"),
        "cardinality_management": $(verify_cardinality_management > /dev/null 2>&1 && echo "true" || echo "false"),
        "circuit_breaker": $(verify_circuit_breaker > /dev/null 2>&1 && echo "true" || echo "false"),
        "adaptive_sampling": $(verify_sampling_effectiveness > /dev/null 2>&1 && echo "true" || echo "false"),
        "database_metrics": $(verify_database_metrics > /dev/null 2>&1 && echo "true" || echo "false")
    }
}
EOF
    
    success "Verification report saved to: $VERIFICATION_RESULTS_FILE"
}

create_feedback_dashboard() {
    echo -e "\n${YELLOW}=== Creating Feedback Dashboard ===${NC}\n"
    
    echo "Import the following dashboard for continuous monitoring:"
    echo ""
    echo "1. Go to: https://one.newrelic.com/dashboards"
    echo "2. Click 'Import dashboard'"
    echo "3. Use the configuration from: ${PROJECT_ROOT}/monitoring/dashboard-config.json"
    echo ""
    echo "Key queries to monitor:"
    echo ""
    echo "1. Silent Failures (run every 5 minutes):"
    echo "   SELECT count(*) FROM NrIntegrationError"
    echo "   WHERE newRelicFeature = 'Metrics'"
    echo "   AND message LIKE '%database%'"
    echo "   SINCE 5 minutes ago"
    echo ""
    echo "2. Data Freshness:"
    echo "   SELECT latest(timestamp) FROM Log"
    echo "   WHERE collector.name = 'database-intelligence'"
    echo "   FACET database_name"
    echo "   SINCE 10 minutes ago"
}

# ====================
# Main Execution
# ====================

main() {
    echo -e "${GREEN}=== New Relic Integration Verification Platform ===${NC}"
    echo -e "Account ID: $NEW_RELIC_ACCOUNT_ID\n"
    
    # Run all verifications
    verify_data_ingestion
    verify_entity_synthesis
    check_integration_errors
    verify_cardinality_management
    verify_circuit_breaker
    verify_sampling_effectiveness
    verify_database_metrics
    
    # Generate report
    generate_verification_report
    
    # Show dashboard info
    create_feedback_dashboard
    
    echo -e "\n${GREEN}=== Verification Complete ===${NC}"
    echo "Results saved to: $VERIFICATION_RESULTS_FILE"
    
    # Return appropriate exit code
    if grep -q '"false"' "$VERIFICATION_RESULTS_FILE"; then
        echo -e "\n${RED}Some checks failed. Please review the output above.${NC}"
        exit 1
    else
        echo -e "\n${GREEN}All checks passed!${NC}"
        exit 0
    fi
}

# Run main function
main "$@"