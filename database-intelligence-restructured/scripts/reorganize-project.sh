#\!/bin/bash
# Complete project reorganization script

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Database Intelligence Project Reorganization ===${NC}"

# Create new clean structure
echo -e "${YELLOW}Creating optimized project structure...${NC}"

# Core directories
mkdir -p {configs,scripts,docs,tests,development,deployments}

# Script subdirectories
mkdir -p scripts/{validation,testing,building,deployment,maintenance}

# Documentation subdirectories
mkdir -p docs/{guides,reference,development,operations}

# Test subdirectories
mkdir -p tests/{unit,integration,e2e,performance}

# Deployment subdirectories
mkdir -p deployments/{docker,kubernetes,examples}

echo -e "${GREEN}✓ Directory structure created${NC}"

# Move scripts to appropriate locations
echo -e "\n${YELLOW}Organizing scripts...${NC}"

# Validation scripts
mv scripts/validation/* scripts/validation/ 2>/dev/null || true

# Test scripts  
mv scripts/testing/* scripts/testing/ 2>/dev/null || true

# Build scripts
mv scripts/building/* scripts/building/ 2>/dev/null || true

# Maintenance scripts
mv scripts/cleanup-*.sh scripts/maintenance/ 2>/dev/null || true
mv scripts/fix-*.sh scripts/maintenance/ 2>/dev/null || true
mv scripts/standardize-*.sh scripts/maintenance/ 2>/dev/null || true

echo -e "${GREEN}✓ Scripts organized${NC}"

# Summary
echo -e "\n${BLUE}=== Reorganization Complete ===${NC}"
echo "New structure:"
echo "  configs/          - All configuration files"
echo "  scripts/          - All executable scripts"
echo "    validation/     - Validation tools"
echo "    testing/        - Test runners"
echo "    building/       - Build scripts"
echo "    deployment/     - Deployment tools"
echo "    maintenance/    - Cleanup and fixes"
echo "  docs/             - All documentation"
echo "    guides/         - User guides"
echo "    reference/      - API and architecture"
echo "    development/    - Developer docs"
echo "    operations/     - Ops guides"
echo "  tests/            - All test code"
echo "    unit/           - Unit tests"
echo "    integration/    - Integration tests"
echo "    e2e/            - End-to-end tests"
echo "    performance/    - Performance tests"
echo "  deployments/      - Deployment configs"
echo "    docker/         - Docker files"
echo "    kubernetes/     - K8s manifests"
echo "    examples/       - Example configs"
