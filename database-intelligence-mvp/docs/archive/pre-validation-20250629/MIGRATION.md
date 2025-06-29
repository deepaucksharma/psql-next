# Migration Guide

## Overview

This guide helps migrate from legacy database monitoring solutions to the Database Intelligence Collector.

## Migration Paths

### From Custom Implementation

If migrating from the earlier custom receiver implementation:

1. **Update Configuration**
   ```yaml
   # Old (custom receiver)
   receivers:
     postgresqlquery:
       datasource: "postgres://..."
       
   # New (standard receivers)
   receivers:
     postgresql:
       endpoint: ${env:POSTGRES_HOST}:5432
     sqlquery:
       driver: postgres
   ```

2. **Update Processors**
   - Adaptive sampling: Now a separate processor
   - Circuit breaker: Now a separate processor
   - Remove custom domain logic

3. **Simplify Deployment**
   - Single binary instead of multiple components
   - Unified configuration
   - Standard OTEL patterns

### From Prometheus + Exporters

1. **Replace Exporters**
   ```yaml
   # Remove postgres_exporter, mysqld_exporter
   # Add OTEL receivers
   receivers:
     postgresql:
       endpoint: localhost:5432
     mysql:
       endpoint: localhost:3306
   ```

2. **Update Scrape Configs**
   ```yaml
   # Prometheus can scrape OTEL collector
   exporters:
     prometheus:
       endpoint: 0.0.0.0:8889
   ```

### From New Relic Infrastructure Agent

1. **Install Collector**
   ```bash
   # Alongside existing agent
   docker run -d database-intelligence:latest
   ```

2. **Configure OTLP Export**
   ```yaml
   exporters:
     otlp:
       endpoint: otlp.nr-data.net:4317
       headers:
         api-key: ${NEW_RELIC_LICENSE_KEY}
   ```

3. **Verify Data Flow**
   - Check both collectors running
   - Compare metrics in New Relic
   - Gradually transition

## Data Mapping

### Metric Name Changes

| Old Metric | New Metric | Notes |
|------------|------------|-------|
| `postgresql.database_size` | `postgresql.database.size` | OTEL semantic conventions |
| `pg_stat_statements_calls` | `db.query.calls` | Normalized naming |
| `active_connections` | `postgresql.connection.count` | Standard receiver metric |

### Attribute Changes

| Old Attribute | New Attribute | Example |
|---------------|---------------|---------|
| `database` | `db.name` | `postgres` |
| `table_name` | `db.sql.table` | `users` |
| `query_hash` | `db.query.id` | `12345` |

## Step-by-Step Migration

### Phase 1: Preparation (Week 1)

1. **Inventory Current Monitoring**
   ```bash
   # List current metrics
   curl http://prometheus:9090/api/v1/label/__name__/values
   
   # Document dashboards
   # Document alerts
   ```

2. **Set Up Test Environment**
   ```bash
   # Deploy collector in test
   docker-compose -f docker-compose.test.yaml up -d
   ```

3. **Validate Metrics**
   - Compare metric values
   - Verify all data present
   - Test dashboards

### Phase 2: Parallel Run (Week 2)

1. **Deploy Alongside Legacy**
   ```yaml
   # Run both collectors
   services:
     legacy-exporter:
       image: postgres_exporter:latest
     otel-collector:
       image: database-intelligence:latest
   ```

2. **Duplicate Data to New Relic**
   - Both systems send data
   - Tag with `collector.type`
   - Compare in dashboards

### Phase 3: Transition (Week 3)

1. **Update Dashboards**
   ```sql
   -- Old query
   SELECT average(postgresql.database_size)
   FROM Metric
   WHERE appName = 'PostgreSQL'
   
   -- New query  
   SELECT average(postgresql.database.size)
   FROM Metric
   WHERE service.name = 'database-monitoring'
   ```

2. **Update Alerts**
   ```yaml
   # Update NRQL conditions
   SELECT count(*)
   FROM Metric
   WHERE metricName = 'db.query.calls'
   AND db.query.mean_time > 1000
   ```

### Phase 4: Cutover (Week 4)

1. **Stop Legacy Collector**
   ```bash
   docker stop legacy-exporter
   ```

2. **Monitor for Issues**
   - Check for missing data
   - Verify alert functionality
   - Monitor performance

3. **Clean Up**
   - Remove old configurations
   - Archive legacy code
   - Update documentation

## Rollback Plan

If issues arise during migration:

1. **Immediate Rollback**
   ```bash
   # Stop new collector
   docker stop otel-collector
   
   # Restart legacy
   docker start legacy-exporter
   ```

2. **Data Recovery**
   - New Relic retains historical data
   - No data loss during parallel run
   - Dashboards can query both sources

## Validation Checklist

- [ ] All databases monitored
- [ ] Query performance metrics present
- [ ] Active session data collected
- [ ] Dashboards updated and working
- [ ] Alerts firing correctly
- [ ] Performance acceptable
- [ ] No data gaps

## Common Migration Issues

### Missing Metrics

**Problem**: Some metrics not appearing
**Solution**: Check receiver configuration includes all databases

### Different Metric Values

**Problem**: Values don't match legacy exactly
**Solution**: 
- Check collection intervals
- Verify calculation methods
- Some differences expected due to implementation

### Performance Impact

**Problem**: Higher load on database
**Solution**:
- Enable adaptive sampling
- Adjust collection intervals
- Use circuit breaker

## Post-Migration

1. **Optimize Configuration**
   - Fine-tune sampling rates
   - Adjust batch sizes
   - Review resource usage

2. **Document Changes**
   - Update runbooks
   - Record metric mappings
   - Note configuration decisions

3. **Train Team**
   - OTEL concepts
   - New tooling
   - Troubleshooting procedures