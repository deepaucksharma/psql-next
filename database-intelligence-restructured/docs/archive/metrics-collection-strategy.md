# Comprehensive Metrics Collection Strategy

## Overview

This document defines the complete metrics collection strategy for database intelligence, covering all metric categories, collection methods, and optimization techniques. The strategy ensures comprehensive observability while managing costs and performance impact.

## Metrics Categories

### 1. Connection and Pool Metrics

#### Purpose
Monitor database connection health, usage patterns, and potential exhaustion issues.

#### Core Metrics

| Metric Name | Type | Unit | Description | Source |
|-------------|------|------|-------------|--------|
| `{db}.connections.active` | Gauge | 1 | Currently active connections | pg_stat_activity |
| `{db}.connections.idle` | Gauge | 1 | Idle connections | pg_stat_activity |
| `{db}.connections.idle_in_transaction` | Gauge | 1 | Connections idle in transaction | pg_stat_activity |
| `{db}.connections.max` | Gauge | 1 | Maximum allowed connections | pg_settings |
| `{db}.connections.used_percent` | Gauge | % | Percentage of max connections used | Calculated |
| `{db}.connections.waiting` | Gauge | 1 | Connections waiting for lock | pg_locks |
| `{db}.connections.rejected` | Sum | 1 | Rejected connection attempts | pg_stat_database |

#### Collection Strategy
```yaml
postgresql:
  metrics:
    postgresql.connection.count:
      enabled: true
    postgresql.connection.max:
      enabled: true
      
sqlquery:
  queries:
    - sql: |
        SELECT 
          state,
          count(*) as connections,
          avg(EXTRACT(epoch FROM (now() - state_change))) as avg_state_duration
        FROM pg_stat_activity
        GROUP BY state
      metrics:
        - metric_name: postgresql.connections.by_state
          value_column: connections
          attribute_columns: [state]
        - metric_name: postgresql.connections.state_duration
          value_column: avg_state_duration
          unit: s
          attribute_columns: [state]
```

### 2. Throughput and Transaction Metrics

#### Purpose
Track database workload, transaction patterns, and rollback rates.

#### Core Metrics

| Metric Name | Type | Unit | Description | Source |
|-------------|------|------|-------------|--------|
| `{db}.transactions.committed` | Sum | {transactions} | Committed transactions | pg_stat_database |
| `{db}.transactions.rolled_back` | Sum | {transactions} | Rolled back transactions | pg_stat_database |
| `{db}.transactions.deadlocks` | Sum | {deadlocks} | Deadlock occurrences | pg_stat_database |
| `{db}.queries.executed` | Sum | {queries} | Total queries executed | pg_stat_statements |
| `{db}.queries.rate` | Gauge | {queries}/s | Query execution rate | Calculated |
| `{db}.operations.{type}` | Sum | {operations} | DML operations by type | pg_stat_user_tables |

#### Advanced Metrics (Enhanced Mode)
```yaml
enhancedsql:
  features:
    transaction_analysis:
      enabled: true
      capture_patterns: true
      metrics:
        - transaction.duration
        - transaction.statements_count
        - transaction.data_size
        - transaction.lock_wait_time
```

### 3. Resource Utilization Metrics

#### Purpose
Monitor database resource consumption and efficiency.

#### Database-Level Resources

| Metric Name | Type | Unit | Description | Source |
|-------------|------|------|-------------|--------|
| `{db}.blocks.hit` | Sum | {blocks} | Buffer cache hits | pg_stat_database |
| `{db}.blocks.read` | Sum | {blocks} | Disk blocks read | pg_stat_database |
| `{db}.cache.hit_ratio` | Gauge | 1 | Cache hit ratio | Calculated |
| `{db}.temp_files.created` | Sum | {files} | Temporary files created | pg_stat_database |
| `{db}.temp_files.size` | Sum | By | Size of temp files | pg_stat_database |
| `{db}.database.size` | Gauge | By | Database size on disk | pg_database_size() |
| `{db}.wal.generated` | Sum | By | WAL data generated | pg_stat_wal |
| `{db}.wal.lag` | Gauge | s | Replication lag | pg_stat_replication |

