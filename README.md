# PostgreSQL Unified Collector

A high-performance PostgreSQL metrics collector supporting both New Relic Infrastructure (NRI) and OpenTelemetry (OTLP) output formats. This collector provides comprehensive PostgreSQL monitoring with dual protocol support.

## 🚀 Features

- **Dual Output Support**: NRI (stdout) and OTLP (HTTP) with simultaneous collection
- **Extended Metrics**: Slow queries, wait events, blocking sessions, Active Session History (ASH)
- **Query Sanitization**: Automatic PII detection and smart query text sanitization
- **Multi-Instance**: Monitor multiple PostgreSQL instances from a single collector
- **Cloud-Native**: Kubernetes-ready with health checks and metrics endpoints
- **Docker & Compose**: Complete containerized deployment options
- **Regional Support**: US and EU New Relic regions supported

## 📦 Quick Start

### Prerequisites

- PostgreSQL 12+ with `pg_stat_statements` extension enabled
- New Relic account with license key
- Docker and Docker Compose (for containerized deployment)

### Environment Configuration

Create a `.env` file with your New Relic credentials:

```bash
# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_ACCOUNT_ID=your_account_id
NEW_RELIC_API_KEY=your_api_key
NEW_RELIC_REGION=US  # Options: US or EU

# PostgreSQL Configuration
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DATABASE=testdb

# Collector Configuration
COLLECTOR_MODE=hybrid  # Options: nri, otel, hybrid
COLLECTION_INTERVAL_SECS=30
```

### Docker Compose Deployment

```bash
# Clone the repository
git clone <repository-url>
cd psql-next

# Copy environment template
cp .env.example .env
# Edit .env with your credentials

# Start the complete stack
./scripts/run.sh start

# Or manually with docker-compose
docker-compose up -d
```

### Manual Binary Deployment

```bash
# Build from source
cargo build --release --features "nri otel"

# Run with config file
./target/release/postgres-unified-collector -c config.toml

# Run in different modes
./target/release/postgres-unified-collector --mode nri      # NRI only
./target/release/postgres-unified-collector --mode otel     # OTLP only  
./target/release/postgres-unified-collector --mode hybrid   # Both outputs
```

## 🏗️ Architecture

The collector uses a unified collection engine with pluggable output adapters:

```
PostgreSQL → Collection Engine → Unified Metrics → [NRI Adapter | OTLP Adapter]
                                                          ↓              ↓
                                                      stdout      OTel Collector → New Relic
```

### Components

- **Collection Engine**: Core metrics gathering and processing
- **NRI Adapter**: Outputs JSON to stdout for New Relic Infrastructure agent
- **OTLP Adapter**: Sends metrics via HTTP to OpenTelemetry Collector
- **Query Engine**: Handles SQL execution and result processing
- **Extension Manager**: Manages PostgreSQL extension compatibility

## 🚢 Deployment Options

### Docker Compose Profiles

```bash
# Start PostgreSQL only
docker-compose --profile postgres up -d

# Start with NRI collector
docker-compose --profile nri up -d

# Start with OTLP collector  
docker-compose --profile otlp up -d

# Start dual mode (NRI + OTLP)
docker-compose --profile dual up -d

# Start hybrid mode (single collector, both outputs)
docker-compose --profile hybrid up -d
```

### Kubernetes

```bash
# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/

# Create secrets (copy from template first)
cp deployments/kubernetes/secrets-template.yaml deployments/kubernetes/secrets.yaml
# Edit secrets.yaml with your credentials
kubectl apply -f deployments/kubernetes/secrets.yaml
```

### Streamlined Scripts

Use the unified `run.sh` script for all operations:

```bash
# Available commands
./scripts/run.sh help

# Quick start
./scripts/run.sh start     # Start all services
./scripts/run.sh test      # Run load tests
./scripts/run.sh verify    # Verify metrics collection
./scripts/run.sh stop      # Stop all services
./scripts/run.sh clean     # Clean up containers and data
```

## 📊 Metrics Collected

### PostgreSQL Metrics

- **Slow Queries**: Execution time, frequency, query text, query plans
- **Wait Events**: Lock waits, I/O waits, CPU usage
- **Blocking Sessions**: Lock contention and blocking query details
- **Active Session History (ASH)**: Oracle-style session sampling
- **Database Statistics**: Connection counts, transaction rates, cache hit ratios

### Output Formats

