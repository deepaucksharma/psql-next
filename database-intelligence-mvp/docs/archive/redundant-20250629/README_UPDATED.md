# Database Intelligence MVP

> **Production-Ready OpenTelemetry-based Database Monitoring Solution**

## 🚀 Current Status

This project provides enterprise-grade database monitoring using OpenTelemetry collectors with the following capabilities:

### ✅ What's Working
- **PostgreSQL Monitoring**: Full metrics collection via standard OTEL receivers
- **Dimensional Metrics**: All data collected as OTEL metrics with proper dimensions
- **OHI Capability Parity**: Matches PostgreSQL On-Host Integration capabilities
- **High Availability**: Redis-backed distributed state management (with proper setup)
- **Basic MySQL Support**: Standard MySQL receiver integration

### 🚧 In Development
- **Custom Components**: Build system available via `make build-collector`
- **Advanced Features**: Adaptive sampling and circuit breaker (requires custom build)
- **Query Plan Collection**: Implemented but requires custom build

### ❌ Not Implemented
- **MongoDB Support**: Not available (removed false claims)
- **pg_get_json_plan()**: Not required - uses native EXPLAIN

## 📋 Prerequisites

- Docker & Docker Compose
- PostgreSQL 12+ with `pg_stat_statements` enabled
- New Relic account with license key
- (Optional) Redis for HA setup
- (Optional) Go 1.21+ for custom builds

## 🚀 Quick Start

### 1. Basic Setup (Single Instance)

```bash
# Clone the repository
git clone https://github.com/database-intelligence/database-intelligence-mvp.git
cd database-intelligence-mvp

# Copy and configure environment
cp .env.example .env
# Edit .env with your credentials

# Start with Docker Compose
docker-compose -f deploy/docker/docker-compose.yaml up -d

# Verify metrics are flowing
./scripts/verify-metrics.sh
```

### 2. High Availability Setup

```bash
# Start HA setup with Redis
docker-compose -f deploy/docker/docker-compose-ha.yaml up -d

# This includes:
# - Redis for distributed state
# - 2 collector instances
# - Nginx load balancer
# - PostgreSQL test database
```

### 3. Custom Build (Advanced Features)

```bash
# Install OpenTelemetry Collector Builder
make install-tools

# Build custom collector with all components
make build-collector

# Build Docker image
make docker-build

# Use custom image in docker-compose
# Update image to: database-intelligence/collector:latest
```

## 📊 Configuration

### Basic Configuration (`config/collector-otel-metrics.yaml`)
```yaml
receivers:
  postgresql:        # Standard PostgreSQL metrics
  sqlquery:         # Custom query metrics

processors:
  memory_limiter:   # Prevent OOM
  batch:           # Optimize sending
  
exporters:
  otlp/newrelic:   # Send to New Relic
```

### High Availability Configuration (`config/collector-ha.yaml`)
- Redis-backed state storage
- Distributed adaptive sampling
- Circuit breaker coordination
- Leader election for stateful operations

## 🔍 Collected Metrics

### PostgreSQL Metrics (Dimensional)
- `postgresql.blocks_read` - I/O operations
- `postgresql.commits` / `postgresql.rollbacks` - Transaction metrics
- `postgresql.connection.count` - Connection pool metrics
- `postgresql.table.size` - Table sizes
- `db.query.count` - Query execution counts
- `db.query.mean_duration` - Query performance
- `db.connections.active/idle/blocked` - Connection states
- `db.wait_events` - Wait event analysis

All metrics include dimensions like:
- `database_name`
- `schema_name`
- `table_name`
- `query_id`
- `statement_type`

## 🛡️ Security

### Credential Management
- Use environment variables for development
- Use Kubernetes secrets for production
- Never commit credentials to git

### Database Permissions
```sql
-- Minimum required permissions
CREATE USER newrelic_monitor WITH PASSWORD 'secure_password';
GRANT pg_read_all_settings TO newrelic_monitor;
GRANT pg_read_all_stats TO newrelic_monitor;
GRANT SELECT ON pg_stat_statements TO newrelic_monitor;
```

## 📈 Monitoring & Alerting

### New Relic Dashboards
Import provided dashboard configurations:
- `monitoring/dashboards/postgresql-overview.json`
- `monitoring/dashboards/query-performance.json`

### Example Queries
```sql
-- Top slow queries
SELECT average(db.query.mean_duration) 
FROM Metric 
WHERE db.system = 'postgresql'
FACET query_id, statement_type
SINCE 1 hour ago

-- Connection pool status
SELECT latest(db.connections.active) as 'Active',
       latest(db.connections.idle) as 'Idle',
       latest(db.connections.blocked) as 'Blocked'
FROM Metric
WHERE db.system = 'postgresql'
```

## 🔧 Troubleshooting

### No Metrics Appearing
1. Check collector logs: `docker logs db-intel-collector`
2. Verify database connectivity
3. Ensure pg_stat_statements is enabled
4. Check New Relic license key

### High Memory Usage
- Adjust `memory_limiter` processor settings
- Reduce collection frequency
- Enable sampling for high-volume queries

### Circuit Breaker Activating
- Check database load
- Adjust error thresholds in configuration
- Monitor `circuit_breaker.state` metrics

## 🏗️ Architecture

```
┌─────────────────┐     ┌─────────────────┐
│   PostgreSQL    │────▶│  OTEL Collector │
└─────────────────┘     └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │     Redis       │
                        │ (State Storage) │
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │   New Relic     │
                        └─────────────────┘
```

## 📝 Documentation

- [Configuration Guide](docs/CONFIGURATION.md)
- [OHI Migration Guide](docs/OHI_TO_OTEL_METRIC_MAPPING.md)
- [Custom Build Guide](docs/CUSTOM_BUILD.md)
- [Troubleshooting Guide](docs/TROUBLESHOOTING.md)

## 🤝 Contributing

1. Fork the repository
2. Create feature branch
3. Run tests: `make test`
4. Submit pull request

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE) file

## ⚠️ Known Limitations

1. **MySQL Support**: Basic metrics only, not extensively tested
2. **Custom Components**: Require building custom collector
3. **PII Detection**: Regex-based, may not catch all cases
4. **State Storage**: File storage limited to single instance

## 🛣️ Roadmap

- [ ] MySQL production validation
- [ ] ClickHouse receiver
- [ ] Automated PII detection improvements
- [ ] Kubernetes operator for easier deployment
- [ ] Multi-cloud support (AWS RDS, GCP CloudSQL)