#!/bin/bash

# Test script to verify kustomize build works correctly

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="${SCRIPT_DIR}/../deployments/kubernetes"

echo "Testing kustomize build..."
cd "$K8S_DIR"

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Copy files
cp -r . "$TEMP_DIR/"

# Replace placeholder
sed -i.bak "s/NEWRELIC_LICENSE_KEY_PLACEHOLDER/test-license-key/" "$TEMP_DIR/kustomization.yaml"

# Build and check output
echo "Building kustomize..."
kustomize build "$TEMP_DIR" > "$TEMP_DIR/output.yaml"

echo "Checking output..."
echo "- Namespace count: $(grep -c "kind: Namespace" "$TEMP_DIR/output.yaml" || echo 0)"
echo "- Secret count: $(grep -c "kind: Secret" "$TEMP_DIR/output.yaml" || echo 0)"
echo "- ConfigMap count: $(grep -c "kind: ConfigMap" "$TEMP_DIR/output.yaml" || echo 0)"
echo "- Deployment count: $(grep -c "kind: Deployment" "$TEMP_DIR/output.yaml" || echo 0)"
echo "- Service count: $(grep -c "kind: Service" "$TEMP_DIR/output.yaml" || echo 0)"

echo ""
echo "Checking license key replacement..."
if grep -q "test-license-key" "$TEMP_DIR/output.yaml"; then
    echo "✓ License key replacement successful"
else
    echo "✗ License key replacement failed"
    exit 1
fi

echo ""
echo "Checking health check configuration..."
if grep -q "path: /health" "$TEMP_DIR/output.yaml"; then
    echo "✓ Health check endpoints configured"
else
    echo "✗ Health check endpoints not found"
    exit 1
fi

echo ""
echo "Test completed successfully!"