#!/bin/bash
set -e

# NRDB Query Simulation for Local Testing
# Simulates NRQL queries against local JSON metrics export

echo "=== NRDB Query Simulation ==="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Input metrics file
METRICS_FILE="${1:-/tmp/e2e-metrics.json}"
if [ ! -f "$METRICS_FILE" ]; then
    echo -e "${RED}Metrics file not found: $METRICS_FILE${NC}"
    exit 1
fi

echo "Using metrics from: $METRICS_FILE"
echo ""

# Helper function to simulate NRQL queries
simulate_query() {
    local query_name="$1"
    local jq_command="$2"
    
    echo -e "${BLUE}Query: $query_name${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    result=$(eval "$jq_command" 2>/dev/null || echo "Error executing query")
    echo "$result"
    echo ""
}

# 1. Count total metrics
simulate_query "SELECT count(*) FROM Metric" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | .name' $METRICS_FILE | wc -l"

# 2. List unique metric names
simulate_query "SELECT uniques(metricName) FROM Metric" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE | sort | uniq"

# 3. PostgreSQL metrics summary
simulate_query "SELECT metricName, count(*) FROM Metric WHERE metricName LIKE 'postgresql%' FACET metricName" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name | startswith(\"postgresql\")) | .name' $METRICS_FILE | sort | uniq -c | awk '{print \$2 \": \" \$1 \" data points\"}'"

# 4. MySQL metrics summary
simulate_query "SELECT metricName, count(*) FROM Metric WHERE metricName LIKE 'mysql%' FACET metricName" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name | startswith(\"mysql\")) | .name' $METRICS_FILE | sort | uniq -c | awk '{print \$2 \": \" \$1 \" data points\"}'"

# 5. Test attributes validation
simulate_query "SELECT count(*) FROM Metric WHERE test.environment = 'e2e'" \
    "jq '[.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[] | select(.attributes[]? | select(.key == \"test.environment\" and .value.stringValue == \"e2e\"))] | length' $METRICS_FILE"

# 6. Database size metrics
simulate_query "SELECT latest(postgresql.db_size) FROM Metric" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name == \"postgresql.db_size\") | .sum.dataPoints[-1] | \"Value: \" + (.asInt // .asDouble | tostring) + \" bytes\"' $METRICS_FILE | head -1"

# 7. MySQL thread connections
simulate_query "SELECT latest(mysql.threads) FROM Metric WHERE thread.state = 'connected'" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name == \"mysql.threads\") | .sum.dataPoints[] | select(.attributes[]? | select(.key == \"mysql.thread_state\" and .value.stringValue == \"connected\")) | \"Connected threads: \" + (.asInt // .asDouble | tostring)' $METRICS_FILE | head -1"

# 8. Metric units and descriptions
simulate_query "SELECT metricName, unit, description FROM Metric LIMIT 5" \
    "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | \"\\(.name):\\n  Unit: \\(.unit)\\n  Description: \\(.description)\"' $METRICS_FILE | head -15"

# 9. Time range of data
simulate_query "SELECT min(timestamp), max(timestamp) FROM Metric" \
    "echo \"Time range:\"; jq -r '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[] | .timeUnixNano' $METRICS_FILE | sort -n | (head -1; tail -1) | xargs -I {} date -r \$(echo {} | cut -c1-10) '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo 'Unable to parse timestamps'"

# 10. Resource attributes
simulate_query "SELECT uniques(postgresql.database.name) FROM Metric" \
    "jq -r '.resourceMetrics[] | select(.resource.attributes[]?.value.stringValue | contains(\"postgres\"))? | .resource.attributes[] | select(.key == \"postgresql.database.name\")? | .value.stringValue' $METRICS_FILE | sort | uniq"

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}NRDB Query Simulation Complete${NC}"
echo ""
echo "Note: This is a local simulation of NRQL queries using jq."
echo "Actual NRDB queries would be more powerful and support:"
echo "- Time series aggregations"
echo "- Complex filtering and grouping"
echo "- Mathematical operations"
echo "- Alert conditions"
echo "- Dashboard visualizations"