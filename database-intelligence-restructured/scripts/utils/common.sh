#!/bin/bash
# Common utilities and functions for all scripts
# Provides logging, validation, and shared functionality

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Log levels
LOG_LEVEL="${LOG_LEVEL:-INFO}"

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
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_debug() {
    if [[ "$VERBOSE" == "true" || "$LOG_LEVEL" == "DEBUG" ]]; then
        echo -e "[DEBUG] $1"
    fi
}

# Error handling
set_error_trap() {
    trap 'log_error "Error on line $LINENO"' ERR
}

# Check prerequisites
check_prerequisites() {
    local prereqs=("$@")
    local missing=()
    
    for cmd in "${prereqs[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            missing+=("$cmd")
        fi
    done
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing prerequisites: ${missing[*]}"
        log_info "Please install missing tools and try again"
        exit 1
    fi
}

# Environment validation
validate_env_vars() {
    local required_vars=("$@")
    local missing=()
    
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var}" ]]; then
            missing+=("$var")
        fi
    done
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required environment variables: ${missing[*]}"
        return 1
    fi
    
    return 0
}

# Load environment file
load_env_file() {
    local env_file="${1:-.env}"
    
    if [[ -f "$env_file" ]]; then
        log_debug "Loading environment from $env_file"
        set -a
        source "$env_file"
        set +a
    else
        log_debug "No environment file found at $env_file"
    fi
}

# Check if running in CI
is_ci() {
    [[ -n "$CI" || -n "$GITHUB_ACTIONS" || -n "$JENKINS_HOME" || -n "$GITLAB_CI" ]]
}

# Get project root directory
get_project_root() {
    local current_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    
    # Navigate up until we find a marker file (go.work or Makefile)
    while [[ "$current_dir" != "/" ]]; do
        if [[ -f "$current_dir/go.work" || -f "$current_dir/Makefile" ]]; then
            echo "$current_dir"
            return
        fi
        current_dir="$(dirname "$current_dir")"
    done
    
    log_error "Could not find project root"
    exit 1
}

# Wait for service to be ready
wait_for_service() {
    local host="$1"
    local port="$2"
    local timeout="${3:-30}"
    local elapsed=0
    
    log_info "Waiting for $host:$port to be ready..."
    
    while ! nc -z "$host" "$port" 2>/dev/null; do
        if [[ $elapsed -ge $timeout ]]; then
            log_error "Timeout waiting for $host:$port"
            return 1
        fi
        
        sleep 1
        ((elapsed++))
    done
    
    log_success "$host:$port is ready"
    return 0
}

# Database connection helpers
get_postgres_dsn() {
    local host="${DB_POSTGRES_HOST:-localhost}"
    local port="${DB_POSTGRES_PORT:-5432}"
    local user="${DB_POSTGRES_USER:-postgres}"
    local pass="${DB_POSTGRES_PASSWORD:-postgres}"
    local db="${DB_POSTGRES_DATABASE:-postgres}"
    
    echo "postgresql://${user}:${pass}@${host}:${port}/${db}"
}

get_mysql_dsn() {
    local host="${DB_MYSQL_HOST:-localhost}"
    local port="${DB_MYSQL_PORT:-3306}"
    local user="${DB_MYSQL_USER:-root}"
    local pass="${DB_MYSQL_PASSWORD:-root}"
    local db="${DB_MYSQL_DATABASE:-mysql}"
    
    echo "${user}:${pass}@tcp(${host}:${port})/${db}"
}

# Docker helpers
docker_compose_up() {
    local compose_file="${1:-docker-compose.yaml}"
    local services="${2:-}"
    
    log_info "Starting Docker Compose services..."
    
    if [[ -n "$services" ]]; then
        docker compose -f "$compose_file" up -d $services
    else
        docker compose -f "$compose_file" up -d
    fi
}

docker_compose_down() {
    local compose_file="${1:-docker-compose.yaml}"
    
    log_info "Stopping Docker Compose services..."
    docker compose -f "$compose_file" down
}

# Configuration helpers
merge_yaml_configs() {
    local base_config="$1"
    local overlay_config="$2"
    local output_config="$3"
    
    if ! command -v yq &> /dev/null; then
        log_error "yq is required for YAML merging"
        return 1
    fi
    
    yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
        "$base_config" "$overlay_config" > "$output_config"
}

# Validation helpers
validate_yaml() {
    local yaml_file="$1"
    
    if ! command -v yq &> /dev/null; then
        # Fallback to python if yq not available
        python3 -c "import yaml; yaml.safe_load(open('$yaml_file'))" 2>/dev/null
    else
        yq eval '.' "$yaml_file" > /dev/null 2>&1
    fi
}

# Export functions
export -f log_info log_success log_warning log_error log_debug
export -f check_prerequisites validate_env_vars load_env_file
export -f get_project_root wait_for_service
export -f get_postgres_dsn get_mysql_dsn
export -f docker_compose_up docker_compose_down
export -f merge_yaml_configs validate_yaml