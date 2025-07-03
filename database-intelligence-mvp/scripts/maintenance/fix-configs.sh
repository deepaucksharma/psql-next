#!/bin/bash
# Unified Configuration Fix Script for Database Intelligence MVP
# Combines functionality from fix-all-configs.sh, fix-critical-configs.sh, and quick-fix-configs.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

# Configuration
CONFIG_DIR="${SCRIPT_DIR}/../config"
BACKUP_DIR="${SCRIPT_DIR}/../config/backups/$(date +%Y%m%d_%H%M%S)"

# Fix modes
MODE_QUICK="quick"
MODE_CRITICAL="critical"  
MODE_ALL="all"

# Default mode
MODE="${1:-${MODE_CRITICAL}}"

usage() {
    cat << EOF
Usage: $0 [MODE]

Unified configuration fix script with multiple modes:

MODES:
  quick     - Fix only environment variable syntax (fastest)
  critical  - Fix critical configuration issues (default)
  all       - Comprehensive fixes for all configurations (slowest)

EXAMPLES:
  $0 quick     # Quick env var fixes
  $0 critical  # Fix critical issues
  $0 all       # Full configuration fixes
  $0           # Default: critical fixes

EOF
}

create_backup() {
    log_info "Creating configuration backup in: $BACKUP_DIR"
    mkdir -p "$BACKUP_DIR"
    
    find "$CONFIG_DIR" -name "*.yaml" -o -name "*.yml" | while read -r file; do
        if [[ -f "$file" ]]; then
            cp "$file" "$BACKUP_DIR/"
        fi
    done
    
    log_success "Backup created successfully"
}

fix_environment_variables() {
    local file=$1
    log_debug "Fixing environment variables in: $file"
    
    # Fix PostgreSQL environment variables
    sed -i.tmp 's/${POSTGRES_HOST:/${env:POSTGRES_HOST:-/g' "$file"
    sed -i.tmp 's/${POSTGRES_PORT:/${env:POSTGRES_PORT:-/g' "$file"
    sed -i.tmp 's/${POSTGRES_USER:/${env:POSTGRES_USER:-/g' "$file"
    sed -i.tmp 's/${POSTGRES_PASSWORD:/${env:POSTGRES_PASSWORD:-/g' "$file"
    sed -i.tmp 's/${POSTGRES_DB:/${env:POSTGRES_DB:-/g' "$file"
    sed -i.tmp 's/${POSTGRES_DATABASE:/${env:POSTGRES_DATABASE:-/g' "$file"
    
    # Fix MySQL environment variables
    sed -i.tmp 's/${MYSQL_HOST:/${env:MYSQL_HOST:-/g' "$file"
    sed -i.tmp 's/${MYSQL_PORT:/${env:MYSQL_PORT:-/g' "$file"
    sed -i.tmp 's/${MYSQL_USER:/${env:MYSQL_USER:-/g' "$file"
    sed -i.tmp 's/${MYSQL_PASSWORD:/${env:MYSQL_PASSWORD:-/g' "$file"
    sed -i.tmp 's/${MYSQL_DB:/${env:MYSQL_DB:-/g' "$file"
    sed -i.tmp 's/${MYSQL_DATABASE:/${env:MYSQL_DATABASE:-/g' "$file"
    
    # Fix New Relic and other common variables
    sed -i.tmp 's/${NEW_RELIC_LICENSE_KEY}/${env:NEW_RELIC_LICENSE_KEY}/g' "$file"
    sed -i.tmp 's/${NEW_RELIC_API_KEY}/${env:NEW_RELIC_API_KEY}/g' "$file"
    sed -i.tmp 's/${HOSTNAME}/${env:HOSTNAME}/g' "$file"
    sed -i.tmp 's/${OTLP_ENDPOINT}/${env:OTLP_ENDPOINT}/g' "$file"
    
    # Clean up temporary files
    rm -f "${file}.tmp"
}

