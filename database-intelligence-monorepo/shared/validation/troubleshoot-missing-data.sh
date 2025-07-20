#!/usr/bin/env bash

# Comprehensive Troubleshooting Script for Database Intelligence Monorepo
# Usage: ./troubleshoot-missing-data.sh [module_name] [--verbose]
# Example: ./troubleshoot-missing-data.sh core-metrics --verbose

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
VERBOSE=false
MODULE=""
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
LOG_FILE="troubleshoot_${TIMESTAMP}.log"

# All available modules
ALL_MODULES=("core-metrics" "sql-intelligence" "wait-profiler" "anomaly-detector" 
             "business-impact" "replication-monitor" "performance-advisor" 
             "resource-monitor" "alert-manager" "canary-tester" "cross-signal-correlator")

# Module port mappings (bash 3.2 compatible)
get_module_port() {
    case "$1" in
        "core-metrics") echo "8081" ;;
        "sql-intelligence") echo "8082" ;;
        "wait-profiler") echo "8083" ;;
        "anomaly-detector") echo "8084" ;;
        "business-impact") echo "8085" ;;
        "replication-monitor") echo "8086" ;;
        "performance-advisor") echo "8087" ;;
        "resource-monitor") echo "8088" ;;
        "alert-manager") echo "8089" ;;
        "canary-tester") echo "8090" ;;
        "cross-signal-correlator") echo "8099" ;;
        *) echo "" ;;
    esac
}

# Health check ports
get_health_port() {
    case "$1" in
        "core-metrics") echo "13133" ;;
        "sql-intelligence") echo "13133" ;;
        "wait-profiler") echo "13133" ;;
        "anomaly-detector") echo "13133" ;;
        "business-impact") echo "13133" ;;
        "replication-monitor") echo "13133" ;;
        "performance-advisor") echo "13133" ;;
        "resource-monitor") echo "13135" ;;
        "alert-manager") echo "13134" ;;
        "canary-tester") echo "13133" ;;
        "cross-signal-correlator") echo "13137" ;;
        *) echo "" ;;
    esac
}

# Expected metrics for each module
get_module_metrics() {
    case "$1" in
        "core-metrics") echo "mysql_connections_current,mysql_threads_running,mysql_slow_queries_total" ;;
        "sql-intelligence") echo "mysql.query.exec_total,mysql.query.latency_ms,mysql.query.rows_examined_total" ;;
        "wait-profiler") echo "mysql.wait.count,mysql.wait.time_ms,mysql.wait.mutex.count" ;;
        "anomaly-detector") echo "anomaly_score_cpu,anomaly_score_memory,anomaly_detected" ;;
        "business-impact") echo "business_impact_score,revenue_impact_hourly,sla_impact" ;;
        "replication-monitor") echo "mysql_replica_lag,mysql_replication_running,mysql_gtid_executed" ;;
        "performance-advisor") echo "db.performance.recommendation.missing_index,db.performance.recommendation.slow_query" ;;
        "resource-monitor") echo "system.cpu.utilization,system.memory.usage,system.disk.io.time" ;;
        "alert-manager") echo "alert.processed,alert.severity,alert.type" ;;
        "canary-tester") echo "canary.test.success_rate,canary.test.response_time" ;;
        "cross-signal-correlator") echo "correlation.strength,cross_signal.match_count" ;;
        *) echo "" ;;
    esac
}

# Functions
log() {
    local level=$1
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        "INFO")  echo -e "${BLUE}[INFO]${NC} $message" | tee -a "$LOG_FILE" ;;
        "WARN")  echo -e "${YELLOW}[WARN]${NC} $message" | tee -a "$LOG_FILE" ;;
        "ERROR") echo -e "${RED}[ERROR]${NC} $message" | tee -a "$LOG_FILE" ;;
        "SUCCESS") echo -e "${GREEN}[SUCCESS]${NC} $message" | tee -a "$LOG_FILE" ;;
        *) echo "$message" | tee -a "$LOG_FILE" ;;
    esac
}

