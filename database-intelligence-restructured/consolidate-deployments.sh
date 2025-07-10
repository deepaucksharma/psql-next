#!/bin/bash

# Deployment Consolidation Script
# Organizes and consolidates all deployment-related files

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
BACKUP_DIR="/Users/deepaksharma/syc/db-otel/backup-deployments-$(date +%Y%m%d-%H%M%S)"

echo -e "${BLUE}=== Deployment Consolidation ===${NC}"

# Create backup
mkdir -p "$BACKUP_DIR"
echo -e "${GREEN}[✓]${NC} Created backup directory: $BACKUP_DIR"

cd "$PROJECT_ROOT"

# Consolidate docker-compose files
echo -e "\n${BLUE}Analyzing Docker Compose files...${NC}"

# Find all docker-compose files and categorize them
DOCKER_FILES=$(find . -name "docker-compose*.y*ml" -type f | grep -v backup | sort)

echo -e "${YELLOW}Found docker-compose files:${NC}"
for file in $DOCKER_FILES; do
    echo "  - $file"
done

# Keep only essential docker-compose files
echo -e "\n${BLUE}Consolidating Docker Compose files...${NC}"

# Define which files to keep and where
declare -A DOCKER_MAPPINGS=(
    ["./deployments/docker/compose/docker-compose.yaml"]="default"
    ["./deployments/docker/compose/docker-compose.prod.yaml"]="production"
    ["./deployments/docker/compose/docker-compose.test.yaml"]="test"
    ["./deployments/docker/compose/docker-compose-databases.yaml"]="databases"
    ["./deployments/docker/compose/docker-compose-ha.yaml"]="high-availability"
)

# Move redundant docker-compose files
for file in $DOCKER_FILES; do
    filename=$(basename "$file")
    dirname=$(dirname "$file")
    
    # Skip if it's already in the right place
    if [[ "$dirname" == "./deployments/docker/compose" ]]; then
        continue
    fi
    
    # Determine action based on file name
    case "$filename" in
        docker-compose.yaml|docker-compose.yml)
            if [[ "$dirname" != "." ]]; then
                mv "$file" "$BACKUP_DIR/"
                echo -e "${YELLOW}[!]${NC} Backed up: $file"
            fi
            ;;
        docker-compose.production.yml|docker-compose.production.yaml)
            mv "$file" "$BACKUP_DIR/"
            echo -e "${YELLOW}[!]${NC} Backed up: $file (using docker-compose.prod.yaml instead)"
            ;;
        docker-compose.ohi-migration.yaml)
            mv "$file" "$BACKUP_DIR/"
            echo -e "${YELLOW}[!]${NC} Backed up: $file (legacy OHI migration)"
            ;;
        docker-compose.secure.yml|docker-compose.production-secure.yml)
            mv "$file" "$BACKUP_DIR/"
            echo -e "${YELLOW}[!]${NC} Backed up: $file (merged into prod config)"
            ;;
        *)
            # Other docker-compose files
            mv "$file" "$BACKUP_DIR/"
            echo -e "${YELLOW}[!]${NC} Backed up: $file"
            ;;
    esac
done

# Create deployment README
echo -e "\n${BLUE}Creating deployment documentation...${NC}"
cat > deployments/README.md << 'EOF'
# Database Intelligence Deployments

This directory contains all deployment configurations for the Database Intelligence project.

## Directory Structure

```
deployments/
├── docker/           # Docker-related deployments
│   ├── compose/      # Docker Compose configurations
│   │   ├── docker-compose.yaml       # Default development setup
│   │   ├── docker-compose.prod.yaml  # Production deployment
│   │   ├── docker-compose.test.yaml  # Testing environment
│   │   ├── docker-compose-databases.yaml  # Database-only setup
│   │   └── docker-compose-ha.yaml    # High availability setup
│   ├── dockerfiles/  # Dockerfile definitions
│   │   ├── Dockerfile              # Main collector image
│   │   ├── Dockerfile.custom       # Custom build
│   │   ├── Dockerfile.loadgen      # Load generator
│   │   └── Dockerfile.test         # Test runner
│   └── init-scripts/ # Database initialization scripts
├── kubernetes/       # Kubernetes manifests
│   ├── base/         # Base Kustomize configurations
│   └── overlays/     # Environment-specific overlays
└── helm/             # Helm charts
    └── database-intelligence/  # Main Helm chart
```

## Quick Start

### Docker Compose

1. **Development Environment**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.yaml up
   ```

2. **Production Deployment**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.prod.yaml up -d
   ```

3. **Running Tests**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.test.yaml run tests
   ```

### Kubernetes

1. **Deploy with kubectl**:
   ```bash
   kubectl apply -k deployments/kubernetes/base/
   ```

2. **Deploy to specific environment**:
   ```bash
   # Development
   kubectl apply -k deployments/kubernetes/overlays/dev/
   
   # Production
   kubectl apply -k deployments/kubernetes/overlays/production/
   ```

### Helm

1. **Install with Helm**:
   ```bash
   helm install database-intelligence deployments/helm/database-intelligence/
   ```

2. **Upgrade deployment**:
   ```bash
   helm upgrade database-intelligence deployments/helm/database-intelligence/
   ```

## Environment Configuration

All deployments support configuration through environment variables. See `configs/templates/environment-template.env` for available options.

### Required Variables
- `DB_USERNAME` - Database username
- `DB_PASSWORD` - Database password
- `NEW_RELIC_LICENSE_KEY` - New Relic API key (if using New Relic export)

### Optional Variables
- `OTEL_LOG_LEVEL` - Logging level (default: info)
- `ENABLE_PROFILING` - Enable performance profiling (default: false)
- `METRIC_INTERVAL` - Metric collection interval (default: 10s)

## Docker Images

### Building Images

```bash
# Build main collector image
docker build -f deployments/docker/dockerfiles/Dockerfile -t database-intelligence:latest .

