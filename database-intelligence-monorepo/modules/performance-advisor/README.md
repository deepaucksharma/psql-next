# Performance Advisor Module

The Performance Advisor module provides automated performance recommendations for MySQL databases by analyzing metrics from other Database Intelligence modules.

## Overview

This module consumes metrics from:
- Anomaly Detector
- Query Analyzer
- Intelligent Alerting

It generates actionable recommendations for:
- Missing indexes
- Slow query optimization
- Connection pool sizing
- Cache hit ratio improvements
- Lock contention resolution
- Memory usage optimization
- Query error patterns
- Replication lag issues

## ⚠️ Health Check Policy

**IMPORTANT**: Health check endpoints (port 13133) have been intentionally removed from production code.

- **For validation**: Use `shared/validation/health-check-all.sh`
- **Documentation**: See `shared/validation/README-health-check.md`
- **Do NOT**: Add health check endpoints back to production configs
- **Do NOT**: Expose port 13133 in Docker configurations

## Quick Start

```bash
# Build and start the service
make build
make up

# View logs
make logs

# Check status (use metrics endpoint)
curl http://localhost:8087/metrics | grep recommendation

# View current recommendations
make recommendations

# Monitor recommendations in real-time
make monitor
```

## Architecture

The Performance Advisor uses OpenTelemetry Collector with custom transform processors to:

1. **Collect Metrics**: Pull metrics from other modules via Prometheus scraping
2. **Analyze Patterns**: Transform metrics into performance recommendations
3. **Generate Advice**: Create new metrics with actionable recommendations
4. **Prioritize Issues**: Assign severity levels and priority scores

## Recommendation Types

### 1. Missing Index Detection
- Analyzes query execution patterns
- Identifies queries with high execution counts and WHERE clauses
- Suggests index creation for performance improvement

### 2. Slow Query Analysis
- Monitors query execution times
- Flags queries exceeding thresholds
- Provides optimization recommendations

### 3. Connection Pool Optimization
- Tracks active database connections
- Recommends optimal pool sizes
- Prevents connection exhaustion

### 4. Cache Efficiency
- Monitors cache hit ratios
- Suggests buffer pool adjustments
- Improves memory utilization

### 5. Lock Contention
- Detects table lock waits
- Recommends row-level locking strategies
- Helps reduce contention

### 6. Memory Management
- Tracks buffer pool usage
- Prevents memory pressure
- Optimizes allocation

### 7. Error Pattern Detection
- Monitors query error rates
- Identifies problematic queries
- Suggests fixes

### 8. Replication Performance
- Tracks replication lag
- Alerts on excessive delays
- Maintains data consistency

## Configuration

The module configuration is in `config/collector.yaml`. Key sections:

### Receivers
- OTLP HTTP on port 8087
- Prometheus scraping for metric collection

### Processors
- Transform: Generates recommendations from metrics
- Filter: Keeps only recommendation metrics
- Resource: Adds service metadata

### Exporters
- Prometheus: Exposes recommendations as metrics
- OTLP: Forwards to Intelligent Alerting
- Debug: Console output for troubleshooting

## Metrics Generated

All recommendation metrics follow the pattern: `db.performance.recommendation.*`

Example metrics:
- `db.performance.recommendation.missing_index`
- `db.performance.recommendation.slow_query`
- `db.performance.recommendation.connection_pool`
- `db.performance.recommendation.cache_efficiency`

Each metric includes attributes:
- `recommendation_type`: Category of recommendation
- `severity`: critical, high, medium, low
- `recommendation`: Human-readable advice
- `priority_score`: Numeric priority (0-100)
- Additional context-specific attributes

## Integration

### Consuming Recommendations

1. **Prometheus Format**:
   ```
   GET http://localhost:8888/metrics
   ```

2. **OTLP Format**:
   Send to downstream collectors on port 8087

### Example Usage

```bash
# Get all recommendations
curl -s http://localhost:8888/metrics | grep db_performance_recommendation

# Filter by severity
curl -s http://localhost:8888/metrics | grep 'severity="critical"'

# Monitor specific recommendation type
curl -s http://localhost:8888/metrics | grep missing_index
```

## Troubleshooting

### No Recommendations Generated
1. Check if source modules are running
2. Verify Prometheus targets are reachable
3. Review logs: `make logs`

### High Memory Usage
1. Adjust batch processor settings
2. Reduce scrape intervals
3. Filter unnecessary metrics

### Connection Issues
1. Ensure network connectivity
2. Check firewall rules
3. Verify service discovery

## Development

### Testing Configuration
```bash
make test
```

### Adding New Recommendations
1. Edit `config/collector.yaml`
2. Add transform statements
3. Define severity logic
4. Test with sample data

### Debugging
```bash
# View detailed logs
docker-compose logs -f performance-advisor

# Access debug interface
open http://localhost:55679/debug/tracez
```

## Performance Considerations

- Scrape interval: 30s (adjustable)
- Batch size: 1024 metrics
- Timeout: 10s
- Memory limits: Configurable in docker-compose.yaml

## Maintenance

```bash
# Clean up resources
make clean

# Restart service
make restart

# Update configuration
# Edit config/collector.yaml, then:
make restart
```