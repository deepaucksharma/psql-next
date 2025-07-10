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
