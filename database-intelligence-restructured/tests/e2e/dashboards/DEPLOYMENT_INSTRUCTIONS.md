# Dashboard Deployment Instructions

## Prerequisites

1. **New Relic Account**: You need an active New Relic account
2. **Account ID**: Your New Relic account ID (found in account settings)
3. **API Key**: A New Relic User API key with dashboard management permissions

### Getting Your Account ID
1. Log into New Relic
2. Click on your name in the bottom left
3. Go to "Administration"
4. Your account ID is shown at the top

### Creating an API Key
1. Go to: https://one.newrelic.com/api-keys
2. Click "Create a key"
3. Key type: "User"
4. Name: "Database Intelligence Dashboard"
5. Permissions: Enable "NerdGraph" access
6. Click "Create"
7. Copy the key immediately (it won't be shown again)

## Deployment Options

### Option 1: Interactive Deployment (Recommended)

Run the interactive deployment script:

```bash
cd /Users/deepaksharma/syc/db-otel/database-intelligence-restructured/tests/e2e/dashboards
./deploy-dashboard-interactive.sh
```

This script will:
- Prompt you for your account ID and API key
- Validate the inputs
- Verify all dashboard queries
- Deploy the dashboard
- Provide the dashboard URL
- Optionally save credentials for future use

### Option 2: Direct Deployment

If you have your credentials ready:

```bash
cd /Users/deepaksharma/syc/db-otel/database-intelligence-restructured/tests/e2e/dashboards
./verify-and-deploy-dashboard.sh YOUR_ACCOUNT_ID YOUR_API_KEY
```

Replace:
- `YOUR_ACCOUNT_ID` with your actual account ID
- `YOUR_API_KEY` with your actual API key

### Option 3: Manual Deployment via New Relic UI

1. Log into New Relic
2. Go to Dashboards
3. Click "Import dashboard"
4. Copy the contents of `database-intelligence-complete-dashboard.json`
5. Paste and click "Import"

## What Happens During Deployment

1. **Query Validation**: All 34 NRQL queries are validated
   - Some may fail if metrics aren't being collected yet
   - This is normal - the dashboard can still be created

2. **Dashboard Creation**: The dashboard is created with:
   - 6 pages of monitoring
   - 34 widgets covering all aspects
   - Variables for filtering

3. **Success Output**: You'll receive:
   - Dashboard GUID
   - Direct URL to access the dashboard

## After Deployment

### 1. Configure Data Collection

Update your collector configuration to send data to New Relic:

```yaml
exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: YOUR_API_KEY
    compression: gzip

service:
  pipelines:
    metrics:
      receivers: [otlp, ash, kernelmetrics]
      processors: [memory_limiter, batch, costcontrol, nrerrormonitor, querycorrelator]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch, costcontrol, adaptivesampler, circuit_breaker, planattributeextractor, verification]
      exporters: [otlp]
```

### 2. Start the Collector

```bash
# From the production directory
cd /Users/deepaksharma/syc/db-otel/database-intelligence-restructured/distributions/production

# Set environment variables
export NEW_RELIC_API_KEY="your-api-key"
export DB_CONNECTION_STRING="postgres://user:pass@localhost/dbname"

# Run with test configuration
./otelcol-complete --config=../../tests/e2e/dashboards/test-collector-config.yaml
```

### 3. Generate Test Data (Optional)

To quickly see data in the dashboard:

```bash
# Send test metrics via OTLP
curl -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/json" \
  -d @test-metrics.json
```

### 4. View Your Dashboard

1. Go to New Relic One
2. Navigate to Dashboards
3. Find "Database Intelligence - Complete Monitoring"
4. Data should appear within 2-3 minutes

## Troubleshooting

### No Data Showing
- Verify collector is running: `ps aux | grep otelcol`
- Check collector logs for errors
- Ensure API key is correct
- Wait 2-3 minutes for data to appear

### Query Errors in Dashboard
- Expected if components aren't sending data yet
- Queries will work once data flows
- Check specific component configuration

### Permission Errors
- Ensure API key has dashboard management permissions
- Verify account ID is correct
- Check if you have dashboard creation limits

## Support

For issues:
1. Check collector logs
2. Verify network connectivity to New Relic
3. Ensure all environment variables are set
4. Review the DASHBOARD_GUIDE.md for query details