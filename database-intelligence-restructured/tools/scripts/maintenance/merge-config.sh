#!/bin/bash

# Configuration Merging Script
# Combines base templates with environment-specific overrides

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_DIR="$PROJECT_ROOT/config"
BASE_DIR="$CONFIG_DIR/base"
ENV_DIR="$CONFIG_DIR/environments"
OUTPUT_DIR="$CONFIG_DIR/generated"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# Check dependencies
check_dependencies() {
    local missing_deps=()
    
    if ! command -v yq &> /dev/null; then
        missing_deps+=("yq")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_info "Install with: brew install yq (macOS) or apt-get install yq (Ubuntu)"
        exit 1
    fi
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] <environment> [overlays...]

Generate consolidated OpenTelemetry collector configuration by merging base templates
with environment-specific overrides and optional feature overlays.

ARGUMENTS:
    environment     Target environment (development, staging, production)
    overlays        Optional feature overlays (querylens, enterprise-gateway, plan-intelligence)

OPTIONS:
    -o, --output DIR    Output directory (default: $OUTPUT_DIR)
    -v, --validate      Validate generated configuration
    -f, --force         Overwrite existing output files
    -h, --help          Show this help message

EXAMPLES:
    $0 development                                    # Generate development config
    $0 production -v                                  # Generate and validate production config
    $0 staging querylens plan-intelligence           # Generate staging with overlays
    $0 production enterprise-gateway -o /tmp/configs # Production with enterprise features

AVAILABLE OVERLAYS:
    querylens         - PostgreSQL QueryLens integration with plan tracking
    enterprise-gateway - Advanced enterprise features (tail sampling, auth)
    plan-intelligence - SQL plan analysis and regression detection

ENVIRONMENT FILES:
    Base templates are located in: $BASE_DIR/
    Environment overrides are in: $ENV_DIR/
    Feature overlays are in: $CONFIG_DIR/overlays/

GENERATED OUTPUT:
    collector-<environment>[+overlays].yaml         # Main collector configuration
    docker-compose-<environment>.yaml               # Docker Compose override
    k8s-<environment>/                               # Kubernetes manifests

EOF
}

# Validate environment
validate_environment() {
    local env="$1"
    local env_file="$ENV_DIR/${env}.yaml"
    
    if [[ ! -f "$env_file" ]]; then
        log_error "Environment file not found: $env_file"
        log_info "Available environments:"
        find "$ENV_DIR" -name "*.yaml" -exec basename {} .yaml \; | sort
        exit 1
    fi
}

# Create output directory
create_output_dir() {
    local output_dir="$1"
    
    if [[ ! -d "$output_dir" ]]; then
        log_info "Creating output directory: $output_dir"
        mkdir -p "$output_dir"
    fi
}

# Merge YAML configurations
merge_yaml_configs() {
    local base_file="$1"
    local env_file="$2"
    local output_file="$3"
    
    log_info "Merging $(basename "$base_file") with $(basename "$env_file")"
    
    # Use yq to merge configurations
    # Environment file takes precedence over base file
    yq eval-all '. as $item ireduce ({}; . * $item)' "$base_file" "$env_file" > "$output_file"
    
    if [[ $? -eq 0 ]]; then
        log_success "Generated: $(basename "$output_file")"
    else
        log_error "Failed to merge configurations"
        return 1
    fi
}

