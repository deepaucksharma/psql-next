# Core Metrics Module

Basic MySQL metrics collection module for the Database Intelligence system.

## Features

- Connection metrics (current, max, aborted)
- Thread metrics (connected, running, cached)
- Operation counters (select, insert, update, delete)
- Handler statistics
- InnoDB metrics (buffer pool, row operations)
- Lock metrics
- Sort operations
- Table cache status

## Quick Start

```bash
# Build the module
make build

# Run the module
make run

# View logs
make logs

# Check metrics
curl http://localhost:8081/metrics

# Stop the module
make stop
```

## Configuration

The module is configured via environment variables:

- `MYSQL_ENDPOINT`: MySQL connection endpoint (default: mysql-test:3306)
- `MYSQL_USER`: MySQL username (default: root)
- `MYSQL_PASSWORD`: MySQL password (default: test)
- `EXPORT_PORT`: Prometheus metrics port (default: 8081)

## Metrics Exposed

All metrics are prefixed with `mysql_` namespace:

- `mysql_uptime`: Server uptime in seconds
- `mysql_connections_current`: Current open connections
- `mysql_connections_max`: Maximum connections allowed
- `mysql_threads_connected`: Currently connected threads
- `mysql_threads_running`: Currently running threads
- `mysql_operations_total`: Operation counters by type
- `mysql_handlers_total`: Handler operation counters
- `mysql_innodb_*`: InnoDB specific metrics
- `mysql_locks_*`: Lock wait metrics
- `mysql_sorts_*`: Sort operation metrics

## Testing

```bash
# Run all tests
make test

# Run only unit tests
make test-unit

# Run only integration tests
make test-integration
```

## Integration

This module can run standalone or integrate with other modules:

```yaml
# Standalone mode
docker-compose up

# With other modules (from root)
make run-core run-intelligence
```