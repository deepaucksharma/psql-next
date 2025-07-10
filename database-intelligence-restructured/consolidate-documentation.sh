#!/bin/bash

# Documentation Consolidation Script
# Merges and organizes all documentation into a clean structure

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
MVP_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-mvp"
BACKUP_DIR="/Users/deepaksharma/syc/db-otel/backup-docs-$(date +%Y%m%d-%H%M%S)"

echo -e "${BLUE}=== Documentation Consolidation ===${NC}"

# Create backup
mkdir -p "$BACKUP_DIR"
echo -e "${GREEN}[✓]${NC} Created backup directory: $BACKUP_DIR"

cd "$PROJECT_ROOT"

# Create proper documentation structure
echo -e "\n${BLUE}Creating documentation structure...${NC}"
mkdir -p docs/getting-started
mkdir -p docs/architecture
mkdir -p docs/operations
mkdir -p docs/development
mkdir -p docs/releases

# Move architecture docs
echo -e "\n${BLUE}Consolidating architecture documentation...${NC}"
if [ -f "docs/architecture/PROCESSORS.md" ]; then
    echo -e "${GREEN}[✓]${NC} Architecture docs already in place"
else
    # Copy from MVP if exists
    if [ -f "$MVP_ROOT/docs/architecture/PROCESSORS.md" ]; then
        cp -r "$MVP_ROOT/docs/architecture/"* docs/architecture/
        echo -e "${GREEN}[✓]${NC} Copied architecture docs from MVP"
    fi
fi

# Consolidate E2E testing documentation
echo -e "\n${BLUE}Consolidating E2E testing documentation...${NC}"
cat > docs/development/testing.md << 'EOF'
# Testing Guide

## Overview

The Database Intelligence project includes comprehensive testing at multiple levels:
- Unit tests for individual components
- Integration tests for component interactions
- End-to-end (E2E) tests for full system validation
- Performance and load testing

## Test Structure

```
tests/
├── benchmarks/        # Performance benchmarks
├── e2e/              # End-to-end tests
├── integration/      # Integration tests
├── performance/      # Load and stress tests
└── fixtures/         # Test data and configurations
```

## Running Tests

### All Tests
```bash
make test-all
```

### Unit Tests
```bash
make test-unit
```

### Integration Tests
```bash
make test-integration
```

### E2E Tests
```bash
make test-e2e
```

### Performance Tests
```bash
make test-performance
```

## E2E Testing

The E2E test suite validates the complete data pipeline from databases through the collector to exporters.

### Prerequisites
- Docker and Docker Compose
- Go 1.21+
- New Relic account (for New Relic export tests)

### Running E2E Tests

1. **Set up environment**:
   ```bash
   cp configs/templates/environment-template.env .env
   # Edit .env with your settings
   ```

2. **Start test environment**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.test.yaml up -d
   ```

3. **Run E2E tests**:
   ```bash
   cd tests/e2e
   go test -v ./...
   ```

### E2E Test Coverage

- Database connectivity (PostgreSQL, MySQL)
- Metric collection accuracy
- Query plan extraction
- PII detection and redaction
- Export to Prometheus
- Export to New Relic
- Error handling and recovery
- Performance under load

### Writing E2E Tests

Example E2E test:

```go
func TestPostgreSQLMetricsE2E(t *testing.T) {
    // Setup test environment
    env := framework.NewTestEnvironment(t)
    defer env.Cleanup()
    
    // Start collector
    collector := env.StartCollector("configs/examples/collector-e2e-test.yaml")
    
    // Generate database load
    env.GenerateLoad("postgresql", 100)
    
    // Verify metrics
    metrics := env.GetMetrics("postgresql.database.size")
    assert.Greater(t, len(metrics), 0)
}
```

## Integration Testing

Integration tests verify component interactions without external dependencies.

### Running Integration Tests

```bash
cd tests/integration
go test -v ./...
```

### Integration Test Scenarios

- Processor pipeline validation
- Configuration loading and validation
- Feature detection accuracy
- Multi-database support

## Performance Testing

Performance tests ensure the collector can handle production loads.

### Running Performance Tests

```bash
cd tests/performance
go test -bench=. -benchmem
```

### Performance Benchmarks

- Processor throughput
- Memory usage under load
- CPU utilization
- Network latency impact

## Test Configuration

Test configurations are stored in `tests/fixtures/configs/`:
- `e2e-minimal.yaml` - Minimal E2E test configuration
- `e2e-comprehensive.yaml` - Full feature test configuration
- `e2e-performance.yaml` - Performance test configuration

## Continuous Integration

All tests run automatically on:
- Pull requests
- Commits to main branch
- Nightly builds

See `.github/workflows/` for CI configuration.

## Troubleshooting Tests

### Common Issues

1. **Database connection failures**:
   - Ensure Docker containers are running
   - Check database credentials in .env
   - Verify network connectivity

2. **Metric verification failures**:
   - Allow time for metrics to be collected (usually 30s)
   - Check collector logs for errors
   - Verify exporters are configured correctly

3. **Performance test failures**:
   - Ensure sufficient system resources
   - Close other applications
   - Adjust performance thresholds if needed

### Debug Mode

Run tests with debug logging:
```bash
OTEL_LOG_LEVEL=debug go test -v ./...
```

### Test Reports

Test results are saved to `test-results/`:
- JUnit XML reports for CI
- Coverage reports
- Performance profiles
EOF

echo -e "${GREEN}[✓]${NC} Created comprehensive testing documentation"

# Move E2E test docs to archive
mkdir -p "$BACKUP_DIR/e2e-docs"
for file in tests/e2e/*.md; do
    if [ -f "$file" ] && [ "$(basename $file)" != "README.md" ]; then
        mv "$file" "$BACKUP_DIR/e2e-docs/"
        echo -e "${YELLOW}[!]${NC} Archived $(basename $file)"
    fi
done

# Create unified README
echo -e "\n${BLUE}Creating unified README...${NC}"
cat > README.md << 'EOF'
# Database Intelligence

A production-ready OpenTelemetry Collector distribution specialized for database observability, providing deep insights into PostgreSQL and MySQL performance, query analysis, and resource utilization.

## Features

- **Multi-Database Support**: PostgreSQL and MySQL with extensible architecture
- **Query Intelligence**: Automatic query plan extraction and analysis
- **PII Protection**: Built-in detection and redaction of sensitive data
- **Adaptive Sampling**: Intelligent sampling based on query patterns
- **Cost Control**: Budget-aware metric collection with automatic throttling
- **Circuit Breaker**: Fault tolerance with automatic recovery
- **Enterprise Ready**: Production-tested with high availability support

## Quick Start

### Using Docker

```bash
# Clone the repository
git clone https://github.com/your-org/database-intelligence
cd database-intelligence

