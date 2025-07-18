#!/bin/bash

# Monitor MySQL Wait Analysis in Real-Time
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Default values
REFRESH_INTERVAL=${REFRESH_INTERVAL:-5}
PROMETHEUS_URL=${PROMETHEUS_URL:-"http://localhost:9091/metrics"}
MODE=${1:-summary}

# Function to display header
display_header() {
    clear
    echo -e "${BLUE}=== MySQL Wait Analysis Monitor ===${NC}"
    echo -e "${CYAN}Mode: $MODE | Refresh: ${REFRESH_INTERVAL}s | Time: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
    echo "Press Ctrl+C to exit"
    echo ""
}

# Function to get top wait queries
show_top_waits() {
    echo -e "${YELLOW}Top Queries by Wait Time:${NC}"
    echo "----------------------------------------"
    
    # Query Prometheus endpoint
    curl -s "$PROMETHEUS_URL" | grep "mysql_gateway_mysql_query_wait_profile" | \
    grep -v "^#" | \
    awk -F'[{} ]' '{
        for(i=1; i<=NF; i++) {
            if($i ~ /query_hash=/) split($i, a, "\""); hash=a[2]
            if($i ~ /wait_category=/) split($i, b, "\""); category=b[2]
            if($i ~ /wait_severity=/) split($i, c, "\""); severity=c[2]
        }
        value=$NF
        print hash, category, severity, value
    }' | \
    sort -k4 -nr | \
    head -10 | \
    while read hash category severity value; do
        # Color code by severity
        case $severity in
            "critical") color=$RED ;;
            "high") color=$YELLOW ;;
            "medium") color=$CYAN ;;
            *) color=$NC ;;
        esac
        
        printf "${color}%-20s %-10s %-10s %10.2f ms${NC}\n" \
            "${hash:0:20}" "$category" "$severity" "$value"
    done
}

# Function to show advisories
show_advisories() {
    echo -e "\n${YELLOW}Active Performance Advisories:${NC}"
    echo "----------------------------------------"
    
    curl -s "$PROMETHEUS_URL" | grep "advisor_type=" | \
    grep -v "^#" | \
    awk -F'[{} ]' '{
        for(i=1; i<=NF; i++) {
            if($i ~ /advisor_type=/) split($i, a, "\""); type=a[2]
            if($i ~ /advisor_priority=/) split($i, b, "\""); priority=b[2]
            if($i ~ /query_hash=/) split($i, c, "\""); hash=c[2]
        }
        print type, priority, hash
    }' | \
    sort | uniq -c | \
    sort -nr | \
    head -10 | \
    while read count type priority hash; do
        # Color code by priority
        case $priority in
            "P1") color=$RED ;;
            "P2") color=$YELLOW ;;
            *) color=$CYAN ;;
        esac
        
        printf "${color}%-30s %s (%d occurrences)${NC}\n" \
            "$type" "$priority" "$count"
    done
}

# Function to show blocking chains
show_blocking() {
    echo -e "\n${YELLOW}Active Blocking Sessions:${NC}"
    echo "----------------------------------------"
    
    local blocks=$(curl -s "$PROMETHEUS_URL" | grep "mysql_blocking_active" | grep -v "^#")
    
    if [ -z "$blocks" ]; then
        echo "No active blocking detected"
    else
        echo "$blocks" | \
        awk -F'[{} ]' '{
            for(i=1; i<=NF; i++) {
                if($i ~ /waiting_thread=/) split($i, a, "\""); waiting=a[2]
                if($i ~ /blocking_thread=/) split($i, b, "\""); blocking=b[2]
                if($i ~ /lock_table=/) split($i, c, "\""); table=c[2]
            }
            duration=$NF
            print waiting, blocking, table, duration
        }' | \
        while read waiting blocking table duration; do
            printf "${RED}Thread %s blocked by %s on %s for %.1f seconds${NC}\n" \
                "$waiting" "$blocking" "$table" "$duration"
        done
    fi
}

