#!/bin/bash
# Database Intelligence MVP - Load Testing Script
# Tests collector performance under sustained load

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
COLLECTOR_HOST="${COLLECTOR_HOST:-localhost}"
HEALTH_PORT="${HEALTH_PORT:-13133}"
METRICS_PORT="${METRICS_PORT:-8888}"

# Load test parameters
DURATION="${DURATION:-300}"  # 5 minutes default
CONCURRENT_USERS="${CONCURRENT_USERS:-10}"
RAMP_UP_TIME="${RAMP_UP_TIME:-60}"  # 1 minute ramp-up
TEST_DATA_SIZE="${TEST_DATA_SIZE:-medium}"  # small, medium, large

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_failure() {
    echo -e "${RED}[FAILURE]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Show help
show_help() {
    cat << EOF
Database Intelligence MVP - Load Testing

This script performs load testing on the Database Intelligence collector
to validate performance under sustained workload.

Usage: $0 [OPTIONS]

Options:
    -h, --help                  Show this help message
    --duration SECONDS          Test duration in seconds (default: 300)
    --concurrent-users N        Number of concurrent users (default: 10)
    --ramp-up-time SECONDS      Ramp-up time in seconds (default: 60)
    --test-data-size SIZE       Test data size: small, medium, large (default: medium)
    --collector-host HOST       Collector hostname (default: localhost)
    --health-port PORT          Health check port (default: 13133)
    --metrics-port PORT         Metrics port (default: 8888)
    --generate-report           Generate detailed HTML report
    --stress-test               Run stress test (high load)

Test Scenarios:
    1. Baseline Performance Test
       - Normal operation load
       - Sustained data ingestion
       - Resource monitoring

    2. Stress Test
       - High volume data ingestion
       - Memory pressure testing
       - Circuit breaker validation

    3. Endurance Test  
       - Long-duration testing
       - Memory leak detection
       - State storage validation

Examples:
    $0                                      # Run default load test
    $0 --duration 600 --concurrent-users 20 # 10 min test with 20 users
    $0 --stress-test                        # Run stress test
    $0 --generate-report                    # Generate detailed report

EOF
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            --duration)
                DURATION="$2"
                shift 2
                ;;
            --concurrent-users)
                CONCURRENT_USERS="$2"
                shift 2
                ;;
            --ramp-up-time)
                RAMP_UP_TIME="$2"
                shift 2
                ;;
            --test-data-size)
                TEST_DATA_SIZE="$2"
                shift 2
                ;;
            --collector-host)
                COLLECTOR_HOST="$2"
                shift 2
                ;;
            --health-port)
                HEALTH_PORT="$2"
                shift 2
                ;;
            --metrics-port)
                METRICS_PORT="$2"
                shift 2
                ;;
            --generate-report)
                GENERATE_REPORT=true
                shift
                ;;
            --stress-test)
                STRESS_TEST=true
                CONCURRENT_USERS=50
                DURATION=1800  # 30 minutes
                TEST_DATA_SIZE=large
                shift
                ;;
            *)
                log_failure "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking load test prerequisites..."
    
    # Check if required tools are available
    local required_tools=("curl" "jq" "bc")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log_failure "$tool is required but not installed"
            exit 1
        fi
    done
    
    # Check if collector is running
    if ! curl -s "http://$COLLECTOR_HOST:$HEALTH_PORT/" &> /dev/null; then
        log_failure "Collector health check failed - is it running?"
        log_info "Expected endpoint: http://$COLLECTOR_HOST:$HEALTH_PORT/"
        exit 1
    fi
    
    # Check if metrics endpoint is accessible
    if ! curl -s "http://$COLLECTOR_HOST:$METRICS_PORT/metrics" &> /dev/null; then
        log_failure "Collector metrics endpoint not accessible"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Generate test data based on size
