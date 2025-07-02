# pg_querylens Integration for Database Intelligence Collector

## Overview

pg_querylens is a PostgreSQL extension that provides advanced query performance insights by:
- Capturing query execution plans automatically
- Tracking plan changes over time
- Identifying query performance regressions
- Providing detailed query statistics beyond pg_stat_statements

## Integration Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    PostgreSQL Database                   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────┐    ┌─────────────────┐          │
│  │ pg_stat_statements│    │   pg_querylens   │          │
│  └────────┬─────────┘    └────────┬─────────┘          │
│           │                        │                     │
│           └────────────┬───────────┘                    │
│                        │                                 │
└────────────────────────┼─────────────────────────────────┘
                         │
                         ▼
                ┌─────────────────┐
                │  SQLQUERY       │
                │  Receiver       │
                └────────┬────────┘
                         │
                         ▼
                ┌─────────────────┐
                │ Plan Attribute  │
                │ Extractor       │
                └────────┬────────┘
                         │
                         ▼
                ┌─────────────────┐
                │ Plan Intelligence│
                │ Processor       │
                └────────┬────────┘
                         │
                         ▼
                ┌─────────────────┐
                │   New Relic     │
                │   (NRDB)        │
                └─────────────────┘
```

## pg_querylens Features

### 1. Automatic Plan Capture
- Captures execution plans for queries exceeding threshold
- Stores plan history with timestamps
- Tracks plan changes and performance impact

### 2. Query Performance Tracking
- Detailed timing for each plan node
- I/O statistics per operation
- Memory usage tracking
- Parallel execution metrics

### 3. Regression Detection
- Identifies when query plans change
- Compares performance before/after changes
- Alerts on performance degradation

## Implementation Plan

### Phase 1: Basic Integration

#### 1. SQL Query Configuration
```yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "host=localhost port=5432 user=monitoring dbname=postgres"
    queries:
      - sql: |
          SELECT 
            queryid,
            query_text,
            plan_id,
            plan_text,
            plan_timestamp,
            total_time,
            mean_time,
            calls,
            rows,
            shared_blks_hit,
            shared_blks_read,
            temp_blks_written,
            planning_time,
            execution_time
          FROM pg_querylens_queries
          WHERE plan_timestamp > NOW() - INTERVAL '5 minutes'
        metrics:
          - metric_name: db.querylens.query.execution_time
            value_column: execution_time
            value_type: double
            data_point_type: gauge
          - metric_name: db.querylens.query.planning_time
            value_column: planning_time
            value_type: double
            data_point_type: gauge
        resource_attributes:
          - attribute_name: db.querylens.queryid
            value_column: queryid
          - attribute_name: db.querylens.plan_id
            value_column: plan_id
```

#### 2. Plan Intelligence Processor Enhancement
```go
// processors/planattributeextractor/querylens_integration.go
package planattributeextractor

import (
    "context"
    "encoding/json"
    "fmt"
    "regexp"
    
    "go.opentelemetry.io/collector/pdata/pmetric"
)

type QueryLensData struct {
    QueryID      int64  `json:"queryid"`
    PlanID       string `json:"plan_id"`
    PlanText     string `json:"plan_text"`
    PlanChanges  []PlanChange `json:"plan_changes"`
}

type PlanChange struct {
    Timestamp   string  `json:"timestamp"`
    OldPlanID   string  `json:"old_plan_id"`
    NewPlanID   string  `json:"new_plan_id"`
    CostChange  float64 `json:"cost_change"`
    Performance string  `json:"performance_impact"`
}

func (p *planAttributeExtractor) processQueryLensData(ctx context.Context, md pmetric.Metrics) error {
    resourceMetrics := md.ResourceMetrics()
    for i := 0; i < resourceMetrics.Len(); i++ {
        rm := resourceMetrics.At(i)
        resource := rm.Resource()
        
        // Check if this is querylens data
        if queryID, ok := resource.Attributes().Get("db.querylens.queryid"); ok {
            planText, _ := resource.Attributes().Get("db.querylens.plan_text")
            
            // Extract plan insights
            planInsights := p.analyzeQueryLensPlan(planText.AsString())
            
            // Add insights as attributes
            resource.Attributes().PutStr("db.plan.type", planInsights.PlanType)
            resource.Attributes().PutDouble("db.plan.estimated_cost", planInsights.EstimatedCost)
            resource.Attributes().PutBool("db.plan.has_regression", planInsights.HasRegression)
            
            // Detect plan changes
            if planID, ok := resource.Attributes().Get("db.querylens.plan_id"); ok {
                if p.detectPlanChange(queryID.AsRaw(), planID.AsString()) {
                    resource.Attributes().PutBool("db.plan.changed", true)
                    resource.Attributes().PutStr("db.plan.change_severity", "high")
                }
            }
        }
    }
    return nil
}

