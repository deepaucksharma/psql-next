# Database Intelligence with OpenTelemetry

Monitor PostgreSQL and MySQL databases using OpenTelemetry Collector with New Relic integration.

## 🚀 Quick Start

### Config-Only Mode (Production Ready)
Works with standard OpenTelemetry Collector Contrib - no custom components needed.

```bash
# 1. Set environment variables
export NEW_RELIC_LICENSE_KEY="your-key-here"
export DB_POSTGRES_HOST="localhost"
export DB_POSTGRES_PORT="5432"
export DB_POSTGRES_USER="monitor_user"
export DB_POSTGRES_PASSWORD="secure_password"
export DB_POSTGRES_DATABASE="postgres"
export SERVICE_NAME="postgresql-prod-01"
export ENVIRONMENT="production"

# 2. Deploy with Docker
docker run -d \
  --name otelcol-db \
  -v $(pwd)/configs/examples/config-only-working.yaml:/etc/otelcol/config.yaml \
  --env-file .env \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otelcol/config.yaml
```

### Enhanced Mode (Requires Custom Build)
⚠️ **Note**: Enhanced mode requires building a custom collector with our components. See [Building Custom Collector](#building-custom-collector) below.

```bash
# Build the custom collector first
make build-collector

# Run with enhanced configuration
./bin/database-intelligence-collector \
  --config=configs/examples/enhanced-mode-corrected.yaml
```

## 📊 What You Get

### Core Metrics (Config-Only Mode) - Available Now
- **Connections**: Active, idle, max connections by state
- **Performance**: Commits, rollbacks, blocks hit/read
- **Database Size**: Database and table sizes
- **Query Performance**: Long-running query detection
- **Table Health**: Bloat estimation, vacuum stats
- **Host Metrics**: CPU, memory, disk, network

### Advanced Features (Enhanced Mode) - Custom Build Required
- **Query Plan Analysis**: Extract and analyze execution plans
- **Active Session History (ASH)**: 1-second sampling of activity
- **Smart Sampling**: Adaptive collection based on load
- **Cost Control**: Stay within New Relic metric budgets
- **OHI Dashboard Compatibility**: Transform metrics for existing dashboards
- **Circuit Breaker**: Protect databases from metric collection overload

## 📁 Repository Structure

```
database-intelligence-restructured/
├── configs/
│   └── examples/
│       ├── config-only-working.yaml      # PostgreSQL (works now)
│       ├── config-only-mysql.yaml        # MySQL (works now)
│       ├── enhanced-mode-corrected.yaml  # Enhanced (custom build)
│       └── .env.template                 # Environment variables
├── components/                           # Custom components source
│   ├── receivers/
│   │   ├── ashreceiver/                 # Active Session History
│   │   ├── enhancedsqlreceiver/         # Smart SQL metrics
│   │   └── kernelmetricsreceiver/       # OS kernel metrics
│   ├── processors/
│   │   ├── adaptivesampler/             # Intelligent sampling
│   │   ├── circuitbreaker/              # Database protection
│   │   ├── ohitransform/                # OHI compatibility
│   │   └── [other processors]/
│   └── exporters/
│       └── nri/                         # New Relic Infrastructure format
├── distributions/
│   ├── minimal/                         # Lightweight build
│   ├── production/                      # Standard build
│   └── enterprise/                      # Full-featured build
└── docs/
    ├── 01-quick-start/
    ├── 02-deployment/
    └── 03-configuration/
```

## 🔧 Building Custom Collector

To use enhanced mode features, you need to build a custom collector:

```bash
# Install builder
go install go.opentelemetry.io/collector/cmd/builder@v0.105.0

# Build collector with all components
builder --config=otelcol-builder-config-complete.yaml

# The binary will be in distributions/production/
./distributions/production/database-intelligence-collector --config=configs/examples/enhanced-mode-corrected.yaml
```

## 📚 Documentation

### Getting Started
1. **[Quick Start Guide](docs/01-quick-start/README.md)** - Get running in 5 minutes
2. **[Configuration Guide](docs/03-configuration/base-configuration.md)** - Customize your setup
3. **[Deployment Guide](docs/02-deployment/deployment-options.md)** - Production deployment

### Configuration Examples
- **PostgreSQL Standard**: [config-only-working.yaml](configs/examples/config-only-working.yaml)
- **MySQL Standard**: [config-only-mysql.yaml](configs/examples/config-only-mysql.yaml)  
- **Enhanced Mode**: [enhanced-mode-corrected.yaml](configs/examples/enhanced-mode-corrected.yaml)

### Environment Variables
See [.env.template](configs/examples/.env.template) for all configuration options.

## 🚦 Current Status

### ✅ Production Ready (Config-Only Mode)
- PostgreSQL metrics collection
- MySQL metrics collection
- Custom SQL queries
- Host metrics
- New Relic OTLP export
- Prometheus metrics export

### 🚧 In Development (Enhanced Mode)
- ASH receiver (partial implementation)
- Query plan extraction
- Advanced processors
- OHI dashboard compatibility

## 🐳 Docker Images

### Available Now
```bash
# Standard OpenTelemetry Collector (for config-only mode)
otel/opentelemetry-collector-contrib:latest
otel/opentelemetry-collector-contrib:0.105.0
```

### Custom Images (Not Yet Published)
```bash
# These will be available after CI/CD setup:
# database-intelligence:latest
# database-intelligence:2.0.0
# database-intelligence:enterprise
```

## 🔍 Metrics Collected

### PostgreSQL Metrics
| Metric | Description | Mode |
|--------|-------------|------|
| postgresql.connections.active | Active connections | Config-Only |
| postgresql.commits | Transaction commits | Config-Only |
| postgresql.blocks.hit | Buffer cache hits | Config-Only |
| postgresql.database.size | Database size in bytes | Config-Only |
| postgresql.table.bloat.ratio | Table bloat estimation | Config-Only |
| db.ash.active_sessions | Active session samples | Enhanced |
| db.query.plan_cost | Query plan cost | Enhanced |

### MySQL Metrics
| Metric | Description | Mode |
|--------|-------------|------|
| mysql.connections | Current connections | Config-Only |
| mysql.commands | Command execution rates | Config-Only |
| mysql.buffer_pool.pages | Buffer pool statistics | Config-Only |
| mysql.innodb.row_lock.time | Row lock wait time | Config-Only |

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request

## 📄 License

This project is licensed under the Apache License 2.0 - see the LICENSE file for details.

## 🆘 Support

- **Documentation**: See the [docs/](docs/) directory
- **Issues**: [GitHub Issues](https://github.com/yourusername/database-intelligence-restructured/issues)
- **New Relic Support**: [support.newrelic.com](https://support.newrelic.com)

## ⚠️ Important Notes

1. **Enhanced Mode**: Requires building a custom collector. Not available as a pre-built image yet.
2. **OHI Migration**: If migrating from New Relic OHI, you'll need the ohitransform processor (enhanced mode only).
3. **Performance**: Config-only mode has <5% overhead. Enhanced mode may use up to 20% CPU.
4. **Security**: Always use read-only database credentials.

## 🚀 Roadmap

- [ ] Publish Docker images to registry
- [ ] Complete ASH receiver implementation  
- [ ] Add dashboard templates
- [ ] Implement missing MySQL enhanced features
- [ ] Add Grafana dashboard support
- [ ] Support for more databases (MongoDB, Redis)