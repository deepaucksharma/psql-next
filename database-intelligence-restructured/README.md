# Database Intelligence for PostgreSQL

[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-Enabled-blue)](https://opentelemetry.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-12%2B-336791)](https://www.postgresql.org)
[![New Relic](https://img.shields.io/badge/New%20Relic-Ready-1CE783)](https://newrelic.com)

Advanced PostgreSQL monitoring using OpenTelemetry with New Relic integration. Choose between standard OpenTelemetry components (Config-Only) or enhanced monitoring with custom components.

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Features](#features)
- [Documentation](#documentation)
- [Installation](#installation)
- [Configuration](#configuration)
- [Dashboards](#dashboards)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

## 🚀 Quick Start

Get up and running in 5 minutes:

```bash
# 1. Clone the repository
git clone https://github.com/newrelic/database-intelligence
cd database-intelligence

# 2. Set environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# 3. Deploy both modes for comparison
./scripts/deploy-parallel-modes.sh

# 4. Verify metrics
./scripts/verify-metrics.sh

# 5. View in New Relic
# Go to New Relic > Query Builder and run:
# SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 5 minutes ago
```

## ✨ Features

### Config-Only Mode (Standard OpenTelemetry)
- ✅ **35+ PostgreSQL metrics** out of the box
- ✅ **No custom build required** - uses standard OTel collector
- ✅ **Production ready** with minimal resource usage
- ✅ **SQL Query receiver** for custom metrics
- ✅ **Host metrics** (CPU, memory, disk, network)

### Custom/Enhanced Mode
Everything in Config-Only plus:
- 🚀 **Active Session History (ASH)** - Real-time session monitoring
- 🔍 **Query Intelligence** - Plan extraction and analysis
- 📊 **Wait Event Analysis** - Detailed performance insights
- 🛡️ **Adaptive Sampling** - Intelligent data reduction
- ⚡ **Circuit Breaker** - Overload protection

## 📚 Documentation

### Essential Guides
- [**Quick Start Guide**](docs/guides/QUICK_START.md) - Get running in 5 minutes
- [**Configuration Guide**](docs/guides/CONFIGURATION.md) - All configuration options
- [**Deployment Guide**](docs/guides/DEPLOYMENT.md) - Production deployment
- [**Troubleshooting Guide**](docs/guides/TROUBLESHOOTING.md) - Common issues and solutions

### Reference Documentation
- [**Metrics Reference**](docs/reference/METRICS.md) - All collected metrics
- [**Architecture Overview**](docs/reference/ARCHITECTURE.md) - System design
- [**API Reference**](docs/reference/API.md) - Component APIs

### Development
- [**Development Setup**](docs/development/SETUP.md) - Build from source
- [**Testing Guide**](docs/development/TESTING.md) - Run tests
- [**Contributing Guidelines**](CONTRIBUTING.md) - How to contribute

## 🛠️ Installation

### Using Docker (Recommended)

```bash
# Config-Only Mode (Standard OTel)
docker run -d \
  --name db-intel-collector \
  -v $(pwd)/configs/config-only-mode.yaml:/etc/otel-collector-config.yaml \
  -e NEW_RELIC_LICENSE_KEY=$NEW_RELIC_LICENSE_KEY \
  otel/opentelemetry-collector-contrib:latest

# Custom Mode (Enhanced Features)
docker run -d \
  --name db-intel-collector-custom \
  -v $(pwd)/configs/custom-mode.yaml:/etc/otel-collector-config.yaml \
  -e NEW_RELIC_LICENSE_KEY=$NEW_RELIC_LICENSE_KEY \
  newrelic/database-intelligence-enterprise:latest
```

### Using Kubernetes

```bash
kubectl apply -f deployments/k8s/
```

### Building from Source

```bash
make build
```

## ⚙️ Configuration

### Basic Configuration

```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - postgres
    collection_interval: 10s

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      exporters: [otlp]
```

See [Configuration Guide](docs/guides/CONFIGURATION.md) for complete options.

## 📊 Dashboards

Pre-built New Relic dashboard included:

```bash
# Deploy dashboard
./scripts/migrate-dashboard.sh deploy dashboards/newrelic/postgresql-parallel-dashboard.json
```

The dashboard includes:
- Executive Overview
- Connection & Performance Metrics
- Wait Events & Blocking Analysis
- Query Intelligence (Custom mode)
- System Resources
- Alert Recommendations

## 🔧 Troubleshooting

### No Metrics Appearing?

1. Check collector logs:
```bash
docker logs db-intel-collector
```

2. Verify connectivity:
```bash
docker exec db-intel-collector curl -s http://localhost:13133/health
```

3. Check New Relic:
```sql
SELECT count(*) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
SINCE 5 minutes ago
```

See [Troubleshooting Guide](docs/guides/TROUBLESHOOTING.md) for more help.

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md).

### Development Quick Start

```bash
# Setup development environment
make dev-setup

# Run tests
make test

# Build locally
make build
```

## 📄 License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

Built with:
- [OpenTelemetry](https://opentelemetry.io)
- [PostgreSQL](https://www.postgresql.org)
- [New Relic](https://newrelic.com)

---

**Current Version**: 2.0 (PostgreSQL-Only)  
**Status**: Production Ready  
**Support**: [GitHub Issues](https://github.com/newrelic/database-intelligence/issues)