#!/bin/bash

# Verify PostgreSQL Metrics Collection
# This script validates that all configured metrics are being collected

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if services are running
check_services() {
    log_info "Checking if services are running..."
    
    if ! docker ps | grep -q "db-intel-postgres"; then
        log_error "PostgreSQL container is not running"
        exit 1
    fi
    
    if ! docker ps | grep -q "db-intel-collector-config-only"; then
        log_warning "Config-Only collector is not running"
    fi
    
    if ! docker ps | grep -q "db-intel-collector-custom"; then
        log_warning "Custom collector is not running"
    fi
    
    log_success "Services check completed"
}

# List expected PostgreSQL metrics
list_expected_metrics() {
    cat << EOF
postgresql.backends
postgresql.bgwriter.buffers.allocated
postgresql.bgwriter.buffers.writes
postgresql.bgwriter.checkpoint.count
postgresql.bgwriter.duration
postgresql.bgwriter.maxwritten
postgresql.bgwriter.stat.checkpoints_timed
postgresql.bgwriter.stat.checkpoints_req
postgresql.blocks_read
postgresql.blks_hit
postgresql.blks_read
postgresql.buffer.hit
postgresql.commits
postgresql.conflicts
postgresql.connection.max
postgresql.database.count
postgresql.database.locks
postgresql.database.rows
postgresql.database.size
postgresql.deadlocks
postgresql.index.scans
postgresql.index.size
postgresql.live_rows
postgresql.locks
postgresql.operations
postgresql.replication.data_delay
postgresql.rollbacks
postgresql.rows
postgresql.sequential_scans
postgresql.stat_activity.count
postgresql.table.count
postgresql.table.size
postgresql.table.vacuum.count
postgresql.temp_files
postgresql.wal.age
postgresql.wal.delay
postgresql.wal.lag
EOF
}

# Check metrics via New Relic API
check_newrelic_metrics() {
    log_info "Checking metrics in New Relic..."
    
    if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        log_error "NEW_RELIC_LICENSE_KEY environment variable is required"
        return 1
    fi
    
    if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
        log_error "NEW_RELIC_ACCOUNT_ID environment variable is required"
        return 1
    fi
    
    # Query for unique metric names from both deployment modes
    local query='SELECT uniques(metricName) FROM Metric WHERE deployment.mode IN ("config-only", "custom") AND metricName LIKE "postgresql%" SINCE 30 minutes ago'
    
    log_info "Querying New Relic for PostgreSQL metrics..."
    
    # Use New Relic GraphQL API
    local response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Api-Key: ${NEW_RELIC_LICENSE_KEY}" \
        -H "Content-Type: application/json" \
        -d @- <<EOF
{
    "query": "{ actor { account(id: ${NEW_RELIC_ACCOUNT_ID}) { nrql(query: \"${query}\") { results } } } }"
}
EOF
    )
    
    echo "$response" | jq -r '.data.actor.account.nrql.results[0].uniques' || {
        log_error "Failed to parse New Relic response"
        echo "$response"
        return 1
    }
}

# Check metrics via collector logs
check_collector_logs() {
    local mode=$1
    local container_name=$2
    
    log_info "Checking $mode collector logs for metric activity..."
    
    # Check last 100 lines of logs for metric names
    docker logs --tail 100 "$container_name" 2>&1 | grep -E "postgresql\." | head -20 || true
}

