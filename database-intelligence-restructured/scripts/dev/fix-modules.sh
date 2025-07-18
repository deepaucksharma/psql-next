#!/bin/bash
# Fix Go module issues for Database Intelligence

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$SCRIPT_DIR/../utils/common.sh"

# Configuration
FIX_TYPE=${1:-all}
GO_VERSION="1.23.0"
OTEL_VERSION="v0.105.0"

# Show usage
usage() {
    cat << EOF
Go Module Fix Utility

Usage: $0 [fix-type]

Fix Types:
  all         Fix all module issues (default)
  paths       Fix module paths
  versions    Standardize Go versions
  otel        Fix OpenTelemetry dependencies
  tidy        Run go mod tidy on all modules
  workspace   Update go.work file

Examples:
  $0                # Fix all issues
  $0 versions       # Fix Go versions only
  $0 otel           # Fix OpenTelemetry dependencies

This script will:
- Standardize Go versions across all modules
- Fix OpenTelemetry dependency conflicts
- Update module paths if needed
- Clean up go.mod and go.sum files
EOF
    exit 0
}

# Check for help
if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    usage
fi

print_header "Go Module Fix Utility"
log_info "Fix Type: $FIX_TYPE"
log_info "Go Version: $GO_VERSION"
log_info "OTel Version: $OTEL_VERSION"

# Check requirements
check_requirements go git || exit 1

# Function to find all go.mod files
find_go_modules() {
    find "$ROOT_DIR" -name "go.mod" -type f | grep -v "/vendor/" | grep -v "/archive/" | sort
}

# Function to fix Go version in a module
fix_go_version() {
    local mod_file=$1
    local dir=$(dirname "$mod_file")
    
    log_info "Fixing Go version in $mod_file"
    
    # Update go directive
    sed -i.bak "s/^go .*/go $GO_VERSION/" "$mod_file"
    rm -f "${mod_file}.bak"
    
    # Run go mod tidy
    (cd "$dir" && go mod tidy)
}

# Function to fix OpenTelemetry dependencies
fix_otel_deps() {
    local mod_file=$1
    local dir=$(dirname "$mod_file")
    
    log_info "Fixing OTel dependencies in $mod_file"
    
    # Common OTel modules that need version alignment
    local otel_modules=(
        "go.opentelemetry.io/collector"
        "go.opentelemetry.io/collector/component"
        "go.opentelemetry.io/collector/confmap"
        "go.opentelemetry.io/collector/consumer"
        "go.opentelemetry.io/collector/exporter"
        "go.opentelemetry.io/collector/pdata"
        "go.opentelemetry.io/collector/processor"
        "go.opentelemetry.io/collector/receiver"
        "go.opentelemetry.io/collector/semconv"
    )
    
    # Update OTel dependencies to consistent version
    for module in "${otel_modules[@]}"; do
        if grep -q "$module" "$mod_file"; then
            (cd "$dir" && go get "$module@$OTEL_VERSION" 2>/dev/null || true)
        fi
    done
    
    # Run go mod tidy
    (cd "$dir" && go mod tidy)
}

# Function to fix module paths
fix_module_paths() {
    local mod_file=$1
    local expected_prefix="github.com/database-intelligence"
    
    log_info "Checking module path in $mod_file"
    
    # Get current module path
    local current_path=$(grep "^module " "$mod_file" | awk '{print $2}')
    
    # Check if it needs fixing
    if [[ ! "$current_path" =~ ^$expected_prefix ]]; then
        log_warning "Module path needs fixing: $current_path"
        
        # Calculate correct path based on directory structure
        local rel_path=$(realpath --relative-to="$ROOT_DIR" "$(dirname "$mod_file")")
        local new_path="$expected_prefix/database-intelligence/$rel_path"
        
        log_info "Updating to: $new_path"
        sed -i.bak "s|^module .*|module $new_path|" "$mod_file"
        rm -f "${mod_file}.bak"
    fi
}

# Function to update go.work file
update_workspace() {
    log_info "Updating go.work file..."
    
    cd "$ROOT_DIR"
    
    # Create go.work if it doesn't exist
    if [[ ! -f "go.work" ]]; then
        log_info "Creating go.work file..."
        go work init
    fi
    
    # Add all modules to workspace
    local modules=$(find_go_modules)
    for mod in $modules; do
        local dir=$(dirname "$mod")
        local rel_dir=$(realpath --relative-to="$ROOT_DIR" "$dir")
        
        if ! grep -q "use ./$rel_dir" go.work 2>/dev/null; then
            log_info "Adding $rel_dir to workspace"
            go work use "./$rel_dir"
        fi
    done
    
    # Sync workspace
    go work sync
}

# Function to run go mod tidy on all modules
tidy_all_modules() {
    log_info "Running go mod tidy on all modules..."
    
    local modules=$(find_go_modules)
    local count=0
    local total=$(echo "$modules" | wc -l)
    
    for mod in $modules; do
        ((count++))
        local dir=$(dirname "$mod")
        log_info "[$count/$total] Tidying $(basename "$dir")..."
        
        (cd "$dir" && go mod tidy) || {
            log_warning "Failed to tidy $dir"
        }
    done
}

# Main execution
case "$FIX_TYPE" in
    all)
        # Fix everything
        log_info "Fixing all module issues..."
        
        modules=$(find_go_modules)
        total=$(echo "$modules" | wc -l)
        log_info "Found $total Go modules"
        
        count=0
        for mod in $modules; do
            ((count++))
            print_separator
            log_info "[$count/$total] Processing $(dirname "$mod")"
            
            fix_module_paths "$mod"
            fix_go_version "$mod"
            fix_otel_deps "$mod"
        done
        
        update_workspace
        ;;
        
    paths)
        modules=$(find_go_modules)
        for mod in $modules; do
            fix_module_paths "$mod"
        done
        ;;
        
    versions)
        modules=$(find_go_modules)
        for mod in $modules; do
            fix_go_version "$mod"
        done
        ;;
        
    otel)
        modules=$(find_go_modules)
        for mod in $modules; do
            fix_otel_deps "$mod"
        done
        ;;
        
    tidy)
        tidy_all_modules
        ;;
        
    workspace)
        update_workspace
        ;;
        
    *)
        log_error "Unknown fix type: $FIX_TYPE"
        usage
        ;;
esac

# Verify fixes
print_separator
log_info "Verifying fixes..."

# Check for inconsistent Go versions
go_versions=$(find_go_modules | xargs grep "^go " | awk '{print $2}' | sort -u)
if [[ $(echo "$go_versions" | wc -l) -eq 1 ]]; then
    log_success "All modules use Go version: $go_versions"
else
    log_warning "Multiple Go versions found:"
    echo "$go_versions"
fi

# Check for module issues
if go work sync 2>&1 | grep -q "error"; then
    log_warning "Some module issues remain"
else
    log_success "No module sync errors detected"
fi

log_success "Module fix completed!"

# Provide next steps
echo
log_info "Next steps:"
log_info "1. Run 'make test' to verify everything works"
log_info "2. Commit the changes if tests pass"
log_info "3. Run 'scripts/build/build.sh' to build the project"