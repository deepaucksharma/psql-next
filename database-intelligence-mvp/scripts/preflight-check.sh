#!/bin/bash
# Pre-flight check script for Database Intelligence MVP
# Validates environment before deployment

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Check results
CHECKS_PASSED=0
CHECKS_FAILED=0
WARNINGS=0

# Mode (standard or experimental)
MODE="${1:-standard}"

# Helper functions
pass_check() {
    echo -e "✅ $1"
    ((CHECKS_PASSED++))
}

fail_check() {
    echo -e "❌ $1"
    ((CHECKS_FAILED++))
}

warn_check() {
    echo -e "⚠️  $1"
    ((WARNINGS++))
}

info_check() {
    echo -e "ℹ️  $1"
}

# System checks
check_system_requirements() {
    log "Checking system requirements..."
    
    # Check OS
    local os_type=$(uname -s)
    case "$os_type" in
        Linux|Darwin)
            pass_check "Operating System: $os_type"
            ;;
        *)
            warn_check "Operating System: $os_type (untested)"
            ;;
    esac
    
    # Check memory
    local total_mem=0
    if [[ "$os_type" == "Linux" ]]; then
        total_mem=$(free -m | awk '/^Mem:/{print $2}')
    elif [[ "$os_type" == "Darwin" ]]; then
        total_mem=$(( $(sysctl -n hw.memsize) / 1024 / 1024 ))
    fi
    
    local required_mem=512
    if [[ "$MODE" == "experimental" ]]; then
        required_mem=2048
    fi
    
    if [[ $total_mem -gt $required_mem ]]; then
        pass_check "Memory: ${total_mem}MB available (${required_mem}MB required)"
    else
        fail_check "Memory: ${total_mem}MB available (${required_mem}MB required)"
    fi
    
    # Check disk space
    local free_space=$(df -m "$PROJECT_ROOT" | awk 'NR==2 {print $4}')
    if [[ $free_space -gt 1000 ]]; then
        pass_check "Disk Space: ${free_space}MB free"
    else
        warn_check "Disk Space: ${free_space}MB free (1GB recommended)"
    fi
}

# Docker checks
check_docker() {
    log "Checking Docker environment..."
    
    # Check Docker
    if command_exists docker; then
        local docker_version=$(docker --version | awk '{print $3}' | sed 's/,$//')
        pass_check "Docker: $docker_version"
        
        # Check if Docker is running
        if docker info &> /dev/null; then
            pass_check "Docker daemon: Running"
        else
            fail_check "Docker daemon: Not running"
            info_check "Start Docker and try again"
        fi
    else
        fail_check "Docker: Not installed"
        info_check "Install Docker from https://docs.docker.com/get-docker/"
    fi
    
    # Check Docker Compose
    if command_exists docker-compose; then
        local compose_version=$(docker-compose --version | awk '{print $3}' | sed 's/,$//')
        pass_check "Docker Compose: $compose_version"
    elif docker compose version &> /dev/null; then
        local compose_version=$(docker compose version | awk '{print $4}')
        pass_check "Docker Compose: $compose_version (plugin)"
    else
        fail_check "Docker Compose: Not installed"
        info_check "Install docker-compose or Docker Desktop"
    fi
}

# Build requirements (experimental mode)
check_build_requirements() {
    if [[ "$MODE" != "experimental" ]]; then
        return
    fi
    
    log "Checking build requirements for experimental mode..."
    
    # Check Go
    if command_exists go; then
        local go_version=$(go version | awk '{print $3}' | sed 's/go//')
        local required_version="1.21"
        
        if [[ "$(printf '%s\n' "$required_version" "$go_version" | sort -V | head -n1)" == "$required_version" ]]; then
            pass_check "Go: $go_version (>= $required_version required)"
        else
            fail_check "Go: $go_version (>= $required_version required)"
        fi
    else
        fail_check "Go: Not installed (required for experimental mode)"
        info_check "Install Go from https://golang.org/dl/"
    fi
    
    # Check if builder is available
    if command -v builder &> /dev/null; then
        pass_check "OpenTelemetry Collector Builder: Installed"
    else
        warn_check "OpenTelemetry Collector Builder: Not installed"
        info_check "Will be installed automatically during build"
    fi
    
    # Check if custom binary exists
    if [[ -f "${PROJECT_ROOT}/dist/db-intelligence-custom" ]]; then
        pass_check "Custom collector binary: Already built"
    else
        info_check "Custom collector binary: Not built yet"
        info_check "Run: ./quickstart.sh --experimental build"
    fi
}

