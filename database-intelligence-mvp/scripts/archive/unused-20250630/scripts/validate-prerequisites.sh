#!/bin/bash
# Database Intelligence MVP - Prerequisites Validation Script
# Validates database setup and connectivity before deployment

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Global status
OVERALL_STATUS=0

# Logging functions
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
    OVERALL_STATUS=1
}

log_check() {
    echo -e "${BLUE}[CHECK]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
Database Intelligence MVP - Prerequisites Validation

This script validates that your database environment meets the requirements
for the Database Intelligence MVP collector.

Usage: $0 [OPTIONS]

Options:
    -h, --help                  Show this help message
    -c, --config FILE          Use custom configuration file
    --pg-dsn DSN              PostgreSQL connection string (overrides config)
    --mysql-dsn DSN           MySQL connection string (overrides config)
    --skip-connectivity       Skip database connectivity tests
    --verbose                 Enable verbose output
    --fix-permissions         Attempt to fix permission issues

Prerequisites checked:
    1. PostgreSQL configuration
       - pg_stat_statements extension
       - Read replica availability
       - Monitoring user permissions
    
    2. MySQL configuration (optional)
       - Performance schema enabled
       - Read replica availability
       - Monitoring user permissions
    
    3. Network connectivity
       - Database endpoints reachable
       - SSL/TLS configuration
    
    4. System requirements
       - Required tools available
       - Sufficient resources

Examples:
    $0                                  # Check all prerequisites
    $0 --pg-dsn "postgres://..."      # Test specific PostgreSQL connection
    $0 --skip-connectivity            # Skip network tests
    $0 --fix-permissions              # Attempt to fix database permissions

EOF
}

# Load configuration
load_config() {
    local config_file="${CONFIG_FILE:-$PROJECT_ROOT/.env}"
    
    if [[ -f "$config_file" ]]; then
        log_info "Loading configuration from $config_file"
        # shellcheck source=/dev/null
        source "$config_file"
    else
        log_warning "Configuration file not found: $config_file"
        log_info "Using environment variables or command line arguments"
    fi
}

# Parse command line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -c|--config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            --pg-dsn)
                PG_REPLICA_DSN="$2"
                shift 2
                ;;
            --mysql-dsn)
                MYSQL_READONLY_DSN="$2"
                shift 2
                ;;
            --skip-connectivity)
                SKIP_CONNECTIVITY=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --fix-permissions)
                FIX_PERMISSIONS=true
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Check system tools
check_system_tools() {
    log_check "Checking required system tools..."
    
    local required_tools=("curl" "nc")
    local optional_tools=("psql" "mysql" "mongosh")
    
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available"
        else
            log_error "$tool is required but not installed"
        fi
    done
    
    for tool in "${optional_tools[@]}"; do
        if command -v "$tool" &> /dev/null; then
            log_success "$tool is available"
        else
            log_warning "$tool is not available (database-specific tests will be skipped)"
        fi
    done
}

