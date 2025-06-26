# PostgreSQL Unified Collector Documentation

Welcome to the comprehensive documentation for the PostgreSQL Unified Collector - a next-generation monitoring solution that combines the reliability of New Relic's On-Host Integration (OHI) with modern observability standards.

## Documentation Structure

### 📋 [01 - Architecture Overview](01-architecture-overview.md)
Understand the system design, core components, and how the unified collector works under the hood.
- System architecture and data flow
- Core components and their responsibilities
- Design principles and technology stack
- Performance characteristics

### 🛠️ [02 - Implementation Guide](02-implementation-guide.md)
Learn how to build, extend, and customize the collector for your needs.
- Building from source
- Adding new metrics
- Extending functionality
- Testing and optimization strategies

### 🚀 [03 - Deployment & Operations](03-deployment-operations.md)
Deploy and operate the collector in production environments.
- Installation methods (binary, Docker, Kubernetes)
- Configuration reference
- Cloud provider deployments
- Monitoring and troubleshooting

### 📊 [04 - Metrics Reference](04-metrics-reference.md)
Complete reference for all collected metrics and their usage.
- OHI-compatible metrics
- Extended metrics (histograms, ASH, kernel)
- Query examples (NRQL and PromQL)
- Dashboard and alerting patterns

### 🔄 [05 - Migration Guide](05-migration-guide.md)
Seamlessly migrate from nri-postgresql to the unified collector.
- Pre-migration assessment
- Migration strategies
- Step-by-step procedures
- Validation and rollback

## Quick Start

### For New Users
1. Start with the [Architecture Overview](01-architecture-overview.md) to understand the system
2. Follow the [Deployment Guide](03-deployment-operations.md) to install the collector
3. Refer to the [Metrics Reference](04-metrics-reference.md) to build dashboards

### For Existing nri-postgresql Users
1. Review the [Migration Guide](05-migration-guide.md) for upgrade paths
2. Check the [Metrics Reference](04-metrics-reference.md) for new capabilities
3. Follow the migration checklist for zero-downtime upgrade

## Key Features

### 100% OHI Compatibility
- Drop-in replacement for nri-postgresql
- All existing dashboards and alerts continue working
- No configuration changes required

### Extended Capabilities
- **Query Latency Histograms**: P50, P95, P99 percentiles
- **Active Session History (ASH)**: 1-second resolution sampling
- **Execution Plans**: Automatic plan capture and regression detection
- **Kernel Metrics**: CPU/IO split via eBPF
- **Wait Event Analysis**: Comprehensive wait event tracking

### Modern Architecture
- **Single Binary**: Multiple deployment modes in one executable
- **Dual Export**: Simultaneous NRI and OpenTelemetry output
- **Cloud-Native**: First-class Kubernetes and cloud provider support
- **Performance Optimized**: <1% overhead with adaptive sampling

## Deployment Options

### Infrastructure Agent Integration
```yaml
integrations:
  - name: nri-postgresql
    env:
      HOSTNAME: localhost
      PORT: 5432
      USERNAME: monitoring
      PASSWORD: ${POSTGRES_PASSWORD}
```

### Kubernetes
```bash
helm install postgres-collector newrelic/postgres-unified-collector
```

### Docker
```bash
docker run -d newrelic/postgres-unified-collector:latest
```

## Configuration Examples

### Basic Configuration
```toml
[collector]
mode = "hybrid"
collection_interval_secs = 60

[postgres]
host = "localhost"
port = 5432
databases = ["postgres", "app"]

[export.nri]
enabled = true

[export.otlp]
enabled = true
endpoint = "https://otlp.nr-data.net:4317"
```

### Extended Features
```toml
[features]
enable_extended_metrics = true
enable_ash = true
enable_ebpf = true
enable_plan_collection = true
```

## Support and Resources

### Getting Help
- **GitHub Issues**: [Report bugs or request features](https://github.com/newrelic/postgres-unified-collector/issues)
- **Community Forum**: [New Relic Explorers Hub](https://discuss.newrelic.com)
- **Documentation**: This comprehensive guide

### Contributing
We welcome contributions! Please see our [Contributing Guide](https://github.com/newrelic/postgres-unified-collector/blob/main/CONTRIBUTING.md) for details.

### License
The PostgreSQL Unified Collector is licensed under the Apache 2.0 License. See [LICENSE](https://github.com/newrelic/postgres-unified-collector/blob/main/LICENSE) for details.

## Version History

### v1.0.0 (Current)
- Initial release with full OHI compatibility
- Extended metrics support
- Dual export capability
- Cloud-native deployment options

### Roadmap
- pg_querylens extension integration
- Advanced plan analysis
- Machine learning insights
- Multi-cluster support

---

*This documentation is continuously updated. For the latest version, visit the [GitHub repository](https://github.com/newrelic/postgres-unified-collector).*