#!/bin/bash
# Database Intelligence MVP - Docker Entrypoint Script
# Handles initialization and safety checks before starting collector

set -euo pipefail

# Get script directory and source common functions
# Note: In Docker, we'll copy the common.sh to /usr/local/lib/
if [ -f "/usr/local/lib/common.sh" ]; then
    source "/usr/local/lib/common.sh"
else
    # Fallback if common.sh not available in container
    log() { echo "[$(date '+%H:%M:%S')] $1" >&2; }
    log_info() { echo "[INFO] $1" >&2; }
    success() { echo "[SUCCESS] $1" >&2; }
    warning() { echo "[WARNING] $1" >&2; }
    error() { echo "[ERROR] $1" >&2; }
fi

# Configuration
OTEL_CONFIG_PATH="/etc/otel/config.yaml"
STATE_DIR="/var/lib/otel/storage"
LOG_DIR="/var/log/otel"

# Check if running as correct user
check_user() {
    current_uid=$(id -u)
    if [ "$current_uid" -ne 10001 ]; then
        error "Container must run as user 10001, currently running as $current_uid"
        exit 1
    fi
    success "Running as correct user (10001)"
}

# Validate required environment variables
validate_environment() {
    log_info "Validating environment variables..."
    
    # Required variables
    required_vars="NEW_RELIC_LICENSE_KEY OTLP_ENDPOINT PG_REPLICA_DSN"
    
    for var in $required_vars; do
        if [ -z "${!var:-}" ]; then
            error "Required environment variable $var is not set"
            exit 1
        fi
    done
    
    # Validate New Relic license key format
    if ! echo "$NEW_RELIC_LICENSE_KEY" | grep -q "^NRAL-"; then
        warning "New Relic license key doesn't start with NRAL-. Please verify."
    fi
    
    # Validate PostgreSQL DSN format
    if ! echo "$PG_REPLICA_DSN" | grep -q "postgres://"; then
        error "PostgreSQL DSN must start with postgres://"
        exit 1
    fi
    
    # Check for replica indicators in DSN
    if ! echo "$PG_REPLICA_DSN" | grep -E "(replica|readonly|read-only)" > /dev/null; then
        warning "PostgreSQL DSN doesn't contain 'replica' or 'readonly'. Ensure you're connecting to a read replica!"
    fi
    
    success "Environment variables validated"
}

# Test database connectivity
test_database_connectivity() {
    log_info "Testing database connectivity..."
    
    # Test PostgreSQL connection
    if command -v psql > /dev/null; then
        if psql "$PG_REPLICA_DSN" -c "SELECT 1;" > /dev/null 2>&1; then
            success "PostgreSQL connection successful"
        else
            error "Failed to connect to PostgreSQL. Check your PG_REPLICA_DSN"
            return 1
        fi
    else
        warning "psql not available, skipping PostgreSQL connection test"
    fi
    
    # Test MySQL connection if configured
    if [ -n "${MYSQL_READONLY_DSN:-}" ] && command -v mysql > /dev/null; then
        # Parse MySQL DSN
        mysql_host=$(echo "$MYSQL_READONLY_DSN" | sed 's/.*@tcp(\([^:]*\):.*/\1/')
        mysql_port=$(echo "$MYSQL_READONLY_DSN" | sed 's/.*@tcp([^:]*:\([0-9]*\)).*/\1/')
        mysql_user=$(echo "$MYSQL_READONLY_DSN" | sed 's/\([^:]*\):.*/\1/')
        mysql_pass=$(echo "$MYSQL_READONLY_DSN" | sed 's/[^:]*:\([^@]*\)@.*/\1/')
        mysql_db=$(echo "$MYSQL_READONLY_DSN" | sed 's/.*\/\([^?]*\).*/\1/')
        
        if mysql -h"$mysql_host" -P"$mysql_port" -u"$mysql_user" -p"$mysql_pass" "$mysql_db" -e "SELECT 1;" > /dev/null 2>&1; then
            log_success "MySQL connection successful"
        else
            log_warning "Failed to connect to MySQL. Check your MYSQL_READONLY_DSN"
        fi
    fi
}

# Initialize storage directories
initialize_storage() {
    log_info "Initializing storage directories..."
    
    # Create directories if they don't exist
    mkdir -p "$STATE_DIR/sampling"
    mkdir -p "$STATE_DIR/compaction"
    mkdir -p "$LOG_DIR"
    
    # Set permissions
    chmod 755 "$STATE_DIR" "$LOG_DIR"
    chmod 755 "$STATE_DIR/sampling" "$STATE_DIR/compaction"
    
    # Check write permissions
    if [ ! -w "$STATE_DIR" ]; then
        log_error "Cannot write to state directory: $STATE_DIR"
        exit 1
    fi
    
    if [ ! -w "$LOG_DIR" ]; then
        log_error "Cannot write to log directory: $LOG_DIR"
        exit 1
    fi
    
    log_success "Storage directories initialized"
}

