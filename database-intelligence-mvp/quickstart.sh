#!/bin/bash
# Database Intelligence MVP - One-Click Quickstart
# Addresses UX and setup complexity issues

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions
source "${SCRIPT_DIR}/scripts/lib/common.sh"

# Configuration
INSTALL_DIR="${INSTALL_DIR:-/opt/db-intelligence}"
CONFIG_FILE="${SCRIPT_DIR}/config/collector-improved.yaml"
COMPOSE_FILE="${SCRIPT_DIR}/deploy/docker/docker-compose.yaml"

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check required tools
    local required_tools=("docker" "curl")
    if ! check_required_commands "${required_tools[@]}"; then
        echo ""
        echo "Please install the missing dependencies and run this script again."
        echo ""
        echo "On Ubuntu/Debian:"
        echo "  sudo apt-get update"
        echo "  sudo apt-get install docker.io docker-compose curl postgresql-client"
        echo ""
        echo "On macOS:"
        echo "  brew install docker docker-compose curl postgresql"
        echo ""
        exit 1
    fi
    
    # Check Docker Compose separately
    if ! command_exists docker-compose && ! docker compose version &> /dev/null; then
        error "Docker Compose not found"
        echo "Please install docker-compose or ensure Docker includes compose plugin"
        exit 1
    fi
    
    # Check optional tools
    if ! command_exists psql; then
        warning "psql not found - database testing will be limited"
    fi
    
    success "All prerequisites satisfied"
}

# Interactive configuration
configure_environment() {
    log "Setting up environment configuration..."
    
    # Use the init-env.sh script for configuration
    if [ -x "${SCRIPT_DIR}/scripts/init-env.sh" ]; then
        "${SCRIPT_DIR}/scripts/init-env.sh" setup
    else
        error "init-env.sh script not found or not executable"
        exit 1
    fi
}

# Database setup validation
validate_database_setup() {
    log "Validating database setup..."
    
    # Load environment
    if ! load_env_file "${SCRIPT_DIR}/.env"; then
        error "Failed to load environment"
        return 1
    fi
    
    # Use the validate-all.sh script if available
    if [ -x "${SCRIPT_DIR}/scripts/validate-all.sh" ]; then
        "${SCRIPT_DIR}/scripts/validate-all.sh" databases
    else
        # Fallback to direct testing
        if [ -n "${PG_REPLICA_DSN:-}" ]; then
            test_postgresql_connection
            check_postgresql_prerequisites
        fi
        
        if [ -n "${MYSQL_READONLY_DSN:-}" ]; then
            test_mysql_connection
            check_mysql_prerequisites
        fi
    fi
}

# Start services
start_services() {
    log "Starting Database Intelligence Collector..."
    
    local env_file="${SCRIPT_DIR}/.env"
    if [ ! -f "$env_file" ]; then
        error "No configuration found. Run configure first."
        return 1
    fi
    
    # Copy environment file for docker-compose
    cp "$env_file" "${SCRIPT_DIR}/deploy/docker/.env"
    
    # Start the collector
    cd "${SCRIPT_DIR}/deploy/docker"
    docker-compose up -d db-intelligence-primary
    
    # Wait for startup
    if wait_for_service "http://localhost:13133" 60; then
        success "Collector is running"
    else
        error "Collector failed to start"
        echo "Check logs with: docker logs db-intel-primary"
        return 1
    fi
    
    if [ $retries -eq 0 ]; then
        error "Collector failed to start"
        echo "Check logs with: docker logs db-intel-primary"
        return 1
    fi
    
    echo ""
    success "Database Intelligence MVP is running!"
    echo ""
    echo "🔗 Useful endpoints:"
    echo "   Health Check: http://localhost:13133"
    echo "   Metrics:      http://localhost:8888/metrics"
    echo "   Debug UI:     http://localhost:55679"
    echo ""
    echo "📊 To check if data is being collected:"
    echo "   curl http://localhost:8888/metrics | grep otelcol_receiver_accepted"
    echo ""
    echo "🛑 To stop:"
    echo "   docker-compose -f ${SCRIPT_DIR}/deploy/docker/docker-compose.yaml down"
    echo ""
}

# Status check
check_status() {
    log "Checking collector status..."
    
    # Health check
    if curl -f http://localhost:13133 &> /dev/null; then
        success "Collector is healthy"
    else
        error "Collector health check failed"
        return 1
    fi
    
    # Metrics check
    local metrics=$(curl -s http://localhost:8888/metrics 2>/dev/null || echo "")
    if [ -n "$metrics" ]; then
        success "Metrics endpoint responding"
        
        # Extract key metrics
        local received=$(echo "$metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        local exported=$(echo "$metrics" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        
        echo "   📈 Records received: $received"
        echo "   📤 Records exported: $exported"
    else
        warning "Metrics endpoint not responding"
    fi
    
    # Container status
    echo ""
    echo "🐳 Container status:"
    docker-compose -f "${SCRIPT_DIR}/deploy/docker/docker-compose.yaml" ps
}

# Stop services
stop_services() {
    log "Stopping Database Intelligence Collector..."
    
    cd "${SCRIPT_DIR}/deploy/docker"
    docker-compose down
    
    success "Services stopped"
}

# Show logs
show_logs() {
    local service="${1:-db-intelligence-primary}"
    docker logs -f "db-intel-${service#db-intelligence-}"
}

# Run safety tests
run_tests() {
    log "Running safety tests..."
    
    # Make sure test script is executable
    chmod +x "${SCRIPT_DIR}/tests/integration/test_collector_safety.sh"
    
    # Run the tests
    "${SCRIPT_DIR}/tests/integration/test_collector_safety.sh"
}

# Print usage
usage() {
    echo "Database Intelligence MVP Quickstart"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  prereq     Check prerequisites"
    echo "  configure  Interactive configuration setup"
    echo "  validate   Validate database connections"
    echo "  start      Start the collector"
    echo "  status     Check collector status"
    echo "  stop       Stop the collector"
    echo "  logs       Show collector logs"
    echo "  test       Run safety tests"
    echo "  all        Run prereq, configure, validate, start"
    echo ""
    echo "Examples:"
    echo "  $0 all          # Complete setup"
    echo "  $0 start        # Start with existing config"
    echo "  $0 logs         # Show logs"
    echo ""
}

# Main script logic
main() {
    local command="${1:-}"
    
    if [ -z "$command" ]; then
        usage
        exit 1
    fi
    
    case "$command" in
        prereq)
            check_prerequisites
            ;;
        configure)
            configure_environment
            ;;
        validate)
            validate_database_setup
            ;;
        start)
            start_services
            ;;
        status)
            check_status
            ;;
        stop)
            stop_services
            ;;
        logs)
            show_logs "${2:-}"
            ;;
        test)
            run_tests
            ;;
        all)
            check_prerequisites
            configure_environment
            validate_database_setup
            start_services
            ;;
        *)
            error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"