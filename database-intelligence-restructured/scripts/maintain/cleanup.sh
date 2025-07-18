#!/bin/bash
# Cleanup and maintenance script for Database Intelligence

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$SCRIPT_DIR/../utils/common.sh"

# Configuration
CLEANUP_TYPE=${1:-all}
DRY_RUN=${DRY_RUN:-false}

# Show usage
usage() {
    cat << EOF
Cleanup and Maintenance Utility

Usage: $0 [cleanup-type]

Cleanup Types:
  all         Run all cleanup tasks (default)
  archives    Clean up archived files
  build       Clean build artifacts
  docker      Clean Docker resources
  logs        Clean log files
  temp        Clean temporary files
  modules     Clean Go module cache

Options:
  DRY_RUN=true  Show what would be cleaned without doing it

Examples:
  $0                      # Run all cleanup
  $0 build                # Clean build artifacts only
  DRY_RUN=true $0 docker  # Show what Docker cleanup would do

This script helps maintain a clean development environment by:
- Removing old archived files
- Cleaning build artifacts
- Pruning Docker resources
- Removing old logs
- Cleaning temporary files
EOF
    exit 0
}

# Check for help
if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    usage
fi

print_header "Cleanup and Maintenance"
log_info "Cleanup Type: $CLEANUP_TYPE"
log_info "Dry Run: $DRY_RUN"

# Function to show size of directory
show_size() {
    local path=$1
    if [[ -d "$path" ]]; then
        local size=$(du -sh "$path" 2>/dev/null | cut -f1)
        echo "$size"
    else
        echo "0"
    fi
}

# Function to remove with dry run support
remove_item() {
    local item=$1
    local desc=${2:-$item}
    
    if [[ "$DRY_RUN" == "true" ]]; then
        if [[ -e "$item" ]]; then
            log_info "[DRY RUN] Would remove: $desc ($(show_size "$item"))"
        fi
    else
        if [[ -e "$item" ]]; then
            local size=$(show_size "$item")
            rm -rf "$item"
            log_success "Removed: $desc ($size)"
        fi
    fi
}

# Cleanup functions
cleanup_archives() {
    log_info "Cleaning up archive directories..."
    
    # Find all archive directories
    local archives=$(find "$ROOT_DIR" -type d -name "archive" -o -name "archives" | grep -v ".git")
    
    for archive in $archives; do
        # Check if it contains only old files (>30 days)
        local old_files=$(find "$archive" -type f -mtime +30 | wc -l)
        local total_files=$(find "$archive" -type f | wc -l)
        
        if [[ $old_files -eq $total_files ]] && [[ $total_files -gt 0 ]]; then
            remove_item "$archive" "Old archive: $archive"
        else
            log_info "Keeping $archive (contains recent files)"
        fi
    done
    
    # Clean specific known archives
    remove_item "$ROOT_DIR/docs/archive/project-management" "Project management archives"
    remove_item "$ROOT_DIR/tests/e2e/archive" "E2E test archives"
}

cleanup_build() {
    log_info "Cleaning build artifacts..."
    
    # Clean distribution directories
    remove_item "$ROOT_DIR/distributions/minimal/dbintel" "Minimal distribution binary"
    remove_item "$ROOT_DIR/distributions/production/dbintel" "Production distribution binary"
    remove_item "$ROOT_DIR/distributions/enterprise/dbintel" "Enterprise distribution binary"
    
    # Clean build cache
    remove_item "$ROOT_DIR/.build-cache" "Build cache"
    remove_item "$ROOT_DIR/build" "Build directory"
    
    # Clean test artifacts
    remove_item "$ROOT_DIR/coverage.out" "Coverage report"
    remove_item "$ROOT_DIR/coverage.html" "Coverage HTML"
    remove_item "$ROOT_DIR/test-report-*.txt" "Test reports"
    
    # Clean collector binaries
    remove_item "$ROOT_DIR/otelcol-*" "Collector binaries"
    remove_item "$ROOT_DIR/dbintel" "Root binary"
}

