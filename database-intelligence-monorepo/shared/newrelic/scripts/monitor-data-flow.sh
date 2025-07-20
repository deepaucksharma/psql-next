#!/bin/bash

# Real-time Data Flow Monitoring Script
# Continuously monitors data flow and dashboard health

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NERDGRAPH_URL="https://api.newrelic.com/graphql"
MONITORING_INTERVAL=${MONITORING_INTERVAL:-30}
ALERT_THRESHOLD_MINUTES=${ALERT_THRESHOLD_MINUTES:-5}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Monitoring state
declare -A LAST_DATA_TIMESTAMP
declare -A METRIC_STATUS
declare -A ALERT_SENT

# Logging with timestamp
log() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] ${1}"
}

# Execute NerdGraph query with error handling
execute_nerdgraph() {
    local query=$1
    local variables=${2:-"{}"}
    local retries=3
    local retry_delay=5
    
    for ((i=1; i<=retries; i++)); do
        local response=$(curl -s -X POST "$NERDGRAPH_URL" \
            -H "Content-Type: application/json" \
            -H "API-Key: $NEW_RELIC_API_KEY" \
            -d "{\"query\": \"$query\", \"variables\": $variables}" 2>/dev/null)
        
        if [[ -n "$response" ]] && ! echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
            echo "$response"
            return 0
        fi
        
        if [[ $i -lt $retries ]]; then
            log "${YELLOW}Retry $i/$retries failed, waiting ${retry_delay}s...${NC}"
            sleep $retry_delay
        fi
    done
    
    log "${RED}Failed to execute NerdGraph query after $retries attempts${NC}"
    return 1
}

# Check data freshness for a specific metric
check_metric_freshness() {
    local metric_name="$1"
    local max_age_minutes="$2"
    
    local nrql="SELECT latest(timestamp) as last_seen FROM Metric WHERE metricName = '$metric_name' AND instrumentation.provider = 'opentelemetry' SINCE $max_age_minutes minutes ago"
    
    local query="
    query checkMetricFreshness(\$accountId: Int!, \$nrql: Nrql!) {
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
    
    if response=$(execute_nerdgraph "$query" "$variables"); then
        local result_count=$(echo "$response" | jq '.data.actor.account.nrql.results | length')
        local last_timestamp=$(echo "$response" | jq -r '.data.actor.account.nrql.results[0].last_seen // "null"')
        
        if [[ "$result_count" -gt 0 && "$last_timestamp" != "null" ]]; then
            LAST_DATA_TIMESTAMP["$metric_name"]="$last_timestamp"
            METRIC_STATUS["$metric_name"]="healthy"
            return 0
        else
            METRIC_STATUS["$metric_name"]="stale"
            return 1
        fi
    else
        METRIC_STATUS["$metric_name"]="error"
        return 1
    fi
}

# Check entity health
check_entity_health() {
    local entity_type="$1"
    
    local query="
    query checkEntityHealth(\$accountId: Int!) {
        actor {
            entitySearch(query: \"type = '$entity_type' AND tags.accountId = '$NEW_RELIC_ACCOUNT_ID'\") {
                results {
                    entities {
                        guid
                        name
                        entityType
                        reporting
                        alertSeverity
                    }
                }
            }
        }
    }"
    
    local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID}"
    
    if response=$(execute_nerdgraph "$query" "$variables"); then
        local entity_count=$(echo "$response" | jq '.data.actor.entitySearch.results.entities | length')
        local reporting_count=$(echo "$response" | jq '[.data.actor.entitySearch.results.entities[] | select(.reporting == true)] | length')
        
        log "${BLUE}Entity Type: $entity_type - Total: $entity_count, Reporting: $reporting_count${NC}"
        
        # Check for entities with critical alerts
        local critical_entities=$(echo "$response" | jq -r '.data.actor.entitySearch.results.entities[] | select(.alertSeverity == "CRITICAL") | .name' 2>/dev/null || echo "")
        
        if [[ -n "$critical_entities" ]]; then
            log "${RED}Critical alerts for $entity_type entities:${NC}"
            echo "$critical_entities" | while read -r entity_name; do
                log "${RED}  - $entity_name${NC}"
            done
        fi
        
        return 0
    else
        log "${RED}Failed to check health for entity type: $entity_type${NC}"
        return 1
    fi
}

