#!/bin/bash
# Performance benchmark script for database collectors

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DURATION=${DURATION:-300}  # 5 minutes default
DATABASE=${1:-postgresql}
CONFIG_FILE="configs/${DATABASE}-maximum-extraction.yaml"

echo -e "${BLUE}=== Database Intelligence Performance Benchmark ===${NC}"
echo -e "Database: ${YELLOW}$DATABASE${NC}"
echo -e "Duration: ${YELLOW}$DURATION seconds${NC}"
echo -e "Config: ${YELLOW}$CONFIG_FILE${NC}"
echo ""

# Check if config exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Configuration file not found: $CONFIG_FILE${NC}"
    exit 1
fi

# Function to get metrics
get_metrics() {
    local endpoint=$1
    curl -s "$endpoint" 2>/dev/null || echo ""
}

# Function to calculate rate
calculate_rate() {
    local start=$1
    local end=$2
    local duration=$3
    echo "scale=2; ($end - $start) / $duration" | bc
}

# Start collector in background
echo -e "${YELLOW}Starting collector...${NC}"
docker run -d \
    --name otel-benchmark-${DATABASE} \
    --rm \
    -v "$(pwd)/$CONFIG_FILE:/etc/otelcol/config.yaml" \
    -e NEW_RELIC_LICENSE_KEY \
    -e ${DATABASE^^}_HOST \
    -e ${DATABASE^^}_PORT \
    -e ${DATABASE^^}_USER \
    -e ${DATABASE^^}_PASSWORD \
    -p 8888:8888 \
    otel/opentelemetry-collector-contrib:latest > /dev/null 2>&1

# Wait for collector to start
echo -e "${YELLOW}Waiting for collector to initialize...${NC}"
sleep 10

# Check if collector is running
if ! docker ps | grep -q otel-benchmark-${DATABASE}; then
    echo -e "${RED}Error: Collector failed to start${NC}"
    docker logs otel-benchmark-${DATABASE} 2>&1 | tail -20
    exit 1
fi

# Get initial metrics
echo -e "${YELLOW}Collecting baseline metrics...${NC}"
METRICS_START=$(get_metrics "http://localhost:8888/metrics")
START_TIME=$(date +%s)

# Parse initial values
DATAPOINTS_START=$(echo "$METRICS_START" | grep "otelcol_processor_accepted_metric_points{" | grep -v "#" | awk '{sum+=$2} END {print sum}')
MEMORY_START=$(echo "$METRICS_START" | grep "process_runtime_memstats_sys_bytes" | grep -v "#" | awk '{print $2}')

echo -e "Initial datapoints: ${DATAPOINTS_START:-0}"
echo -e "Initial memory: ${MEMORY_START:-0} bytes"

# Run benchmark
echo -e "\n${YELLOW}Running benchmark for $DURATION seconds...${NC}"
echo -e "Progress: "

# Progress bar
for i in $(seq 1 $DURATION); do
    if [ $((i % 10)) -eq 0 ]; then
        echo -n "."
    fi
    if [ $((i % 60)) -eq 0 ]; then
        echo " ${i}s"
    fi
    sleep 1
done
echo " Done!"

# Get final metrics
echo -e "\n${YELLOW}Collecting final metrics...${NC}"
METRICS_END=$(get_metrics "http://localhost:8888/metrics")
END_TIME=$(date +%s)
ACTUAL_DURATION=$((END_TIME - START_TIME))

# Parse final values
DATAPOINTS_END=$(echo "$METRICS_END" | grep "otelcol_processor_accepted_metric_points{" | grep -v "#" | awk '{sum+=$2} END {print sum}')
MEMORY_END=$(echo "$METRICS_END" | grep "process_runtime_memstats_sys_bytes" | grep -v "#" | awk '{print $2}')
CPU_SECONDS=$(echo "$METRICS_END" | grep "process_cpu_seconds_total" | grep -v "#" | awk '{print $2}')