# Configuration checks
check_configuration() {
    log "Checking configuration..."
    
    # Check for .env file
    if [[ -f "${PROJECT_ROOT}/.env" ]]; then
        pass_check "Environment file: .env exists"
        
        # Source it safely
        set -a
        source "${PROJECT_ROOT}/.env"
        set +a
        
        # Check required variables
        if [[ -n "${PG_REPLICA_DSN:-}" ]]; then
            pass_check "PostgreSQL DSN: Configured"
        else
            warn_check "PostgreSQL DSN: Not configured"
        fi
        
        if [[ -n "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
            pass_check "New Relic License Key: Configured"
        else
            fail_check "New Relic License Key: Not configured"
        fi
        
        if [[ -n "${MYSQL_READONLY_DSN:-}" ]]; then
            info_check "MySQL DSN: Configured (optional)"
        else
            info_check "MySQL DSN: Not configured (optional)"
        fi
    else
        warn_check "Environment file: .env not found"
        info_check "Run: ./quickstart.sh configure"
    fi
    
    # Check config files
    if [[ -f "${PROJECT_ROOT}/config/collector.yaml" ]]; then
        pass_check "Standard collector config: Found"
    else
        fail_check "Standard collector config: Missing"
    fi
    
    if [[ "$MODE" == "experimental" ]] && [[ -f "${PROJECT_ROOT}/config/collector-experimental.yaml" ]]; then
        pass_check "Experimental collector config: Found"
    elif [[ "$MODE" == "experimental" ]]; then
        fail_check "Experimental collector config: Missing"
    fi
}

# Network checks
check_network() {
    log "Checking network connectivity..."
    
    # Check internet connectivity
    if ping -c 1 -W 2 8.8.8.8 &> /dev/null; then
        pass_check "Internet connectivity: OK"
    else
        warn_check "Internet connectivity: Limited or none"
    fi
    
    # Check DNS resolution
    if host otlp.nr-data.net &> /dev/null; then
        pass_check "DNS resolution: OK"
    else
        fail_check "DNS resolution: Cannot resolve otlp.nr-data.net"
    fi
    
    # Check if ports are available
    local ports=(13133 8888)
    if [[ "$MODE" == "experimental" ]]; then
        ports+=(13134 8889 55680 6061)
    fi
    
    for port in "${ports[@]}"; do
        if ! lsof -i:$port &> /dev/null; then
            pass_check "Port $port: Available"
        else
            fail_check "Port $port: Already in use"
            local process=$(lsof -i:$port | grep LISTEN | awk '{print $1}' | head -1)
            info_check "Used by: $process"
        fi
    done
}

# Database connectivity check
check_database_connectivity() {
    log "Checking database connectivity..."
    
    if [[ -z "${PG_REPLICA_DSN:-}" ]]; then
        info_check "PostgreSQL DSN not configured, skipping connection test"
        return
    fi
    
    # Check if psql is available
    if command_exists psql; then
        info_check "Testing PostgreSQL connection..."
        
        if psql "$PG_REPLICA_DSN" -c "SELECT 1" &> /dev/null; then
            pass_check "PostgreSQL connection: Success"
            
            # Check pg_stat_statements
            if psql "$PG_REPLICA_DSN" -c "SELECT * FROM pg_stat_statements LIMIT 1" &> /dev/null; then
                pass_check "pg_stat_statements: Accessible"
            else
                fail_check "pg_stat_statements: Not accessible or not enabled"
                info_check "Enable with: CREATE EXTENSION pg_stat_statements;"
            fi
        else
            fail_check "PostgreSQL connection: Failed"
            info_check "Check your connection string and network access"
        fi
    else
        warn_check "psql not installed, cannot test database connection"
        info_check "Install postgresql-client for connection testing"
    fi
}

# Security checks
check_security() {
    log "Checking security settings..."
    
    # Check file permissions
    if [[ -f "${PROJECT_ROOT}/.env" ]]; then
        local perms=$(stat -c %a "${PROJECT_ROOT}/.env" 2>/dev/null || stat -f %p "${PROJECT_ROOT}/.env" | tail -c 4)
        if [[ "$perms" == "600" ]] || [[ "$perms" == "640" ]]; then
            pass_check "Environment file permissions: Secure ($perms)"
        else
            warn_check "Environment file permissions: $perms (recommend 600)"
            info_check "Fix with: chmod 600 ${PROJECT_ROOT}/.env"
        fi
    fi
    
    # Check for default credentials
    if [[ -n "${NEW_RELIC_LICENSE_KEY:-}" ]] && [[ "$NEW_RELIC_LICENSE_KEY" == "your-license-key-here" ]]; then
        fail_check "New Relic License Key: Using default placeholder"
    fi
    
    # Check SSL in connection strings
    if [[ -n "${PG_REPLICA_DSN:-}" ]]; then
        if [[ "$PG_REPLICA_DSN" == *"sslmode=require"* ]] || [[ "$PG_REPLICA_DSN" == *"sslmode=verify"* ]]; then
            pass_check "PostgreSQL SSL: Enabled"
        else
            warn_check "PostgreSQL SSL: Not enforced"
            info_check "Add ?sslmode=require to connection string"
        fi
    fi
}

# Generate report
generate_report() {
    echo ""
    echo "======================================"
    echo "    PRE-FLIGHT CHECK REPORT"
    echo "======================================"
    echo ""
    echo "Mode: $(echo $MODE | tr '[:lower:]' '[:upper:]')"
    echo "Date: $(date)"
    echo ""
    echo "Summary:"
    echo "  ✅ Passed: $CHECKS_PASSED"
    echo "  ❌ Failed: $CHECKS_FAILED"
    echo "  ⚠️  Warnings: $WARNINGS"
    echo ""
    
    if [[ $CHECKS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}✅ All critical checks passed!${NC}"
        echo ""
        echo "You're ready to deploy Database Intelligence MVP."
        echo ""
        if [[ "$MODE" == "experimental" ]]; then
            echo "Next step: ./quickstart.sh --experimental all"
        else
            echo "Next step: ./quickstart.sh all"
        fi
    else
        echo -e "${RED}❌ Some critical checks failed!${NC}"
        echo ""
        echo "Please address the failed checks before proceeding."
        echo "See the information messages above for guidance."
    fi
    
    if [[ $WARNINGS -gt 0 ]]; then
        echo ""
        echo -e "${YELLOW}Note: You have $WARNINGS warnings that should be reviewed.${NC}"
    fi
    
    echo ""
    echo "======================================"
}

# Main execution
main() {
    echo "Database Intelligence MVP - Pre-flight Check"
    echo ""
    
    # Run all checks
    check_system_requirements
    echo ""
    
    check_docker
    echo ""
    
    check_build_requirements
    echo ""
    
    check_configuration
    echo ""
    
    check_network
    echo ""
    
    check_database_connectivity
    echo ""
    
    check_security
    echo ""
    
    # Generate report
    generate_report
    
    # Exit with appropriate code
    if [[ $CHECKS_FAILED -gt 0 ]]; then
        exit 1
    else
        exit 0
    fi
}

# Show usage if needed
if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
    echo "Usage: $0 [standard|experimental]"
    echo ""
    echo "Run pre-flight checks for Database Intelligence MVP deployment."
    echo ""
    echo "Options:"
    echo "  standard      Check requirements for standard mode (default)"
    echo "  experimental  Check requirements for experimental mode"
    echo ""
    echo "Examples:"
    echo "  $0                    # Check for standard deployment"
    echo "  $0 experimental       # Check for experimental deployment"
    exit 0
fi

# Run main
main