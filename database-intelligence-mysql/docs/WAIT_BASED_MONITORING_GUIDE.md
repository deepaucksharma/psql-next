# MySQL Wait-Based Performance Monitoring Guide

## Overview

This guide covers the deployment and operation of a comprehensive MySQL wait-based performance monitoring solution using OpenTelemetry and New Relic. The architecture follows proven methodologies from SolarWinds DPA, focusing on **what matters most: wait times**.

## Architecture

```
┌─────────────────────┐     ┌─────────────────────┐     ┌─────────────────────┐
│   MySQL Primary     │     │   MySQL Replica     │     │                     │
│  (Performance       │     │  (Performance       │     │    New Relic        │
│   Schema Enabled)   │     │   Schema Enabled)   │     │   (Dashboards &     │
└──────────┬──────────┘     └──────────┬──────────┘     │     Alerts)         │
           │                           │                 └──────────▲──────────┘
           │                           │                            │
           ▼                           ▼                            │
┌─────────────────────────────────────────────────┐                │
│         Edge Collector (Wait Analysis)          │                │
│  • Collects wait events every 10s               │                │
│  • Correlates waits with queries                │                │
│  • Generates wait profiles                      │                │
│  • Detects advisories                          │                │
│  • Ultra-low overhead (<1%)                    │                │
└─────────────────────────┬───────────────────────┘                │
                          │                                         │
                          ▼                                         │
┌─────────────────────────────────────────────────┐                │
│       Gateway Collector (Advisory Processing)    │                │
│  • Enriches with composite advisories           │────────────────┘
│  • Controls cardinality                         │
│  • Detects anomalies & trends                   │
│  • Exports to New Relic                         │
└─────────────────────────────────────────────────┘
```

## Key Features

### 1. Wait-Time First Methodology
- Every metric oriented around response time impact
- Wait categories: I/O, Lock, CPU, Network, System
- Wait severity: Critical (>90%), High (70-90%), Medium (50-70%), Low (<50%)

### 2. Intelligent Query Advisors
- **Missing Index Detection**: Identifies full table scans with high I/O waits
- **Lock Contention Analysis**: Detects blocking chains and lock escalation
- **Inefficient Join Detection**: Finds queries with poor join strategies
- **Temp Table Advisory**: Identifies on-disk temporary table creation
- **Composite Advisories**: Complex issue detection (e.g., lock escalation due to missing index)

### 3. Plan Change Detection
- Lightweight fingerprinting without storing full plans
- Tracks execution characteristics changes
- Alerts on performance regressions

### 4. Ultra-Low Overhead
- <1% CPU usage
- <400MB memory for edge collector
- Smart sampling and batching
- Performance Schema optimizations

## Deployment

### Prerequisites

1. **MySQL 8.0+** with Performance Schema support
2. **Docker** and **Docker Compose**
3. **New Relic License Key**
4. **System Requirements**:
   - 2+ CPU cores
   - 4GB+ RAM
   - 10GB+ disk space

### Quick Start

```bash
# 1. Clone the repository
git clone <repository-url>
cd database-intelligence-mysql

# 2. Set environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# 3. Deploy the monitoring stack
./scripts/deploy-wait-monitoring.sh

# 4. Generate test workload (optional)
./scripts/deploy-wait-monitoring.sh --skip-checks --with-workload
```

### Manual Deployment

```bash
# 1. Start services
docker-compose up -d

# 2. Verify health
curl http://localhost:13133/  # Edge collector
curl http://localhost:13134/  # Gateway

# 3. Check metrics
curl http://localhost:9091/metrics | grep mysql_query_wait_profile
```

## Configuration

### Performance Schema Setup

The initialization scripts automatically configure Performance Schema for optimal wait analysis:

```sql
-- Enable wait instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/%' OR NAME LIKE 'statement/%';

-- Enable consumers
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME IN (
  'events_waits_current',
  'events_waits_history_long',
  'events_statements_history_long'
);
```

### Collector Configuration

#### Edge Collector (`config/edge-collector-wait.yaml`)
- Collects wait profiles every 10 seconds
- Performs initial advisory detection
- Implements memory limiting (384MB)
- Batches metrics (2000 per batch)

#### Gateway Collector (`config/gateway-advisory.yaml`)
- Generates composite advisories
- Controls cardinality through sampling
- Enriches with baselines and trends
- Exports to New Relic

## Monitoring

### Real-Time Monitoring

Use the provided monitoring script:

```bash
# Show summary (default)
./scripts/monitor-waits.sh

# Show only blocking analysis
./scripts/monitor-waits.sh blocking

# Show all information with 10s refresh
REFRESH_INTERVAL=10 ./scripts/monitor-waits.sh all
```

### New Relic Dashboards

Import the provided dashboards:

1. **Wait Analysis Dashboard** (`dashboards/newrelic/wait-analysis-dashboard.json`)
   - Total database wait time
   - Wait category breakdown
   - Top wait contributors
   - Active advisories
   - Blocking analysis

2. **Query Detail Dashboard** (`dashboards/newrelic/query-detail-dashboard.json`)
   - Per-query wait profiles
   - Execution trends
   - Plan changes
   - Advisory history

