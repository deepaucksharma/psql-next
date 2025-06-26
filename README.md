# PostgreSQL Unified Collector

A unified PostgreSQL metrics collector that supports both New Relic Infrastructure (NRI) and OpenTelemetry (OTel) output formats. This collector maintains 100% compatibility with the existing OHI PostgreSQL integration while adding enhanced capabilities.

## 📚 Documentation

For comprehensive documentation, please see the [docs/](docs/) directory:

- [Architecture Overview](docs/01-architecture-overview.md) - System design and components
- [Implementation Guide](docs/02-implementation-guide.md) - Building and extending
- [Deployment & Operations](docs/03-deployment-operations.md) - Installation and configuration
- [Metrics Reference](docs/04-metrics-reference.md) - Complete metrics documentation
- [Migration Guide](docs/05-migration-guide.md) - Upgrading from nri-postgresql

## Features

### Core Features
- **100% OHI Compatibility**: Drop-in replacement for existing nri-postgresql integration
- **Dual Output Modes**: Supports both New Relic Infrastructure and OpenTelemetry formats
- **Version-Aware**: Automatically adapts queries for PostgreSQL versions 12+
- **RDS Support**: Special handling for AWS RDS PostgreSQL instances
- **Extension Management**: Automatic detection and configuration of PostgreSQL extensions

### Metric Types (OHI Compatible)
- **Slow Running Queries**: Query performance metrics with execution counts and timing
- **Wait Events**: Database wait event analysis (requires pg_wait_sampling)
- **Blocking Sessions**: Lock contention and blocking query detection
- **Individual Queries**: Real-time query monitoring (enhanced with pg_stat_monitor)
- **Execution Plans**: Query plan collection and analysis

### Extended Capabilities
- **Active Session History (ASH)**: Oracle-like session sampling
- **eBPF Integration**: Kernel-level performance metrics (optional)
- **Plan Change Detection**: Automatic detection of query plan changes
- **Adaptive Sampling**: Intelligent metric sampling based on load
- **Per-Query Tracing**: Detailed execution traces

## Installation

### Binary Installation

```bash
# Download the latest release
wget https://github.com/your-org/postgres-unified-collector/releases/latest/download/postgres-unified-collector
chmod +x postgres-unified-collector
sudo mv postgres-unified-collector /usr/local/bin/
```

### From Source

```bash
# Clone the repository
git clone https://github.com/your-org/postgres-unified-collector.git
cd postgres-unified-collector

# Build all features
cargo build --release --features all

# Install binaries
sudo cp target/release/postgres-unified-collector /usr/local/bin/
sudo cp target/release/nri-postgresql /usr/local/bin/
sudo cp target/release/postgres-otel-collector /usr/local/bin/
```

### Docker

```bash
# Using Docker Compose
docker-compose -f deployments/docker/docker-compose.yml up -d

# Using standalone container
docker run -d \
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PASSWORD=mypassword \
  -e NEW_RELIC_LICENSE_KEY=your_key \
  postgres-unified-collector:latest
```

### Kubernetes

```bash
# Create namespace and secrets
kubectl create namespace postgres-monitoring
kubectl -n postgres-monitoring create secret generic postgres-credentials \
  --from-literal=username=postgres \
  --from-literal=password=yourpassword
kubectl -n postgres-monitoring create secret generic newrelic-license \
  --from-literal=key=your_license_key

# Deploy collector
kubectl apply -f deployments/kubernetes/deployment.yaml
```

## Configuration

### Basic Configuration

```toml
# /etc/postgres-collector/config.toml

connection_string = "postgresql://postgres:password@localhost:5432/postgres"
databases = ["postgres", "myapp"]
collection_interval_secs = 60
collection_mode = "hybrid"  # "otel", "nri", or "hybrid"

[outputs.nri]
enabled = true
entity_key = "${HOSTNAME}:${PORT}"

[outputs.otlp]
enabled = true
endpoint = "http://localhost:4317"
```

### Environment Variables

For backward compatibility with nri-postgresql:

```bash
export HOSTNAME=localhost
export PORT=5432
export USERNAME=postgres
export PASSWORD=mypassword
export DATABASE=postgres
export COLLECTION_LIST='{"postgres": {"schemas": ["public"]}}'
export QUERY_MONITORING_COUNT_THRESHOLD=20
export ENABLE_EXTENDED_METRICS=true
```

## Usage

### Unified Collector (Recommended)

```bash
# Run with default configuration
postgres-unified-collector

# Run with custom config
postgres-unified-collector --config /path/to/config.toml

# Run in specific mode
postgres-unified-collector --mode otel
postgres-unified-collector --mode nri
postgres-unified-collector --mode hybrid

# Debug mode
postgres-unified-collector --debug

# Dry run (collect but don't send)
postgres-unified-collector --dry-run
```