#### Host-Level Resources
```yaml
hostmetrics:
  scrapers:
    cpu:
      metrics:
        system.cpu.utilization:
          enabled: true
          attributes: [cpu.state]
    memory:
      metrics:
        system.memory.utilization:
          enabled: true
        system.memory.available:
          enabled: true
    disk:
      include_devices: ["/dev/sda*", "/dev/nvme*"]
      metrics:
        system.disk.io:
          enabled: true
          attributes: [device, direction]
        system.disk.pending_operations:
          enabled: true
    postgresql:
      process_name: "postgres"
      metrics:
        process.cpu.utilization:
          enabled: true
        process.memory.physical:
          enabled: true
        process.memory.virtual:
          enabled: true
```

### 4. Query Performance Metrics

#### Purpose
Provide deep insights into query execution patterns and performance.

#### Basic Query Metrics

| Metric Name | Type | Unit | Description | Source |
|-------------|------|------|-------------|--------|
| `{db}.query.duration` | Histogram | ms | Query execution time distribution | pg_stat_statements |
| `{db}.query.calls` | Sum | {calls} | Number of query executions | pg_stat_statements |
| `{db}.query.rows` | Histogram | {rows} | Rows returned/affected | pg_stat_statements |
| `{db}.query.mean_time` | Gauge | ms | Average query time | pg_stat_statements |
| `{db}.query.total_time` | Sum | ms | Total time spent in queries | pg_stat_statements |

#### Query Categorization
```yaml
sqlquery:
  queries:
    - sql: |
        SELECT 
          CASE 
            WHEN query LIKE 'SELECT%' THEN 'select'
            WHEN query LIKE 'INSERT%' THEN 'insert'
            WHEN query LIKE 'UPDATE%' THEN 'update'
            WHEN query LIKE 'DELETE%' THEN 'delete'
            ELSE 'other'
          END as query_type,
          COUNT(*) as count,
          AVG(mean_exec_time) as avg_time,
          PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY mean_exec_time) as p95_time
        FROM pg_stat_statements
        GROUP BY query_type
      metrics:
        - metric_name: postgresql.query.type.count
          value_column: count
          attribute_columns: [query_type]
        - metric_name: postgresql.query.type.avg_duration
          value_column: avg_time
          unit: ms
          attribute_columns: [query_type]
        - metric_name: postgresql.query.type.p95_duration
          value_column: p95_time
          unit: ms
          attribute_columns: [query_type]
```

#### Advanced Query Intelligence (Enhanced Mode)
```yaml
enhancedsql:
  features:
    query_stats:
      enabled: true
      top_n_queries: 100
      group_by: [normalized_query, user, database]
      capture_histograms: true
      percentiles: [50, 75, 90, 95, 99]
      
    query_patterns:
      enabled: true
      pattern_detection:
        - full_table_scans
        - missing_indexes
        - cartesian_products
        - excessive_sorting
```

### 5. Query Plan and Optimization Metrics

#### Purpose
Track query plan changes, performance regressions, and optimization opportunities.

#### Plan Metrics (Enhanced Mode Only)

| Metric Name | Type | Unit | Description |
|-------------|------|------|-------------|
| `{db}.plan.cost` | Gauge | 1 | Query plan cost estimate |
| `{db}.plan.actual_time` | Histogram | ms | Actual execution time |
| `{db}.plan.rows_estimate_error` | Gauge | % | Plan estimation accuracy |
| `{db}.plan.changes` | Sum | {changes} | Plan change events |
| `{db}.plan.regressions` | Sum | {regressions} | Plan regression detections |
| `{db}.plan.cache_hits` | Sum | {hits} | Plan cache hit rate |

#### Plan Analysis Configuration
```yaml
planattributeextractor:
  extract_fields:
    - total_cost
    - startup_cost
    - plan_rows
    - plan_width
    - actual_time
    - actual_rows
    - loops
    - node_types
    
  regression_detection:
    enabled: true
    thresholds:
      cost_increase: 1.5x
      time_increase: 2.0x
      rows_error: 10x
      
  plan_stability:
    track_changes: true
    history_size: 100
    alert_on_change: true
```

### 6. Active Session History (ASH) Metrics

#### Purpose
Provide real-time and historical analysis of database activity patterns.

#### Session Metrics (Enhanced Mode)

| Metric Name | Type | Unit | Description |
|-------------|------|------|-------------|
| `{db}.ash.sessions.active` | Gauge | 1 | Active session count |
| `{db}.ash.sessions.waiting` | Gauge | 1 | Sessions in wait state |
| `{db}.ash.wait_time` | Histogram | ms | Wait time distribution |
| `{db}.ash.cpu_time` | Histogram | ms | CPU time distribution |
| `{db}.ash.blocking_sessions` | Gauge | 1 | Number of blocking sessions |

