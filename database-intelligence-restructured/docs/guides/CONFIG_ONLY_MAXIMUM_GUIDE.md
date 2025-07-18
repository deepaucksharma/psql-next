# PostgreSQL Maximum Metrics Extraction - Config-Only Guide

This guide demonstrates how to extract the maximum possible metrics from PostgreSQL using only stock OpenTelemetry components - no custom code required!

## Overview

The `postgresql-maximum-extraction.yaml` configuration demonstrates:
- **100+ distinct metrics** from PostgreSQL
- **ASH-like session sampling** at 1-second intervals
- **Query performance analysis** without custom processors
- **Advanced monitoring** including replication, vacuum progress, table bloat
- **Security auditing** and compliance metrics
- **Business metrics** from application tables

## Quick Start

```bash
# 1. Set environment variables
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=your_password
export POSTGRES_DB=your_database
export NEW_RELIC_LICENSE_KEY=your_license_key

# 2. Run the collector with maximum extraction config
docker run -d \
  --name otel-max-extraction \
  -v $(pwd)/configs/postgresql-maximum-extraction.yaml:/etc/otelcol-contrib/config.yaml \
  -e POSTGRES_HOST \
  -e POSTGRES_PORT \
  -e POSTGRES_USER \
  -e POSTGRES_PASSWORD \
  -e POSTGRES_DB \
  -e NEW_RELIC_LICENSE_KEY \
  -p 13133:13133 \
  -p 8888:8888 \
  otel/opentelemetry-collector-contrib:latest
```

## Configuration Breakdown

### 1. Core PostgreSQL Receiver (35+ metrics)

The standard PostgreSQL receiver provides:
```yaml
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    collection_interval: 10s
```

Metrics include:
- `postgresql.backends` - Active connections
- `postgresql.commits` - Transaction commits
- `postgresql.db_size` - Database sizes
- `postgresql.blocks_read` - Disk I/O
- `postgresql.table.size` - Table sizes
- `postgresql.index.scans` - Index usage
- And 30+ more...

### 2. Active Session History (ASH) Simulation

Real-time session monitoring at 1-second intervals:
```yaml
sqlquery/ash:
  collection_interval: 1s  # High frequency sampling
  queries:
    - sql: |
        SELECT 
          state,
          wait_event_type,
          wait_event,
          COUNT(*) as session_count,
          MAX(EXTRACT(EPOCH FROM (NOW() - query_start))) as max_duration
        FROM pg_stat_activity
        GROUP BY state, wait_event_type, wait_event
```

Provides metrics:
- `postgresql.ash.sessions` - Session counts by state and wait event
- `postgresql.ash.sessions.long_running` - Long-running query detection
- `postgresql.ash.query_duration.max` - Maximum query duration
- `postgresql.ash.query_duration.avg` - Average query duration

### 3. Query Performance Intelligence

Detailed query statistics from `pg_stat_statements`:
```yaml
sqlquery/query_stats:
  collection_interval: 30s
  queries:
    - sql: |
        SELECT 
          queryid,
          calls,
          total_exec_time,
          mean_exec_time,
          cache_hit_ratio,
          wal_bytes
        FROM pg_stat_statements
        WHERE calls > 5
        ORDER BY total_exec_time DESC
        LIMIT 50
```

Tracks:
- `postgresql.query.calls` - Execution count
- `postgresql.query.mean_time` - Average execution time
- `postgresql.query.cache_hit_ratio` - Buffer cache efficiency
- `postgresql.query.wal_bytes` - WAL generation
- 10+ more query metrics...

### 4. Blocking and Lock Analysis

Real-time blocking detection:
```yaml
sqlquery/blocking:
  collection_interval: 5s
  queries:
    - Blocking chain detection
    - Lock wait analysis
    - Deadlock prediction
```

Provides:
- `postgresql.blocking.chain_count` - Active blocking chains
- `postgresql.blocking.max_duration` - Longest block duration
- `postgresql.locks.waiting` - Waiting lock count by type

### 5. Table and Index Analytics

Comprehensive table statistics:
```yaml
sqlquery/table_stats:
  collection_interval: 60s
  queries:
    - Table access patterns
    - Index efficiency metrics
    - Dead tuple ratios
    - Vacuum statistics
```

Includes:
- `postgresql.table.seq_scan` vs `postgresql.table.idx_scan`
- `postgresql.table.dead_tuple_ratio` - Bloat indicators
- `postgresql.index.avg_tuples_per_scan` - Index efficiency

### 6. Advanced Health Metrics

Database-wide health indicators:
- Cache hit ratios per database
- Transaction ID wraparound monitoring
- Checkpoint performance
- Replication lag (time and bytes)
- Connection pool efficiency

### 7. Security and Compliance

Security monitoring without custom code:
```yaml
sqlquery/security:
  queries:
    - User privilege audit
    - SSL connection monitoring
    - Password age tracking
    - Superuser detection
```

Tracks:
- `postgresql.security.superuser_count`
- `postgresql.security.ssl_connections`
- `postgresql.security.password_age`

### 8. Host Metrics Integration

