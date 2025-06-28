#!/bin/bash
# Database Intelligence MVP - One-Click Quickstart
# Addresses UX and setup complexity issues

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/opt/db-intelligence}"
CONFIG_FILE="${SCRIPT_DIR}/config/collector-improved.yaml"
COMPOSE_FILE="${SCRIPT_DIR}/deploy/docker/docker-compose.yaml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
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

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    local missing_deps=()
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        missing_deps+=("docker")
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        missing_deps+=("docker-compose")
    fi
    
    # Check curl
    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi
    
    # Check psql (optional but recommended)
    if ! command -v psql &> /dev/null; then
        warning "psql not found - database testing will be limited"
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        error "Missing dependencies: ${missing_deps[*]}"
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
    
    success "All prerequisites satisfied"
}

# Interactive configuration
configure_environment() {
    log "Setting up environment configuration..."
    
    local env_file="${SCRIPT_DIR}/.env"
    
    # Check if .env already exists
    if [ -f "$env_file" ]; then
        read -p "Configuration file exists. Overwrite? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log "Using existing configuration"
            return 0
        fi
    fi
    
    echo "# Database Intelligence MVP Configuration" > "$env_file"
    echo "# Generated on $(date)" >> "$env_file"
    echo "" >> "$env_file"
    
    # New Relic configuration
    echo ""
    echo "🔧 New Relic Configuration"
    echo "=========================="
    read -p "New Relic License Key: " -r nr_license_key
    if [ -z "$nr_license_key" ]; then
        error "New Relic License Key is required"
        exit 1
    fi
    echo "NEW_RELIC_LICENSE_KEY=${nr_license_key}" >> "$env_file"
    
    read -p "New Relic OTLP Endpoint (default: https://otlp.nr-data.net:4317): " -r nr_endpoint
    nr_endpoint="${nr_endpoint:-https://otlp.nr-data.net:4317}"
    echo "OTLP_ENDPOINT=${nr_endpoint}" >> "$env_file"
    
    # Database configuration
    echo ""
    echo "🗄️ Database Configuration"
    echo "========================="
    
    # PostgreSQL
    read -p "Configure PostgreSQL? (Y/n): " -r
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        read -p "PostgreSQL Host (default: localhost): " -r pg_host
        pg_host="${pg_host:-localhost}"
        
        read -p "PostgreSQL Port (default: 5432): " -r pg_port
        pg_port="${pg_port:-5432}"
        
        read -p "PostgreSQL Database: " -r pg_db
        read -p "PostgreSQL User: " -r pg_user
        read -p "PostgreSQL Password: " -s -r pg_pass
        echo ""
        
        echo "PG_REPLICA_DSN=postgres://${pg_user}:${pg_pass}@${pg_host}:${pg_port}/${pg_db}?sslmode=prefer" >> "$env_file"
    fi
    
    # MySQL
    echo ""
    read -p "Configure MySQL? (y/N): " -r
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        read -p "MySQL Host (default: localhost): " -r mysql_host
        mysql_host="${mysql_host:-localhost}"
        
        read -p "MySQL Port (default: 3306): " -r mysql_port
        mysql_port="${mysql_port:-3306}"
        
        read -p "MySQL Database: " -r mysql_db
        read -p "MySQL User: " -r mysql_user
        read -p "MySQL Password: " -s -r mysql_pass
        echo ""
        
        echo "MYSQL_READONLY_DSN=${mysql_user}:${mysql_pass}@tcp(${mysql_host}:${mysql_port})/${mysql_db}?tls=true" >> "$env_file"
    fi
    
    success "Configuration saved to $env_file"
}

# Database setup validation
validate_database_setup() {
    log "Validating database setup..."
    
    local env_file="${SCRIPT_DIR}/.env"
    if [ ! -f "$env_file" ]; then
        error "No configuration found. Run configure first."
        return 1
    fi
    
    source "$env_file"
    
    # Test PostgreSQL if configured
    if [ -n "${PG_REPLICA_DSN:-}" ]; then
        log "Testing PostgreSQL connection..."
        
        if command -v psql &> /dev/null; then
            if psql "$PG_REPLICA_DSN" -c "SELECT 1;" &> /dev/null; then
                success "PostgreSQL connection OK"
                
                # Check pg_stat_statements
                if psql "$PG_REPLICA_DSN" -c "SELECT * FROM pg_stat_statements LIMIT 1;" &> /dev/null; then
                    success "pg_stat_statements extension available"
                else
                    warning "pg_stat_statements extension not found"
                    echo "         To enable it, run as superuser:"
                    echo "         CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
                fi
            else
                warning "PostgreSQL connection failed"
                echo "         Please check your connection parameters"
            fi
        else
            warning "psql not available - cannot test PostgreSQL connection"
        fi
    fi
    
    # Test MySQL if configured
    if [ -n "${MYSQL_READONLY_DSN:-}" ]; then
        log "Testing MySQL connection..."
        # Add MySQL testing here if mysql client is available
        warning "MySQL validation not implemented in quickstart"
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
    log "Waiting for collector to start..."
    local retries=30
    while [ $retries -gt 0 ]; do
        if curl -f http://localhost:13133 &> /dev/null; then
            success "Collector is running"
            break
        fi
        
        echo -n "."
        sleep 2
        retries=$((retries - 1))
    done
    
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