# Changelog

All notable changes to the PostgreSQL Unified Collector project are documented here.

## [1.0.0] - 2025-06-27

### Added
- **Dual Mode Operation**: True independent NRI and OTLP outputs, each sending complete metrics
- **PostgreSQL Metrics Collection**:
  - Slow queries from pg_stat_statements
  - Wait events and blocking sessions
  - Individual query monitoring
  - Execution plan collection
- **PgBouncer Support**: Monitor connection pooler statistics
- **Multi-Instance Monitoring**: Single collector can monitor multiple PostgreSQL instances
- **Query Sanitization**: Smart PII detection and removal
- **Active Session History (ASH)**: Oracle-style session sampling with memory bounds
- **Memory Management**: Bounded collections to prevent OOM
- **Health Monitoring**: HTTP endpoints for health, readiness, and metrics
- **Kubernetes Support**:
  - Raw YAML deployments
  - Helm chart with extensive configuration options
  - Support for Deployment and DaemonSet modes
- **Docker Support**: Multi-stage optimized Dockerfile
- **Configuration**: Flexible TOML configuration with environment variable overrides

### Architecture
- **Collection Engine**: Unified metrics collection with capability detection
- **Output Adapters**: 
  - NRI: Outputs JSON to stdout for Infrastructure agent
  - OTLP: Sends metrics via HTTP to OpenTelemetry collectors
- **Extension Support**: Automatic detection of pg_stat_statements, pg_wait_sampling, pg_stat_monitor
- **Error Isolation**: Failures in one metric type don't affect others

### Testing
- Comprehensive unit tests
- Integration test framework
- End-to-end verification scripts
- Load testing capabilities

### Documentation
- README with quick start guide
- Implementation details in docs/IMPLEMENTATION.md
- Deployment guide in docs/DEPLOYMENT.md
- Example configurations in examples/
- Helm chart with multiple deployment scenarios

### Known Issues
- OTLP HTTP client currently doesn't support HTTPS (requires TLS implementation)
- Some eBPF features are compile-time optional

### Migration
- Drop-in replacement for nri-postgresql
- 100% OHI compatibility maintained
- Additional metrics available through extended features