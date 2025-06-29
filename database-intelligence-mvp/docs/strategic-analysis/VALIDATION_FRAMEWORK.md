# Database Intelligence MVP: Validation Framework

## Overview

This framework ensures comprehensive validation throughout the OHI to OTEL migration, with specific focus on New Relic integration requirements and database metric accuracy.

## Validation Layers

### Layer 1: Build & Unit Testing ✅

**Objective**: Ensure code quality and basic functionality

```bash
# Validation Script: validate-build.sh
#!/bin/bash
set -e

echo "=== Build Validation ==="

# 1. Module consistency check
echo "Checking module paths..."
MODULE_PATH=$(grep "^module" go.mod | awk '{print $2}')
MISMATCHES=$(grep -r "github.com/newrelic/database-intelligence-mvp\|github.com/database-intelligence/" --include="*.yaml" --include="*.go" | grep -v "$MODULE_PATH" || true)
if [ -n "$MISMATCHES" ]; then
    echo "❌ Module path inconsistencies found:"
    echo "$MISMATCHES"
    exit 1
fi

# 2. Build validation
echo "Building collector..."
make build
if [ ! -f "dist/database-intelligence-collector" ]; then
    echo "❌ Build failed - no binary produced"
    exit 1
fi

# 3. Unit test validation
echo "Running unit tests..."
make test
TEST_COVERAGE=$(go test -cover ./... | grep -o '[0-9\.]*%' | tail -1)
echo "Test coverage: $TEST_COVERAGE"

# 4. Linting validation
echo "Running linters..."
make lint

echo "✅ Build validation passed!"
```

### Layer 2: Configuration Validation 🔧

**Objective**: Ensure configurations are valid and follow best practices

```yaml
# validation/config-validator.yaml
validation_rules:
  receivers:
    postgresql:
      required_fields:
        - endpoint
        - username
        - password
      recommended_settings:
        collection_interval: ">=60s"
        tls.insecure: false  # For production
    
    sqlquery:
      max_query_timeout: "30s"
      required_for_ohi_parity:
        - pg_stat_statements queries
        - performance tracking
  
  processors:
    memory_limiter:
      required: true
      position: "first"
    
    batch:
      recommended_settings:
        send_batch_size: "<=10000"
        timeout: ">=5s"
    
    resource:
      required_attributes:
        - service.name
        - deployment.environment
  
  exporters:
    otlp:
      new_relic_requirements:
        endpoint: "https://otlp.nr-data.net:4318"
        compression: "gzip"
        retry_on_failure.enabled: true
```

### Layer 3: Data Collection Validation 📊

**Objective**: Verify metrics are collected correctly

```sql
-- validation/postgresql-validation.sql
-- Run these queries to validate metric collection

-- 1. Verify pg_stat_statements is working
SELECT count(*) as statement_count,
       sum(calls) as total_calls,
       avg(mean_exec_time) as avg_exec_time
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat_statements%';

-- Expected: statement_count > 0

-- 2. Verify database activity
SELECT datname,
       numbackends,
       xact_commit,
       xact_rollback,
       blks_read,
       blks_hit
FROM pg_stat_database
WHERE datname NOT IN ('template0', 'template1', 'postgres')
ORDER BY numbackends DESC;

-- Expected: At least one database with activity

-- 3. Verify table statistics
SELECT schemaname,
       tablename,
       n_tup_ins + n_tup_upd + n_tup_del as total_changes
FROM pg_stat_user_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY total_changes DESC
LIMIT 10;

-- Expected: Some tables with changes
```

### Layer 4: New Relic Integration Validation 🔌

**Objective**: Ensure data arrives correctly in New Relic

```javascript
// validation/newrelic-validator.js
const https = require('https');

class NewRelicValidator {
  constructor(apiKey, accountId) {
    this.apiKey = apiKey;
    this.accountId = accountId;
    this.graphqlEndpoint = 'https://api.newrelic.com/graphql';
  }

  async validateMetricArrival() {
    const query = `
      {
        actor {
          account(id: ${this.accountId}) {
            nrql(query: "SELECT count(*) FROM Metric WHERE service.name = 'database-intelligence' SINCE 10 minutes ago") {
              results
            }
          }
        }
      }
    `;

    const result = await this.executeQuery(query);
    const count = result.data.actor.account.nrql.results[0].count;
    
    return {
      success: count > 0,
      metricCount: count,
      message: count > 0 ? '✅ Metrics arriving' : '❌ No metrics found'
    };
  }

  async validateEntitySynthesis() {
    const query = `
      {
        actor {
          entitySearch(query: "type = 'SERVICE' AND name LIKE 'database-intelligence%'") {
            results {
              entities {
                guid
                name
                type
                tags {
                  key
                  values
                }
              }
            }
          }
        }
      }
    `;

    const result = await this.executeQuery(query);
    const entities = result.data.actor.entitySearch.results.entities;
    
    return {
      success: entities.length > 0,
      entityCount: entities.length,
      entities: entities
    };
  }

  async validateNrIntegrationErrors() {
    const query = `
      {
        actor {
          account(id: ${this.accountId}) {
            nrql(query: "SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' SINCE 1 hour ago") {
              results
            }
          }
        }
      }
    `;

    const result = await this.executeQuery(query);
    const errorCount = result.data.actor.account.nrql.results[0].count;
    
    return {
      success: errorCount === 0,
      errorCount: errorCount,
      message: errorCount === 0 ? '✅ No integration errors' : `❌ ${errorCount} integration errors found`
    };
  }
}
```

