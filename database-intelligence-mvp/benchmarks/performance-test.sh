#!/bin/bash

# Database Intelligence Collector Performance Benchmark
# This script tests the collector under various load conditions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COLLECTOR_BINARY="$PROJECT_ROOT/dist/database-intelligence-collector"
RESULTS_DIR="$PROJECT_ROOT/benchmarks/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create results directory
mkdir -p "$RESULTS_DIR"

echo -e "${GREEN}Database Intelligence Collector - Performance Benchmark${NC}"
echo "=================================================="
echo "Timestamp: $TIMESTAMP"
echo "Binary: $COLLECTOR_BINARY"
echo ""

# Function to measure collector performance
benchmark_collector() {
    local test_name=$1
    local config_file=$2
    local duration=$3
    local description=$4
    
    echo -e "${YELLOW}Running test: $test_name${NC}"
    echo "Description: $description"
    echo "Duration: ${duration}s"
    echo ""
    
    # Start collector in background
    $COLLECTOR_BINARY --config="$config_file" > "$RESULTS_DIR/${test_name}_${TIMESTAMP}.log" 2>&1 &
    local collector_pid=$!
    
    # Wait for collector to start
    sleep 5
    
    # Monitor resource usage
    local cpu_samples=()
    local mem_samples=()
    local start_time=$(date +%s)
    
    while [ $(($(date +%s) - start_time)) -lt $duration ]; do
        # Get CPU and memory usage
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS
            cpu=$(ps -p $collector_pid -o %cpu | tail -1 | tr -d ' ')
            mem=$(ps -p $collector_pid -o rss | tail -1 | tr -d ' ')
            mem=$((mem / 1024)) # Convert to MB
        else
            # Linux
            cpu=$(ps -p $collector_pid -o %cpu --no-headers)
            mem=$(ps -p $collector_pid -o rss --no-headers)
            mem=$((mem / 1024)) # Convert to MB
        fi
        
        cpu_samples+=($cpu)
        mem_samples+=($mem)
        
        sleep 1
    done
    
    # Stop collector
    kill -TERM $collector_pid 2>/dev/null || true
    wait $collector_pid 2>/dev/null || true
    
    # Calculate statistics
    local cpu_avg=$(echo "${cpu_samples[@]}" | awk '{sum=0; for(i=1;i<=NF;i++)sum+=$i; print sum/NF}')
    local cpu_max=$(echo "${cpu_samples[@]}" | awk '{max=$1; for(i=2;i<=NF;i++)if($i>max)max=$i; print max}')
    local mem_avg=$(echo "${mem_samples[@]}" | awk '{sum=0; for(i=1;i<=NF;i++)sum+=$i; print sum/NF}')
    local mem_max=$(echo "${mem_samples[@]}" | awk '{max=$1; for(i=2;i<=NF;i++)if($i>max)max=$i; print max}')
    
    # Extract metrics from logs
    local metrics_collected=$(grep -c "otelcol_receiver_accepted_metric_points" "$RESULTS_DIR/${test_name}_${TIMESTAMP}.log" 2>/dev/null || echo "0")
    local errors=$(grep -c "error" "$RESULTS_DIR/${test_name}_${TIMESTAMP}.log" 2>/dev/null || echo "0")
    
    # Print results
    echo -e "${GREEN}Results for $test_name:${NC}"
    echo "  CPU Usage: avg=${cpu_avg}%, max=${cpu_max}%"
    echo "  Memory Usage: avg=${mem_avg}MB, max=${mem_max}MB"
    echo "  Metrics Collected: $metrics_collected"
    echo "  Errors: $errors"
    echo ""
    
    # Save results to CSV
    echo "$test_name,$TIMESTAMP,$cpu_avg,$cpu_max,$mem_avg,$mem_max,$metrics_collected,$errors" >> "$RESULTS_DIR/benchmark_results.csv"
}

# Create test configurations
create_test_configs() {
    # Test 1: Minimal configuration
    cat > "$RESULTS_DIR/test1_minimal.yaml" << EOF
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    collection_interval: 30s
    
processors:
  batch:
    send_batch_size: 100
    
exporters:
  debug:
    verbosity: normal
    
service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [debug]
EOF

    # Test 2: Standard configuration
    cat > "$RESULTS_DIR/test2_standard.yaml" << EOF
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    collection_interval: 10s
  mysql:
    endpoint: localhost:3306
    username: root
    password: mysql
    collection_interval: 10s
    
processors:
  memory_limiter:
    limit_percentage: 75
  batch:
    send_batch_size: 1000
    
exporters:
  file:
    path: ./metrics.json
    
service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, batch]
      exporters: [file]
EOF

    # Test 3: High frequency collection
    cat > "$RESULTS_DIR/test3_high_freq.yaml" << EOF
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    collection_interval: 1s
  mysql:
    endpoint: localhost:3306
    username: root
    password: mysql
    collection_interval: 1s
    
