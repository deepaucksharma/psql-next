# New Relic Dashboard Validation with NerdGraph

This guide shows how to use the enhanced setup script to validate dashboards and data flow using New Relic's NerdGraph API directly.

## Quick Start

```bash
# Set up your New Relic credentials
export NEW_RELIC_API_KEY="your-user-api-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id" 
export NEW_RELIC_LICENSE_KEY="your-license-key"

# Optional: Set region (default is US)
export NEW_RELIC_REGION="US"  # or "EU"

# Validate existing dashboards and data flow
./shared/newrelic/scripts/setup-newrelic.sh validate

# Deploy all dashboards from JSON files
./shared/newrelic/scripts/setup-newrelic.sh deploy

# Full setup (dashboards, alerts, workloads)
./shared/newrelic/scripts/setup-newrelic.sh setup
```

## Available Commands

### 1. Validate Mode
```bash
./setup-newrelic.sh validate
```

**What it does:**
- ✅ **Dashboard Existence Check**: Queries all dashboards in your account via NerdGraph
- ✅ **Widget Data Validation**: Tests every NRQL query in each dashboard widget
- ✅ **Metrics Flow Verification**: Checks that critical metrics are flowing to NRDB
- ✅ **Entity Synthesis Validation**: Verifies that OpenTelemetry entities are properly synthesized

**Example Output:**
```
🔍 Validating New Relic MySQL Intelligence Setup
================================================
🔍 Validating dashboards...
✅ Found 12 dashboards in account
  ✅ Found: MySQL Intelligence Dashboard (GUID: MjU2MDUzN...)
    📊 Validating data for: MySQL Intelligence Dashboard
    ✅ All 24 widgets have data
  ⚠️  Missing: Plan Explorer Dashboard
  ✅ Found: Database Intelligence Executive Dashboard (GUID: MjU2MDUzO...)

📈 Validating metrics data flow...
    Checking: mysql.intelligence.comprehensive
    ✅ mysql.intelligence.comprehensive: Data found
    Checking: mysql.connections.current
    ✅ mysql.connections.current: Data found

🏗️  Validating entity synthesis...
    Checking: MYSQL_INSTANCE entities
    ✅ MYSQL_INSTANCE: 3 entities (3 reporting)
    Checking: HOST entities
    ✅ HOST: 5 entities (5 reporting)
```

### 2. Deploy Mode
```bash
./setup-newrelic.sh deploy
```

**What it does:**
- 📊 **Dashboard Deployment**: Creates dashboards from JSON files in the dashboards directory
- 🔄 **Environment Variable Replacement**: Automatically replaces `${NEW_RELIC_ACCOUNT_ID}` in dashboard JSON
- ✅ **Error Handling**: Reports deployment success/failure with detailed error messages

**Dashboard Files Deployed:**
- `mysql-intelligence-dashboard.json` - Main MySQL monitoring dashboard
- `plan-explorer-dashboard.json` - SolarWinds Plan Explorer equivalent
- `database-intelligence-executive-dashboard.json` - Executive summary dashboard

### 3. Setup Mode
```bash
./setup-newrelic.sh setup
```

**Complete New Relic setup including:**
- 📊 Dashboard creation
- 🚨 Alert policy and conditions setup
- 🔧 Workload creation for MySQL entities
- 🔍 Synthetic monitor setup
- 🤖 Applied Intelligence workflow configuration

## NerdGraph Queries Used

### Dashboard Validation Query
```graphql
query getDashboards($accountId: Int!) {
    actor {
        account(id: $accountId) {
            dashboards {
                name
                guid
                createdAt
                updatedAt
                pages {
                    name
                    guid
                    widgets {
                        title
                        configuration
                    }
                }
            }
        }
    }
}
```

### NRQL Testing Query
```graphql
query testNRQL($accountId: Int!, $nrql: Nrql!) {
    actor {
        account(id: $accountId) {
            nrql(query: $nrql) {
                results
                metadata {
                    timeWindow {
                        begin
                        end
                    }
                }
            }
        }
    }
}
```

