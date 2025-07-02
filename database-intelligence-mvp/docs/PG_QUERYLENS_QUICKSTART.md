# pg_querylens Integration Quick Start Guide

This guide helps you quickly set up pg_querylens integration with the Database Intelligence Collector.

## Prerequisites

- PostgreSQL 12+ with superuser access
- Database Intelligence Collector deployed
- New Relic account with OTLP ingest enabled

## Step 1: Install pg_querylens Extension

### Option A: From Source
```bash
# Clone pg_querylens repository
git clone https://github.com/pgexperts/pg_querylens.git
cd pg_querylens

# Build and install
make
sudo make install
```

### Option B: Using Package Manager (if available)
```bash
# Debian/Ubuntu
sudo apt-get install postgresql-14-querylens

# RHEL/CentOS
sudo yum install pg_querylens14
```

## Step 2: Configure PostgreSQL

1. **Add to shared_preload_libraries**:
```bash
# Edit postgresql.conf
sudo vi /etc/postgresql/14/main/postgresql.conf

# Add pg_querylens to shared_preload_libraries
shared_preload_libraries = 'pg_querylens'

# Configure pg_querylens parameters
pg_querylens.enabled = on
pg_querylens.track = 'all'
pg_querylens.max_plans_per_query = 100
pg_querylens.plan_capture_threshold_ms = 100
pg_querylens.sample_rate = 0.1
```

2. **Restart PostgreSQL**:
```bash
sudo systemctl restart postgresql
```

3. **Create Extension**:
```sql
-- Connect as superuser
psql -U postgres

-- Create extension
CREATE EXTENSION pg_querylens;

-- Verify installation
SELECT * FROM pg_extension WHERE extname = 'pg_querylens';
```

## Step 3: Grant Permissions

```sql
-- Create monitoring role if not exists
CREATE ROLE monitoring WITH LOGIN PASSWORD 'secure_password';

-- Grant necessary permissions
GRANT pg_monitor TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_querylens TO monitoring;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA pg_querylens TO monitoring;

-- Grant access to system catalogs
GRANT SELECT ON pg_stat_statements TO monitoring;
GRANT SELECT ON pg_stat_database TO monitoring;
GRANT SELECT ON pg_stat_user_tables TO monitoring;
```

## Step 4: Configure Collector

1. **Update collector configuration**:
```yaml
# config/collector-querylens.yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "host=localhost port=5432 user=monitoring password=${POSTGRES_PASSWORD} dbname=postgres sslmode=require"
    collection_interval: 30s
    queries:
      - sql: |
          SELECT 
            queryid, plan_id, plan_text, mean_exec_time_ms,
            calls, rows, shared_blks_hit, shared_blks_read
          FROM pg_querylens.current_plans
          WHERE last_execution > NOW() - INTERVAL '5 minutes'
        metrics:
          - metric_name: db.querylens.query.execution_time_mean
            value_column: mean_exec_time_ms
            value_type: double
            data_point_type: gauge

processors:
  planattributeextractor:
    querylens:
      enabled: true
      regression_detection:
        enabled: true
        time_increase: 1.5

exporters:
  otlp:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [sqlquery]
      processors: [planattributeextractor]
      exporters: [otlp]
```

2. **Set environment variables**:
```bash
export POSTGRES_PASSWORD='secure_password'
export NEW_RELIC_LICENSE_KEY='your-license-key'
export NEW_RELIC_OTLP_ENDPOINT='otlp.nr-data.net:4317'
```

3. **Start collector**:
```bash
./otelcol-custom --config=config/collector-querylens.yaml
```

## Step 5: Verify Data Collection

1. **Check pg_querylens is collecting data**:
```sql
-- View current plans
SELECT queryid, plan_id, calls, mean_exec_time_ms 
FROM pg_querylens.current_plans 
LIMIT 10;

-- Check plan history
SELECT queryid, count(distinct plan_id) as plan_versions 
FROM pg_querylens.plan_history 
GROUP BY queryid 
HAVING count(distinct plan_id) > 1;
```

2. **Verify in New Relic**:
```sql
-- Run in New Relic Query Builder
SELECT count(*) 
FROM Metric 
WHERE metricName LIKE 'db.querylens%' 
SINCE 5 minutes ago
```

## Step 6: Import Dashboard

1. **Download dashboard**:
```bash
curl -O https://raw.githubusercontent.com/database-intelligence-mvp/database-intelligence-collector/main/dashboards/pg-querylens-dashboard.json
```

2. **Import to New Relic**:
   - Go to New Relic One > Dashboards
   - Click "Import dashboard"
   - Select the downloaded JSON file
   - Configure account ID

## Troubleshooting

### No Data in New Relic

1. **Check collector logs**:
```bash
grep -i querylens collector.log
```

2. **Verify connectivity**:
```bash
psql -h localhost -U monitoring -d postgres -c "SELECT 1 FROM pg_querylens.current_plans LIMIT 1;"
```

3. **Check pg_querylens status**:
```sql
SHOW pg_querylens.enabled;
SELECT * FROM pg_querylens.stats;
```

### High Overhead

If pg_querylens causes performance issues:

1. **Increase capture threshold**:
```sql
ALTER SYSTEM SET pg_querylens.plan_capture_threshold_ms = 500;
SELECT pg_reload_conf();
```

2. **Reduce sample rate**:
```sql
ALTER SYSTEM SET pg_querylens.sample_rate = 0.05;
SELECT pg_reload_conf();
```

3. **Limit plans per query**:
```sql
ALTER SYSTEM SET pg_querylens.max_plans_per_query = 20;
SELECT pg_reload_conf();
```

## Best Practices

1. **Start Conservative**: Begin with high thresholds and low sample rates
2. **Monitor Overhead**: Watch pg_querylens statistics table for resource usage
3. **Regular Maintenance**: Periodically clean old plan history
4. **Selective Tracking**: Consider tracking only specific databases or users
5. **Coordinate with DBAs**: Ensure database team is aware of monitoring

## Next Steps

- Configure alerts for plan regressions
- Set up automated reports for top queries
- Integrate with CI/CD for query performance testing
- Explore advanced features like plan pinning