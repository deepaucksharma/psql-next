# Known Issues and Limitations

This document tracks known issues, limitations, and workarounds for the Database Intelligence Collector v1.0.0.

## Current Limitations

### 1. Single Instance Only
**Description**: The collector operates as a single instance with no high availability support.

**Impact**: 
- No automatic failover
- Brief monitoring gaps during restarts
- State loss on restart

**Workaround**:
- Use container orchestration for quick restarts
- Configure aggressive health checks
- Accept brief data gaps (typically 2-3 seconds)

**Future Plan**: Multi-instance support planned for v2.0

### 2. In-Memory State Loss
**Description**: All processor state is kept in memory and lost on restart.

**Impact**:
- Sampling patterns reset on restart
- Circuit breaker states reset
- Cache data lost

**Workaround**:
- Use conservative default configurations
- Implement quick startup procedures
- Monitor post-restart behavior

**Future Plan**: Optional state persistence in v1.1

### 3. Plan Size Limitations
**Description**: Large query plans (>100KB) may be truncated or skipped.

**Impact**:
- Complex query plans may not be fully analyzed
- Plan hash might be incomplete for very large plans

**Workaround**:
```yaml
plan_attribute_extractor:
  max_plan_size: 200KB  # Increase limit
  parse_timeout: 10s    # Increase timeout
```

### 4. No Horizontal Scaling
**Description**: Cannot distribute load across multiple instances.

**Impact**:
- Limited to single machine resources
- No load balancing capabilities

**Workaround**:
- Use powerful single instance
- Implement aggressive sampling
- Partition databases across multiple collectors

## Performance Limitations

### 1. Memory Usage Scaling
**Issue**: Memory usage scales with:
- Number of unique metrics
- Cache sizes
- Active circuit breakers

**Workaround**:
```yaml
# Reduce cache sizes
adaptive_sampler:
  deduplication:
    cache_size: 5000  # Reduced from 10000

# Increase memory limits
memory_limiter:
  limit_percentage: 80
  spike_limit_percentage: 20
```

### 2. High Cardinality Metrics
**Issue**: High cardinality can cause:
- Memory pressure
- Export failures
- Increased costs

**Workaround**:
```yaml
# Add cardinality reduction
transform:
  metric_statements:
    - context: metric
      statements:
        - keep_keys(attributes, ["db.system", "db.name", "db.operation"])
        - truncate_all(attributes, 50)
```

## Configuration Limitations

### 1. Environment Variable Arrays
**Issue**: Cannot pass arrays via environment variables.

**Example**:
```yaml
# This doesn't work with env vars
databases: ${POSTGRES_DATABASES}  # Can't pass array
```

**Workaround**:
Use configuration files for complex structures or implement custom parsing:
```yaml
databases:
  - ${POSTGRES_DB_1}
  - ${POSTGRES_DB_2}
  - ${POSTGRES_DB_3}
```

### 2. Dynamic Reloading
**Issue**: Configuration changes require restart.

**Impact**:
- Brief monitoring gaps
- State loss

**Workaround**:
- Plan configuration changes during maintenance windows
- Use feature flags where possible
- Implement gradual rollouts

## Database-Specific Issues

### 1. PostgreSQL pg_stat_statements
**Issue**: Requires extension installation and configuration.

**Setup**:
```sql
-- Install extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Configure postgresql.conf
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.track = all
pg_stat_statements.max = 10000
```

### 2. MySQL Performance Schema
**Issue**: Performance schema must be enabled and configured.

**Setup**:
```sql
-- Check if enabled
SHOW VARIABLES LIKE 'performance_schema';

-- Enable in my.cnf
[mysqld]
performance_schema = ON
performance_schema_instrument = '%=ON'
```

### 3. Connection Pool Exhaustion
**Issue**: Can exhaust database connection limits.

**Workaround**:
```yaml
receivers:
  postgresql:
    connection_pool:
      max_open: 5      # Limit connections
      max_idle: 2      
    collection_interval: 60s  # Reduce frequency
```

## Export Issues

### 1. New Relic Cardinality Limits
**Issue**: New Relic has metric cardinality limits.

**Error**: `NrIntegrationError: Metric cardinality limit exceeded`

**Workaround**:
- Enable aggressive sampling
- Reduce attribute count
- Use metric transformations

### 2. Network Timeout
**Issue**: Slow networks can cause export timeouts.

**Workaround**:
```yaml
exporters:
  otlp/newrelic:
    timeout: 30s  # Increase from default
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 60s
```

## Security Considerations

### 1. PII Detection False Positives
**Issue**: Regex patterns may have false positives.

**Example**: Phone number patterns matching order IDs

**Workaround**:
- Tune patterns for your data
- Use custom patterns
- Implement whitelist

### 2. Credential Storage
**Issue**: Credentials in environment variables are visible in process list.

**Workaround**:
- Use secrets management systems
- Implement credential rotation
- Use IAM roles where possible

## Monitoring Blind Spots

### 1. Collector Self-Monitoring
**Issue**: If collector fails, monitoring stops.

**Workaround**:
- External health checks
- Multiple monitoring layers
- Alerting on absence of data

### 2. Sampling Bias
**Issue**: Aggressive sampling may miss important events.

**Workaround**:
- Always sample critical operations
- Use multiple sampling rules
- Monitor sampling effectiveness

## Upgrade Considerations

### 1. Breaking Changes
**Issue**: Configuration format may change between versions.

**Workaround**:
- Test upgrades in staging
- Keep configuration backups
- Review changelog carefully

### 2. Processor Compatibility
**Issue**: Custom processors may need updates.

**Workaround**:
- Maintain processor version compatibility
- Test thoroughly before upgrading
- Keep old binaries available

## Troubleshooting Guide

### Quick Diagnostics

1. **Collector Won't Start**
   ```bash
   # Check configuration
   collector --config=config.yaml --dry-run
   
   # Check logs
   journalctl -u db-intelligence-collector -n 100
   ```

2. **No Metrics Collected**
   ```bash
   # Check health
   curl http://localhost:13133/health
   
   # Check specific component
   curl http://localhost:13133/health | jq '.components.postgresql'
   ```

3. **High Memory Usage**
   ```bash
   # Check metrics
   curl http://localhost:8888/metrics | grep memory
   
   # Force garbage collection (if pprof enabled)
   curl -X POST http://localhost:1777/debug/pprof/gc
   ```

## Reporting Issues

When reporting issues, please include:

1. **Version Information**
   ```bash
   collector --version
   ```

2. **Configuration** (sanitized)
   ```bash
   collector --config=config.yaml --dry-run
   ```

3. **Error Logs**
   ```bash
   journalctl -u db-intelligence-collector --since "10 minutes ago"
   ```

4. **Metrics Snapshot**
   ```bash
   curl http://localhost:8888/metrics > metrics.txt
   curl http://localhost:13133/health > health.json
   ```

5. **Environment**
   - OS and version
   - Deployment method
   - Database versions
   - Network configuration

## Support Channels

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/database-intelligence-mvp/database-intelligence-mvp/issues)
- **Discussions**: [Community support](https://github.com/database-intelligence-mvp/database-intelligence-mvp/discussions)
- **Slack**: #database-intelligence channel

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025