# Calculate results
DATAPOINTS_TOTAL=$((${DATAPOINTS_END:-0} - ${DATAPOINTS_START:-0}))
DATAPOINTS_RATE=$(calculate_rate ${DATAPOINTS_START:-0} ${DATAPOINTS_END:-0} $ACTUAL_DURATION)
MEMORY_GROWTH=$((${MEMORY_END:-0} - ${MEMORY_START:-0}))
MEMORY_GROWTH_MB=$(echo "scale=2; $MEMORY_GROWTH / 1024 / 1024" | bc)
AVG_CPU=$(echo "scale=2; ${CPU_SECONDS:-0} / $ACTUAL_DURATION * 100" | bc)

# Get error counts
ERRORS=$(echo "$METRICS_END" | grep "otelcol_processor_refused_metric_points{" | grep -v "#" | awk '{sum+=$2} END {print sum}')
QUEUE_SIZE=$(echo "$METRICS_END" | grep "otelcol_processor_batch_batch_size_trigger_send{" | grep -v "#" | awk '{sum+=$2} END {print sum}')

# Display results
echo -e "\n${BLUE}=== Benchmark Results ===${NC}"
echo -e "Duration: ${GREEN}$ACTUAL_DURATION seconds${NC}"
echo -e "Total Datapoints: ${GREEN}$DATAPOINTS_TOTAL${NC}"
echo -e "Datapoints/sec: ${GREEN}$DATAPOINTS_RATE${NC}"
echo -e "Memory Growth: ${YELLOW}$MEMORY_GROWTH_MB MB${NC}"
echo -e "Average CPU: ${GREEN}$AVG_CPU%${NC}"
echo -e "Errors: ${RED}${ERRORS:-0}${NC}"
echo -e "Batch Sends: ${GREEN}${QUEUE_SIZE:-0}${NC}"

# Performance analysis
echo -e "\n${BLUE}=== Performance Analysis ===${NC}"

if (( $(echo "$DATAPOINTS_RATE < 100" | bc -l) )); then
    echo -e "${RED}⚠ Low throughput detected${NC}"
elif (( $(echo "$DATAPOINTS_RATE > 10000" | bc -l) )); then
    echo -e "${GREEN}✓ Excellent throughput${NC}"
else
    echo -e "${GREEN}✓ Good throughput${NC}"
fi

if (( $(echo "$MEMORY_GROWTH_MB > 100" | bc -l) )); then
    echo -e "${RED}⚠ High memory growth detected${NC}"
else
    echo -e "${GREEN}✓ Memory usage stable${NC}"
fi

if (( $(echo "$AVG_CPU > 80" | bc -l) )); then
    echo -e "${RED}⚠ High CPU usage detected${NC}"
else
    echo -e "${GREEN}✓ CPU usage acceptable${NC}"
fi

# Cardinality check
echo -e "\n${BLUE}=== Cardinality Analysis ===${NC}"
UNIQUE_METRICS=$(docker logs otel-benchmark-${DATABASE} 2>&1 | grep -E "metric_name:" | sort | uniq | wc -l)
echo -e "Unique metric types: ${GREEN}$UNIQUE_METRICS${NC}"

# Cleanup
echo -e "\n${YELLOW}Cleaning up...${NC}"
docker stop otel-benchmark-${DATABASE} > /dev/null 2>&1

echo -e "\n${GREEN}✓ Benchmark complete!${NC}"

# Save results
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="benchmark_results_${DATABASE}_${TIMESTAMP}.txt"
cat > "$RESULTS_FILE" << EOF
Database Intelligence Performance Benchmark Results
Database: $DATABASE
Timestamp: $(date)
Duration: $ACTUAL_DURATION seconds

Metrics:
- Total Datapoints: $DATAPOINTS_TOTAL
- Datapoints/sec: $DATAPOINTS_RATE
- Unique Metrics: $UNIQUE_METRICS

Resources:
- Memory Growth: $MEMORY_GROWTH_MB MB
- Average CPU: $AVG_CPU%

Errors:
- Refused Points: ${ERRORS:-0}

Configuration: $CONFIG_FILE
EOF

echo -e "\nResults saved to: ${YELLOW}$RESULTS_FILE${NC}"