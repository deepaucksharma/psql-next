#!/bin/bash

# Configuration Consolidation Script
# This script organizes and consolidates all configuration files

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
BACKUP_DIR="/Users/deepaksharma/syc/db-otel/backup-configs-$(date +%Y%m%d-%H%M%S)"

echo -e "${BLUE}=== Configuration Consolidation ===${NC}"

# Create backup
mkdir -p "$BACKUP_DIR"
echo -e "${GREEN}[✓]${NC} Created backup directory: $BACKUP_DIR"

cd "$PROJECT_ROOT"

# Move docker-compose files from root to deployments
echo -e "\n${BLUE}Consolidating Docker Compose files...${NC}"
mkdir -p deployments/docker/compose

# Move root-level docker-compose files
for file in docker-compose*.yml docker-compose*.yaml; do
    if [ -f "$file" ]; then
        # Determine target name based on purpose
        case "$file" in
            docker-compose.unified.yml)
                mv "$file" deployments/docker/compose/docker-compose.yaml
                echo -e "${GREEN}[✓]${NC} Moved $file to docker-compose.yaml (default)"
                ;;
            docker-compose.production.yml)
                mv "$file" deployments/docker/compose/docker-compose.prod.yaml
                echo -e "${GREEN}[✓]${NC} Moved $file to docker-compose.prod.yaml"
                ;;
            *)
                mv "$file" "$BACKUP_DIR/"
                echo -e "${YELLOW}[!]${NC} Backed up $file"
                ;;
        esac
    fi
done

# Consolidate test configurations
echo -e "\n${BLUE}Organizing test configurations...${NC}"
mkdir -p tests/fixtures/configs

# Move test configs from root
for file in test-receivers-config.yaml; do
    if [ -f "$file" ]; then
        mv "$file" tests/fixtures/configs/
        echo -e "${GREEN}[✓]${NC} Moved $file to tests/fixtures/configs/"
    fi
done

# Remove redundant example configurations
echo -e "\n${BLUE}Cleaning up redundant configurations...${NC}"

# List of configs that have duplicates
REDUNDANT_CONFIGS=(
    "configs/examples/collector-simple-alternate.yaml"
    "configs/examples/collector-minimal-test.yaml"
    "configs/examples/collector-local-test.yaml"
)

for config in "${REDUNDANT_CONFIGS[@]}"; do
    if [ -f "$config" ]; then
        mv "$config" "$BACKUP_DIR/"
        echo -e "${GREEN}[✓]${NC} Removed redundant: $(basename $config)"
    fi
done

# Consolidate environment configurations
echo -e "\n${BLUE}Organizing environment configurations...${NC}"

# Move environment configs from examples to overlays
if [ -f "configs/examples/development.yaml" ]; then
    mv configs/examples/development.yaml configs/overlays/environments/development/
    echo -e "${GREEN}[✓]${NC} Moved development.yaml to overlays"
fi

if [ -f "configs/examples/staging.yaml" ]; then
    mv configs/examples/staging.yaml configs/overlays/environments/staging/
    echo -e "${GREEN}[✓]${NC} Moved staging.yaml to overlays"
fi

if [ -f "configs/examples/production.yaml" ]; then
    mv configs/examples/production.yaml configs/overlays/environments/production/
    echo -e "${GREEN}[✓]${NC} Moved production.yaml to overlays"
fi

# Create configuration templates
echo -e "\n${BLUE}Creating configuration templates...${NC}"
mkdir -p configs/templates

cat > configs/templates/collector-template.yaml << 'EOF'
# Database Intelligence Collector Configuration Template
# Copy and modify this template for your specific use case

receivers:
  # PostgreSQL receiver
  postgresql:
    endpoint: localhost:5432
    username: ${DB_USERNAME}
    password: ${DB_PASSWORD}
    databases:
      - ${DB_NAME}
    collection_interval: 10s
    
  # MySQL receiver (optional)
  # mysql:
  #   endpoint: localhost:3306
  #   username: ${MYSQL_USERNAME}
  #   password: ${MYSQL_PASSWORD}
  #   database: ${MYSQL_DATABASE}

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
    
  # Add custom processors as needed
  # adaptivesampler:
  #   enabled: true
  # circuitbreaker:
  #   enabled: true