### Layer 5: OHI Parity Validation 🔄

**Objective**: Ensure OTEL metrics match OHI metrics

```python
# validation/ohi-parity-validator.py
import pandas as pd
import numpy as np
from datetime import datetime, timedelta

class OHIParityValidator:
    def __init__(self, nrql_client):
        self.nrql = nrql_client
        self.tolerance = 0.05  # 5% tolerance
        
    def validate_metric_parity(self, ohi_metric, otel_metric, time_range='1 hour'):
        """Compare OHI and OTEL metrics for parity"""
        
        # Query OHI metric
        ohi_query = f"""
        SELECT average({ohi_metric}) as value 
        FROM PostgresqlDatabaseSample 
        SINCE {time_range} ago 
        TIMESERIES 1 minute
        """
        ohi_data = self.nrql.query(ohi_query)
        
        # Query OTEL metric
        otel_query = f"""
        SELECT average({otel_metric}) as value 
        FROM Metric 
        WHERE service.name = 'database-intelligence' 
        SINCE {time_range} ago 
        TIMESERIES 1 minute
        """
        otel_data = self.nrql.query(otel_query)
        
        # Compare values
        comparison = self._compare_timeseries(ohi_data, otel_data)
        
        return {
            'metric': ohi_metric,
            'parity_achieved': comparison['within_tolerance'],
            'mean_difference': comparison['mean_diff'],
            'max_difference': comparison['max_diff'],
            'correlation': comparison['correlation']
        }
    
    def _compare_timeseries(self, series1, series2):
        """Compare two time series for similarity"""
        df1 = pd.DataFrame(series1)
        df2 = pd.DataFrame(series2)
        
        # Align timestamps
        merged = pd.merge(df1, df2, on='timestamp', suffixes=('_ohi', '_otel'))
        
        # Calculate differences
        merged['diff'] = abs(merged['value_ohi'] - merged['value_otel'])
        merged['pct_diff'] = merged['diff'] / merged['value_ohi']
        
        return {
            'within_tolerance': (merged['pct_diff'] <= self.tolerance).all(),
            'mean_diff': merged['pct_diff'].mean(),
            'max_diff': merged['pct_diff'].max(),
            'correlation': merged['value_ohi'].corr(merged['value_otel'])
        }
```

## Validation Test Suites

### 1. Smoke Test Suite (5 minutes)
```bash
# Quick validation after any change
make validate-smoke

# Checks:
# - Build success
# - Config validity  
# - Basic metric collection
# - Health endpoint
```

### 2. Integration Test Suite (30 minutes)
```bash
# Full integration validation
make validate-integration

# Checks:
# - All receivers working
# - All processors functioning
# - New Relic data arrival
# - Entity synthesis
# - No integration errors
```

### 3. Parity Test Suite (2 hours)
```bash
# Complete OHI parity validation
make validate-parity

# Checks:
# - All OHI metrics mapped
# - Values within tolerance
# - Query performance metrics
# - Dashboard compatibility
```

## Validation Dashboard

Create a New Relic dashboard for continuous validation:

```sql
-- Dashboard queries

-- 1. Metric Collection Rate
SELECT rate(count(*), 1 minute) as 'Metrics/min'
FROM Metric
WHERE service.name = 'database-intelligence'
TIMESERIES

-- 2. Integration Errors
SELECT count(*) as 'Errors'
FROM NrIntegrationError
WHERE newRelicFeature = 'Metrics'
TIMESERIES

-- 3. Collector Health
SELECT latest(up) as 'Collector Status'
FROM Metric
WHERE metricName = 'otelcol_process_uptime'
AND service.name = 'database-intelligence'

-- 4. Database Coverage
SELECT uniqueCount(database_name) as 'Databases Monitored'
FROM Metric
WHERE service.name = 'database-intelligence'
AND database_name IS NOT NULL

-- 5. Query Performance Tracking
SELECT average(db.query.exec_time.mean) as 'Avg Query Time (ms)'
FROM Metric
WHERE service.name = 'database-intelligence'
FACET database_name
TIMESERIES
```

## Validation Runbook

### Daily Validation Checklist
- [ ] Collector health check passing
- [ ] No NrIntegrationError events
- [ ] Metric collection rate stable
- [ ] Memory usage within limits
- [ ] No circuit breaker trips

### Weekly Validation
- [ ] Run parity test suite
- [ ] Review cardinality metrics
- [ ] Check for missing databases
- [ ] Validate alerting rules
- [ ] Performance benchmarking

### Pre-Production Validation
- [ ] Full parity validation passed
- [ ] Load testing completed
- [ ] Security scan passed
- [ ] Documentation updated
- [ ] Rollback tested

## Success Criteria

### Phase 1 (Foundation)
- Build success rate: 100%
- Unit test coverage: >80%
- Config validation: Pass

### Phase 2 (Integration)
- New Relic data arrival: 100%
- Entity synthesis: Working
- Integration errors: 0

### Phase 3 (Parity)
- OHI metric coverage: >95%
- Value accuracy: ±5%
- Query metrics: Working

### Phase 4 (Production)
- Uptime: 99.9%
- Latency p99: <100ms
- Memory usage: <500MB

---

**Version**: 1.0  
**Last Updated**: 2025-06-30  
**Owner**: Database Intelligence Team