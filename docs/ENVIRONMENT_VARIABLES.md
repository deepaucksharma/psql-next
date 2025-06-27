# Environment Variables Reference

This document provides a comprehensive reference for all environment variables supported by the PostgreSQL Unified Collector.

## Variable Naming Convention

The collector supports environment variables in two formats:

1. **Direct mapping**: Variable names that directly map to configuration fields (e.g., `POSTGRES_HOST`)
2. **Prefixed mapping**: Variables with the `POSTGRES_COLLECTOR_` prefix for namespacing

## Database Connection

### Connection Methods

You can configure database connections using either a connection string or individual parameters:

**Method 1: Connection String (Recommended)**
```bash
POSTGRES_CONNECTION_STRING=postgresql://username:password@host:port/database
```

**Method 2: Individual Parameters**
```bash
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USERNAME=postgres
POSTGRES_PASSWORD=secret
POSTGRES_DATABASE=postgres
```

### Complete Database Configuration

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `POSTGRES_CONNECTION_STRING` | string | `postgresql://postgres:password@localhost:5432/postgres` | Full PostgreSQL connection string |
| `POSTGRES_HOST` | string | `localhost` | PostgreSQL server hostname |
| `POSTGRES_PORT` | integer | `5432` | PostgreSQL server port |
| `POSTGRES_USERNAME` | string | `postgres` | Database username |
| `POSTGRES_PASSWORD` | string | `password` | Database password |
| `POSTGRES_DATABASE` | string | `postgres` | Primary database name |
| `POSTGRES_DATABASES` | string | `postgres` | Comma-separated list of databases to monitor |
| `POSTGRES_MAX_CONNECTIONS` | integer | `10` | Maximum number of database connections |
| `POSTGRES_CONNECT_TIMEOUT_SECS` | integer | `30` | Connection timeout in seconds |

### SSL Configuration

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `POSTGRES_SSLMODE` | string | `prefer` | SSL mode: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` |
| `POSTGRES_SSLCERT` | string | - | Path to client certificate file |
| `POSTGRES_SSLKEY` | string | - | Path to client private key file |
| `POSTGRES_SSLROOTCERT` | string | - | Path to CA certificate file |

## Collection Configuration

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `COLLECTION_INTERVAL_SECS` | integer | `30` | How often to collect metrics (seconds) |
| `COLLECTION_MODE` | string | `hybrid` | Collection mode: `nri`, `otlp`, or `hybrid` |
| `ENABLE_EXTENDED_METRICS` | boolean | `true` | Enable additional metric collection |
| `ENABLE_ASH` | boolean | `true` | Enable Active Session History sampling |
| `ENABLE_EBPF` | boolean | `false` | Enable eBPF-based metrics (Linux only) |

### Query Monitoring

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `QUERY_MONITORING_COUNT_THRESHOLD` | integer | `100` | Minimum execution count for slow query tracking |
| `QUERY_MONITORING_RESPONSE_TIME_THRESHOLD` | integer | `1000` | Minimum response time (ms) for slow query tracking |
| `MAX_SLOW_QUERIES` | integer | `1000` | Maximum number of slow queries to collect |
| `SANITIZE_QUERY_TEXT` | boolean | `true` | Enable query text sanitization |
| `SANITIZATION_MODE` | string | `smart` | Sanitization mode: `none`, `smart`, or `full` |

### Active Session History (ASH)

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `ASH_SAMPLE_INTERVAL_SECS` | integer | `5` | ASH sampling frequency (seconds) |
| `ASH_RETENTION_HOURS` | integer | `24` | How long to retain ASH samples |
| `ASH_MAX_MEMORY_MB` | integer | `100` | Maximum memory for ASH data (MB) |

## New Relic Configuration

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `NEW_RELIC_API_KEY` | string | - | New Relic API key (NRAK-...) |
| `NEW_RELIC_ACCOUNT_ID` | string | - | New Relic account ID |
| `NEW_RELIC_REGION` | string | `US` | New Relic region: `US` or `EU` |
| `NEW_RELIC_LICENSE_KEY` | string | - | New Relic license key (alternative to API key) |

### Regional Endpoints (Auto-configured)

Based on `NEW_RELIC_REGION`, these endpoints are automatically set:

**US Region:**
- `NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318`
- `NEW_RELIC_API_ENDPOINT=https://api.newrelic.com`

