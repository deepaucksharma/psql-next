#!/bin/bash
set -euo pipefail

# PostgreSQL Unified Collector - Helm Deployment Script

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CHART_DIR="$PROJECT_ROOT/charts/postgres-collector"

# Default values
RELEASE_NAME="${RELEASE_NAME:-postgres-collector}"
NAMESPACE="${NAMESPACE:-postgres-monitoring}"
VALUES_FILE="${VALUES_FILE:-}"
DRY_RUN="${DRY_RUN:-false}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

usage() {
    cat << EOF
Usage: $0 [command] [options]

Commands:
    install         Install the PostgreSQL collector using Helm
    upgrade         Upgrade an existing installation
    uninstall       Remove the PostgreSQL collector
    template        Render the Helm templates locally
    lint            Lint the Helm chart
    package         Package the Helm chart
    
Options:
    -n, --namespace <namespace>     Kubernetes namespace (default: postgres-monitoring)
    -r, --release <name>           Release name (default: postgres-collector)
    -f, --values <file>            Values file to use
    -d, --dry-run                  Perform a dry run
    -h, --help                     Show this help message

Environment Variables:
    NEW_RELIC_LICENSE_KEY          New Relic license key (required for install/upgrade)
    POSTGRES_PASSWORD              PostgreSQL password (required for install/upgrade)
    
Examples:
    # Install with default values
    NEW_RELIC_LICENSE_KEY=xxx POSTGRES_PASSWORD=yyy $0 install
    
    # Install with custom values file
    $0 install -f custom-values.yaml
    
    # Upgrade existing installation
    $0 upgrade -r my-collector
    
    # Render templates for review
    $0 template -f production-values.yaml
EOF
}

# Parse command line arguments
COMMAND="${1:-}"
shift || true

while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -r|--release)
            RELEASE_NAME="$2"
            shift 2
            ;;
        -f|--values)
            VALUES_FILE="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN="true"
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Validate command
case "$COMMAND" in
    install|upgrade|uninstall|template|lint|package)
        ;;
    "")
        usage
        exit 1
        ;;
    *)
        error "Unknown command: $COMMAND"
        ;;
esac

# Check prerequisites
check_prerequisites() {
    if ! command -v helm &> /dev/null; then
        error "Helm is not installed. Please install Helm 3.8+"
    fi
    
    if ! command -v kubectl &> /dev/null; then
        error "kubectl is not installed. Please install kubectl"
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        error "Cannot connect to Kubernetes cluster. Please check your kubeconfig"
    fi
}

# Validate environment for install/upgrade
validate_environment() {
    if [[ "$COMMAND" == "install" || "$COMMAND" == "upgrade" ]]; then
        if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]] && [[ -z "$VALUES_FILE" ]]; then
            error "NEW_RELIC_LICENSE_KEY environment variable is required"
        fi
        
        if [[ -z "${POSTGRES_PASSWORD:-}" ]] && [[ -z "$VALUES_FILE" ]]; then
            warn "POSTGRES_PASSWORD not set. You'll need to provide it in your values file"
        fi
    fi
}

# Create namespace if it doesn't exist
create_namespace() {
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        info "Creating namespace: $NAMESPACE"
        kubectl create namespace "$NAMESPACE"
    fi
}

# Build Helm arguments
build_helm_args() {
    local args=()
    
    if [[ -n "$VALUES_FILE" ]]; then
        args+=(-f "$VALUES_FILE")
    fi
    
    if [[ -n "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        args+=(--set "newrelic.licenseKey=$NEW_RELIC_LICENSE_KEY")
    fi
    
    if [[ -n "${POSTGRES_PASSWORD:-}" ]]; then
        args+=(--set "postgresql.password=$POSTGRES_PASSWORD")
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        args+=(--dry-run)
    fi
    
    echo "${args[@]}"
}

# Install command
cmd_install() {
    info "Installing PostgreSQL collector"
    info "Release: $RELEASE_NAME"
    info "Namespace: $NAMESPACE"
    
    create_namespace
    
    local helm_args=($(build_helm_args))
    
    helm install "$RELEASE_NAME" "$CHART_DIR" \
        --namespace "$NAMESPACE" \
        "${helm_args[@]}"
    
    if [[ "$DRY_RUN" != "true" ]]; then
        info "Installation complete!"
        info "Run 'helm status $RELEASE_NAME -n $NAMESPACE' to check status"
    fi
}

# Upgrade command
cmd_upgrade() {
    info "Upgrading PostgreSQL collector"
    info "Release: $RELEASE_NAME"
    info "Namespace: $NAMESPACE"
    
    local helm_args=($(build_helm_args))
    
    helm upgrade "$RELEASE_NAME" "$CHART_DIR" \
        --namespace "$NAMESPACE" \
        "${helm_args[@]}"
    
    if [[ "$DRY_RUN" != "true" ]]; then
        info "Upgrade complete!"
    fi
}

# Uninstall command
cmd_uninstall() {
    info "Uninstalling PostgreSQL collector"
    info "Release: $RELEASE_NAME"
    info "Namespace: $NAMESPACE"
    
    helm uninstall "$RELEASE_NAME" --namespace "$NAMESPACE"
    
    info "Uninstall complete!"
}

# Template command
cmd_template() {
    info "Rendering Helm templates"
    
    local helm_args=($(build_helm_args))
    
    helm template "$RELEASE_NAME" "$CHART_DIR" \
        --namespace "$NAMESPACE" \
        "${helm_args[@]}"
}

# Lint command
cmd_lint() {
    info "Linting Helm chart"
    
    helm lint "$CHART_DIR"
    
    info "Lint complete!"
}

# Package command
cmd_package() {
    info "Packaging Helm chart"
    
    local version=$(grep '^version:' "$CHART_DIR/Chart.yaml" | awk '{print $2}')
    
    helm package "$CHART_DIR"
    
    info "Chart packaged: postgres-collector-${version}.tgz"
}

# Main execution
main() {
    check_prerequisites
    validate_environment
    
    case "$COMMAND" in
        install)
            cmd_install
            ;;
        upgrade)
            cmd_upgrade
            ;;
        uninstall)
            cmd_uninstall
            ;;
        template)
            cmd_template
            ;;
        lint)
            cmd_lint
            ;;
        package)
            cmd_package
            ;;
    esac
}

main