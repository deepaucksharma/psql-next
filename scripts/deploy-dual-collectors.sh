#!/bin/bash

# Deploy Dual PostgreSQL Collectors (NRI + OTLP)
# This script deploys both NRI and OTLP collectors to monitor the same PostgreSQL instance

set -euo pipefail

# Configuration
NAMESPACE="postgres-monitoring"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENTS_DIR="$PROJECT_ROOT/deployments/kubernetes"

# Color codes for output
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

# Check if kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_info "kubectl is available and connected to cluster"
}

# Check if deployment files exist
check_files() {
    local files=(
        "$DEPLOYMENTS_DIR/collector-nri.yaml"
        "$DEPLOYMENTS_DIR/collector-otlp.yaml"
    )
    
    for file in "${files[@]}"; do
        if [[ ! -f "$file" ]]; then
            log_error "Required file not found: $file"
            exit 1
        fi
    done
    
    log_info "All required deployment files found"
}

# Create namespace if it doesn't exist
create_namespace() {
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace '$NAMESPACE' already exists"
    else
        log_info "Creating namespace '$NAMESPACE'"
        kubectl create namespace "$NAMESPACE"
        log_success "Namespace '$NAMESPACE' created"
    fi
}

# Deploy NRI collector
deploy_nri_collector() {
    log_info "Deploying NRI collector..."
    
    if kubectl apply -f "$DEPLOYMENTS_DIR/collector-nri.yaml"; then
        log_success "NRI collector deployed successfully"
    else
        log_error "Failed to deploy NRI collector"
        return 1
    fi
}

# Deploy OTLP collector
deploy_otlp_collector() {
    log_info "Deploying OTLP collector..."
    
    if kubectl apply -f "$DEPLOYMENTS_DIR/collector-otlp.yaml"; then
        log_success "OTLP collector deployed successfully"
    else
        log_error "Failed to deploy OTLP collector"
        return 1
    fi
}

# Wait for deployments to be ready
wait_for_deployments() {
    log_info "Waiting for deployments to be ready..."
    
    # Wait for NRI collector
    log_info "Waiting for NRI collector deployment..."
    if kubectl wait --for=condition=available --timeout=300s deployment/postgres-collector-nri -n "$NAMESPACE"; then
        log_success "NRI collector is ready"
    else
        log_error "NRI collector failed to become ready"
        return 1
    fi
    
    # Wait for OTLP collector
    log_info "Waiting for OTLP collector deployment..."
    if kubectl wait --for=condition=available --timeout=300s deployment/postgres-collector-otlp -n "$NAMESPACE"; then
        log_success "OTLP collector is ready"
    else
        log_error "OTLP collector failed to become ready"
        return 1
    fi
}

# Show deployment status
show_status() {
    echo
    log_info "=== Deployment Status ==="
    echo
    
    # Show deployments
    log_info "Deployments:"
    kubectl get deployments -n "$NAMESPACE" -o wide
    echo
    
    # Show pods
    log_info "Pods:"
    kubectl get pods -n "$NAMESPACE" -o wide
    echo
    
    # Show services
    log_info "Services:"
    kubectl get services -n "$NAMESPACE" -o wide
    echo
    
    # Show configmaps
    log_info "ConfigMaps:"
    kubectl get configmaps -n "$NAMESPACE"
    echo
}

# Show access information
show_access_info() {
    echo
    log_info "=== Access Information ==="
    echo
    
    log_info "NRI Collector:"
    echo "  Health Check: http://<node-ip>:$(kubectl get svc postgres-collector-nri-service -n "$NAMESPACE" -o jsonpath='{.spec.ports[?(@.name=="health")].nodePort}' 2>/dev/null || echo "8080")/health"
    echo "  Metrics:      http://<node-ip>:$(kubectl get svc postgres-collector-nri-service -n "$NAMESPACE" -o jsonpath='{.spec.ports[?(@.name=="metrics")].nodePort}' 2>/dev/null || echo "9090")/metrics"
    echo
    
    log_info "OTLP Collector:"
    echo "  Health Check: http://<node-ip>:$(kubectl get svc postgres-collector-otlp-service -n "$NAMESPACE" -o jsonpath='{.spec.ports[?(@.name=="health")].nodePort}' 2>/dev/null || echo "8081")/health"
    echo "  Metrics:      http://<node-ip>:$(kubectl get svc postgres-collector-otlp-service -n "$NAMESPACE" -o jsonpath='{.spec.ports[?(@.name=="metrics")].nodePort}' 2>/dev/null || echo "9091")/metrics"
    echo
    
    log_info "To check logs:"
    echo "  NRI Collector:  kubectl logs -f deployment/postgres-collector-nri -n $NAMESPACE"
    echo "  OTLP Collector: kubectl logs -f deployment/postgres-collector-otlp -n $NAMESPACE"
    echo
}

# Show configuration reminders
show_config_reminders() {
    echo
    log_warning "=== Configuration Reminders ==="
    echo
    log_warning "1. Update the New Relic License Key:"
    echo "   kubectl patch secret newrelic-license -n $NAMESPACE -p '{\"stringData\":{\"key\":\"YOUR_ACTUAL_LICENSE_KEY\"}}'"
    echo
    log_warning "2. Update PostgreSQL credentials if needed:"
    echo "   kubectl patch secret postgres-credentials -n $NAMESPACE -p '{\"stringData\":{\"username\":\"your_user\",\"password\":\"your_password\"}}'"
    echo
    log_warning "3. Ensure PostgreSQL service 'postgres-primary' is available in the namespace"
    echo
    log_warning "4. For OTLP mode, ensure 'otel-collector' service is available at port 4317"
    echo
}

# Cleanup function
cleanup() {
    log_info "Cleaning up dual collector deployment..."
    
    kubectl delete -f "$DEPLOYMENTS_DIR/collector-nri.yaml" --ignore-not-found=true
    kubectl delete -f "$DEPLOYMENTS_DIR/collector-otlp.yaml" --ignore-not-found=true
    
    log_success "Cleanup completed"
}

# Main deployment function
main() {
    echo
    log_info "=== PostgreSQL Dual Collector Deployment ==="
    echo
    
    # Parse command line arguments
    case "${1:-deploy}" in
        "deploy")
            check_kubectl
            check_files
            create_namespace
            deploy_nri_collector
            deploy_otlp_collector
            wait_for_deployments
            show_status
            show_access_info
            show_config_reminders
            
            echo
            log_success "=== Dual collector deployment completed successfully! ==="
            echo
            log_info "Both NRI and OTLP collectors are now monitoring the same PostgreSQL instance"
            log_info "NRI collector sends data via New Relic Infrastructure format"
            log_info "OTLP collector sends data via OpenTelemetry format"
            ;;
        "cleanup"|"clean")
            cleanup
            ;;
        "status")
            show_status
            show_access_info
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [command]"
            echo
            echo "Commands:"
            echo "  deploy    Deploy both NRI and OTLP collectors (default)"
            echo "  cleanup   Remove both collectors"
            echo "  status    Show deployment status"
            echo "  help      Show this help message"
            echo
            echo "Examples:"
            echo "  $0                # Deploy both collectors"
            echo "  $0 deploy         # Deploy both collectors"
            echo "  $0 status         # Check deployment status"
            echo "  $0 cleanup        # Remove both collectors"
            ;;
        *)
            log_error "Unknown command: $1"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"