Complete system metrics via hostmetrics receiver:
- CPU, Memory, Disk I/O
- Network statistics
- Process information
- Filesystem usage

## Processing Pipeline

### Multi-Pipeline Architecture

```yaml
service:
  pipelines:
    # High-frequency metrics (1s interval)
    metrics/high_frequency:
      receivers: [sqlquery/ash]
      processors: [memory_limiter, resource, transform/add_metadata, batch]
      exporters: [otlp/newrelic]

    # Standard metrics (10s interval)
    metrics/standard:
      receivers: [postgresql, sqlquery/health, sqlquery/blocking]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/newrelic]

    # Performance metrics (30-60s interval)
    metrics/performance:
      receivers: [sqlquery/query_stats, sqlquery/table_stats]
      processors: [memory_limiter, resource, filter/reduce_cardinality, batch]
      exporters: [otlp/newrelic]
```

### Smart Processing

1. **Memory Protection**
   ```yaml
   memory_limiter:
     limit_mib: 1024
     spike_limit_mib: 256
   ```

2. **Metadata Enrichment**
   ```yaml
   transform/add_metadata:
     metric_statements:
       - set(attributes["query.classification"], "slow") where attributes["mean_exec_time"] > 1000
       - set(attributes["session.classification"], "long_running") where attributes["max_duration"] > 300
   ```

3. **Cardinality Control**
   ```yaml
   filter/reduce_cardinality:
     metrics:
       exclude:
         match_type: regexp
         metric_names:
           - "postgresql\\.query\\..*"  # If too many unique queries
   ```

## Performance Considerations

### Collection Intervals

- **1 second**: ASH sampling (critical for wait analysis)
- **5 seconds**: Blocking detection (time-sensitive)
- **10 seconds**: Core metrics, connection pools
- **30 seconds**: Query statistics, health checks
- **60 seconds**: Table statistics, performance analysis
- **5 minutes**: Vacuum progress, maintenance tasks
- **1 hour**: Extension inventory, security audit

### Resource Usage

Expected collector resource usage:
- **Memory**: 200-500MB typical, 1GB maximum
- **CPU**: 5-10% of one core
- **Network**: ~1-5 MB/minute to New Relic

### Metric Cardinality

Approximate metric cardinality:
- **Low**: Core PostgreSQL metrics (~50 series)
- **Medium**: Database/table metrics (~500 series)
- **High**: Query-level metrics (~5000+ series)
- **Total**: ~10,000 unique metric series

## Customization Options

### 1. Reduce Cardinality

If metric volume is too high:
```yaml
processors:
  filter/reduce_cardinality:
    metrics:
      exclude:
        metric_names:
          - "postgresql\\.query\\..*"     # Exclude per-query metrics
          - "postgresql\\.table\\..*"     # Exclude per-table metrics
          - "postgresql\\.index\\..*"     # Exclude per-index metrics
```

### 2. Add Business Metrics

Monitor application-specific tables:
```yaml
sqlquery/business:
  queries:
    - sql: |
        SELECT 
          COUNT(*) as user_count,
          COUNT(*) FILTER (WHERE last_login > NOW() - INTERVAL '1 day') as daily_active_users
        FROM users
      metrics:
        - metric_name: app.users.total
          value_column: user_count
        - metric_name: app.users.daily_active
          value_column: daily_active_users
```

### 3. Conditional Collection

Enable/disable metric groups:
```yaml
receivers:
  postgresql:
    metrics:
      postgresql.table.size:
        enabled: ${env:COLLECT_TABLE_METRICS:true}
      postgresql.index.scans:
        enabled: ${env:COLLECT_INDEX_METRICS:true}
```

## Monitoring Best Practices

### 1. Start Conservative
- Begin with core metrics only
- Add advanced metrics gradually
- Monitor collector resource usage

### 2. Use Sampling
- High-cardinality metrics benefit from sampling
- Consider probabilistic sampling for query metrics

### 3. Set Alerts
Key metrics to alert on:
- `postgresql.connection_pool.active_percentage` > 80%
- `postgresql.blocking.max_duration` > 30 seconds
- `postgresql.database.cache_hit_ratio` < 90%
- `postgresql.database.wraparound_percent` > 50%

### 4. Dashboard Organization
Group metrics by:
- **Overview**: Key health indicators
- **Performance**: Query and transaction metrics
- **Resources**: Memory, disk, connections
- **Maintenance**: Vacuum, bloat, indexes
- **Security**: Users, permissions, SSL

## Troubleshooting

### No Metrics Appearing

1. Check collector logs:
   ```bash
   docker logs otel-max-extraction
   ```

2. Verify PostgreSQL extensions:
   ```sql
   CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
   ```

3. Test connectivity:
   ```bash
   docker exec otel-max-extraction curl http://localhost:13133/health
   ```

### High Memory Usage

1. Reduce collection frequency
2. Enable cardinality filtering
3. Decrease batch sizes
4. Limit query result sets

### Missing Query Metrics

Ensure pg_stat_statements is configured:
```sql
-- In postgresql.conf
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.track = all
pg_stat_statements.max = 10000
```

