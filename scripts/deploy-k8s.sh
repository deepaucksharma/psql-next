#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
K8S_DIR="${PROJECT_ROOT}/deployments/kubernetes"
ENV_FILE="${PROJECT_ROOT}/.env"
IMAGE_NAME="postgres-unified-collector"
IMAGE_TAG="${IMAGE_TAG:-latest}"
NAMESPACE="postgres-monitoring"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check for required tools
check_requirements() {
    print_status "Checking requirements..."
    
    local missing_tools=()
    
    for tool in docker kubectl kustomize; do
        if ! command -v $tool &> /dev/null; then
            missing_tools+=($tool)
        fi
    done
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        print_error "Missing required tools: ${missing_tools[*]}"
        print_error "Please install the missing tools and try again."
        exit 1
    fi
    
    print_status "All required tools are installed."
}

# Load environment variables
load_env() {
    if [ ! -f "$ENV_FILE" ]; then
        print_error ".env file not found at $ENV_FILE"
        exit 1
    fi
    
    print_status "Loading environment variables from .env file..."
    
    # Export variables from .env file
    while IFS='=' read -r key value; do
        # Skip empty lines and comments
        [[ -z "$key" || "$key" =~ ^[[:space:]]*# ]] && continue
        # Remove leading/trailing whitespace
        key=$(echo "$key" | xargs)
        value=$(echo "$value" | xargs)
        # Export the variable
        export "$key=$value"
    done < "$ENV_FILE"
    
    # Verify required variables
    if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
        print_error "NEW_RELIC_LICENSE_KEY not found in .env file"
        exit 1
    fi
    
    print_status "Environment variables loaded successfully."
}

# Build Docker image
build_docker_image() {
    print_status "Building Docker image..."
    
    cd "$PROJECT_ROOT"
    
    docker build \
        -f Dockerfile \
        -t "${IMAGE_NAME}:${IMAGE_TAG}" \
        --build-arg RUST_BACKTRACE=1 \
        .
    
    if [ $? -eq 0 ]; then
        print_status "Docker image built successfully: ${IMAGE_NAME}:${IMAGE_TAG}"
    else
        print_error "Failed to build Docker image"
        exit 1
    fi
}

# Apply Kubernetes resources
apply_kubernetes_resources() {
    print_status "Applying Kubernetes resources with New Relic license key..."
    
    cd "$K8S_DIR"
    
    # Create a temporary directory for the build
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT
    
    # Copy all files to temp directory
    cp -r . "$TEMP_DIR/"
    
    # Update the license key in the temporary kustomization file
    sed -i.bak "s/NEWRELIC_LICENSE_KEY_PLACEHOLDER/${NEW_RELIC_LICENSE_KEY}/" "$TEMP_DIR/kustomization.yaml"
    
    # Build and apply resources using kustomize from temp directory
    kustomize build "$TEMP_DIR" | kubectl apply -f -
    
    if [ $? -eq 0 ]; then
        print_status "Kubernetes resources applied successfully."
    else
        print_error "Failed to apply Kubernetes resources"
        exit 1
    fi
}

# Wait for deployment to be ready
wait_for_deployment() {
    print_status "Waiting for deployment to be ready..."
    
    kubectl wait --for=condition=available --timeout=300s \
        deployment/postgres-collector -n "$NAMESPACE"
    
    if [ $? -eq 0 ]; then
        print_status "Deployment is ready!"
    else
        print_error "Deployment failed to become ready within timeout"
        exit 1
    fi
}

# Verify deployment
verify_deployment() {
    print_status "Verifying deployment..."
    
    echo ""
    echo "=== Deployment Status ==="
    kubectl get deployment postgres-collector -n "$NAMESPACE"
    
    echo ""
    echo "=== Pod Status ==="
    kubectl get pods -n "$NAMESPACE" -l app=postgres-collector
    
    echo ""
    echo "=== Recent Pod Events ==="
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app=postgres-collector -o jsonpath='{.items[0].metadata.name}')
    kubectl describe pod "$POD_NAME" -n "$NAMESPACE" | tail -20
    
    echo ""
    echo "=== Health Check Status ==="
    
    # Port forward to access health endpoints
    print_status "Setting up port forward to access health endpoints..."
    kubectl port-forward -n "$NAMESPACE" "pod/$POD_NAME" 8080:8080 &
    PORT_FORWARD_PID=$!
    
    # Wait for port forward to be ready
    sleep 3
    
    # Check health endpoint
    print_status "Checking health endpoint..."
    curl -s http://localhost:8080/health | jq . || echo "Failed to get health status"
    
    # Check readiness endpoint
    print_status "Checking readiness endpoint..."
    curl -s http://localhost:8080/ready || echo "Failed to get readiness status"
    
    # Clean up port forward
    kill $PORT_FORWARD_PID 2>/dev/null
    
    echo ""
}

# Print verification instructions
print_verification_instructions() {
    echo ""
    echo "========================================="
    echo "Deployment completed successfully!"
    echo "========================================="
    echo ""
    echo "To verify the deployment:"
    echo ""
    echo "1. Check pod logs:"
    echo "   kubectl logs -n $NAMESPACE -l app=postgres-collector -f"
    echo ""
    echo "2. Check health endpoints:"
    echo "   kubectl port-forward -n $NAMESPACE svc/postgres-collector-metrics 8080:8080"
    echo "   curl http://localhost:8080/health"
    echo "   curl http://localhost:8080/ready"
    echo ""
    echo "3. Check metrics endpoint:"
    echo "   kubectl port-forward -n $NAMESPACE svc/postgres-collector-metrics 9090:9090"
    echo "   curl http://localhost:9090/metrics"
    echo ""
    echo "4. View New Relic data:"
    echo "   - Log in to New Relic (https://one.newrelic.com)"
    echo "   - Navigate to Infrastructure > Third-party services"
    echo "   - Look for PostgreSQL integrations"
    echo ""
    echo "5. Troubleshooting:"
    echo "   - Check all resources: kubectl get all -n $NAMESPACE"
    echo "   - Describe deployment: kubectl describe deployment postgres-collector -n $NAMESPACE"
    echo "   - Check secrets: kubectl get secrets -n $NAMESPACE"
    echo ""
}

# Main execution
main() {
    print_status "Starting PostgreSQL collector deployment to Kubernetes..."
    
    check_requirements
    load_env
    build_docker_image
    apply_kubernetes_resources
    wait_for_deployment
    verify_deployment
    print_verification_instructions
}

# Run main function
main "$@"