func (p *planAttributeExtractor) analyzeQueryLensPlan(planText string) PlanInsights {
    insights := PlanInsights{}
    
    // Extract plan type (Seq Scan, Index Scan, etc.)
    planTypeRegex := regexp.MustCompile(`(Seq Scan|Index Scan|Index Only Scan|Bitmap Heap Scan|Hash Join|Nested Loop|Merge Join)`)
    if matches := planTypeRegex.FindStringSubmatch(planText); len(matches) > 0 {
        insights.PlanType = matches[1]
    }
    
    // Extract estimated cost
    costRegex := regexp.MustCompile(`cost=(\d+\.?\d*)\.\.(\d+\.?\d*)`)
    if matches := costRegex.FindStringSubmatch(planText); len(matches) > 2 {
        insights.EstimatedCost = parseFloat(matches[2])
    }
    
    // Detect potential regressions
    insights.HasRegression = p.detectRegression(planText)
    
    return insights
}
```

### Phase 2: Advanced Features

#### 1. Plan History Tracking
```yaml
queries:
  - sql: |
      SELECT 
        q.queryid,
        q.query_text,
        ph.plan_id,
        ph.timestamp as plan_timestamp,
        ph.execution_count,
        ph.mean_time_ms,
        ph.total_time_ms,
        ph.stddev_time_ms,
        LAG(ph.plan_id) OVER (PARTITION BY q.queryid ORDER BY ph.timestamp) as previous_plan_id,
        LAG(ph.mean_time_ms) OVER (PARTITION BY q.queryid ORDER BY ph.timestamp) as previous_mean_time
      FROM pg_querylens_queries q
      JOIN pg_querylens_plan_history ph ON q.queryid = ph.queryid
      WHERE ph.timestamp > NOW() - INTERVAL '1 hour'
    metrics:
      - metric_name: db.querylens.plan.mean_time
        value_column: mean_time_ms
      - metric_name: db.querylens.plan.regression_ratio
        value_expression: "CASE WHEN previous_mean_time > 0 THEN mean_time_ms / previous_mean_time ELSE 1 END"
```

#### 2. Regression Detection Algorithm
```go
func (p *planAttributeExtractor) detectRegression(current, previous PlanMetrics) RegressionAnalysis {
    analysis := RegressionAnalysis{
        Detected: false,
        Severity: "none",
    }
    
    // Check execution time regression
    timeIncrease := (current.MeanTime - previous.MeanTime) / previous.MeanTime
    if timeIncrease > 0.5 { // 50% increase
        analysis.Detected = true
        if timeIncrease > 2.0 { // 200% increase
            analysis.Severity = "critical"
        } else if timeIncrease > 1.0 { // 100% increase
            analysis.Severity = "high"
        } else {
            analysis.Severity = "medium"
        }
    }
    
    // Check resource usage regression
    ioIncrease := float64(current.BlocksRead - previous.BlocksRead) / float64(previous.BlocksRead)
    if ioIncrease > 1.0 { // 100% increase in I/O
        analysis.Detected = true
        analysis.IORegression = true
    }
    
    return analysis
}
```

### Phase 3: Integration with Existing Processors

#### 1. Circuit Breaker Integration
```go
// Automatically trigger circuit breaker for queries with critical regressions
if regression.Severity == "critical" {
    circuitBreaker.OpenCircuit(queryID, "Critical performance regression detected")
}
```

#### 2. Adaptive Sampler Integration
```go
// Increase sampling for queries with plan changes
samplingRules = append(samplingRules, Rule{
    Name: "plan_change_sampling",
    Expression: `attributes["db.plan.changed"] == true`,
    SampleRate: 1.0, // 100% sampling for plan changes
})
```

## Configuration Examples

### Basic Setup
```yaml
receivers:
  sqlquery:
    collection_interval: 30s
    queries:
      - sql: "SELECT * FROM pg_querylens_current_queries"
        metrics:
          - metric_name: db.querylens.active_queries
            value_type: int
            data_point_type: sum