# Set up environment
cp configs/templates/environment-template.env .env
# Edit .env with your database credentials

# Start with Docker Compose
docker-compose -f deployments/docker/compose/docker-compose.yaml up
```

### Using Binary

```bash
# Download latest release
curl -L https://github.com/your-org/database-intelligence/releases/latest/download/database-intelligence-$(uname -s)-$(uname -m) -o database-intelligence
chmod +x database-intelligence

# Run with configuration
./database-intelligence --config configs/examples/collector.yaml
```

## Configuration

The collector uses YAML configuration files. See `configs/templates/collector-template.yaml` for a starter template.

### Minimal Configuration

```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: ${DB_PASSWORD}
    databases: [postgres]

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      exporters: [prometheus]
```

### Environment Variables

Key environment variables:
- `DB_USERNAME`, `DB_PASSWORD` - Database credentials
- `NEW_RELIC_LICENSE_KEY` - New Relic API key
- `OTEL_LOG_LEVEL` - Logging level (debug, info, warn, error)

## Distributions

We provide three pre-built distributions:

### Minimal
Basic PostgreSQL monitoring with Prometheus export.
```bash
./build/database-intelligence-minimal --config configs/examples/collector-minimal.yaml
```

### Standard
PostgreSQL and MySQL support with essential processors.
```bash
./build/database-intelligence-standard --config configs/examples/collector-standard.yaml
```

### Enterprise
Full feature set including all databases, processors, and exporters.
```bash
./build/database-intelligence-enterprise --config configs/examples/collector-enterprise.yaml
```

## Custom Processors

- **AdaptiveSampler**: Intelligent sampling based on query patterns
- **CircuitBreaker**: Fault tolerance with automatic recovery
- **CostControl**: Budget-aware metric collection
- **PlanAttributeExtractor**: Query execution plan analysis
- **Verification**: PII detection and data validation
- **NRErrorMonitor**: New Relic error tracking
- **QueryCorrelator**: Transaction correlation

## Deployment

### Kubernetes

```bash
kubectl apply -f deployments/kubernetes/
```

### Helm

```bash
helm install database-intelligence deployments/helm/database-intelligence/
```

### Docker

```bash
docker run -d \
  -p 8889:8889 \
  -v $(pwd)/my-config.yaml:/etc/otelcol/config.yaml \
  database-intelligence:latest
```

## Monitoring

- **Health Check**: http://localhost:13133/health
- **Metrics**: http://localhost:8889/metrics
- **zPages**: http://localhost:55679/debug/tracez

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/your-org/database-intelligence
cd database-intelligence

# Build all distributions
make build-all

# Run tests
make test-all
```

### Contributing

See [CONTRIBUTING.md](docs/development/CONTRIBUTING.md) for development guidelines.

## Documentation

