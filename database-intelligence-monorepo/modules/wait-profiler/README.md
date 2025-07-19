# Wait Profiler Module

MySQL wait event profiling and analysis module for identifying performance bottlenecks.

## Features

- **Wait Event Analysis**
  - Global wait event statistics
  - Wait time profiling (total, average, max)
  - Wait event categorization (I/O, locks, mutex, etc.)

- **Mutex Wait Tracking**
  - Mutex contention analysis
  - Hot mutex identification
  - Wait time per mutex

- **I/O Wait Profiling**
  - File-level I/O wait analysis
  - Read/write operation counts
  - Bytes transferred tracking

- **Lock Wait Monitoring**
  - Active lock tracking
  - Lock type and mode analysis
  - Schema and table level lock metrics

## Quick Start

```bash
# Build the module
make build

# Run the module
make run

# Generate test wait events
make generate-load

# View wait metrics
make status

# Stop the module
make stop
```

## Configuration

Environment variables:

- `MYSQL_ENDPOINT`: MySQL connection endpoint (default: mysql-test:3306)
- `MYSQL_USER`: MySQL username (default: root)
- `MYSQL_PASSWORD`: MySQL password (default: test)
- `EXPORT_PORT`: Prometheus metrics port (default: 8083)

## Metrics Exposed

### Wait Event Metrics
- `mysql_wait_count`: Wait event occurrence count
- `mysql_wait_time_total`: Total wait time in seconds
- `mysql_wait_time_avg`: Average wait time per event
- `mysql_wait_time_max`: Maximum wait time observed

### Mutex Metrics
- `mysql_mutex_wait_count`: Mutex wait occurrences
- `mysql_mutex_wait_time`: Total mutex wait time

### I/O Metrics
- `mysql_io_wait_count`: I/O wait occurrences
- `mysql_io_wait_time`: I/O wait time by file
- `mysql_io_bytes_read`: Bytes read per file
- `mysql_io_bytes_write`: Bytes written per file

### Lock Metrics
- `mysql_lock_active_count`: Active locks by type and mode

## Wait Categories

The module automatically categorizes wait events:

- **io**: I/O related waits (file operations)
- **lock**: Lock acquisition waits
- **mutex**: Mutex synchronization waits
- **condition**: Condition variable waits
- **rwlock**: Read/write lock waits
- **other**: Uncategorized waits

## Severity Levels

Wait events are classified by severity:

- **critical**: Average wait > 1 second
- **warning**: Average wait > 0.1 second
- **normal**: Average wait < 0.1 second

## Integration

### Standalone Mode
```bash
docker-compose up
```

### With Other Modules
```bash
# From root directory
make run-wait-profiler run-anomaly-detector
```

## Performance Impact

Wait profiling uses MySQL Performance Schema which has minimal overhead when properly configured. The module samples events at regular intervals to balance visibility with performance.

## Troubleshooting

### No Wait Events Showing
```bash
# Check if performance schema is enabled
docker exec -it wait-profiler_mysql-test_1 mysql -uroot -ptest -e "
  SHOW VARIABLES LIKE 'performance_schema';
  SELECT * FROM performance_schema.setup_consumers;
"
```

### Generate Test Waits
```bash
# Create some wait events
make generate-load
```