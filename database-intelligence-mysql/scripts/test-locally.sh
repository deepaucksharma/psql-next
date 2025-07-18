#!/bin/bash

# Local testing script for MySQL Wait-Based Monitoring without Docker
# This script sets up a local testing environment using native MySQL and OTel collectors

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}=== MySQL Wait-Based Monitoring - Local Testing Setup ===${NC}"
echo -e "${YELLOW}Note: This script runs without Docker for testing purposes${NC}"
echo ""

# Function to print status
print_status() {
    local status=$1
    local message=$2
    case $status in
        "success")
            echo -e "${GREEN}✓${NC} $message"
            ;;
        "error")
            echo -e "${RED}✗${NC} $message"
            ;;
        "info")
            echo -e "${CYAN}ℹ${NC} $message"
            ;;
    esac
}

# Check for required tools
check_requirements() {
    echo -e "${CYAN}Checking requirements...${NC}"
    
    # Check for MySQL
    if command -v mysql &> /dev/null; then
        print_status "success" "MySQL client found: $(mysql --version | head -1)"
    else
        print_status "error" "MySQL client not found. Install with: brew install mysql"
        return 1
    fi
    
    # Check for curl
    if command -v curl &> /dev/null; then
        print_status "success" "curl found"
    else
        print_status "error" "curl not found"
        return 1
    fi
    
    # Check for jq
    if command -v jq &> /dev/null; then
        print_status "success" "jq found"
    else
        print_status "warning" "jq not found. Install with: brew install jq"
    fi
    
    return 0
}

# Download OTel Collector if not present
download_otel_collector() {
    local OTEL_VERSION="0.96.0"
    local BINARY_NAME="otelcol-contrib"
    local DOWNLOAD_DIR="./bin"
    
    mkdir -p "$DOWNLOAD_DIR"
    
    if [ -f "$DOWNLOAD_DIR/$BINARY_NAME" ]; then
        print_status "success" "OTel Collector already downloaded"
        return 0
    fi
    
    echo -e "${CYAN}Downloading OpenTelemetry Collector...${NC}"
    
    # Detect OS and architecture
    local OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    local ARCH=$(uname -m)
    
    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
    esac
    
    local URL="https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_VERSION}/otelcol-contrib_${OTEL_VERSION}_${OS}_${ARCH}.tar.gz"
    
    print_status "info" "Downloading from: $URL"
    
    if curl -L "$URL" -o "$DOWNLOAD_DIR/otelcol-contrib.tar.gz"; then
        tar -xzf "$DOWNLOAD_DIR/otelcol-contrib.tar.gz" -C "$DOWNLOAD_DIR" otelcol-contrib
        rm "$DOWNLOAD_DIR/otelcol-contrib.tar.gz"
        chmod +x "$DOWNLOAD_DIR/$BINARY_NAME"
        print_status "success" "OTel Collector downloaded successfully"
    else
        print_status "error" "Failed to download OTel Collector"
        return 1
    fi
}

# Create local test configuration
create_test_config() {
    echo -e "${CYAN}Creating test configuration...${NC}"
    
    cat > config/local-test.yaml << 'EOF'
# Local test configuration for MySQL Wait-Based Monitoring
receivers:
  # Prometheus receiver for self-metrics
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s
          static_configs:
            - targets: ['localhost:8888']

  # SQL Query receiver for testing
  sqlquery/test:
    driver: mysql
    datasource: "root:password@tcp(localhost:3306)/test"
    collection_interval: 10s
    queries:
      - sql: "SELECT VERSION() as version, NOW() as current_time"
        metrics:
          - metric_name: mysql.test.info
            value_column: current_time
            value_type: string
            attribute_columns: [version]

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
    spike_limit_mib: 32

  batch:
    timeout: 10s
    send_batch_size: 1000

exporters:
  prometheus:
    endpoint: 0.0.0.0:9091
    namespace: mysql_wait_test

  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100

  file:
    path: ./metrics.json
    format: json

service:
  pipelines:
    metrics:
      receivers: [prometheus, sqlquery/test]
      processors: [memory_limiter, batch]
      exporters: [prometheus, debug, file]
  
  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: "/health"
  pprof:
    endpoint: 0.0.0.0:1777
  zpages:
    endpoint: 0.0.0.0:55679
EOF
    
    print_status "success" "Created local test configuration"
}

# Start edge collector locally
start_edge_collector() {
    echo -e "${CYAN}Starting edge collector...${NC}"
    
    # Create log directory
    mkdir -p logs
    
    # Set environment variables
    export MYSQL_MONITOR_USER="${MYSQL_MONITOR_USER:-root}"
    export MYSQL_MONITOR_PASS="${MYSQL_MONITOR_PASS:-password}"
    export MYSQL_PRIMARY_HOST="${MYSQL_PRIMARY_HOST:-localhost}"
    export GATEWAY_ENDPOINT="${GATEWAY_ENDPOINT:-localhost:4317}"
    export HOSTNAME="${HOSTNAME:-local-test}"
    export ENVIRONMENT="${ENVIRONMENT:-test}"
    
    # Start collector in background
    ./bin/otelcol-contrib --config=config/local-test.yaml > logs/edge-collector.log 2>&1 &
    local PID=$!
    echo $PID > edge-collector.pid
    
    sleep 5
    
    # Check if collector started successfully
    if kill -0 $PID 2>/dev/null; then
        print_status "success" "Edge collector started (PID: $PID)"
        return 0
    else
        print_status "error" "Edge collector failed to start"
        tail -20 logs/edge-collector.log
        return 1
    fi
}

