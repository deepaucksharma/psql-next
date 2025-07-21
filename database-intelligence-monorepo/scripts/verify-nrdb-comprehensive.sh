#!/bin/bash

# Comprehensive NRDB Data Verification Script
# Consolidates all verification functionality into one script
# Supports multiple modes: quick, full, continuous, dashboard

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
NR_API_KEY="${NEW_RELIC_API_KEY}"
NR_ACCOUNT_ID="${NEW_RELIC_ACCOUNT_ID:-3630072}"
GRAPHQL_ENDPOINT="https://api.newrelic.com/graphql"
MODE="${1:-quick}"  # quick, full, continuous, dashboard, modules

# Module list
MODULES=(
    "core-metrics"
    "sql-intelligence"
    "wait-profiler"
    "anomaly-detector"
    "business-impact"
    "replication-monitor"
    "performance-advisor"
    "resource-monitor"
    "alert-manager"
    "canary-tester"
    "cross-signal-correlator"
)

# Display usage
usage() {
    cat << EOF
\033[0;34mDatabase Intelligence NRDB Verification Tool\033[0m

Usage: $0 [mode] [options]

Modes:
  quick       Quick verification of data flow (default)
  full        Comprehensive verification of all modules
  continuous  Continuous monitoring with alerts
  dashboard   Validate dashboard queries
  modules     Check individual module metrics
  
Options:
  -h, --help      Show this help message
  -t, --time      Time range in minutes (default: 5)
  -m, --module    Specific module to check
  -v, --verbose   Verbose output
  
Environment Variables:
  NEW_RELIC_API_KEY       Required: New Relic API Key
  NEW_RELIC_ACCOUNT_ID    Optional: Account ID (default: 3630072)
  
Examples:
  $0 quick                    # Quick data flow check
  $0 full                     # Full system verification
  $0 modules -m core-metrics  # Check specific module
  $0 continuous              # Start continuous monitoring
EOF
    exit 1
}

# Parse arguments
TIME_RANGE=5
SPECIFIC_MODULE=""
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help) usage ;;
        -t|--time) TIME_RANGE="$2"; shift 2 ;;
        -m|--module) SPECIFIC_MODULE="$2"; shift 2 ;;
        -v|--verbose) VERBOSE=true; shift ;;
        *) shift ;;
    esac
done

# Check API key
if [ -z "$NR_API_KEY" ]; then
    echo -e "\033[0;31mERROR: NEW_RELIC_API_KEY not set\033[0m"
    echo "Please set: export NEW_RELIC_API_KEY=your-api-key"
    exit 1
fi

# Function to query NRDB
query_nrdb() {
    local nrql_query=$1
    local description=$2
    
    if [ "$VERBOSE" = true ]; then
        echo -e "\033[0;36mQuery: $nrql_query\033[0m"
    fi
    
    local query=$(cat <<EOF
{
  "query": "{ actor { account(id: $NR_ACCOUNT_ID) { nrql(query: \"$nrql_query\") { results } } } }"
}
EOF
)
    
    local response=$(curl -s -X POST "$GRAPHQL_ENDPOINT" \
        -H 'Content-Type: application/json' \
        -H "API-Key: $NR_API_KEY" \
        -d "$query")
    
    echo "$response"
}

