# Canary Tester Module

A synthetic monitoring and availability testing module for MySQL databases using OpenTelemetry.

## Overview

The Canary Tester module performs continuous synthetic monitoring of MySQL databases by executing predefined test queries and measuring their performance. It generates Service Level Indicator (SLI) metrics to help monitor database availability, performance, and reliability.

## Features

- **Synthetic Query Execution**: Runs a comprehensive suite of test queries
- **Performance Monitoring**: Measures query response times and latency
- **Availability Testing**: Continuous health checks and availability monitoring
- **SLI Metrics Generation**: Produces metrics for SLOs and alerting
- **OpenTelemetry Integration**: Full observability with OTLP export
- **Prometheus Metrics**: Exposes metrics in Prometheus format

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Canary Tester  │────▶│      MySQL       │     │ OTEL Collector  │
│   (Port 8090)   │     │   (Port 3306)    │     │  (Port 4317)    │
└─────────────────┘     └──────────────────┘     └─────────────────┘
         │                                                  │
         └──────────────── Metrics ────────────────────────┘
```

## Quick Start

```bash
# Initialize and build
make init
make build

# Run the canary tester
make run

# Check health status
make health

# View metrics
make metrics

# Stop services
make stop
```

## Configuration

### Environment Variables

- `MYSQL_HOST`: MySQL host (default: mysql)
- `MYSQL_PORT`: MySQL port (default: 3306)
- `MYSQL_USER`: MySQL user (default: root)
- `MYSQL_PASSWORD`: MySQL password
- `MYSQL_DATABASE`: Target database (default: test)
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OTLP endpoint
- `CANARY_INTERVAL`: Test execution interval (default: 30s)
- `CANARY_TIMEOUT`: Query timeout (default: 10s)

### Ports

- **8090**: Canary tester metrics endpoint
- **3306**: MySQL database
- **4317**: OTLP gRPC receiver
- **4318**: OTLP HTTP receiver
- **8888**: Prometheus metrics endpoint

## Test Queries

The module executes various test queries to measure different aspects:

1. **Connectivity Tests**: Basic ping and timestamp queries
2. **CRUD Operations**: Insert, select, update, delete performance
3. **Complex Queries**: Joins, aggregations, and index scans
4. **Transaction Tests**: Transaction commit latency
5. **Metadata Queries**: Schema and table information
6. **Maintenance Operations**: Cleanup and table analysis

## Metrics

### Canary Metrics

- `canary_query_duration_seconds`: Query execution time histogram
- `canary_query_errors_total`: Total query errors counter
- `canary_query_success_total`: Successful queries counter
- `canary_availability`: Database availability gauge (0/1)
- `canary_connection_errors_total`: Connection error counter

### MySQL Metrics (via receiver)

- `mysql_uptime`: Database uptime
- `mysql_connections`: Active connections
- `mysql_operations`: Operation counts
- `mysql_query_count`: Query statistics
- `mysql_query_slow_count`: Slow query count

## Usage Examples

### View Recent Test Results

```bash
make results
```

### Monitor Performance

```bash
make monitor
```

### Export Metrics

```bash
make export-metrics
```

### Connect to MySQL

```bash
make mysql-cli
```

### View Logs

```bash
# All services
make logs

# Specific service
make logs-canary
make logs-mysql
make logs-collector
```

## Development

### Adding New Test Queries

1. Edit `src/canary-queries.sql`
2. Add query with metadata comments:
   ```sql
   -- Name: query_name
   -- Description: What this query tests
   -- SLI: metric_category
   SELECT ...;
   ```

### Building Locally

```bash
# Initialize Go module
make init

# Build and run in dev mode
make dev
```

### Testing

```bash
# Run canary tests manually
make test

# Validate collector config
make validate
```

## Monitoring Dashboard

Access the following endpoints:

- **Metrics**: http://localhost:8090/metrics
- **Prometheus**: http://localhost:8888/metrics
- **Health Check**: http://localhost:13133/health

## Troubleshooting

### Common Issues

1. **MySQL Connection Failed**
   ```bash
   # Check MySQL status
   make health
   
   # View MySQL logs
   make logs-mysql
   ```

2. **No Metrics Available**
   ```bash
   # Check collector status
   make logs-collector
   
   # Verify configuration
   make show-config
   ```

3. **High Query Latency**
   ```bash
   # Monitor performance
   make monitor
   
   # Check slow queries
   make mysql-cli
   SHOW PROCESSLIST;
   ```

## Maintenance

### Cleanup Old Data

The canary tester automatically cleans up test data older than 24 hours. Manual cleanup:

```bash
make mysql-cli
DELETE FROM canary_test WHERE created_at < DATE_SUB(NOW(), INTERVAL 1 DAY);
```

### Restart Services

```bash
make restart-canary
make restart-collector
make restart-mysql
```

## Integration

### With Prometheus

Add to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'canary-tester'
    static_configs:
      - targets: ['localhost:8090']
  
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['localhost:8888']
```

### With Grafana

Import the included dashboard or create custom dashboards using the exposed metrics.

## License

See the main repository license.