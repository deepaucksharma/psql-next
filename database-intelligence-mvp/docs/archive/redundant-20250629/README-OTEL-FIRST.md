# Database Intelligence MVP - OTEL-First Architecture

[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-First-orange)](https://opentelemetry.io)
[![Production Ready](https://img.shields.io/badge/Production-Ready-green)](docs/DEPLOYMENT.md)
[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/database-intelligence-mvp/graphs/commit-activity)

## 🎯 What This Is

A **production-ready database monitoring solution** that maximizes standard OpenTelemetry components and only implements custom code where OTEL has genuine gaps.

### Key Principles

- **90% Standard OTEL**: Use community-maintained components wherever possible
- **10% Custom Code**: Only for true OTEL gaps (adaptive sampling, circuit breaking)
- **Zero Lock-in**: Works with any OTEL-compatible backend
- **Simple Configuration**: Just 3 config files instead of 15+
- **Easy Maintenance**: Automatic updates with OTEL releases

## 🚀 Quick Start

### 1. Basic Setup (5 minutes)

```bash
# Clone and configure
git clone https://github.com/database-intelligence-mvp
cd database-intelligence-mvp

# Set your credentials
export NEW_RELIC_LICENSE_KEY=your-key
export PG_HOST=localhost
export PG_USER=postgres
export PG_PASSWORD=postgres

# Run with standard OTEL collector
docker run -v $(pwd)/config/collector.yaml:/etc/otelcol/config.yaml \
  -e NEW_RELIC_LICENSE_KEY=$NEW_RELIC_LICENSE_KEY \
  -e PG_HOST=$PG_HOST \
  -e PG_USER=$PG_USER \
  -e PG_PASSWORD=$PG_PASSWORD \
  otel/opentelemetry-collector-contrib:latest
```

### 2. What You Get

- ✅ PostgreSQL & MySQL metrics (cpu, memory, connections, locks, etc.)
- ✅ Query performance statistics (duration, frequency, errors)
- ✅ PII sanitization (emails, credit cards, SSNs)
- ✅ Automatic New Relic entity creation
- ✅ Production-ready with health checks and resource limits

## 📊 Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  PostgreSQL │────▶│                  │────▶│ New Relic   │
├─────────────┤     │ OTEL Collector   │     └─────────────┘
│    MySQL    │────▶│                  │
└─────────────┘     │ • postgresql     │
                    │ • mysql          │
                    │ • sqlquery       │
                    │ • transform      │
                    │ • batch          │
                    └──────────────────┘
```

## 🔧 Configuration

We've simplified from 15+ config files to just 3:

### 1. Production (`config/collector.yaml`)
```yaml
receivers:
  postgresql:
    endpoint: ${env:PG_HOST}:5432
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    
processors:
  batch:
    timeout: 10s
    
exporters:
  otlp:
    endpoint: https://otlp.nr-data.net:4317
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
```

### 2. Development (`config/collector-dev.yaml`)
Includes debug output, faster intervals, and local file export.

### 3. Minimal Example (`config/examples/minimal.yaml`)
Simplest possible configuration for getting started.

## 🎨 Key Features

### Standard OTEL Components (90%)

| Component | Purpose | OTEL Component |
|-----------|---------|----------------|
| Database Metrics | CPU, memory, connections | `postgresql` receiver |
| Query Stats | Performance metrics | `sqlquery` receiver |
| PII Sanitization | Remove sensitive data | `transform` processor |
| Batching | Optimize throughput | `batch` processor |
| Export | Send to backends | `otlp` exporter |

### Custom Components (10%) - Only for OTEL Gaps

| Component | OTEL Gap | Our Solution |
|-----------|----------|--------------|
| Adaptive Sampling | No query-cost awareness | Custom processor |
| Circuit Breaker | No database protection | Custom processor |

## 🚀 Deployment Options

### Docker (Simplest)
```bash
docker-compose up -d
```

### Kubernetes (Production)
```bash
kubectl apply -f deploy/k8s/deployment.yaml
```

### Helm (Advanced)
```bash
helm install database-intelligence ./deploy/helm
```

## 📈 What's Different?

### Before (Custom Everything)
- 1000+ lines custom receiver code
- 15+ configuration files
- Complex DDD architecture
- Hard to maintain
- Limited community support

### After (OTEL-First)
- Standard OTEL receivers
- 3 configuration files
- Simple, clear architecture
- Easy to maintain
- Full community support

## 🔄 Migration Guide

Moving from custom components? See [MIGRATION_TO_OTEL.md](MIGRATION_TO_OTEL.md)

## 📚 Documentation

- [Configuration Guide](docs/CONFIGURATION_GUIDE.md) - Detailed configuration options
- [OTEL-First Approach](docs/OTEL_FIRST_APPROACH.md) - Why we chose this architecture
- [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment options
- [Troubleshooting](docs/TROUBLESHOOTING.md) - Common issues and solutions

## 🤝 Contributing

We welcome contributions! The best way to contribute is to:

1. Use standard OTEL components first
2. Only build custom if there's a genuine gap
3. Keep it simple and maintainable

## 📊 Metrics Collected

### PostgreSQL (via standard receiver)
- `postgresql.blocks_read` - Disk I/O
- `postgresql.commits` - Transaction rate
- `postgresql.connection.count` - Active connections
- `postgresql.database.size` - Database sizes
- Plus 50+ more standard metrics

### MySQL (via standard receiver)
- `mysql.connections` - Connection metrics
- `mysql.operations` - Query operations
- `mysql.buffer_pool.usage` - Memory usage
- Plus 40+ more standard metrics

### Custom Queries (via sqlquery receiver)
- `database.query.duration` - Query performance
- `database.query.count` - Execution frequency
- Any custom metric via SQL

## 🎯 When to Use Custom Processors

Only enable custom processors if you need:

1. **Adaptive Sampling**: Different sampling rates based on query cost
   ```yaml
   processors:
     database_intelligence/adaptive_sampler:
       enabled: true
       high_cost_threshold_ms: 1000
   ```

2. **Circuit Breaking**: Protect database from monitoring overhead
   ```yaml
   processors:
     database_intelligence/circuit_breaker:
       enabled: true
       error_threshold: 0.5
   ```

## 📞 Support

- **Documentation**: See `/docs` folder
- **Issues**: GitHub Issues
- **Community**: OpenTelemetry Slack

## 📜 License

MIT License - See [LICENSE](LICENSE) file

---

**Remember**: The best code is standard code. Only build custom when you must.