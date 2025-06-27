# PostgreSQL Unified Collector Configuration

## Overview

This directory contains all configuration files for the PostgreSQL Unified Collector. The configuration system is designed to be flexible and environment-aware.

## Configuration Files

### Core Configurations

- **`collector-config.toml`** - Main configuration file (template)
- **`collector-config-env.toml`** - Environment-aware configuration using variable substitution
- **`.env.example`** - Example environment variables file

### Mode-Specific Configurations

- **`collector-nri.toml`** - Configuration for NRI-only mode
- **`collector-otlp.toml`** - Configuration for OTLP-only mode
- **`collector-hybrid.toml`** - Configuration for hybrid mode (both NRI and OTLP)

### Integration Configurations

- **`otel-collector-config.yaml`** - OpenTelemetry Collector configuration
- **`newrelic-infra.yml`** - New Relic Infrastructure agent configuration
- **`nri-postgresql-config.yml`** - Legacy NRI PostgreSQL integration config

## Configuration Hierarchy

1. **Environment Variables** - Highest priority
2. **Command Line Arguments** - Override config file settings
3. **Configuration File** - Base settings
4. **Default Values** - Built-in defaults

## Environment Variables

All configuration values can be overridden using environment variables:

```bash
# Database Connection
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DATABASE=postgres

# New Relic
NEW_RELIC_LICENSE_KEY=your-license-key
NEW_RELIC_ACCOUNT_ID=your-account-id
NEW_RELIC_API_KEY=your-api-key

# Collector Settings
COLLECTOR_MODE=hybrid
COLLECTION_INTERVAL_SECS=30
LOG_LEVEL=info
```

## Usage Examples

### Local Development
```bash
# Copy and configure environment
cp .env.example .env
# Edit .env with your settings

# Run with environment-aware config
./scripts/run.sh start --mode hybrid
```

### Docker
```bash
# Using docker-compose (reads .env automatically)
docker-compose up postgres-collector

# Using docker run with env file
docker run --env-file .env \
  -v $(pwd)/configs:/app/configs \
  postgres-unified-collector:latest
```

### Kubernetes
```bash
# Create secret from .env file
kubectl create secret generic collector-env \
  --from-env-file=.env \
  -n postgres-monitoring

# Deploy with ConfigMap
kubectl create configmap collector-config \
  --from-file=configs/collector-config-env.toml \
  -n postgres-monitoring
```

## Configuration Validation

The collector validates configuration on startup:
- Required fields must be present
- Connection strings are validated
- Mode-specific outputs are checked

Run validation without starting collection:
```bash
postgres-unified-collector --config configs/collector-config.toml --validate
```

## Best Practices

1. **Use Environment Variables** for sensitive data (passwords, API keys)
2. **Use Config Files** for static settings (intervals, thresholds)
3. **Version Control** config templates, not actual configs with secrets
4. **Validate Changes** before deploying to production
5. **Monitor Config** changes through audit logs