#!/bin/bash
# Database Intelligence MVP - Comprehensive Validation Script
# Consolidates all validation logic from multiple scripts

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Validation counters
PASSED=0
FAILED=0
WARNINGS=0

# ====================
# Validation modes
# ====================
MODE="${1:-all}"
VERBOSE="${VERBOSE:-false}"

# ====================
# System validation
# ====================
validate_system() {
    log "Validating system requirements..."
    
    # Check not running as root
    if check_not_root; then
        ((PASSED++))
    else
        ((FAILED++))
    fi
    
    # Check required tools
    local required_tools=("docker" "curl")
    if check_required_commands "${required_tools[@]}"; then
        success "Required system tools available"
        ((PASSED++))
    else
        ((FAILED++))
    fi
    
    # Check optional tools
    local optional_tools=("psql" "mysql" "nc" "jq")
    for tool in "${optional_tools[@]}"; do
        if command_exists "$tool"; then
            success "$tool is available"
            ((PASSED++))
        else
            warning "$tool not found (optional)"
            ((WARNINGS++))
        fi
    done
}

# ====================
# Environment validation
# ====================
validate_environment() {
    log "Validating environment configuration..."
    
    # Load environment
    local project_root="$(get_project_root)"
    if load_env_file "$project_root/.env"; then
        success "Environment file loaded"
        ((PASSED++))
    else
        error "Failed to load environment file"
        ((FAILED++))
        return 1
    fi
    
    # Validate required variables
    local required_vars=(
        "NEW_RELIC_LICENSE_KEY"
        "OTLP_ENDPOINT"
        "DEPLOYMENT_ENV"
    )
    
    for var in "${required_vars[@]}"; do
        if validate_env_var "$var"; then
            success "$var is set"
            ((PASSED++))
        else
            ((FAILED++))
        fi
    done
    
    # Validate license key format
    if [[ -n "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        if [[ "$NEW_RELIC_LICENSE_KEY" == "your-license-key-here" ]]; then
            error "NEW_RELIC_LICENSE_KEY has not been updated from default"
            ((FAILED++))
        elif validate_license_key "$NEW_RELIC_LICENSE_KEY"; then
            success "License key format is valid"
            ((PASSED++))
        else
            warning "License key format may be incorrect"
            ((WARNINGS++))
        fi
    fi
    
    # Validate deployment environment
    case "${DEPLOYMENT_ENV:-}" in
        development|staging|production)
            success "DEPLOYMENT_ENV is valid: $DEPLOYMENT_ENV"
            ((PASSED++))
            ;;
        *)
            error "DEPLOYMENT_ENV must be one of: development, staging, production"
            ((FAILED++))
            ;;
    esac
    
    # Production-specific checks
    if [[ "${DEPLOYMENT_ENV:-}" == "production" ]]; then
        log "Running production-specific checks..."
        
        # Collection interval
        if [[ "${COLLECTION_INTERVAL_SECONDS:-300}" -ge 60 ]]; then
            success "Collection interval is safe for production"
            ((PASSED++))
        else
            error "Collection interval must be >= 60s for production"
            ((FAILED++))
        fi
        
        # PII sanitization
        if [[ "${ENABLE_PII_SANITIZATION:-true}" == "true" ]]; then
            success "PII sanitization is enabled"
            ((PASSED++))
        else
            error "PII sanitization must be enabled for production"
            ((FAILED++))
        fi
        
        # TLS verification
        if [[ "${TLS_INSECURE_SKIP_VERIFY:-false}" == "false" ]]; then
            success "TLS verification is enabled"
            ((PASSED++))
        else
            error "TLS verification must be enabled for production"
            ((FAILED++))
        fi
    fi
}

# ====================
# Database validation
# ====================
validate_databases() {
    log "Validating database configurations..."
    
    # PostgreSQL validation
    if [[ -n "${PG_REPLICA_DSN:-}" ]]; then
        log_info "Validating PostgreSQL configuration..."
        
        # Check DSN format
        if validate_postgresql_dsn "$PG_REPLICA_DSN"; then
            success "PostgreSQL DSN format is valid"
            ((PASSED++))
        else
            error "PostgreSQL DSN format is invalid"
            ((FAILED++))
        fi
        
        # Test connection if requested
        if [[ "$MODE" == "all" ]] || [[ "$MODE" == "connections" ]]; then
            if test_postgresql_connection; then
                ((PASSED++))
                
                # Check prerequisites
                if check_postgresql_prerequisites; then
                    ((PASSED++))
                else
                    ((FAILED++))
                fi
            else
                ((FAILED++))
            fi
        fi
    else
        warning "PostgreSQL not configured"
        ((WARNINGS++))
    fi
    
    # MySQL validation
    if [[ -n "${MYSQL_READONLY_DSN:-}" ]]; then
        log_info "Validating MySQL configuration..."
        
        # Check DSN format
        if validate_mysql_dsn "$MYSQL_READONLY_DSN"; then
            success "MySQL DSN format is valid"
            ((PASSED++))
        else
            error "MySQL DSN format is invalid"
            ((FAILED++))
        fi
        
        # Test connection if requested
        if [[ "$MODE" == "all" ]] || [[ "$MODE" == "connections" ]]; then
            if test_mysql_connection; then
                ((PASSED++))
                
                # Check prerequisites
                if check_mysql_prerequisites; then
                    ((PASSED++))
                else
                    ((FAILED++))
                fi
            else
                ((FAILED++))
            fi
        fi
    else
        warning "MySQL not configured"
        ((WARNINGS++))
    fi
}

