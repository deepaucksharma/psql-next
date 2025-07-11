# MySQL OpenTelemetry Monitoring with New Relic

A production-ready MySQL monitoring solution using OpenTelemetry collectors with deep New Relic integration. This project provides comprehensive MySQL 8.x performance insights through metrics collection, designed specifically for New Relic's observability platform.

## Features

- **Complete MySQL Metrics**: 40+ MySQL metrics including connections, queries, InnoDB, replication, and table I/O
- **OpenTelemetry Native**: Built on OTel standards for future-proof observability
- **New Relic Optimized**: Deep integration with New Relic's OTLP endpoint
- **Docker Compose Setup**: Easy local deployment with MySQL primary/replica configuration
- **Auto-instrumentation**: Automatic Performance Schema configuration
- **Sample Application**: Traffic generator for testing and demonstration
- **Production Ready**: Includes health checks, resource limits, and error handling

## Quick Start

### Prerequisites

- Docker and Docker Compose V2
- New Relic account with API key
- 4GB RAM minimum (for running all services)

### Setup

1. Clone the repository:
```bash
git clone <repository-url>
cd database-intelligence-mysql
```

2. Copy environment file and configure:
```bash
cp .env.example .env
```

3. Edit `.env` and add your New Relic credentials:
```env
NEW_RELIC_API_KEY=your_new_relic_ingest_license_key_here
NEW_RELIC_ACCOUNT_ID=your_new_relic_account_id_here
```

4. Run the setup script:
```bash
./scripts/setup.sh
```

This will:
- Start MySQL primary and replica instances
- Configure replication
- Create monitoring user
- Start OpenTelemetry collector
- Begin sending metrics to New Relic

### Verify Setup

Test connections and monitoring:
```bash
./scripts/test-connection.sh
```

Generate sample traffic:
```bash
./scripts/generate-load.sh
```

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  MySQL Primary  │     │  MySQL Replica  │     │   Sample App    │
│    Port 3306    │────▶│    Port 3307    │     │ (Traffic Gen)   │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                         │
         │                       │                         │
         └───────────┬───────────┘                         │
                     │                                     │
                     ▼                                     ▼
           ┌─────────────────────┐                ┌──────────────┐
           │   OTel Collector    │                │    MySQL     │
           │  - MySQL Receiver   │◀───────────────│  Primary DB  │
           │  - Batch Processor  │                └──────────────┘
           │  - Resource Attrs   │
           └─────────┬───────────┘
                     │
                     ▼
            ┌────────────────────┐
            │    New Relic       │
            │  OTLP Endpoint     │
            │  otlp.nr-data.net  │
            └────────────────────┘
```

## Collected Metrics

### Connection Metrics
- `mysql.connection.count` - Total connections created
- `mysql.connection.errors` - Failed connection attempts  
- `mysql.threads` - Current connected threads
- `mysql.connection.max` - Maximum allowed connections

### Query Performance
- `mysql.query.count` - Total queries executed
- `mysql.query.slow.count` - Slow queries
- `mysql.query.client.count` - Client-initiated queries
- `mysql.statement_event.count` - Statement executions by digest
- `mysql.statement_event.wait.time` - Statement execution time

### InnoDB Metrics
- `mysql.buffer_pool.usage` - Buffer pool memory usage
- `mysql.buffer_pool.limit` - Buffer pool size limit
- `mysql.buffer_pool.operations` - Read/write operations
- `mysql.innodb.row_operations` - Row-level operations
- `mysql.innodb.row_lock_waits` - Row lock wait events
- `mysql.innodb.pages_created/read/written` - Page operations

### Replication Metrics
- `mysql.replica.time_behind_source` - Replication lag in seconds
- `mysql.replica.sql_delay` - SQL thread delay

### Table/Index I/O
- `mysql.table.io.wait.count` - Table I/O wait events
- `mysql.table.io.wait.time` - Table I/O wait time
- `mysql.index.io.wait.count` - Index I/O wait events
- `mysql.table.lock_wait.time` - Table lock wait time

## Configuration

### OpenTelemetry Collector

The collector configuration is in `otel/config/otel-collector-config.yaml`:

```yaml
receivers:
  mysql/primary:
    endpoint: ${env:MYSQL_PRIMARY_ENDPOINT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    collection_interval: 10s
    
processors:
  batch:
    send_batch_size: 1000
    timeout: 10s
    
exporters:
  otlp/newrelic:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_API_KEY}
```

### MySQL Configuration

Custom MySQL configurations are in:
- `mysql/conf/primary.cnf` - Primary instance settings
- `mysql/conf/replica.cnf` - Replica instance settings

Key settings:
- Performance Schema enabled
- Binary logging for replication
- Slow query log enabled
- InnoDB monitoring enabled

## New Relic Integration

### Viewing Metrics

1. Log into your New Relic account
2. Navigate to **Explorer** → **Metrics**
3. Filter by `instrumentation.provider = 'opentelemetry'`
4. Look for metrics prefixed with `mysql.`

### Creating Dashboards

Import the provided dashboard:
1. Go to **Dashboards** → **Import dashboard**
2. Upload `dashboards/newrelic/mysql-dashboard.json`

Or create custom queries:
```sql
SELECT rate(sum(mysql.query.count), 1 minute) as 'QPS' 
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
FACET mysql.instance.endpoint 
TIMESERIES
```

### Setting Up Alerts

Example alert conditions are provided in `dashboards/newrelic/alerts.yaml`:
- Connection saturation
- Replication lag
- Slow query rate
- Buffer pool efficiency

## Troubleshooting

### No metrics in New Relic

1. Check collector logs:
```bash
docker compose logs -f otel-collector
```

2. Verify API key:
```bash
docker compose exec otel-collector env | grep NEW_RELIC
```

3. Test collector health:
```bash
curl http://localhost:13133/
```

### MySQL connection errors

1. Check MySQL is running:
```bash
docker compose ps
```

2. Verify monitoring user:
```bash
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SHOW GRANTS;"
```

### High memory usage

Adjust collector limits in `docker-compose.yml`:
```yaml
environment:
  GOMEMLIMIT: "1750MiB"
```

## Advanced Configuration

### Custom Metrics

Add custom MySQL queries to the receiver:
```yaml
receivers:
  mysql/custom:
    queries:
      - query: "SELECT COUNT(*) as value FROM custom_table"
        metric_name: "mysql.custom.table.count"
        value_column: "value"
```

### Sampling High-Cardinality Metrics

Configure sampling in the transform processor:
```yaml
processors:
  transform:
    metric_statements:
      - context: datapoint
        statements:
          - limit(attributes["table"], 50) where metric.name == "mysql.table.io.wait.count"
```

### Multi-Environment Setup

Use different `.env` files:
```bash
docker compose --env-file .env.production up -d
```

## Performance Considerations

- The collector uses ~200MB RAM under normal load
- MySQL Performance Schema adds ~5-10% overhead
- Network latency to New Relic affects batch sizes
- Consider sampling for environments with >1000 tables

## Security

- All credentials are stored in environment variables
- MySQL monitoring user has minimal required privileges
- TLS can be enabled for MySQL connections
- New Relic API key should be kept secure

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `./scripts/test-connection.sh`
5. Submit a pull request

## License

This project is licensed under the MIT License - see LICENSE file for details.

## Support

- New Relic Documentation: https://docs.newrelic.com/
- OpenTelemetry MySQL Receiver: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/mysqlreceiver
- Issues: Create an issue in this repository