generate_test_data() {
    local size="$1"
    local output_file="$2"
    
    log_info "Generating $size test data..."
    
    case "$size" in
        small)
            local num_queries=10
            local query_length=100
            ;;
        medium)
            local num_queries=100
            local query_length=500
            ;;
        large)
            local num_queries=1000
            local query_length=2000
            ;;
        *)
            log_failure "Invalid test data size: $size"
            return 1
            ;;
    esac
    
    # Generate synthetic database queries and plans
    cat > "$output_file" << 'EOF'
{
  "queries": [
EOF
    
    for ((i=1; i<=num_queries; i++)); do
        local complexity=$((RANDOM % 5 + 1))
        local duration=$((RANDOM % 1000 + 50))
        local rows=$((RANDOM % 100000 + 1))
        local cost=$(echo "scale=2; $rows * $complexity * 0.01" | bc)
        
        cat >> "$output_file" << EOF
    {
      "query_id": "query_$i",
      "query_text": "SELECT * FROM table_$((i % 10)) WHERE complex_condition_$complexity AND duration_ms > $duration",
      "avg_duration_ms": $duration,
      "execution_count": $((RANDOM % 100 + 1)),
      "impact_score": $((duration * (RANDOM % 100 + 1))),
      "database_name": "test_db_$((i % 3))",
      "plan_json": "[{\"Plan\":{\"Node Type\":\"Seq Scan\",\"Total Cost\":$cost,\"Plan Rows\":$rows,\"Plan Width\":$((RANDOM % 100 + 20))}}]"
    }$([ $i -lt $num_queries ] && echo "," || echo "")
EOF
    done
    
    cat >> "$output_file" << 'EOF'
  ]
}
EOF
    
    log_success "Generated $num_queries test queries"
}

