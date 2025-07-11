#!/bin/bash
# Script to clean up and organize configuration files

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Configuration Cleanup and Organization Script${NC}"
echo "=============================================="

# Create organized config structure
echo -e "${YELLOW}Creating organized configuration structure...${NC}"

# Main configs directory structure
mkdir -p configs/{production,development,staging,examples,templates}

# Move production configs
echo -e "${YELLOW}Organizing production configurations...${NC}"
if [ -f "distributions/production/production-config.yaml" ]; then
    cp distributions/production/production-config.yaml configs/production/config-basic.yaml
fi
if [ -f "distributions/production/production-config-enhanced.yaml" ]; then
    cp distributions/production/production-config-enhanced.yaml configs/production/config-enhanced.yaml
fi
if [ -f "distributions/production/production-config-full.yaml" ]; then
    cp distributions/production/production-config-full.yaml configs/production/config-full.yaml
fi

# Move templates
echo -e "${YELLOW}Organizing templates...${NC}"
if [ -f "configs/.env.template" ]; then
    mv configs/.env.template configs/templates/env.template
fi
if [ -f "configs/.env.template.fixed" ]; then
    mv configs/.env.template.fixed configs/templates/env.template.fixed
fi

# Copy builder configs to templates
cp otelcol-builder-config.yaml configs/templates/builder-config-basic.yaml
cp otelcol-builder-config-complete.yaml configs/templates/builder-config-complete.yaml