print_header() {
    echo -e "\n${BLUE}============================================================${NC}"
    echo -e "${BLUE} $1 ${NC}"
    echo -e "${BLUE}============================================================${NC}\n"
}

check_prerequisites() {
    log "INFO" "Checking prerequisites..."
    
    # Check if running in correct directory
    if [[ ! -f "Makefile" ]] || [[ ! -d "modules" ]]; then
        log "ERROR" "Please run this script from the database-intelligence-monorepo root directory"
        exit 1
    fi
    
    # Check required tools
    local tools=("curl" "jq" "docker" "docker-compose")
    for tool in "${tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log "ERROR" "Required tool '$tool' is not installed"
            exit 1
        fi
    done
    
    log "SUCCESS" "Prerequisites check passed"
}

check_module_health() {
    local module=$1
    local port=$(get_module_port "$module")
    local health_port=$(get_health_port "$module")
    
    print_header "HEALTH CHECK: $module"
    
    # Check if container is running
    local container_name="${module}-otel-collector"
    if docker ps --format "table {{.Names}}" | grep -q "$container_name"; then
        log "SUCCESS" "Container $container_name is running"
    else
        log "ERROR" "Container $container_name is not running"
        log "INFO" "Checking if container exists..."
        if docker ps -a --format "table {{.Names}}" | grep -q "$container_name"; then
            log "WARN" "Container exists but is stopped. Checking logs..."
            docker logs --tail 20 "$container_name" 2>&1 | tee -a "$LOG_FILE"
        else
            log "ERROR" "Container does not exist. Module may not be deployed."
        fi
        return 1
    fi
    
    # Check health endpoint
    log "INFO" "Checking health endpoint at localhost:$health_port"
    if curl -sf "http://localhost:$health_port/" > /dev/null 2>&1; then
        log "SUCCESS" "Health endpoint is responding"
    else
        log "ERROR" "Health endpoint is not responding"
        log "INFO" "Checking if port is open..."
        if nc -z localhost "$health_port" 2>/dev/null; then
            log "WARN" "Port is open but health check failed"
        else
            log "ERROR" "Port $health_port is not open"
        fi
        return 1
    fi
    
    # Check metrics endpoint
    log "INFO" "Checking metrics endpoint at localhost:$port"
    if curl -sf "http://localhost:$port/metrics" > /dev/null 2>&1; then
        log "SUCCESS" "Metrics endpoint is responding"
        
        # Count available metrics
        local metric_count=$(curl -s "http://localhost:$port/metrics" | grep -c "^[^#].*{.*}" || echo "0")
        log "INFO" "Found $metric_count metric series"
        
        if [[ $VERBOSE == "true" ]]; then
            log "INFO" "Sample metrics:"
            curl -s "http://localhost:$port/metrics" | grep -E "^[^#].*{.*}" | head -5
        fi
    else
        log "ERROR" "Metrics endpoint is not responding"
        return 1
    fi
    
    return 0
}

