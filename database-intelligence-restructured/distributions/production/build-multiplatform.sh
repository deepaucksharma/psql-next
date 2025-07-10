#!/bin/bash

# Build multi-platform Docker images for Database Intelligence Collector
set -e

# Configuration
IMAGE_NAME="${IMAGE_NAME:-database-intelligence}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
REGISTRY="${REGISTRY:-}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64,linux/arm/v7}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check requirements
check_requirements() {
    log_info "Checking requirements..."
    
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    # Check if buildx is available
    if ! docker buildx version &> /dev/null; then
        log_error "Docker buildx is not available"
        log_info "Install buildx: https://github.com/docker/buildx#installing"
        exit 1
    fi
    
    # Check if qemu is set up for multi-platform builds
    if ! docker run --rm --privileged multiarch/qemu-user-static --reset -p yes &> /dev/null; then
        log_warn "Setting up QEMU for multi-platform builds..."
        docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
    fi
}

# Create or use buildx builder
setup_builder() {
    log_info "Setting up buildx builder..."
    
    BUILDER_NAME="database-intelligence-builder"
    
    # Check if builder exists
    if docker buildx ls | grep -q "$BUILDER_NAME"; then
        log_info "Using existing builder: $BUILDER_NAME"
    else
        log_info "Creating new builder: $BUILDER_NAME"
        docker buildx create --name "$BUILDER_NAME" --driver docker-container --use
    fi
    
    # Bootstrap the builder
    docker buildx inspect --bootstrap
}

# Build multi-platform image
build_image() {
    log_info "Building multi-platform image..."
    log_info "Platforms: $PLATFORMS"
    log_info "Image: $IMAGE_NAME:$IMAGE_TAG"
    
    # Determine push behavior
    PUSH_FLAG=""
    if [ -n "$REGISTRY" ]; then
        PUSH_FLAG="--push"
        FULL_IMAGE_NAME="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"
        log_info "Will push to registry: $REGISTRY"
    else
        PUSH_FLAG="--load"
        FULL_IMAGE_NAME="$IMAGE_NAME:$IMAGE_TAG"
        log_warn "No registry specified, will load to local Docker (single platform only)"
        
        # For local load, we can only build one platform
        if [[ "$PLATFORMS" == *","* ]]; then
            log_warn "Multiple platforms specified but --load only supports one platform"
            log_warn "Building for current platform only"
            PLATFORMS="linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"
        fi
    fi
    
    # Build command
    docker buildx build \
        --platform "$PLATFORMS" \
        --file Dockerfile.multiplatform \
        --tag "$FULL_IMAGE_NAME" \
        --build-arg BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
        --build-arg VCS_REF="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
        --build-arg VERSION="$IMAGE_TAG" \
        $PUSH_FLAG \
        .
    
    if [ $? -eq 0 ]; then
        log_info "Build successful!"
        if [ -n "$REGISTRY" ]; then
            log_info "Image pushed to: $FULL_IMAGE_NAME"
            log_info "Pull with: docker pull $FULL_IMAGE_NAME"
        else
            log_info "Image loaded locally: $FULL_IMAGE_NAME"
            log_info "Run with: docker run $FULL_IMAGE_NAME"
        fi
    else
        log_error "Build failed!"
        exit 1
    fi
}

# Main execution
main() {
    log_info "Database Intelligence Multi-Platform Docker Build"
    log_info "================================================"
    
    check_requirements
    setup_builder
    build_image
    
    log_info "================================================"
    log_info "Build complete!"
}

# Show usage
usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Build multi-platform Docker images for Database Intelligence Collector

Options:
    -h, --help              Show this help message
    -n, --name NAME         Image name (default: database-intelligence)
    -t, --tag TAG           Image tag (default: latest)
    -r, --registry REG      Registry URL (e.g., docker.io/myuser)
    -p, --platforms PLAT    Platforms to build (default: linux/amd64,linux/arm64,linux/arm/v7)

Examples:
    # Build for local use (current platform only)
    $0

    # Build and push to Docker Hub
    $0 --registry docker.io/myuser --tag v2.0.0

    # Build specific platforms
    $0 --platforms linux/amd64,linux/arm64

Environment Variables:
    IMAGE_NAME      Override image name
    IMAGE_TAG       Override image tag
    REGISTRY        Override registry URL
    PLATFORMS       Override target platforms
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -n|--name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -t|--tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -p|--platforms)
            PLATFORMS="$2"
            shift 2
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Run main function
main