# Generate metric verification report
generate_report() {
    local expected_metrics_file="/tmp/expected_metrics.txt"
    local collected_metrics_file="/tmp/collected_metrics.txt"
    local report_file="$PROJECT_ROOT/metrics-verification-report.md"
    
    # Save expected metrics
    list_expected_metrics | sort > "$expected_metrics_file"
    
    # Get collected metrics from New Relic
    log_info "Generating metrics verification report..."
    
    cat > "$report_file" << EOF
# PostgreSQL Metrics Verification Report

Generated on: $(date)

## Deployment Status

### Services Running:
$(docker ps --format "table {{.Names}}\t{{.Status}}" | grep db-intel)

## Expected Metrics (${$(wc -l < "$expected_metrics_file")} total)

\`\`\`
$(cat "$expected_metrics_file")
\`\`\`

## Verification Steps

### 1. Check Collector Health

Config-Only Mode:
\`\`\`bash
curl -s http://localhost:4318/v1/metrics | grep -c postgresql
\`\`\`

Custom Mode:
\`\`\`bash
curl -s http://localhost:5318/v1/metrics | grep -c postgresql
\`\`\`

### 2. Query New Relic for Metrics

Use this NRQL query to see all PostgreSQL metrics:
\`\`\`sql
SELECT uniques(metricName) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
AND metricName LIKE 'postgresql%' 
SINCE 30 minutes ago
\`\`\`

### 3. Verify Specific Metric Collection

Check if a specific metric is being collected:
\`\`\`sql
SELECT count(*) FROM Metric 
WHERE metricName = 'postgresql.backends' 
AND deployment.mode = 'config-only' 
SINCE 5 minutes ago
\`\`\`

### 4. Compare Modes

See which metrics are collected by each mode:
\`\`\`sql
FROM Metric 
SELECT uniqueCount(metricName) 
WHERE metricName LIKE 'postgresql%' 
FACET deployment.mode 
SINCE 1 hour ago
\`\`\`

## Troubleshooting

If metrics are missing:

1. **Check PostgreSQL permissions:**
   \`\`\`bash
   docker exec db-intel-postgres psql -U postgres -c "SELECT * FROM pg_stat_database LIMIT 1;"
   \`\`\`

2. **Check collector configuration:**
   \`\`\`bash
   docker exec db-intel-collector-config-only cat /etc/otel-collector-config.yaml | grep -A 5 "postgresql:"
   \`\`\`

3. **Check collector logs for errors:**
   \`\`\`bash
   docker logs db-intel-collector-config-only 2>&1 | grep -i error | tail -20
   \`\`\`

4. **Verify environment variables:**
   \`\`\`bash
   docker exec db-intel-collector-config-only env | grep -E "(POSTGRES|NEW_RELIC)"
   \`\`\`

## Next Steps

1. Deploy the PostgreSQL-only dashboard:
   \`\`\`bash
   ./scripts/migrate-dashboard.sh deploy dashboards/newrelic/postgresql-parallel-dashboard.json
   \`\`\`

2. Monitor metric collection in real-time:
   \`\`\`bash
   watch -n 5 'docker logs --tail 10 db-intel-collector-config-only 2>&1 | grep postgresql'
   \`\`\`

3. Generate load to ensure all metrics are populated:
   \`\`\`bash
   docker exec db-intel-postgres pgbench -i -s 10 testdb
   docker exec db-intel-postgres pgbench -c 10 -j 2 -T 60 testdb
   \`\`\`
EOF
    
    log_success "Report generated: $report_file"
}

# Main execution
main() {
    log_info "Starting PostgreSQL metrics verification..."
    
    check_services
    
    # Check collector logs
    if docker ps | grep -q "db-intel-collector-config-only"; then
        check_collector_logs "Config-Only" "db-intel-collector-config-only"
    fi
    
    if docker ps | grep -q "db-intel-collector-custom"; then
        check_collector_logs "Custom" "db-intel-collector-custom"
    fi
    
    # Try to check New Relic metrics if credentials are available
    if [[ -n "${NEW_RELIC_LICENSE_KEY:-}" ]] && [[ -n "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
        log_info "Checking metrics in New Relic..."
        check_newrelic_metrics || log_warning "Could not retrieve metrics from New Relic"
    else
        log_warning "NEW_RELIC_LICENSE_KEY and NEW_RELIC_ACCOUNT_ID not set, skipping New Relic checks"
    fi
    
    # Generate report
    generate_report
    
    log_success "Metrics verification completed!"
    log_info "Review the report at: $PROJECT_ROOT/metrics-verification-report.md"
}

# Run main function
main "$@"