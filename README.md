# PostgreSQL Unified Collector

A high-performance PostgreSQL monitoring collector that supports both New Relic Infrastructure (NRI) and OpenTelemetry (OTLP) output formats.

## Quick Start

```bash
# Setup
cp .env.example .env
# Edit .env with your credentials

# Build and run
./run.sh build
./run.sh run

# Verify metrics
./verify-nrdb.sh --all
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

- **Dual Output Support**: NRI and OTLP protocols
- **Comprehensive Monitoring**: Slow queries, wait events, blocking sessions
- **Production Ready**: Health checks, Kubernetes support, Docker
- **Query Sanitization**: Automatic PII detection and removal
- **Multi-Instance Support**: Monitor multiple PostgreSQL instances

## Requirements

- PostgreSQL 12+ with `pg_stat_statements`
- New Relic account
- Rust 1.70+ (for building from source)

## License

Apache License 2.0