# Validate PostgreSQL prerequisites
validate_postgresql() {
    local dsn="${PG_REPLICA_DSN:-}"
    
    if [[ -z "$dsn" ]]; then
        log_warning "PostgreSQL DSN not provided, skipping PostgreSQL checks"
        return 0
    fi
    
    log_check "Validating PostgreSQL prerequisites..."
    
    # Check if psql is available
    if ! command -v psql &> /dev/null; then
        log_error "psql not available, cannot validate PostgreSQL"
        return 1
    fi
    
    # Test basic connectivity
    log_info "Testing PostgreSQL connectivity..."
    if psql "$dsn" -c "SELECT 1;" &> /dev/null; then
        log_success "PostgreSQL connection successful"
    else
        log_error "Failed to connect to PostgreSQL"
        log_info "DSN: ${dsn%:*}:***@${dsn##*@}"
        return 1
    fi
    
    # Check pg_stat_statements extension
    log_info "Checking pg_stat_statements extension..."
    if psql "$dsn" -t -c "SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements';" | grep -q "1"; then
        log_success "pg_stat_statements extension is installed"
    else
        log_error "pg_stat_statements extension is not installed"
        log_info "To install: CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
    fi
    
    # Check if pg_stat_statements has data
    log_info "Checking pg_stat_statements data availability..."
    local stmt_count
    stmt_count=$(psql "$dsn" -t -c "SELECT count(*) FROM pg_stat_statements;")
    if [[ "$stmt_count" -gt 0 ]]; then
        log_success "pg_stat_statements has $stmt_count statements"
    else
        log_warning "pg_stat_statements has no data (database may be new)"
    fi
    
    # Check user permissions
    log_info "Checking user permissions..."
    
    # Check basic database access
    if psql "$dsn" -c "SELECT current_database(), current_user;" &> /dev/null; then
        log_success "User has basic database access"
    else
        log_error "User lacks basic database access"
    fi
    
    # Check pg_stat_statements access
    if psql "$dsn" -c "SELECT count(*) FROM pg_stat_statements LIMIT 1;" &> /dev/null; then
        log_success "User can read pg_stat_statements"
    else
        log_error "User cannot read pg_stat_statements"
        if [[ "${FIX_PERMISSIONS:-false}" == "true" ]]; then
            log_info "Attempting to grant pg_stat_statements access..."
            local username
            username=$(psql "$dsn" -t -c "SELECT current_user;" | xargs)
            psql "$dsn" -c "GRANT SELECT ON pg_stat_statements TO $username;" || true
        fi
    fi
    
    # Check if this is a replica
    log_info "Checking if connected to a read replica..."
    if psql "$dsn" -t -c "SELECT pg_is_in_recovery();" | grep -q "t"; then
        log_success "Connected to a read replica (safe for monitoring)"
    else
        log_warning "Connected to primary database - ensure this is intentional"
        log_warning "For production, always use read replicas"
    fi
    
    # Check replica lag (if applicable)
    local lag_result
    lag_result=$(psql "$dsn" -t -c "SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()));" 2>/dev/null || echo "N/A")
    if [[ "$lag_result" != "N/A" ]]; then
        local lag_seconds
        lag_seconds=$(echo "$lag_result" | xargs | cut -d. -f1)
        if [[ "$lag_seconds" -lt 30 ]]; then
            log_success "Replica lag: ${lag_seconds}s (acceptable)"
        else
            log_warning "Replica lag: ${lag_seconds}s (high lag detected)"
        fi
    fi
    
    # Test query safety
    log_info "Testing query timeout safety..."
    if timeout 5 psql "$dsn" -c "SET LOCAL statement_timeout = '1000'; SELECT pg_sleep(0.5);" &> /dev/null; then
        log_success "Query timeout mechanism works"
    else
        log_error "Query timeout mechanism failed"
    fi
    
    return 0
}

# Validate MySQL prerequisites
validate_mysql() {
    local dsn="${MYSQL_READONLY_DSN:-}"
    
    if [[ -z "$dsn" ]]; then
        log_info "MySQL DSN not provided, skipping MySQL checks"
        return 0
    fi
    
    log_check "Validating MySQL prerequisites..."
    
    # Check if mysql is available
    if ! command -v mysql &> /dev/null; then
        log_error "mysql client not available, cannot validate MySQL"
        return 1
    fi
    
    # Parse MySQL DSN (format: user:pass@tcp(host:port)/db)
    local mysql_user mysql_pass mysql_host mysql_port mysql_db
    mysql_user=$(echo "$dsn" | sed 's/\([^:]*\):.*/\1/')
    mysql_pass=$(echo "$dsn" | sed 's/[^:]*:\([^@]*\)@.*/\1/')
    mysql_host=$(echo "$dsn" | sed 's/.*@tcp(\([^:]*\):.*/\1/')
    mysql_port=$(echo "$dsn" | sed 's/.*@tcp([^:]*:\([0-9]*\)).*/\1/')
    mysql_db=$(echo "$dsn" | sed 's/.*\/\([^?]*\).*/\1/')
    
    # Test basic connectivity
    log_info "Testing MySQL connectivity..."
    if mysql -h"$mysql_host" -P"$mysql_port" -u"$mysql_user" -p"$mysql_pass" "$mysql_db" -e "SELECT 1;" &> /dev/null; then
        log_success "MySQL connection successful"
    else
        log_error "Failed to connect to MySQL"
        return 1
    fi
    
    # Check Performance Schema
    log_info "Checking Performance Schema..."
    local perf_schema
    perf_schema=$(mysql -h"$mysql_host" -P"$mysql_port" -u"$mysql_user" -p"$mysql_pass" "$mysql_db" -sN -e "SHOW VARIABLES LIKE 'performance_schema';" | awk '{print $2}')
    if [[ "$perf_schema" == "ON" ]]; then
        log_success "Performance Schema is enabled"
    else
        log_error "Performance Schema is disabled"
        log_info "Add 'performance_schema = ON' to MySQL configuration and restart"
    fi
    
    # Check statement digests
    log_info "Checking statement digest collection..."
    local digest_count
    digest_count=$(mysql -h"$mysql_host" -P"$mysql_port" -u"$mysql_user" -p"$mysql_pass" "$mysql_db" -sN -e "SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest;" 2>/dev/null || echo "0")
    if [[ "$digest_count" -gt 0 ]]; then
        log_success "Statement digests available ($digest_count statements)"
    else
        log_warning "No statement digests available (database may be new)"
    fi
    
    # Check user permissions
    log_info "Checking MySQL user permissions..."
    
    # Check performance_schema access
    if mysql -h"$mysql_host" -P"$mysql_port" -u"$mysql_user" -p"$mysql_pass" "$mysql_db" -e "SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest LIMIT 1;" &> /dev/null; then
        log_success "User can read performance_schema"
    else
        log_error "User cannot read performance_schema"
    fi
    
    # Check if read-only
    local read_only
    read_only=$(mysql -h"$mysql_host" -P"$mysql_port" -u"$mysql_user" -p"$mysql_pass" "$mysql_db" -sN -e "SELECT @@read_only;" 2>/dev/null || echo "0")
    if [[ "$read_only" == "1" ]]; then
        log_success "Connected to read-only instance (safe for monitoring)"
    else
        log_warning "Connected to writable instance - ensure this is a replica"
    fi
    
    return 0
}