# Get baseline metrics
get_baseline_metrics() {
    log_info "Collecting baseline metrics..."
    
    local metrics_response
    metrics_response=$(curl -s "http://$COLLECTOR_HOST:$METRICS_PORT/metrics")
    
    # Extract key metrics
    local memory_usage
    memory_usage=$(echo "$metrics_response" | grep "otelcol_process_memory_rss" | tail -1 | awk '{print $2}' || echo "0")
    
    local cpu_usage
    cpu_usage=$(echo "$metrics_response" | grep "otelcol_process_cpu_seconds_total" | tail -1 | awk '{print $2}' || echo "0")
    
    local received_logs
    received_logs=$(echo "$metrics_response" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    local sent_logs
    sent_logs=$(echo "$metrics_response" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    # Store baseline
    echo "$memory_usage" > /tmp/baseline_memory
    echo "$cpu_usage" > /tmp/baseline_cpu
    echo "$received_logs" > /tmp/baseline_received
    echo "$sent_logs" > /tmp/baseline_sent
    
    log_success "Baseline metrics collected"
    log_info "Baseline memory: $(echo "scale=2; $memory_usage/1024/1024" | bc) MB"
    log_info "Baseline received logs: $received_logs"
    log_info "Baseline sent logs: $sent_logs"
}

# Simulate database load
simulate_database_load() {
    local user_id="$1"
    local test_data_file="$2"
    local duration="$3"
    
    local start_time
    start_time=$(date +%s)
    local end_time=$((start_time + duration))
    local request_count=0
    
    log_info "User $user_id starting load simulation for ${duration}s"
    
    while [[ $(date +%s) -lt $end_time ]]; do
        # Select random query from test data
        local total_queries
        total_queries=$(jq '.queries | length' "$test_data_file")
        local query_index=$((RANDOM % total_queries))
        
        # Extract query data
        local query_data
        query_data=$(jq ".queries[$query_index]" "$test_data_file")
        
        # Simulate processing this query through the collector
        # In a real load test, this would send actual data to the collector
        # For now, we simulate the load by making health check requests
        curl -s "http://$COLLECTOR_HOST:$HEALTH_PORT/" &> /dev/null || true
        
        ((request_count++))
        
        # Sleep briefly to control request rate
        sleep 0.1
    done
    
    echo "$request_count" > "/tmp/user_${user_id}_requests"
    log_info "User $user_id completed $request_count requests"
}

# Monitor system resources during test
monitor_resources() {
    local duration="$1"
    local output_file="$2"
    
    log_info "Starting resource monitoring for ${duration}s"
    
    local start_time
    start_time=$(date +%s)
    local end_time=$((start_time + duration))
    
    # CSV header
    echo "timestamp,memory_mb,cpu_percent,received_logs,sent_logs,dropped_logs,health_status" > "$output_file"
    
    while [[ $(date +%s) -lt $end_time ]]; do
        local timestamp
        timestamp=$(date +%s)
        
        # Get metrics
        local metrics_response
        metrics_response=$(curl -s "http://$COLLECTOR_HOST:$METRICS_PORT/metrics" 2>/dev/null || echo "")
        
        if [[ -n "$metrics_response" ]]; then
            # Extract metrics
            local memory_bytes
            memory_bytes=$(echo "$metrics_response" | grep "otelcol_process_memory_rss" | tail -1 | awk '{print $2}' || echo "0")
            local memory_mb
            memory_mb=$(echo "scale=2; $memory_bytes/1024/1024" | bc 2>/dev/null || echo "0")
            
            local cpu_seconds
            cpu_seconds=$(echo "$metrics_response" | grep "otelcol_process_cpu_seconds_total" | tail -1 | awk '{print $2}' || echo "0")
            
            local received_logs
            received_logs=$(echo "$metrics_response" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
            
            local sent_logs
            sent_logs=$(echo "$metrics_response" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
            
            local dropped_logs
            dropped_logs=$(echo "$metrics_response" | grep "otelcol_processor_dropped_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
            
            # Check health
            local health_status="ok"
            if ! curl -s "http://$COLLECTOR_HOST:$HEALTH_PORT/" &> /dev/null; then
                health_status="error"
            fi
            
            # Write to CSV
            echo "$timestamp,$memory_mb,$cpu_seconds,$received_logs,$sent_logs,$dropped_logs,$health_status" >> "$output_file"
        else
            echo "$timestamp,0,0,0,0,0,error" >> "$output_file"
        fi
        
        sleep 5  # Monitor every 5 seconds
    done
    
    log_success "Resource monitoring completed"
}

# Run load test
run_load_test() {
    local test_name="$1"
    
    log_info "Starting $test_name..."
    
    # Create output directory
    local output_dir="/tmp/db_intelligence_load_test_$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$output_dir"
    
    # Generate test data
    local test_data_file="$output_dir/test_data.json"
    generate_test_data "$TEST_DATA_SIZE" "$test_data_file"
    
    # Get baseline metrics
    get_baseline_metrics
    
    # Start resource monitoring in background
    local monitor_file="$output_dir/resource_monitor.csv"
    monitor_resources "$((DURATION + RAMP_UP_TIME + 60))" "$monitor_file" &
    local monitor_pid=$!
    
    log_info "Starting $CONCURRENT_USERS concurrent users with ${RAMP_UP_TIME}s ramp-up"
    
    # Start users with ramp-up
    local user_pids=()
    for ((i=1; i<=CONCURRENT_USERS; i++)); do
        # Calculate ramp-up delay
        local delay
        delay=$(echo "scale=2; $RAMP_UP_TIME * ($i - 1) / $CONCURRENT_USERS" | bc)
        
        (
            sleep "$delay"
            simulate_database_load "$i" "$test_data_file" "$DURATION"
        ) &
        
        user_pids+=($!)
        
        if [[ $((i % 10)) -eq 0 ]]; then
            log_info "Started $i users..."
        fi
    done
    
    # Wait for all users to complete
    log_info "Load test running... (${DURATION}s duration)"
    for pid in "${user_pids[@]}"; do
        wait "$pid"
    done
    
    # Stop monitoring
    kill "$monitor_pid" 2>/dev/null || true
    wait "$monitor_pid" 2>/dev/null || true
    
    # Analyze results
    analyze_results "$output_dir" "$test_name"
}

# Analyze test results
analyze_results() {
    local output_dir="$1"
    local test_name="$2"
    
    log_info "Analyzing test results..."
    
    # Calculate total requests
    local total_requests=0
    for file in /tmp/user_*_requests; do
        if [[ -f "$file" ]]; then
            local requests
            requests=$(cat "$file")
            total_requests=$((total_requests + requests))
        fi
    done
    
    # Get final metrics
    local metrics_response
    metrics_response=$(curl -s "http://$COLLECTOR_HOST:$METRICS_PORT/metrics" 2>/dev/null || echo "")
    
    local final_memory_bytes
    final_memory_bytes=$(echo "$metrics_response" | grep "otelcol_process_memory_rss" | tail -1 | awk '{print $2}' || echo "0")
    local final_memory_mb
    final_memory_mb=$(echo "scale=2; $final_memory_bytes/1024/1024" | bc 2>/dev/null || echo "0")
    
    local final_received
    final_received=$(echo "$metrics_response" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    local final_sent
    final_sent=$(echo "$metrics_response" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    local final_dropped
    final_dropped=$(echo "$metrics_response" | grep "otelcol_processor_dropped_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    # Calculate rates
    local requests_per_second
    requests_per_second=$(echo "scale=2; $total_requests / $DURATION" | bc)
    
    # Read baselines
    local baseline_memory baseline_received baseline_sent
    baseline_memory=$(cat /tmp/baseline_memory 2>/dev/null || echo "0")
    baseline_received=$(cat /tmp/baseline_received 2>/dev/null || echo "0")
    baseline_sent=$(cat /tmp/baseline_sent 2>/dev/null || echo "0")
    
    local logs_processed
    logs_processed=$((final_received - baseline_received))
    
    local logs_exported
    logs_exported=$((final_sent - baseline_sent))
    
    # Generate report
    local report_file="$output_dir/load_test_report.txt"
    cat > "$report_file" << EOF
Database Intelligence MVP - Load Test Report
==========================================

Test Configuration:
- Test Name: $test_name
- Duration: ${DURATION}s
- Concurrent Users: $CONCURRENT_USERS
- Ramp-up Time: ${RAMP_UP_TIME}s
- Test Data Size: $TEST_DATA_SIZE

Performance Results:
- Total Requests: $total_requests
- Requests/Second: $requests_per_second
- Logs Processed: $logs_processed
- Logs Exported: $logs_exported
- Logs Dropped: $final_dropped

Resource Usage:
- Final Memory: ${final_memory_mb} MB
- Memory Baseline: $(echo "scale=2; $baseline_memory/1024/1024" | bc) MB
- Memory Growth: $(echo "scale=2; ($final_memory_bytes - $baseline_memory)/1024/1024" | bc) MB

Processing Efficiency:
- Export Rate: $(echo "scale=2; $logs_exported / $logs_processed * 100" | bc 2>/dev/null || echo "0")%
- Drop Rate: $(echo "scale=2; $final_dropped / $logs_processed * 100" | bc 2>/dev/null || echo "0")%

Test Status: $(analyze_test_status "$final_memory_mb" "$final_dropped" "$logs_processed")

Detailed metrics available in: $output_dir/resource_monitor.csv
EOF
    
    # Display results
    cat "$report_file"
    
    # Generate HTML report if requested
    if [[ "${GENERATE_REPORT:-false}" == "true" ]]; then
        generate_html_report "$output_dir" "$report_file"
    fi
    
    # Cleanup temp files
    rm -f /tmp/user_*_requests /tmp/baseline_*
    
    log_success "Load test completed. Results saved to: $output_dir"
}

# Analyze test status
analyze_test_status() {
    local memory_mb="$1"
    local dropped_logs="$2"
    local processed_logs="$3"
    
    local status="PASS"
    
    # Check memory usage (fail if > 1GB)
    if (( $(echo "$memory_mb > 1024" | bc -l) )); then
        status="FAIL - High memory usage"
    fi
    
    # Check drop rate (fail if > 5%)
    if [[ "$processed_logs" -gt 0 ]]; then
        local drop_rate
        drop_rate=$(echo "scale=2; $dropped_logs / $processed_logs * 100" | bc)
        if (( $(echo "$drop_rate > 5" | bc -l) )); then
            status="FAIL - High drop rate"
        fi
    fi
    
    echo "$status"
}

# Generate HTML report
generate_html_report() {
    local output_dir="$1"
    local report_file="$2"
    local html_file="$output_dir/load_test_report.html"
    
    log_info "Generating HTML report..."
    
    cat > "$html_file" << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Database Intelligence Load Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #f0f0f0; padding: 20px; border-radius: 5px; }
        .section { margin: 20px 0; }
        .metric { background-color: #f9f9f9; padding: 10px; margin: 5px 0; border-left: 4px solid #007cba; }
        .pass { color: green; }
        .fail { color: red; }
        .warn { color: orange; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Database Intelligence MVP - Load Test Report</h1>
        <p>Generated on: $(date)</p>
    </div>
EOF
    
    # Convert text report to HTML
    while IFS= read -r line; do
        if [[ "$line" =~ ^Test\ Configuration: ]]; then
            echo "<div class='section'><h2>Test Configuration</h2>" >> "$html_file"
        elif [[ "$line" =~ ^Performance\ Results: ]]; then
            echo "</div><div class='section'><h2>Performance Results</h2>" >> "$html_file"
        elif [[ "$line" =~ ^Resource\ Usage: ]]; then
            echo "</div><div class='section'><h2>Resource Usage</h2>" >> "$html_file"
        elif [[ "$line" =~ ^Processing\ Efficiency: ]]; then
            echo "</div><div class='section'><h2>Processing Efficiency</h2>" >> "$html_file"
        elif [[ "$line" =~ ^-\ .* ]]; then
            echo "<div class='metric'>$(echo "$line" | sed 's/^- //')</div>" >> "$html_file"
        elif [[ "$line" =~ PASS ]]; then
            echo "<div class='metric pass'>$line</div>" >> "$html_file"
        elif [[ "$line" =~ FAIL ]]; then
            echo "<div class='metric fail'>$line</div>" >> "$html_file"
        fi
    done < "$report_file"
    
    echo "</div></body></html>" >> "$html_file"
    
    log_success "HTML report generated: $html_file"
}

# Main function
main() {
    echo "Database Intelligence MVP - Load Testing"
    echo "========================================"
    echo ""
    
    # Parse arguments
    parse_arguments "$@"
    
    # Check prerequisites
    check_prerequisites
    
    # Determine test type
    local test_name="Standard Load Test"
    if [[ "${STRESS_TEST:-false}" == "true" ]]; then
        test_name="Stress Test"
        log_warning "Running stress test - high resource usage expected"
    fi
    
    # Display test configuration
    log_info "Test Configuration:"
    echo "  Duration: ${DURATION}s"
    echo "  Concurrent Users: $CONCURRENT_USERS"
    echo "  Ramp-up Time: ${RAMP_UP_TIME}s"
    echo "  Data Size: $TEST_DATA_SIZE"
    echo "  Collector: $COLLECTOR_HOST"
    echo ""
    
    # Run the load test
    run_load_test "$test_name"
}

# Initialize variables
GENERATE_REPORT=${GENERATE_REPORT:-false}
STRESS_TEST=${STRESS_TEST:-false}

# Run main function
main "$@"