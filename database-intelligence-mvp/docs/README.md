# Database Intelligence Collector Documentation

## Project Status: Production Ready (Single-Instance)

The Database Intelligence Collector is an OpenTelemetry-based monitoring solution for PostgreSQL and MySQL databases. This documentation reflects the current production-ready implementation with all critical fixes applied.

## 📚 Documentation Structure

### 1. Getting Started
- **[Quick Start Guide](./QUICK_START.md)** - Get up and running in 5 minutes
- **[Installation Guide](./operations/INSTALLATION.md)** - Detailed installation procedures
- **[Configuration Guide](./CONFIGURATION.md)** - Complete configuration reference

### 2. Architecture & Design
- **[Architecture Overview](./ARCHITECTURE.md)** - System design and components
- **[Technical Implementation](./architecture/IMPLEMENTATION.md)** - Deep dive into code structure
- **[Custom Processors](./architecture/PROCESSORS.md)** - Details on our 4 custom processors

### 3. Operations
- **[Operations Runbook](./operations/RUNBOOK.md)** - Production procedures and troubleshooting
- **[Deployment Guide](./operations/DEPLOYMENT.md)** - Deployment options and procedures
- **[Monitoring Guide](./operations/MONITORING.md)** - Health checks and metrics
- **[Dashboard Guide](./operations/DASHBOARD_GUIDE.md)** - Dashboard setup and configuration
- **[Migration Guide](./operations/MIGRATION_GUIDE.md)** - Migration from other collectors

### 4. Development
- **[Development Guide](./development/GUIDE.md)** - Contributing and local development
- **[E2E Testing Guide](./E2E_TESTING_COMPLETE.md)** - Comprehensive end-to-end testing
- **[API Reference](./development/API.md)** - Internal APIs and interfaces
- **[Taskfile Usage](./development/TASKFILE_USAGE.md)** - Task automation and build scripts

### 5. Project Information
- **[Project Status](./PROJECT_STATUS.md)** - Current implementation status and roadmap
- **[Change Log](./CHANGELOG.md)** - Version history and updates
- **[Known Issues](./KNOWN_ISSUES.md)** - Current limitations and workarounds
- **[Troubleshooting Guide](./TROUBLESHOOTING.md)** - Common issues and solutions

## 🚀 Key Features

### Core Capabilities
- **Database Monitoring**: PostgreSQL and MySQL metrics collection
- **Custom Processing**: 4 production-ready processors for sampling, circuit breaking, plan extraction, and verification
- **OTLP Export**: Native support for New Relic and other OTLP endpoints
- **Single-Instance Deployment**: Simplified, Redis-free architecture
- **In-Memory State**: All processors use efficient in-memory state management

### Production Enhancements (June 2025)
1. **Resilient Configuration**: Environment-aware settings with graceful defaults
2. **Comprehensive Monitoring**: Self-telemetry and health endpoints
3. **Operational Safety**: Rate limiting and circuit breakers
4. **Performance Optimization**: Caching and efficient processing
5. **Complete Runbooks**: Operational procedures and troubleshooting

## 🏗️ Architecture Summary

```
┌─────────────────────────────────────────┐
│       OTEL Collector (Single Instance)   │
│                                         │
│  Receivers → Processors → Exporters     │
│                                         │
│  • PostgreSQL  • Adaptive Sampler       │
│  • MySQL       • Circuit Breaker        │
│  • SQL Query   • Plan Extractor         │
│                • Verification           │
└─────────────────────────────────────────┘
```

## 📊 Current Status

| Component | Status | Details |
|-----------|--------|---------|
| Core Collector | ✅ Production Ready | OTEL v0.96.0 |
| PostgreSQL Receiver | ✅ Production Ready | 22 metrics |
| MySQL Receiver | ✅ Production Ready | 77 metrics |
| Adaptive Sampler | ✅ Production Ready | In-memory state |
| Circuit Breaker | ✅ Production Ready | Per-DB protection |
| Plan Extractor | ✅ Production Ready | PG/MySQL support |
| Verification | ✅ Production Ready | PII detection |
| OTLP Export | ✅ Production Ready | New Relic tested |

## 🔧 Quick Configuration

```yaml
# Minimal production configuration
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases:
      - ${POSTGRES_DB}
    collection_interval: 30s

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
  
  adaptive_sampler:
    in_memory_only: true  # Production setting
    rules:
      - name: slow_queries
        sample_rate: 1.0
        conditions:
          - attribute: duration_ms
            operator: gt
            value: 1000

exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, adaptive_sampler]
      exporters: [otlp/newrelic]
```

## 📈 Performance Characteristics

| Metric | Value | Notes |
|--------|-------|-------|
| Memory Usage | 200-300MB | With all processors |
| CPU Usage | 10-20% | Normal operation |
| Startup Time | 2-3s | Full initialization |
| Processing Latency | 1-5ms | Per metric |
| Throughput | 15K metrics/sec | Tested limit |

## 🛡️ Security & Compliance

- **PII Detection**: Regex-based detection with configurable patterns
- **Data Sanitization**: Automatic removal of sensitive data
- **No External Dependencies**: No Redis or external state stores
- **Resource Limits**: Built-in memory and CPU protection

## 📞 Support & Resources

- **Issues**: [GitHub Issues](https://github.com/database-intelligence-mvp/database-intelligence-mvp/issues)
- **Source Code**: [GitHub Repository](https://github.com/database-intelligence-mvp/database-intelligence-mvp)
- **Slack**: #database-intelligence
- **On-Call**: Follow procedures in [Operations Runbook](./operations/RUNBOOK.md)

## 📁 Documentation Organization

After recent cleanup (June 30, 2025), the documentation is now organized as follows:

**Top-Level Files (9 core documents)**:
- Core guides: README, QUICK_START, CONFIGURATION, TROUBLESHOOTING
- Architecture: ARCHITECTURE
- Testing: E2E_TESTING_COMPLETE  
- Project info: PROJECT_STATUS, CHANGELOG, KNOWN_ISSUES

**Subdirectories**:
- `architecture/` - Technical implementation details
- `operations/` - Deployment, monitoring, maintenance, dashboards
- `development/` - Contributing, testing, APIs, build tools
- `strategic-analysis/` - Executive summaries and roadmaps

## 🔄 Migration Notes

For teams migrating from OHI or other collectors:
1. Review [Migration Guide](./operations/MIGRATION_GUIDE.md)
2. Use provided configuration templates
3. Test with minimal configuration first
4. Enable processors incrementally

---

**Last Updated**: June 30, 2025  
**Version**: 1.0.0-production  
**Status**: Production Ready (Single-Instance)