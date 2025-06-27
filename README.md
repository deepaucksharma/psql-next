# PostgreSQL OpenTelemetry Collector

A high-performance PostgreSQL monitoring collector that exports metrics in OpenTelemetry (OTLP) format.

## Quick Start

```bash
# Setup
cp .env.example .env
# Edit .env with your OTLP endpoint

# Build and run
cargo build --release
./target/release/postgres-otel-collector -c config.toml

# Or use Docker
docker compose up
```

## Documentation

For complete documentation, see [DOCUMENTATION.md](DOCUMENTATION.md)

### Key Sections:
- [Architecture & Design](DOCUMENTATION.md#architecture)
- [Installation & Configuration](DOCUMENTATION.md#installation--configuration)
- [Deployment Options](DOCUMENTATION.md#deployment)
- [Metrics Reference](DOCUMENTATION.md#metrics-reference)
- [Troubleshooting](DOCUMENTATION.md#troubleshooting)

## Features

- **OpenTelemetry Native**: Exports metrics in OTLP format
- **Comprehensive Monitoring**: Slow queries, wait events, blocking sessions, execution plans
- **Production Ready**: Health checks, Kubernetes support, Docker
- **Query Sanitization**: Automatic PII detection and removal
- **Multi-Instance Support**: Monitor multiple PostgreSQL instances
- **Extended Metrics**: Active Session History (ASH), performance insights

## Requirements

- PostgreSQL 12+ with `pg_stat_statements`
- OpenTelemetry-compatible backend (Jaeger, Prometheus, etc.)
- Rust 1.70+ (for building from source)

## License

Apache License 2.0