# Test dashboard widget performance
test_widget_performance() {
    local widget_nrql="$1"
    local widget_name="$2"
    
    local start_time=$(date +%s%3N)
    
    local query="
    query testWidgetPerformance(\$accountId: Int!, \$nrql: Nrql!) {
        actor {
            account(id: \$accountId) {
                nrql(query: \$nrql) {
                    results
                    metadata {
                        timeWindow {
                            begin
                            end
                        }
                        messages
                    }
                }
            }
        }
    }"
    
    local variables="{\"accountId\": $NEW_RELIC_ACCOUNT_ID, \"nrql\": \"$widget_nrql\"}"
    
    if response=$(execute_nerdgraph "$query" "$variables"); then
        local end_time=$(date +%s%3N)
        local duration=$((end_time - start_time))
        local result_count=$(echo "$response" | jq '.data.actor.account.nrql.results | length')
        local messages=$(echo "$response" | jq -r '.data.actor.account.nrql.metadata.messages[]?' 2>/dev/null || echo "")
        
        if [[ $duration -gt 5000 ]]; then
            log "${YELLOW}Slow widget: $widget_name (${duration}ms, $result_count results)${NC}"
        else
            log "${GREEN}✓ Widget OK: $widget_name (${duration}ms, $result_count results)${NC}"
        fi
        
        if [[ -n "$messages" ]]; then
            log "${YELLOW}Widget messages: $messages${NC}"
        fi
        
        return 0
    else
        log "${RED}✗ Widget failed: $widget_name${NC}"
        return 1
    fi
}

# Send alert (placeholder for integration with notification systems)
send_alert() {
    local alert_type="$1"
    local message="$2"
    local metric_name="${3:-unknown}"
    
    # Prevent duplicate alerts
    local alert_key="${alert_type}_${metric_name}"
    if [[ "${ALERT_SENT[$alert_key]:-false}" == "true" ]]; then
        return 0
    fi
    
    log "${RED}ALERT [$alert_type]: $message${NC}"
    
    # Mark alert as sent
    ALERT_SENT["$alert_key"]="true"
    
    # Integration points for external alerting
    if [[ -n "${WEBHOOK_URL:-}" ]]; then
        curl -s -X POST "$WEBHOOK_URL" \
            -H "Content-Type: application/json" \
            -d "{\"alert_type\": \"$alert_type\", \"message\": \"$message\", \"metric\": \"$metric_name\", \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" \
            > /dev/null || true
    fi
    
    if [[ -n "${SLACK_WEBHOOK_URL:-}" ]]; then
        curl -s -X POST "$SLACK_WEBHOOK_URL" \
            -H "Content-Type: application/json" \
            -d "{\"text\": \"🚨 Database Intelligence Alert: $message\"}" \
            > /dev/null || true
    fi
}

# Monitor critical metrics
monitor_critical_metrics() {
    log "${BLUE}Monitoring critical metrics...${NC}"
    
    local critical_metrics=(
        "mysql.intelligence.comprehensive"
        "mysql.connections.current"
        "mysql.threads.running"
        "mysql.query.exec_count"
        "system.cpu.utilization"
        "system.memory.usage"
    )
    
    for metric in "${critical_metrics[@]}"; do
        if check_metric_freshness "$metric" "$ALERT_THRESHOLD_MINUTES"; then
            log "${GREEN}✓ $metric: ${METRIC_STATUS[$metric]}${NC}"
        else
            log "${RED}✗ $metric: ${METRIC_STATUS[$metric]}${NC}"
            send_alert "STALE_DATA" "Metric $metric has not reported data in the last $ALERT_THRESHOLD_MINUTES minutes" "$metric"
        fi
    done
}

# Monitor dashboard widgets
monitor_dashboard_widgets() {
    log "${BLUE}Monitoring dashboard widgets...${NC}"
    
    # Key dashboard widgets to monitor
    local widgets=(
        "SELECT count(*) FROM Metric WHERE metricName LIKE 'mysql.%' SINCE 5 minutes ago|MySQL Metrics Count"
        "SELECT average(mysql.connections.current) FROM Metric SINCE 5 minutes ago|Average Connections"
        "SELECT latest(mysql.intelligence.comprehensive) FROM Metric WHERE performance_issue != 'OK' SINCE 5 minutes ago|Intelligence Score"
        "SELECT count(*) FROM Metric WHERE metricName LIKE 'system.%' SINCE 5 minutes ago|System Metrics Count"
    )
    
    for widget in "${widgets[@]}"; do
        IFS='|' read -r nrql name <<< "$widget"
        test_widget_performance "$nrql" "$name"
    done
}