# Move example configs
echo -e "${YELLOW}Organizing example configurations...${NC}"
if [ -d "configs/examples" ]; then
    cp -r configs/examples/* configs/examples/ 2>/dev/null || true
fi

# Create development and staging configs based on production
echo -e "${YELLOW}Creating environment-specific configurations...${NC}"

# Development config
cat > configs/development/config.yaml << 'EOF'
# Development configuration - includes debug exporters
# Inherits from production-config-enhanced.yaml with development overrides

receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 30s
    tls:
      insecure: true
  
  mysql:
    endpoint: localhost:3306
    username: root
    password: mysql
    database: mysql
    collection_interval: 30s
    tls:
      insecure: true

processors:
  resource:
    attributes:
      - key: service.name
        value: database-intelligence-dev
        action: upsert
      - key: deployment.environment
        value: development
        action: upsert
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
    spike_limit_mib: 64
    
  batch:
    timeout: 1s
    send_batch_size: 512

exporters:
  # File exporter for local development
  file:
    path: /tmp/otel-dev-data.json
    format: json
  
  # Debug exporter with detailed verbosity
  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100
  
  # Prometheus for local monitoring
  prometheus:
    endpoint: 0.0.0.0:8889
    namespace: db_intel_dev

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  
  pprof:
    endpoint: 0.0.0.0:1777
  
  zpages:
    endpoint: 0.0.0.0:55679

service:
  extensions: [health_check, pprof, zpages]
  
  pipelines:
    metrics:
      receivers: [otlp, postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [file, debug, prometheus]
    
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [file, debug]
    
    logs:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [file, debug]
  
  telemetry:
    logs:
      level: debug
      development: true
      encoding: console
    metrics:
      level: detailed
      address: 0.0.0.0:8888
EOF

# Staging config
cat > configs/staging/config.yaml << 'EOF'
# Staging configuration - similar to production but with staging endpoints

receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  
  postgresql:
    endpoint: ${env:POSTGRES_HOST:-staging-db.internal}:${env:POSTGRES_PORT:-5432}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB:-postgres}
    collection_interval: 60s
    tls:
      insecure: false
      ca_file: ${env:POSTGRES_CA_FILE}
  
  mysql:
    endpoint: ${env:MYSQL_HOST:-staging-mysql.internal}:${env:MYSQL_PORT:-3306}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    database: ${env:MYSQL_DB:-mysql}
    collection_interval: 60s

processors:
  resource:
    attributes:
      - key: service.name
        value: ${env:SERVICE_NAME:-database-intelligence-staging}
        action: upsert
      - key: deployment.environment
        value: staging
        action: upsert
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
    
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  otlphttp/newrelic:
    endpoint: ${env:STAGING_OTLP_ENDPOINT:-https://staging-otlp.nr-data.net}
    headers:
      api-key: ${env:STAGING_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
  
  prometheus:
    endpoint: 0.0.0.0:8889
    namespace: database_intelligence_staging

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
  
  pipelines:
    metrics/postgresql:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp/newrelic, prometheus]
    
    metrics/mysql:
      receivers: [mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp/newrelic, prometheus]
    
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp/newrelic, prometheus]
    
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp/newrelic]
    
    logs:
      receivers: [otlp]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp/newrelic]
  
  telemetry:
    logs:
      level: info
      encoding: json
    metrics:
      level: normal
      address: 0.0.0.0:8888
EOF

# Create README for configs
cat > configs/README.md << 'EOF'
# Database Intelligence Collector Configurations

This directory contains all configuration files for the Database Intelligence OpenTelemetry Collector.

## Directory Structure

- **production/** - Production-ready configurations
  - `config-basic.yaml` - Basic configuration with standard components only
  - `config-enhanced.yaml` - Enhanced configuration with resource processors and full telemetry
  - `config-full.yaml` - Full configuration including all custom components
  
- **development/** - Development configurations with debug exporters and local file output
  
- **staging/** - Staging configurations with staging endpoints
  
- **examples/** - Example configurations for various use cases
  
- **templates/** - Configuration templates and builder configs
  - `env.template` - Environment variable template
  - `builder-config-*.yaml` - OpenTelemetry Collector Builder configurations

## Usage

1. Choose the appropriate configuration file for your environment
2. Copy the `env.template` to `.env` and fill in your values
3. Run the collector with: `./database-intelligence-collector --config=<config-file>`

## Configuration Levels

### Basic (config-basic.yaml)
- Standard OpenTelemetry components only
- PostgreSQL and MySQL receivers
- OTLP export to New Relic
- Suitable for simple monitoring needs

### Enhanced (config-enhanced.yaml)
- Includes resource processors for service identification
- TLS support for database connections
- Multiple exporters (New Relic, Prometheus)
- Health check and debugging extensions
- Recommended for most production deployments

### Full (config-full.yaml)
- All custom components included
- Advanced processors: adaptive sampling, circuit breaker, cost control, etc.
- Custom receivers: ASH, enhanced SQL, kernel metrics
- Complete database intelligence pipeline
- For advanced monitoring and analysis needs

## Environment Variables

See `templates/env.template` for all available environment variables.

Key variables:
- `NEW_RELIC_LICENSE_KEY` - Your New Relic ingest key
- `POSTGRES_*` - PostgreSQL connection settings
- `MYSQL_*` - MySQL connection settings
- `SERVICE_NAME` - Name of your service
- `DEPLOYMENT_ENVIRONMENT` - Environment (development/staging/production)
EOF

# Archive old configs
echo -e "${YELLOW}Archiving old configuration files...${NC}"
mkdir -p archive/configs-backup-$(date +%Y%m%d)
find . -name "*.yaml" -path "./configs/*" -not -path "./configs/production/*" \
       -not -path "./configs/development/*" -not -path "./configs/staging/*" \
       -not -path "./configs/examples/*" -not -path "./configs/templates/*" \
       -exec mv {} archive/configs-backup-$(date +%Y%m%d)/ \; 2>/dev/null || true

# Clean up test configs
echo -e "${YELLOW}Cleaning up test configurations...${NC}"
find . -name "test-*.yaml" -not -path "./archive/*" -exec mv {} archive/configs-backup-$(date +%Y%m%d)/ \; 2>/dev/null || true

# Create symlinks for convenience
echo -e "${YELLOW}Creating convenience symlinks...${NC}"
cd distributions/production
ln -sf ../../configs/production/config-enhanced.yaml production-config.yaml
ln -sf ../../configs/templates/env.template.fixed .env.template
cd ../..

echo -e "${GREEN}Configuration cleanup complete!${NC}"
echo ""
echo "New structure:"
echo "  configs/"
echo "  ├── production/     - Production configs (basic, enhanced, full)"
echo "  ├── development/    - Development config with debug exporters"
echo "  ├── staging/        - Staging config"
echo "  ├── examples/       - Example configurations"
echo "  ├── templates/      - Templates and builder configs"
echo "  └── README.md       - Configuration documentation"
echo ""
echo "Old configs have been archived to: archive/configs-backup-$(date +%Y%m%d)/"