# Build with custom configuration
docker build -f deployments/docker/dockerfiles/Dockerfile.custom -t database-intelligence:custom .
```

### Multi-platform Builds

```bash
# Build for multiple platforms
docker buildx build --platform linux/amd64,linux/arm64 \
  -f deployments/docker/dockerfiles/Dockerfile \
  -t database-intelligence:latest .
```

## Production Considerations

1. **Resource Limits**: Set appropriate CPU and memory limits
2. **Persistence**: Mount volumes for data persistence
3. **Security**: Use secrets for sensitive configuration
4. **Monitoring**: Enable health checks and metrics export
5. **Scaling**: Use horizontal pod autoscaling for Kubernetes

## Troubleshooting

See [Operations Guide](../docs/operations/deployment.md) for detailed troubleshooting steps.
EOF

echo -e "${GREEN}[✓]${NC} Created deployment documentation"

# Consolidate Kubernetes files
echo -e "\n${BLUE}Organizing Kubernetes deployments...${NC}"

# Ensure proper structure
mkdir -p deployments/kubernetes/base
mkdir -p deployments/kubernetes/overlays/dev
mkdir -p deployments/kubernetes/overlays/staging  
mkdir -p deployments/kubernetes/overlays/production

# Create kustomization files if missing
if [ ! -f "deployments/kubernetes/base/kustomization.yaml" ]; then
    cat > deployments/kubernetes/base/kustomization.yaml << 'EOF'
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: database-intelligence

resources:
  - namespace.yaml
  - configmap.yaml
  - secret.yaml
  - deployment.yaml
  - service.yaml
  - rbac.yaml
  - networkpolicy.yaml

configMapGenerator:
  - name: collector-config
    files:
      - config.yaml=../../../configs/examples/collector.yaml

secretGenerator:
  - name: collector-secrets
    envs:
      - secrets.env
EOF
    echo -e "${GREEN}[✓]${NC} Created base kustomization.yaml"
fi

# Clean up Helm charts
echo -e "\n${BLUE}Consolidating Helm charts...${NC}"

# Remove duplicate Helm charts
if [ -d "deployments/helm/db-intelligence" ] && [ -d "deployments/helm/database-intelligence" ]; then
    # Merge values files
    if [ -f "deployments/helm/db-intelligence/values.yaml" ]; then
        cp deployments/helm/db-intelligence/values*.yaml "$BACKUP_DIR/"
        echo -e "${YELLOW}[!]${NC} Backed up db-intelligence Helm values"
    fi
    
    # Remove duplicate chart
    rm -rf deployments/helm/db-intelligence
    echo -e "${GREEN}[✓]${NC} Removed duplicate Helm chart"
fi

# Remove postgres-collector if it's a duplicate
if [ -d "deployments/helm/postgres-collector" ]; then
    mv deployments/helm/postgres-collector "$BACKUP_DIR/"
    echo -e "${YELLOW}[!]${NC} Backed up postgres-collector Helm chart"
fi

# Create docker-compose environment file template
echo -e "\n${BLUE}Creating Docker Compose environment template...${NC}"
cat > deployments/docker/compose/.env.example << 'EOF'
# Database Intelligence Docker Compose Environment

# PostgreSQL Configuration
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=password
POSTGRES_DB=testdb

# MySQL Configuration  
MYSQL_HOST=mysql
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=password
MYSQL_DATABASE=testdb

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your-license-key-here
NEW_RELIC_ACCOUNT_ID=your-account-id

# Collector Configuration
OTEL_LOG_LEVEL=info
OTEL_RESOURCE_ATTRIBUTES=service.name=database-intelligence,environment=docker

# Feature Flags
ENABLE_ADAPTIVE_SAMPLER=true
ENABLE_CIRCUIT_BREAKER=true
ENABLE_COST_CONTROL=true
ENABLE_PLAN_EXTRACTOR=true
ENABLE_PII_DETECTION=true
EOF

echo -e "${GREEN}[✓]${NC} Created Docker Compose environment template"

# Summary
echo -e "\n${BLUE}=== Deployment Consolidation Complete ===${NC}"
echo -e "${GREEN}[✓]${NC} Docker Compose files consolidated"
echo -e "${GREEN}[✓]${NC} Kubernetes structure organized"
echo -e "${GREEN}[✓]${NC} Helm charts cleaned up"
echo -e "${GREEN}[✓]${NC} Documentation created"
echo -e "\nBackup location: $BACKUP_DIR"

# Show final structure
echo -e "\n${BLUE}Final deployment structure:${NC}"
tree -d -L 3 deployments/ 2>/dev/null || find deployments -type d -maxdepth 3 | sort