#!/bin/bash
# Script to standardize naming conventions across the project

set -e

echo "Standardizing naming conventions..."
echo "================================="
echo "Standard: database-intelligence (with hyphens)"
echo ""

# Update docker-compose files
echo "Updating Docker Compose files..."
find ./deployments/docker -name "*.yaml" -o -name "*.yml" | while read -r file; do
    # Update image names
    sed -i 's/database_intelligence/database-intelligence/g' "$file"
    sed -i 's/db-intel/database-intelligence/g' "$file"
    sed -i 's/db-otel/database-intelligence/g' "$file"
    echo "  - Updated: $file"
done

# Update Kubernetes manifests
if [ -d "./deployments/kubernetes" ]; then
    echo ""
    echo "Updating Kubernetes manifests..."
    find ./deployments/kubernetes -name "*.yaml" -o -name "*.yml" | while read -r file; do
        sed -i 's/database_intelligence/database-intelligence/g' "$file"
        sed -i 's/db-intel/database-intelligence/g' "$file"
        sed -i 's/db-otel/database-intelligence/g' "$file"
        echo "  - Updated: $file"
    done
fi

# Update documentation
echo ""
echo "Updating documentation..."
find ./docs -name "*.md" | while read -r file; do
    # Preserve code blocks and URLs
    sed -i '/```/,/```/!s/database_intelligence/database-intelligence/g' "$file"
    sed -i '/```/,/```/!s/db-intel/database-intelligence/g' "$file"
    echo "  - Updated: $file"
done

# Update environment templates
echo ""
echo "Updating environment templates..."
find . -name "*.env*" -o -name "env.template*" | grep -v node_modules | while read -r file; do
    sed -i 's/database_intelligence/database-intelligence/g' "$file"
    echo "  - Updated: $file"
done

# Create naming convention guide
cat > ./docs/naming-conventions.md << 'EOF'
# Naming Conventions

This document defines the naming conventions used throughout the Database Intelligence project.

## Standard Name

The project uses **`database-intelligence`** as the standard name (with hyphen, all lowercase).

## Usage Guidelines

### Binary Names
- Collector binary: `database-intelligence-collector`
- Builder output: `database-intelligence-collector`

### Docker Images
- Image name: `database-intelligence:tag`
- Registry path: `ghcr.io/[org]/database-intelligence:tag`

### Kubernetes Resources
- Deployment: `database-intelligence`
- Service: `database-intelligence`
- ConfigMap: `database-intelligence-config`
- Secret: `database-intelligence-secrets`

### Service Names
- OpenTelemetry service.name: `database-intelligence-collector`
- Prometheus namespace: `database_intelligence` (underscore for Prometheus compatibility)

### Environment Variables
- Use underscores: `DATABASE_INTELLIGENCE_*`
- Example: `DATABASE_INTELLIGENCE_VERSION`

### Go Modules
- Module path: `github.com/[org]/database-intelligence`
- Package names: `databaseintelligence` (no hyphen in Go packages)

### File Names
- Config files: `database-intelligence-*.yaml`
- Scripts: `database-intelligence-*.sh`

## Deprecated Names

The following naming patterns are deprecated and should not be used:
- `db-otel`
- `db-intel`
- `database_intelligence` (except in Prometheus metrics)
- `databaseIntelligence` (except in Go code where required)
EOF

echo ""
echo "Created naming conventions guide: ./docs/naming-conventions.md"
echo ""
echo "Naming standardization complete!"
echo "Please review the changes and test thoroughly."