**EU Region:**
- `NEW_RELIC_OTLP_ENDPOINT=https://otlp.eu01.nr-data.net:4318`
- `NEW_RELIC_API_ENDPOINT=https://api.eu.newrelic.com`

## Output Configuration

### New Relic Infrastructure (NRI)

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `NRI_ENABLED` | boolean | `true` | Enable NRI output format |
| `NRI_ENTITY_KEY` | string | `${HOSTNAME}:${POSTGRES_PORT}` | Unique entity identifier |
| `NRI_INTEGRATION_NAME` | string | `com.newrelic.postgresql` | Integration name |

### OpenTelemetry Protocol (OTLP)

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `OTLP_ENABLED` | boolean | `true` | Enable OTLP output format |
| `OTLP_ENDPOINT` | string | `${NEW_RELIC_OTLP_ENDPOINT}` | OTLP endpoint URL |
| `OTLP_PROTOCOL` | string | `http` | Protocol: `http` or `grpc` |
| `OTLP_COMPRESSION` | string | `gzip` | Compression: `none`, `gzip`, or `deflate` |
| `OTLP_TIMEOUT_SECS` | integer | `30` | Request timeout (seconds) |
| `OTLP_HEADERS` | string | `Api-Key=${NEW_RELIC_API_KEY}` | Headers for authentication |

## PgBouncer Integration

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `PGBOUNCER_ENABLED` | boolean | `false` | Enable PgBouncer monitoring |
| `PGBOUNCER_ADMIN_CONNECTION_STRING` | string | - | PgBouncer admin connection string |
| `PGBOUNCER_COLLECTION_INTERVAL_SECS` | integer | `30` | PgBouncer collection frequency |

## Health and Monitoring

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `HEALTH_ENABLED` | boolean | `true` | Enable health check endpoints |
| `HEALTH_BIND_ADDRESS` | string | `0.0.0.0:8080` | Health endpoint bind address |
| `HEALTH_INCLUDE_METRICS` | boolean | `true` | Include Prometheus metrics |

## Logging Configuration

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `LOG_LEVEL` | string | `info` | Log level: `trace`, `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | string | `json` | Log format: `json` or `pretty` |
| `LOG_OUTPUT` | string | `stdout` | Log output: `stdout`, `stderr`, or file path |

## Performance and Security

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `MAX_MEMORY_MB` | integer | `500` | Maximum memory usage (MB) |
| `WORKER_THREADS` | integer | `0` | Number of async worker threads (0 = auto) |
| `BLOCKING_THREADS` | integer | `4` | Number of blocking operation threads |
| `REQUIRE_SSL` | boolean | `false` | Require SSL for database connections |
| `VALIDATE_CERTIFICATES` | boolean | `true` | Validate SSL certificates |
| `MASK_CREDENTIALS_IN_LOGS` | boolean | `true` | Mask passwords in log output |

## Environment Identification

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `ENVIRONMENT` | string | `development` | Environment name: `development`, `staging`, `production` |
| `SERVICE_NAME` | string | `postgres-collector` | Service name for telemetry |
| `SERVICE_VERSION` | string | `1.0.0` | Service version for telemetry |
| `DEPLOYMENT_REGION` | string | - | Deployment region identifier |

## Container/Kubernetes Specific

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `HOSTNAME` | string | - | Host/container name |
| `POD_NAME` | string | - | Kubernetes pod name |
| `NODE_NAME` | string | - | Kubernetes node name |
| `NAMESPACE` | string | - | Kubernetes namespace |

## Development and Debugging

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `ENABLE_DEBUG_ENDPOINTS` | boolean | `false` | Enable debug HTTP endpoints |
| `ENABLE_QUERY_LOGGING` | boolean | `false` | Log all executed SQL queries |
| `ENABLE_METRICS_DUMP` | boolean | `false` | Dump collected metrics to file |

## Feature Flags

### Experimental Features

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `EXPERIMENTAL_QUERY_OPTIMIZATION` | boolean | `false` | Experimental query optimizations |
| `EXPERIMENTAL_COMPRESSION` | boolean | `false` | Experimental compression algorithms |
| `EXPERIMENTAL_CACHING` | boolean | `false` | Experimental result caching |

### Beta Features

| Environment Variable | Type | Default | Description |
|---------------------|------|---------|-------------|
| `BETA_ENHANCED_METRICS` | boolean | `false` | Enhanced metric collection |
| `BETA_SMART_SAMPLING` | boolean | `false` | Intelligent metric sampling |
| `BETA_ADAPTIVE_INTERVALS` | boolean | `false` | Adaptive collection intervals |

## Configuration Precedence

Environment variables are loaded in the following order (later sources override earlier ones):

1. **Default values** (built into the application)
2. **Configuration file** (TOML format)
3. **Unprefixed environment variables** (e.g., `POSTGRES_HOST`)
4. **Prefixed environment variables** (e.g., `POSTGRES_COLLECTOR_HOST`)

## Examples

### Basic Development Setup

```bash
# Database connection
export POSTGRES_CONNECTION_STRING="postgresql://postgres:secret@localhost:5432/myapp"