processors:
  memory_limiter:
    limit_percentage: 75
  batch:
    send_batch_size: 10000
    timeout: 5s
    
exporters:
  file:
    path: ./metrics-highfreq.json
    
service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, batch]
      exporters: [file]
EOF

    # Test 4: With all processors
    cat > "$RESULTS_DIR/test4_full_pipeline.yaml" << EOF
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    collection_interval: 5s
  mysql:
    endpoint: localhost:3306
    username: root
    password: mysql
    collection_interval: 5s
  filelog:
    include: [ "./sample-query.log" ]
    
processors:
  memory_limiter:
    limit_percentage: 75
  resource:
    attributes:
      - key: environment
        value: benchmark
        action: upsert
  transform:
    metric_statements:
      - context: metric
        statements:
          - set(resource.attributes["test"], "benchmark")
  batch:
    send_batch_size: 5000
    
exporters:
  prometheus:
    endpoint: 0.0.0.0:9090
  file:
    path: ./metrics-full.json
    
service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, transform, batch]
      exporters: [prometheus, file]
    logs:
      receivers: [filelog]
      processors: [memory_limiter, batch]
      exporters: [file]
EOF
}

# Generate load on databases
generate_database_load() {
    echo -e "${YELLOW}Generating database load...${NC}"
    
    # PostgreSQL load
    PGPASSWORD=postgres psql -h localhost -U postgres -d postgres << EOF > /dev/null 2>&1 || true
-- Create test table if not exists
CREATE TABLE IF NOT EXISTS benchmark_test (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Generate load
DO \$\$
BEGIN
    FOR i IN 1..1000 LOOP
        INSERT INTO benchmark_test (data) VALUES (md5(random()::text));
        UPDATE benchmark_test SET data = md5(random()::text) WHERE id = (random() * 1000)::int;
        DELETE FROM benchmark_test WHERE id = (random() * 1000)::int;
    END LOOP;
END\$\$;
EOF

    # MySQL load
    mysql -h localhost -u root -pmysql << EOF > /dev/null 2>&1 || true
USE mysql;
CREATE TABLE IF NOT EXISTS benchmark_test (
    id INT AUTO_INCREMENT PRIMARY KEY,
    data VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

DELIMITER //
CREATE PROCEDURE generate_load()
BEGIN
    DECLARE i INT DEFAULT 0;
    WHILE i < 1000 DO
        INSERT INTO benchmark_test (data) VALUES (MD5(RAND()));
        UPDATE benchmark_test SET data = MD5(RAND()) WHERE id = FLOOR(RAND() * 1000);
        DELETE FROM benchmark_test WHERE id = FLOOR(RAND() * 1000);
        SET i = i + 1;
    END WHILE;
END//
DELIMITER ;

CALL generate_load();
EOF
}

# Main benchmark execution
main() {
    # Check if collector binary exists
    if [ ! -f "$COLLECTOR_BINARY" ]; then
        echo -e "${RED}Error: Collector binary not found at $COLLECTOR_BINARY${NC}"
        echo "Please build the collector first with: make build"
        exit 1
    fi
    
    # Initialize results CSV
    echo "test_name,timestamp,cpu_avg,cpu_max,mem_avg_mb,mem_max_mb,metrics_collected,errors" > "$RESULTS_DIR/benchmark_results.csv"
    
    # Create test configurations
    create_test_configs
    
    # Run benchmarks
    benchmark_collector "test1_minimal" "$RESULTS_DIR/test1_minimal.yaml" 60 "Minimal configuration with PostgreSQL only"
    
    benchmark_collector "test2_standard" "$RESULTS_DIR/test2_standard.yaml" 60 "Standard configuration with both databases"
    
    # Generate load for high-frequency tests
    generate_database_load &
    load_pid=$!
    
    benchmark_collector "test3_high_freq" "$RESULTS_DIR/test3_high_freq.yaml" 120 "High frequency collection (1s interval)"
    
    benchmark_collector "test4_full_pipeline" "$RESULTS_DIR/test4_full_pipeline.yaml" 120 "Full pipeline with all processors"
    
    # Stop load generation
    kill $load_pid 2>/dev/null || true
    
    # Generate summary report
    echo -e "${GREEN}Benchmark Complete!${NC}"
    echo ""
    echo "Summary Report:"
    echo "==============="
    
    # Display results table
    column -t -s ',' "$RESULTS_DIR/benchmark_results.csv"
    
    # Archive results
    tar -czf "$RESULTS_DIR/benchmark_${TIMESTAMP}.tar.gz" \
        "$RESULTS_DIR"/*_${TIMESTAMP}.log \
        "$RESULTS_DIR"/test*.yaml \
        "$RESULTS_DIR/benchmark_results.csv"
    
    echo ""
    echo "Results archived to: $RESULTS_DIR/benchmark_${TIMESTAMP}.tar.gz"
}

# Run main function
main "$@"