#!/bin/bash
# Check metric cardinality and identify high-cardinality metrics

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DATABASE=${1:-postgresql}
DURATION=${2:-60}  # Default 60 seconds

echo -e "${BLUE}=== Metric Cardinality Analysis ===${NC}"
echo -e "Database: ${YELLOW}$DATABASE${NC}"
echo -e "Analysis Duration: ${YELLOW}$DURATION seconds${NC}"
echo ""

CONFIG_FILE="configs/${DATABASE}-maximum-extraction.yaml"

# Check if config exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Configuration file not found: $CONFIG_FILE${NC}"
    exit 1
fi

# Start collector with debug logging
echo -e "${YELLOW}Starting collector with debug logging...${NC}"
docker run -d \
    --name otel-cardinality-${DATABASE} \
    --rm \
    -v "$(pwd)/$CONFIG_FILE:/etc/otelcol/config.yaml" \
    -e NEW_RELIC_LICENSE_KEY \
    -e ${DATABASE^^}_HOST \
    -e ${DATABASE^^}_PORT \
    -e ${DATABASE^^}_USER \
    -e ${DATABASE^^}_PASSWORD \
    -p 8888:8888 \
    otel/opentelemetry-collector-contrib:latest \
    --set=service.telemetry.logs.level=debug > /dev/null 2>&1

# Wait for collector to start
echo -e "${YELLOW}Waiting for collector to initialize...${NC}"
sleep 10

# Check if collector is running
if ! docker ps | grep -q otel-cardinality-${DATABASE}; then
    echo -e "${RED}Error: Collector failed to start${NC}"
    docker logs otel-cardinality-${DATABASE} 2>&1 | tail -20
    exit 1
fi

# Collect metrics for specified duration
echo -e "${YELLOW}Collecting metrics for $DURATION seconds...${NC}"
TEMP_LOG="/tmp/otel_cardinality_${DATABASE}_$$.log"
docker logs -f otel-cardinality-${DATABASE} > "$TEMP_LOG" 2>&1 &
LOG_PID=$!

# Progress indicator
for i in $(seq 1 $DURATION); do
    if [ $((i % 10)) -eq 0 ]; then
        echo -n "."
    fi
    sleep 1
done
echo " Done!"

# Stop log collection
kill $LOG_PID 2>/dev/null || true

# Analyze cardinality
echo -e "\n${BLUE}=== Cardinality Analysis ===${NC}"

# Extract metric information
echo -e "\n${YELLOW}Processing collected metrics...${NC}"

# Function to analyze metric cardinality
analyze_cardinality() {
    local log_file=$1
    
    # Extract metrics with their attribute combinations
    grep -E "Metric #|Name:|Value:|Attributes:" "$log_file" | \
    awk '
    /Metric #/ { 
        if (metric_name != "") {
            metrics[metric_name][attributes]++
            total_points[metric_name]++
        }
        metric_name = ""
        attributes = ""
    }
    /Name:/ { 
        gsub(/.*Name: /, "")
        metric_name = $0
    }
    /Attributes:/ {
        gsub(/.*Attributes: /, "")
        attributes = $0
    }
    END {
        for (metric in metrics) {
            cardinality[metric] = length(metrics[metric])
        }
        # Sort by cardinality
        n = asorti(cardinality, sorted_metrics, "@val_num_desc")
        
        print "Top High-Cardinality Metrics:"
        print "============================="
        for (i = 1; i <= n && i <= 20; i++) {
            metric = sorted_metrics[i]
            printf "%-50s Cardinality: %-6d Points: %d\n", 
                   metric, cardinality[metric], total_points[metric]
        }
    }'
}

# Analyze from Prometheus endpoint
echo -e "\n${YELLOW}Fetching metrics from Prometheus endpoint...${NC}"
curl -s http://localhost:8888/metrics | grep -E "^[a-z].*{" | \
awk -F'{' '{
    metric = $1
    attributes = $2
    gsub(/}.*/, "", attributes)
    
    # Count unique attribute combinations per metric
    if (attributes != "") {
        cardinality[metric][attributes] = 1
    }
}
END {
    print "\nMetric Cardinality Summary:"
    print "=========================="
    total_series = 0
    for (metric in cardinality) {
        card = length(cardinality[metric])
        total_series += card
        if (card > 10) {
            printf "%-50s %d series\n", metric, card
        }
    }
    print "\nTotal Active Series: " total_series
}'

# Memory impact estimation
echo -e "\n${BLUE}=== Memory Impact Estimation ===${NC}"
METRICS_RESPONSE=$(curl -s http://localhost:8888/metrics)
TOTAL_SERIES=$(echo "$METRICS_RESPONSE" | grep -E "^[a-z].*{" | wc -l)
ESTIMATED_MEMORY_MB=$(echo "scale=2; $TOTAL_SERIES * 2 / 1024" | bc)  # ~2KB per series

echo -e "Total Active Series: ${YELLOW}$TOTAL_SERIES${NC}"
echo -e "Estimated Memory Usage: ${YELLOW}$ESTIMATED_MEMORY_MB MB${NC}"

# Recommendations
echo -e "\n${BLUE}=== Recommendations ===${NC}"

if [ $TOTAL_SERIES -gt 10000 ]; then
    echo -e "${RED}⚠ High cardinality detected!${NC}"
    echo -e "Consider:"
    echo -e "  - Adding metric filters to reduce cardinality"
    echo -e "  - Dropping unnecessary attributes"
    echo -e "  - Increasing collection intervals for high-cardinality metrics"
elif [ $TOTAL_SERIES -gt 5000 ]; then
    echo -e "${YELLOW}⚠ Moderate cardinality${NC}"
    echo -e "Monitor memory usage and consider optimization if needed"
else
    echo -e "${GREEN}✓ Cardinality is within acceptable range${NC}"
fi

# Attribute analysis
echo -e "\n${BLUE}=== High-Cardinality Attributes ===${NC}"
grep -E "Attributes:" "$TEMP_LOG" | \
    sed 's/.*Attributes: //' | \
    tr ',' '\n' | \
    grep -E "query_id|session_id|client_addr|query_text" | \
    sort | uniq -c | sort -nr | head -10

# Cleanup
echo -e "\n${YELLOW}Cleaning up...${NC}"
docker stop otel-cardinality-${DATABASE} > /dev/null 2>&1
rm -f "$TEMP_LOG"

echo -e "\n${GREEN}✓ Cardinality analysis complete!${NC}"

# Save detailed report
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="cardinality_report_${DATABASE}_${TIMESTAMP}.txt"
cat > "$REPORT_FILE" << EOF
Metric Cardinality Analysis Report
Database: $DATABASE
Timestamp: $(date)
Analysis Duration: $DURATION seconds

Summary:
- Total Active Series: $TOTAL_SERIES
- Estimated Memory Usage: $ESTIMATED_MEMORY_MB MB

High-Cardinality Metrics:
$(curl -s http://localhost:8888/metrics | grep -E "^[a-z].*{" | cut -d'{' -f1 | sort | uniq -c | sort -nr | head -20)

Configuration: $CONFIG_FILE

Recommendations:
$(if [ $TOTAL_SERIES -gt 10000 ]; then
    echo "- Implement aggressive filtering"
    echo "- Consider metric sampling"
    echo "- Review attribute inclusion"
elif [ $TOTAL_SERIES -gt 5000 ]; then
    echo "- Monitor growth trends"
    echo "- Plan for optimization"
else
    echo "- Current cardinality is acceptable"
fi)
EOF

echo -e "\nDetailed report saved to: ${YELLOW}$REPORT_FILE${NC}"