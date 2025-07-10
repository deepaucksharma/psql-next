#!/bin/bash
# Database Intelligence MVP - Common Shell Functions Library
# This library provides shared functions used across multiple scripts

# Ensure this library is sourced, not executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    echo "This script should be sourced, not executed directly"
    exit 1
fi

# ====================
# Color definitions
# ====================
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export PURPLE='\033[0;35m'
export CYAN='\033[0;36m'
export NC='\033[0m' # No Color

# ====================
# Logging functions
# ====================
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_debug() {
    if [[ "${DEBUG:-false}" == "true" ]] || [[ "${LOG_LEVEL:-info}" == "debug" ]]; then
        echo -e "${PURPLE}[DEBUG]${NC} $1"
    fi
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

# ====================
# Script utilities
# ====================

# Get the absolute path of the project root
get_project_root() {
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)"
    # Navigate up until we find a marker file (like README.md or .git)
    while [[ ! -f "$script_dir/README.md" ]] && [[ "$script_dir" != "/" ]]; do
        script_dir="$(dirname "$script_dir")"
    done
    echo "$script_dir"
}

# Check if running as root
check_not_root() {
    if [[ $EUID -eq 0 ]]; then
        error "This script should not be run as root"
        return 1
    fi
    return 0
}

# ====================
# Dependency checking
# ====================

# Check if a command exists
command_exists() {
    command -v "$1" &> /dev/null
}

# Check for required commands
check_required_commands() {
    local commands=("$@")
    local missing=()
    
    for cmd in "${commands[@]}"; do
        if ! command_exists "$cmd"; then
            missing+=("$cmd")
        fi
    done
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required commands: ${missing[*]}"
        return 1
    fi
    
    return 0
}

# ====================
# Environment functions
# ====================

# Load environment file
load_env_file() {
    local env_file="${1:-$(get_project_root)/.env}"
    
    if [[ ! -f "$env_file" ]]; then
        error "Environment file not found: $env_file"
        return 1
    fi
    
    # Export all variables from .env file
    set -a
    source "$env_file"
    set +a
    
    log_debug "Loaded environment from $env_file"
    return 0
}

# Validate environment variable exists and is not empty
validate_env_var() {
    local var_name="$1"
    local var_value="${!var_name:-}"
    
    if [[ -z "$var_value" ]]; then
        error "$var_name is not set or empty"
        return 1
    fi
    
    return 0
}

# Validate multiple environment variables
validate_env_vars() {
    local vars=("$@")
    local all_valid=true
    
    for var in "${vars[@]}"; do
        if ! validate_env_var "$var"; then
            all_valid=false
        fi
    done
    
    [[ "$all_valid" == "true" ]]
}

# ====================
# Database validation
# ====================

# Test PostgreSQL connection
test_postgresql_connection() {
    local dsn="${1:-$PG_REPLICA_DSN}"
    
    if [[ -z "$dsn" ]]; then
        warning "PostgreSQL DSN not provided"
        return 1
    fi
    
    if ! command_exists psql; then
        warning "psql not installed - cannot test PostgreSQL connection"
        return 2
    fi
    
    log_debug "Testing PostgreSQL connection..."
    if psql "$dsn" -c "SELECT 1;" &> /dev/null; then
        success "PostgreSQL connection successful"
        return 0
    else
        error "PostgreSQL connection failed"
        return 1
    fi
}

# Check PostgreSQL prerequisites
check_postgresql_prerequisites() {
    local dsn="${1:-$PG_REPLICA_DSN}"
    
    if [[ -z "$dsn" ]]; then
        return 1
    fi
    
    # Check pg_stat_statements
    log_debug "Checking pg_stat_statements extension..."
    if psql "$dsn" -c "SELECT * FROM pg_stat_statements LIMIT 1;" &> /dev/null; then
        success "pg_stat_statements is available"
    else
        error "pg_stat_statements is not available"
        echo "  To enable: CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
        return 1
    fi
    
    # Check if connected to replica
    log_debug "Checking if connected to replica..."
    local is_replica=$(psql "$dsn" -t -c "SELECT pg_is_in_recovery();" 2>/dev/null | xargs)
    if [[ "$is_replica" == "t" ]]; then
        success "Connected to read replica"
    else
        warning "Not connected to a read replica - ensure you're not using primary!"
    fi
    
    return 0
}

# Test MySQL connection
test_mysql_connection() {
    local dsn="${1:-$MYSQL_READONLY_DSN}"
    
    if [[ -z "$dsn" ]]; then
        warning "MySQL DSN not provided"
        return 1
    fi
    
    if ! command_exists mysql; then
        warning "mysql client not installed - cannot test MySQL connection"
        return 2
    fi
    
    # Parse MySQL DSN (format: user:pass@tcp(host:port)/database?params)
    if [[ "$dsn" =~ ^([^:]+):([^@]+)@tcp\(([^:]+):([0-9]+)\)/([^?]+) ]]; then
        local user="${BASH_REMATCH[1]}"
        local pass="${BASH_REMATCH[2]}"
        local host="${BASH_REMATCH[3]}"
        local port="${BASH_REMATCH[4]}"
        local db="${BASH_REMATCH[5]}"
        
        log_debug "Testing MySQL connection..."
        if mysql -h "$host" -P "$port" -u "$user" -p"$pass" "$db" -e "SELECT 1;" &> /dev/null; then
            success "MySQL connection successful"
            return 0
        else
            error "MySQL connection failed"
            return 1
        fi
    else
        error "Invalid MySQL DSN format"
        return 1
    fi
}

