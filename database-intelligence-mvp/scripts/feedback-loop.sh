#!/bin/bash
# Automated Feedback Loop for Database Intelligence MVP
# Monitors integration health and provides actionable feedback

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Load environment
load_env_file "${PROJECT_ROOT}/.env"

# Configuration
FEEDBACK_LOG="${PROJECT_ROOT}/logs/feedback-$(date +%Y%m%d).log"
HEALTH_REPORT="${PROJECT_ROOT}/monitoring/health-report.json"
SLACK_WEBHOOK="${SLACK_WEBHOOK:-}"  # Optional Slack integration

# Ensure log directory exists
mkdir -p "$(dirname "$FEEDBACK_LOG")"

# ====================
# Health Check Functions
# ====================

log_feedback() {
    local level="$1"
    local message="$2"
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
    
    echo -e "${timestamp} [${level}] ${message}" >> "$FEEDBACK_LOG"
    
    case "$level" in
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${message}"
            ;;
        WARNING)
            echo -e "${YELLOW}[WARNING]${NC} ${message}"
            ;;
        SUCCESS)
            echo -e "${GREEN}[SUCCESS]${NC} ${message}"
            ;;
        INFO)
            echo -e "${BLUE}[INFO]${NC} ${message}"
            ;;
    esac
    
    # Send to Slack if configured
    if [[ -n "$SLACK_WEBHOOK" ]]; then
        send_slack_notification "$level" "$message"
    fi
}

send_slack_notification() {
    local level="$1"
    local message="$2"
    
    local color="good"
    case "$level" in
        ERROR) color="danger" ;;
        WARNING) color="warning" ;;
    esac
    
    curl -s -X POST "$SLACK_WEBHOOK" \
        -H "Content-Type: application/json" \
        -d "{
            \"attachments\": [{
                \"color\": \"$color\",
                \"title\": \"Database Intelligence MVP - $level\",
                \"text\": \"$message\",
                \"footer\": \"Feedback Loop System\",
                \"ts\": $(date +%s)
            }]
        }" > /dev/null 2>&1 || true
}

