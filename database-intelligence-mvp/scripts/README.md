# Database Intelligence Dashboard Scripts

This directory contains scripts for creating and managing the Database Intelligence Dashboard in New Relic.

## Scripts Overview

### 1. `deploy-database-dashboard.sh`
Main deployment script that orchestrates the entire dashboard creation process.
- Verifies environment configuration
- Checks metrics collection
- Creates the dashboard
- Provides deployment summary

**Usage:**
```bash
./deploy-database-dashboard.sh
```

### 2. `verify-collected-metrics.js`
Verification script that checks what metrics are actually being collected in New Relic.
- Queries for PostgreSQL metrics
- Queries for MySQL metrics
- Checks for query log data
- Provides recommendations based on findings

**Usage:**
```bash
node verify-collected-metrics.js
```

### 3. `create-database-dashboard.js`
Dashboard creation script that uses NerdGraph API to create the dashboard.
- Creates a multi-page dashboard
- Includes widgets for both PostgreSQL and MySQL
- Adapts MySQL OHI dashboard structure for OTEL metrics
- Saves dashboard info to `dashboard-info.json`

**Usage:**
```bash
node create-database-dashboard.js
```

## Prerequisites

1. **Environment Variables** (in `.env` file):
   ```
   NEW_RELIC_USER_KEY=your_user_api_key
   NEW_RELIC_ACCOUNT_ID=your_account_id
   ```

2. **Node.js**: Required for running JavaScript scripts

3. **Active Data Collection**: OTEL collector should be running and sending data to New Relic

## Dashboard Features

The created dashboard includes:

### Page 1: Bird's-Eye View
- Database count and overview
- Query execution time trends
- Active connections monitoring
- Disk usage by database
- Slowest queries table
- Transaction rates (commits/rollbacks)
- Buffer pool usage
- Operations summary

### Page 2: Query Performance
- Detailed query log analysis
- Query execution trends over time
- Average duration by database
- Query plan information (when available)

### Page 3: Database Resources
- I/O operations monitoring
- Handler operations (MySQL)
- Background writer stats (PostgreSQL)
- Temporary resource usage
- Connection and thread details

## Metric Sources

The dashboard uses metrics from:

1. **PostgreSQL Receiver** (`postgresql.*` metrics)
   - Connections, commits, rollbacks
   - Disk usage, block I/O
   - Background writer statistics

2. **MySQL Receiver** (`mysql.*` metrics)
   - Threads, uptime
   - Buffer pool metrics
   - Handler operations
   - Temporary resources

3. **SQL Query Receiver** (Log data)
   - Query text and IDs
   - Execution times
   - Execution counts
   - Plan metadata

## Customization

To customize the dashboard, edit `create-database-dashboard.js`:

1. **Modify Queries**: Update NRQL queries in widget configurations
2. **Add Widgets**: Add new widget definitions to pages
3. **Change Layout**: Adjust column, row, width, and height values
4. **Add Variables**: Include dashboard variables for filtering

## Troubleshooting

### No Data in Dashboard
1. Run `verify-collected-metrics.js` to check data availability
2. Ensure OTEL collector is running
3. Verify database connections are working
4. Check New Relic for any ingestion errors

### API Errors
- **401**: Invalid API key
- **403**: Insufficient permissions
- **Account mismatch**: Wrong account ID

### Missing Specific Metrics
- PostgreSQL: Enable `pg_stat_statements`
- MySQL: Enable Performance Schema
- Both: Check user permissions

## Output

After successful dashboard creation:
- Console output with dashboard GUID and URL
- `dashboard-info.json` file containing:
  ```json
  {
    "guid": "dashboard-guid",
    "name": "Database Intelligence - OTEL Metrics",
    "accountId": "account-id",
    "createdAt": "timestamp",
    "url": "https://one.newrelic.com/dashboards/dashboard-guid"
  }
  ```

## Support

For detailed documentation, see:
- [Dashboard Guide](../docs/DASHBOARD_GUIDE.md)
- [OpenTelemetry PostgreSQL Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/postgresqlreceiver)
- [OpenTelemetry MySQL Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/mysqlreceiver)