# Generate full collector configuration
generate_collector_config() {
    local environment="$1"
    local output_dir="$2"
    local force="$3"
    shift 3
    local overlays=("$@")
    
    local env_file="$ENV_DIR/${environment}.yaml"
    
    # Generate output filename with overlays
    local overlay_suffix=""
    if [[ ${#overlays[@]} -gt 0 ]]; then
        overlay_suffix="+$(IFS=+; echo "${overlays[*]}")"
    fi
    local output_file="$output_dir/collector-${environment}${overlay_suffix}.yaml"
    
    # Check if output exists and force flag
    if [[ -f "$output_file" && "$force" != "true" ]]; then
        log_warning "Output file exists: $output_file (use -f to overwrite)"
        return 1
    fi
    
    log_info "Generating collector configuration for environment: $environment"
    if [[ ${#overlays[@]} -gt 0 ]]; then
        log_info "Including overlays: ${overlays[*]}"
    fi
    
    # Create temporary combined base file
    local temp_base=$(mktemp)
    trap "rm -f $temp_base" EXIT
    
    # Combine all base templates
    cat > "$temp_base" << 'EOF'
# Generated Configuration - DO NOT EDIT MANUALLY
# Generated by merge-config.sh from base templates and environment overrides
#
# Base templates included:
# - extensions-base.yaml
# - receivers-base.yaml  
# - processors-base.yaml
# - exporters-base.yaml
#

EOF
    
    # Merge base configurations in order
    for base_file in extensions-base.yaml receivers-base.yaml processors-base.yaml exporters-base.yaml; do
        local full_base_path="$BASE_DIR/$base_file"
        if [[ -f "$full_base_path" ]]; then
            echo "# --- $base_file ---" >> "$temp_base"
            cat "$full_base_path" >> "$temp_base"
            echo "" >> "$temp_base"
        else
            log_warning "Base file not found: $full_base_path"
        fi
    done
    
    # Create temporary file for merging with environment
    local temp_env=$(mktemp)
    trap "rm -f $temp_env" EXIT
    merge_yaml_configs "$temp_base" "$env_file" "$temp_env"
    
    # Apply overlays if specified
    local final_config="$temp_env"
    for overlay in "${overlays[@]}"; do
        local overlay_file="$CONFIG_DIR/overlays/${overlay}-overlay.yaml"
        if [[ -f "$overlay_file" ]]; then
            log_info "Applying overlay: $overlay"
            local temp_overlay=$(mktemp)
            trap "rm -f $temp_overlay" EXIT
            merge_yaml_configs "$final_config" "$overlay_file" "$temp_overlay"
            final_config="$temp_overlay"
        else
            log_warning "Overlay file not found: $overlay_file"
        fi
    done
    
    # Copy final configuration to output
    cp "$final_config" "$output_file"
    
    # Add generation metadata
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    yq eval ".metadata.generated_at = \"$timestamp\"" -i "$output_file"
    yq eval ".metadata.environment = \"$environment\"" -i "$output_file"
    yq eval ".metadata.overlays = [\"$(IFS='","'; echo "${overlays[*]}")\"]" -i "$output_file"
    yq eval ".metadata.generator = \"merge-config.sh\"" -i "$output_file"
}

# Validate configuration
validate_config() {
    local config_file="$1"
    
    log_info "Validating configuration: $(basename "$config_file")"
    
    # Check YAML syntax
    if ! yq eval '.' "$config_file" >/dev/null 2>&1; then
        log_error "Invalid YAML syntax in $config_file"
        return 1
    fi
    
    # Check required sections
    local required_sections=("service" "receivers" "processors" "exporters")
    for section in "${required_sections[@]}"; do
        if ! yq eval "has(\"$section\")" "$config_file" | grep -q "true"; then
            log_error "Missing required section: $section"
            return 1
        fi
    done
    
    # Check service pipelines
    if ! yq eval '.service | has("pipelines")' "$config_file" | grep -q "true"; then
        log_error "Missing service.pipelines configuration"
        return 1
    fi
    
    log_success "Configuration validation passed"
}

# Generate Docker Compose override
generate_docker_compose() {
    local environment="$1"
    local output_dir="$2"
    local force="$3"
    
    local output_file="$output_dir/docker-compose-${environment}.yaml"
    
    if [[ -f "$output_file" && "$force" != "true" ]]; then
        log_warning "Docker Compose file exists: $output_file (use -f to overwrite)"
        return 1
    fi
    
    log_info "Generating Docker Compose configuration for: $environment"
    
    # Generate environment-specific Docker Compose
    cat > "$output_file" << EOF
# Docker Compose Configuration for $environment
# Generated by merge-config.sh

version: '3.8'

services:
  collector:
    volumes:
      - $output_dir/collector-${environment}.yaml:/etc/otelcol/config.yaml:ro
    environment:
      - DEPLOYMENT_ENVIRONMENT=$environment
      - OTEL_LOG_LEVEL=\${OTEL_LOG_LEVEL:-info}
      - SERVICE_NAME=\${SERVICE_NAME:-database-intelligence-collector}
      - SERVICE_VERSION=\${SERVICE_VERSION:-2.0.0}

# Environment-specific overrides can be added here
EOF

    case "$environment" in
        "development")
            cat >> "$output_file" << 'EOF'
      - OTEL_LOG_LEVEL=${OTEL_LOG_LEVEL:-debug}
    profiles: ["dev"]
EOF
            ;;
        "production")
            cat >> "$output_file" << 'EOF'
      - OTEL_LOG_LEVEL=${OTEL_LOG_LEVEL:-info}
    profiles: ["prod"]
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '2.0'
        reservations:
          memory: 512M
          cpus: '1.0'
EOF
            ;;
    esac
    
    log_success "Generated: $(basename "$output_file")"
}

# Main execution
main() {
    local environment=""
    local output_dir="$OUTPUT_DIR"
    local validate_flag="false"
    local force_flag="false"
    local overlays=()
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -o|--output)
                output_dir="$2"
                shift 2
                ;;
            -v|--validate)
                validate_flag="true"
                shift
                ;;
            -f|--force)
                force_flag="true"
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            -*)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
            *)
                if [[ -z "$environment" ]]; then
                    environment="$1"
                else
                    # Remaining arguments are overlays
                    overlays+=("$1")
                fi
                shift
                ;;
        esac
    done
    
    # Validate required arguments
    if [[ -z "$environment" ]]; then
        log_error "Environment argument is required"
        show_usage
        exit 1
    fi
    
    # Check dependencies
    check_dependencies
    
    # Validate environment
    validate_environment "$environment"
    
    # Create output directory
    create_output_dir "$output_dir"
    
    # Generate configurations
    log_info "Starting configuration generation for environment: $environment"
    if [[ ${#overlays[@]} -gt 0 ]]; then
        log_info "With overlays: ${overlays[*]}"
    fi
    
    if generate_collector_config "$environment" "$output_dir" "$force_flag" "${overlays[@]}"; then
        if generate_docker_compose "$environment" "$output_dir" "$force_flag"; then
            # Generate validation filename with overlays
            local overlay_suffix=""
            if [[ ${#overlays[@]} -gt 0 ]]; then
                overlay_suffix="+$(IFS=+; echo "${overlays[*]}")"
            fi
            
            if [[ "$validate_flag" == "true" ]]; then
                validate_config "$output_dir/collector-${environment}${overlay_suffix}.yaml"
            fi
            
            log_success "Configuration generation completed successfully!"
            log_info "Generated files in: $output_dir"
            log_info "  - collector-${environment}${overlay_suffix}.yaml"
            log_info "  - docker-compose-${environment}.yaml"
        else
            log_error "Failed to generate Docker Compose configuration"
            exit 1
        fi
    else
        log_error "Failed to generate collector configuration"
        exit 1
    fi
}

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi