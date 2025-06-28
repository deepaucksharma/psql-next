#!/bin/bash
# Database Intelligence MVP - Kubernetes Deployment Script
# This script deploys the Database Intelligence collector safely

set -euo pipefail

# Configuration
NAMESPACE="monitoring"
DEPLOYMENT_NAME="nr-db-intelligence"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# Check if kubectl is available and configured
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl first."
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "kubectl not configured or cluster unreachable."
        exit 1
    fi
    
    log_success "kubectl is configured and cluster is reachable"
}

# Check if namespace exists, create if not
ensure_namespace() {
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace $NAMESPACE already exists"
    else
        log_info "Creating namespace $NAMESPACE"
        kubectl create namespace "$NAMESPACE"
        kubectl label namespace "$NAMESPACE" name="$NAMESPACE"
        log_success "Namespace $NAMESPACE created"
    fi
}

# Validate required files
validate_files() {
    local required_files=(
        "statefulset.yaml"
        "configmap.yaml"
        "secrets.yaml"
        "rbac.yaml"
        "network-policy.yaml"
    )
    
    for file in "${required_files[@]}"; do
        if [[ ! -f "$SCRIPT_DIR/$file" ]]; then
            log_error "Required file not found: $file"
            exit 1
        fi
    done
    
    log_success "All required deployment files found"
}

# Check if credentials are configured
check_credentials() {
    log_warning "IMPORTANT: Please ensure you have updated the following in secrets.yaml:"
    echo "  1. New Relic license key"
    echo "  2. PostgreSQL replica connection string"
    echo "  3. MySQL connection string (if applicable)"
    echo ""
    
    read -p "Have you updated the credentials in secrets.yaml? (y/N): " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_error "Please update credentials in secrets.yaml before deploying"
        exit 1
    fi
}

# Check database prerequisites
check_database_prerequisites() {
    log_info "Checking database prerequisites..."
    
    echo "Please verify the following database prerequisites:"
    echo "  1. PostgreSQL has pg_stat_statements extension enabled"
    echo "  2. Read replica is available and accessible"
    echo "  3. Monitoring user exists with proper permissions"
    echo "  4. Network connectivity from Kubernetes to database"
    echo ""
    
    read -p "Have you verified all database prerequisites? (y/N): " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_warning "Please check PREREQUISITES.md for database setup requirements"
        log_error "Database prerequisites must be met before deployment"
        exit 1
    fi
}

# Deploy RBAC first
deploy_rbac() {
    log_info "Deploying RBAC configuration..."
    kubectl apply -f "$SCRIPT_DIR/rbac.yaml"
    log_success "RBAC configuration deployed"
}

# Deploy secrets
deploy_secrets() {
    log_info "Deploying secrets..."
    kubectl apply -f "$SCRIPT_DIR/secrets.yaml"
    log_success "Secrets deployed"
}

# Deploy config map
deploy_configmap() {
    log_info "Deploying configuration..."
    kubectl apply -f "$SCRIPT_DIR/configmap.yaml"
    log_success "Configuration deployed"
}

# Deploy network policies
deploy_network_policies() {
    log_info "Deploying network policies..."
    kubectl apply -f "$SCRIPT_DIR/network-policy.yaml"
    log_success "Network policies deployed"
}

# Deploy the main StatefulSet
deploy_statefulset() {
    log_info "Deploying Database Intelligence collector..."
    kubectl apply -f "$SCRIPT_DIR/statefulset.yaml"
    log_success "StatefulSet deployed"
}

# Wait for deployment to be ready
wait_for_deployment() {
    log_info "Waiting for collector to be ready..."
    
    # Wait for StatefulSet to be ready
    kubectl rollout status statefulset/$DEPLOYMENT_NAME-collector -n "$NAMESPACE" --timeout=300s
    
    # Wait for pod to be running
    kubectl wait --for=condition=Ready pod -l app=$DEPLOYMENT_NAME -n "$NAMESPACE" --timeout=300s
    
    log_success "Collector is ready and running"
}

# Verify deployment health
verify_deployment() {
    log_info "Verifying deployment health..."
    
    # Check pod status
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l app=$DEPLOYMENT_NAME -o jsonpath='{.items[0].metadata.name}')
    local pod_status=$(kubectl get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.status.phase}')
    
    if [[ "$pod_status" != "Running" ]]; then
        log_error "Pod is not running. Status: $pod_status"
        return 1
    fi
    
    # Check health endpoint
    log_info "Checking health endpoint..."
    if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:13133/ &> /dev/null; then
        log_success "Health check passed"
    else
        log_error "Health check failed"
        return 1
    fi
    
    # Check metrics endpoint
    log_info "Checking metrics endpoint..."
    if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:8888/metrics &> /dev/null; then
        log_success "Metrics endpoint is working"
    else
        log_warning "Metrics endpoint check failed (this may be normal during startup)"
    fi
    
    return 0
}

# Show deployment information
show_deployment_info() {
    log_info "Deployment Information:"
    echo ""
    
    # Pod information
    kubectl get pods -n "$NAMESPACE" -l app=$DEPLOYMENT_NAME
    echo ""
    
    # Service information
    kubectl get services -n "$NAMESPACE" -l app=$DEPLOYMENT_NAME
    echo ""
    
    # PVC information
    kubectl get pvc -n "$NAMESPACE" -l app=$DEPLOYMENT_NAME
    echo ""
    
    log_info "Access Information:"
    echo "  Health Check: kubectl port-forward -n $NAMESPACE svc/$DEPLOYMENT_NAME-service 13133:13133"
    echo "  Metrics:      kubectl port-forward -n $NAMESPACE svc/$DEPLOYMENT_NAME-service 8888:8888"
    echo "  Debug:        kubectl port-forward -n $NAMESPACE svc/$DEPLOYMENT_NAME-service 55679:55679"
    echo ""
    
    log_info "Useful Commands:"
    echo "  Logs:         kubectl logs -n $NAMESPACE -l app=$DEPLOYMENT_NAME -f"
    echo "  Status:       kubectl get statefulset -n $NAMESPACE $DEPLOYMENT_NAME-collector"
    echo "  Events:       kubectl get events -n $NAMESPACE --sort-by=.metadata.creationTimestamp"
    echo "  Shell:        kubectl exec -n $NAMESPACE -it \$(kubectl get pod -n $NAMESPACE -l app=$DEPLOYMENT_NAME -o jsonpath='{.items[0].metadata.name}') -- sh"
}

# Show monitoring setup
show_monitoring_info() {
    log_info "Monitoring Setup:"
    echo ""
    echo "1. Verify data is flowing to New Relic:"
    echo "   - Check your New Relic account for incoming log data"
    echo "   - Look for db.intelligence.* attributes"
    echo ""
    echo "2. Monitor collector health:"
    echo "   - Prometheus metrics: http://localhost:8888/metrics"
    echo "   - Health endpoint: http://localhost:13133/"
    echo "   - Debug pages: http://localhost:55679/debug/"
    echo ""
    echo "3. Common metrics to watch:"
    echo "   - otelcol_receiver_accepted_log_records"
    echo "   - otelcol_processor_dropped_log_records"
    echo "   - otelcol_exporter_sent_log_records"
}

# Main deployment function
main() {
    log_info "Starting Database Intelligence MVP deployment..."
    echo ""
    
    # Pre-flight checks
    check_kubectl
    validate_files
    check_credentials
    check_database_prerequisites
    
    # Create namespace
    ensure_namespace
    
    # Deploy components in order
    deploy_rbac
    deploy_secrets
    deploy_configmap
    deploy_network_policies
    deploy_statefulset
    
    # Wait and verify
    wait_for_deployment
    
    if verify_deployment; then
        log_success "Database Intelligence MVP deployed successfully!"
        echo ""
        show_deployment_info
        echo ""
        show_monitoring_info
    else
        log_error "Deployment verification failed. Check the logs for details."
        echo ""
        log_info "Troubleshooting commands:"
        echo "  kubectl logs -n $NAMESPACE -l app=$DEPLOYMENT_NAME"
        echo "  kubectl describe pod -n $NAMESPACE -l app=$DEPLOYMENT_NAME"
        echo "  kubectl get events -n $NAMESPACE --sort-by=.metadata.creationTimestamp"
        exit 1
    fi
}

# Show usage information
show_usage() {
    echo "Database Intelligence MVP - Kubernetes Deployment Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -n, --namespace NAMESPACE   Set deployment namespace (default: monitoring)"
    echo "  --skip-checks  Skip interactive prerequisite checks"
    echo "  --dry-run      Show what would be deployed without actually deploying"
    echo ""
    echo "Prerequisites:"
    echo "  1. kubectl configured and connected to cluster"
    echo "  2. Database prerequisites met (see PREREQUISITES.md)"
    echo "  3. Credentials updated in secrets.yaml"
    echo ""
    echo "Examples:"
    echo "  $0                    # Deploy with defaults"
    echo "  $0 -n production      # Deploy to production namespace"
    echo "  $0 --dry-run          # Show deployment plan"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --skip-checks)
            SKIP_CHECKS=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Handle dry run
if [[ "${DRY_RUN:-false}" == "true" ]]; then
    log_info "DRY RUN MODE - Showing deployment plan"
    echo ""
    echo "Would deploy to namespace: $NAMESPACE"
    echo "Files that would be applied:"
    echo "  - rbac.yaml"
    echo "  - secrets.yaml"
    echo "  - configmap.yaml"
    echo "  - network-policy.yaml"
    echo "  - statefulset.yaml"
    echo ""
    log_info "Use '$0' without --dry-run to perform actual deployment"
    exit 0
fi

# Run main deployment
main