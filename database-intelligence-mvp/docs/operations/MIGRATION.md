# Migration Guide

## Overview

This guide provides instructions for migrating to the Database Intelligence Collector from other monitoring solutions or upgrading between versions.

## Table of Contents

1. [Migrating from Native Database Monitoring](#migrating-from-native-database-monitoring)
2. [Migrating from Prometheus Exporters](#migrating-from-prometheus-exporters)
3. [Version Upgrade Guide](#version-upgrade-guide)
4. [Configuration Migration](#configuration-migration)
5. [Data Continuity](#data-continuity)
6. [Rollback Procedures](#rollback-procedures)

## Migrating from Native Database Monitoring

### From pg_stat_statements

If you're currently using PostgreSQL's pg_stat_statements:

1. **Keep pg_stat_statements enabled** - The collector uses it as a data source
2. **Create monitoring user**:
   ```sql
   CREATE USER monitor WITH PASSWORD 'secure_password';
   GRANT pg_read_all_stats TO monitor;
   GRANT SELECT ON pg_stat_statements TO monitor;
   ```
3. **Configure collector**:
   ```yaml
   receivers:
     postgresql:
       endpoint: localhost:5432
       username: monitor
       password: secure_password
       databases:
         - postgres
   ```

### From MySQL Performance Schema

If you're using MySQL Performance Schema:

1. **Keep Performance Schema enabled**
2. **Create monitoring user**:
   ```sql
   CREATE USER 'monitor'@'%' IDENTIFIED BY 'secure_password';
   GRANT SELECT ON performance_schema.* TO 'monitor'@'%';
   GRANT PROCESS ON *.* TO 'monitor'@'%';
   ```
3. **Configure collector**:
   ```yaml
   receivers:
     mysql:
       endpoint: localhost:3306
       username: monitor
       password: secure_password
   ```

## Migrating from Prometheus Exporters

### From postgres_exporter

1. **Metric Name Mapping**:
   | postgres_exporter | Database Intelligence Collector |
   |-------------------|--------------------------------|
   | `pg_stat_database_*` | `postgresql.database.*` |
   | `pg_stat_user_tables_*` | `postgresql.table.*` |
   | `pg_stat_bgwriter_*` | `postgresql.bgwriter.*` |

2. **Update Dashboards**:
   ```promql
   # Old query
   rate(pg_stat_database_xact_commit[5m])
   
   # New query
   rate(postgresql.commits[5m])
   ```

3. **Parallel Running**:
   - Run both exporters during transition
   - Use different ports to avoid conflicts
   - Gradually migrate dashboards and alerts

### From mysqld_exporter

1. **Metric Name Mapping**:
   | mysqld_exporter | Database Intelligence Collector |
   |-----------------|--------------------------------|
   | `mysql_global_status_*` | `mysql.*` |
   | `mysql_info_schema_*` | `mysql.schema.*` |
   | `mysql_perf_schema_*` | `mysql.performance.*` |

2. **Configuration Differences**:
   ```yaml
   # mysqld_exporter uses DSN
   DATA_SOURCE_NAME="user:password@tcp(localhost:3306)/"
   
   # Collector uses structured config
   receivers:
     mysql:
       endpoint: localhost:3306
       username: user
       password: password
   ```

## Version Upgrade Guide

### Upgrading from 0.x to 1.0

1. **Backup Current Configuration**:
   ```bash
   cp config/collector.yaml config/collector.yaml.backup
   ```

2. **Review Breaking Changes**:
   - Processor names have changed
   - Some configuration keys renamed
   - Default ports may have changed

3. **Update Configuration**:
   ```yaml
   # Old (0.x)
   processors:
     sampling:
       percentage: 10
   
   # New (1.0)
   processors:
     adaptivesampler:
       default_sampling_rate: 10
   ```

4. **Test in Staging**:
   ```bash
   # Validate configuration
   ./otelcol validate --config=config/collector.yaml
   
   # Dry run
   ./otelcol --config=config/collector.yaml --dry-run
   ```

### Rolling Upgrade (Kubernetes)

1. **Update ConfigMap**:
   ```bash
   kubectl apply -f k8s/configmap.yaml
   ```

2. **Update Deployment**:
   ```bash
   kubectl set image deployment/database-intelligence-collector \
     collector=database-intelligence-collector:v1.0.0 \
     -n database-intelligence
   ```

3. **Monitor Rollout**:
   ```bash
   kubectl rollout status deployment/database-intelligence-collector \
     -n database-intelligence
   ```

## Configuration Migration

### Environment Variables

Map old environment variables to new ones:

```bash
# Old
export DB_HOST=localhost
export DB_USER=monitor

# New
export POSTGRES_HOST=localhost
export POSTGRES_USER=monitor
```

### Processor Configuration

Migrate processor settings:

```yaml
# Old configuration style
processors:
  batch:
    size: 100
    timeout: 5s

# New configuration style
processors:
  batch:
    send_batch_size: 100
    timeout: 5s
    send_batch_max_size: 200
```

### Export Configuration

Update exporter settings:

```yaml
# Old OTLP configuration
exporters:
  otlp:
    endpoint: "otlp.example.com:4317"
    insecure: true

# New OTLP configuration
exporters:
  otlp:
    endpoint: "otlp.example.com:4318"
    compression: gzip
    headers:
      "api-key": "${NEW_RELIC_LICENSE_KEY}"
```

## Data Continuity

### Metric Name Changes

To maintain dashboard continuity during migration:

1. **Use Prometheus Recording Rules**:
   ```yaml
   groups:
     - name: migration
       rules:
         - record: pg_stat_database_xact_commit
           expr: postgresql.commits
   ```

2. **Grafana Variable Mapping**:
   ```json
   {
     "templating": {
       "list": [{
         "name": "metric_prefix",
         "type": "custom",
         "options": [
           {"text": "postgresql", "value": "postgresql"},
           {"text": "pg_stat_database", "value": "pg_stat_database"}
         ]
       }]
     }
   }
   ```

### Historical Data

To preserve historical metrics:

1. **Export existing data**:
   ```bash
   # Export from Prometheus
   promtool tsdb dump /prometheus/data
   ```

2. **Import to new system**:
   - Use remote write to backfill
   - Or maintain old system for historical queries

## Rollback Procedures

### Quick Rollback

1. **Stop new collector**:
   ```bash
   docker-compose down
   # or
   kubectl scale deployment/database-intelligence-collector --replicas=0
   ```

2. **Restore old configuration**:
   ```bash
   cp config/collector.yaml.backup config/collector.yaml
   ```

3. **Restart old monitoring**:
   ```bash
   # Restart prometheus exporters
   systemctl start postgres_exporter
   systemctl start mysqld_exporter
   ```

### Gradual Rollback

1. **Reduce traffic gradually**:
   - Update load balancer weights
   - Reduce collector replicas
   - Monitor for issues

2. **Maintain both systems**:
   - Keep old system running
   - Compare metrics between systems
   - Fix discrepancies before full migration

## Migration Checklist

- [ ] Backup current monitoring configuration
- [ ] Document current metric names and queries
- [ ] Create monitoring user accounts in databases
- [ ] Deploy collector in test environment
- [ ] Validate metric collection
- [ ] Update one dashboard as pilot
- [ ] Create rollback plan
- [ ] Schedule maintenance window
- [ ] Deploy to production
- [ ] Monitor for 24 hours
- [ ] Update all dashboards and alerts
- [ ] Decommission old monitoring

## Troubleshooting

### Missing Metrics

If metrics are missing after migration:

1. Check collector logs:
   ```bash
   kubectl logs -n database-intelligence deployment/database-intelligence-collector
   ```

2. Verify permissions:
   ```sql
   -- PostgreSQL
   SELECT has_table_privilege('monitor', 'pg_stat_statements', 'SELECT');
   
   -- MySQL
   SHOW GRANTS FOR 'monitor'@'%';
   ```

3. Validate configuration:
   ```bash
   ./otelcol validate --config=config/collector.yaml
   ```

### Performance Issues

If experiencing performance degradation:

1. Adjust collection interval:
   ```yaml
   receivers:
     postgresql:
       collection_interval: 60s  # Increase from default 30s
   ```

2. Enable sampling:
   ```yaml
   processors:
     probabilistic_sampler:
       sampling_percentage: 10
   ```

3. Increase resources:
   ```yaml
   resources:
     limits:
       memory: 2Gi
       cpu: 2000m
   ```

## Support

For migration assistance:

- Documentation: [Full documentation](../README.md)
- Issues: [GitHub Issues](https://github.com/database-intelligence-mvp/issues)
- Community: [Slack #database-intelligence](https://otel-community.slack.com)