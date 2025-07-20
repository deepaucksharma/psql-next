# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a modular MySQL monitoring system built as a monorepo with 11 independent, composable modules based on OpenTelemetry Collector. The system provides comprehensive database monitoring including metrics collection, query analysis, wait profiling, anomaly detection, business impact scoring, replication monitoring, performance advising, and resource tracking.

## Repository Structure

```
database-intelligence-monorepo/
├── modules/                    # Independent monitoring modules
│   ├── core-metrics/          # Basic MySQL metrics (port 8081)
│   ├── sql-intelligence/      # Query analysis (port 8082)
│   ├── wait-profiler/         # Wait event profiling (port 8083)
│   ├── anomaly-detector/      # Anomaly detection (port 8084)
│   ├── business-impact/       # Business scoring (port 8085)
│   ├── replication-monitor/   # Replication health (port 8086)
│   ├── performance-advisor/   # Recommendations (port 8087)
│   ├── resource-monitor/      # System resources (port 8088)
│   ├── alert-manager/         # Alert aggregation (port 8089)
│   ├── canary-tester/         # Synthetic monitoring (port 8090)
│   └── cross-signal-correlator/ # Trace/log/metric correlation (port 8099)
├── shared/                    # Shared resources
│   ├── validation/           # Health check scripts (validation-only)
│   ├── config/              # Shared configurations
│   └── newrelic/            # New Relic dashboards and configs
├── integration/             # Integration testing
└── Makefile                # Root orchestration
```

## Key Commands

### Build and Run
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

# Run with enhanced configurations
make run-enhanced
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

### Testing and Validation
```bash
# Test all modules in parallel
make test

# Test individual module
make quick-test-core-metrics

# Validate modules (health checks are validation-only)
./shared/validation/health-check-all.sh

# Run integration tests
make integration
```

## Module Port Assignments

- Core Metrics: 8081 (Prometheus metrics)
- SQL Intelligence: 8082 (Prometheus metrics)
- Wait Profiler: 8083 (Prometheus metrics)
- Anomaly Detector: 8084 (Prometheus metrics)
- Business Impact: 8085 (Prometheus metrics)
- Replication Monitor: 8086 (Prometheus metrics)
- Performance Advisor: 8087 (Prometheus metrics)
- Resource Monitor: 8088 (Prometheus metrics)
- Alert Manager: 8089 (Prometheus metrics)
- Canary Tester: 8090 (Prometheus metrics)
- Cross-Signal Correlator: 8099 (Prometheus metrics)

## Architecture and Communication

### Module Independence
- Each module has its own docker-compose.yaml
- Can run standalone or integrated
- Own test suite and configuration
- Modules communicate via Prometheus federation or OTLP

### Communication Patterns
1. **Prometheus Federation**: Modules expose metrics on their designated ports that others can scrape
2. **OTLP Forward**: Modules can send metrics to others via OTLP (ports 4317/4318)
3. **File Export**: Shared metrics via file system (for debugging)

### Configuration Files
- Each module's main config: `modules/<name>/config/collector.yaml`
- Enhanced configs available: `collector-enhanced.yaml`
- Enterprise configs: `collector-enterprise.yaml`
- Functional configs (fixed versions): `collector-functional.yaml`

## Environment Configuration

### Key Environment Variables
```bash
# MySQL Connection
MYSQL_ENDPOINT=mysql:3306
MYSQL_USER=root
MYSQL_PASSWORD=test

# New Relic Integration
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
NEW_RELIC_LICENSE_KEY=<your-key>
NEW_RELIC_ACCOUNT_ID=<your-account>

# Module Configuration
ENVIRONMENT=production
CLUSTER_NAME=database-intelligence-cluster

# Module Federation Endpoints
CORE_METRICS_ENDPOINT=core-metrics:8081
SQL_INTELLIGENCE_ENDPOINT=sql-intelligence:8082
WAIT_PROFILER_ENDPOINT=wait-profiler:8083
```

### Service Endpoints
- Internal communication uses Docker service names
- External access uses localhost with module ports
- Federation endpoints defined in `shared/config/service-endpoints.env`

## OpenTelemetry Collector Patterns

### Standard Pipeline Structure
```yaml
service:
  pipelines:
    metrics:
      receivers: [mysql, prometheus, otlp]
      processors: [
        memory_limiter,    # Always first
        batch,
        attributes,
        resource,
        transform/<specific>,
        attributes/newrelic,
        attributes/entity_synthesis
      ]
      exporters: [otlphttp/newrelic, prometheus, debug]
```

### Common Processors
- `memory_limiter`: Resource protection (always first)
- `batch`: Optimize throughput
- `attributes`: Add module metadata
- `resource`: Service identification
- `transform/*`: Module-specific logic
- `attributes/newrelic`: New Relic integration
- `attributes/entity_synthesis`: Entity GUID generation

## ⚠️ Health Check Policy

**CRITICAL**: Health check endpoints (port 13133) are intentionally REMOVED from production.

- **Do NOT** add health_check extension to configs
- **Do NOT** expose port 13133 in Docker
- **Do NOT** add health targets to Makefiles
- **For validation**: Use `./shared/validation/health-check-all.sh`
- **For monitoring**: Use Prometheus metrics endpoints (8081-8099)

All collector.yaml files contain WARNING comments about this policy.

## Development Workflow

1. Choose module to work on: `cd modules/<module>`
2. Modify configuration: `config/collector.yaml`
3. Test locally: `make test`
4. Run in isolation: `docker-compose up`
5. Test with dependencies: Use root Makefile
6. Run integration tests: `make integration`

## Troubleshooting

### Check Module Status
```bash
# View logs
make logs-<module>

# Check running containers
docker ps | grep <module>

# Verify metrics endpoint
curl http://localhost:<port>/metrics
```

### Common Issues
- Port conflicts: Check netstat for port usage
- No metrics: Verify MySQL connection and credentials
- Module not starting: Check Docker logs and config syntax
- Federation issues: Verify endpoint connectivity

## New Relic Integration

All modules include:
- OTLP HTTP exporter configuration
- Entity synthesis for proper entity mapping
- Instrumentation metadata
- Environment and cluster tagging

Dashboard templates available in `shared/newrelic/dashboards/`

## CI/CD Support

```bash
make ci-build      # CI-friendly build
make ci-test       # CI-friendly test
make ci-integration # CI integration tests
```

## Module-Specific Notes

### anomaly-detector
- Uses z-score based detection
- Federates metrics from all other modules
- Configurable baselines via environment variables

### business-impact
- Configuration-based scoring (business-mappings.yaml)
- No longer uses regex matching
- Single pipeline architecture

### replication-monitor
- Requires primary/replica endpoints
- Monitors GTID and traditional replication
- Converts string states to numeric values

### wait-profiler
- Comprehensive wait event collection
- Thread-level analysis
- Lock wait details with blocking info