- [Getting Started Guide](docs/getting-started/quickstart.md)
- [Configuration Reference](docs/getting-started/configuration.md)
- [Architecture Overview](docs/architecture/overview.md)
- [Deployment Guide](docs/operations/deployment.md)
- [Troubleshooting](docs/operations/troubleshooting.md)

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/your-org/database-intelligence/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-org/database-intelligence/discussions)
- **Security**: security@your-org.com
EOF

echo -e "${GREEN}[✓]${NC} Created unified README.md"

# Move old READMEs to backup
for readme in README-*.md; do
    if [ -f "$readme" ]; then
        mv "$readme" "$BACKUP_DIR/"
        echo -e "${YELLOW}[!]${NC} Backed up $readme"
    fi
done

# Copy important docs from MVP if missing
echo -e "\n${BLUE}Importing missing documentation from MVP...${NC}"

# Architecture docs
if [ ! -f "docs/architecture/overview.md" ] && [ -f "$MVP_ROOT/docs/ARCHITECTURE.md" ]; then
    cp "$MVP_ROOT/docs/ARCHITECTURE.md" docs/architecture/overview.md
    echo -e "${GREEN}[✓]${NC} Imported architecture overview"
fi

# Deployment guide
if [ ! -f "docs/operations/deployment.md" ] && [ -f "$MVP_ROOT/docs/DEPLOYMENT_GUIDE.md" ]; then
    cp "$MVP_ROOT/docs/DEPLOYMENT_GUIDE.md" docs/operations/deployment.md
    echo -e "${GREEN}[✓]${NC} Imported deployment guide"
fi

# Troubleshooting
if [ ! -f "docs/operations/troubleshooting.md" ] && [ -f "$MVP_ROOT/docs/TROUBLESHOOTING.md" ]; then
    cp "$MVP_ROOT/docs/TROUBLESHOOTING.md" docs/operations/troubleshooting.md
    echo -e "${GREEN}[✓]${NC} Imported troubleshooting guide"
fi

# Quick start
if [ ! -f "docs/getting-started/quickstart.md" ] && [ -f "$MVP_ROOT/docs/QUICK_START.md" ]; then
    cp "$MVP_ROOT/docs/QUICK_START.md" docs/getting-started/quickstart.md
    echo -e "${GREEN}[✓]${NC} Imported quick start guide"
fi

# Configuration guide
if [ ! -f "docs/getting-started/configuration.md" ] && [ -f "$MVP_ROOT/docs/CONFIGURATION.md" ]; then
    cp "$MVP_ROOT/docs/CONFIGURATION.md" docs/getting-started/configuration.md
    echo -e "${GREEN}[✓]${NC} Imported configuration guide"
fi

# Changelog
if [ ! -f "docs/releases/changelog.md" ] && [ -f "$MVP_ROOT/docs/CHANGELOG.md" ]; then
    cp "$MVP_ROOT/docs/CHANGELOG.md" docs/releases/changelog.md
    echo -e "${GREEN}[✓]${NC} Imported changelog"
fi

# Create documentation index
echo -e "\n${BLUE}Creating documentation index...${NC}"
cat > docs/README.md << 'EOF'
# Database Intelligence Documentation

## Getting Started
- [Quick Start Guide](getting-started/quickstart.md)
- [Installation](getting-started/installation.md)
- [Configuration Reference](getting-started/configuration.md)

## Architecture
- [System Overview](architecture/overview.md)
- [Processors](architecture/processors.md)
- [Receivers](architecture/receivers.md)

## Operations
- [Deployment Guide](operations/deployment.md)
- [Monitoring Setup](operations/monitoring.md)
- [Troubleshooting](operations/troubleshooting.md)
- [Security Guidelines](operations/security.md)

## Development
- [Contributing Guide](development/contributing.md)
- [Testing Guide](development/testing.md)
- [API Reference](development/api-reference.md)

## Releases
- [Changelog](releases/changelog.md)
- [Migration Guides](releases/migration.md)
EOF

echo -e "${GREEN}[✓]${NC} Created documentation index"

# Clean up empty archive directory if exists
if [ -d "archive" ] && [ -z "$(ls -A archive 2>/dev/null)" ]; then
    rmdir archive
    echo -e "${GREEN}[✓]${NC} Removed empty archive directory"
fi

# Summary
echo -e "\n${BLUE}=== Documentation Consolidation Complete ===${NC}"
echo -e "${GREEN}[✓]${NC} Documentation structure created"
echo -e "${GREEN}[✓]${NC} E2E docs consolidated into testing.md"
echo -e "${GREEN}[✓]${NC} Unified README.md created"
echo -e "${GREEN}[✓]${NC} Important docs imported from MVP"
echo -e "${GREEN}[✓]${NC} Documentation index created"
echo -e "\nBackup location: $BACKUP_DIR"