processors:
  planattributeextractor:
    timeout: 100ms
    querylens:
      enabled: true
      plan_history_hours: 24
      regression_threshold: 0.5

exporters:
  otlp:
    endpoint: "otlp.nr-data.net:4317"
```

### Advanced Setup with Alerting
```yaml
processors:
  planattributeextractor:
    querylens:
      enabled: true
      regression_detection:
        enabled: true
        thresholds:
          time_increase: 0.5     # 50% increase
          io_increase: 1.0       # 100% increase
          cost_increase: 2.0     # 200% increase
      alert_on_regression: true
      alert_destinations:
        - type: "newrelic_event"
          event_type: "QueryRegression"
```

## NRQL Queries for pg_querylens Data

### Query Performance Overview
```sql
SELECT 
  average(db.querylens.query.execution_time) as 'Avg Execution Time',
  max(db.querylens.query.execution_time) as 'Max Execution Time',
  uniqueCount(db.querylens.queryid) as 'Unique Queries',
  sum(db.querylens.query.calls) as 'Total Executions'
FROM Metric
WHERE db.system = 'postgresql'
FACET db.querylens.query_text
SINCE 1 hour ago
```

### Plan Change Detection
```sql
SELECT 
  count(*) as 'Plan Changes',
  latest(db.plan.change_severity) as 'Severity'
FROM Metric
WHERE db.plan.changed = true
FACET db.querylens.queryid, db.statement
SINCE 1 hour ago
```

### Regression Analysis
```sql
SELECT 
  histogram(db.querylens.plan.regression_ratio, 10, 20) as 'Performance Change Distribution'
FROM Metric
WHERE db.querylens.plan.regression_ratio > 1
SINCE 1 day ago
```

## Benefits of pg_querylens Integration

1. **Proactive Performance Management**
   - Detect plan regressions before they impact users
   - Track query performance trends over time
   - Identify optimization opportunities

2. **Root Cause Analysis**
   - See exactly when and why query performance changed
   - Compare execution plans before/after changes
   - Correlate with database changes or deployments

3. **Intelligent Monitoring**
   - Automatically adjust monitoring based on query behavior
   - Focus on queries with performance issues
   - Reduce noise from stable queries

4. **Cost Optimization**
   - Identify expensive queries and plans
   - Track resource usage trends
   - Optimize based on actual execution patterns

## Installation and Setup

### 1. Install pg_querylens Extension
```sql
-- Install the extension
CREATE EXTENSION pg_querylens;

-- Configure collection parameters
ALTER SYSTEM SET pg_querylens.max_plans_per_query = 100;
ALTER SYSTEM SET pg_querylens.plan_capture_threshold_ms = 100;
ALTER SYSTEM SET pg_querylens.track_planning = on;

-- Reload configuration
SELECT pg_reload_conf();
```

### 2. Grant Permissions
```sql
-- Grant permissions to monitoring user
GRANT SELECT ON ALL TABLES IN SCHEMA pg_querylens TO monitoring;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA pg_querylens TO monitoring;
```

### 3. Configure Collector
Add the pg_querylens queries to your collector configuration and enable the plan intelligence features in the processors.

## Monitoring Best Practices

1. **Set Appropriate Thresholds**
   - Plan capture threshold based on your SLA
   - Regression detection sensitivity
   - Collection intervals

2. **Regular Maintenance**
   - Periodic cleanup of old plan history
   - Archive historical data for long-term analysis
   - Update regression baselines

3. **Integration with CI/CD**
   - Test query performance in staging
   - Compare plans between environments
   - Gate deployments on regression tests

## Troubleshooting

### Common Issues

1. **No Data Collected**
   - Verify pg_querylens is installed and enabled
   - Check monitoring user permissions
   - Ensure queries exceed capture threshold

2. **High Storage Usage**
   - Adjust retention policies
   - Increase plan capture threshold
   - Implement data archival

3. **False Positive Regressions**
   - Tune regression thresholds
   - Account for data volume changes
   - Consider time-of-day patterns