# New Relic credentials
export NEW_RELIC_API_KEY="NRAK-..."
export NEW_RELIC_ACCOUNT_ID="1234567"
export NEW_RELIC_REGION="US"

# Collection settings
export COLLECTION_INTERVAL_SECS=30
export COLLECTION_MODE=hybrid
export ENABLE_EXTENDED_METRICS=true
```

### Production Container Setup

```bash
# Database connection with SSL
export POSTGRES_CONNECTION_STRING="postgresql://collector:${DB_PASSWORD}@prod-db.example.com:5432/analytics?sslmode=require"

# New Relic production settings
export NEW_RELIC_API_KEY="${NRQL_API_KEY}"
export NEW_RELIC_ACCOUNT_ID="${NR_ACCOUNT_ID}"
export NEW_RELIC_REGION="US"

# Production collection settings
export COLLECTION_INTERVAL_SECS=60
export COLLECTION_MODE=hybrid
export ENABLE_EXTENDED_METRICS=true
export SANITIZE_QUERY_TEXT=true
export SANITIZATION_MODE=smart

# Performance settings
export MAX_MEMORY_MB=200
export WORKER_THREADS=4

# Security settings
export REQUIRE_SSL=true
export VALIDATE_CERTIFICATES=true
export MASK_CREDENTIALS_IN_LOGS=true

# Environment identification
export ENVIRONMENT=production
export SERVICE_VERSION="${BUILD_VERSION}"
export DEPLOYMENT_REGION="us-east-1"
```

### Multi-Instance Setup

```bash
# Primary instance
export POSTGRES_CONNECTION_STRING="postgresql://collector:${PRIMARY_DB_PASSWORD}@primary-db:5432/app"

# Multiple databases
export POSTGRES_DATABASES="app,analytics,reporting"

# Instance identification
export SERVICE_NAME="postgres-collector-primary"
export ENVIRONMENT=production
```

## Validation

The collector validates environment variables on startup and will fail with descriptive error messages if:

- Required variables are missing
- Values are outside valid ranges
- Invalid enum values are provided
- Connection strings are malformed

## Troubleshooting

### Common Issues

1. **Connection refused**: Check `POSTGRES_HOST`, `POSTGRES_PORT`, and network connectivity
2. **Authentication failed**: Verify `POSTGRES_USERNAME` and `POSTGRES_PASSWORD`
3. **SSL errors**: Check `POSTGRES_SSLMODE` and certificate paths
4. **Permission denied**: Ensure database user has required permissions
5. **Invalid region**: Verify `NEW_RELIC_REGION` is "US" or "EU"

### Debug Mode

Enable debug logging to see how environment variables are resolved:

```bash
export LOG_LEVEL=debug
export ENABLE_DEBUG_ENDPOINTS=true
./postgres-collector
```

### Variable Resolution Check

Use the debug endpoint to see current configuration:

```bash
curl http://localhost:8080/debug/config
```