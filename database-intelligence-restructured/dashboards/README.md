# Database Intelligence Dashboards

This directory contains dashboard configurations for Database Intelligence monitoring, supporting multiple monitoring platforms and database types.

## Available Dashboards

### Multi-Database Overview Dashboards
Comprehensive monitoring across PostgreSQL, MySQL, MongoDB, and Redis databases.

- **`multi-database-overview.json`** - Grafana dashboard with Prometheus queries
- **`multi-database-overview-newrelic.json`** - New Relic dashboard with NRQL queries

Features:
- Unified health monitoring across all database types
- Performance metrics comparison
- Resource usage tracking
- Replication and high availability status
- Alert correlation and issue tracking

### Cross-Database Correlation Dashboards
Advanced correlation analysis and dependency mapping between databases.

- **`cross-database-correlation.json`** - Grafana dashboard for correlation analysis
- **`cross-database-correlation-newrelic.json`** - New Relic dashboard for correlation analysis

Features:
- Query correlation across databases
- Distributed transaction tracking
- Data flow visualization
- Performance impact analysis
- Resource contention detection
- Anomaly detection for unusual patterns
- Service-to-database dependency mapping

### Legacy and Migration Dashboards
For OHI (On-Host Integration) to OpenTelemetry migration.

## Directory Structure

```
dashboards/
├── multi-database-overview.json         # Grafana multi-DB dashboard
├── multi-database-overview-newrelic.json # New Relic multi-DB dashboard
├── newrelic/           # Legacy OHI dashboards
│   ├── database-intelligence-dashboard.json
│   └── alerts-config.yaml
├── otel/              # New OpenTelemetry dashboards
│   └── database-intelligence-otel.json
└── backups/           # Dashboard backups (created by migration script)
```

## Dashboard Migration

We provide comprehensive tooling to migrate from OHI to OpenTelemetry dashboards.

### Prerequisites

1. **Environment Variables**:
   ```bash
   export NEW_RELIC_API_KEY="your-api-key"
   export NEW_RELIC_ACCOUNT_ID="your-account-id"
   ```

2. **Required Tools**:
   - `jq` - JSON processor
   - `curl` - HTTP client
   - `go` - For validation tool (optional)

### Migration Process

#### 1. Automatic Migration

Use the migration script for a complete migration:

```bash
# Full migration with backup
./scripts/migrate-dashboard.sh full YOUR_DASHBOARD_GUID

# Individual steps
./scripts/migrate-dashboard.sh backup YOUR_DASHBOARD_GUID
./scripts/migrate-dashboard.sh translate
./scripts/migrate-dashboard.sh deploy
```

#### 2. Manual Migration

1. **Export existing OHI dashboard**:
   ```bash
   curl -X POST https://api.newrelic.com/graphql \
     -H "Api-Key: $NEW_RELIC_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"query": "{ dashboardExport(guid: \"YOUR_GUID\") { dashboardJson } }"}'
   ```

2. **Apply query translations** (see Query Translation Guide below)

3. **Import new OTel dashboard**:
   ```bash
   # Use the provided OTel dashboard template
   cat dashboards/otel/database-intelligence-otel.json | \
   curl -X POST https://api.newrelic.com/graphql \
     -H "Api-Key: $NEW_RELIC_API_KEY" \
     -H "Content-Type: application/json" \
     -d @-
   ```

### Query Translation Guide

| OHI Pattern | OTel Replacement |
|-------------|------------------|
| `FROM PostgresSlowQueries` | `FROM Metric WHERE metricName LIKE 'postgres.slow_queries%'` |
| `FROM PostgresWaitEvents` | `FROM Metric WHERE metricName LIKE 'postgres.wait_events%'` |
| `facet query_id` | `FACET attributes.db.postgresql.query_id` |
| `facet database_name` | `FACET attributes.db.name` |
| `avg_elapsed_time_ms` | `postgres.slow_queries.elapsed_time` |
| `execution_count` | `postgres.slow_queries.count` |

### Data Validation

Ensure data parity between OHI and OTel dashboards:

```bash
# Build validation tool
cd tests/dashboard-validation
go build -o validator

# Run validation
./validator
```

The validator will:
- Execute matching queries on both dashboards
- Compare numeric results within tolerance thresholds
- Generate a detailed validation report

## Dashboard Features

### OTel Dashboard Advantages

1. **Unified Metrics Model**: All database metrics follow OTel conventions
2. **Resource Attributes**: Better filtering with `resource.*` attributes
3. **Trace Correlation**: Link slow queries to distributed traces
4. **Custom Dimensions**: Add business-specific attributes
5. **Future-Proof**: Compatible with OTel ecosystem tools

### Key Widgets

1. **Overview Page**:
   - Active databases by query count
   - Average query execution time
   - Current active sessions
   - Top wait events distribution
   - Blocked sessions trend

2. **Query Performance**:
   - Slowest queries table with full details
   - Query execution trends by operation type
   - Disk I/O patterns by database

3. **Active Session History**:
   - Real-time session state monitoring
   - Wait event heatmaps
   - Long-running query detection
   - Blocking chain analysis

4. **Resource Utilization**:
   - Connection pool usage
   - Database size growth
   - Buffer cache efficiency

## Troubleshooting

### Common Issues

1. **Missing Data After Migration**:
   - Ensure OTel collector is running with `ohitransform` processor
   - Verify metric names match the transformation rules
   - Check time range alignment between dashboards

2. **Query Syntax Errors**:
   - Validate all attribute names include proper prefixes
   - Ensure metric names are properly quoted
   - Check for missing WHERE clauses on Metric queries

3. **Performance Differences**:
   - OTel queries may need optimization for large datasets
   - Consider adding metric-specific WHERE clauses
   - Use appropriate time aggregations

### Rollback Procedure

If issues occur, rollback to OHI dashboard:

1. Restore from backup:
   ```bash
   # List available backups
   ls dashboards/backups/
   
   # Restore specific backup
   cat dashboards/backups/dashboard_GUID_timestamp.json | \
   curl -X POST https://api.newrelic.com/graphql \
     -H "Api-Key: $NEW_RELIC_API_KEY" \
     -d @-
   ```

2. Disable OTel collection (keep OHI running)

3. Update alert policies to use OHI queries

## Best Practices

1. **Gradual Migration**:
   - Run both dashboards in parallel initially
   - Compare data daily for first week
   - Migrate alerts only after validation

2. **Query Optimization**:
   - Add specific WHERE clauses for metric names
   - Use LIMIT appropriately for table widgets
   - Leverage FACET for efficient grouping

3. **Monitoring**:
   - Set up alerts for data discrepancies
   - Track dashboard performance metrics
   - Monitor user adoption rates

## Support

For assistance with dashboard migration:

1. Check the [Dashboard Migration Strategy](../docs/DASHBOARD_MIGRATION_STRATEGY.md)
2. Review [Query Mapping Reference](../docs/03-ohi-migration/02-query-mapping.md)
3. Run the validation tool for specific issues
4. Contact the Database Intelligence team

## Future Enhancements

- [ ] Automated A/B testing framework
- [ ] Real-time migration progress tracking
- [ ] Custom widget templates
- [ ] Mobile-optimized layouts
- [ ] Export to Grafana format