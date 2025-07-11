# Environment Variables Reference

This document provides a comprehensive reference for all environment variables used by the Database Intelligence OpenTelemetry Collector.

## Naming Convention

All environment variables follow a consistent naming pattern:
- Database-specific variables use prefixes: `DB_POSTGRES_*` and `DB_MYSQL_*`
- Service configuration uses no prefix: `SERVICE_NAME`, `DEPLOYMENT_ENVIRONMENT`
- New Relic specific: `NEW_RELIC_*`
- OpenTelemetry specific: `OTLP_*`

## Core Service Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `SERVICE_NAME` | Name of the service for identification | `database-intelligence-collector` | No |
| `SERVICE_VERSION` | Version of the collector | `2.0.0` | No |
| `DEPLOYMENT_ENVIRONMENT` | Environment name (development/staging/production) | `production` | No |
| `HOSTNAME` | Hostname for host.name attribute | System hostname | No |

## PostgreSQL Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DB_POSTGRES_HOST` | PostgreSQL server hostname | `localhost` | Yes |
| `DB_POSTGRES_PORT` | PostgreSQL server port | `5432` | No |
| `DB_POSTGRES_USER` | PostgreSQL username | `postgres` | Yes |
| `DB_POSTGRES_PASSWORD` | PostgreSQL password | `postgres` | Yes |
| `DB_POSTGRES_DATABASE` | PostgreSQL database name | `postgres` | No |
| `POSTGRES_COLLECTION_INTERVAL` | Metrics collection interval | `60s` | No |
| `POSTGRES_QUERY_INTERVAL` | Custom query interval | `300s` | No |

### PostgreSQL TLS Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `POSTGRES_TLS_INSECURE` | Skip TLS verification | `true` | No |
| `POSTGRES_SSLMODE` | SSL mode (disable/require/verify-ca/verify-full) | `disable` | No |
| `POSTGRES_CA_FILE` | Path to CA certificate | - | No |
| `POSTGRES_CERT_FILE` | Path to client certificate | - | No |
| `POSTGRES_KEY_FILE` | Path to client key | - | No |

## MySQL Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DB_MYSQL_HOST` | MySQL server hostname | `localhost` | Yes |
| `DB_MYSQL_PORT` | MySQL server port | `3306` | No |
| `DB_MYSQL_USER` | MySQL username | `root` | Yes |
| `DB_MYSQL_PASSWORD` | MySQL password | `mysql` | Yes |
| `DB_MYSQL_DATABASE` | MySQL database name | `mysql` | No |
| `MYSQL_COLLECTION_INTERVAL` | Metrics collection interval | `60s` | No |
| `MYSQL_QUERY_INTERVAL` | Custom query interval | `300s` | No |

### MySQL TLS Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MYSQL_TLS_INSECURE` | Skip TLS verification | `true` | No |
| `MYSQL_CA_FILE` | Path to CA certificate | - | No |

## OTLP Export Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `OTLP_ENDPOINT` | OTLP endpoint URL | `https://otlp.nr-data.net` | No |
| `NEW_RELIC_LICENSE_KEY` | New Relic license key for OTLP | - | **Yes** |
| `NEW_RELIC_ACCOUNT_ID` | New Relic account ID | - | No |
| `OTLP_TIMEOUT` | Export timeout | `30s` | No |
| `OTLP_RETRY_INITIAL_INTERVAL` | Initial retry interval | `5s` | No |
| `OTLP_RETRY_MAX_INTERVAL` | Maximum retry interval | `30s` | No |
| `OTLP_RETRY_MAX_ELAPSED_TIME` | Maximum retry duration | `300s` | No |
| `OTLP_QUEUE_CONSUMERS` | Number of queue consumers | `10` | No |
| `OTLP_QUEUE_SIZE` | Export queue size | `5000` | No |

