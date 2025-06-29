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
CONFIG_FILE="${SCRIPT_DIR}/config/collector.yaml"
COMPOSE_FILE="${SCRIPT_DIR}/deploy/docker/docker-compose.yaml"
EXPERIMENTAL_MODE="${EXPERIMENTAL_MODE:-false}"

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
    
    # Choose deployment based on mode
    cd "${SCRIPT_DIR}/deploy/docker"
    
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        log "Starting in EXPERIMENTAL mode with custom components"
        
        # Check if custom collector is built
        if [ ! -f "${SCRIPT_DIR}/dist/db-intelligence-custom" ]; then
            error "Custom collector not built. Run: $0 --experimental build"
            return 1
        fi
        
        # Start experimental deployment
        docker-compose -f docker-compose.experimental.yaml up -d db-intelligence-experimental
        local container_name="db-intel-experimental"
        local health_port="13134"
        local metrics_port="8889"
        local debug_port="55680"
    else
        # Start standard deployment
        docker-compose up -d db-intelligence-primary
        local container_name="db-intel-primary"
        local health_port="13133"
        local metrics_port="8888"
        local debug_port="55679"
    fi
    
    # Wait for startup
    if wait_for_service "http://localhost:${health_port}" 60; then
        success "Collector is running"
    else
        error "Collector failed to start"
        echo "Check logs with: docker logs ${container_name}"
        return 1
    fi
    
    echo ""
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        success "Database Intelligence MVP is running in EXPERIMENTAL mode!"
        echo ""
        echo "⚠️  WARNING: Experimental components are active"
        echo "   - Single instance deployment (stateful components)"
        echo "   - Higher resource usage expected"
        echo "   - Monitor closely for issues"
    else
        success "Database Intelligence MVP is running!"
    fi
    echo ""
    echo "🔗 Useful endpoints:"
    echo "   Health Check: http://localhost:${health_port}"
    echo "   Metrics:      http://localhost:${metrics_port}/metrics"
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        echo "   Debug UI:     http://localhost:${debug_port}/debug/tracez"
        echo "   pprof:        http://localhost:6061/debug/pprof"
    fi
    echo ""
    echo "📊 To check if data is being collected:"
    echo "   curl http://localhost:${metrics_port}/metrics | grep otelcol_receiver_accepted"
    echo ""
    echo "🛑 To stop:"
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        echo "   docker-compose -f ${SCRIPT_DIR}/deploy/docker/docker-compose.experimental.yaml down"
    else
        echo "   docker-compose -f ${SCRIPT_DIR}/deploy/docker/docker-compose.yaml down"
    fi
    echo ""
}

# Status check
check_status() {
    log "Checking collector status..."
    
    # Determine ports based on mode
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        local health_port="13134"
        local metrics_port="8889"
        local compose_file="docker-compose.experimental.yaml"
    else
        local health_port="13133"
        local metrics_port="8888"
        local compose_file="docker-compose.yaml"
    fi
    
    # Health check
    if curl -f http://localhost:${health_port} &> /dev/null; then
        success "Collector is healthy"
    else
        error "Collector health check failed"
        return 1
    fi
    
    # Metrics check
    local metrics=$(curl -s http://localhost:${metrics_port}/metrics 2>/dev/null || echo "")
    if [ -n "$metrics" ]; then
        success "Metrics endpoint responding"
        
        # Extract key metrics
        local received=$(echo "$metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        local exported=$(echo "$metrics" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        
        echo "   📈 Records received: $received"
        echo "   📤 Records exported: $exported"
        
        # Experimental-specific metrics
        if [ "$EXPERIMENTAL_MODE" = "true" ]; then
            local ash_samples=$(echo "$metrics" | grep "db_intelligence_ash_samples_total" | tail -1 | awk '{print $2}' || echo "0")
            local circuit_state=$(echo "$metrics" | grep "db_intelligence_circuitbreaker_open" | tail -1 | awk '{print $2}' || echo "0")
            local sampling_rate=$(echo "$metrics" | grep "db_intelligence_adaptivesampler_current_rate" | tail -1 | awk '{print $2}' || echo "0")
            
            echo ""
            echo "   🔬 Experimental Metrics:"
            echo "   ASH Samples: $ash_samples"
            echo "   Circuit Breaker: $([ "$circuit_state" = "0" ] && echo "closed" || echo "OPEN")"
            echo "   Sampling Rate: ${sampling_rate}%"
        fi
    else
        warning "Metrics endpoint not responding"
    fi
    
    # Container status
    echo ""
    echo "🐳 Container status:"
    docker-compose -f "${SCRIPT_DIR}/deploy/docker/${compose_file}" ps
}

# Stop services
stop_services() {
    log "Stopping Database Intelligence Collector..."
    
    cd "${SCRIPT_DIR}/deploy/docker"
    
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        docker-compose -f docker-compose.experimental.yaml down
    else
        docker-compose down
    fi
    
    success "Services stopped"
}

# Show logs
show_logs() {
    if [ "$EXPERIMENTAL_MODE" = "true" ]; then
        local service="${1:-experimental}"
    else
        local service="${1:-primary}"
    fi
    
    docker logs -f "db-intel-${service}"
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
    echo "Usage: $0 [--experimental] <command>"
    echo ""
    echo "Options:"
    echo "  --experimental  Use experimental components (requires build)"
    echo ""
    echo "Commands:"
    echo "  prereq      Check prerequisites"
    echo "  configure   Interactive configuration setup"
    echo "  validate    Validate database connections"
    echo "  start       Start the collector"
    echo "  status      Check collector status"
    echo "  stop        Stop the collector"
    echo "  logs        Show collector logs"
    echo "  test        Run safety tests"
    echo "  build       Build experimental collector (experimental mode only)"
    echo "  all         Run prereq, configure, validate, start"
    echo ""
    echo "Examples:"
    echo "  $0 all                    # Complete setup (production)"
    echo "  $0 --experimental all     # Complete setup (experimental)"
    echo "  $0 --experimental build   # Build custom collector"
    echo "  $0 start                  # Start with existing config"
    echo "  $0 logs                   # Show logs"
    echo ""
}

# Build experimental collector
build_experimental() {
    log "Building experimental collector..."
    
    if [ ! -f "${SCRIPT_DIR}/scripts/build-custom-collector.sh" ]; then
        error "Build script not found"
        return 1
    fi
    
    # Make script executable
    chmod +x "${SCRIPT_DIR}/scripts/build-custom-collector.sh"
    
    # Run build
    "${SCRIPT_DIR}/scripts/build-custom-collector.sh" --with-docker --with-tests
    
    if [ $? -eq 0 ]; then
        success "Experimental collector built successfully"
        echo ""
        echo "You can now start the experimental collector with:"
        echo "  $0 --experimental start"
    else
        error "Build failed"
        return 1
    fi
}

# Main script logic
main() {
    # Check for experimental flag
    if [ "${1:-}" = "--experimental" ]; then
        EXPERIMENTAL_MODE="true"
        shift
    fi
    
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
        build)
            if [ "$EXPERIMENTAL_MODE" = "true" ]; then
                build_experimental
            else
                error "Build command requires --experimental flag"
                exit 1
            fi
            ;;
        all)
            check_prerequisites
            if [ "$EXPERIMENTAL_MODE" = "true" ]; then
                build_experimental
            fi
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