fix_memory_limiter() {
    local file=$1
    log_debug "Fixing memory limiter configuration in: $file"
    
    # Replace percentage-based with MiB-based limits
    sed -i.tmp 's/limit_percentage: *[0-9]*/limit_mib: 1024/g' "$file"
    sed -i.tmp 's/spike_limit_percentage: *[0-9]*/spike_limit_mib: 256/g' "$file"
    
    # Ensure memory_limiter is properly configured
    if grep -q "memory_limiter:" "$file"; then
        sed -i.tmp '/memory_limiter:/,/^[[:space:]]*[^[:space:]]/ {
            /limit_percentage:/d
            /spike_limit_percentage:/d
            /memory_limiter:/a\
      limit_mib: 1024\
      spike_limit_mib: 256\
      check_interval: 1s
        }' "$file"
    fi
    
    rm -f "${file}.tmp"
}

fix_telemetry_config() {
    local file=$1
    log_debug "Fixing telemetry configuration in: $file"
    
    # Fix telemetry section structure
    if grep -q "service:" "$file"; then
        # Update telemetry configuration
        sed -i.tmp '/service:/,$ {
            /telemetry:/,/^[[:space:]]*[^[:space:]]/ {
                /logs:/,/^[[:space:]]*[^[:space:]]/ {
                    s/level: .*/level: info/
                    /development:/d
                    /sampling:/d
                    /encoding:/d
                }
                /metrics:/,/^[[:space:]]*[^[:space:]]/ {
                    s/address: .*/address: 0.0.0.0:8888/
                    /level:/d
                }
            }
        }' "$file"
    fi
    
    rm -f "${file}.tmp"
}

fix_processor_config() {
    local file=$1
    log_debug "Fixing processor configuration in: $file"
    
    # Fix batch processor
    sed -i.tmp '/batch:/,/^[[:space:]]*[^[:space:]]/ {
        s/send_batch_size: *[0-9]*/send_batch_size: 1024/
        s/send_batch_max_size: *[0-9]*/send_batch_max_size: 2048/
        s/timeout: *.*/timeout: 5s/
    }' "$file"
    
    # Fix resource processor
    if grep -q "resource:" "$file"; then
        sed -i.tmp '/resource:/,/^[[:space:]]*[^[:space:]]/ {
            /attributes:/,/^[[:space:]]*[^[:space:]]/ {
                /service\.name:/d
                /service\.version:/d
                /service\.instance\.id:/d
            }
        }' "$file"
    fi
    
    rm -f "${file}.tmp"
}

fix_receiver_config() {
    local file=$1
    log_debug "Fixing receiver configuration in: $file"
    
    # Fix PostgreSQL receiver
    sed -i.tmp '/postgresql:/,/^[[:space:]]*[^[:space:]]/ {
        s/collection_interval: *.*/collection_interval: 60s/
        /transport:/d
        /tls:/,/^[[:space:]]*[^[:space:]]/ {
            /insecure:/d
        }
    }' "$file"
    
    # Fix MySQL receiver  
    sed -i.tmp '/mysql:/,/^[[:space:]]*[^[:space:]]/ {
        s/collection_interval: *.*/collection_interval: 60s/
        /transport:/d
        /tls:/,/^[[:space:]]*[^[:space:]]/ {
            /insecure:/d
        }
    }' "$file"
    
    rm -f "${file}.tmp"
}

fix_exporter_config() {
    local file=$1
    log_debug "Fixing exporter configuration in: $file"
    
    # Fix OTLP HTTP exporter
    sed -i.tmp '/otlphttp:/,/^[[:space:]]*[^[:space:]]/ {
        /compression:/d
        /timeout: *[0-9]/s/timeout: *.*/timeout: 30s/
        /retry_on_failure:/,/^[[:space:]]*[^[:space:]]/ {
            s/enabled: .*/enabled: true/
            s/initial_interval: *.*/initial_interval: 5s/
            s/max_interval: *.*/max_interval: 30s/
            s/max_elapsed_time: *.*/max_elapsed_time: 5m/
        }
    }' "$file"
    
    rm -f "${file}.tmp"
}

