# Database Intelligence Dashboard Guide

## Overview

The Database Intelligence Dashboard provides comprehensive monitoring for PostgreSQL and MySQL databases using metrics collected via OpenTelemetry. This dashboard is based on the MySQL OHI dashboard structure but adapted for OTEL metrics.

## Prerequisites

1. **OTEL Collector Running**: The Database Intelligence collector must be actively collecting metrics
2. **New Relic Account**: Valid New Relic account with API access
3. **API Key**: New Relic User API key with dashboard creation permissions
4. **Node.js**: Required for running the dashboard scripts

## Dashboard Structure

The dashboard contains three main pages:

### 1. Bird's-Eye View
Overview of database performance with widgets for:
- **Databases Overview**: Count of monitored databases
- **Query Execution Time**: Average query duration trends for PostgreSQL
- **Active Connections**: Real-time connection monitoring
- **Database Disk Usage**: Storage utilization by database
- **Slowest Queries**: Table view of top slow queries from logs
- **PostgreSQL Commits vs Rollbacks**: Transaction success rate monitoring
- **InnoDB Buffer Pool Usage**: MySQL memory utilization
- **Database Operations Overview**: Summary table of key metrics

### 2. Query Performance
Detailed query analysis with:
- **Query Log Analysis**: Comprehensive table of query performance data
- **Query Execution Trends**: Time-series view of query volume
- **Average Query Duration by Database**: Comparative performance metrics

### 3. Database Resources
Resource utilization metrics including:
- **PostgreSQL Table I/O**: Block read operations by source
- **MySQL Handler Operations**: Handler operation rates
- **PostgreSQL Background Writer**: Buffer management metrics
- **MySQL Temporary Resources**: Temporary table/file usage
- **Connection and Thread Status**: Detailed connection information

## Metric Mapping

The dashboard uses the following OTEL metric mappings:

### PostgreSQL Metrics
| Dashboard Widget | OTEL Metric | Description |
|-----------------|-------------|-------------|
| Active Connections | `postgresql.backends` | Number of active backend connections |
| Commits/Rollbacks | `postgresql.commits`, `postgresql.rollbacks` | Transaction rates |
| Disk Usage | `postgresql.database.disk_usage` | Database size in bytes |
| Block I/O | `postgresql.blocks_read` | Block read operations |
| Background Writer | `postgresql.bgwriter.*` | Buffer management metrics |

### MySQL Metrics
| Dashboard Widget | OTEL Metric | Description |
|-----------------|-------------|-------------|
| Thread Count | `mysql.threads` | Active thread count |
| Buffer Pool | `mysql.buffer_pool.*` | InnoDB buffer pool metrics |
| Handler Operations | `mysql.handlers` | Various handler operation counts |
| Uptime | `mysql.uptime` | Server uptime in seconds |
| Temp Resources | `mysql.tmp_resources` | Temporary resource usage |

### Query Logs
Query performance data is collected via the `sqlquery` receiver and appears in New Relic Logs with:
- `query_id`: Unique query identifier
- `query_text`: Sanitized query text
- `avg_duration_ms`: Average execution time
- `execution_count`: Number of executions
- `total_duration_ms`: Total time spent

## Deployment Instructions

### 1. Configure Environment

Ensure your `.env` file contains:
```bash
NEW_RELIC_USER_KEY=your_user_api_key_here
NEW_RELIC_ACCOUNT_ID=your_account_id
```

### 2. Verify Metrics Collection

Run the verification script to ensure metrics are being collected:
```bash
node scripts/verify-collected-metrics.js
```

This will check for:
- PostgreSQL metrics presence
- MySQL metrics presence
- Query log data
- Available dimensions

### 3. Deploy the Dashboard

Use the deployment script for a guided process:
```bash
./scripts/deploy-database-dashboard.sh
```

Or create the dashboard directly:
```bash
node scripts/create-database-dashboard.js
```

### 4. Access Your Dashboard

After successful creation, you'll receive:
- Dashboard GUID
- Direct URL to access the dashboard
- A `dashboard-info.json` file with dashboard details

## Customization

### Modifying Queries

To customize NRQL queries, edit `scripts/create-database-dashboard.js`:

```javascript
nrqlQueries: [
  {
    accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
    query: `YOUR_CUSTOM_NRQL_QUERY_HERE`
  }
]
```

### Adding New Widgets

Add widget definitions to the appropriate page in the dashboard definition:

```javascript
{
  title: "Your Widget Title",
  layout: {
    column: 1,
    row: 1,
    width: 4,
    height: 3
  },
  visualization: {
    id: "viz.line" // or viz.bar, viz.table, etc.
  },
  rawConfiguration: {
    nrqlQueries: [{
      accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
      query: `SELECT count(*) FROM Metric WHERE metricName = 'your.metric'`
    }]
  }
}
```

### Visualization Types

Available visualization types:
- `viz.line`: Line charts for time series
- `viz.bar`: Bar charts for comparisons
- `viz.table`: Tables for detailed data
- `viz.area`: Area charts for cumulative metrics
- `viz.stacked-bar`: Stacked bars for multi-dimensional data

## Troubleshooting

### No Data Appearing

1. **Check Collector Status**: Ensure the OTEL collector is running
   ```bash
   docker-compose ps
   ```

2. **Verify Database Connections**: Check collector logs
   ```bash
   docker-compose logs otel-collector
   ```

3. **Confirm Data Export**: Look for successful export messages in logs

4. **Run Verification**: Use the verification script to check what data is available
   ```bash
   node scripts/verify-collected-metrics.js
   ```

### API Errors

- **401 Unauthorized**: Check your NEW_RELIC_USER_KEY is valid
- **403 Forbidden**: Ensure your API key has dashboard creation permissions
- **Account ID Mismatch**: Verify NEW_RELIC_ACCOUNT_ID matches your account

### Missing Metrics

If specific database metrics are missing:
- PostgreSQL: Ensure `pg_stat_statements` extension is enabled
- MySQL: Verify Performance Schema is enabled
- Both: Check user permissions for metric collection

## Maintenance

### Updating the Dashboard

To update an existing dashboard:
1. Delete the old dashboard in New Relic UI
2. Run the creation script again
3. Or use NerdGraph mutations to update specific widgets

### Monitoring Dashboard Health

Regular checks:
- Verify all widgets show data
- Check for query timeouts
- Monitor dashboard load times
- Review metric cardinality

## Advanced Features

### Adding Variables

To add dashboard variables for filtering:

```javascript
variables: [
  {
    name: "database",
    title: "Database",
    type: "NRQL",
    nrqlQuery: {
      accountIds: [parseInt(NEW_RELIC_ACCOUNT_ID)],
      query: `SELECT uniques(dimensions.database_name) FROM Metric`
    }
  }
]
```

### Linking Entities

To link dashboard widgets to specific entities:

```javascript
linkedEntityGuids: ["YOUR_ENTITY_GUID_HERE"]
```

## Support

For issues or questions:
1. Check collector logs for errors
2. Verify metrics in New Relic Query Builder
3. Review this documentation
4. Check OpenTelemetry receiver documentation