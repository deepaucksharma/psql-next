#!/bin/bash

# Common library functions for database-intelligence scripts
# Source this file in other scripts: source "$(dirname "$0")/scripts/lib/common.sh"

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
}

# Check prerequisites
check_prerequisites() {
    local failed=false
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        failed=true
    fi
    
    if ! command -v docker-compose &> /dev/null && ! command -v docker compose &> /dev/null; then
        log_error "Docker Compose is not installed"
        failed=true
    fi
    
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        failed=true
    fi
    
    if [ "$failed" = true ]; then
        return 1
    fi
    
    return 0
}

# Get docker compose command
get_docker_compose_cmd() {
    if command -v docker-compose &> /dev/null; then
        echo "docker-compose"
    else
        echo "docker compose"
    fi
}

# Database management functions
start_databases() {
    local compose_file="${1:-deployments/docker/compose/docker-compose-databases.yaml}"
    local docker_compose=$(get_docker_compose_cmd)
    
    log_info "Starting test databases..."
    $docker_compose -f "$compose_file" up -d postgres mysql
    
    # Wait for databases to be ready
    wait_for_postgres
    wait_for_mysql
}

stop_databases() {
    local compose_file="${1:-deployments/docker/compose/docker-compose-databases.yaml}"
    local docker_compose=$(get_docker_compose_cmd)
    
    log_info "Stopping databases..."
    $docker_compose -f "$compose_file" down -v
}

wait_for_postgres() {
    log_info "Waiting for PostgreSQL to be ready..."
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if docker exec postgres pg_isready -U testuser -d testdb &> /dev/null; then
            log_success "PostgreSQL is ready"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    log_error "PostgreSQL failed to start after $max_attempts attempts"
    return 1
}

wait_for_mysql() {
    log_info "Waiting for MySQL to be ready..."
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if docker exec mysql mysqladmin ping -h localhost -u testuser -ptestpass --silent &> /dev/null; then
            log_success "MySQL is ready"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    log_error "MySQL failed to start after $max_attempts attempts"
    return 1
}

# Test database connectivity
test_database_connectivity() {
    log_info "Testing database connectivity..."
    
    # Test PostgreSQL
    if docker exec postgres psql -U testuser -d testdb -c "SELECT 1" &> /dev/null; then
        log_success "PostgreSQL connection successful"
    else
        log_error "PostgreSQL connection failed"
        return 1
    fi
    
    # Test MySQL
    if docker exec mysql mysql -u testuser -ptestpass -e "SELECT 1" testdb &> /dev/null; then
        log_success "MySQL connection successful"
    else
        log_error "MySQL connection failed"
        return 1
    fi
    
    return 0
}

# Environment file management
ensure_env_file() {
    if [ ! -f .env ]; then
        log_info "Creating .env file from template..."
        cat > .env << 'EOF'
# Database Intelligence Environment Configuration

# PostgreSQL Configuration
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=testuser
POSTGRES_PASSWORD=testpass
POSTGRES_DB=testdb

# MySQL Configuration
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=testuser
MYSQL_PASSWORD=testpass
MYSQL_DB=testdb

# New Relic Configuration (optional)
NEW_RELIC_API_KEY=your-api-key-here
NEW_RELIC_ACCOUNT_ID=your-account-id-here

# Collector Configuration
OTEL_COLLECTOR_PORT=4317
OTEL_METRICS_PORT=8888
OTEL_HEALTH_PORT=13133
EOF
        log_success "Created .env file"
    fi
}

# Go workspace management
sync_go_workspace() {
    if [ -f go.work ]; then
        log_info "Syncing Go workspace..."
        go work sync
    fi
}

# Collector validation
validate_collector_config() {
    local config_file="$1"
    local collector_binary="${2:-./otelcol}"
    
    if [ ! -f "$collector_binary" ]; then
        log_error "Collector binary not found at $collector_binary"
        return 1
    fi
    
    log_info "Validating collector configuration: $config_file"
    if $collector_binary validate --config="$config_file"; then
        log_success "Configuration is valid"
        return 0
    else
        log_error "Configuration validation failed"
        return 1
    fi
}

# Report generation helpers
generate_report_header() {
    local title="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    cat << EOF
# $title
Generated: $timestamp

---

EOF
}

calculate_success_rate() {
    local passed=$1
    local total=$2
    
    if [ "$total" -eq 0 ]; then
        echo "0"
    else
        echo "$((passed * 100 / total))"
    fi
}