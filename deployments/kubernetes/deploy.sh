#!/bin/bash
# Streamlined Kubernetes Deployment Script

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default values
OVERLAY="hybrid"
NAMESPACE="postgres-monitoring"
DRY_RUN=false
BUILD_IMAGE=true

# Show help
show_help() {
    cat << EOF
PostgreSQL Unified Collector - Kubernetes Deployment

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    deploy      Deploy to Kubernetes
    remove      Remove from Kubernetes
    status      Show deployment status
    logs        Show collector logs
    test        Run deployment test

Options:
    --overlay OVERLAY    Deployment overlay (nri|otlp|hybrid|production)
    --namespace NS       Target namespace (default: postgres-monitoring)
    --dry-run           Show what would be deployed without applying
    --no-build          Skip Docker image build
    --image IMAGE       Custom image to use

Examples:
    $0 deploy --overlay nri
    $0 deploy --overlay production --namespace prod-monitoring
    $0 status
    $0 logs --follow
    $0 remove --overlay nri

EOF
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --overlay)
                OVERLAY="$2"
                shift 2
                ;;
            --namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --no-build)
                BUILD_IMAGE=false
                shift
                ;;
            --image)
                CUSTOM_IMAGE="$2"
                shift 2
                ;;
            *)
                shift
                ;;
        esac
    done
}

# Build Docker image
build_image() {
    if [ "$BUILD_IMAGE" = true ]; then
        echo -e "${GREEN}Building Docker image...${NC}"
        cd "$PROJECT_ROOT"
        docker build -t postgres-unified-collector:latest .
        
        # Tag with registry if specified
        if [ -n "${DOCKER_REGISTRY:-}" ]; then
            docker tag postgres-unified-collector:latest "$DOCKER_REGISTRY/postgres-unified-collector:latest"
            docker push "$DOCKER_REGISTRY/postgres-unified-collector:latest"
        fi
    fi
}

# Deploy to Kubernetes
deploy() {
    echo -e "${GREEN}Deploying PostgreSQL Unified Collector (overlay: $OVERLAY)...${NC}"
    
    # Build image first
    build_image
    
    # Check if overlay exists
    if [ ! -d "$SCRIPT_DIR/overlays/$OVERLAY" ]; then
        echo -e "${RED}Error: Overlay '$OVERLAY' not found${NC}"
        echo "Available overlays:"
        ls -1 "$SCRIPT_DIR/overlays"
        exit 1
    fi
    
    # Apply with kustomize
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}Dry run mode - showing what would be deployed:${NC}"
        kubectl kustomize "$SCRIPT_DIR/overlays/$OVERLAY"
    else
        kubectl apply -k "$SCRIPT_DIR/overlays/$OVERLAY"
        
        # Wait for deployment
        echo "Waiting for deployment to be ready..."
        kubectl wait --for=condition=available --timeout=300s \
            deployment -l app=postgres-collector -n "$NAMESPACE"
        
        echo -e "${GREEN}Deployment complete!${NC}"
        show_status
    fi
}

# Remove from Kubernetes
remove() {
    echo -e "${YELLOW}Removing PostgreSQL Unified Collector (overlay: $OVERLAY)...${NC}"
    
    if [ "$DRY_RUN" = true ]; then
        echo -e "${YELLOW}Dry run mode - showing what would be removed:${NC}"
        kubectl kustomize "$SCRIPT_DIR/overlays/$OVERLAY"
    else
        kubectl delete -k "$SCRIPT_DIR/overlays/$OVERLAY" --ignore-not-found=true
        echo -e "${GREEN}Removal complete!${NC}"
    fi
}

# Show deployment status
show_status() {
    echo -e "${BLUE}=== Deployment Status ===${NC}"
    
    # Check namespace
    if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
        echo -e "${RED}Namespace '$NAMESPACE' not found${NC}"
        return 1
    fi
    
    echo -e "\n${YELLOW}Deployments:${NC}"
    kubectl get deployments -n "$NAMESPACE" -l app=postgres-collector
    
    echo -e "\n${YELLOW}Pods:${NC}"
    kubectl get pods -n "$NAMESPACE" -l app=postgres-collector
    
    echo -e "\n${YELLOW}Services:${NC}"
    kubectl get services -n "$NAMESPACE" -l app=postgres-collector
    
    echo -e "\n${YELLOW}ConfigMaps:${NC}"
    kubectl get configmaps -n "$NAMESPACE" -l app=postgres-collector
}

# Show logs
show_logs() {
    local follow=""
    if [ "${1:-}" == "--follow" ] || [ "${1:-}" == "-f" ]; then
        follow="-f"
    fi
    
    echo -e "${BLUE}=== Collector Logs ===${NC}"
    kubectl logs -n "$NAMESPACE" -l app=postgres-collector --tail=100 $follow
}

# Run deployment test
run_test() {
    echo -e "${GREEN}Running deployment test...${NC}"
    
    # Deploy
    deploy
    
    # Wait for pods to be ready
    sleep 30
    
    # Check health endpoint
    echo "Checking health endpoint..."
    POD=$(kubectl get pod -n "$NAMESPACE" -l app=postgres-collector -o jsonpath='{.items[0].metadata.name}')
    kubectl exec -n "$NAMESPACE" "$POD" -- wget -q -O- http://localhost:8080/health || echo "Health check failed"
    
    # Check metrics
    echo "Checking metrics endpoint..."
    kubectl exec -n "$NAMESPACE" "$POD" -- wget -q -O- http://localhost:9090/metrics | head -20
    
    echo -e "${GREEN}Test complete!${NC}"
}

# Main execution
main() {
    local command="${1:-help}"
    
    case "$command" in
        deploy)
            parse_args "${@:2}"
            deploy
            ;;
        remove)
            parse_args "${@:2}"
            remove
            ;;
        status)
            parse_args "${@:2}"
            show_status
            ;;
        logs)
            parse_args "${@:2}"
            show_logs "${@:2}"
            ;;
        test)
            parse_args "${@:2}"
            run_test
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo -e "${RED}Unknown command: $command${NC}"
            show_help
            exit 1
            ;;
    esac
}

# Run main function
main "$@"