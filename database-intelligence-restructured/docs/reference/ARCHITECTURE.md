# Database Intelligence with OpenTelemetry - Overview

## 🎯 Project Purpose

Monitor PostgreSQL and MySQL databases using OpenTelemetry Collector with New Relic integration, providing comprehensive database intelligence without vendor lock-in.

## 🏗️ Architecture Modes

### 1. Config-Only Mode (Production Ready) ✅

**What it is**: Uses only standard OpenTelemetry components configured via YAML.

**Capabilities**:
- ✅ Core database metrics (connections, transactions, locks)
- ✅ Performance metrics (query rates, cache hit ratios)
- ✅ Resource utilization (CPU, memory, disk I/O)
- ✅ Health monitoring (replication lag, deadlocks)
- ✅ Custom SQL queries for business metrics

**Resource Impact**:
- CPU: <5% overhead
- Memory: <512MB
- Network: Low bandwidth usage

**Deployment**: Works with any OpenTelemetry Collector distribution.

```yaml
# Example: Basic PostgreSQL monitoring
receivers:
  postgresql:
    endpoint: "postgresql://localhost:5432/db"
    username: "${DB_USER}"
    password: "${DB_PASS}"
    collection_interval: 30s

processors:
  batch: {}
  resource:
    attributes:
      - key: service.name
        value: "my-database"

exporters:
  otlp:
    endpoint: "${NEW_RELIC_OTLP_ENDPOINT}"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [resource, batch]
      exporters: [otlp]
```

### 2. Enhanced Mode (Development) ⚠️

**What it is**: Includes custom receivers and processors for advanced database intelligence.

**Additional Capabilities** (when fully integrated):
- 🔄 Active Session History (ASH) monitoring
- 🔄 Query execution plan analysis
- 🔄 Intelligent adaptive sampling
- 🔄 Circuit breaker protection
- 🔄 Cost control and budget management
- 🔄 Query correlation and tracing

**Current Status**: Components exist in source code but are not integrated into any distribution.

**Resource Impact** (theoretical):
- CPU: <20% overhead  
- Memory: <2GB
- Network: Medium bandwidth usage

## 📊 Data Flow

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐    ┌─────────────┐
│  Database   │───▶│  OTel        │───▶│ Processors  │───▶│ New Relic   │
│ (Postgres/  │    │ Receivers    │    │ (Transform/ │    │ (via OTLP)  │
│  MySQL)     │    │              │    │  Enrich)    │    │             │
└─────────────┘    └──────────────┘    └─────────────┘    └─────────────┘
                            │
                   ┌──────────────┐
                   │   Health     │
                   │  Monitoring  │
                   │  (Prometheus)│
                   └──────────────┘
```

## 🔌 Supported Databases

| Database | Version | Receiver | Custom SQL | Status |
|----------|---------|----------|------------|--------|
| PostgreSQL | 11+ | ✅ Built-in | ✅ Yes | Production |
| MySQL | 5.7+ | ✅ Built-in | ✅ Yes | Production |

## 📈 Metrics Coverage

### Core Database Metrics (✅ Available Now)

**Connection Management**:
- Active/idle/max connections
- Connection pool utilization
- New connections per second

**Transaction Performance**:
- Commits/rollbacks per second
- Transaction duration histograms
- Lock waits and deadlocks

**Query Performance**:
- Query execution rates
- Slow query detection
- Cache hit ratios

**Resource Utilization**:
- CPU usage by database processes
- Memory allocation and usage
- Disk I/O rates and latency

### Advanced Analytics (⚠️ Development)

**Active Session History**:
- 1-second session sampling
- Wait event categorization
- Blocking session identification

**Query Intelligence**:
- Execution plan capture
- Performance regression detection
- Cost estimation analysis

**Operational Intelligence**:
- Predictive load balancing
- Automated scaling triggers
- Health trend analysis

## 🚀 Quick Start Options

### Option 1: Standard OpenTelemetry (Recommended)

```bash
# 1. Download standard collector
curl -L -o otelcol \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/latest/download/otelcol_linux_amd64

# 2. Use our config
curl -L -o config.yaml \
  https://raw.githubusercontent.com/your-repo/configs/examples/config-only-base.yaml

# 3. Set environment variables
export NEW_RELIC_LICENSE_KEY="your-key"
export DB_ENDPOINT="postgresql://localhost:5432/mydb"

# 4. Run
./otelcol --config=config.yaml
```

### Option 2: Docker

```bash
docker run -d \
  --name db-otel \
  -v $(pwd)/config.yaml:/etc/otelcol/config.yaml \
  -e NEW_RELIC_LICENSE_KEY="your-key" \
  -e DB_ENDPOINT="postgresql://host.docker.internal:5432/mydb" \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otelcol/config.yaml
```

## 🔐 Security Considerations

**Database Access**:
- Use read-only database credentials
- Limit connection pool size
- Enable SSL/TLS for database connections

**Credential Management**:
- Store secrets in environment variables
- Use secret management systems (K8s secrets, AWS SSM, etc.)
- Rotate API keys regularly

**Network Security**:
- Encrypt OTLP traffic to New Relic
- Use private networks where possible
- Implement network segmentation

## 📋 Prerequisites

**Database Requirements**:
- PostgreSQL 11+ or MySQL 5.7+
- Read-only user with appropriate permissions
- Network access from collector to database

**New Relic Requirements**:
- New Relic account with OTLP endpoint access
- Valid license key or API key
- Sufficient data ingest limits

**Infrastructure Requirements**:
- Linux, macOS, or Windows
- 512MB+ RAM for Config-Only mode
- 2GB+ RAM for Enhanced mode (when available)
- Docker or Kubernetes (optional)

## 🎯 Use Cases

**DevOps Teams**:
- Monitor database health across environments
- Set up alerting on key performance indicators
- Track resource utilization trends

**Database Administrators**:
- Identify slow queries and performance bottlenecks
- Monitor replication lag and consistency
- Analyze query execution patterns

**Application Developers**:
- Correlate application performance with database metrics
- Monitor connection pool efficiency
- Track custom business metrics via SQL queries

**Platform Engineers**:
- Standardize monitoring across database types
- Implement infrastructure as code for monitoring
- Maintain observability without vendor lock-in

## 📖 Next Steps

1. **Get Started**: Follow the [Quick Start Guide](QUICK_START.md)
2. **Configure**: Review [Configuration Reference](CONFIGURATION.md)
3. **Deploy**: See [Deployment Guide](DEPLOYMENT.md)
4. **Test**: Run [E2E Tests](TESTING.md)
5. **Troubleshoot**: Check [Troubleshooting Guide](TROUBLESHOOTING.md)