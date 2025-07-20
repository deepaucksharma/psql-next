# Health Check All Modules

## ⚠️ Health Check Policy

**IMPORTANT**: Health check endpoints (port 13133) have been intentionally removed from production code.

- **Production**: Health check endpoints are NOT available on port 13133
- **Validation**: Use this script (`shared/validation/health-check-all.sh`) for testing purposes only
- **Monitoring**: Use production metrics endpoints (ports 8081-8088) for monitoring
- **Do NOT**: Add health check endpoints back to production configs
- **Do NOT**: Expose port 13133 in Docker configurations
- **Do NOT**: Add health check targets to Makefiles

This directory contains the consolidated health check script that replaces all individual module health check targets for **validation purposes only**.

## Usage

Run the comprehensive health check for all database intelligence modules:

```bash
./shared/validation/health-check-all.sh
```

## What it checks

The script performs health checks for all database intelligence modules:

- **Core Metrics** (Port 8081) - Basic database metrics collection
- **SQL Intelligence** (Port 8082) - SQL query analysis and intelligence
- **Wait Profiler** (Port 8083) - Database wait event profiling
- **Anomaly Detector** (Port 8084) - Anomaly detection and alerting
- **Business Impact** (Port 8085) - Business impact analysis
- **Replication Monitor** (Port 8086) - MySQL replication monitoring
- **Performance Advisor** (Port 8087) - Performance recommendations
- **Resource Monitor** (Port 8088-8091) - System resource monitoring
- **Alert Manager** (Port 9091, Health 13134) - Alert management
- **Canary Tester** (Port 8090) - Canary testing framework
- **Cross-Signal Correlator** (Port 8892, Health 13137) - Cross-signal correlation

## Health Check Types

For each module, the script checks:

1. **Metrics Endpoints** - Production Prometheus metrics availability (ports 8081-8088)
2. **Expected Data Patterns** - Module-specific metric patterns
3. **Dependencies** - Cross-module dependencies where applicable
4. **Container Status** - Docker container health where applicable
5. **Note**: Traditional health endpoints (port 13133) are NOT used in production

## Exit Codes

- `0` - All modules are healthy
- `1` - One or more modules have health issues

## Replaced Makefile Targets

This script consolidates the following health check targets that were removed from individual module Makefiles:

- `business-impact/Makefile` - `health:` target
- `performance-advisor/Makefile` - `health:` target  
- `alert-manager/Makefile` - `health-check:` target
- `cross-signal-correlator/Makefile` - `health:` target
- `replication-monitor/Makefile` - `health:` target
- `canary-tester/Makefile` - `health:` target

## Integration

This script is designed to be used in:

- **CI/CD pipelines** for health validation (testing only)
- **Manual testing and debugging** (development environments)
- **Development environment validation** (pre-deployment testing)
- **NOT for production monitoring** - use metrics endpoints instead

## Production Monitoring

For production monitoring, use the dedicated metrics endpoints:

```bash
# Production metrics endpoints (DO use these)
curl http://localhost:8081/metrics  # core-metrics
curl http://localhost:8082/metrics  # sql-intelligence  
curl http://localhost:8083/metrics  # wait-profiler
curl http://localhost:8084/metrics  # anomaly-detector
curl http://localhost:8085/metrics  # business-impact
curl http://localhost:8086/metrics  # replication-monitor
curl http://localhost:8087/metrics  # performance-advisor
curl http://localhost:8088/metrics  # resource-monitor

# Traditional health endpoints (DO NOT use in production)
# Port 13133 health endpoints are intentionally disabled
```