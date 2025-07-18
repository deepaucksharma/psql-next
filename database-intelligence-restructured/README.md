# Database Intelligence - OpenTelemetry Collector Distribution

[![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-Enabled-blue)](https://opentelemetry.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-12%2B-336791)](https://www.postgresql.org)
[![MySQL](https://img.shields.io/badge/MySQL-5.7%2B-4479A1)](https://www.mysql.com)
[![Go Version](https://img.shields.io/badge/Go-1.23.0-00ADD8)](https://golang.org)
[![Status](https://img.shields.io/badge/Status-Production%20Ready-green)](https://github.com/database-intelligence/db-intel)

Advanced database monitoring using OpenTelemetry with enhanced features for query intelligence, performance analysis, and adaptive sampling.

## 🚀 Quick Links

| **Getting Started** | **Documentation** | **Support** |
|:-----------------:|:---------------:|:---------:|
| [Quick Start Guide](docs/guides/QUICK_START.md) | [Full Documentation](docs/README.md) | [GitHub Issues](https://github.com/database-intelligence/db-intel/issues) |
| [Installation Guide](docs/guides/DEPLOYMENT.md) | [Configuration Reference](docs/guides/CONFIGURATION.md) | [Discussions](https://github.com/database-intelligence/db-intel/discussions) |
| [Examples](configs/examples/) | [Architecture](docs/reference/ARCHITECTURE.md) | [Contributing](CONTRIBUTING.md) |

## 🔍 What is Database Intelligence?

A specialized OpenTelemetry Collector distribution that provides:
- **Deep database insights** beyond standard metrics
- **Query intelligence** with execution plan analysis
- **Adaptive monitoring** that adjusts to system load
- **Multi-database support** with unified dashboards

## ✨ Key Features

### 🎯 Two Operating Modes
1. **Config-Only Mode** - Uses standard OpenTelemetry components
   - ✅ Production-ready with official OTel Collector
   - 📊 35+ database metrics out of the box
   - 🔧 Simple YAML configuration
   - 💾 <512MB memory, <5% CPU

2. **Enhanced Mode** - Includes custom intelligence components
   - 🧠 Query plan analysis and optimization hints
   - 📈 Active Session History (ASH) for PostgreSQL
   - 🎛️ Adaptive sampling based on system load
   - 🛡️ Circuit breaker for overload protection
   - 💰 Cost control and telemetry budgeting

## 🗄️ Supported Databases

### Fully Supported
- **PostgreSQL** (12+)
  - Standard metrics via `postgresqlreceiver`
  - Enhanced metrics via `enhancedsql` receiver
  - Active Session History via `ash` receiver
  - Query plan extraction
  - Wait event analysis
  
- **MySQL** (5.7+, 8.0+)
  - Standard metrics via `mysqlreceiver`
  - Enhanced metrics via `enhancedsql` receiver
  - Performance schema integration
  - InnoDB metrics

### Beta Support
- **MongoDB** (3.6+)
  - Enhanced receiver with replica set support
  - Sharding metrics via `mongodb` receiver
  - Query profiling and custom metrics
  - Oplog monitoring and replication lag

- **Redis** (2.8+)
  - Enhanced receiver with cluster support
  - Sentinel monitoring via `redis` receiver
  - Slow log analysis and latency tracking
  - Memory breakdown and command statistics

### Planned Support
- **Oracle** - ASH/AWR integration (pending)
- **SQL Server** - Query Store integration (pending)

## 🚀 Quick Start (5 minutes)

### Option 1: Docker (Fastest)
```bash
# Set up environment
export NEW_RELIC_LICENSE_KEY="your_key_here"
export DB_ENDPOINT="postgresql://user:pass@localhost:5432/mydb"

# Run with Docker
docker run -d \
  -e NEW_RELIC_LICENSE_KEY \
  -e DB_ENDPOINT \
  -p 13133:13133 \
  dbintel/collector:latest
```

### Option 2: Binary Installation
```bash
# Download latest release
curl -L -o dbintel https://github.com/database-intelligence/releases/latest/dbintel_linux_amd64
chmod +x dbintel

# Run with config-only mode
./dbintel --config=configs/modes/config-only.yaml
```

📖 See [Quick Start Guide](docs/guides/QUICK_START.md) for detailed instructions.

## 📦 Project Structure

```
database-intelligence/
├── .ci/                # Build & CI/CD automation
├── components/         # Custom OTel components
│   ├── receivers/      # ASH, Enhanced SQL, MongoDB, Redis
│   ├── processors/     # Adaptive sampling, circuit breaker
│   └── exporters/      # New Relic integration
├── configs/            # Configuration files
│   ├── modes/          # config-only.yaml, enhanced.yaml
│   └── examples/       # Ready-to-use examples
├── deployments/        # Docker, K8s, Helm charts
├── distributions/      # Binary distributions
├── docs/               # Documentation
└── tests/              # E2E and unit tests
```

## ⚙️ Configuration Examples

### Config-Only Mode (Standard OTel)
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: ${env:DB_USER}
    password: ${env:DB_PASSWORD}
    databases: [postgres]
    collection_interval: 10s

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
```

### Enhanced Mode (With Intelligence)
```yaml
receivers:
  ash:  # Active Session History
    driver: postgres
    datasource: "${env:DB_ENDPOINT}"
    buffer_size: 10000

processors:
  adaptivesampler:
    mode: adaptive
    target_records_per_second: 1000
  
  querycorrelator:
    max_queries_tracked: 10000

exporters:
  nri:
    api_key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [ash]
      processors: [adaptivesampler, querycorrelator]
      exporters: [nri]
```

📖 See [Configuration Guide](docs/guides/CONFIGURATION.md) for complete reference.

## 🧩 Components

| Component | Type | Description | Mode |
|-----------|------|-------------|------|
| `postgresql` | Receiver | Standard PostgreSQL metrics | Config-Only |
| `mysql` | Receiver | Standard MySQL metrics | Config-Only |
| `enhancedsql` | Receiver | Advanced SQL with feature detection | Enhanced |
| `ash` | Receiver | Active Session History (PostgreSQL) | Enhanced |
| `mongodb` | Receiver | MongoDB with replica set support | Both |
| `redis` | Receiver | Redis with cluster support | Both |
| `adaptivesampler` | Processor | Intelligent load-based sampling | Enhanced |
| `circuitbreaker` | Processor | Overload protection | Enhanced |
| `querycorrelator` | Processor | Query-to-metric correlation | Enhanced |
| `otlp` | Exporter | OpenTelemetry Protocol | Both |
| `nri` | Exporter | New Relic Infrastructure | Both |

## 🔧 Development

### Prerequisites
- Go 1.23.0+
- Docker 20.10+
- Make

### Quick Development Setup
```bash
# Clone and build
git clone https://github.com/database-intelligence/db-intel
cd db-intel
make build

# Run tests
make test          # Unit tests
make test-e2e      # End-to-end tests

# Development mode
make dev           # Format, lint, test
make dev-run       # Run with hot reload
```

📖 See [Development Guide](docs/development/SETUP.md) for detailed setup.

## 📊 Monitoring & Dashboards

### New Relic Integration
- Automatic dashboard creation
- Pre-built alerts for common issues
- Query performance insights
- Cost tracking dashboard

### Metrics Available
- **Performance**: Query latency, cache hit ratios, throughput
- **Resources**: CPU, memory, disk I/O, connections
- **Health**: Replication lag, deadlocks, long-running queries
- **Custom**: User-defined SQL queries

📖 See [Metrics Reference](docs/reference/METRICS.md) for complete list.

## 🤝 Contributing

We welcome contributions! See [Contributing Guidelines](CONTRIBUTING.md).

### Quick Contribution Guide
1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Make changes and test: `make dev`
4. Commit: `git commit -m 'Add amazing feature'`
5. Push and create PR

## 📚 Resources

| Resource | Description |
|----------|-------------|
| [Documentation](docs/README.md) | Complete documentation |
| [Examples](configs/examples/) | Configuration examples |
| [Troubleshooting](docs/guides/TROUBLESHOOTING.md) | Common issues & solutions |
| [API Reference](docs/reference/API.md) | Component APIs |
| [Architecture](docs/reference/ARCHITECTURE.md) | System design |
| [Releases](https://github.com/database-intelligence/releases) | Download binaries |

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

---

**Version**: 3.0.0 | **Go**: 1.23.0+ | **Status**: Production Ready (PostgreSQL/MySQL), Beta (MongoDB/Redis)  
**Support**: [Issues](https://github.com/database-intelligence/db-intel/issues) | [Discussions](https://github.com/database-intelligence/db-intel/discussions)