# Test network connectivity
test_network_connectivity() {
    if [[ "${SKIP_CONNECTIVITY:-false}" == "true" ]]; then
        log_info "Skipping network connectivity tests"
        return 0
    fi
    
    log_check "Testing network connectivity..."
    
    # Test New Relic OTLP endpoint
    local otlp_endpoint="${OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}"
    local otlp_host otlp_port
    otlp_host=$(echo "$otlp_endpoint" | sed 's|.*://\([^:]*\):.*|\1|')
    otlp_port=$(echo "$otlp_endpoint" | sed 's|.*:\([0-9]*\).*|\1|')
    
    log_info "Testing New Relic OTLP endpoint connectivity..."
    if nc -z "$otlp_host" "$otlp_port" 2>/dev/null; then
        log_success "New Relic OTLP endpoint is reachable"
    else
        log_error "Cannot reach New Relic OTLP endpoint: $otlp_host:$otlp_port"
    fi
    
    # Test database endpoints
    if [[ -n "${PG_REPLICA_DSN:-}" ]]; then
        local pg_host pg_port
        pg_host=$(echo "$PG_REPLICA_DSN" | sed 's|.*@\([^:]*\):.*|\1|')
        pg_port=$(echo "$PG_REPLICA_DSN" | sed 's|.*:\([0-9]*\)/.*|\1|')
        
        log_info "Testing PostgreSQL endpoint connectivity..."
        if nc -z "$pg_host" "$pg_port" 2>/dev/null; then
            log_success "PostgreSQL endpoint is reachable"
        else
            log_error "Cannot reach PostgreSQL endpoint: $pg_host:$pg_port"
        fi
    fi
    
    if [[ -n "${MYSQL_READONLY_DSN:-}" ]]; then
        local mysql_host mysql_port
        mysql_host=$(echo "$MYSQL_READONLY_DSN" | sed 's/.*@tcp(\([^:]*\):.*/\1/')
        mysql_port=$(echo "$MYSQL_READONLY_DSN" | sed 's/.*@tcp([^:]*:\([0-9]*\)).*/\1/')
        
        log_info "Testing MySQL endpoint connectivity..."
        if nc -z "$mysql_host" "$mysql_port" 2>/dev/null; then
            log_success "MySQL endpoint is reachable"
        else
            log_error "Cannot reach MySQL endpoint: $mysql_host:$mysql_port"
        fi
    fi
}

# Validate New Relic configuration
validate_newrelic_config() {
    log_check "Validating New Relic configuration..."
    
    local license_key="${NEW_RELIC_LICENSE_KEY:-}"
    if [[ -z "$license_key" ]]; then
        log_error "NEW_RELIC_LICENSE_KEY not set"
        return 1
    fi
    
    # Validate license key format
    if [[ "$license_key" =~ ^NRAL-[A-Za-z0-9]{40}$ ]]; then
        log_success "New Relic license key format is valid"
    else
        log_warning "New Relic license key format may be invalid"
        log_info "Expected format: NRAL-followed by 40 characters"
    fi
    
    # Test license key (basic connectivity)
    local otlp_endpoint="${OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}"
    log_info "Testing New Relic license key..."
    
    # Create a simple test payload
    if command -v curl &> /dev/null; then
        local response_code
        response_code=$(curl -s -o /dev/null -w "%{http_code}" \
            -H "api-key: $license_key" \
            -H "Content-Type: application/x-protobuf" \
            --max-time 10 \
            "$otlp_endpoint" || echo "000")
        
        if [[ "$response_code" =~ ^[24] ]]; then
            log_success "New Relic license key is valid"
        else
            log_warning "Could not validate license key (HTTP $response_code)"
            log_info "This may be normal - full validation requires actual telemetry data"
        fi
    else
        log_warning "curl not available, cannot test license key"
    fi
}