# Monitor entity health
monitor_entity_health() {
    log "${BLUE}Monitoring entity health...${NC}"
    
    local entity_types=(
        "MYSQL_INSTANCE"
        "HOST"
        "SYNTHETIC_MONITOR"
        "APPLICATION"
    )
    
    for entity_type in "${entity_types[@]}"; do
        check_entity_health "$entity_type"
    done
}

# Check New Relic API status
check_api_status() {
    local query='
    query checkAPIStatus {
        actor {
            user {
                name
                email
            }
        }
    }'
    
    if response=$(execute_nerdgraph "$query"); then
        local user_name=$(echo "$response" | jq -r '.data.actor.user.name')
        log "${GREEN}✓ New Relic API accessible (User: $user_name)${NC}"
        return 0
    else
        log "${RED}✗ New Relic API not accessible${NC}"
        send_alert "API_ERROR" "Unable to access New Relic API"
        return 1
    fi
}

# Generate monitoring summary
generate_summary() {
    local healthy_metrics=0
    local total_metrics=0
    
    for metric in "${!METRIC_STATUS[@]}"; do
        total_metrics=$((total_metrics + 1))
        if [[ "${METRIC_STATUS[$metric]}" == "healthy" ]]; then
            healthy_metrics=$((healthy_metrics + 1))
        fi
    done
    
    local health_percentage=0
    if [[ $total_metrics -gt 0 ]]; then
        health_percentage=$((healthy_metrics * 100 / total_metrics))
    fi
    
    log "${BLUE}=== Monitoring Summary ===${NC}"
    log "${BLUE}Healthy Metrics: $healthy_metrics/$total_metrics ($health_percentage%)${NC}"
    log "${BLUE}Monitoring Interval: ${MONITORING_INTERVAL}s${NC}"
    log "${BLUE}Alert Threshold: ${ALERT_THRESHOLD_MINUTES} minutes${NC}"
    
    if [[ $health_percentage -lt 80 ]]; then
        send_alert "HEALTH_DEGRADED" "Overall system health at $health_percentage% ($healthy_metrics/$total_metrics metrics healthy)"
    fi
}

# Main monitoring loop
main() {
    log "${BLUE}=== Database Intelligence Data Flow Monitor Started ===${NC}"
    log "${BLUE}Monitoring Interval: ${MONITORING_INTERVAL}s${NC}"
    log "${BLUE}Alert Threshold: ${ALERT_THRESHOLD_MINUTES} minutes${NC}"
    
    # Check environment
    if [[ -z "${NEW_RELIC_API_KEY:-}" ]]; then
        log "${RED}ERROR: NEW_RELIC_API_KEY environment variable is required${NC}"
        exit 1
    fi
    
    if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
        log "${RED}ERROR: NEW_RELIC_ACCOUNT_ID environment variable is required${NC}"
        exit 1
    fi
    
    # Initial API check
    if ! check_api_status; then
        log "${RED}Initial API check failed, exiting${NC}"
        exit 1
    fi
    
    # Monitoring loop
    while true; do
        log "${BLUE}=== Monitoring Cycle Started ===${NC}"
        
        # Reset alert flags for this cycle
        for key in "${!ALERT_SENT[@]}"; do
            unset ALERT_SENT["$key"]
        done
        
        # Run monitoring checks
        check_api_status || true
        monitor_critical_metrics
        monitor_entity_health
        monitor_dashboard_widgets
        generate_summary
        
        log "${BLUE}=== Monitoring Cycle Completed ===${NC}"
        log "${BLUE}Next check in ${MONITORING_INTERVAL} seconds...${NC}"
        
        sleep "$MONITORING_INTERVAL"
    done
}

# Handle interruption gracefully
trap 'log "${YELLOW}Monitoring stopped by user${NC}"; exit 0' INT TERM

# Script execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi