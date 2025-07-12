#!/bin/bash
# Unified deployment script for Database Intelligence
# Handles Docker, Kubernetes, and binary deployments

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$PROJECT_ROOT/scripts/utils/common.sh"

# Deployment configuration
DEPLOY_TYPE="${1:-docker}"
ENVIRONMENT="${2:-production}"
DRY_RUN="${DRY_RUN:-false}"

usage() {
    cat << EOF
Usage: $0 [DEPLOY_TYPE] [ENVIRONMENT] [OPTIONS]

Deploy Database Intelligence Collector

Deploy Types:
  docker           Deploy using Docker Compose (default)
  kubernetes       Deploy to Kubernetes cluster
  binary           Deploy as system service
  parallel         Deploy parallel config-only and enhanced modes

Environments:
  production       Production environment (default)
  staging          Staging environment
  development      Development environment

Environment Variables:
  DRY_RUN=true     Show what would be deployed without deploying
  NAMESPACE=name   Kubernetes namespace (default: database-intelligence)
  SERVICE_NAME=name Override service name

Examples:
  $0 docker production           # Deploy to Docker in production
  $0 kubernetes staging          # Deploy to Kubernetes staging
  $0 binary production           # Deploy as systemd service

EOF
    exit 1
}

# Validate deployment prerequisites
validate_prerequisites() {
    case "$DEPLOY_TYPE" in
        docker)
            check_prerequisites docker
            ;;
        kubernetes|k8s)
            check_prerequisites kubectl helm
            ;;
        binary)
            check_prerequisites systemctl
            ;;
    esac
}

# Load environment configuration
load_environment() {
    local env_file="$PROJECT_ROOT/.env.$ENVIRONMENT"
    
    if [[ -f "$env_file" ]]; then
        log_info "Loading environment: $ENVIRONMENT"
        load_env_file "$env_file"
    else
        log_warning "Environment file not found: $env_file"
        log_info "Using default environment variables"
    fi
    
    # Validate required variables
    validate_env_vars NEW_RELIC_LICENSE_KEY DB_POSTGRES_HOST DB_POSTGRES_USER
}

# Deploy with Docker Compose
deploy_docker() {
    log_info "Deploying with Docker Compose..."
    
    cd "$PROJECT_ROOT/deployments/docker"
    
    local compose_file="compose/docker-compose.$ENVIRONMENT.yaml"
    
    if [[ ! -f "$compose_file" ]]; then
        compose_file="compose/docker-compose.yaml"
        log_warning "Environment-specific compose file not found, using default"
    fi
    
    # Generate config from template if needed
    generate_config
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would deploy using $compose_file"
        docker compose -f "$compose_file" config
        return
    fi
    
    # Pull latest images
    docker compose -f "$compose_file" pull
    
    # Deploy
    docker compose -f "$compose_file" up -d
    
    # Wait for health check
    wait_for_health_check
    
    log_success "Docker deployment completed"
}

# Deploy to Kubernetes
deploy_kubernetes() {
    log_info "Deploying to Kubernetes..."
    
    local namespace="${NAMESPACE:-database-intelligence}"
    
    # Create namespace if it doesn't exist
    kubectl create namespace "$namespace" --dry-run=client -o yaml | kubectl apply -f -
    
    # Create secrets
    create_k8s_secrets "$namespace"
    
    # Generate ConfigMap from config
    generate_config
    kubectl create configmap otel-config \
        --from-file=config.yaml="$PROJECT_ROOT/runtime/collector-config.yaml" \
        --namespace="$namespace" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would deploy to namespace $namespace"
        return
    fi
    
    # Deploy using Helm if available
    if command -v helm &> /dev/null && [[ -d "$PROJECT_ROOT/deployments/kubernetes/helm" ]]; then
        deploy_with_helm "$namespace"
    else
        deploy_with_kubectl "$namespace"
    fi
    
    # Wait for deployment
    kubectl wait --for=condition=available --timeout=300s \
        deployment/database-intelligence -n "$namespace"
    
    log_success "Kubernetes deployment completed"
}

# Deploy with Helm
deploy_with_helm() {
    local namespace="$1"
    
    cd "$PROJECT_ROOT/deployments/kubernetes/helm"
    
    # Update dependencies
    helm dependency update
    
    # Install or upgrade
    helm upgrade --install database-intelligence . \
        --namespace "$namespace" \
        --values "values.$ENVIRONMENT.yaml" \
        --set image.tag="$VERSION" \
        --set newrelic.licenseKey="$NEW_RELIC_LICENSE_KEY" \
        --wait
}

# Deploy with kubectl
deploy_with_kubectl() {
    local namespace="$1"
    
    cd "$PROJECT_ROOT/deployments/kubernetes"
    
    # Apply manifests
    kubectl apply -f manifests/ -n "$namespace"
}

# Create Kubernetes secrets
create_k8s_secrets() {
    local namespace="$1"
    
    kubectl create secret generic database-intelligence-secrets \
        --from-literal=new-relic-license-key="$NEW_RELIC_LICENSE_KEY" \
        --from-literal=db-postgres-password="$DB_POSTGRES_PASSWORD" \
        --from-literal=db-mysql-password="$DB_MYSQL_PASSWORD" \
        --namespace="$namespace" \
        --dry-run=client -o yaml | kubectl apply -f -
}

# Deploy as binary/systemd service
deploy_binary() {
    log_info "Deploying as system service..."
    
    # Build if not exists
    local binary_path="$PROJECT_ROOT/distributions/production/database-intelligence-collector"
    
    if [[ ! -f "$binary_path" ]]; then
        log_info "Building collector..."
        "$PROJECT_ROOT/scripts/build/build.sh" production
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would install binary and create systemd service"
        return
    fi
    
    # Copy binary
    sudo cp "$binary_path" /usr/local/bin/database-intelligence-collector
    sudo chmod +x /usr/local/bin/database-intelligence-collector
    
    # Generate config
    generate_config
    sudo mkdir -p /etc/database-intelligence
    sudo cp "$PROJECT_ROOT/runtime/collector-config.yaml" /etc/database-intelligence/config.yaml
    
    # Create systemd service
    create_systemd_service
    
    # Start service
    sudo systemctl daemon-reload
    sudo systemctl enable database-intelligence
    sudo systemctl start database-intelligence
    
    # Check status
    sleep 2
    sudo systemctl status database-intelligence
    
    log_success "Binary deployment completed"
}

# Create systemd service file
create_systemd_service() {
    cat << EOF | sudo tee /etc/systemd/system/database-intelligence.service
[Unit]
Description=Database Intelligence OpenTelemetry Collector
After=network.target

[Service]
Type=simple
User=otel
Group=otel
ExecStart=/usr/local/bin/database-intelligence-collector --config=/etc/database-intelligence/config.yaml
Restart=on-failure
RestartSec=10
Environment="NEW_RELIC_LICENSE_KEY=$NEW_RELIC_LICENSE_KEY"
Environment="DB_POSTGRES_HOST=$DB_POSTGRES_HOST"
Environment="DB_POSTGRES_USER=$DB_POSTGRES_USER"
Environment="DB_POSTGRES_PASSWORD=$DB_POSTGRES_PASSWORD"
Environment="DB_MYSQL_HOST=$DB_MYSQL_HOST"
Environment="DB_MYSQL_USER=$DB_MYSQL_USER"
Environment="DB_MYSQL_PASSWORD=$DB_MYSQL_PASSWORD"

[Install]
WantedBy=multi-user.target
EOF

    # Create otel user if not exists
    if ! id -u otel &>/dev/null; then
        sudo useradd -r -s /bin/false otel
    fi
}

# Deploy parallel modes (config-only and enhanced)
deploy_parallel() {
    log_info "Deploying parallel config-only and enhanced modes..."
    
    cd "$PROJECT_ROOT/deployments/docker"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would deploy parallel modes"
        docker compose -f compose/docker-compose-parallel.yaml config
        return
    fi
    
    # Deploy both modes
    docker compose -f compose/docker-compose-parallel.yaml up -d
    
    # Wait for both to be healthy
    wait_for_service localhost 13133 60  # Config-only health check
    wait_for_service localhost 13134 60  # Enhanced health check
    
    log_success "Parallel deployment completed"
}

# Generate configuration from template
generate_config() {
    log_info "Generating configuration..."
    
    local template="$PROJECT_ROOT/configs/base.yaml"
    local overlay="$PROJECT_ROOT/configs/overlays/$ENVIRONMENT.yaml"
    local output="$PROJECT_ROOT/runtime/collector-config.yaml"
    
    mkdir -p "$PROJECT_ROOT/runtime"
    
    if [[ -f "$overlay" ]]; then
        merge_yaml_configs "$template" "$overlay" "$output"
    else
        cp "$template" "$output"
    fi
    
    # Substitute environment variables
    envsubst < "$output" > "$output.tmp" && mv "$output.tmp" "$output"
}

# Wait for health check
wait_for_health_check() {
    local health_endpoint="${HEALTH_ENDPOINT:-http://localhost:13133/health}"
    local timeout=60
    local elapsed=0
    
    log_info "Waiting for collector to be healthy..."
    
    while ! curl -sf "$health_endpoint" &>/dev/null; do
        if [[ $elapsed -ge $timeout ]]; then
            log_error "Health check timeout"
            return 1
        fi
        
        sleep 2
        ((elapsed+=2))
    done
    
    log_success "Collector is healthy"
}

# Show deployment status
show_status() {
    case "$DEPLOY_TYPE" in
        docker)
            docker compose ps
            ;;
        kubernetes|k8s)
            kubectl get all -n "${NAMESPACE:-database-intelligence}"
            ;;
        binary)
            sudo systemctl status database-intelligence
            ;;
    esac
}

# Main execution
main() {
    # Validate prerequisites
    validate_prerequisites
    
    # Load environment
    load_environment
    
    case "$DEPLOY_TYPE" in
        docker)
            deploy_docker
            ;;
        kubernetes|k8s)
            deploy_kubernetes
            ;;
        binary|systemd)
            deploy_binary
            ;;
        parallel)
            deploy_parallel
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            log_error "Unknown deployment type: $DEPLOY_TYPE"
            usage
            ;;
    esac
    
    # Show status
    show_status
    
    log_success "Deployment completed successfully!"
}

# Run main
main