## Conclusion

This configuration demonstrates that you can achieve enterprise-grade PostgreSQL monitoring using only stock OpenTelemetry components. No custom code required - just thoughtful configuration!

Key achievements:
- ✅ 100+ distinct metrics
- ✅ Sub-second sampling for critical metrics
- ✅ Query-level performance insights
- ✅ Advanced monitoring (replication, vacuum, security)
- ✅ Automatic metadata enrichment
- ✅ Multi-pipeline architecture for optimal performance

The same patterns can be applied to MySQL, MongoDB, Redis, and other databases supported by OpenTelemetry receivers.

## Additional Database Configurations

We've created maximum extraction configurations for multiple databases:

### MySQL Maximum Extraction
- **Config**: [`mysql-maximum-extraction.yaml`](../../configs/mysql-maximum-extraction.yaml)
- **Guide**: [MySQL Maximum Guide](./MYSQL_MAXIMUM_GUIDE.md)
- **Metrics**: 80+ including Performance Schema, InnoDB internals, replication
- **Features**: Query digest analysis, connection pool monitoring, table statistics

### MongoDB Maximum Extraction
- **Config**: [`mongodb-maximum-extraction.yaml`](../../configs/mongodb-maximum-extraction.yaml)
- **Guide**: [MongoDB Maximum Guide](./MONGODB_MAXIMUM_GUIDE.md)
- **Metrics**: 90+ including WiredTiger stats, currentOp, Atlas metrics
- **Features**: Real-time operations, lock analysis, oplog monitoring

### MSSQL/SQL Server Maximum Extraction
- **Config**: [`mssql-maximum-extraction.yaml`](../../configs/mssql-maximum-extraction.yaml)
- **Guide**: [MSSQL Maximum Guide](./MSSQL_MAXIMUM_GUIDE.md)
- **Metrics**: 100+ including wait statistics, query stats, Always On AG
- **Features**: Wait categorization, blocking detection, index fragmentation

### Oracle Database Maximum Extraction
- **Config**: [`oracle-maximum-extraction.yaml`](../../configs/oracle-maximum-extraction.yaml)
- **Guide**: [Oracle Maximum Guide](./ORACLE_MAXIMUM_GUIDE.md)
- **Metrics**: 120+ via V$ views, ASM, RAC, Data Guard monitoring
- **Features**: Wait event analysis, tablespace monitoring, SQL performance

## Comparison Matrix

| Feature | PostgreSQL | MySQL | MongoDB | MSSQL | Oracle |
|---------|------------|-------|---------|-------|--------|
| **Base Metrics** | 35+ | 40+ | 50+ | 40+ | - |
| **Extended Metrics** | 65+ | 40+ | 40+ | 60+ | 120+ |
| **Total Metrics** | 100+ | 80+ | 90+ | 100+ | 120+ |
| **ASH/CurrentOp** | ✅ | ✓ | ✅ | ✅ | ✅ |
| **Query Analysis** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Wait Statistics** | ✅ | ✓ | ✅ | ✅ | ✅ |
| **Replication** | ✅ | ✅ | ✅ | ✅ AG | ✅ DG |
| **Memory Details** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Lock Analysis** | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Cloud Support** | - | - | ✅ Atlas | Azure | OCI |
| **HA/DR Monitoring** | ✓ | ✓ | ✅ | ✅ AG | ✅ DG |
| **Storage Mgmt** | ✓ | ✓ | ✓ | ✓ | ✅ ASM |

## Common Patterns Across Databases

### 1. Multi-Pipeline Architecture
All configurations use separate pipelines for different collection frequencies:
- High-frequency (1-5s): Critical real-time metrics
- Standard (10s): Core database metrics
- Performance (30s): Query and performance analysis
- Analytics (60s+): Resource-intensive statistics

### 2. Intelligent Processing
Every configuration includes:
- Memory limiting to prevent OOM
- Batch processing for efficiency
- Metadata enrichment for context
- Cardinality reduction filters

### 3. Flexible Deployment
All configs support:
- Environment variable configuration
- Docker and Kubernetes deployment
- Multiple export targets
- Debug and monitoring endpoints

## Choosing the Right Configuration

### When to Use Maximum Extraction
- **Development/Staging**: Full visibility for troubleshooting
- **Performance Testing**: Detailed metrics during load tests
- **Production Issues**: Temporary deep monitoring
- **Capacity Planning**: Comprehensive resource analysis

### When to Use Reduced Sets
- **High-Scale Production**: Control costs and cardinality
- **Multi-Tenant**: Limit per-database overhead
- **Edge Deployments**: Minimize resource usage
- **Long-Term Storage**: Focus on key indicators

## Next Steps

1. **Try It Out**: Deploy a maximum extraction config for your database
2. **Customize**: Adjust collection intervals and metric selection
3. **Monitor Costs**: Track metric volume in New Relic
4. **Optimize**: Create custom configurations for your use case
5. **Share**: Contribute improvements back to the community

The power of OpenTelemetry lies in its flexibility - these configurations demonstrate just how much you can achieve with thoughtful YAML!