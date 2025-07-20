# MySQL Intelligence Dashboard Guide

This guide describes the comprehensive MySQL monitoring dashboards that maximize value from all collected metrics.

## Dashboard Overview

### 1. MySQL Intelligence Overview
**Purpose**: Executive-level MySQL health and performance monitoring  
**Key Features**:
- Real-time availability and uptime tracking
- Query performance trends by operation type
- Buffer pool utilization and hit ratios
- Active connection monitoring
- Top tables by I/O with workload classification
- Lock contention analysis with heatmaps
- Index effectiveness scoring

**Use Cases**:
- Executive reporting
- Quick health checks
- Performance baseline establishment
- Capacity planning inputs

### 2. Query Performance Deep Dive
**Purpose**: Detailed query execution analysis and optimization  
**Key Features**:
- Query type distribution and throughput trends
- Handler operations breakdown
- Row operations analysis
- Table access patterns by workload type
- Lock wait time analysis by table
- Performance bottleneck identification
- Temporary resource usage tracking

**Use Cases**:
- Query optimization
- Slow query identification
- Lock contention resolution
- Resource usage optimization

### 3. Index & I/O Optimization
**Purpose**: Index effectiveness and I/O performance optimization  
**Key Features**:
- Index health scoring with recommendations
- Unused index identification
- Low selectivity index detection
- I/O operations analysis (table vs index)
- Storage savings estimates
- Workload distribution insights
- Actionable optimization recommendations

**Use Cases**:
- Index maintenance planning
- Storage optimization
- Query performance tuning
- I/O bottleneck resolution

### 4. MySQL Operational Excellence
**Purpose**: Real-time operations, alerting, and capacity planning  
**Key Features**:
- Live system health monitoring
- Connection pool utilization
- Query response time percentiles
- Critical performance alerts
- Resource saturation indicators
- Capacity growth projections
- Optimization ROI estimates

**Use Cases**:
- 24/7 operational monitoring
- Alert condition management
- Capacity planning
- Budget justification for optimizations

## Key Metrics Utilized

### Core MySQL Metrics
- `mysql.up` - Availability status
- `mysql.uptime_seconds` - System uptime
- `mysql.threads` - Connection monitoring
- `mysql.operations` - Query throughput by type
- `mysql.buffer_pool_*` - Memory efficiency

### Intelligence Metrics
- `mysql.table_iops_estimate` - Table I/O patterns
- `mysql.index_effectiveness_score` - Index health
- `mysql.lock_wait_milliseconds` - Lock contention
- `mysql.table_io_wait_*` - I/O performance
- `mysql.tmp_resources` - Temporary resource usage

### Derived Insights
- Workload classification (read/write/mixed)
- Time-based patterns (business hours/off-hours)
- Index recommendations (drop/optimize/keep)
- Lock contention levels (low/medium/high/critical)
- Performance optimization scores

## Best Practices

1. **Start with Overview Dashboard**
   - Get high-level health status
   - Identify areas needing attention
   - Track trends over time

2. **Drill Down for Details**
   - Use Query Performance dashboard for slow queries
   - Check Index & I/O dashboard for optimization opportunities
   - Monitor Operational Excellence for real-time issues

3. **Regular Reviews**
   - Weekly: Check optimization recommendations
   - Monthly: Review capacity trends
   - Quarterly: Analyze growth projections

4. **Alert Configuration**
   - Set alerts based on Operational Excellence thresholds
   - Monitor critical lock wait times (>100ms)
   - Track buffer pool utilization (>90%)
   - Watch connection pool usage (>80%)

## Implementation Notes

All dashboards are configured to:
- Filter by `entity.type = 'MYSQL_QUERY_INTELLIGENCE'`
- Use appropriate time windows for each metric type
- Provide actionable insights, not just data
- Support drill-down capabilities
- Include visual thresholds for quick status assessment

## Dashboard Import

To import these dashboards to New Relic:

1. Navigate to New Relic One > Dashboards
2. Click "Import dashboard"
3. Select the JSON file for each dashboard
4. Adjust the account ID if needed
5. Save and share with your team

## Customization

Feel free to customize these dashboards by:
- Adjusting time windows
- Adding custom NRQL queries
- Modifying thresholds
- Creating filtered versions for specific databases
- Adding additional widgets for custom metrics