### Entity Search Query
```graphql
query validateEntities($accountId: Int!, $entityType: String!) {
    actor {
        entitySearch(query: $entityType) {
            results {
                entities {
                    guid
                    name
                    entityType
                    reporting
                }
            }
        }
    }
}
```

### Dashboard Creation Mutation
```graphql
mutation CreateDashboard($accountId: Int!, $dashboard: DashboardInput!) {
    dashboardCreate(accountId: $accountId, dashboard: $dashboard) {
        entityResult {
            guid
            name
            permalink
        }
        errors {
            description
            type
        }
    }
}
```

## Metrics Validated

The validation checks these critical metrics:
- `mysql.intelligence.comprehensive` - Core SQL intelligence scoring
- `mysql.connections.current` - Current MySQL connections
- `mysql.threads.running` - Active MySQL threads
- `mysql.query.exec_count` - Query execution counts
- `system.cpu.utilization` - Host CPU utilization
- `system.memory.usage` - Host memory usage

## Entity Types Validated

The validation checks these entity types:
- `MYSQL_INSTANCE` - MySQL database instances
- `HOST` - Host/server entities
- `SYNTHETIC_MONITOR` - Synthetic monitoring entities
- `APPLICATION` - Application entities

## Troubleshooting

### Common Issues

**1. Authentication Error**
```
❌ NerdGraph Error: {"message": "Unauthorized"}
```
- Verify your `NEW_RELIC_API_KEY` is correct
- Ensure you're using a User API key, not a License key

**2. No Dashboards Found**
```
⚠️  Missing: MySQL Intelligence Dashboard
```
- Run `./setup-newrelic.sh deploy` to create dashboards from JSON files
- Check that dashboard JSON files exist in the dashboards directory

**3. No Data in Widgets**
```
❌ No widgets have data
```
- Verify OpenTelemetry collectors are running and sending data
- Check that `instrumentation.provider = 'opentelemetry'` tag is set
- Ensure metrics are reaching New Relic (check in Data Explorer)

**4. No Entities Found**
```
⚠️  MYSQL_INSTANCE: No entities found
```
- Verify entity synthesis is configured in your OpenTelemetry collectors
- Check that entity.type, entity.guid, and entity.name attributes are set
- Allow 5-10 minutes for entity synthesis to take effect

### Manual NerdGraph Testing

You can test NerdGraph queries manually using curl:

```bash
# Test a simple query
curl -X POST https://api.newrelic.com/graphql \
  -H "Content-Type: application/json" \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{
    "query": "query { actor { user { name email } } }"
  }'

# Test NRQL query
curl -X POST https://api.newrelic.com/graphql \
  -H "Content-Type: application/json" \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{
    "query": "query($accountId: Int!, $nrql: Nrql!) { actor { account(id: $accountId) { nrql(query: $nrql) { results } } } }",
    "variables": {
      "accountId": '${NEW_RELIC_ACCOUNT_ID}',
      "nrql": "SELECT count(*) FROM Metric WHERE instrumentation.provider = '\''opentelemetry'\'' SINCE 1 hour ago"
    }
  }'
```

## Integration with CI/CD

You can use the validation script in your CI/CD pipeline:

```yaml
# GitHub Actions example
- name: Validate New Relic Dashboards
  env:
    NEW_RELIC_API_KEY: ${{ secrets.NEW_RELIC_API_KEY }}
    NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
    NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
  run: |
    ./shared/newrelic/scripts/setup-newrelic.sh validate
```

## Best Practices

1. **Regular Validation**: Run validation after deploying new collectors or updating configurations
2. **Environment-Specific**: Use different New Relic accounts for dev/staging/production
3. **Monitoring**: Set up alerts on dashboard widget failures
4. **Documentation**: Keep dashboard JSON files in version control
5. **Testing**: Validate dashboards work with sample data before production deployment

## Advanced Usage

### Custom Metric Validation
You can extend the script to validate custom metrics by modifying the `critical_metrics` array in the `validate_metrics_flow()` function.

### Custom Entity Types
Add custom entity types to the `entity_types` array in the `validate_entities()` function.

### Dashboard Templates
Create environment-specific dashboard templates by using environment variables in the JSON files that get replaced during deployment.