exporters:
  # Choose your exporters
  prometheus:
    endpoint: "0.0.0.0:8889"
    
  # otlp/newrelic:
  #   endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
  #   headers:
  #     api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [prometheus]
EOF

echo -e "${GREEN}[✓]${NC} Created collector configuration template"

# Create environment template
cat > configs/templates/environment-template.env << 'EOF'
# Database Intelligence Environment Configuration Template
# Copy to .env and fill in your values

# Database Configuration
DB_USERNAME=postgres
DB_PASSWORD=postgres
DB_NAME=postgres
DB_HOST=localhost
DB_PORT=5432

# MySQL Configuration (optional)
MYSQL_USERNAME=root
MYSQL_PASSWORD=root
MYSQL_DATABASE=mysql
MYSQL_HOST=localhost
MYSQL_PORT=3306

# New Relic Configuration (optional)
NEW_RELIC_LICENSE_KEY=your-license-key-here
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Collector Configuration
OTEL_LOG_LEVEL=info
OTEL_RESOURCE_ATTRIBUTES=service.name=database-intelligence

# Performance Settings
GOMAXPROCS=4
GOMEMLIMIT=1GiB
EOF

echo -e "${GREEN}[✓]${NC} Created environment configuration template"

# Create configuration README
cat > configs/README.md << 'EOF'
# Database Intelligence Configuration

This directory contains all configuration files for the Database Intelligence project.

## Directory Structure

```
configs/
├── base/              # Base component configurations
│   ├── exporters.yaml    # Available exporters
│   ├── extensions.yaml   # Available extensions
│   ├── processors.yaml   # Available processors
│   └── receivers.yaml    # Available receivers
├── examples/          # Example collector configurations
│   ├── collector-*.yaml  # Various collector examples
│   └── receiver-*.yaml   # Receiver-specific examples
├── overlays/          # Configuration overlays
│   ├── environments/     # Environment-specific overlays
│   └── features/        # Feature-specific overlays
├── queries/           # Database query definitions
│   ├── mysql/           # MySQL queries
│   └── postgresql/      # PostgreSQL queries
├── templates/         # Configuration templates
│   ├── collector-template.yaml
│   └── environment-template.env
└── unified/           # Unified configurations
    └── database-intelligence-complete.yaml
```

## Usage

### Quick Start

1. Copy the template:
   ```bash
   cp configs/templates/collector-template.yaml my-config.yaml
   cp configs/templates/environment-template.env .env
   ```

2. Edit the configuration files with your settings

3. Run the collector:
   ```bash
   ./database-intelligence --config my-config.yaml
   ```

### Using Overlays

Apply environment-specific settings:
```bash
# Development
./database-intelligence --config configs/examples/collector.yaml \
  --config configs/overlays/environments/development/

# Production  
./database-intelligence --config configs/examples/collector.yaml \
  --config configs/overlays/environments/production/
```

### Examples

See the `examples/` directory for various configuration scenarios:
- `collector-gateway-enterprise.yaml` - Enterprise gateway configuration
- `collector-plan-intelligence.yaml` - Query plan intelligence
- `collector-querylens.yaml` - PostgreSQL query lens integration
- `collector-secure.yaml` - Security-focused configuration

## Configuration Reference

For detailed configuration options, see:
- [Receivers Documentation](../docs/architecture/receivers.md)
- [Processors Documentation](../docs/architecture/processors.md)
- [Exporters Documentation](../docs/architecture/exporters.md)
EOF

echo -e "${GREEN}[✓]${NC} Created configuration README"

# Summary
echo -e "\n${BLUE}=== Configuration Consolidation Complete ===${NC}"
echo -e "${GREEN}[✓]${NC} Docker compose files organized"
echo -e "${GREEN}[✓]${NC} Test configurations consolidated"
echo -e "${GREEN}[✓]${NC} Redundant configs removed"
echo -e "${GREEN}[✓]${NC} Templates created"
echo -e "${GREEN}[✓]${NC} Documentation added"
echo -e "\nBackup location: $BACKUP_DIR"