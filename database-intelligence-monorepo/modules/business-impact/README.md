# Business Impact Module

Configuration-based business impact scoring for database operations.

## ⚠️ Health Check Policy

**IMPORTANT**: Health check endpoints (port 13133) have been intentionally removed from production code.

- **For validation**: Use `shared/validation/health-check-all.sh`
- **Documentation**: See `shared/validation/README-health-check.md`
- **Do NOT**: Add health check endpoints back to production configs
- **Do NOT**: Expose port 13133 in Docker configurations

## Overview

The business impact module enriches database metrics with business context by mapping tables and operations to business categories and impact scores. Unlike regex-based approaches, it uses a maintainable configuration file to define business rules.

## Features

### Implemented
- **Configuration-based Mapping**: Uses `business-mappings.yaml` to define table-to-business relationships
- **Table Extraction**: Extracts table names from SQL statements when not provided as attributes
- **Operation Multipliers**: Different SQL operations (DELETE, UPDATE, INSERT, SELECT) have different impact multipliers
- **SLA Impact**: Adds additional scoring based on query duration
- **Business Categorization**: Maps operations to revenue, customer, operations, analytics, or admin categories
- **Critical Routing**: Routes high-impact metrics to priority exporters
- **Input Validation**: Validates presence of required attributes before processing

### Architecture

```
[OTLP Data] ─┐
             ├─> [Business Impact] ─> [Routing] ─> [New Relic Standard]
[SQL Intel] ─┘                               └─> [New Relic Critical]
                                             └─> [Alert File]
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SQL_INTELLIGENCE_ENDPOINT` | SQL intelligence metrics endpoint | `sql-intelligence:8082` |
| `NEW_RELIC_LICENSE_KEY` | New Relic license key | Required |
| `NEW_RELIC_OTLP_ENDPOINT` | New Relic OTLP endpoint | Required |
| `ENVIRONMENT` | Deployment environment | `production` |

### Business Mappings Configuration

The `business-mappings.yaml` file defines:

1. **Table Mappings**: Maps table names to business categories and scores
   ```yaml
   orders:
     category: revenue
     impact_score: 10.0
     revenue_impact: direct
   ```

2. **Operation Multipliers**: Adjusts scores based on SQL operation type
   ```yaml
   DELETE:
     revenue: 2.0  # DELETE on revenue tables is very critical
   ```

3. **SLA Thresholds**: Additional scoring for slow queries
   ```yaml
   critical:
     duration_seconds: 5.0
     additional_score: 2.0
   ```

## Metrics

### Input Requirements
The module requires metrics with at least one of:
- `db.sql.table`: The table name being accessed
- `db.statement`: The full SQL statement (table will be extracted)

### Output Metrics
- `business_impact_score`: Calculated impact score (0-15)

### Output Attributes
- `business_category`: revenue/customer/operations/analytics/admin/general
- `business_impact_score`: Numeric score indicating business importance
- `revenue_impact`: direct/indirect/none
- `business_criticality`: CRITICAL/HIGH/MEDIUM/LOW
- `sla_impact`: critical/warning/normal

## Usage

### Basic Deployment

```bash
docker-compose up -d
```

### Testing with Sample Data

```bash
# Send a test metric via OTLP
curl -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{
    "resourceMetrics": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "test-app"}
        }]
      },
      "scopeMetrics": [{
        "metrics": [{
          "name": "mysql_query_duration_milliseconds",
          "gauge": {
            "dataPoints": [{
              "timeUnixNano": "'$(date +%s000000000)'",
              "asDouble": 3500,
              "attributes": [
                {"key": "db.sql.table", "value": {"stringValue": "orders"}},
                {"key": "db.operation", "value": {"stringValue": "UPDATE"}}
              ]
            }]
          }
        }]
      }]
    }]
  }'
```

## Pipeline Design

The module uses a single processing pipeline with routing at the end:

1. **Input Validation**: Filters out metrics without required attributes
2. **Table Extraction**: Extracts table names from SQL statements if needed
3. **Business Scoring**: Applies configuration-based scoring rules
4. **Routing**: Sends critical metrics to priority exporters

This design processes each metric only once, avoiding the inefficiency of duplicate pipelines.

## Extending the Configuration

To add new business rules:

1. Edit `config/business-mappings.yaml`
2. Add new table mappings with appropriate categories and scores
3. Adjust operation multipliers if needed
4. Restart the module

Example:
```yaml
# Add a new critical table
payment_methods:
  category: revenue
  impact_score: 9.5
  revenue_impact: direct
  description: "Customer payment methods"
```

## Troubleshooting

### No Business Impact Scores
- Check if input metrics have `db.sql.table` or `db.statement` attributes
- Verify sql-intelligence module is running and sending metrics
- Check logs for validation errors

### Incorrect Scores
- Review `business-mappings.yaml` for correct table mappings
- Check if table names match exactly (case-sensitive)
- Verify operation multipliers are appropriate

### Missing Critical Alerts
- Ensure scores exceed criticality thresholds (10.0 for CRITICAL)
- Check routing configuration
- Verify critical exporters are configured correctly