## OTLP Receiver Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `OTLP_GRPC_ENDPOINT` | OTLP gRPC receiver endpoint | `0.0.0.0:4317` | No |
| `OTLP_HTTP_ENDPOINT` | OTLP HTTP receiver endpoint | `0.0.0.0:4318` | No |
| `OTLP_MAX_RECV_MSG_SIZE` | Max receive message size (MiB) | `32` | No |
| `OTLP_MAX_REQUEST_BODY_SIZE` | Max HTTP body size (MiB) | `32` | No |

## Resource Management

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MEMORY_LIMIT_MIB` | Memory limit in MiB | `512` | No |
| `MEMORY_SPIKE_LIMIT_MIB` | Memory spike limit in MiB | `128` | No |
| `MEMORY_CHECK_INTERVAL` | Memory check interval | `1s` | No |
| `MEMORY_BALLAST_SIZE_MIB` | Memory ballast size | `64` | No |

## Batch Processing

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `BATCH_TIMEOUT` | Batch timeout | `1s` | No |
| `BATCH_SIZE` | Batch size | `1024` | No |
| `BATCH_MAX_SIZE` | Maximum batch size | `2048` | No |

## Custom Processors Configuration

### Adaptive Sampler
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ADAPTIVE_SAMPLING_PERCENTAGE` | Base sampling percentage | `10` | No |
| `MAX_TRACES_PER_SECOND` | Max traces per second | `100` | No |
| `SAMPLED_CACHE_SIZE` | Sampled items cache size | `100000` | No |
| `SELECT_SAMPLING_PERCENTAGE` | SELECT query sampling % | `5` | No |
| `DML_SAMPLING_PERCENTAGE` | DML query sampling % | `50` | No |
| `AUDIT_SAMPLING_PERCENTAGE` | Audit query sampling % | `1` | No |

### Circuit Breaker
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `CIRCUIT_BREAKER_MAX_FAILURES` | Max consecutive failures | `5` | No |
| `CIRCUIT_BREAKER_FAILURE_THRESHOLD` | Failure threshold % | `50` | No |
| `CIRCUIT_BREAKER_TIMEOUT` | Circuit breaker timeout | `30s` | No |
| `CIRCUIT_BREAKER_RECOVERY_TIMEOUT` | Recovery timeout | `60s` | No |
| `PER_DATABASE_CIRCUIT` | Per-database circuits | `true` | No |
| `HEALTH_CHECK_INTERVAL` | Health check interval | `10s` | No |

### Cost Control
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DAILY_BUDGET_USD` | Daily budget in USD | `100` | No |
| `MONTHLY_BUDGET_USD` | Monthly budget in USD | `3000` | No |
| `COST_PER_GB` | Cost per GB of data | `0.25` | No |
| `COST_PER_MILLION_EVENTS` | Cost per million events | `2.00` | No |
| `COST_ALERT_THRESHOLD` | Cost alert threshold % | `80` | No |
| `COST_ENFORCEMENT_ENABLED` | Enable cost enforcement | `false` | No |

### Plan Analysis
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ENABLE_PLAN_COLLECTION` | Enable plan collection | `false` | No |
| `PLAN_CACHE_SIZE` | Plan cache size | `1000` | No |
| `PLAN_CACHE_ENABLED` | Enable plan cache | `true` | No |
| `PLAN_CACHE_TTL` | Plan cache TTL | `3600s` | No |
| `ENABLE_ANONYMIZATION` | Enable query anonymization | `true` | No |
| `MAX_QUERY_LENGTH` | Max query length | `4096` | No |

### Data Verification
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ENABLE_PII_DETECTION` | Enable PII detection | `true` | No |
| `ENABLE_DATA_VALIDATION` | Enable data validation | `true` | No |
| `MAX_FIELD_LENGTH` | Max field length | `1000` | No |
| `VERIFICATION_SAMPLE_RATE` | Verification sample rate | `0.1` | No |

### Query Correlation
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `CORRELATION_WINDOW` | Correlation time window | `30s` | No |
| `MAX_CORRELATIONS` | Max correlations to track | `1000` | No |
| `ENABLE_TRACE_CORRELATION` | Enable trace correlation | `true` | No |

### Error Monitoring
| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NR_ERROR_THRESHOLD` | Error threshold | `10` | No |
| `NR_VALIDATION_INTERVAL` | Validation interval | `300s` | No |
| `ENABLE_NR_VALIDATION` | Enable NR validation | `true` | No |

