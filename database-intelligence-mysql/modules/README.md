# MySQL OpenTelemetry Collector Modules

This directory contains modular docker-compose configurations for different aspects of MySQL monitoring. Each module can be run independently or together for comprehensive monitoring.

## Available Modules

### 1. Core Metrics Module
- **Location**: `core-metrics/`
- **Purpose**: Basic MySQL and host metrics collection
- **Resource Usage**: Low (1 CPU, 750MB memory)
- **Use Case**: Essential monitoring for all deployments

### 2. SQL Intelligence Module
- **Location**: `sql-intelligence/`
- **Purpose**: Deep query analysis and performance intelligence
- **Resource Usage**: High (4 CPUs, 3.5GB memory)
- **Use Case**: Performance troubleshooting and optimization

### 3. Replication Monitor Module
- **Location**: `replication-monitor/`
- **Purpose**: MySQL replication lag and health monitoring
- **Resource Usage**: Low (1 CPU, 750MB memory)
- **Use Case**: Multi-instance MySQL deployments

### 4. Canary Tester Module
- **Location**: `canary-tester/`
- **Purpose**: Synthetic monitoring and baseline establishment
- **Resource Usage**: Minimal (1 CPU, 500MB memory)
- **Use Case**: Proactive monitoring and SLA validation

### 5. Cross-Signal Correlator Module
- **Location**: `cross-signal-correlator/`
- **Purpose**: Correlate traces, logs, and metrics
- **Resource Usage**: High (4 CPUs, 3.5GB memory)
- **Use Case**: Full observability with distributed tracing

## Using Enhanced Configurations

Each module supports both standard and enhanced configurations:

```bash
# Use standard configuration (default)
docker-compose up -d

# Use enhanced configuration
COLLECTOR_CONFIG=collector-enhanced.yaml docker-compose up -d
```

## Environment Variables

### Common Variables (All Modules)
```bash
# MySQL Connection
MYSQL_PRIMARY_ENDPOINT=mysql-primary:3306
MYSQL_REPLICA_ENDPOINT=mysql-replica:3306
MYSQL_USER=otel_monitor
MYSQL_PASSWORD=otelmonitorpass

# New Relic
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_ACCOUNT_ID=your_account_id

# Environment
ENVIRONMENT=production
CLUSTER_NAME=mysql-cluster
```

### Module-Specific Variables

#### Core Metrics
```bash
MYSQL_COLLECTION_INTERVAL=10s
HOST_COLLECTION_INTERVAL=10s
```

#### SQL Intelligence
```bash
SQL_INTELLIGENCE_INTERVAL=5s
ML_FEATURES_ENABLED=true
BUSINESS_CONTEXT_ENABLED=true
```

#### Replication Monitor
```bash
REPLICATION_LAG_ALERT_THRESHOLD=30
REPLICATION_LAG_CRITICAL_THRESHOLD=60
```

#### Canary Tester
```bash
CANARY_INTERVAL=60s
CANARY_LATENCY_THRESHOLD_MS=100
```

#### Cross-Signal Correlator
```bash
ENABLE_TRACE_CORRELATION=true
ENABLE_LOG_CORRELATION=true
ENABLE_METRIC_EXEMPLARS=true
```

## Deployment Patterns

### 1. Minimal Deployment
```bash
# Just core metrics
cd modules/core-metrics
docker-compose up -d
```

### 2. Standard Deployment
```bash
# Core metrics + replication monitoring
cd modules
docker-compose -f core-metrics/docker-compose.yaml \
              -f replication-monitor/docker-compose.yaml \
              up -d
```

### 3. Advanced Deployment
```bash
# All intelligence features
cd modules
docker-compose -f core-metrics/docker-compose.yaml \
              -f sql-intelligence/docker-compose.yaml \
              -f replication-monitor/docker-compose.yaml \
              -f canary-tester/docker-compose.yaml \
              up -d
```

### 4. Full Observability
```bash
# Everything including cross-signal correlation
cd modules
docker-compose -f core-metrics/docker-compose.yaml \
              -f sql-intelligence/docker-compose.yaml \
              -f replication-monitor/docker-compose.yaml \
              -f canary-tester/docker-compose.yaml \
              -f cross-signal-correlator/docker-compose.yaml \
              up -d
```

## Network Requirements

All modules expect an external network named `mysql-monitoring-network`. Create it before starting any modules:

```bash
docker network create mysql-monitoring-network
```

## Storage Volumes

Each module uses its own storage volume for persistent queues:
- `otel_core_metrics_storage`
- `otel_sql_intelligence_storage`
- `otel_replication_monitor_storage`
- `otel_canary_tester_storage`
- `otel_cross_signal_storage`

## Health Checks

Each module exposes a health endpoint on different ports:
- Core Metrics: `http://localhost:13133/health`
- SQL Intelligence: `http://localhost:13134/health`
- Replication Monitor: `http://localhost:13135/health`
- Canary Tester: `http://localhost:13136/health`
- Cross-Signal Correlator: `http://localhost:13137/health`

## Monitoring the Collectors

Each collector exposes metrics about itself:
- Core Metrics: `http://localhost:8888/metrics`
- SQL Intelligence: `http://localhost:8889/metrics`
- Replication Monitor: `http://localhost:8890/metrics`
- Canary Tester: `http://localhost:8891/metrics`
- Cross-Signal Correlator: `http://localhost:8892/metrics`

## Troubleshooting

1. **Check collector logs**:
   ```bash
   docker logs otel-collector-<module-name>
   ```

2. **Verify configuration**:
   ```bash
   docker exec otel-collector-<module-name> /otelcol validate --config=/etc/otel-collector-config.yaml
   ```

3. **Enable debug logging**:
   ```bash
   LOG_LEVEL=debug docker-compose up
   ```

4. **Use pprof for performance profiling** (SQL Intelligence and Cross-Signal Correlator only):
   - SQL Intelligence: `http://localhost:1778/debug/pprof/`
   - Cross-Signal Correlator: `http://localhost:1779/debug/pprof/`