# Test collector endpoints
test_endpoints() {
    echo -e "${CYAN}Testing collector endpoints...${NC}"
    
    # Test health endpoint
    if curl -s http://localhost:13133/health | grep -q "Server available"; then
        print_status "success" "Health endpoint responding"
    else
        print_status "error" "Health endpoint not responding"
    fi
    
    # Test metrics endpoint
    if curl -s http://localhost:8888/metrics | grep -q "otelcol_"; then
        print_status "success" "Metrics endpoint responding"
        
        # Show some key metrics
        echo -e "${CYAN}Key metrics:${NC}"
        curl -s http://localhost:8888/metrics | grep -E "(otelcol_receiver_accepted|otelcol_exporter_sent|otelcol_processor_batch)" | head -5
    else
        print_status "error" "Metrics endpoint not responding"
    fi
    
    # Test Prometheus exporter
    if curl -s http://localhost:9091/metrics | grep -q "mysql_wait_test"; then
        print_status "success" "Prometheus exporter responding"
    else
        print_status "warning" "Prometheus exporter not yet ready"
    fi
}

# Generate test queries
generate_test_queries() {
    echo -e "${CYAN}Generating test SQL queries...${NC}"
    
    cat > test-queries.sql << 'EOF'
-- Test queries for wait analysis

-- 1. Check Performance Schema status
SELECT * FROM performance_schema.setup_instruments 
WHERE NAME LIKE 'wait/%' 
LIMIT 10;

-- 2. Current wait events
SELECT 
    THREAD_ID,
    EVENT_NAME,
    TIMER_WAIT/1000000 as wait_ms,
    OBJECT_SCHEMA,
    OBJECT_NAME
FROM performance_schema.events_waits_current
WHERE TIMER_WAIT > 0
ORDER BY TIMER_WAIT DESC
LIMIT 10;

-- 3. Statement digest summary
SELECT 
    DIGEST_TEXT,
    COUNT_STAR as exec_count,
    AVG_TIMER_WAIT/1000000 as avg_ms,
    SUM_TIMER_WAIT/1000000000 as total_sec
FROM performance_schema.events_statements_summary_by_digest
WHERE DIGEST_TEXT IS NOT NULL
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 10;

-- 4. Table I/O waits
SELECT 
    OBJECT_SCHEMA,
    OBJECT_NAME,
    COUNT_STAR as total_ios,
    SUM_TIMER_WAIT/1000000000 as total_wait_sec,
    AVG_TIMER_WAIT/1000000 as avg_wait_ms
FROM performance_schema.table_io_waits_summary_by_table
WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 10;

-- 5. Lock waits
SELECT 
    OBJECT_SCHEMA,
    OBJECT_NAME,
    INDEX_NAME,
    LOCK_TYPE,
    LOCK_MODE,
    LOCK_STATUS,
    LOCK_DATA
FROM performance_schema.data_locks
WHERE LOCK_STATUS = 'WAITING'
LIMIT 10;
EOF
    
    print_status "success" "Created test-queries.sql"
    print_status "info" "Run with: mysql -u root -p < test-queries.sql"
}

# Stop services
stop_services() {
    echo -e "${CYAN}Stopping services...${NC}"
    
    if [ -f edge-collector.pid ]; then
        local PID=$(cat edge-collector.pid)
        if kill $PID 2>/dev/null; then
            print_status "success" "Stopped edge collector"
        fi
        rm edge-collector.pid
    fi
}

# Show monitoring commands
show_monitoring_commands() {
    echo -e "\n${CYAN}=== Monitoring Commands ===${NC}"
    cat << 'EOF'

# Watch collector metrics
watch -n 2 'curl -s http://localhost:8888/metrics | grep -E "(receiver_accepted|exporter_sent|queue_size)"'

# View Prometheus metrics
curl -s http://localhost:9091/metrics | grep mysql_

# Check health
curl -s http://localhost:13133/health | jq .

# View logs
tail -f logs/edge-collector.log

# Check exported metrics
tail -f metrics.json | jq .

# Run test queries
mysql -u root -p < test-queries.sql

# Stop services
./scripts/test-locally.sh stop
EOF
}

# Main function
main() {
    case "${1:-start}" in
        start)
            check_requirements || exit 1
            download_otel_collector || exit 1
            create_test_config
            start_edge_collector || exit 1
            sleep 2
            test_endpoints
            generate_test_queries
            show_monitoring_commands
            ;;
        stop)
            stop_services
            ;;
        status)
            test_endpoints
            ;;
        *)
            echo "Usage: $0 {start|stop|status}"
            exit 1
            ;;
    esac
}

# Run main
main "$@"