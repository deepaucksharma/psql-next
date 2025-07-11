# Database Intelligence with OpenTelemetry

Monitor PostgreSQL and MySQL databases using OpenTelemetry Collector with New Relic integration.

## 🚀 Quick Start

### Option 1: Config-Only Mode (Recommended)
No custom code needed - just YAML configuration with standard OTel components.

```bash
# 1. Set environment variables
export NEW_RELIC_LICENSE_KEY="your-key-here"
export DB_ENDPOINT="postgresql://localhost:5432/mydb"
export DB_USERNAME="monitor_user"
export DB_PASSWORD="secure_password"

# 2. Deploy with Docker
docker run -d \
  --name otelcol-db \
  -v $(pwd)/configs/examples/config-only-base.yaml:/etc/otelcol/config.yaml \
  --env-file .env \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otelcol/config.yaml
```

### Option 2: Enhanced Mode
Advanced features with custom components for enterprise use cases.

```bash
# Use our enhanced distribution
docker run -d \
  --name otelcol-db-enhanced \
  -v $(pwd)/configs/examples/enhanced-mode-full.yaml:/etc/otelcol/config.yaml \
  --env-file .env \
  dbotel/collector-enhanced:latest
```

## 📊 What You Get

### Core Metrics (Config-Only)
- **Connections**: Active, idle, max connections
- **Performance**: Query rates, transaction throughput
- **Resources**: CPU, memory, disk I/O, cache hit ratios
- **Health**: Replication lag, deadlocks, errors

### Advanced Analytics (Enhanced Mode)
- **Query Intelligence**: Execution plans, performance regression detection
- **Active Session History**: 1-second sampling of database activity
- **Smart Sampling**: Adaptive collection based on load
- **Cost Control**: Stay within metric budgets

## 📁 Repository Structure

```
database-intelligence-restructured/
├── configs/
│   └── examples/          # Ready-to-use configurations
│       ├── config-only-base.yaml      # PostgreSQL standard
│       ├── config-only-mysql.yaml     # MySQL standard
│       └── enhanced-mode-full.yaml    # Full advanced setup
├── docs/
│   ├── architecture/      # Technical design docs
│   ├── deployment-guide.md
│   ├── performance-tuning-guide.md
│   └── new-relic-integration-guide.md
└── internal/             # Custom components (enhanced mode)
    ├── receivers/        # Enhanced SQL, ASH receivers
    └── processors/       # 7 custom processors
```

## 📚 Documentation

### Getting Started
1. **[Architecture Overview](docs/architecture/otel-integration-strategy.md)** - Understand the two modes
2. **[Deployment Guide](docs/deployment-guide.md)** - Docker, Kubernetes, systemd options
3. **[New Relic Integration](docs/new-relic-integration-guide.md)** - Setup and validation

### Configuration Examples
- **PostgreSQL**: [config-only-base.yaml](configs/examples/config-only-base.yaml)
- **MySQL**: [config-only-mysql.yaml](configs/examples/config-only-mysql.yaml)
- **Enhanced**: [enhanced-mode-full.yaml](configs/examples/enhanced-mode-full.yaml)

### Advanced Topics
- **[Performance Tuning](docs/performance-tuning-guide.md)** - Optimize for scale
- **[Metrics Reference](docs/metrics-collection-strategy.md)** - All collected metrics
- **[Custom Components](docs/architecture/custom-components-design.md)** - Enhanced mode details

## 🔧 Key Features

### Config-Only Mode
- ✅ No custom code required
- ✅ Standard OTel components
- ✅ Works with any OTel distribution
- ✅ Low resource overhead
- ✅ Quick to deploy

### Enhanced Mode
- ✅ Query plan analysis
- ✅ Active session monitoring
- ✅ Intelligent sampling
- ✅ Circuit breaker protection
- ✅ Cost management

## 🏗️ Architecture

```
Your Database → OTel Collector → New Relic (via OTLP)
                    ↓
              Config-Only Mode:
              - PostgreSQL receiver
              - MySQL receiver
              - Basic processors
                    OR
              Enhanced Mode:
              - All standard receivers
              - Enhanced SQL receiver
              - ASH receiver
              - 7 custom processors
```

## 🚦 Prerequisites

- Database: PostgreSQL 11+ or MySQL 5.7+
- New Relic account with OTLP endpoint access
- Docker or Kubernetes (for deployment)
- Database read-only user with appropriate permissions

## 🔐 Security

- Use read-only database credentials
- Store secrets in environment variables
- Enable TLS for database connections
- Follow [security best practices](docs/deployment-guide.md#security-considerations)

## 📈 Performance Impact

| Mode | CPU Overhead | Memory Usage | Network | Database Impact |
|------|--------------|--------------|---------|------------------|
| Config-Only | < 5% | < 512MB | Low | Minimal |
| Enhanced | < 20% | < 2GB | Medium | Low-Medium |

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## 📄 License

This project is licensed under the Apache License 2.0 - see [LICENSE](LICENSE) file.



## 🔗 Quick Links

- [Example Configs](configs/examples/)
- [Troubleshooting](docs/new-relic-integration-guide.md#troubleshooting)
- [Performance Tuning](docs/performance-tuning-guide.md)
- [New Relic OTLP Docs](https://docs.newrelic.com/docs/more-integrations/open-source-telemetry-integrations/opentelemetry/opentelemetry-introduction/)

---

**Need help?** Check the [docs](docs/) or open an issue.

