# Resource Monitor Module

This module monitors system resource usage related to MySQL operations using OpenTelemetry Collector.

## Overview

The Resource Monitor collects and exports metrics about:
- CPU usage and load averages
- Memory usage and utilization
- Disk I/O operations and performance
- Network traffic and connections
- Process-specific metrics for MySQL

## Architecture

The module uses OpenTelemetry Collector with the host metrics receiver to gather system-level metrics. It includes specialized pipelines for filtering and processing MySQL-related metrics.

## Quick Start

1. Build the Docker image:
   ```bash
   make build
   ```

2. Start the resource monitor:
   ```bash
   make start
   ```

3. View metrics:
   ```bash
   # View Prometheus metrics
   curl http://localhost:8090/metrics
   
   # Check health status
   make status
   
   # View logs
   make logs
   ```

4. Stop the monitor:
   ```bash
   make stop
   ```

## Configuration

The collector configuration (`config/collector.yaml`) includes:

- **Host Metrics Receiver**: Collects system resource metrics
- **Prometheus Receiver**: For scraping MySQL exporter metrics (if available)
- **OTLP Receiver**: Accepts metrics from other sources
- **Multiple Exporters**: Prometheus, OTLP, File, and Debug

### Key Metrics Collected

- **CPU**: Utilization, load averages
- **Memory**: Usage, utilization, paging
- **Disk**: I/O operations, throughput, latency
- **Network**: Bytes transferred, packets, errors, connections
- **Process**: CPU time, memory usage, disk I/O for MySQL processes

## Ports

- `8088`: OTLP gRPC receiver
- `8089`: OTLP HTTP receiver
- `8090`: Prometheus metrics endpoint
- `8091`: Health check endpoint

## Advanced Usage

### Monitor MySQL Processes
```bash
make monitor-mysql
```

### Export Metrics to File
```bash
make export-metrics
```

### Debug Mode
```bash
make debug
```

### View Configuration
```bash
make test
```

## Integration

This module can be integrated with:
- Prometheus for metrics storage and querying
- Grafana for visualization
- Other OpenTelemetry collectors for data aggregation
- MySQL exporters for database-specific metrics

## Troubleshooting

1. **Container won't start**: Check port conflicts
   ```bash
   docker ps -a
   netstat -tulpn | grep -E '(8088|8089|8090|8091)'
   ```

2. **No metrics appearing**: Check collector logs
   ```bash
   make logs-tail
   ```

3. **High memory usage**: Adjust memory limits in `collector.yaml`
   ```yaml
   memory_limiter:
     limit_mib: 512  # Adjust as needed
   ```

## Development

### Testing Configuration Changes
1. Edit `config/collector.yaml`
2. Test the configuration:
   ```bash
   make test
   ```
3. Reload configuration:
   ```bash
   make config-reload
   ```

### Accessing Debug Information
- Z-Pages: http://localhost:55679
- pprof: http://localhost:1777/debug/pprof
- Health: http://localhost:8091/

## Metrics Pipeline

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│ Host System │────▶│  Collectors  │────▶│ Processors │
└─────────────┘     └──────────────┘     └────────────┘
                           │                      │
                    ┌──────▼──────┐        ┌─────▼─────┐
                    │ Host Metrics│        │  Filters  │
                    │  Receiver   │        │  Batch    │
                    └─────────────┘        │  Resource │
                                          └───────────┘
                                                 │
                                          ┌──────▼──────┐
                                          │  Exporters  │
                                          ├─────────────┤
                                          │ Prometheus  │
                                          │    OTLP     │
                                          │    File     │
                                          └─────────────┘
```

## License

This module is part of the Database Intelligence project.