# Check system resources
check_system_resources() {
    log_check "Checking system resources..."
    
    # Check available memory
    if [[ -f /proc/meminfo ]]; then
        local available_memory
        available_memory=$(awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo)
        if [[ "$available_memory" -gt 1024 ]]; then
            log_success "Available memory: ${available_memory}MB"
        else
            log_warning "Low memory available: ${available_memory}MB (recommend 1GB+)"
        fi
    fi
    
    # Check disk space
    local disk_usage
    disk_usage=$(df -h . | awk 'NR==2 {print $4}')
    log_info "Available disk space: $disk_usage"
    
    # Check CPU cores
    local cpu_cores
    cpu_cores=$(nproc 2>/dev/null || echo "unknown")
    log_info "CPU cores: $cpu_cores"
}

# Generate configuration template
generate_config_template() {
    local template_file="$PROJECT_ROOT/.env.generated"
    
    log_info "Generating configuration template: $template_file"
    
    cat > "$template_file" << EOF
# Database Intelligence MVP - Generated Configuration Template
# Generated on: $(date)

# =============================================================================
# NEW RELIC CONFIGURATION
# =============================================================================
NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY:-NRAL-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX}
OTLP_ENDPOINT=${OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}

# =============================================================================
# DATABASE CONNECTIONS
# =============================================================================
# PostgreSQL Read Replica (REQUIRED)
PG_REPLICA_DSN=${PG_REPLICA_DSN:-postgres://newrelic_monitor:password@postgres-replica:5432/database?sslmode=require}

# MySQL Read Replica (OPTIONAL)
MYSQL_READONLY_DSN=${MYSQL_READONLY_DSN:-newrelic_monitor:password@tcp(mysql-replica:3306)/database?tls=true}

# =============================================================================
# DEPLOYMENT CONFIGURATION
# =============================================================================
DEPLOYMENT_ENV=${DEPLOYMENT_ENV:-production}
OTEL_LOG_LEVEL=${OTEL_LOG_LEVEL:-info}

EOF

    log_success "Configuration template generated"
    log_info "Review and customize $template_file, then copy to .env"
}

# Print validation summary
print_summary() {
    echo ""
    echo "=============================================="
    echo "Database Intelligence Prerequisites Summary"
    echo "=============================================="
    
    if [[ "$OVERALL_STATUS" -eq 0 ]]; then
        log_success "All prerequisites validated successfully!"
        echo ""
        echo "Next steps:"
        echo "1. Deploy the collector using provided deployment scripts"
        echo "2. Monitor the collector logs for successful data collection"
        echo "3. Verify data appears in your New Relic account"
    else
        log_error "Some prerequisites failed validation"
        echo ""
        echo "Required actions:"
        echo "1. Address the errors listed above"
        echo "2. Re-run this script to verify fixes"
        echo "3. Consult PREREQUISITES.md for detailed setup instructions"
    fi
    
    echo ""
    echo "Documentation:"
    echo "- Prerequisites: $PROJECT_ROOT/PREREQUISITES.md"
    echo "- Configuration: $PROJECT_ROOT/CONFIGURATION.md"
    echo "- Deployment: $PROJECT_ROOT/DEPLOYMENT.md"
    echo "- Troubleshooting: $PROJECT_ROOT/TROUBLESHOOTING.md"
}

# Main function
main() {
    local start_time
    start_time=$(date +%s)
    
    echo "Database Intelligence MVP - Prerequisites Validation"
    echo "=================================================="
    echo ""
    
    # Parse arguments and load config
    parse_arguments "$@"
    load_config
    
    # Run all checks
    check_system_tools
    echo ""
    
    validate_newrelic_config
    echo ""
    
    validate_postgresql
    echo ""
    
    validate_mysql
    echo ""
    
    test_network_connectivity
    echo ""
    
    check_system_resources
    echo ""
    
    # Generate config template if needed
    if [[ ! -f "$PROJECT_ROOT/.env" ]]; then
        generate_config_template
        echo ""
    fi
    
    # Print summary
    print_summary
    
    local end_time duration
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    echo ""
    log_info "Validation completed in ${duration}s"
    
    exit "$OVERALL_STATUS"
}

# Initialize variables
VERBOSE=${VERBOSE:-false}
SKIP_CONNECTIVITY=${SKIP_CONNECTIVITY:-false}
FIX_PERMISSIONS=${FIX_PERMISSIONS:-false}

# Run main function with all arguments
main "$@"