#### Wait Event Categories
```yaml
ash:
  wait_event_mapping:
    IO:
      - DataFileRead
      - DataFileWrite
      - WALWrite
    Lock:
      - relation
      - tuple
      - transactionid
    CPU:
      - CPU
    Network:
      - ClientRead
      - ClientWrite
    System:
      - Extension
      - BufferPin
```

### 7. Index and Table Metrics

#### Purpose
Monitor table and index usage, maintenance needs, and performance.

#### Table Metrics

| Metric Name | Type | Unit | Description |
|-------------|------|------|-------------|
| `{db}.table.size` | Gauge | By | Table size including indexes |
| `{db}.table.rows.live` | Gauge | {rows} | Estimated live rows |
| `{db}.table.rows.dead` | Gauge | {rows} | Dead rows needing vacuum |
| `{db}.table.sequential_scans` | Sum | {scans} | Sequential scan count |
| `{db}.table.index_scans` | Sum | {scans} | Index scan count |
| `{db}.table.rows.inserted` | Sum | {rows} | Rows inserted |
| `{db}.table.rows.updated` | Sum | {rows} | Rows updated |
| `{db}.table.rows.deleted` | Sum | {rows} | Rows deleted |
| `{db}.table.vacuum.count` | Sum | {vacuums} | Vacuum operations |
| `{db}.table.analyze.count` | Sum | {analyzes} | Analyze operations |

#### Index Metrics
```yaml
sqlquery:
  queries:
    - sql: |
        SELECT 
          schemaname,
          tablename,
          indexname,
          idx_scan,
          idx_tup_read,
          idx_tup_fetch,
          pg_size_pretty(pg_relation_size(indexrelid)) as index_size
        FROM pg_stat_user_indexes
        WHERE idx_scan = 0 
        AND schemaname NOT IN ('pg_catalog', 'information_schema')
      metrics:
        - metric_name: postgresql.index.unused
          value_column: idx_scan
          attribute_columns: [schemaname, tablename, indexname]
```

### 8. Lock and Blocking Metrics

#### Purpose
Identify concurrency issues and blocking patterns.

#### Lock Metrics

| Metric Name | Type | Unit | Description |
|-------------|------|------|-------------|
| `{db}.locks.total` | Gauge | {locks} | Total lock count |
| `{db}.locks.granted` | Gauge | {locks} | Granted locks |
| `{db}.locks.waiting` | Gauge | {locks} | Waiting locks |
| `{db}.locks.deadlocks` | Sum | {deadlocks} | Deadlock occurrences |
| `{db}.locks.wait_time` | Histogram | ms | Lock wait time distribution |

```yaml
sqlquery:
  queries:
    - sql: |
        SELECT 
          locktype,
          mode,
          granted,
          COUNT(*) as lock_count
        FROM pg_locks
        GROUP BY locktype, mode, granted
      metrics:
        - metric_name: postgresql.locks.by_type
          value_column: lock_count
          attribute_columns: [locktype, mode, granted]
```

## Collection Optimization Strategies

### 1. Tiered Collection Intervals

```yaml
# Critical metrics - collect frequently
receivers:
  postgresql/critical:
    collection_interval: 10s
    metrics:
      postgresql.connection.count:
        enabled: true
      postgresql.locks.waiting:
        enabled: true
        
# Standard metrics - normal interval  
  postgresql/standard:
    collection_interval: 30s
    metrics:
      postgresql.database.size:
        enabled: true
      postgresql.table.size:
        enabled: true
        
# Analytical metrics - less frequent
  sqlquery/analytical:
    collection_interval: 5m
    queries:
      - sql: "SELECT * FROM pg_stat_user_tables"
```

### 2. Dynamic Sampling

```yaml
adaptive_sampler:
  strategies:
    - name: "Critical Path"
      rules:
        - metric_pattern: "*.connections.*|*.locks.*"
          sample_rate: 1.0  # Always sample
          
    - name: "Query Performance"
      rules:
        - metric_pattern: "*.query.*"
          base_rate: 0.1
          spike_multiplier: 10.0
          conditions:
            - query.duration > 1000  # Sample all slow queries
            
    - name: "Volume Metrics"
      rules:
        - metric_pattern: "*.table.rows.*"
          base_rate: 0.05  # 5% sampling for high-volume
```