check_module_configuration() {
    local module=$1
    
    print_header "CONFIGURATION CHECK: $module"
    
    local config_file="modules/$module/config/collector.yaml"
    if [[ ! -f "$config_file" ]]; then
        log "ERROR" "Configuration file not found: $config_file"
        return 1
    fi
    
    log "SUCCESS" "Configuration file exists: $config_file"
    
    # Validate YAML syntax
    if python3 -c "import yaml; yaml.safe_load(open('$config_file', 'r'))" 2>/dev/null; then
        log "SUCCESS" "YAML syntax is valid"
    else
        log "ERROR" "YAML syntax is invalid"
        python3 -c "import yaml; yaml.safe_load(open('$config_file', 'r'))" 2>&1 | tee -a "$LOG_FILE"
        return 1
    fi
    
    # Check required sections
    local sections=("receivers" "processors" "exporters" "service")
    for section in "${sections[@]}"; do
        if grep -q "^$section:" "$config_file"; then
            log "SUCCESS" "Required section '$section' found"
        else
            log "ERROR" "Required section '$section' missing"
        fi
    done
    
    # Check New Relic exporter configuration
    if grep -q "otlphttp.*newrelic" "$config_file"; then
        log "SUCCESS" "New Relic exporter configuration found"
        
        # Check environment variables
        if grep -q "NEW_RELIC_LICENSE_KEY" "$config_file"; then
            if [[ -n "$NEW_RELIC_LICENSE_KEY" ]]; then
                log "SUCCESS" "NEW_RELIC_LICENSE_KEY environment variable is set"
            else
                log "ERROR" "NEW_RELIC_LICENSE_KEY environment variable is not set"
            fi
        fi
        
        if grep -q "NEW_RELIC_OTLP_ENDPOINT" "$config_file"; then
            if [[ -n "$NEW_RELIC_OTLP_ENDPOINT" ]]; then
                log "SUCCESS" "NEW_RELIC_OTLP_ENDPOINT environment variable is set"
            else
                log "ERROR" "NEW_RELIC_OTLP_ENDPOINT environment variable is not set"
            fi
        fi
    else
        log "WARN" "New Relic exporter configuration not found"
    fi
    
    return 0
}

check_mysql_connectivity() {
    local module=$1
    
    print_header "MYSQL CONNECTIVITY CHECK: $module"
    
    # Check if module uses MySQL receiver
    local config_file="modules/$module/config/collector.yaml"
    if grep -q "mysql:" "$config_file" || grep -q "sqlquery:" "$config_file"; then
        log "INFO" "Module uses MySQL connectivity"
        
        # Extract MySQL endpoint from config or environment
        local mysql_endpoint=${MYSQL_ENDPOINT:-"mysql-test:3306"}
        local mysql_user=${MYSQL_USER:-"root"}
        local mysql_password=${MYSQL_PASSWORD:-"test"}
        
        log "INFO" "Testing MySQL connectivity to $mysql_endpoint"
        
        # Test connection using container if available
        local container_name="${module}-otel-collector"
        if docker ps --format "table {{.Names}}" | grep -q "$container_name"; then
            if docker exec "$container_name" mysqladmin ping -h"${mysql_endpoint%:*}" -P"${mysql_endpoint#*:}" -u"$mysql_user" -p"$mysql_password" 2>/dev/null; then
                log "SUCCESS" "MySQL connectivity test passed"
            else
                log "ERROR" "MySQL connectivity test failed"
                log "INFO" "Checking MySQL container status..."
                if docker ps --format "table {{.Names}}" | grep -q "mysql"; then
                    log "INFO" "MySQL container is running"
                else
                    log "ERROR" "MySQL container is not running"
                fi
            fi
        else
            log "WARN" "Cannot test MySQL connectivity - collector container not running"
        fi
    else
        log "INFO" "Module does not use MySQL connectivity"
    fi
}

check_network_connectivity() {
    local module=$1
    
    print_header "NETWORK CONNECTIVITY CHECK: $module"
    
    # Check federation endpoints for modules that use them
    local config_file="modules/$module/config/collector.yaml"
    if grep -q "prometheus:" "$config_file" && grep -q "targets.*ENDPOINT" "$config_file"; then
        log "INFO" "Module uses federation - checking endpoints"
        
        # Extract federation targets
        local endpoints=$(grep -A 5 "targets:" "$config_file" | grep "env:" | sed 's/.*env:\([^}]*\).*/\1/' | sort | uniq)
        
        for endpoint_var in $endpoints; do
            local endpoint_value=$(eval echo \$$endpoint_var)
            if [[ -n "$endpoint_value" ]]; then
                log "INFO" "Testing federation endpoint: $endpoint_value"
                if curl -sf "http://$endpoint_value/metrics" > /dev/null 2>&1; then
                    log "SUCCESS" "Federation endpoint $endpoint_value is responding"
                else
                    log "ERROR" "Federation endpoint $endpoint_value is not responding"
                fi
            else
                log "WARN" "Environment variable $endpoint_var is not set"
            fi
        done
    else
        log "INFO" "Module does not use federation"
    fi
}

