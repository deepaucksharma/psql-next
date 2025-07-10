# Unified OHI Parity Validation Platform & Test Strategy

## Executive Summary

This document presents a comprehensive validation platform that deeply integrates OHI dashboard parity validation with our New Relic compatibility test strategy. The platform ensures 100% metric parity while providing automated, continuous validation of all PostgreSQL OHI dashboard components.

## Table of Contents

1. [Platform Architecture](#platform-architecture)
2. [OHI Dashboard Analysis](#ohi-dashboard-analysis)
3. [Integrated Validation Framework](#integrated-validation-framework)
4. [Metric Mapping System](#metric-mapping-system)
5. [Automated Test Suites](#automated-test-suites)
6. [Continuous Validation Pipeline](#continuous-validation-pipeline)
7. [Implementation Roadmap](#implementation-roadmap)

## Platform Architecture

### Overview

The validation platform consists of five interconnected components that work together to ensure complete OHI parity:

```
┌─────────────────────────────────────────────────────────────────┐
│                   OHI Parity Validation Platform                  │
├─────────────────────────┬─────────────────────────┬─────────────┤
│   Dashboard Parser      │   Metric Mapper         │   Validator │
│   - NRQL Extraction     │   - OHI → OTEL Maps     │   - Compare │
│   - Event Detection     │   - Transformations     │   - Analyze │
│   - Attribute Catalog   │   - Unit Conversions    │   - Report  │
├─────────────────────────┼─────────────────────────┼─────────────┤
│   Test Generator        │   Continuous Runner     │   Reporter  │
│   - Widget Tests        │   - Scheduled Runs      │   - Parity  │
│   - Query Tests         │   - Drift Detection     │   - Trends  │
│   - Alert Tests         │   - Auto-Remediation    │   - Alerts  │
└─────────────────────────┴─────────────────────────┴─────────────┘
```

### Core Components

#### 1. **OHI Dashboard Parser** (`pkg/validation/dashboard_parser.go`)
```go
type DashboardParser struct {
    dashboardJSON  map[string]interface{}
    nrqlQueries    []NRQLQuery
    ohiEvents      map[string]OHIEvent
    attributes     map[string][]string
}

type NRQLQuery struct {
    Query          string
    EventType      string
    Metrics        []string
    Attributes     []string
    Aggregations   []string
    TimeWindow     string
}

type OHIEvent struct {
    Name           string
    RequiredFields []string
    OptionalFields []string
    OTELMapping    string
}
```

#### 2. **Metric Mapping Registry** (`configs/validation/metric_mappings.yaml`)
```yaml
ohi_to_otel_mappings:
  # PostgreSQL Sample Metrics
  PostgreSQLSample:
    otel_metric_type: "Metric"
    filter: "db.system = 'postgresql'"
    metrics:
      db.commitsPerSecond:
        otel_name: "postgresql.commits"
        transformation: "rate_per_second"
        unit_conversion: null
      db.rollbacksPerSecond:
        otel_name: "postgresql.rollbacks"
        transformation: "rate_per_second"
      db.bufferHitRatio:
        otel_name: "calculated"
        formula: "postgresql.blocks.hit / (postgresql.blocks.hit + postgresql.blocks.read)"
      db.connections.active:
        otel_name: "postgresql.connections.active"
        transformation: "direct"

  # Slow Query Events
  PostgresSlowQueries:
    otel_metric_type: "Metric"
    filter: "db.system = 'postgresql' AND db.query.duration > 500"
    attributes:
      query_id:
        otel_name: "db.querylens.queryid"
      query_text:
        otel_name: "db.statement"
        transformation: "anonymize"
      avg_elapsed_time_ms:
        otel_name: "db.query.execution_time_mean"
      execution_count:
        otel_name: "db.query.calls"
      avg_disk_reads:
        otel_name: "db.query.disk_io.reads_avg"
      avg_disk_writes:
        otel_name: "db.query.disk_io.writes_avg"

  # Wait Events
  PostgresWaitEvents:
    otel_metric_type: "Metric"
    filter: "db.system = 'postgresql' AND wait.event_name IS NOT NULL"
    attributes:
      wait_event_name:
        otel_name: "wait.event_name"
      total_wait_time_ms:
        otel_name: "wait.duration_ms"
        aggregation: "sum"

  # Blocking Sessions
  PostgresBlockingSessions:
    otel_metric_type: "Log"
    filter: "db.system = 'postgresql' AND blocking.detected = true"
    attributes:
      blocked_pid:
        otel_name: "session.blocked.pid"
      blocking_pid:
        otel_name: "session.blocking.pid"
```

## OHI Dashboard Analysis

### Dashboard Structure Analysis

From the provided PostgreSQL dashboard, we've identified:

#### **Page 1: Bird's-Eye View**
1. **Database Query Distribution**
   - NRQL: `SELECT uniqueCount(query_id) from PostgresSlowQueries facet database_name`
   - Validation: Count unique queries per database

2. **Average Execution Time**
   - NRQL: `SELECT latest(avg_elapsed_time_ms) from PostgresSlowQueries facet query_text`
   - Validation: Query performance metrics accuracy

3. **Execution Counts Timeline**
   - NRQL: `SELECT count(execution_count) from PostgresSlowQueries TIMESERIES`
   - Validation: Time series data continuity

4. **Top Wait Events**
   - NRQL: `SELECT latest(total_wait_time_ms) from PostgresWaitEvents facet wait_event_name`
   - Validation: Wait event categorization

5. **Top N Slowest Queries Table**
   - Complex table with multiple attributes
   - Validation: All attributes present and accurate

6. **Disk IO Usage Charts**
   - Average disk reads/writes over time
   - Validation: IO metric accuracy

7. **Blocking Details Table**
   - Comprehensive blocking session information
   - Validation: Blocking detection accuracy

#### **Page 2: Query Details**
- Individual query execution details
- Query execution plan metrics

#### **Page 3: Wait Time Analysis**
- Wait event trends and categorization
- Database-level wait analysis

## Integrated Validation Framework

### Validation Test Structure

```go
// pkg/validation/ohi_parity_validator.go
type OHIParityValidator struct {
    ohiClient      *OHIDataClient
    otelClient     *OTELDataClient
    mappingRegistry *MetricMappingRegistry
    tolerance      float64
}

type ValidationResult struct {
    Timestamp      time.Time
    Widget         DashboardWidget
    OHIData        []DataPoint
    OTELData       []DataPoint
    Accuracy       float64
    MissingMetrics []string
    ExtraMetrics   []string
    Issues         []ValidationIssue
}

type DashboardWidget struct {
    Title          string
    NRQLQuery      string
    VisualizationType string
    RequiredMetrics []string
    RequiredAttributes []string
}
```

### Per-Widget Validation Tests

```go
// tests/e2e/suites/ohi_dashboard_validation_test.go

func (s *OHIDashboardValidationSuite) TestDatabaseQueryDistribution() {
    // Widget: "Database" - uniqueCount of query_id by database
    ohiQuery := "SELECT uniqueCount(query_id) from PostgresSlowQueries facet database_name"
    otelQuery := `SELECT uniqueCount(db.querylens.queryid) 
                  FROM Metric 
                  WHERE db.system = 'postgresql' 
                  FACET db.name`
    
    s.validateWidgetParity("Database Query Distribution", ohiQuery, otelQuery, 0.95)
}

func (s *OHIDashboardValidationSuite) TestAverageExecutionTime() {
    // Widget: "Average execution time (ms)"
    ohiQuery := "SELECT latest(avg_elapsed_time_ms) from PostgresSlowQueries facet query_text"
    otelQuery := `SELECT latest(db.query.execution_time_mean) 
                  FROM Metric 
                  WHERE db.system = 'postgresql' 
                  FACET db.statement`
    
    s.validateWidgetParity("Average Execution Time", ohiQuery, otelQuery, 0.95)
}

func (s *OHIDashboardValidationSuite) TestTopWaitEvents() {
    // Widget: "Top wait events"
    ohiQuery := "SELECT latest(total_wait_time_ms) from PostgresWaitEvents facet wait_event_name"
    otelQuery := `SELECT sum(wait.duration_ms) 
                  FROM Metric 
                  WHERE db.system = 'postgresql' AND wait.event_name IS NOT NULL
                  FACET wait.event_name`
    
    s.validateWidgetParity("Top Wait Events", ohiQuery, otelQuery, 0.90)
}

func (s *OHIDashboardValidationSuite) TestSlowQueryTable() {
    // Widget: "Top n slowest" - Complex table validation
    requiredFields := []string{
        "database_name", "query_text", "schema_name", 
        "execution_count", "avg_elapsed_time_ms",
        "avg_disk_reads", "avg_disk_writes", "statement_type"
    }
    
    s.validateTableWidgetParity("Top N Slowest Queries", requiredFields)
}

func (s *OHIDashboardValidationSuite) TestBlockingDetails() {
    // Widget: "Blocking details"
    requiredFields := []string{
        "blocked_pid", "blocked_query", "blocked_query_id",
        "blocking_pid", "blocking_query", "blocking_query_id",
        "database_name", "blocking_database"
    }
    
    s.validateBlockingSessionsParity(requiredFields)
}
```

## Metric Mapping System

### Comprehensive Mapping Implementation

```go
// pkg/validation/metric_mapper.go

type MetricMapper struct {
    mappings map[string]MetricMapping
}

type MetricMapping struct {
    OHIEvent       string
    OTELMetric     string
    Transformation TransformationType
    Formula        string
    Attributes     map[string]AttributeMapping
}

type AttributeMapping struct {
    OHIName        string
    OTELName       string
    Transformation func(interface{}) interface{}
}

func (m *MetricMapper) TransformOHIQuery(nrqlQuery string) string {
    // Parse NRQL query
    parsed := m.parseNRQL(nrqlQuery)
    
    // Map event type
    otelFrom := m.mapEventType(parsed.From)
    
    // Map metrics and attributes
    otelSelect := m.mapSelectClause(parsed.Select)
    otelWhere := m.mapWhereClause(parsed.Where)
    otelFacet := m.mapFacetClause(parsed.Facet)
    
    // Reconstruct query
    return m.buildOTELQuery(otelFrom, otelSelect, otelWhere, otelFacet)
}
```

### Transformation Functions

```go
// pkg/validation/transformations.go

type Transformations struct{}

func (t *Transformations) RatePerSecond(value float64, interval time.Duration) float64 {
    return value / interval.Seconds()
}

func (t *Transformations) AnonymizeQuery(query string) string {
    // Replace literals with placeholders
    // Remove PII patterns
    // Normalize whitespace
    return anonymized
}

func (t *Transformations) CalculateBufferHitRatio(hits, reads float64) float64 {
    if hits + reads == 0 {
        return 0
    }
    return hits / (hits + reads) * 100
}
```

## Automated Test Suites

### Test Organization

```yaml
# tests/e2e/configs/ohi_validation_suites.yaml
test_suites:
  ohi_core_metrics:
    description: "Validate core PostgreSQL metrics"
    tests:
      - connection_metrics
      - transaction_metrics
      - buffer_cache_metrics
      - database_size_metrics
      - replication_metrics
      
  ohi_query_performance:
    description: "Validate query performance data"
    tests:
      - slow_query_detection
      - query_execution_metrics
      - query_io_metrics
      - query_plan_metrics
      
  ohi_wait_events:
    description: "Validate wait event tracking"
    tests:
      - wait_event_categories
      - wait_time_aggregation
      - query_wait_correlation
      
  ohi_blocking_sessions:
    description: "Validate blocking detection"
    tests:
      - blocking_session_detection
      - blocking_chain_analysis
      - blocking_duration_tracking
      
  ohi_dashboard_widgets:
    description: "Validate each dashboard widget"
    tests:
      - all 8 widgets from Bird's-Eye View
      - all widgets from Query Details
      - all widgets from Wait Time Analysis
```

### Test Implementation Pattern

```go
// tests/e2e/suites/ohi_widget_validation_test.go

type WidgetValidationTest struct {
    Name           string
    OHIQuery       string
    OTELQuery      string
    Tolerance      float64
    ValidateFunc   func(*ValidationResult) error
}

func (s *OHIDashboardValidationSuite) runWidgetValidation(test WidgetValidationTest) {
    // 1. Execute OHI query
    ohiResults, err := s.nrdb.Query(test.OHIQuery)
    s.Require().NoError(err)
    
    // 2. Execute OTEL query
    otelResults, err := s.nrdb.Query(test.OTELQuery)
    s.Require().NoError(err)
    
    // 3. Compare results
    result := s.compareResults(ohiResults, otelResults)
    
    // 4. Validate accuracy
    s.Assert().GreaterOrEqual(result.Accuracy, test.Tolerance,
        "Widget %s accuracy below threshold: %.2f < %.2f",
        test.Name, result.Accuracy, test.Tolerance)
    
    // 5. Custom validation
    if test.ValidateFunc != nil {
        err = test.ValidateFunc(result)
        s.Assert().NoError(err)
    }
    
    // 6. Record result
    s.recordValidationResult(test.Name, result)
}
```

## Continuous Validation Pipeline

### Validation Runner Architecture

```go
// pkg/validation/continuous_validator.go

type ContinuousValidator struct {
    validator      *OHIParityValidator
    scheduler      *cron.Cron
    alerter        *ParityAlerter
    reporter       *ParityReporter
    driftDetector  *DriftDetector
}

func (cv *ContinuousValidator) Start() {
    // Hourly quick validation
    cv.scheduler.AddFunc("0 * * * *", cv.runQuickValidation)
    
    // Daily comprehensive validation
    cv.scheduler.AddFunc("0 2 * * *", cv.runComprehensiveValidation)
    
    // Weekly trend analysis
    cv.scheduler.AddFunc("0 3 * * 0", cv.runTrendAnalysis)
    
    cv.scheduler.Start()
}

func (cv *ContinuousValidator) runQuickValidation() {
    // Validate critical widgets only
    criticalWidgets := []string{
        "Database Query Distribution",
        "Average Execution Time",
        "Top Wait Events",
    }
    
    results := cv.validator.ValidateWidgets(criticalWidgets)
    cv.checkThresholds(results)
}

func (cv *ContinuousValidator) runComprehensiveValidation() {
    // Validate all dashboard widgets
    allResults := cv.validator.ValidateAllDashboards()
    
    // Generate detailed report
    report := cv.reporter.GenerateDetailedReport(allResults)
    
    // Check for drift
    drift := cv.driftDetector.AnalyzeDrift(allResults)
    
    // Alert if necessary
    if drift.Severity > DriftSeverityWarning {
        cv.alerter.SendDriftAlert(drift)
    }
}
```

### Drift Detection System

```go
// pkg/validation/drift_detector.go

type DriftDetector struct {
    historyStore   *ValidationHistoryStore
    baselineWindow time.Duration
}

type DriftAnalysis struct {
    Timestamp      time.Time
    Severity       DriftSeverity
    AffectedMetrics []MetricDrift
    Recommendations []string
}

type MetricDrift struct {
    MetricName     string
    BaselineAccuracy float64
    CurrentAccuracy  float64
    DriftPercentage  float64
    Trend           string // "improving", "degrading", "stable"
}

func (dd *DriftDetector) AnalyzeDrift(currentResults []ValidationResult) DriftAnalysis {
    baseline := dd.getBaseline()
    
    analysis := DriftAnalysis{
        Timestamp: time.Now(),
    }
    
    for _, result := range currentResults {
        baselineAccuracy := baseline.GetAccuracy(result.Widget.Name)
        drift := dd.calculateDrift(baselineAccuracy, result.Accuracy)
        
        if math.Abs(drift) > 0.02 { // 2% drift threshold
            analysis.AffectedMetrics = append(analysis.AffectedMetrics, MetricDrift{
                MetricName:       result.Widget.Name,
                BaselineAccuracy: baselineAccuracy,
                CurrentAccuracy:  result.Accuracy,
                DriftPercentage:  drift,
                Trend:           dd.getTrend(result.Widget.Name),
            })
        }
    }
    
    analysis.Severity = dd.calculateSeverity(analysis.AffectedMetrics)
    analysis.Recommendations = dd.generateRecommendations(analysis)
    
    return analysis
}
```

### Automated Remediation

```go
// pkg/validation/auto_remediation.go

type AutoRemediator struct {
    configManager  *ConfigurationManager
    metricMapper   *MetricMapper
}

type RemediationAction struct {
    Type           string
    Description    string
    ConfigChanges  map[string]interface{}
    RequiresRestart bool
}

func (ar *AutoRemediator) RemediateDrift(drift DriftAnalysis) []RemediationAction {
    actions := []RemediationAction{}
    
    for _, metric := range drift.AffectedMetrics {
        switch {
        case strings.Contains(metric.MetricName, "execution_time"):
            // Adjust query performance collection thresholds
            actions = append(actions, ar.adjustQueryThresholds(metric))
            
        case strings.Contains(metric.MetricName, "wait_event"):
            // Update wait event sampling
            actions = append(actions, ar.updateWaitEventSampling(metric))
            
        case metric.DriftPercentage > 0.10:
            // Major drift - recommend manual intervention
            actions = append(actions, RemediationAction{
                Type:        "manual_review",
                Description: fmt.Sprintf("Manual review required for %s", metric.MetricName),
            })
        }
    }
    
    return actions
}
```

## Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
1. **Dashboard Parser Implementation**
   ```go
   - Parse dashboard JSON
   - Extract all NRQL queries
   - Catalog OHI events and attributes
   - Generate validation requirements
   ```

2. **Metric Mapping Registry**
   ```yaml
   - Define all OHI → OTEL mappings
   - Document transformations
   - Create unit conversion library
   - Build query translation engine
   ```

### Phase 2: Core Validation (Week 3-4)
1. **Validation Test Suite**
   ```go
   - Implement per-widget tests
   - Create comparison framework
   - Build accuracy calculators
   - Develop issue detection
   ```

2. **Parity Engine**
   ```go
   - Side-by-side data collection
   - Statistical analysis
   - Report generation
   - Threshold management
   ```

### Phase 3: Automation (Week 5-6)
1. **Continuous Validation**
   ```go
   - Scheduled validation runs
   - Drift detection
   - Trend analysis
   - Alert integration
   ```

2. **Auto-Remediation**
   ```go
   - Configuration adjustments
   - Mapping updates
   - Performance tuning
   - Issue resolution
   ```

### Phase 4: Integration (Week 7-8)
1. **Platform Integration**
   - CI/CD pipeline integration
   - Dashboard migration tools
   - Alert conversion utilities
   - Documentation generation

2. **Production Rollout**
   - Staged deployment
   - Performance validation
   - User acceptance testing
   - Go-live support

## Validation Scenarios

### Scenario 1: Daily Validation Run
```yaml
schedule: "0 2 * * *"
steps:
  1. Collect 24h of data from OHI
  2. Collect 24h of data from OTEL
  3. Run all widget validations
  4. Generate accuracy report
  5. Check drift thresholds
  6. Send summary to stakeholders
```

### Scenario 2: Migration Validation
```yaml
trigger: "pre-migration"
steps:
  1. Baseline current OHI metrics
  2. Deploy OTEL in parallel
  3. Run 7-day validation
  4. Analyze trends
  5. Generate migration readiness report
  6. Provide go/no-go recommendation
```

### Scenario 3: Incident Response
```yaml
trigger: "parity_alert"
steps:
  1. Identify affected metrics
  2. Analyze root cause
  3. Apply auto-remediation
  4. Re-validate affected widgets
  5. Escalate if not resolved
  6. Document resolution
```

## Success Metrics

### Platform KPIs
- **Widget Coverage**: 100% of OHI dashboard widgets validated
- **Metric Accuracy**: ≥95% parity for all metrics
- **Validation Frequency**: Hourly for critical, daily for all
- **Drift Detection**: <2% drift tolerance
- **Auto-Remediation**: 80% of issues resolved automatically
- **MTTR**: <30 minutes for parity issues

### Operational Metrics
- **Validation Runtime**: <5 minutes for quick, <30 minutes for comprehensive
- **Resource Usage**: <500MB memory, <10% CPU
- **Alert Accuracy**: <5% false positives
- **Report Generation**: <1 minute
- **Historical Data**: 90 days retention

## Conclusion

This unified validation platform provides:

1. **Complete OHI Dashboard Coverage** - Every widget, metric, and attribute validated
2. **Automated Validation** - Continuous monitoring with drift detection
3. **Intelligent Remediation** - Auto-correction of common issues
4. **Comprehensive Reporting** - Detailed accuracy metrics and trends
5. **Migration Confidence** - Data-driven go/no-go decisions

The platform ensures that the OpenTelemetry implementation maintains complete feature parity with OHI while providing enhanced capabilities for monitoring, analysis, and optimization.