### 3. Cardinality Management

```yaml
verification:
  cardinality_controls:
    # Limit unique query fingerprints
    - metric_pattern: "*.query.*"
      max_cardinality: 1000
      grouping_rules:
        - normalize_sql: true
        - remove_literals: true
        
    # Aggregate table metrics by schema
    - metric_pattern: "*.table.*"
      max_cardinality: 500
      aggregate_by: [schema]
      
    # Drop user dimension for high-volume
    - metric_pattern: "*.operations.*"
      drop_attributes: [user, client_addr]
```

### 4. Cost-Based Collection

```yaml
costcontrol:
  tiers:
    - name: "Essential"
      budget_percent: 40
      metrics:
        - "*.connections.*"
        - "*.transactions.*"
        - "*.locks.*"
        
    - name: "Performance"
      budget_percent: 40
      metrics:
        - "*.query.*"
        - "*.cache.*"
        - "*.wait.*"
        
    - name: "Analytical"
      budget_percent: 20
      metrics:
        - "*.table.*"
        - "*.index.*"
        - "*.plan.*"
```

## Metric Aggregation Rules

### Time-based Aggregations

```yaml
transform:
  metric_statements:
    # Convert to rates
    - context: metric
      statements:
        - set(name, "postgresql.transactions.rate") where name == "postgresql.transactions.committed"
        - set(unit, "{transactions}/s") where name == "postgresql.transactions.rate"
        
    # Calculate ratios
    - context: datapoint
      statements:
        - set(value, attributes["hits"] / (attributes["hits"] + attributes["reads"]) * 100) 
          where metric.name == "postgresql.cache.hit_ratio"
```

### Dimensional Reductions

```yaml
processors:
  groupbyattrs:
    keys:
      - service.name
      - db.name
      - host.name
    metrics:
      - "postgresql.query.*"
    aggregate:
      type: sum
      
  metricstransform:
    transforms:
      - include: "postgresql.table.*"
        action: aggregate
        aggregation_type: sum
        dimensions: [schema]
```

## Performance Impact Considerations

### Resource Usage by Metric Category

| Category | DB Impact | Network | Storage | Priority |
|----------|-----------|---------|---------|----------|
| Connections | Minimal | Low | Low | Critical |
| Transactions | Minimal | Low | Low | High |
| Resource Util | Low | Medium | Medium | High |
| Query Perf | Medium | High | High | Medium |
| Query Plans | High | High | Very High | Low |
| ASH | Medium | High | High | Medium |
| Tables/Indexes | Low | Medium | Medium | Low |
| Locks | Low | Low | Low | High |

### Overhead Mitigation

1. **Use Read Replicas**
   ```yaml
   postgresql:
     endpoint: "postgresql://read-replica:5432"
     read_only: true
   ```

2. **Circuit Breaker Protection**
   ```yaml
   circuitbreaker:
     triggers:
       - metric: "database.cpu.usage"
         threshold: 80
         action: reduce_collection
       - metric: "collector.query.duration"
         threshold: 1000ms
         action: pause_collection
   ```

3. **Caching Strategies**
   ```yaml
   cache:
     - query: "pg_stat_statements"
       ttl: 30s
     - query: "pg_stat_database"
       ttl: 10s
   ```

## Implementation Roadmap

### Phase 1: Core Metrics (Week 1-2)
- Connections and transactions
- Basic resource utilization
- Essential error tracking

### Phase 2: Performance Metrics (Week 3-4)
- Query performance basics
- Cache and I/O metrics
- Lock monitoring

### Phase 3: Advanced Analytics (Week 5-6)
- Query plan analysis
- ASH implementation
- Pattern detection

### Phase 4: Optimization (Week 7-8)
- Cardinality tuning
- Cost optimization
- Performance validation

## Summary

This comprehensive metrics collection strategy provides:

1. **Complete Coverage**: All critical database observability dimensions
2. **Flexible Implementation**: Phased approach from basic to advanced
3. **Cost Control**: Built-in optimization and budget management
4. **Performance Protection**: Multiple safeguards against impact
5. **Actionable Insights**: Metrics designed for troubleshooting and optimization

The strategy ensures that whether using config-only or enhanced mode, organizations can achieve deep database observability while maintaining control over costs and performance impact.