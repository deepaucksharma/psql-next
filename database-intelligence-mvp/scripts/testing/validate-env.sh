#!/bin/bash
# Database Intelligence MVP - Environment Validation Script
# This script validates the environment configuration

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Validation results
VALIDATION_PASSED=true
WARNINGS=0
ERRORS=0

# Helper functions
log() {
    echo -e "${BLUE}[VALIDATE]${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARNINGS++))
}

error() {
    echo -e "${RED}✗${NC} $1"
    ((ERRORS++))
    VALIDATION_PASSED=false
}

# Check if .env exists
check_env_file() {
    log "Checking for .env file..."
    
    if [ ! -f "$PROJECT_ROOT/.env" ]; then
        error ".env file not found. Run './scripts/init-env.sh setup' first."
        return 1
    fi
    
    success ".env file exists"
    
    # Source the environment
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
    
    return 0
}

# Validate required variables
validate_required_vars() {
    log "Validating required environment variables..."
    
    local required_vars=(
        "NEW_RELIC_LICENSE_KEY"
        "OTLP_ENDPOINT"
        "DEPLOYMENT_ENV"
    )
    
    for var in "${required_vars[@]}"; do
        if [ -z "${!var:-}" ]; then
            error "$var is not set"
        else
            success "$var is set"
        fi
    done
}