fix_pipeline_config() {
    local file=$1
    log_debug "Fixing pipeline configuration in: $file"
    
    # Ensure proper pipeline order
    sed -i.tmp '/pipelines:/,$ {
        /processors:/,/^[[:space:]]*[^[:space:]]/ {
            s/- memory_limiter/- memory_limiter/
            s/- resource/- resource/
            s/- batch/- batch/
        }
    }' "$file"
    
    rm -f "${file}.tmp"
}

apply_quick_fixes() {
    local file=$1
    log_info "Applying quick fixes to: $(basename "$file")"
    
    fix_environment_variables "$file"
    fix_memory_limiter "$file"
}

apply_critical_fixes() {
    local file=$1
    log_info "Applying critical fixes to: $(basename "$file")"
    
    fix_environment_variables "$file"
    fix_memory_limiter "$file"
    fix_telemetry_config "$file"
    fix_processor_config "$file"
}

apply_all_fixes() {
    local file=$1
    log_info "Applying comprehensive fixes to: $(basename "$file")"
    
    fix_environment_variables "$file"
    fix_memory_limiter "$file"
    fix_telemetry_config "$file"
    fix_processor_config "$file"
    fix_receiver_config "$file"
    fix_exporter_config "$file"
    fix_pipeline_config "$file"
}

validate_config_file() {
    local file=$1
    
    # Basic YAML syntax check
    if command -v yq &> /dev/null; then
        if ! yq eval . "$file" > /dev/null 2>&1; then
            log_error "YAML syntax error in: $file"
            return 1
        fi
    fi
    
    # Check for required sections
    if ! grep -q "receivers:" "$file"; then
        log_warning "Missing receivers section in: $file"
    fi
    
    if ! grep -q "processors:" "$file"; then
        log_warning "Missing processors section in: $file"
    fi
    
    if ! grep -q "exporters:" "$file"; then
        log_warning "Missing exporters section in: $file"
    fi
    
    return 0
}

process_config_files() {
    local fix_function=$1
    local file_count=0
    local success_count=0
    
    # Find all YAML configuration files
    while IFS= read -r -d '' file; do
        file_count=$((file_count + 1))
        
        if [[ -f "$file" && -w "$file" ]]; then
            log_debug "Processing: $file"
            
            if $fix_function "$file"; then
                if validate_config_file "$file"; then
                    success_count=$((success_count + 1))
                    log_success "Fixed: $(basename "$file")"
                else
                    log_error "Validation failed: $(basename "$file")"
                fi
            else
                log_error "Fix failed: $(basename "$file")"
            fi
        else
            log_warning "Skipped (not writable): $(basename "$file")"
        fi
    done < <(find "$CONFIG_DIR" -name "*.yaml" -o -name "*.yml" -print0)
    
    log_info "Processed $success_count of $file_count configuration files"
}

main() {
    log_info "Starting unified configuration fixes in mode: $MODE"
    
    # Validate mode
    case "$MODE" in
        "$MODE_QUICK"|"$MODE_CRITICAL"|"$MODE_ALL")
            ;;
        "-h"|"--help"|"help")
            usage
            exit 0
            ;;
        *)
            log_error "Invalid mode: $MODE"
            usage
            exit 1
            ;;
    esac
    
    # Check if config directory exists
    if [[ ! -d "$CONFIG_DIR" ]]; then
        log_error "Configuration directory not found: $CONFIG_DIR"
        exit 1
    fi
    
    # Create backup
    create_backup
    
    # Apply fixes based on mode
    case "$MODE" in
        "$MODE_QUICK")
            log_info "Applying quick fixes (environment variables and memory limiter)"
            process_config_files apply_quick_fixes
            ;;
        "$MODE_CRITICAL")
            log_info "Applying critical fixes (env vars, memory, telemetry, processors)"
            process_config_files apply_critical_fixes
            ;;
        "$MODE_ALL")
            log_info "Applying comprehensive fixes (all configuration aspects)"
            process_config_files apply_all_fixes
            ;;
    esac
    
    log_success "Configuration fixes completed successfully!"
    log_info "Backup available at: $BACKUP_DIR"
    log_info "You can restore with: cp $BACKUP_DIR/*.yaml $CONFIG_DIR/"
}

# Run main function
main "$@"