## Monitoring Endpoints

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PROMETHEUS_ENDPOINT` | Prometheus metrics endpoint | `0.0.0.0:8889` | No |
| `PROMETHEUS_NAMESPACE` | Prometheus namespace | `database_intelligence` | No |
| `HEALTH_CHECK_ENDPOINT` | Health check endpoint | `0.0.0.0:13133` | No |
| `PPROF_ENDPOINT` | pprof debug endpoint | `0.0.0.0:1777` | No |
| `ZPAGES_ENDPOINT` | zPages debug endpoint | `0.0.0.0:55679` | No |

## Logging Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` | No |
| `LOG_DEVELOPMENT` | Development logging mode | `false` | No |
| `LOG_ENCODING` | Log encoding (json/console) | `json` | No |
| `DEBUG_VERBOSITY` | Debug verbosity (basic/normal/detailed) | `normal` | No |

## Telemetry Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `TELEMETRY_METRICS_LEVEL` | Metrics level (none/basic/normal/detailed) | `basic` | No |
| `TELEMETRY_METRICS_ADDRESS` | Metrics endpoint address | `0.0.0.0:8888` | No |

## File Storage (Persistence)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `FILE_STORAGE_DIRECTORY` | Storage directory | `/tmp/otel-storage` | No |
| `FILE_STORAGE_TIMEOUT` | Storage timeout | `1s` | No |
| `FILE_STORAGE_COMPACTION_DIRECTORY` | Compaction directory | `/tmp/otel-storage-compaction` | No |

## Usage Examples

### Basic Setup (.env file)
```bash
# Required
NEW_RELIC_LICENSE_KEY=your-license-key-here
DB_POSTGRES_HOST=postgres.example.com
DB_POSTGRES_USER=dbuser
DB_POSTGRES_PASSWORD=dbpass
DB_MYSQL_HOST=mysql.example.com
DB_MYSQL_USER=dbuser
DB_MYSQL_PASSWORD=dbpass

# Optional
SERVICE_NAME=my-database-monitor
DEPLOYMENT_ENVIRONMENT=production
MEMORY_LIMIT_MIB=1024
```

### Docker Compose
```yaml
environment:
  - NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
  - DB_POSTGRES_HOST=postgres
  - DB_POSTGRES_USER=${DB_POSTGRES_USER:-postgres}
  - DB_POSTGRES_PASSWORD=${DB_POSTGRES_PASSWORD}
```

### Kubernetes ConfigMap
```yaml
data:
  DB_POSTGRES_HOST: "postgres-service"
  DB_POSTGRES_PORT: "5432"
  SERVICE_NAME: "database-intelligence"
  DEPLOYMENT_ENVIRONMENT: "production"
```

### Kubernetes Secret
```yaml
stringData:
  NEW_RELIC_LICENSE_KEY: "your-license-key"
  DB_POSTGRES_PASSWORD: "secure-password"
  DB_MYSQL_PASSWORD: "secure-password"
```

## Migration Guide

If migrating from older configurations:

1. Replace `DB_USERNAME` → `DB_POSTGRES_USER` and `DB_MYSQL_USER`
2. Replace `DB_PASSWORD` → `DB_POSTGRES_PASSWORD` and `DB_MYSQL_PASSWORD`
3. Replace `DB_NAME` → `DB_POSTGRES_DATABASE` and `DB_MYSQL_DATABASE`
4. Replace `NEW_RELIC_API_KEY` → `NEW_RELIC_LICENSE_KEY`
5. Replace `NEW_RELIC_OTLP_ENDPOINT` → `OTLP_ENDPOINT`

## Best Practices

1. **Security**: Never commit `.env` files with real credentials
2. **Defaults**: Use defaults appropriate for your environment
3. **Validation**: Test configuration before production deployment
4. **Documentation**: Document any custom environment variables
5. **Consistency**: Use the same variable names across all deployment methods