**NRI Format** (for New Relic Infrastructure):
```json
{
  "name": "com.newrelic.postgresql",
  "protocol_version": "4",
  "data": [{
    "entity": {
      "name": "postgres:localhost:5432",
      "type": "pg-instance"
    },
    "metrics": [{
      "event_type": "PostgresSlowQueries",
      "query_id": "123456789",
      "avg_elapsed_time_ms": 1500.5,
      "query_text": "SELECT * FROM users WHERE ...",
      "execution_count": 42
    }]
  }]
}
```

**OTLP Format** (for OpenTelemetry):
```
postgresql.query.duration{query_id="123456789", database="testdb"} = 1500.5
postgresql.query.count{query_id="123456789", database="testdb"} = 42
```

## 🔧 Configuration

### Configuration Files

Example configuration with all options:

```toml
# Connection settings
connection_string = "postgresql://postgres:postgres@localhost:5432/testdb"
host = "localhost"
port = 5432
databases = ["testdb"]
max_connections = 5
connect_timeout_secs = 30

# Collection settings
collection_interval_secs = 30
collection_mode = "hybrid"  # nri, otel, or hybrid

# Query monitoring thresholds
query_monitoring_count_threshold = 20
query_monitoring_response_time_threshold = 500
max_query_length = 4095

# Extended features
enable_extended_metrics = true
enable_ash = true
ash_sample_interval_secs = 15
ash_retention_hours = 24
ash_max_memory_mb = 512

# Security
sanitize_query_text = true
sanitization_mode = "smart"  # none, basic, smart

# Output configurations
[outputs.nri]
enabled = true
entity_key = "postgres:localhost:5432"
integration_name = "com.newrelic.postgresql"

[outputs.otlp]
enabled = true
endpoint = "http://otel-collector:4318"
compression = "gzip"
timeout_secs = 10
headers = [
    ["api-key", "${NEW_RELIC_LICENSE_KEY}"]
]

# Sampling configuration
[sampling]
mode = "fixed"
base_sample_rate = 1.0
rules = []
```

### Regional Configuration

The collector automatically configures endpoints based on your New Relic region:

- **US Region**: `https://otlp.nr-data.net:4318`
- **EU Region**: `https://otlp.eu01.nr-data.net:4318`

Set `NEW_RELIC_REGION=EU` in your `.env` file for EU accounts.

## 🧪 Testing and Verification

### Generate Test Load

```bash
# Generate slow queries for testing
./scripts/run.sh test

# Verify metrics collection
./scripts/run.sh verify

# Query metrics in New Relic
./scripts/verify-metrics.sh
```

### Health Checks

```bash
# Check collector health
curl http://localhost:8080/health

# Check OpenTelemetry Collector health  
curl http://localhost:13133/health

# View Prometheus metrics
curl http://localhost:8888/metrics
```

## 🔒 Security

### Secrets Management

- All sensitive configuration is managed via environment variables
- No secrets are hardcoded in source code
- Kubernetes deployments use Secret resources
- Query text sanitization removes PII automatically

### Network Security

- TLS encryption for all New Relic communications
- Configurable timeout and retry policies
- Memory-bounded operations prevent DoS

## 📖 Documentation Structure

```
docs/
├── IMPLEMENTATION.md     # Architecture and design details
├── DEPLOYMENT.md        # Installation and configuration guide
├── TESTING.md          # Testing procedures and results
└── MIGRATION.md        # Migration from nri-postgresql

examples/
├── docker-config.toml   # Docker environment configuration
├── working-config.toml  # Local development configuration
└── simple-config.toml   # Minimal configuration example

scripts/
├── run.sh              # Master control script
├── set-newrelic-endpoint.sh  # Region configuration
└── verify-metrics.sh   # Metrics validation
```

## 🚀 Performance

### Benchmarks

- **Memory Usage**: ~50MB typical, bounded ASH sampling
- **CPU Impact**: <2% on monitored PostgreSQL instance
- **Collection Latency**: ~100ms per collection cycle
- **Throughput**: 1000+ metrics/second OTLP export

### Scaling

- Single collector can monitor multiple PostgreSQL instances
- Horizontal scaling via multiple collector instances
- Built-in connection pooling and query optimization

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## 📄 License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

## 🆘 Support

- **GitHub Issues**: [Report bugs or request features](https://github.com/newrelic/postgres-unified-collector/issues)
- **New Relic Support**: [Contact support](https://support.newrelic.com)
- **Documentation**: Full documentation in `/docs` directory