check_expected_metrics() {
    local module=$1
    local port=$(get_module_port "$module")
    
    print_header "EXPECTED METRICS CHECK: $module"
    
    local expected_metrics=$(get_module_metrics "$module")
    if [[ -z "$expected_metrics" ]]; then
        log "WARN" "No expected metrics defined for $module"
        return 0
    fi
    
    # Get current metrics
    local metrics_output=$(curl -s "http://localhost:$port/metrics" 2>/dev/null || echo "")
    if [[ -z "$metrics_output" ]]; then
        log "ERROR" "Could not retrieve metrics from $module"
        return 1
    fi
    
    # Check each expected metric
    IFS=',' read -ra METRICS <<< "$expected_metrics"
    local found_count=0
    local total_count=${#METRICS[@]}
    
    for metric in "${METRICS[@]}"; do
        if echo "$metrics_output" | grep -q "^$metric"; then
            log "SUCCESS" "Expected metric found: $metric"
            ((found_count++))
        else
            log "ERROR" "Expected metric missing: $metric"
        fi
    done
    
    log "INFO" "Found $found_count/$total_count expected metrics"
    
    if [[ $found_count -eq $total_count ]]; then
        log "SUCCESS" "All expected metrics are present"
        return 0
    else
        log "ERROR" "Some expected metrics are missing"
        return 1
    fi
}

check_new_relic_export() {
    local module=$1
    
    print_header "NEW RELIC EXPORT CHECK: $module"
    
    # Check if module has New Relic exporter
    local config_file="modules/$module/config/collector.yaml"
    if grep -q "otlphttp.*newrelic" "$config_file"; then
        log "INFO" "Module has New Relic exporter configured"
        
        # Check container logs for export success/failure
        local container_name="${module}-otel-collector"
        if docker ps --format "table {{.Names}}" | grep -q "$container_name"; then
            log "INFO" "Checking recent container logs for export activity..."
            
            local log_output=$(docker logs --tail 50 "$container_name" 2>&1)
            
            # Look for successful exports
            if echo "$log_output" | grep -q "successfully sent"; then
                log "SUCCESS" "Found successful export logs"
            else
                log "WARN" "No successful export logs found in recent history"
            fi
            
            # Look for export errors
            if echo "$log_output" | grep -q -E "(export.*error|failed.*export|otlphttp.*error)"; then
                log "ERROR" "Found export error logs:"
                echo "$log_output" | grep -E "(export.*error|failed.*export|otlphttp.*error)" | tail -5 | tee -a "$LOG_FILE"
            fi
            
            # Look for authentication errors
            if echo "$log_output" | grep -q -E "(401|403|unauthorized|forbidden)"; then
                log "ERROR" "Found authentication errors - check NEW_RELIC_LICENSE_KEY"
            fi
            
            # Look for network errors
            if echo "$log_output" | grep -q -E "(connection.*refused|timeout|dns.*error)"; then
                log "ERROR" "Found network connectivity errors"
            fi
        else
            log "ERROR" "Cannot check export logs - container not running"
        fi
    else
        log "INFO" "Module does not have New Relic exporter configured"
    fi
}

generate_diagnosis_report() {
    local module=$1
    
    print_header "DIAGNOSIS REPORT: $module"
    
    local report_file="diagnosis_${module}_${TIMESTAMP}.md"
    
    cat > "$report_file" << EOF
# Diagnosis Report: $module

**Generated:** $(date)
**Module:** $module
**Port:** $(get_module_port "$module")
**Health Port:** $(get_health_port "$module")

## Summary

EOF

    # Add health status
    if check_module_health "$module" > /dev/null 2>&1; then
        echo "- ✅ Module is healthy and running" >> "$report_file"
    else
        echo "- ❌ Module has health issues" >> "$report_file"
    fi
    
    # Add configuration status
    if check_module_configuration "$module" > /dev/null 2>&1; then
        echo "- ✅ Configuration is valid" >> "$report_file"
    else
        echo "- ❌ Configuration has issues" >> "$report_file"
    fi
    
    # Add metrics status
    if check_expected_metrics "$module" > /dev/null 2>&1; then
        echo "- ✅ Expected metrics are present" >> "$report_file"
    else
        echo "- ❌ Some expected metrics are missing" >> "$report_file"
    fi
    
    cat >> "$report_file" << EOF

## Recommended Actions

1. **If module is not running:**
   \`\`\`bash
   cd modules/$module
   docker-compose up -d
   \`\`\`

2. **If metrics are missing:**
   - Check MySQL connectivity
   - Verify configuration syntax
   - Review container logs

3. **If New Relic export is failing:**
   - Verify NEW_RELIC_LICENSE_KEY
   - Check NEW_RELIC_OTLP_ENDPOINT
   - Review network connectivity

4. **For federation issues:**
   - Ensure dependent modules are running
   - Verify service endpoint configuration

## Useful Commands

\`\`\`bash
# Check container status
docker ps | grep $module

# View container logs
docker logs ${module}-otel-collector

# Test health endpoint
curl http://localhost:$(get_health_port "$module")/

# Test metrics endpoint  
curl http://localhost:$(get_module_port "$module")/metrics

# Restart module
cd modules/$module && docker-compose restart
\`\`\`

EOF

    log "SUCCESS" "Diagnosis report generated: $report_file"
}

run_full_diagnosis() {
    local module=$1
    
    log "INFO" "Starting full diagnosis for module: $module"
    
    local issues=0
    
    # Run all checks
    check_module_health "$module" || ((issues++))
    check_module_configuration "$module" || ((issues++))
    check_mysql_connectivity "$module"
    check_network_connectivity "$module"
    check_expected_metrics "$module" || ((issues++))
    check_new_relic_export "$module"
    
    # Generate report
    generate_diagnosis_report "$module"
    
    if [[ $issues -eq 0 ]]; then
        log "SUCCESS" "Diagnosis completed - no critical issues found"
    else
        log "WARN" "Diagnosis completed - $issues critical issues found"
    fi
    
    return $issues
}

show_usage() {
    echo "Usage: $0 [module_name] [options]"
    echo ""
    echo "Available modules:"
    for module in "${ALL_MODULES[@]}"; do
        echo "  - $module"
    done
    echo ""
    echo "Options:"
    echo "  --verbose    Show detailed output"
    echo "  --all        Check all modules"
    echo "  --help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 core-metrics"
    echo "  $0 --all --verbose"
    echo "  $0 sql-intelligence --verbose"
}

# Main execution
main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --verbose)
                VERBOSE=true
                shift
                ;;
            --all)
                MODULE="all"
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            -*)
                echo "Unknown option $1"
                show_usage
                exit 1
                ;;
            *)
                if [[ -z "$MODULE" ]]; then
                    MODULE="$1"
                fi
                shift
                ;;
        esac
    done
    
    # Check prerequisites
    check_prerequisites
    
    log "INFO" "Starting troubleshooting session - Log file: $LOG_FILE"
    
    if [[ "$MODULE" == "all" ]]; then
        log "INFO" "Running diagnosis for all modules..."
        local total_issues=0
        for module in "${ALL_MODULES[@]}"; do
            echo ""
            run_full_diagnosis "$module"
            ((total_issues += $?))
        done
        log "INFO" "Total issues found across all modules: $total_issues"
    elif [[ -n "$MODULE" ]]; then
        # Validate module name
        if [[ ! " ${ALL_MODULES[@]} " =~ " ${MODULE} " ]]; then
            log "ERROR" "Invalid module name: $MODULE"
            show_usage
            exit 1
        fi
        run_full_diagnosis "$MODULE"
    else
        log "ERROR" "No module specified"
        show_usage
        exit 1
    fi
    
    log "SUCCESS" "Troubleshooting session completed"
}

# Run main function
main "$@"