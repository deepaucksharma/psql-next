# MySQL Replication Monitor

This module provides comprehensive monitoring for MySQL master-slave replication using OpenTelemetry Collector.

## Overview

The replication-monitor module sets up a complete MySQL replication environment with:
- MySQL 8.0 master server on port 3306
- MySQL 8.0 replica server on port 8086
- OpenTelemetry Collector for metrics collection
- Automated replication setup with GTID support
- Real-time replication lag monitoring
- Performance metrics collection

## Key Features

- **Replication Metrics**:
  - Seconds behind master (replication lag)
  - Binary log position tracking
  - IO and SQL thread status
  - GTID execution status
  - Connection status monitoring

- **Performance Monitoring**:
  - Query performance metrics
  - Connection pool statistics
  - Buffer pool utilization
  - Lock wait times
  - Table I/O statistics

- **Health Monitoring**:
  - Automatic health checks
  - Replication error detection
  - Thread status monitoring
  - Performance schema integration

## Quick Start

```bash
# Build and start all services
make up

# Check replication status
make status

# Monitor replication lag in real-time
make monitor-lag

# Test replication is working
make test-replication

# Generate load to test lag
make generate-load
```

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│  MySQL Master   │────────▶│  MySQL Replica  │
│   Port: 3306    │         │   Port: 8086    │
└────────┬────────┘         └────────┬────────┘
         │                           │
         └───────────┬───────────────┘
                     │
              ┌──────▼──────┐
              │  OTel       │
              │ Collector   │
              └─────────────┘
                     │
         ┌───────────┼───────────┐
         │           │           │
    Prometheus    Logging     OTLP
    Port: 8889    Console   Port: 4317
```

## Configuration

### MySQL Configuration
- GTID-based replication enabled
- Binary logging configured
- Performance schema enabled
- Slow query log enabled

### Collector Configuration
- MySQL receiver for both master and replica
- SQL query receiver for custom replication metrics
- Prometheus exporter on port 8889
- OTLP exporter on port 4317

## Available Commands

| Command | Description |
|---------|-------------|
| `make build` | Build Docker images |
| `make up` | Start all services |
| `make down` | Stop all services |
| `make status` | Check replication status |
| `make monitor-lag` | Monitor replication lag in real-time |
| `make test-replication` | Test replication is working |
| `make generate-load` | Generate load on master |
| `make check-gtid` | Check GTID status |
| `make metrics` | View Prometheus metrics |
| `make health` | Check service health |
| `make connect-master` | Connect to master MySQL |
| `make connect-replica` | Connect to replica MySQL |

## Monitoring Endpoints

- **Prometheus Metrics**: http://localhost:8889/metrics
- **Collector Health**: http://localhost:13133/health
- **Collector Metrics**: http://localhost:8888/metrics
- **MySQL Replica**: localhost:8086

## Metrics Collected

### Replication Metrics
- `mysql.replication.slave.seconds_behind_master` - Replication lag in seconds
- `mysql.replication.slave.io_running` - IO thread status
- `mysql.replication.slave.sql_running` - SQL thread status
- `mysql.replication.master.log_position` - Master binary log position
- `mysql.replication.gtid.executed` - Executed GTID set
- `mysql.replication.connection.status` - Connection status

### Performance Metrics
- `mysql.commands` - Command execution counts
- `mysql.connection.count` - Active connections
- `mysql.query.slow.count` - Slow query count
- `mysql.buffer_pool.usage` - Buffer pool utilization
- `mysql.innodb_row_lock.time` - Row lock wait time

## Testing Replication

### Basic Test
```bash
# Insert data on master and verify replication
make test-replication
```

### Load Testing
```bash
# Generate continuous load on master
make generate-load

# Monitor lag in another terminal
make monitor-lag
```

### Manual Testing
```bash
# Connect to master
docker exec -it mysql-master mysql -uroot -prootpassword test_db

# Insert test data
INSERT INTO users (username, email) VALUES ('test', 'test@example.com');

# Connect to replica
docker exec -it mysql-replica mysql -uroot -prootpassword test_db

# Verify data
SELECT * FROM users WHERE username = 'test';
```

## Troubleshooting

### Check Replication Status
```bash
# Full replication status
docker exec mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G"

# Check for errors
make status | grep Last_Error
```

### Common Issues

1. **Replication Not Starting**
   - Check master is healthy: `docker ps`
   - Verify replication user: Check init-master.sql
   - Check network connectivity between containers

2. **High Replication Lag**
   - Monitor with `make monitor-lag`
   - Check replica resources
   - Verify network latency
   - Review slow queries on master

3. **GTID Issues**
   - Use `make check-gtid` to compare GTID sets
   - Ensure GTID mode is ON on both servers
   - Check for errant transactions

### Reset Replication
```bash
# Stop services
make down

# Clean volumes
make clean

# Start fresh
make up
```

## Advanced Usage

### Custom Metrics Queries

Add custom SQL queries to `config/collector.yaml`:

```yaml
sqlquery:
  queries:
    - sql: "YOUR CUSTOM QUERY"
      metrics:
        - metric_name: custom.metric.name
          value_column: "column_name"
```

### Performance Tuning

Adjust MySQL settings in docker-compose.yaml:
- `innodb_buffer_pool_size`
- `max_connections`
- `sync_binlog`
- `innodb_flush_log_at_trx_commit`

## Development

### Project Structure
```
replication-monitor/
├── docker-compose.yaml    # Service definitions
├── Dockerfile            # MySQL image configuration
├── Makefile             # Command shortcuts
├── README.md            # This file
├── config/
│   └── collector.yaml   # OTel Collector configuration
└── src/
    ├── init-master.sql  # Master initialization
    └── init-replica.sql # Replica initialization
```

### Adding New Metrics

1. Update `collector.yaml` with new receivers
2. Add SQL queries for custom metrics
3. Update processors and exporters as needed
4. Rebuild with `make restart`

## Best Practices

1. **Monitor Replication Lag**: Keep lag under 1 second for most applications
2. **Use GTID**: Enables easier failover and recovery
3. **Regular Backups**: Use `make backup-master` periodically
4. **Resource Monitoring**: Watch CPU and memory on replica
5. **Network Optimization**: Ensure low latency between master and replica

## Security Considerations

- Change default passwords in production
- Use SSL for replication in production
- Restrict replication user permissions
- Enable audit logging for compliance
- Use network isolation in production

## License

This module is part of the Database Intelligence project.