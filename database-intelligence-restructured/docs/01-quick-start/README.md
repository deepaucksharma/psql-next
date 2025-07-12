# Quick Start Guide

Get database monitoring running in 5 minutes with OpenTelemetry and New Relic.

## Prerequisites

- Docker installed
- New Relic account with license key
- PostgreSQL or MySQL database with read access

## Option 1: Config-Only Mode (Recommended)

This mode works immediately with standard OpenTelemetry Collector images.

### Step 1: Create Environment File

Create a `.env` file with your settings:

```bash
# Copy the template
cp configs/examples/.env.template .env

# Edit with your values
vim .env
```

Required variables:
```bash
# PostgreSQL
DB_POSTGRES_HOST=your-db-host
DB_POSTGRES_PORT=5432
DB_POSTGRES_USER=monitor_user
DB_POSTGRES_PASSWORD=your-password
DB_POSTGRES_DATABASE=postgres

# New Relic
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
NEW_RELIC_LICENSE_KEY=your-license-key

# Service Identity
SERVICE_NAME=my-postgres-prod
ENVIRONMENT=production
```

### Step 2: Run the Collector

```bash
# PostgreSQL monitoring
docker run -d \
  --name otelcol-postgres \
  --env-file .env \
  -v $(pwd)/configs/examples/config-only-working.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector-contrib:0.105.0 \
  --config=/etc/otelcol/config.yaml

# MySQL monitoring  
docker run -d \
  --name otelcol-mysql \
  --env-file .env \
  -v $(pwd)/configs/examples/config-only-mysql.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector-contrib:0.105.0 \
  --config=/etc/otelcol/config.yaml
```

### Step 3: Verify Data in New Relic

Check your metrics in New Relic:

```sql
-- In New Relic Query Builder
SELECT count(*) FROM Metric 
WHERE metricName LIKE 'postgresql.%' 
SINCE 5 minutes ago

-- Check specific metrics
SELECT latest(postgresql.connections.active) 
FROM Metric 
FACET db.name 
SINCE 30 minutes ago
```

## Option 2: Enhanced Mode (Advanced Users)

Enhanced mode provides additional features but requires building a custom collector.

### Prerequisites
- Go 1.22+ installed
- OpenTelemetry Builder installed

### Step 1: Install Builder

```bash
go install go.opentelemetry.io/collector/cmd/builder@v0.105.0
```

### Step 2: Build Custom Collector

```bash
# Build with all custom components
builder --config=otelcol-builder-config-complete.yaml

# Check the output
ls -la distributions/production/database-intelligence-collector
```

### Step 3: Run Enhanced Collector

```bash
# Make sure you have the .env file configured
./distributions/production/database-intelligence-collector \
  --config=configs/examples/enhanced-mode-corrected.yaml
```

## What Metrics Are Collected?

### Config-Only Mode (Available Now)
- **Database Metrics**: Connections, transactions, cache hits, table sizes
- **Custom Queries**: Long-running queries, bloat estimation, connection states
- **Host Metrics**: CPU, memory, disk I/O, network

### Enhanced Mode (With Custom Build)
- **Active Session History**: 1-second sampling of all active sessions
- **Query Plans**: Automatic extraction and analysis
- **Smart Sampling**: Reduces data volume while keeping important metrics
- **OHI Compatibility**: Works with existing New Relic dashboards

## Troubleshooting

### Check Collector Logs
```bash
docker logs otelcol-postgres
```

### Enable Debug Mode
Add to your config:
```yaml
exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      exporters: [otlphttp, debug]
```

### Common Issues

**Issue**: No data in New Relic
- Check your license key is correct
- Verify network connectivity to otlp.nr-data.net
- Ensure database credentials have SELECT permissions

**Issue**: High memory usage
- Adjust memory_limiter processor settings
- Reduce collection_interval for expensive queries
- Enable adaptive sampling (enhanced mode)

**Issue**: "Component not found" errors
- You're trying to use enhanced mode with standard image
- Either switch to config-only mode or build custom collector

## Next Steps

1. **Customize Queries**: Add your own SQL queries to monitor specific metrics
2. **Set Up Alerts**: Create New Relic alerts based on the metrics
3. **Deploy to Production**: See our [deployment guide](../02-deployment/deployment-options.md)
4. **Performance Tuning**: Optimize for your scale with our [tuning guide](../04-operation/performance-tuning.md)

## Example Dashboards

We're working on dashboard templates. For now, you can create custom dashboards in New Relic using queries like:

```sql
-- Connection pool utilization
SELECT average(postgresql.connections.active) / average(postgresql.connection.max) * 100 
as 'Connection Pool Usage %' 
FROM Metric 
TIMESERIES

-- Query performance  
SELECT count(*) as 'Long Running Queries' 
FROM Metric 
WHERE metricName = 'postgresql.queries.long_running.count' 
AND value > 0
TIMESERIES

-- Cache hit ratio
SELECT average(postgresql.blocks.hit) / 
       (average(postgresql.blocks.hit) + average(postgresql.blocks.read)) * 100 
as 'Cache Hit Ratio %' 
FROM Metric 
TIMESERIES
```