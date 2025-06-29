# New Relic Database Intelligence MVP

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-enabled-orange)](https://opentelemetry.io)
[![Production Ready](https://img.shields.io/badge/Production-Ready-green)](DEPLOYMENT.md)

## What This Is

A comprehensive database monitoring solution with two deployment options:

*   **Standard Mode**: Production-ready OpenTelemetry Collector using proven components for maximum stability.
*   **Experimental Mode**: Advanced monitoring with Active Session History (ASH), adaptive sampling, and circuit breaker protection using custom Go components.

### Key Features

*   🛡️ **Production-Safe**: Connects to read-replicas only, with built-in timeouts and resource limits.
*   🚀 **High Availability**: Natively supports HA deployments in Kubernetes for scalability and resilience.
*   🔒 **Security First**: Includes PII sanitization, credential management via secrets, and network policies.
*   📊 **Observable**: Provides comprehensive metrics, pre-built monitoring dashboards, and Prometheus alerting rules.
*   🎯 **Easy Setup**: A one-command `quickstart.sh` script provides an interactive setup and validation experience.
*   🔗 **Entity Correlation**: Automatically creates database entities in New Relic for seamless correlation.

### Current Capabilities

| Feature | PostgreSQL | MySQL | Status |
|---|---|---|---|
| Query Performance Metrics | ✅ | ✅ | Production |
| Query Metadata Collection | ✅ | ✅ | Production |
| Execution Plans | ❌ | ❌ | Future Enhancement |
| PII Sanitization | ✅ | ✅ | Production |
| High Availability | ✅ | ✅ | Production |

## Architecture

The v1.0.0 production architecture uses a standard OpenTelemetry Collector deployed to Kubernetes. It uses a leader election mechanism for safe horizontal scaling.

For a detailed explanation of the architecture, see [**Technical Architecture**](ARCHITECTURE.md).

## Quick Start

Get up and running in minutes with the interactive quickstart script.

### Prerequisites

*   **PostgreSQL**: `pg_stat_statements` extension enabled, read-only user with access to `pg_stat_statements`.
*   **MySQL**: Performance Schema enabled, read-only user.
*   **Docker and Docker Compose**.

### One-Command Setup

```bash
# Clone the repository
git clone https://github.com/newrelic/database-intelligence-mvp
cd database-intelligence-mvp
./quickstart.sh all
```

This guides you through setup, validates connections, and starts the collector in Docker.

## Production Deployment

The recommended deployment method for production is the Kubernetes HA (High Availability) configuration.

### Deployment Options

*   **Kubernetes HA (Recommended)**: Scalable and resilient deployment. See `deploy/k8s/ha-deployment.yaml`.
    ```bash
    kubectl apply -f deploy/k8s/ha-deployment.yaml
    ```
*   **Docker Compose**: Ideal for local development and testing. See `deploy/docker/docker-compose.yaml`.
    ```bash
    cd deploy/docker
    docker-compose up -d
    ```

For more detailed deployment instructions, see [**Deployment Guide**](DEPLOYMENT.md).

## Deployment Options

### Standard Mode (Default)

Ready-to-deploy configuration using standard OpenTelemetry components. No build required.

```bash
./quickstart.sh all
```

### Experimental Mode

Advanced features including ASH, adaptive sampling, circuit breaker, and multi-database support.

```bash
./quickstart.sh --experimental all
```

See [**Deployment Options**](DEPLOYMENT-OPTIONS.md) for detailed comparison.

## Documentation

*   [DEPLOYMENT-OPTIONS.md](DEPLOYMENT-OPTIONS.md): Choose between Standard and Experimental modes.
*   [IMPLEMENTATION-GUIDE.md](IMPLEMENTATION-GUIDE.md): Technical details of both implementations.
*   [EXPERIMENTAL-FEATURES.md](EXPERIMENTAL-FEATURES.md): Guide to advanced capabilities.
*   [ARCHITECTURE.md](ARCHITECTURE.md): Technical design and data flow.
*   [CONFIGURATION.md](CONFIGURATION.md): Detailed configuration reference.
*   [OPERATIONS.md](OPERATIONS.md): Daily operations and monitoring.
*   [TROUBLESHOOTING-GUIDE.md](TROUBLESHOOTING-GUIDE.md): Common issues and solutions.

## Support & Community

*   **Issues**: [GitHub Issues](https://github.com/newrelic/database-intelligence-mvp/issues).
*   **Discussions**: [GitHub Discussions](https://github.com/newrelic/database-intelligence-mvp/discussions).

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE.md) file for details.
