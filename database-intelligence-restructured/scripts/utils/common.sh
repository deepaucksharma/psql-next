#!/bin/bash
# Common utilities for Database Intelligence scripts

# Colors
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export MAGENTA='\033[0;35m'
export CYAN='\033[0;36m'
export WHITE='\033[1;37m'
export NC='\033[0m' # No Color

# Icons
export CHECK_MARK="✓"
export CROSS_MARK="✗"
export WARNING_SIGN="⚠"
export INFO_SIGN="ℹ"
export ROCKET="🚀"
export GEAR="⚙"
export PACKAGE="📦"
export FOLDER="📁"

# Logging functions
log_info() {
    echo -e "${BLUE}${INFO_SIGN}${NC} $1"
}

log_success() {
    echo -e "${GREEN}${CHECK_MARK}${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}${WARNING_SIGN}${NC} $1"
}

log_error() {
    echo -e "${RED}${CROSS_MARK}${NC} $1" >&2
}

log_debug() {
    if [[ "${DEBUG:-false}" == "true" ]]; then
        echo -e "${CYAN}[DEBUG]${NC} $1"
    fi
}

# Print formatted header
print_header() {
    local title="$1"
    local width=60
    local padding=$(( (width - ${#title} - 2) / 2 ))
    
    echo
    echo -e "${BLUE}$(printf '=%.0s' $(seq 1 $width))${NC}"
    echo -e "${BLUE}=$(printf ' %.0s' $(seq 1 $padding))${WHITE}$title${BLUE}$(printf ' %.0s' $(seq 1 $padding))=${NC}"
    echo -e "${BLUE}$(printf '=%.0s' $(seq 1 $width))${NC}"
    echo
}

# Print separator line
print_separator() {
    echo -e "${BLUE}$(printf '─%.0s' $(seq 1 60))${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check required commands
check_requirements() {
    local requirements=("$@")
    local missing=()
    
    for cmd in "${requirements[@]}"; do
        if ! command_exists "$cmd"; then
            missing+=("$cmd")
        fi
    done
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required commands: ${missing[*]}"
        return 1
    fi
    
    return 0
}

# Get project root directory
get_project_root() {
    local current_dir="$PWD"
    while [[ "$current_dir" != "/" ]]; do
        if [[ -f "$current_dir/go.mod" ]] && grep -q "module github.com/database-intelligence" "$current_dir/go.mod" 2>/dev/null; then
            echo "$current_dir"
            return 0
        fi
        current_dir="$(dirname "$current_dir")"
    done
    
    # Fallback to git root
    git rev-parse --show-toplevel 2>/dev/null || echo "$PWD"
}

# Check if running in CI environment
is_ci() {
    [[ "${CI:-false}" == "true" ]] || [[ -n "${GITHUB_ACTIONS:-}" ]] || [[ -n "${JENKINS_HOME:-}" ]]
}

# Create temporary directory
create_temp_dir() {
    local prefix="${1:-dbintel}"
    mktemp -d -t "${prefix}-XXXXXX"
}

# Clean up function
cleanup() {
    local exit_code=$?
    if [[ -n "${TEMP_DIR:-}" ]] && [[ -d "$TEMP_DIR" ]]; then
        log_debug "Cleaning up temporary directory: $TEMP_DIR"
        rm -rf "$TEMP_DIR"
    fi
    exit $exit_code
}

# Set up trap for cleanup
setup_cleanup_trap() {
    trap cleanup EXIT INT TERM
}

# Wait for service to be ready
wait_for_service() {
    local service_name="$1"
    local port="$2"
    local timeout="${3:-30}"
    local host="${4:-localhost}"
    
    log_info "Waiting for $service_name to be ready on $host:$port..."
    
    local count=0
    while ! nc -z "$host" "$port" >/dev/null 2>&1; do
        if [[ $count -ge $timeout ]]; then
            log_error "$service_name failed to start within ${timeout}s"
            return 1
        fi
        sleep 1
        ((count++))
    done
    
    log_success "$service_name is ready!"
    return 0
}

# Check Docker availability
check_docker() {
    if ! command_exists docker; then
        log_error "Docker is not installed"
        return 1
    fi
    
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker daemon is not running"
        return 1
    fi
    
    return 0
}

# Get container status
get_container_status() {
    local container_name="$1"
    docker ps -a --filter "name=$container_name" --format "{{.Status}}" | head -1
}

# Check if container is running
is_container_running() {
    local container_name="$1"
    local status=$(get_container_status "$container_name")
    [[ "$status" =~ ^Up ]]
}

# Execute command with timeout
execute_with_timeout() {
    local timeout="$1"
    shift
    
    if command_exists timeout; then
        timeout "$timeout" "$@"
    else
        # Fallback for systems without timeout command
        "$@" &
        local pid=$!
        
        ( sleep "$timeout" && kill -TERM $pid 2>/dev/null ) &
        local watcher=$!
        
        wait $pid 2>/dev/null
        local exit_code=$?
        
        kill $watcher 2>/dev/null
        wait $watcher 2>/dev/null
        
        return $exit_code
    fi
}

# Generate timestamp
timestamp() {
    date +"%Y-%m-%d %H:%M:%S"
}

# Generate filename-safe timestamp
file_timestamp() {
    date +"%Y%m%d-%H%M%S"
}

# Calculate duration
calculate_duration() {
    local start_time="$1"
    local end_time="$2"
    local duration=$((end_time - start_time))
    
    if [[ $duration -lt 60 ]]; then
        echo "${duration}s"
    elif [[ $duration -lt 3600 ]]; then
        echo "$((duration / 60))m $((duration % 60))s"
    else
        echo "$((duration / 3600))h $((duration % 3600 / 60))m"
    fi
}

# Export all functions
export -f log_info log_success log_warning log_error log_debug
export -f print_header print_separator
export -f command_exists check_requirements
export -f get_project_root is_ci
export -f create_temp_dir cleanup setup_cleanup_trap
export -f wait_for_service check_docker
export -f get_container_status is_container_running
export -f execute_with_timeout
export -f timestamp file_timestamp calculate_duration