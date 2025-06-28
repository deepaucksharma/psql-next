# New Relic Database Intelligence MVP

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-enabled-orange)](https://opentelemetry.io)
[![Production Ready](https://img.shields.io/badge/Production-Ready-green)](DEPLOYMENT.md)

## What This Is

A production-ready OpenTelemetry Collector configuration that safely collects database performance metrics and query metadata, sending them to New Relic for analysis. Built on the principle of "Configure, Don't Build"—leveraging standard OTEL components for maximum stability and security.

### Key Features

- 🛡️ **Production-Safe**: Connects to read-replicas only, with built-in timeouts and resource limits.
- 🚀 **High Availability**: Natively supports HA deployments in Kubernetes for scalability and resilience.
- 🔒 **Security First**: Includes PII sanitization, credential management via secrets, and network policies.
- 📊 **Observable**: Provides comprehensive metrics, pre-built monitoring dashboards, and Prometheus alerting rules.
- 🎯 **Easy Setup**: A one-command `quickstart.sh` script provides an interactive setup and validation experience.
- 🔗 **Entity Correlation**: Automatically creates database entities in New Relic for seamless correlation.

### Current Capabilities

| Feature                   | PostgreSQL | MySQL | Status             |
| ------------------------- |:----------:|:-----:|:-------------------|
| Query Performance Metrics |      ✅      |   ✅    | Production         |
| Query Metadata Collection |      ✅      |   ✅    | Production         |
| Execution Plans           |      ❌      |   ❌    | Future Enhancement |
| PII Sanitization          |      ✅      |   ✅    | Production         |
| High Availability         |      ✅      |   ✅    | Production         |

## Architecture

The v1.0.0 production architecture uses a standard OpenTelemetry Collector deployed to Kubernetes. It uses a leader election mechanism to ensure that only one collector instance is actively polling the databases at any time, which allows for safe horizontal scaling.

For a detailed explanation of the architecture, including the data flow and the future vision with custom processors, please see the [**Technical Architecture**](ARCHITECTURE.md) document.

## Quick Start

Get up and running in minutes with the interactive quickstart script.

### Prerequisites

- **PostgreSQL**: `pg_stat_statements` extension enabled, and a read-only user with access to the `pg_stat_statements` view.
- **MySQL**: Performance Schema enabled with a read-only user.
- **Docker and Docker Compose**

### One-Command Setup

```bash
# Clone the repository and run the quickstart script
git clone https://github.com/newrelic/database-intelligence-mvp
cd database-intelligence-mvp
./quickstart.sh all
```

This will guide you through an interactive setup, validate your database connections, and start the collector in Docker.

## Production Deployment

The recommended deployment method for production is the Kubernetes HA (High Availability) configuration.

### Deployment Options

- **Kubernetes HA (Recommended)**: Provides a scalable and resilient deployment. See `deploy/k8s/ha-deployment.yaml`.
  ```bash
  kubectl apply -f deploy/k8s/ha-deployment.yaml
  ```
- **Docker Compose**: Ideal for local development and testing. See `deploy/docker/docker-compose.yaml`.
  ```bash
  cd deploy/docker
  docker-compose up -d
  ```

For more detailed deployment instructions, including resource requirements and security hardening, please see the [**Deployment Guide**](DEPLOYMENT.md).

## Future Vision: Custom Components

The project includes source code for several experimental Go components that represent the future vision of this project:

- **`postgresqlquery`**: An advanced receiver with ASH sampling.
- **`adaptivesampler`**: An intelligent, stateful sampler.
- **`circuitbreaker`**: A processor for isolating database failures.
- **`planattributeextractor`**: A processor for deep query plan analysis.

These components are **not** part of the v1.0.0 production release but are under active development. For more details, see the [**Evolution Roadmap**](EVOLUTION.md).

## Documentation

- **[ARCHITECTURE.md](ARCHITECTURE.md)**: Technical design and data flow.
- **[DEPLOYMENT.md](DEPLOYMENT.md)**: Production deployment guide.
- **[CONFIGURATION.md](CONFIGURATION.md)**: Detailed configuration reference.
- **[OPERATIONS.md](OPERATIONS.md)**: Daily operations and monitoring.
- **[LIMITATIONS.md](LIMITATIONS.md)**: Known limitations and workarounds.
- **[CHANGELOG.md](CHANGELOG.md)**: Release history and fixes.

## Support & Community

- **Issues**: [GitHub Issues](https://github.com/newrelic/database-intelligence-mvp/issues)
- **Discussions**: [GitHub Discussions](https://github.com/newrelic/database-intelligence-mvp/discussions)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