# Function to show wait category distribution
show_wait_distribution() {
    echo -e "\n${YELLOW}Wait Category Distribution:${NC}"
    echo "----------------------------------------"
    
    local total=$(curl -s "$PROMETHEUS_URL" | \
        grep "mysql_gateway_mysql_query_wait_profile" | \
        grep -v "^#" | \
        awk '{sum+=$NF} END {print sum}')
    
    if [ -z "$total" ] || [ "$total" = "0" ]; then
        echo "No wait data available"
        return
    fi
    
    curl -s "$PROMETHEUS_URL" | \
    grep "mysql_gateway_mysql_query_wait_profile" | \
    grep -v "^#" | \
    awk -F'[{} ]' -v total="$total" '{
        for(i=1; i<=NF; i++) {
            if($i ~ /wait_category=/) split($i, a, "\""); category=a[2]
        }
        value=$NF
        sum[category]+=value
    } END {
        for(cat in sum) {
            pct = (sum[cat]/total)*100
            printf "%-15s %10.2f ms (%5.1f%%)\n", cat, sum[cat], pct
        }
    }' | sort -k2 -nr
}

# Function to show real-time stats
show_realtime_stats() {
    echo -e "\n${YELLOW}Real-Time Statistics:${NC}"
    echo "----------------------------------------"
    
    # Get collector stats
    local edge_accepted=$(curl -s http://localhost:8888/metrics | \
        grep "otelcol_receiver_accepted_metric_points" | \
        tail -1 | awk '{print $NF}')
    
    local gateway_sent=$(curl -s http://localhost:8889/metrics | \
        grep "otelcol_exporter_sent_metric_points" | \
        grep newrelic | tail -1 | awk '{print $NF}')
    
    printf "Edge Collector Metrics Received: %.0f\n" "${edge_accepted:-0}"
    printf "Gateway Metrics Sent to NR:      %.0f\n" "${gateway_sent:-0}"
}

# Function to run in continuous mode
continuous_mode() {
    while true; do
        display_header
        
        case $MODE in
            "summary")
                show_top_waits
                show_advisories
                show_wait_distribution
                ;;
            "blocking")
                show_blocking
                ;;
            "advisories")
                show_advisories
                ;;
            "waits")
                show_top_waits
                show_wait_distribution
                ;;
            "stats")
                show_realtime_stats
                ;;
            "all")
                show_top_waits
                show_advisories
                show_blocking
                show_wait_distribution
                show_realtime_stats
                ;;
        esac
        
        sleep $REFRESH_INTERVAL
    done
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [mode] [options]"
    echo ""
    echo "Modes:"
    echo "  summary    - Show top waits, advisories, and distribution (default)"
    echo "  blocking   - Show only blocking analysis"
    echo "  advisories - Show only performance advisories"
    echo "  waits      - Show only wait analysis"
    echo "  stats      - Show collector statistics"
    echo "  all        - Show all information"
    echo ""
    echo "Options:"
    echo "  REFRESH_INTERVAL=n  - Set refresh interval in seconds (default: 5)"
    echo "  PROMETHEUS_URL=url  - Set Prometheus endpoint URL"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run in summary mode"
    echo "  $0 blocking           # Show only blocking information"
    echo "  REFRESH_INTERVAL=10 $0 all  # Show all info, refresh every 10s"
}

# Main execution
main() {
    # Check if help requested
    if [ "$MODE" = "-h" ] || [ "$MODE" = "--help" ]; then
        show_usage
        exit 0
    fi
    
    # Validate mode
    case $MODE in
        summary|blocking|advisories|waits|stats|all)
            ;;
        *)
            echo "Invalid mode: $MODE"
            show_usage
            exit 1
            ;;
    esac
    
    # Check if services are running
    if ! curl -s "$PROMETHEUS_URL" > /dev/null; then
        echo -e "${RED}Error: Cannot connect to Prometheus endpoint${NC}"
        echo "Make sure the monitoring stack is running:"
        echo "  cd $PROJECT_ROOT"
        echo "  docker-compose up -d"
        exit 1
    fi
    
    # Run in continuous mode
    continuous_mode
}

# Handle Ctrl+C gracefully
trap 'echo -e "\n${GREEN}Monitoring stopped${NC}"; exit 0' INT

# Run main
main