# CLAUDE.md - Database Intelligence MySQL Monorepo

This file provides guidance to Claude Code when working with this monorepo project.

## Project Overview

This is a modular MySQL monitoring system built as a monorepo with 8 independent, composable modules based on OpenTelemetry Collector.

## Repository Structure

```
database-intelligence-monorepo/
├── modules/              # Independent monitoring modules
│   ├── core-metrics/     # Basic MySQL metrics (port 8081)
│   ├── sql-intelligence/ # Query analysis (port 8082)
│   ├── wait-profiler/    # Wait event profiling (port 8083)
│   ├── anomaly-detector/ # Anomaly detection (port 8084)
│   ├── business-impact/  # Business scoring (port 8085)
│   ├── replication-monitor/ # Replication health (port 8086)
│   ├── performance-advisor/ # Recommendations (port 8087)
│   └── resource-monitor/ # System resources (port 8088)
├── shared/               # Shared resources
├── integration/          # Integration testing
└── Makefile             # Root orchestration
```

## Key Commands

### Quick Start
```bash
# Build all modules in parallel
make build

# Run specific modules
make run-core-metrics
make run-sql-intelligence

# Run module groups
make run-core         # core-metrics + resource-monitor
make run-intelligence # sql-intelligence + wait-profiler + anomaly-detector
make run-business     # business-impact + performance-advisor

# Run all modules
make run-all

# Check health
make health

# View help
make help
```

### Module-Specific Operations
```bash
# Work with individual modules
make build-<module>
make test-<module>
make run-<module>
make stop-<module>
make logs-<module>
make clean-<module>

# Example
make run-wait-profiler
make logs-anomaly-detector
```

## Module Details

### Core Metrics (8081)
- Basic MySQL metrics collection
- Connection, thread, operation metrics
- Foundation for other modules

### SQL Intelligence (8082)
- Query performance analysis
- Slow query detection
- Index usage analysis
- Table I/O statistics

### Wait Profiler (8083)
- Wait event analysis
- Mutex contention tracking
- I/O wait profiling
- Lock wait monitoring

### Anomaly Detector (8084)
- Statistical anomaly detection
- Consumes metrics from other modules
- Z-score based detection
- Alert generation

### Business Impact (8085)
- Business value scoring
- Revenue impact detection
- SLA assessment
- Query categorization

### Replication Monitor (8086)
- Master-slave replication health
- Lag monitoring
- GTID tracking
- Thread status

### Performance Advisor (8087)
- Automated recommendations
- Missing index detection
- Connection pool sizing
- Cache optimization advice

### Resource Monitor (8088)
- Host metrics collection
- CPU, memory, disk, network
- MySQL process monitoring

## Architecture Principles

### Module Independence
- Each module has its own docker-compose.yaml
- Can run standalone or integrated
- Own test suite and configuration
- Communicates via Prometheus metrics or OTLP

### Communication Patterns
1. **Prometheus Federation**: Modules expose metrics that others can scrape
2. **OTLP Forward**: Modules can send metrics to others via OTLP
3. **File Export**: Shared metrics via file system (for debugging)

### Development Workflow
1. Choose module to work on
2. Use module's Makefile
3. Test in isolation
4. Test with dependencies
5. Run integration tests

## Testing

### Module Testing
```bash
# Test individual module
cd modules/core-metrics
make test

# Quick test from root
make quick-test-core-metrics

# Test all modules in parallel
make test
```

### Integration Testing
```bash
# Run full integration suite
make integration

# Performance testing
make perf-test
```

## Environment Variables

### Common Variables
- `MYSQL_ENDPOINT`: MySQL connection (default: mysql-test:3306)
- `MYSQL_USER`: MySQL username (default: root)
- `MYSQL_PASSWORD`: MySQL password (default: test)
- `EXPORT_PORT`: Module's metrics port

### Module-Specific
- `METRICS_ENDPOINT`: For modules consuming from others
- `ANOMALY_THRESHOLD_*`: Anomaly detection thresholds
- `ENABLE_*`: Feature flags

## Troubleshooting

### Module Not Starting
```bash
# Check logs
make logs-<module>

# Verify dependencies
cd modules/anomaly-detector
make check-dependencies

# Check port conflicts
netstat -an | grep <port>
```

### No Metrics
```bash
# Check module health
curl http://localhost:<port>/metrics

# Verify configuration
docker-compose exec <module> cat /etc/otel/collector.yaml
```

### Integration Issues
```bash
# Check network connectivity
docker network ls
docker network inspect database-intelligence-monorepo_db-intelligence

# Verify service discovery
docker-compose exec <module> nslookup <other-module>
```

## Important Notes

1. **Parallel Operations**: The root Makefile supports parallel builds/tests with `-j` flag
2. **Module Ports**: Each module has a dedicated port (8081-8088)
3. **Health Checks**: All modules expose health endpoints on port 13133
4. **Metrics Format**: Prometheus exposition format on module ports
5. **Configuration**: Each module's config is in `modules/<name>/config/collector.yaml`

## CI/CD Helpers

```bash
# CI build
make ci-build

# CI test
make ci-test

# CI integration
make ci-integration
```

## Docker Management

```bash
# Clean up resources
make docker-clean

# Remove all module containers
make clean
```