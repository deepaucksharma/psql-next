# Database Intelligence Collector

An enterprise-grade OpenTelemetry-based database monitoring solution that provides comprehensive observability for PostgreSQL and MySQL databases.

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Build the collector
make build

# Run with Docker Compose
docker-compose up -d

# View metrics
curl http://localhost:8888/metrics
```

## 📋 Overview

The Database Intelligence Collector is a production-ready monitoring solution built on OpenTelemetry that:

- **Collects** detailed metrics from PostgreSQL and MySQL databases
- **Processes** data with intelligent sampling and circuit breaker protection
- **Exports** metrics to New Relic, Prometheus, and other observability platforms
- **Scales** horizontally in Kubernetes environments
- **Protects** databases from monitoring overhead with adaptive rate limiting

## 🏗️ Architecture

The collector follows OpenTelemetry's pipeline architecture:

```
Databases → Receivers → Processors → Exporters → Observability Platforms
```

### Key Components

- **Receivers**: PostgreSQL, MySQL, and query log collectors
- **Processors**: Adaptive sampling, circuit breaker, plan extraction, PII detection
- **Exporters**: OTLP (New Relic), Prometheus, File, Debug
- **Extensions**: Health check, zPages for debugging

## 📦 Installation

### Prerequisites

- Go 1.21+ (for building from source)
- Docker & Docker Compose (for containerized deployment)
- PostgreSQL 12+ and/or MySQL 5.7+
- New Relic account (for cloud export)

### Building from Source

```bash
# Install dependencies
make setup

# Build the collector binary
make build

# Run tests
make test
```

### Docker Deployment

```bash
# Start with sample databases
docker-compose up -d

# Start production stack
docker-compose -f docker-compose.production.yml up -d
```

### Kubernetes Deployment

```bash
# Create namespace and secrets
kubectl apply -f k8s/namespace.yaml
kubectl create secret generic database-credentials \
  --from-literal=new-relic-license-key=YOUR_KEY \
  -n database-intelligence

# Deploy collector
kubectl apply -f k8s/
```

## ⚙️ Configuration

The collector uses YAML configuration files. See [`config/`](./config/) for examples.

### Basic Configuration

```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: monitor
    password: secure-password
    
processors:
  batch:
    timeout: 10s
    
exporters:
  otlp:
    endpoint: https://otlp.nr-data.net:4318
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
      
service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
```

## 📊 Metrics Collected

### PostgreSQL Metrics
- Connection statistics
- Query performance
- Buffer cache hit rates
- Replication lag
- Lock statistics
- Table/index sizes

### MySQL Metrics
- InnoDB buffer pool stats
- Query cache performance
- Replication status
- Connection pool usage
- Handler statistics
- Table locks

## 🔧 Advanced Features

- **Adaptive Sampling**: Intelligently samples data based on query patterns
- **Circuit Breaker**: Protects databases from monitoring overload
- **Query Plan Analysis**: Extracts insights from query execution plans
- **PII Detection**: Automatically sanitizes sensitive data
- **Auto-scaling**: Kubernetes HPA support for dynamic scaling

## 📖 Documentation

Comprehensive documentation is available in the [`docs/`](./docs/) directory:

- [Architecture Overview](./docs/ARCHITECTURE.md)
- [Configuration Guide](./docs/CONFIGURATION.md)
- [Deployment Guide](./docs/DEPLOYMENT.md)
- [Development Guide](./docs/development/README.md)
- [Troubleshooting](./docs/TROUBLESHOOTING.md)

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](./CONTRIBUTING.md) for details.

```bash
# Fork the repository
# Create your feature branch
git checkout -b feature/amazing-feature

# Commit your changes
git commit -m 'Add amazing feature'

# Push to the branch
git push origin feature/amazing-feature

# Open a Pull Request
```

## 🧪 Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run benchmarks
make benchmark
```

## 🚨 Monitoring & Alerts

The collector exposes its own metrics for monitoring:

- Health endpoint: `http://localhost:13133/health`
- Metrics endpoint: `http://localhost:8888/metrics`
- zPages debugging: `http://localhost:55679/debug/tracez`

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](./LICENSE) file for details.

## 🌟 Acknowledgments

Built with:
- [OpenTelemetry](https://opentelemetry.io/)
- [OpenTelemetry Collector](https://github.com/open-telemetry/opentelemetry-collector)
- [OpenTelemetry Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib)

## 📞 Support

- 📧 Email: support@database-intelligence.io
- 💬 Slack: [#database-intelligence](https://otel-community.slack.com)
- 🐛 Issues: [GitHub Issues](https://github.com/database-intelligence-mvp/database-intelligence-mvp/issues)

---

**Current Version**: 1.0.0 | **Status**: Production Ready