# Validate configuration file
validate_configuration() {
    log_info "Validating OpenTelemetry configuration..."
    
    if [ ! -f "$OTEL_CONFIG_PATH" ]; then
        log_error "Configuration file not found: $OTEL_CONFIG_PATH"
        exit 1
    fi
    
    if [ ! -r "$OTEL_CONFIG_PATH" ]; then
        log_error "Configuration file not readable: $OTEL_CONFIG_PATH"
        exit 1
    fi
    
    # Validate YAML syntax
    if command -v otelcol-contrib > /dev/null; then
        if otelcol-contrib validate --config="$OTEL_CONFIG_PATH" > /dev/null 2>&1; then
            log_success "Configuration file is valid"
        else
            log_error "Configuration file validation failed"
            otelcol-contrib validate --config="$OTEL_CONFIG_PATH"
            exit 1
        fi
    else
        log_warning "otelcol-contrib not available for config validation"
    fi
}

# Check system resources
check_system_resources() {
    log_info "Checking system resources..."
    
    # Check available memory
    available_memory=$(awk '/MemAvailable/ {print $2}' /proc/meminfo 2>/dev/null || echo "0")
    if [ "$available_memory" -lt 524288 ]; then  # 512MB in KB
        log_warning "Less than 512MB memory available. Collector may experience issues."
    fi
    
    # Check disk space
    state_dir_space=$(df "$STATE_DIR" | awk 'NR==2 {print $4}')
    if [ "$state_dir_space" -lt 1048576 ]; then  # 1GB in KB
        log_warning "Less than 1GB disk space available for state storage"
    fi
    
    log_success "System resource check completed"
}

# Setup signal handlers for graceful shutdown
setup_signal_handlers() {
    # Function to handle shutdown signals
    shutdown_handler() {
        log_info "Received shutdown signal, stopping collector gracefully..."
        if [ -n "${collector_pid:-}" ]; then
            kill -TERM "$collector_pid" 2>/dev/null || true
            wait "$collector_pid" 2>/dev/null || true
        fi
        log_info "Collector stopped"
        exit 0
    }
    
    # Register signal handlers
    trap 'shutdown_handler' TERM INT
}

# Start the OpenTelemetry Collector
start_collector() {
    log_info "Starting Database Intelligence collector..."
    
    # Default arguments
    default_args="--config=$OTEL_CONFIG_PATH"
    
    # Add any additional arguments passed to the script
    collector_args="$default_args $*"
    
    log_info "Starting collector with args: $collector_args"
    
    # Start collector in background to handle signals
    otelcol-contrib $collector_args &
    collector_pid=$!
    
    log_success "Collector started with PID: $collector_pid"
    
    # Wait for collector to finish
    wait "$collector_pid"
}

# Health check function
health_check() {
    log_info "Performing initial health check..."
    
    # Wait for collector to start
    sleep 5
    
    # Check health endpoint
    if command -v wget > /dev/null; then
        if wget --quiet --tries=1 --spider http://localhost:13133/ 2>/dev/null; then
            log_success "Health check passed"
        else
            log_warning "Health check failed - collector may still be starting"
        fi
    elif command -v curl > /dev/null; then
        if curl -f http://localhost:13133/ > /dev/null 2>&1; then
            log_success "Health check passed"
        else
            log_warning "Health check failed - collector may still be starting"
        fi
    else
        log_warning "No HTTP client available for health check"
    fi
}

# Print startup information
print_startup_info() {
    log_info "Database Intelligence MVP Collector"
    log_info "Version: mvp-1.0"
    log_info "Deployment: Docker"
    log_info "Environment: ${DEPLOYMENT_ENV:-production}"
    echo ""
    log_info "Endpoints:"
    echo "  Health:  http://localhost:13133/"
    echo "  Metrics: http://localhost:8888/metrics"
    echo "  Debug:   http://localhost:55679/debug/"
    echo ""
    log_info "State directory: $STATE_DIR"
    log_info "Log directory: $LOG_DIR"
    echo ""
}

# Main function
main() {
    print_startup_info
    
    # Run all checks
    check_user
    validate_environment
    initialize_storage
    validate_configuration
    check_system_resources
    
    # Test connectivity (optional, can be disabled with env var)
    if [ "${SKIP_CONNECTIVITY_TEST:-false}" != "true" ]; then
        test_database_connectivity
    fi
    
    # Setup signal handling
    setup_signal_handlers
    
    # Start collector
    start_collector "$@"
}

# Run main function with all arguments
main "$@"