cleanup_docker() {
    log_info "Cleaning Docker resources..."
    
    if ! command_exists docker; then
        log_warning "Docker not installed, skipping Docker cleanup"
        return
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY RUN] Would clean Docker resources:"
        docker system df
    else
        # Stop and remove project containers
        local containers=$(docker ps -a --filter "label=project=database-intelligence" -q)
        if [[ -n "$containers" ]]; then
            log_info "Stopping project containers..."
            docker stop $containers
            docker rm $containers
        fi
        
        # Remove project images
        local images=$(docker images --filter "reference=dbintel*" -q)
        if [[ -n "$images" ]]; then
            log_info "Removing project images..."
            docker rmi $images
        fi
        
        # Prune system
        log_info "Pruning Docker system..."
        docker system prune -f
        
        log_success "Docker cleanup completed"
    fi
}

cleanup_logs() {
    log_info "Cleaning log files..."
    
    # Clean collector logs
    remove_item "$ROOT_DIR/collector-*.log" "Collector logs"
    remove_item "$ROOT_DIR/logs" "Logs directory"
    
    # Clean test logs
    remove_item "$ROOT_DIR/test-*.log" "Test logs"
    remove_item "$ROOT_DIR/e2e-*.log" "E2E test logs"
    
    # Find and clean old log files
    if [[ "$DRY_RUN" != "true" ]]; then
        find "$ROOT_DIR" -name "*.log" -mtime +7 -type f -delete
        log_success "Removed log files older than 7 days"
    else
        local old_logs=$(find "$ROOT_DIR" -name "*.log" -mtime +7 -type f | wc -l)
        log_info "[DRY RUN] Would remove $old_logs log files older than 7 days"
    fi
}

cleanup_temp() {
    log_info "Cleaning temporary files..."
    
    # Clean temp directories
    remove_item "$ROOT_DIR/tmp" "Temp directory"
    remove_item "$ROOT_DIR/.tmp" "Hidden temp directory"
    
    # Clean editor temp files
    if [[ "$DRY_RUN" != "true" ]]; then
        find "$ROOT_DIR" -name "*~" -o -name "*.swp" -o -name ".DS_Store" -type f -delete
        log_success "Removed editor temporary files"
    else
        local temp_files=$(find "$ROOT_DIR" -name "*~" -o -name "*.swp" -o -name ".DS_Store" -type f | wc -l)
        log_info "[DRY RUN] Would remove $temp_files temporary files"
    fi
    
    # Clean test temp files
    remove_item "/tmp/dbintel-*" "System temp files"
}

cleanup_modules() {
    log_info "Cleaning Go module cache..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[DRY RUN] Would run: go clean -modcache"
        log_info "[DRY RUN] Would run: go clean -cache"
    else
        log_info "Running go clean..."
        go clean -modcache
        go clean -cache
        log_success "Go module cache cleaned"
    fi
}

# Calculate space before cleanup
if [[ "$DRY_RUN" != "true" ]]; then
    BEFORE_SIZE=$(du -sh "$ROOT_DIR" | cut -f1)
fi

# Main execution
case "$CLEANUP_TYPE" in
    all)
        cleanup_archives
        cleanup_build
        cleanup_docker
        cleanup_logs
        cleanup_temp
        # Note: not running cleanup_modules in 'all' as it's more disruptive
        log_info "Skipping module cache cleanup (run explicitly if needed)"
        ;;
        
    archives)
        cleanup_archives
        ;;
        
    build)
        cleanup_build
        ;;
        
    docker)
        cleanup_docker
        ;;
        
    logs)
        cleanup_logs
        ;;
        
    temp)
        cleanup_temp
        ;;
        
    modules)
        cleanup_modules
        ;;
        
    *)
        log_error "Unknown cleanup type: $CLEANUP_TYPE"
        usage
        ;;
esac

# Calculate space after cleanup
if [[ "$DRY_RUN" != "true" ]]; then
    AFTER_SIZE=$(du -sh "$ROOT_DIR" | cut -f1)
    
    print_separator
    log_success "Cleanup completed!"
    log_info "Space before: $BEFORE_SIZE"
    log_info "Space after: $AFTER_SIZE"
else
    print_separator
    log_info "Dry run completed. No changes made."
    log_info "Run without DRY_RUN=true to perform actual cleanup"
fi