# Quick verification mode
quick_verification() {
    echo -e "\033[0;34m=== Quick NRDB Verification ===\033[0m"
    echo -e "Time Range: Last ${TIME_RANGE} minutes\n"
    
    # Overall metrics
    echo -e "\033[1;33mOverall Metrics:\033[0m"
    QUERY="SELECT count(*) FROM Metric WHERE module IS NOT NULL SINCE ${TIME_RANGE} minutes ago"
    response=$(query_nrdb "$QUERY" "Total metrics")
    
    total_count=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(results[0]['count'] if results else 0)
except:
    print(0)
")
    
    if [ "$total_count" -gt 0 ]; then
        echo -e "\033[0;32m✓\033[0m Total metrics: ${total_count}"
    else
        echo -e "\033[0;31m✗\033[0m No metrics found"
    fi
    
    # Active modules
    echo -e "\n\033[1;33mActive Modules:\033[0m"
    QUERY="SELECT count(*) FROM Metric WHERE module IS NOT NULL SINCE ${TIME_RANGE} minutes ago FACET module"
    response=$(query_nrdb "$QUERY" "Module activity")
    
    echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    active = 0
    for result in results:
        module = result.get('module', 'unknown')
        count = result.get('count', 0)
        if count > 0:
            active += 1
            print(f'\033[0;32m✓\033[0m {module}: {count} metrics')
    total_modules = ${#MODULES[@]}
    print(f'\nActive modules: {active}/{total_modules}')
except Exception as e:
    print(f'\033[0;31mError: {e}\033[0m')
"
}

# Full verification mode
full_verification() {
    echo -e "\033[0;34m=== Comprehensive NRDB Verification ===\033[0m"
    echo -e "Account: ${NR_ACCOUNT_ID} | Time Range: Last ${TIME_RANGE} minutes\n"
    
    # 1. Docker Container Status
    echo -e "\033[1;33m1. Docker Container Status\033[0m"
    echo "-----------------------------"
    
    running_count=0
    for module in "${MODULES[@]}"; do
        container_name="${module}-${module}-1"
        if [ "$module" = "replication-monitor" ] || [ "$module" = "resource-monitor" ]; then
            container_name="${module}-collector"
        fi
        
        if docker ps --format "{{.Names}}" | grep -q "$container_name"; then
            echo -e "\033[0;32m✓\033[0m $module: Running"
            ((running_count++))
        else
            echo -e "\033[0;31m✗\033[0m $module: Not running"
        fi
    done
    echo -e "\nContainers running: ${running_count}/${#MODULES[@]}"
    
    # 2. Module Metrics Summary
    echo -e "\n\033[1;33m2. Module Metrics Summary\033[0m"
    echo "-----------------------------"
    
    QUERY="SELECT count(*) FROM Metric WHERE module IS NOT NULL SINCE ${TIME_RANGE} minutes ago FACET module ORDER BY count DESC"
    response=$(query_nrdb "$QUERY" "Module metrics")
    
    echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results:
        print('\033[0;31m✗ No metrics found\033[0m')
    else:
        total = 0
        modules_with_data = 0
        for result in results:
            module = result.get('module', 'unknown')
            count = result.get('count', 0)
            total += count
            modules_with_data += 1
            
            if count > 1000:
                status = '\033[0;32m✓ Healthy\033[0m'
            elif count > 100:
                status = '\033[1;33m⚠ Limited\033[0m'
            else:
                status = '\033[0;31m✗ Minimal\033[0m'
            
            print(f'{module:25} {count:8,} metrics  {status}')
        
        print(f'\nTotal: {total:,} metrics from {modules_with_data} modules')
except Exception as e:
    print(f'\033[0;31mError: {e}\033[0m')
"
    
    # 3. Critical Metrics Status
    echo -e "\n\033[1;33m3. Critical Metrics Status\033[0m"
    echo "-----------------------------"
    
    critical_metrics=(
        "mysql.connection.count:Connection Count"
        "mysql.queries:Query Rate"
        "mysql.slow_queries:Slow Queries"
        "mysql.innodb.buffer_pool.pages.dirty:InnoDB Dirty Pages"
        "mysql.replication.time_behind_source:Replication Lag"
        "system.cpu.utilization:CPU Usage"
        "system.memory.usage:Memory Usage"
    )
    
    for metric_pair in "${critical_metrics[@]}"; do
        IFS=':' read -r metric_name metric_desc <<< "$metric_pair"
        
        QUERY="SELECT count(*) FROM Metric WHERE metricName = '$metric_name' SINCE ${TIME_RANGE} minutes ago"
        response=$(query_nrdb "$QUERY" "$metric_desc")
        
        count=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(results[0]['count'] if results else 0)
except:
    print(0)
")
        
        if [ "$count" -gt 0 ]; then
            echo -e "\033[0;32m✓\033[0m $metric_desc: Active ($count data points)"
        else
            echo -e "\033[0;31m✗\033[0m $metric_desc: No data"
        fi
    done
    
    # 4. MySQL Entity Status
    echo -e "\n\033[1;33m4. MySQL Entity Status\033[0m"
    echo "-----------------------------"
    
    QUERY="SELECT uniqueCount(entity.name) FROM Metric WHERE entity.type = 'MYSQL_INSTANCE' SINCE ${TIME_RANGE} minutes ago"
    response=$(query_nrdb "$QUERY" "MySQL entities")
    
    entity_count=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(results[0]['uniqueCount.entity.name'] if results else 0)
except:
    print(0)
")
    
    echo -e "MySQL Instances Reporting: ${entity_count}"
    
    # 5. Alert Status
    echo -e "\n\033[1;33m5. Alert/Anomaly Status\033[0m"
    echo "-----------------------------"
    
    QUERY="SELECT count(*) FROM Metric WHERE attributes.alert.severity IS NOT NULL SINCE ${TIME_RANGE} minutes ago FACET attributes.alert.severity"
    response=$(query_nrdb "$QUERY" "Alerts")
    
    echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if not results:
        print('\033[0;32m✓ No alerts\033[0m')
    else:
        for result in results:
            severity = result.get('attributes.alert.severity', 'unknown')
            count = result.get('count', 0)
            
            if severity == 'critical':
                color = '\033[0;31m'
            elif severity == 'warning':
                color = '\033[1;33m'
            else:
                color = '\033[0;32m'
            
            print(f'{color}⚠ {severity}: {count} alerts\033[0m')
except Exception as e:
    print(f'\033[0;32m✓ No alerts\033[0m')
"
    
    # 6. Data Freshness
    echo -e "\n\033[1;33m6. Data Freshness\033[0m"
    echo "-----------------------------"
    
    QUERY="SELECT latest(timestamp) FROM Metric WHERE module IS NOT NULL SINCE 1 hour ago FACET module"
    response=$(query_nrdb "$QUERY" "Data freshness")
    
    echo "$response" | python3 -c "
import sys, json
from datetime import datetime
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if results:
        now = datetime.now()
        for result in results:
            module = result.get('module', 'unknown')
            timestamp = result.get('latest.timestamp', 0)
            if timestamp:
                # Convert milliseconds to seconds
                last_seen = datetime.fromtimestamp(timestamp / 1000)
                age = (now - last_seen).total_seconds()
                
                if age < 60:
                    status = f'\033[0;32m✓ {int(age)}s ago\033[0m'
                elif age < 300:
                    status = f'\033[1;33m⚠ {int(age/60)}m ago\033[0m'
                else:
                    status = f'\033[0;31m✗ {int(age/60)}m ago\033[0m'
                
                print(f'{module:25} {status}')
except Exception as e:
    print(f'\033[0;31mError checking freshness: {e}\033[0m')
"
}

# Module-specific verification
module_verification() {
    local module="${SPECIFIC_MODULE:-all}"
    
    if [ "$module" = "all" ]; then
        echo -e "\033[0;34m=== Module-Specific Metrics ===\033[0m\n"
        modules_to_check=("${MODULES[@]}")
    else
        echo -e "\033[0;34m=== Metrics for $module ===\033[0m\n"
        modules_to_check=("$module")
    fi
    
    for mod in "${modules_to_check[@]}"; do
        echo -e "\033[1;33mModule: $mod\033[0m"
        echo "----------------------------------------"
        
        # Get metric count
        QUERY="SELECT count(*) FROM Metric WHERE module = '$mod' SINCE ${TIME_RANGE} minutes ago"
        response=$(query_nrdb "$QUERY" "$mod count")
        
        count=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(results[0]['count'] if results else 0)
except:
    print(0)
")
        
        if [ "$count" -eq 0 ]; then
            echo -e "\033[0;31m✗ No data found\033[0m\n"
            continue
        fi
        
        echo -e "\033[0;32m✓ Total metrics: $count\033[0m"
        
        # Get sample metrics
        QUERY="SELECT uniques(metricName) FROM Metric WHERE module = '$mod' SINCE ${TIME_RANGE} minutes ago LIMIT 20"
        response=$(query_nrdb "$QUERY" "$mod metrics")
        
        echo -e "\nSample metrics:"
        echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    
    if results and results[0].get('uniques.metricName'):
        metrics = results[0]['uniques.metricName']
        for i, metric in enumerate(metrics[:10]):
            print(f'  • {metric}')
        
        if len(metrics) > 10:
            print(f'  ... and {len(metrics) - 10} more')
except Exception as e:
    print(f'  \033[0;31mError listing metrics: {e}\033[0m')
"
        
        # Get latest values for key metrics
        if [ "$mod" = "core-metrics" ]; then
            echo -e "\nKey metric values:"
            key_metrics=("mysql.connection.count" "mysql.queries" "mysql.slow_queries")
            
            for km in "${key_metrics[@]}"; do
                QUERY="SELECT latest($km) FROM Metric WHERE module = '$mod' SINCE ${TIME_RANGE} minutes ago"
                response=$(query_nrdb "$QUERY" "$km latest")
                
                value=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    val = results[0].get('latest', 0) if results else 0
    print(f'{val:.2f}' if val else 'N/A')
except:
    print('N/A')
")
                
                echo -e "  • $km: $value"
            done
        fi
        
        echo ""
    done
}

# Dashboard query validation
dashboard_validation() {
    echo -e "\033[0;34m=== Dashboard Query Validation ===\033[0m\n"
    
    dashboard_queries=(
        "SELECT count(*) FROM Metric WHERE entity.type = 'MYSQL_INSTANCE' SINCE 5 minutes ago:MySQL Entity Check"
        "SELECT latest(mysql.uptime) FROM Metric WHERE entity.type = 'MYSQL_INSTANCE' SINCE 5 minutes ago:MySQL Uptime"
        "SELECT average(mysql.query.client.latency) FROM Metric WHERE module = 'sql-intelligence' SINCE 5 minutes ago:Query Latency"
        "SELECT uniqueCount(entity.name) FROM Metric WHERE entity.type IS NOT NULL SINCE 5 minutes ago:Entity Count"
        "SELECT count(*) FROM Metric WHERE module = 'anomaly-detector' AND metricName LIKE '%anomaly%' SINCE 10 minutes ago:Anomaly Detection"
        "SELECT rate(sum(mysql.queries), 1 minute) FROM Metric WHERE module = 'core-metrics' SINCE 5 minutes ago:Query Rate"
    )
    
    passed=0
    failed=0
    
    for query_pair in "${dashboard_queries[@]}"; do
        IFS=':' read -r query desc <<< "$query_pair"
        
        echo -e "\033[1;33mTesting: $desc\033[0m"
        
        response=$(query_nrdb "$query" "$desc")
        
        # Check for errors
        if echo "$response" | grep -q "errors"; then
            echo -e "\033[0;31m✗ Query failed\033[0m"
            ((failed++))
            if [ "$VERBOSE" = true ]; then
                echo "$response" | jq '.errors'
            fi
        else
            # Check for results
            has_results=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    if results and len(results) > 0:
        # Check if result has a non-zero/non-null value
        for result in results:
            for key, value in result.items():
                if value and value != 0:
                    print('true')
                    sys.exit(0)
    print('false')
except:
    print('false')
")
            
            if [ "$has_results" = "true" ]; then
                echo -e "\033[0;32m✓ Query passed\033[0m"
                ((passed++))
            else
                echo -e "\033[1;33m⚠ Query returned no data\033[0m"
                ((failed++))
            fi
        fi
        echo ""
    done
    
    echo -e "\033[0;34mSummary:\033[0m"
    echo -e "Passed: \033[0;32m$passed\033[0m"
    echo -e "Failed: \033[0;31m$failed\033[0m"
}

# Continuous monitoring mode
continuous_monitoring() {
    echo -e "\033[0;34m=== Continuous NRDB Monitoring ===\033[0m"
    echo -e "Press Ctrl+C to stop\n"
    
    local iteration=0
    local alert_threshold=5  # minutes
    
    while true; do
        ((iteration++))
        echo -e "\033[0;36m[$(date '+%Y-%m-%d %H:%M:%S')] Monitoring Cycle #$iteration\033[0m"
        
        # Check overall data flow
        QUERY="SELECT count(*) FROM Metric WHERE module IS NOT NULL SINCE 1 minute ago"
        response=$(query_nrdb "$QUERY" "Recent metrics")
        
        recent_count=$(echo "$response" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    print(results[0]['count'] if results else 0)
except:
    print(0)
")
        
        if [ "$recent_count" -eq 0 ]; then
            echo -e "\033[0;31m⚠ ALERT: No metrics received in last minute!\033[0m"
        else
            echo -e "\033[0;32m✓ Data flow active: $recent_count metrics in last minute\033[0m"
        fi
        
        # Check each module's freshness
        stale_modules=()
        for module in "${MODULES[@]}"; do
            QUERY="SELECT latest(timestamp) FROM Metric WHERE module = '$module' SINCE 10 minutes ago"
            response=$(query_nrdb "$QUERY" "$module freshness")
            
            age_minutes=$(echo "$response" | python3 -c "
import sys, json
from datetime import datetime
try:
    data = json.load(sys.stdin)
    results = data['data']['actor']['account']['nrql']['results']
    if results and results[0].get('latest.timestamp'):
        timestamp = results[0]['latest.timestamp']
        last_seen = datetime.fromtimestamp(timestamp / 1000)
        age = (datetime.now() - last_seen).total_seconds() / 60
        print(int(age))
    else:
        print(999)
except:
    print(999)
")
            
            if [ "$age_minutes" -gt "$alert_threshold" ]; then
                stale_modules+=("$module")
                echo -e "\033[0;31m⚠ $module: No data for ${age_minutes} minutes\033[0m"
            fi
        done
        
        # Summary
        active_modules=$((${#MODULES[@]} - ${#stale_modules[@]}))
        echo -e "\n\033[1;33mStatus: $active_modules/${#MODULES[@]} modules active\033[0m"
        
        if [ ${#stale_modules[@]} -gt 0 ]; then
            echo -e "\033[0;31mStale modules: ${stale_modules[*]}\033[0m"
        fi
        
        echo -e "\033[0;36mNext check in 30 seconds...\033[0m\n"
        sleep 30
    done
}

# Main execution
echo -e "\033[0;35m╔═══════════════════════════════════════════════════════╗\033[0m"
echo -e "\033[0;35m║     Database Intelligence NRDB Verification Tool      ║\033[0m"
echo -e "\033[0;35m╚═══════════════════════════════════════════════════════╝\033[0m"
echo ""

case $MODE in
    quick)
        quick_verification
        ;;
    full)
        full_verification
        ;;
    modules)
        module_verification
        ;;
    dashboard)
        dashboard_validation
        ;;
    continuous)
        continuous_monitoring
        ;;
    *)
        echo -e "\033[0;31mInvalid mode: $MODE\033[0m"
        usage
        ;;
esac

echo -e "\n\033[0;34mVerification complete.\033[0m"

# Provide next steps
echo -e "\n\033[1;33mNext Steps:\033[0m"
echo "1. View in New Relic UI:"
echo "   • Metrics Explorer: https://one.newrelic.com/metrics-explorer"
echo "   • Query Builder: https://one.newrelic.com/data-exploration"
echo ""
echo "2. Common NRQL queries:"
echo "   • All modules: FROM Metric SELECT * WHERE module IS NOT NULL"
echo "   • Specific module: FROM Metric SELECT * WHERE module = 'core-metrics'"
echo "   • MySQL metrics: FROM Metric SELECT * WHERE metricName LIKE 'mysql.%'"
echo ""
echo "3. For issues, check:"
echo "   • Container logs: docker logs <container-name>"
echo "   • Module config: cat modules/<module>/config/collector.yaml"
echo "   • Network connectivity: docker network ls"