# ====================
# Network validation
# ====================
validate_network() {
    log "Validating network connectivity..."
    
    # Test New Relic endpoint
    if [[ -n "${OTLP_ENDPOINT:-}" ]]; then
        # Extract host and port from endpoint
        if [[ "$OTLP_ENDPOINT" =~ ^https?://([^:]+):([0-9]+) ]]; then
            local host="${BASH_REMATCH[1]}"
            local port="${BASH_REMATCH[2]}"
            
            if test_endpoint "$host" "$port"; then
                success "New Relic endpoint is reachable"
                ((PASSED++))
            else
                error "Cannot reach New Relic endpoint"
                ((FAILED++))
            fi
        fi
    fi
    
    # Test database network connectivity
    if [[ -n "${PG_REPLICA_DSN:-}" ]]; then
        if [[ "$PG_REPLICA_DSN" =~ @([^:]+):([0-9]+)/ ]]; then
            local pg_host="${BASH_REMATCH[1]}"
            local pg_port="${BASH_REMATCH[2]}"
            
            if test_endpoint "$pg_host" "$pg_port"; then
                success "PostgreSQL host is reachable"
                ((PASSED++))
            else
                error "Cannot reach PostgreSQL host"
                ((FAILED++))
            fi
        fi
    fi
}

# ====================
# Docker validation
# ====================
validate_docker() {
    log "Validating Docker environment..."
    
    # Check Docker daemon
    if docker info &> /dev/null; then
        success "Docker daemon is running"
        ((PASSED++))
    else
        error "Docker daemon is not running"
        ((FAILED++))
        return 1
    fi
    
    # Check Docker Compose
    if command_exists docker-compose || docker compose version &> /dev/null; then
        success "Docker Compose is available"
        ((PASSED++))
    else
        error "Docker Compose not found"
        ((FAILED++))
    fi
    
    # Check collector image availability
    local collector_image="otel/opentelemetry-collector-contrib:latest"
    if docker image inspect "$collector_image" &> /dev/null; then
        success "Collector image is available locally"
        ((PASSED++))
    else
        warning "Collector image not found locally (will be pulled)"
        ((WARNINGS++))
    fi
}

# ====================
# Collector validation
# ====================
validate_collector() {
    log "Validating collector configuration..."
    
    local project_root="$(get_project_root)"
    local config_file="$project_root/config/collector-improved.yaml"
    
    # Check config file exists
    if [[ -f "$config_file" ]]; then
        success "Collector configuration file exists"
        ((PASSED++))
    else
        error "Collector configuration file not found"
        ((FAILED++))
        return 1
    fi
    
    # Validate YAML syntax (if yq is available)
    if command_exists yq; then
        if yq eval '.' "$config_file" > /dev/null 2>&1; then
            success "Collector configuration syntax is valid"
            ((PASSED++))
        else
            error "Collector configuration has syntax errors"
            ((FAILED++))
        fi
    fi
    
    # Check if collector is running
    if is_container_running "db-intel-primary"; then
        success "Collector container is running"
        ((PASSED++))
        
        # Check health endpoint
        if curl -f -s "http://localhost:13133" &> /dev/null; then
            success "Collector health check passed"
            ((PASSED++))
        else
            error "Collector health check failed"
            ((FAILED++))
        fi
    else
        warning "Collector is not running"
        ((WARNINGS++))
    fi
}

# ====================
# Main validation flow
# ====================
show_usage() {
    cat << EOF
Usage: $0 [mode] [options]

Modes:
  all           Run all validations (default)
  quick         Run quick validations only (no connection tests)
  system        Validate system requirements only
  environment   Validate environment configuration only
  databases     Validate database configurations only
  connections   Test database connections only
  network       Validate network connectivity only
  docker        Validate Docker environment only
  collector     Validate collector setup only

Options:
  -v, --verbose    Enable verbose output
  -h, --help       Show this help message

Examples:
  $0                    # Run all validations
  $0 quick              # Quick validation without connection tests
  $0 databases -v       # Validate databases with verbose output

EOF
}

# Parse command line options
while [[ $# -gt 0 ]]; do
    case "$1" in
        -v|--verbose)
            VERBOSE=true
            export DEBUG=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        all|quick|system|environment|databases|connections|network|docker|collector)
            MODE="$1"
            shift
            ;;
        *)
            error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Main execution
main() {
    echo "Database Intelligence MVP - Comprehensive Validation"
    echo "==================================================="
    echo "Mode: $MODE"
    echo ""
    
    case "$MODE" in
        all)
            validate_system
            echo ""
            validate_environment
            echo ""
            validate_databases
            echo ""
            validate_network
            echo ""
            validate_docker
            echo ""
            validate_collector
            ;;
        quick)
            validate_system
            echo ""
            validate_environment
            echo ""
            validate_docker
            ;;
        system)
            validate_system
            ;;
        environment)
            validate_environment
            ;;
        databases)
            validate_environment  # Need env for DSNs
            echo ""
            validate_databases
            ;;
        connections)
            validate_environment  # Need env for DSNs
            echo ""
            MODE="connections"  # Force connection testing
            validate_databases
            ;;
        network)
            validate_environment  # Need env for endpoints
            echo ""
            validate_network
            ;;
        docker)
            validate_docker
            ;;
        collector)
            validate_collector
            ;;
    esac
    
    # Generate report
    generate_summary "Validation Report" "$PASSED" "$FAILED" "$WARNINGS"
    
    # Exit with appropriate code
    if [[ $FAILED -eq 0 ]]; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main