### NRI Mode (OHI Compatible)

```bash
# Run as New Relic Infrastructure integration
nri-postgresql --metrics

# Run specific metric collection
nri-postgresql --metrics --mode slow_queries
nri-postgresql --metrics --mode wait_events
nri-postgresql --metrics --mode blocking_sessions
```

### OTel Collector Mode

```bash
# Run as OpenTelemetry collector
postgres-otel-collector --config otel-config.toml

# Export to console for debugging
postgres-otel-collector --console

# Override endpoint
postgres-otel-collector --endpoint http://otel-backend:4317
```

## PostgreSQL Setup

### Required Extensions

```sql
-- Required for basic functionality
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Optional for enhanced metrics
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;
CREATE EXTENSION IF NOT EXISTS pg_stat_monitor;

-- Configure pg_stat_statements
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Reload configuration
SELECT pg_reload_conf();
```

### Required Permissions

```sql
-- Create monitoring user
CREATE USER monitoring WITH PASSWORD 'secure_password';

-- Grant necessary permissions
GRANT pg_monitor TO monitoring;
GRANT EXECUTE ON FUNCTION pg_stat_statements_reset() TO monitoring;

-- For each database to monitor
GRANT CONNECT ON DATABASE myapp TO monitoring;
GRANT USAGE ON SCHEMA public TO monitoring;
```

## Deployment Patterns

### 1. Standalone Binary
Best for: Simple deployments, single PostgreSQL instances

```bash
sudo systemctl enable postgres-unified-collector
sudo systemctl start postgres-unified-collector
```

### 2. Container Sidecar
Best for: Kubernetes deployments, containerized PostgreSQL

```yaml
containers:
- name: postgres
  image: postgres:15
- name: collector
  image: postgres-unified-collector:latest
```

### 3. DaemonSet with eBPF
Best for: Cluster-wide monitoring, kernel-level metrics

```bash
kubectl apply -f deployments/kubernetes/daemonset-ebpf.yaml
```

### 4. New Relic Infrastructure Integration
Best for: Existing New Relic Infrastructure deployments

```yaml
integrations:
  - name: nri-postgresql
    env:
      HOSTNAME: postgres.example.com
      PORT: 5432
```

## Monitoring

### Metrics Exported

#### NRI Format (New Relic Infrastructure)
- Event Type: `PostgresSlowQueries`
- Event Type: `PostgresWaitEvents`
- Event Type: `PostgresBlockingSessions`
- Event Type: `PostgresIndividualQueries`
- Event Type: `PostgresExecutionPlanMetrics`

#### OTLP Format (OpenTelemetry)
- Metric: `postgresql.query.duration`
- Metric: `postgresql.query.count`
- Metric: `postgresql.wait.time`
- Metric: `postgresql.locks.blocking.duration`
- Metric: `postgresql.connections.active`

### Dashboards

Sample Grafana dashboards are available in `deployments/docker/grafana/dashboards/`.

## Troubleshooting

### Enable Debug Logging

```bash
# For unified collector
postgres-unified-collector --debug

# For systemd service
sudo systemctl edit postgres-unified-collector
# Add: Environment="RUST_LOG=debug"
```

### Common Issues

1. **pg_stat_statements not available**
   ```sql
   ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
   -- Restart PostgreSQL
   ```

2. **Permission denied errors**
   ```sql
   GRANT pg_monitor TO monitoring_user;
   ```

3. **High memory usage**
   ```toml
   # Reduce sampling rate
   [sampling]
   base_sample_rate = 0.1
   ```

## Migration from OHI

The unified collector is a drop-in replacement for nri-postgresql:

1. **Binary replacement**:
   ```bash
   sudo mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql{,.backup}
   sudo cp nri-postgresql /var/db/newrelic-infra/newrelic-integrations/bin/
   ```

2. **Configuration compatible**: Existing environment variables work as-is

3. **Gradual migration**: Run in shadow mode first to validate metrics

## Development

### Building from Source

```bash
# Full build with all features
cargo build --release --features all

# Minimal build (NRI only)
cargo build --release --features nri

# OTel only
cargo build --release --features otel

# With eBPF support
cargo build --release --features ebpf
```

### Running Tests

```bash
# Unit tests
cargo test

# Integration tests
cargo test --features integration-tests

# OHI compatibility tests
cargo test --test ohi_compatibility
```

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Support

- GitHub Issues: [Report bugs or request features](https://github.com/your-org/postgres-unified-collector/issues)
- Documentation: [Full documentation](https://docs.your-org.com/postgres-collector)
- Community: [Join our Slack](https://slack.your-org.com)