# Migration Guide: Configuration Updates

This guide helps you migrate existing Database Intelligence Collector deployments to the new configuration format.

## Overview

Critical configuration changes are required to ensure:
1. Dashboard queries return data
2. Database connections work properly
3. Collector starts successfully

## Migration Checklist

- [ ] Update environment variable syntax
- [ ] Add resource processor with collector.name
- [ ] Fix memory limiter configuration
- [ ] Add logs section to SQL query receivers
- [ ] Remove deprecated extensions
- [ ] Update pipeline processor order
- [ ] Validate configuration

## Step-by-Step Migration

### Step 1: Update Environment Variables

**Find and replace all occurrences:**

| Old Syntax | New Syntax |
|------------|------------|
| `${POSTGRES_HOST:localhost}` | `${env:POSTGRES_HOST:-localhost}` |
| `${POSTGRES_PORT:5432}` | `${env:POSTGRES_PORT:-5432}` |
| `${POSTGRES_USER:postgres}` | `${env:POSTGRES_USER:-postgres}` |
| `${POSTGRES_PASSWORD:postgres}` | `${env:POSTGRES_PASSWORD:-postgres}` |
| `${MYSQL_HOST:localhost}` | `${env:MYSQL_HOST:-localhost}` |
| `${NEW_RELIC_LICENSE_KEY}` | `${env:NEW_RELIC_LICENSE_KEY}` |
| `${HOSTNAME}` | `${env:HOSTNAME}` |

### Step 2: Add Resource Processor (CRITICAL)

**This is required for dashboards to work!**

Add to your processors section:
```yaml
processors:
  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert
      - key: collector.instance.id
        value: ${env:HOSTNAME}
        action: upsert
```

Update your pipelines to include the resource processor:
```yaml
service:
  pipelines:
    metrics:
      processors: [memory_limiter, resource, batch]  # resource is required!
```

### Step 3: Fix Memory Limiter

Replace percentage-based configuration:

**Before:**
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
```

**After:**
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1024
    spike_limit_mib: 256
```

### Step 4: Fix SQL Query Receivers

Add logs configuration to all sqlquery receivers:

**Before:**
```yaml
receivers:
  sqlquery/postgresql:
    queries:
      - sql: "SELECT ..."
```

**After:**
```yaml
receivers:
  sqlquery/postgresql:
    queries:
      - sql: "SELECT ..."
        logs:
          - body_column: query_text
            attributes:
              query_id: query_id
              avg_duration_ms: avg_duration_ms
              execution_count: execution_count
              total_duration_ms: total_duration_ms
              database_name: database_name
```

### Step 5: Remove Deprecated Extensions

Remove from extensions section:
- `memory_ballast` (use memory_limiter processor instead)
- `leader_election` (unless specifically built)

**Before:**
```yaml
service:
  extensions: [health_check, memory_ballast, zpages]
```

**After:**
```yaml
service:
  extensions: [health_check, zpages]
```

### Step 6: Update Processor Order

Ensure processors are in the correct order:

```yaml
service:
  pipelines:
    metrics/databases:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, transform/metrics, batch]
      exporters: [otlp/newrelic]
      
    logs/queries:
      receivers: [sqlquery/postgresql, sqlquery/mysql]
      processors: [memory_limiter, resource, transform/logs, transform/sanitize_pii, batch]
      exporters: [otlp/newrelic]
```

## Automated Migration Script

Run the provided script to automatically fix common issues:

```bash
./scripts/fix-all-configs.sh
```

This will:
- Backup your existing configs
- Fix environment variable syntax
- Update memory limiter settings
- Remove deprecated extensions
- Generate a report of manual fixes needed

## Validation

After migration, validate your configuration:

```bash
./dist/database-intelligence-collector validate --config=your-config.yaml
```

## Rollback Plan

If issues occur:
1. Configuration backups are created in `config-backup-YYYYMMDD-HHMMSS/`
2. Restore from backup: `cp config-backup-*/your-config.yaml.backup your-config.yaml`
3. Report issues to the team

## Common Migration Issues

### Issue 1: Dashboard Shows No Data
**Cause:** Missing `collector.name = 'otelcol'` attribute
**Fix:** Ensure resource processor is added and included in all pipelines

### Issue 2: Connection Failures
**Cause:** Old environment variable syntax
**Fix:** Update all variables to use `${env:VAR:-default}` format

### Issue 3: Collector Won't Start
**Cause:** Invalid memory_limiter or missing logs section
**Fix:** Update to MiB values and add logs configuration to sqlquery receivers

### Issue 4: "unknown type" Errors
**Cause:** Extension not built into collector
**Fix:** Remove the extension or rebuild collector with required extensions

## Testing After Migration

1. **Start collector with debug logging:**
   ```bash
   ./dist/database-intelligence-collector --config=your-config.yaml --set=service.telemetry.logs.level=debug
   ```

2. **Check metrics are being collected:**
   ```bash
   curl http://localhost:8888/metrics | grep -E "^(postgresql_|mysql_)"
   ```

3. **Verify dashboard queries return data:**
   - Check New Relic for metrics with `collector.name = 'otelcol'`
   - Verify query performance widgets show data

## Support

If you encounter issues:
1. Check the troubleshooting section in CONFIGURATION_GUIDE.md
2. Review collector logs for specific errors
3. Use debug exporter to verify data flow