### Key Metrics

| Metric | Description | Use Case |
|--------|-------------|----------|
| `mysql.query.wait_profile` | Wait time by query and category | Identify slow queries |
| `mysql.blocking.active` | Active blocking duration | Detect lock contention |
| `mysql.query.execution_stats` | Query execution statistics | Track plan changes |
| `wait.severity` | Wait severity classification | Prioritize issues |
| `advisor.type` | Performance advisory type | Guide optimization |

## Alerts

### Critical Alerts

1. **Critical Query Wait Time** (>90% wait)
   ```
   Action: Check advisor.recommendation
   Review wait.category
   Implement suggested fix
   ```

2. **Database Blocking Chain** (>60s)
   ```
   Action: Identify blocking session
   Review transaction logic
   Consider killing blocker
   ```

3. **P1 Performance Advisory**
   ```
   Action: Immediate attention required
   Follow advisory recommendation
   Test fix in non-production
   ```

### Warning Alerts

1. **High Impact Missing Index** (>10s total wait)
2. **Query Plan Regression** (5x slower)
3. **I/O Wait Saturation** (>30s total)

## Troubleshooting

### Common Issues

#### 1. No Metrics Appearing

```bash
# Check collector logs
docker-compose logs otel-collector-edge
docker-compose logs otel-gateway

# Verify Performance Schema
docker exec mysql-primary mysql -u root -prootpassword \
  -e "SELECT @@performance_schema"
```

#### 2. High Memory Usage

```bash
# Check memory limits
docker stats otel-collector-edge

# Adjust in docker-compose.yml if needed
GOMEMLIMIT: "300MiB"
```

#### 3. Missing Advisories

```bash
# Verify advisory processing
curl http://localhost:9091/metrics | grep advisor_type

# Check gateway logs for errors
docker-compose logs otel-gateway | grep advisor
```

### Performance Tuning

#### Reduce Collection Overhead

1. Increase collection interval:
   ```yaml
   collection_interval: 30s  # Default: 10s
   ```

2. Limit query count:
   ```sql
   LIMIT 50  # Default: 100
   ```

3. Adjust batching:
   ```yaml
   send_batch_size: 1000  # Default: 2000
   ```

#### Optimize Cardinality

1. Enable aggressive sampling:
   ```yaml
   - 'attributes["wait.severity"] == "low" and rand() < 0.05'
   ```

2. Aggregate by query pattern:
   ```yaml
   label_set: ["query_pattern", "wait.category"]
   ```

## Best Practices

### 1. Start Small
- Deploy to one MySQL instance first
- Monitor overhead and adjust
- Gradually expand coverage

### 2. Focus on P1/P2 Advisories
- Address critical issues first
- Track resolution effectiveness
- Document common patterns

### 3. Establish Baselines
- Monitor for 1 week minimum
- Identify normal wait patterns
- Set appropriate thresholds

### 4. Regular Reviews
- Weekly advisory review
- Monthly trend analysis
- Quarterly threshold adjustment

## Advanced Topics

### Custom Advisories

Add custom advisory rules in `edge-collector-wait.yaml`:

```yaml
- set(attributes["advisor.type"], "custom_slow_join")
  where attributes["wait.category"] == "cpu" 
    and attributes["full_joins"] > 0
    and attributes["statement_time_ms"] > 5000
```

### Integration with CI/CD

```yaml
# GitHub Actions example
- name: Check Query Performance
  run: |
    metrics=$(curl -s http://monitoring.example.com:9091/metrics)
    critical=$(echo "$metrics" | grep 'wait_severity="critical"' | wc -l)
    if [ $critical -gt 0 ]; then
      echo "Critical wait times detected!"
      exit 1
    fi
```

### Multi-Region Deployment

```yaml
# Add region tags
resource:
  attributes:
    - key: cloud.region
      value: ${AWS_REGION}
    - key: cloud.availability_zone
      value: ${AWS_AZ}
```

## Support

### Getting Help

1. Check logs: `docker-compose logs`
2. Run diagnostics: `./scripts/validate-metrics.sh`
3. Review this guide
4. Contact support with:
   - Collector logs
   - Sample metrics
   - Configuration files

### Useful Commands

```bash
# View real-time waits
./scripts/monitor-waits.sh

# Validate metrics collection
./scripts/validate-metrics.sh

# Generate test load
docker exec mysql-primary mysql -u root -prootpassword \
  wait_analysis_test -e "CALL generate_io_waits()"

# Check Performance Schema
docker exec mysql-primary mysql -u root -prootpassword \
  -e "SELECT * FROM performance_schema.setup_instruments 
      WHERE NAME LIKE 'wait/%' AND ENABLED='NO'"
```

## Conclusion

This wait-based monitoring solution provides deep insights into MySQL performance by focusing on what matters most - the time users spend waiting. By following this guide, you'll have a production-ready monitoring system that:

- Identifies performance issues before users complain
- Provides actionable recommendations
- Maintains ultra-low overhead
- Scales to hundreds of MySQL instances

Remember: **If users aren't waiting, there's no performance problem.**