# Validate New Relic configuration
validate_newrelic() {
    log "Validating New Relic configuration..."
    
    # Check license key format
    if [ -n "${NEW_RELIC_LICENSE_KEY:-}" ]; then
        if [[ "$NEW_RELIC_LICENSE_KEY" == "your-license-key-here" ]]; then
            error "NEW_RELIC_LICENSE_KEY has not been updated from default"
        elif [[ ! "$NEW_RELIC_LICENSE_KEY" =~ ^[a-zA-Z0-9]{40}$ ]] && 
             [[ ! "$NEW_RELIC_LICENSE_KEY" =~ ^eu01xx[a-zA-Z0-9]{34}$ ]]; then
            warning "NEW_RELIC_LICENSE_KEY format may be incorrect"
        else
            success "NEW_RELIC_LICENSE_KEY format looks valid"
        fi
    fi
    
    # Check OTLP endpoint
    if [ -n "${OTLP_ENDPOINT:-}" ]; then
        if [[ "$OTLP_ENDPOINT" =~ ^https://otlp\.(eu01\.)?nr-data\.net:4317$ ]]; then
            success "OTLP_ENDPOINT is valid"
        else
            warning "OTLP_ENDPOINT may be incorrect: $OTLP_ENDPOINT"
        fi
    fi
}

# Validate database configuration
validate_databases() {
    log "Validating database configuration..."
    
    # Check PostgreSQL
    if [ -n "${PG_REPLICA_DSN:-}" ]; then
        if [[ "$PG_REPLICA_DSN" == *"your-password"* ]] || 
           [[ "$PG_REPLICA_DSN" == *"localhost"* && "$DEPLOYMENT_ENV" == "production" ]]; then
            warning "PG_REPLICA_DSN appears to have default values"
        else
            success "PG_REPLICA_DSN is configured"
        fi
        
        # Validate format
        if [[ ! "$PG_REPLICA_DSN" =~ ^postgres://[^:]+:[^@]+@[^:]+:[0-9]+/[^?]+(\?.*)?$ ]]; then
            error "PG_REPLICA_DSN format is incorrect"
        fi
    else
        warning "PG_REPLICA_DSN is not configured"
    fi
    
    # Check MySQL
    if [ -n "${MYSQL_READONLY_DSN:-}" ]; then
        if [[ "$MYSQL_READONLY_DSN" == *"your-password"* ]] || 
           [[ "$MYSQL_READONLY_DSN" == *"localhost"* && "$DEPLOYMENT_ENV" == "production" ]]; then
            warning "MYSQL_READONLY_DSN appears to have default values"
        else
            success "MYSQL_READONLY_DSN is configured"
        fi
        
        # Validate format
        if [[ ! "$MYSQL_READONLY_DSN" =~ ^[^:]+:[^@]+@tcp\([^:]+:[0-9]+\)/[^?]+ ]]; then
            error "MYSQL_READONLY_DSN format is incorrect"
        fi
    else
        warning "MYSQL_READONLY_DSN is not configured"
    fi
}

# Validate deployment settings
validate_deployment() {
    log "Validating deployment settings..."
    
    # Check deployment environment
    case "${DEPLOYMENT_ENV:-}" in
        development|staging|production)
            success "DEPLOYMENT_ENV is valid: $DEPLOYMENT_ENV"
            ;;
        *)
            error "DEPLOYMENT_ENV must be one of: development, staging, production"
            ;;
    esac
    
    # Production-specific checks
    if [ "${DEPLOYMENT_ENV:-}" == "production" ]; then
        log "Running production-specific validations..."
        
        # Check collection interval
        if [ "${COLLECTION_INTERVAL_SECONDS:-300}" -lt 60 ]; then
            error "COLLECTION_INTERVAL_SECONDS must be >= 60 for production"
        else
            success "Collection interval is safe for production"
        fi
        
        # Check PII sanitization
        if [ "${ENABLE_PII_SANITIZATION:-true}" != "true" ]; then
            error "ENABLE_PII_SANITIZATION must be true for production"
        else
            success "PII sanitization is enabled"
        fi
        
        # Check TLS
        if [ "${TLS_INSECURE_SKIP_VERIFY:-false}" == "true" ]; then
            error "TLS_INSECURE_SKIP_VERIFY must be false for production"
        else
            success "TLS verification is enabled"
        fi
    fi
}

# Validate resource limits
validate_resources() {
    log "Validating resource limits..."
    
    # Check memory limit
    if [ "${MEMORY_LIMIT_MIB:-1024}" -lt 512 ]; then
        warning "MEMORY_LIMIT_MIB is below recommended minimum (512)"
    elif [ "${MEMORY_LIMIT_MIB:-1024}" -gt 2048 ]; then
        warning "MEMORY_LIMIT_MIB is above recommended maximum (2048)"
    else
        success "Memory limit is within recommended range"
    fi
    
    # Check sampling percentage
    if [ "${SAMPLING_PERCENTAGE:-25}" -lt 1 ] || [ "${SAMPLING_PERCENTAGE:-25}" -gt 100 ]; then
        error "SAMPLING_PERCENTAGE must be between 1 and 100"
    else
        success "Sampling percentage is valid"
    fi
}

# Test connectivity (optional)
test_connectivity() {
    if [ "${1:-}" != "--test-connections" ]; then
        return 0
    fi
    
    log "Testing database connections..."
    
    # Test PostgreSQL
    if [ -n "${PG_REPLICA_DSN:-}" ] && command -v psql &> /dev/null; then
        log "Testing PostgreSQL connection..."
        if psql "$PG_REPLICA_DSN" -c "SELECT 1;" &> /dev/null; then
            success "PostgreSQL connection successful"
        else
            error "PostgreSQL connection failed"
        fi
    fi
    
    # Test MySQL
    if [ -n "${MYSQL_READONLY_DSN:-}" ] && command -v mysql &> /dev/null; then
        log "Testing MySQL connection..."
        # Parse and test MySQL connection
        # (Implementation depends on MySQL client availability)
        warning "MySQL connection test not implemented"
    fi
    
    # Test New Relic endpoint
    if [ -n "${OTLP_ENDPOINT:-}" ] && [ -n "${NEW_RELIC_LICENSE_KEY:-}" ]; then
        log "Testing New Relic endpoint..."
        if curl -s -o /dev/null -w "%{http_code}" \
               -H "Api-Key: ${NEW_RELIC_LICENSE_KEY}" \
               "${OTLP_ENDPOINT%:*}/v1/health" | grep -q "200\|404"; then
            success "New Relic endpoint is reachable"
        else
            warning "Could not verify New Relic endpoint"
        fi
    fi
}

# Generate validation report
generate_report() {
    echo ""
    echo "====================================="
    echo "Environment Validation Report"
    echo "====================================="
    echo ""
    
    if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
        echo -e "${GREEN}All validations passed!${NC}"
        echo ""
        echo "Your environment is properly configured."
    elif [ $ERRORS -eq 0 ]; then
        echo -e "${YELLOW}Validation passed with warnings${NC}"
        echo ""
        echo "Errors:   0"
        echo "Warnings: $WARNINGS"
        echo ""
        echo "Review the warnings above, but you can proceed."
    else
        echo -e "${RED}Validation failed${NC}"
        echo ""
        echo "Errors:   $ERRORS"
        echo "Warnings: $WARNINGS"
        echo ""
        echo "Fix the errors above before proceeding."
    fi
    
    echo ""
    echo "Environment: ${DEPLOYMENT_ENV:-unknown}"
    echo "Timestamp:   $(date)"
    echo ""
}

# Main validation flow
main() {
    echo "Database Intelligence MVP - Environment Validation"
    echo "================================================="
    echo ""
    
    # Check for .env file
    if ! check_env_file; then
        generate_report
        exit 1
    fi
    
    # Run validations
    validate_required_vars
    echo ""
    validate_newrelic
    echo ""
    validate_databases
    echo ""
    validate_deployment
    echo ""
    validate_resources
    echo ""
    
    # Optional connection tests
    if [ "${1:-}" == "--test-connections" ]; then
        test_connectivity "$@"
        echo ""
    fi
    
    # Generate report
    generate_report
    
    # Exit with appropriate code
    if [ "$VALIDATION_PASSED" == "true" ]; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main "$@"