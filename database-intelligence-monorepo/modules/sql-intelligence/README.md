# SQL Intelligence Module

Advanced SQL query analysis and performance intelligence for MySQL.

## Features

- **Query Performance Analysis**
  - Slow query detection and analysis
  - Query digest tracking
  - Execution count and latency metrics
  - Index usage analysis

- **Table I/O Statistics**
  - Read/write latency per table
  - I/O operation counts
  - Hot table identification

- **Index Analysis**
  - Index cardinality tracking
  - Unused index detection
  - Index recommendation data

- **Query Classification**
  - Automatic query type detection (SELECT, INSERT, UPDATE, DELETE)
  - Optimization need flagging
  - Business impact correlation (when integrated)

## Quick Start

```bash
# Build the module
make build

# Run standalone
make run

# Run with core-metrics integration
make run-with-core

# View logs
make logs

# Check metrics
curl http://localhost:8082/metrics | grep mysql_query

# Stop the module
make stop
```

## Configuration

Environment variables:

- `MYSQL_ENDPOINT`: MySQL connection endpoint (default: mysql-test:3306)
- `MYSQL_USER`: MySQL username (default: root)
- `MYSQL_PASSWORD`: MySQL password (default: test)
- `EXPORT_PORT`: Prometheus metrics port (default: 8082)
- `METRICS_ENDPOINT`: Optional core-metrics endpoint for federation

## Metrics Exposed

### Query Metrics
- `mysql_query_exec_count`: Query execution count by digest
- `mysql_query_latency_total`: Total query latency by digest
- `mysql_query_latency_avg`: Average query latency
- `mysql_query_latency_max`: Maximum query latency
- `mysql_query_rows_examined`: Total rows examined
- `mysql_query_no_index_used`: Queries not using indexes

### Table I/O Metrics
- `mysql_table_io_read_latency`: Read latency per table
- `mysql_table_io_write_latency`: Write latency per table
- `mysql_table_io_read_count`: Read operations per table
- `mysql_table_io_write_count`: Write operations per table

### Index Metrics
- `mysql_index_cardinality`: Index cardinality by table and index

## Integration

### Standalone Mode
```bash
docker-compose up
```

### With Core Metrics
```bash
# From module directory
make run-with-core

# From root directory
make run-core run-sql-intelligence
```

### Consuming Metrics
Other modules can scrape this module's metrics:

```yaml
prometheus:
  config:
    scrape_configs:
      - job_name: 'sql-intelligence'
        static_configs:
          - targets: ['sql-intelligence:8082']
```

## Testing

```bash
# Run all tests
make test

# Generate test queries
docker exec -it sql-intelligence_mysql-test_1 mysql -uroot -ptest -e "
  CREATE DATABASE IF NOT EXISTS test;
  USE test;
  CREATE TABLE IF NOT EXISTS users (id INT PRIMARY KEY, name VARCHAR(100));
  INSERT INTO users VALUES (1, 'test');
  SELECT * FROM users WHERE name = 'test';
"
```