# Check MySQL prerequisites
check_mysql_prerequisites() {
    local dsn="${1:-$MYSQL_READONLY_DSN}"
    
    if [[ -z "$dsn" ]]; then
        return 1
    fi
    
    # Parse DSN
    if [[ "$dsn" =~ ^([^:]+):([^@]+)@tcp\(([^:]+):([0-9]+)\)/([^?]+) ]]; then
        local user="${BASH_REMATCH[1]}"
        local pass="${BASH_REMATCH[2]}"
        local host="${BASH_REMATCH[3]}"
        local port="${BASH_REMATCH[4]}"
        local db="${BASH_REMATCH[5]}"
        
        # Check performance schema
        log_debug "Checking performance schema..."
        local perf_schema=$(mysql -h "$host" -P "$port" -u "$user" -p"$pass" "$db" \
            -e "SELECT VARIABLE_VALUE FROM performance_schema.global_variables WHERE VARIABLE_NAME='performance_schema';" \
            2>/dev/null | tail -1)
        
        if [[ "$perf_schema" == "ON" ]]; then
            success "Performance schema is enabled"
        else
            error "Performance schema is not enabled"
            return 1
        fi
    fi
    
    return 0
}

# ====================
# Network utilities
# ====================

# Test endpoint connectivity
test_endpoint() {
    local host="$1"
    local port="$2"
    local timeout="${3:-5}"
    
    if command_exists nc; then
        if nc -z -w "$timeout" "$host" "$port" &> /dev/null; then
            return 0
        fi
    elif command_exists telnet; then
        if timeout "$timeout" telnet "$host" "$port" </dev/null 2>&1 | grep -q "Connected"; then
            return 0
        fi
    elif command_exists curl; then
        if curl -s --connect-timeout "$timeout" "telnet://${host}:${port}" &> /dev/null; then
            return 0
        fi
    else
        warning "No network testing tool available (nc, telnet, or curl)"
        return 2
    fi
    
    return 1
}

# ====================
# Service management
# ====================

# Wait for service to be ready
wait_for_service() {
    local url="$1"
    local timeout="${2:-30}"
    local interval="${3:-2}"
    
    log "Waiting for service at $url..."
    
    local elapsed=0
    while [[ $elapsed -lt $timeout ]]; do
        if curl -f -s "$url" &> /dev/null; then
            success "Service is ready"
            return 0
        fi
        
        echo -n "."
        sleep "$interval"
        elapsed=$((elapsed + interval))
    done
    
    echo ""
    error "Service failed to become ready within ${timeout}s"
    return 1
}

# Check if Docker container is running
is_container_running() {
    local container_name="$1"
    
    if ! command_exists docker; then
        error "Docker not installed"
        return 2
    fi
    
    docker ps --format "{{.Names}}" | grep -q "^${container_name}$"
}

# ====================
# Validation utilities
# ====================

# Validate New Relic license key format
validate_license_key() {
    local key="$1"
    
    # US format: 40 characters
    # EU format: eu01xx + 34 characters
    if [[ "$key" =~ ^[a-zA-Z0-9]{40}$ ]] || [[ "$key" =~ ^eu01xx[a-zA-Z0-9]{34}$ ]]; then
        return 0
    else
        return 1
    fi
}

# Validate PostgreSQL DSN format
validate_postgresql_dsn() {
    local dsn="$1"
    
    if [[ "$dsn" =~ ^postgres://[^:]+:[^@]+@[^:]+:[0-9]+/[^?]+(\?.*)?$ ]]; then
        return 0
    else
        return 1
    fi
}

# Validate MySQL DSN format
validate_mysql_dsn() {
    local dsn="$1"
    
    if [[ "$dsn" =~ ^[^:]+:[^@]+@tcp\([^:]+:[0-9]+\)/[^?]+ ]]; then
        return 0
    else
        return 1
    fi
}

# ====================
# Report generation
# ====================

# Generate summary report
generate_summary() {
    local title="$1"
    local passed="${2:-0}"
    local failed="${3:-0}"
    local warnings="${4:-0}"
    
    echo ""
    echo "====================================="
    echo "$title"
    echo "====================================="
    echo ""
    
    if [[ $failed -eq 0 ]] && [[ $warnings -eq 0 ]]; then
        echo -e "${GREEN}All checks passed!${NC}"
    elif [[ $failed -eq 0 ]]; then
        echo -e "${YELLOW}Passed with warnings${NC}"
    else
        echo -e "${RED}Failed${NC}"
    fi
    
    echo ""
    echo "Passed:   $passed"
    echo "Failed:   $failed"
    echo "Warnings: $warnings"
    echo ""
    echo "Timestamp: $(date)"
    echo ""
}

# ====================
# Cleanup utilities
# ====================

# Setup trap for cleanup
setup_cleanup_trap() {
    local cleanup_function="$1"
    trap "$cleanup_function" EXIT INT TERM
}

# Common cleanup function
cleanup() {
    local exit_code=$?
    log_debug "Cleaning up..."
    
    # Add any common cleanup tasks here
    
    exit $exit_code
}