check_collector_health() {
    echo -e "\n${PURPLE}=== Checking Collector Health ===${NC}\n"
    
    # Check if collector is running
    if docker ps | grep -q "db-intel-primary"; then
        log_feedback "SUCCESS" "Collector container is running"
        
        # Check container health status
        local health=$(docker inspect db-intel-primary --format='{{.State.Health.Status}}' 2>/dev/null || echo "unknown")
        
        case "$health" in
            healthy)
                log_feedback "SUCCESS" "Container health check passing"
                ;;
            unhealthy)
                log_feedback "WARNING" "Container marked as unhealthy - this may be due to missing curl in container"
                ;;
            *)
                log_feedback "INFO" "No health check configured"
                ;;
        esac
        
        # Check metrics endpoint
        if curl -sf http://localhost:8888/metrics > /dev/null; then
            log_feedback "SUCCESS" "Metrics endpoint responding"
            
            # Get key metrics
            local metrics=$(curl -s http://localhost:8888/metrics)
            local received=$(echo "$metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
            local exported=$(echo "$metrics" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
            local failed=$(echo "$metrics" | grep "otelcol_exporter_send_failed_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
            
            log_feedback "INFO" "Collector metrics - Received: $received, Exported: $exported, Failed: $failed"
            
            if [[ "$failed" != "0" && "$failed" != "" ]]; then
                log_feedback "WARNING" "Detected $failed failed exports - check configuration"
            fi
        else
            log_feedback "ERROR" "Metrics endpoint not responding"
        fi
    else
        log_feedback "ERROR" "Collector container not running - run: docker-compose up -d db-intelligence-primary"
        return 1
    fi
}

check_data_flow() {
    echo -e "\n${PURPLE}=== Checking Data Flow ===${NC}\n"
    
    # Query New Relic for recent data
    local query="SELECT count(*) as count FROM Log WHERE collector.name = 'database-intelligence' SINCE 5 minutes ago"
    
    local result=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -d "{\"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"}" | \
        jq -r '.data.actor.account.nrql.results[0].count' 2>/dev/null || echo "0")
    
    if [[ "$result" -gt 0 ]]; then
        log_feedback "SUCCESS" "Data flowing to New Relic: $result records in last 5 minutes"
    else
        log_feedback "ERROR" "No data received in New Relic in last 5 minutes"
        
        # Provide troubleshooting steps
        echo -e "\n${YELLOW}Troubleshooting Steps:${NC}"
        echo "1. Check collector logs: docker logs db-intel-primary --tail 50"
        echo "2. Verify environment variables: grep NEW_RELIC .env"
        echo "3. Test database connectivity: docker exec db-intel-primary /otelcol-contrib --config /etc/otel/config.yaml --dry-run"
        echo "4. Check for NrIntegrationError events in New Relic"
    fi
}

check_integration_errors() {
    echo -e "\n${PURPLE}=== Checking Integration Errors ===${NC}\n"
    
    # Check for NrIntegrationError events
    local query="SELECT count(*) as count, latest(message) as message FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' AND (message LIKE '%database%' OR message LIKE '%otel%') SINCE 30 minutes ago"
    
    local result=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -d "{\"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"}")
    
    local error_count=$(echo "$result" | jq -r '.data.actor.account.nrql.results[0].count' 2>/dev/null || echo "0")
    local latest_error=$(echo "$result" | jq -r '.data.actor.account.nrql.results[0].message' 2>/dev/null || echo "")
    
    if [[ "$error_count" -eq 0 ]]; then
        log_feedback "SUCCESS" "No integration errors detected"
    else
        log_feedback "ERROR" "Found $error_count integration errors in last 30 minutes"
        if [[ -n "$latest_error" && "$latest_error" != "null" ]]; then
            log_feedback "ERROR" "Latest error: $latest_error"
            
            # Provide specific remediation based on error type
            if [[ "$latest_error" == *"cardinality"* ]]; then
                echo -e "\n${YELLOW}Cardinality Issue Detected:${NC}"
                echo "1. Review query normalization in collector config"
                echo "2. Increase sampling rate in adaptivesampler processor"
                echo "3. Check for unbounded attributes in queries"
            elif [[ "$latest_error" == *"api-key"* ]]; then
                echo -e "\n${YELLOW}API Key Issue:${NC}"
                echo "1. Verify NEW_RELIC_LICENSE_KEY in .env"
                echo "2. Check key permissions in New Relic"
            elif [[ "$latest_error" == *"rate limit"* ]]; then
                echo -e "\n${YELLOW}Rate Limit Issue:${NC}"
                echo "1. Reduce batch size in collector config"
                echo "2. Increase batch timeout"
                echo "3. Enable sampling"
            fi
        fi
    fi
}

generate_health_report() {
    echo -e "\n${PURPLE}=== Generating Health Report ===${NC}\n"
    
    # Collect all health metrics
    local timestamp=$(date -u +"%Y-%m-%d %H:%M:%S UTC")
    local collector_status="unknown"
    local data_flow_status="unknown"
    local error_status="unknown"
    
    # Check each component
    docker ps | grep -q "db-intel-primary" && collector_status="running" || collector_status="stopped"
    
    # Generate JSON report
    cat > "$HEALTH_REPORT" <<EOF
{
    "timestamp": "$timestamp",
    "account_id": "$NEW_RELIC_ACCOUNT_ID",
    "collector": {
        "status": "$collector_status",
        "endpoint": "http://localhost:8888/metrics"
    },
    "data_flow": {
        "status": "$data_flow_status",
        "records_last_5m": $result
    },
    "errors": {
        "integration_errors_30m": $error_count,
        "latest_error": "$latest_error"
    },
    "recommendations": []
}
EOF
    
    log_feedback "SUCCESS" "Health report generated: $HEALTH_REPORT"
}

provide_recommendations() {
    echo -e "\n${PURPLE}=== Recommendations ===${NC}\n"
    
    local recommendations=()
    
    # Check if we need to set up alerts
    local alerts_query="SELECT count(*) FROM NrAlertCondition WHERE name LIKE '%database%' OR name LIKE '%integration%'"
    local alerts_result=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -d "{\"query\": \"{ actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$alerts_query\\\") { results } } } }\"}" | \
        jq -r '.data.actor.account.nrql.results[0].count' 2>/dev/null || echo "0")
    
    if [[ "$alerts_result" -eq 0 ]]; then
        recommendations+=("Set up alerts for integration errors and data freshness")
    fi
    
    # Check if dashboard is imported
    recommendations+=("Import monitoring dashboard from monitoring/dashboard-config.json")
    
    # Check sampling configuration
    recommendations+=("Review adaptive sampling configuration if seeing high cardinality")
    
    # Display recommendations
    if [[ ${#recommendations[@]} -gt 0 ]]; then
        echo -e "${YELLOW}Recommendations:${NC}"
        for i in "${!recommendations[@]}"; do
            echo "$((i+1)). ${recommendations[$i]}"
        done
    else
        echo -e "${GREEN}No immediate recommendations - system appears healthy${NC}"
    fi
}

run_feedback_loop() {
    echo -e "${GREEN}=== Database Intelligence MVP Feedback Loop ===${NC}"
    echo -e "Time: $(date -u +"%Y-%m-%d %H:%M:%S UTC")\n"
    
    # Run all checks
    check_collector_health
    check_data_flow
    check_integration_errors
    
    # Generate report
    generate_health_report
    
    # Provide recommendations
    provide_recommendations
    
    echo -e "\n${GREEN}=== Feedback Loop Complete ===${NC}"
    echo "Logs saved to: $FEEDBACK_LOG"
    echo "Health report: $HEALTH_REPORT"
}

# ====================
# Continuous Mode
# ====================

run_continuous() {
    echo -e "${GREEN}Starting continuous feedback loop (Ctrl+C to stop)${NC}\n"
    
    while true; do
        run_feedback_loop
        echo -e "\n${BLUE}Next check in 5 minutes...${NC}\n"
        sleep 300
    done
}

# ====================
# Main Execution
# ====================

main() {
    local mode="${1:-once}"
    
    case "$mode" in
        once)
            run_feedback_loop
            ;;
        continuous)
            run_continuous
            ;;
        *)
            echo "Usage: $0 [once|continuous]"
            echo "  once       - Run feedback loop once (default)"
            echo "  continuous - Run continuously every 5 minutes"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"