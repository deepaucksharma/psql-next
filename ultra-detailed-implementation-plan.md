# Ultra-Detailed Implementation Plan for OpenTelemetry Database Intelligence Collector v2

## Executive Summary

This document provides a comprehensive implementation plan for enhancing the OpenTelemetry-aligned database observability collector from MVP to production-grade solution. The plan addresses critical gaps in PII protection, feature detection, plan-level intelligence, and operational readiness while maintaining 100% backward compatibility with New Relic's existing database monitoring.

## Current State Assessment

### Strengths
- **OTEL-Native Architecture**: Leverages standard OpenTelemetry receivers (PostgreSQL, MySQL, SQLQuery) and exporters (OTLP)
- **Custom Processors**: Six production-ready processors addressing database-specific needs:
  - AdaptiveSampler: Intelligent query sampling and deduplication
  - CircuitBreaker: Database protection from monitoring overload
  - PlanAttributeExtractor: Query plan analysis and anonymization
  - Verification: PII detection and compliance
  - NRErrorMonitor: Enterprise error tracking
  - CostControl: Resource usage monitoring
- **Complete Metric Coverage**: Full parity with legacy New Relic Postgres integration
- **Robust Anonymization**: Comprehensive query text sanitization with regex patterns
- **Production Configurations**: Multiple deployment patterns (local, gateway, resilient, production)

### Critical Gaps
1. **Incomplete PII Protection**: Plan JSON and wait event data not fully anonymized
2. **No Feature Detection**: Assumes database extensions exist without checking
3. **Plan Intelligence Not Active**: pg_querylens extension exists but not integrated
4. **Limited ASH Implementation**: Only aggregate session counts, no session history
5. **Performance Overhead**: Still uses polling approach instead of zero-overhead collection

## Implementation Phases

### Phase 1: Critical Security and Robustness (Weeks 1-3)

#### 1.1 Complete Query Anonymization & PII Scrubbing

**Objective**: Ensure zero PII leakage across all telemetry data

**Implementation Tasks**:

1. **Extend PlanAttributeExtractor Anonymization**
   ```go
   // processors/planattributeextractor/plan_anonymizer.go
   
   type PlanAnonymizer struct {
       queryAnonymizer *QueryAnonymizer
       jsonParser      *PlanJSONParser
   }
   
   func (pa *PlanAnonymizer) AnonymizePlanJSON(planJSON string) (string, error) {
       // Parse JSON plan structure
       var plan map[string]interface{}
       if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
           return "", err
       }
       
       // Recursively anonymize all literal values in plan nodes
       pa.anonymizePlanNode(plan)
       
       // Re-serialize with consistent formatting
       anonymized, err := json.Marshal(plan)
       return string(anonymized), err
   }
   
   func (pa *PlanAnonymizer) anonymizePlanNode(node map[string]interface{}) {
       // Anonymize filter conditions
       if filter, ok := node["Filter"]; ok {
           node["Filter"] = pa.queryAnonymizer.AnonymizeQuery(filter.(string))
       }
       
       // Anonymize index conditions
       if indexCond, ok := node["Index Cond"]; ok {
           node["Index Cond"] = pa.queryAnonymizer.AnonymizeQuery(indexCond.(string))
       }
       
       // Recursively process child nodes
       if plans, ok := node["Plans"].([]interface{}); ok {
           for _, childPlan := range plans {
               if childMap, ok := childPlan.(map[string]interface{}); ok {
                   pa.anonymizePlanNode(childMap)
               }
           }
       }
   }
   ```

2. **Update Processor Configuration**
   ```yaml
   # processors/planattributeextractor/config.go
   
   type Config struct {
       // ... existing fields ...
       
       PlanAnonymization PlanAnonymizationConfig `mapstructure:"plan_anonymization"`
   }
   
   type PlanAnonymizationConfig struct {
       Enabled              bool     `mapstructure:"enabled"`
       AnonymizeFilters     bool     `mapstructure:"anonymize_filters"`
       AnonymizeJoinConds   bool     `mapstructure:"anonymize_join_conditions"`
       RemoveCostEstimates  bool     `mapstructure:"remove_cost_estimates"`
       SensitiveNodeTypes   []string `mapstructure:"sensitive_node_types"`
   }
   ```

3. **Add Wait Event Anonymization**
   ```go
   // processors/verification/wait_event_sanitizer.go
   
   func SanitizeWaitEventQuery(query string, waitEvent string) string {
       // Apply standard query anonymization
       anonymized := anonymizeQuery(query)
       
       // Additional sanitization for wait event context
       if strings.Contains(waitEvent, "Lock") {
           // Remove specific lock identifiers that might contain table names
           anonymized = regexp.MustCompile(`relation \d+`).ReplaceAllString(anonymized, "relation ?")
       }
       
       return anonymized
   }
   ```

4. **Integration Tests for PII Protection**
   ```go
   // tests/pii_protection_test.go
   
   func TestComprehensivePIISanitization(t *testing.T) {
       testCases := []struct {
           name     string
           input    map[string]interface{}
           expected map[string]interface{}
       }{
           {
               name: "plan_with_sensitive_literals",
               input: map[string]interface{}{
                   "plan_json": `{"Filter": "email = 'user@example.com'"}`,
                   "query_text": "SELECT * FROM users WHERE email = 'user@example.com'",
                   "wait_event_query": "UPDATE users SET token = 'abc123' WHERE id = 1",
               },
               expected: map[string]interface{}{
                   "plan_json": `{"Filter": "email = ?"}`,
                   "query_text": "SELECT * FROM users WHERE email = ?",
                   "wait_event_query": "UPDATE users SET token = ? WHERE id = ?",
               },
           },
       }
       
       // Run through full processor pipeline
       for _, tc := range testCases {
           result := processPipeline(tc.input)
           assert.Equal(t, tc.expected, result)
       }
   }
   ```

#### 1.2 Implement Feature/Extension Detection & Graceful Fallback

**Objective**: Automatically detect database capabilities and adapt collection strategy

**Implementation Tasks**:

1. **Create Feature Detector Component**
   ```go
   // receivers/postgresql/feature_detector.go
   
   type FeatureDetector struct {
       db     *sql.DB
       cache  map[string]FeatureStatus
       mu     sync.RWMutex
       logger *zap.Logger
   }
   
   type FeatureStatus struct {
       Available   bool
       Version     string
       LastChecked time.Time
       Metadata    map[string]interface{}
   }
   
   func (fd *FeatureDetector) DetectFeatures(ctx context.Context) (*FeatureSet, error) {
       features := &FeatureSet{
           Extensions:     make(map[string]FeatureStatus),
           ServerVersion:  fd.getServerVersion(ctx),
           Capabilities:   make(map[string]bool),
       }
       
       // Check extensions
       extensions := []string{
           "pg_stat_statements",
           "pg_wait_sampling", 
           "pg_stat_monitor",
           "auto_explain",
           "pg_querylens",
       }
       
       for _, ext := range extensions {
           features.Extensions[ext] = fd.checkExtension(ctx, ext)
       }
       
       // Check capabilities
       features.Capabilities["track_io_timing"] = fd.checkSetting(ctx, "track_io_timing")
       features.Capabilities["track_functions"] = fd.checkSetting(ctx, "track_functions")
       
       // Check cloud provider
       features.CloudProvider = fd.detectCloudProvider(ctx)
       
       return features, nil
   }
   
   func (fd *FeatureDetector) checkExtension(ctx context.Context, name string) FeatureStatus {
       query := `
           SELECT 
               installed_version,
               default_version,
               comment
           FROM pg_available_extensions
           WHERE name = $1 AND installed_version IS NOT NULL
       `
       
       var status FeatureStatus
       err := fd.db.QueryRowContext(ctx, query, name).Scan(
           &status.Version,
           &defaultVersion,
           &comment,
       )
       
       status.Available = err == nil
       status.LastChecked = time.Now()
       
       return status
   }
   ```

2. **Implement Query Strategy Selection**
   ```go
   // receivers/sqlquery/query_selector.go
   
   type QuerySelector struct {
       features     *FeatureSet
       queryConfigs map[string][]QueryConfig
   }
   
   type QueryConfig struct {
       Name         string
       SQL          string
       Requirements []string // Required extensions/features
       Priority     int      // Higher priority = preferred
       Fallback     string   // Name of fallback query
   }
   
   func (qs *QuerySelector) SelectQueries() []QueryConfig {
       var selected []QueryConfig
       
       // Group queries by category
       categories := []string{"slow_queries", "wait_events", "active_sessions", "plans"}
       
       for _, category := range categories {
           queries := qs.queryConfigs[category]
           
           // Sort by priority
           sort.Slice(queries, func(i, j int) bool {
               return queries[i].Priority > queries[j].Priority
           })
           
           // Select first query whose requirements are met
           for _, query := range queries {
               if qs.meetsRequirements(query.Requirements) {
                   selected = append(selected, query)
                   break
               }
           }
       }
       
       return selected
   }
   
   func (qs *QuerySelector) meetsRequirements(requirements []string) bool {
       for _, req := range requirements {
           if !qs.features.HasFeature(req) {
               return false
           }
       }
       return true
   }
   ```

3. **Define Query Configurations**
   ```yaml
   # config/queries/postgresql.yaml
   
   query_definitions:
     slow_queries:
       - name: "pg_stat_monitor_queries"
         priority: 100
         requirements: ["pg_stat_monitor"]
         sql: |
           SELECT 
             queryid,
             query,
             calls,
             total_time,
             mean_time,
             p99_time,
             rows,
             shared_blks_hit,
             shared_blks_read,
             cpu_user_time,
             cpu_sys_time
           FROM pg_stat_monitor
           WHERE mean_time > $1
           ORDER BY mean_time DESC
           LIMIT $2
           
       - name: "pg_stat_statements_enhanced"
         priority: 90
         requirements: ["pg_stat_statements", "track_io_timing"]
         sql: |
           SELECT 
             queryid::text as query_id,
             query,
             calls,
             total_exec_time as total_time,
             mean_exec_time as mean_time,
             rows,
             shared_blks_hit,
             shared_blks_read,
             blk_read_time,
             blk_write_time
           FROM pg_stat_statements
           WHERE mean_exec_time > $1
           ORDER BY mean_exec_time DESC
           LIMIT $2
           
       - name: "pg_stat_statements_basic"
         priority: 50
         requirements: ["pg_stat_statements"]
         sql: |
           SELECT 
             queryid::text as query_id,
             query,
             calls,
             total_exec_time as total_time,
             mean_exec_time as mean_time,
             rows
           FROM pg_stat_statements
           WHERE mean_exec_time > $1
           ORDER BY mean_exec_time DESC
           LIMIT $2
           
       - name: "pg_stat_activity_fallback"
         priority: 10
         requirements: []
         sql: |
           SELECT 
             pid::text as query_id,
             query,
             1 as calls,
             EXTRACT(EPOCH FROM (now() - query_start)) * 1000 as total_time,
             EXTRACT(EPOCH FROM (now() - query_start)) * 1000 as mean_time,
             0 as rows
           FROM pg_stat_activity
           WHERE state = 'active' 
             AND query_start < now() - interval '1 second'
           ORDER BY query_start
           LIMIT $2
   ```

4. **Add CloudWatch Integration for RDS**
   ```go
   // receivers/cloudwatch/rds_metrics.go
   
   type RDSMetricsCollector struct {
       cwClient    *cloudwatch.Client
       dbInstances []string
       interval    time.Duration
   }
   
   func (rmc *RDSMetricsCollector) CollectMetrics(ctx context.Context) ([]Metric, error) {
       // Define metrics to collect
       metricQueries := []types.MetricDataQuery{
           {
               Id: aws.String("cpu"),
               MetricStat: &types.MetricStat{
                   Metric: &types.Metric{
                       Namespace:  aws.String("AWS/RDS"),
                       MetricName: aws.String("CPUUtilization"),
                       Dimensions: []types.Dimension{
                           {Name: aws.String("DBInstanceIdentifier"), Value: &dbInstance},
                       },
                   },
                   Period: aws.Int32(60),
                   Stat:   aws.String("Average"),
               },
           },
           // Add more metrics: ReadIOPS, WriteIOPS, FreeableMemory, etc.
       }
       
       // Query CloudWatch
       result, err := rmc.cwClient.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
           MetricDataQueries: metricQueries,
           StartTime:         aws.Time(time.Now().Add(-5 * time.Minute)),
           EndTime:           aws.Time(time.Now()),
       })
       
       return rmc.convertToOTELMetrics(result), err
   }
   ```

5. **Circuit Breaker Integration**
   ```go
   // processors/circuitbreaker/adaptive_circuit_breaker.go
   
   type AdaptiveCircuitBreaker struct {
       baseBreaker     *CircuitBreaker
       featureBreakers map[string]*CircuitBreaker
       errorPatterns   map[string]ErrorPattern
   }
   
   type ErrorPattern struct {
       Pattern      *regexp.Regexp
       Feature      string
       Action       string // "disable", "fallback", "retry"
       BackoffTime  time.Duration
   }
   
   func (acb *AdaptiveCircuitBreaker) HandleError(err error, context string) error {
       // Check if error matches known patterns
       for _, pattern := range acb.errorPatterns {
           if pattern.Pattern.MatchString(err.Error()) {
               switch pattern.Action {
               case "disable":
                   acb.DisableFeature(pattern.Feature)
               case "fallback":
                   acb.EnableFallback(pattern.Feature)
               case "retry":
                   return acb.RetryWithBackoff(pattern.BackoffTime)
               }
           }
       }
       
       // Update circuit breaker state
       return acb.baseBreaker.RecordError(err)
   }
   ```

### Phase 2: Plan Intelligence Implementation (Weeks 4-6)

#### 2.1 Safe Plan Collection Integration

**Objective**: Enable execution plan collection without impacting database performance

**Implementation Tasks**:

1. **Auto-Explain Integration**
   ```go
   // receivers/autoexplain/receiver.go
   
   type AutoExplainReceiver struct {
       config       *Config
       logParser    *LogParser
       planCache    *PlanCache
       metrics      *MetricsBuilder
   }
   
   type LogParser struct {
       patterns map[string]*regexp.Regexp
   }
   
   func (lp *LogParser) ParseAutoExplainLog(line string) (*ExplainEntry, error) {
       // Extract auto_explain JSON output
       if match := lp.patterns["json"].FindStringSubmatch(line); match != nil {
           var plan ExplainPlan
           if err := json.Unmarshal([]byte(match[1]), &plan); err != nil {
               return nil, err
           }
           
           return &ExplainEntry{
               Timestamp: extractTimestamp(line),
               Duration:  extractDuration(line),
               QueryID:   hashQuery(plan.Query),
               Plan:      plan,
           }, nil
       }
       
       return nil, nil
   }
   
   type ExplainPlan struct {
       Query       string                 `json:"Query Text"`
       Plan        map[string]interface{} `json:"Plan"`
       TotalCost   float64               `json:"Total Cost"`
       ExecutionTime float64             `json:"Execution Time"`
   }
   ```

2. **Plan Storage and Versioning**
   ```go
   // processors/planattributeextractor/plan_store.go
   
   type PlanStore struct {
       plans      map[string]*PlanHistory
       mu         sync.RWMutex
       maxPlans   int
       ttl        time.Duration
   }
   
   type PlanHistory struct {
       QueryID      string
       Plans        []*VersionedPlan
       CurrentPlan  *VersionedPlan
       LastRegression *PlanRegression
   }
   
   type VersionedPlan struct {
       PlanID       string
       PlanHash     string
       PlanJSON     string
       FirstSeen    time.Time
       LastSeen     time.Time
       Metrics      PlanMetrics
       Version      int
   }
   
   type PlanMetrics struct {
       ExecutionCount int64
       TotalTime      float64
       MinTime        float64
       MaxTime        float64
       MeanTime       float64
       StdDev         float64
       P95Time        float64
       P99Time        float64
   }
   
   func (ps *PlanStore) AddPlan(queryID string, plan *VersionedPlan) (*PlanRegression, error) {
       ps.mu.Lock()
       defer ps.mu.Unlock()
       
       history, exists := ps.plans[queryID]
       if !exists {
           history = &PlanHistory{QueryID: queryID}
           ps.plans[queryID] = history
       }
       
       // Check if this is a new plan
       if history.CurrentPlan == nil || history.CurrentPlan.PlanHash != plan.PlanHash {
           plan.Version = len(history.Plans) + 1
           history.Plans = append(history.Plans, plan)
           
           // Check for regression
           if history.CurrentPlan != nil {
               regression := ps.detectRegression(history.CurrentPlan, plan)
               if regression != nil {
                   history.LastRegression = regression
                   return regression, nil
               }
           }
           
           history.CurrentPlan = plan
       }
       
       return nil, nil
   }
   ```

3. **Plan Regression Detection**
   ```go
   // processors/planattributeextractor/regression_detector.go
   
   type RegressionDetector struct {
       thresholds RegressionThresholds
       analyzer   *PlanAnalyzer
   }
   
   type RegressionThresholds struct {
       PerformanceDegradation float64 // e.g., 0.2 for 20% slower
       CostIncrease          float64 // e.g., 0.5 for 50% higher cost
       MinExecutions         int64   // Minimum executions before comparison
       StatisticalConfidence float64 // e.g., 0.95 for 95% confidence
   }
   
   func (rd *RegressionDetector) DetectRegression(oldPlan, newPlan *VersionedPlan) *PlanRegression {
       // Skip if insufficient data
       if newPlan.Metrics.ExecutionCount < rd.thresholds.MinExecutions {
           return nil
       }
       
       // Calculate performance change
       perfChange := (newPlan.Metrics.MeanTime - oldPlan.Metrics.MeanTime) / oldPlan.Metrics.MeanTime
       
       // Statistical significance test
       if !rd.isStatisticallySignificant(oldPlan.Metrics, newPlan.Metrics) {
           return nil
       }
       
       // Check thresholds
       if perfChange > rd.thresholds.PerformanceDegradation {
           return &PlanRegression{
               QueryID:          newPlan.QueryID,
               OldPlanID:        oldPlan.PlanID,
               NewPlanID:        newPlan.PlanID,
               PerformanceChange: perfChange,
               RegressionType:   "performance",
               DetectedAt:       time.Now(),
               Confidence:       rd.calculateConfidence(oldPlan, newPlan),
               Impact:           rd.assessImpact(oldPlan, newPlan),
           }
       }
       
       return nil
   }
   
   func (rd *RegressionDetector) isStatisticallySignificant(old, new PlanMetrics) bool {
       // Use Welch's t-test for unequal variances
       t := (new.MeanTime - old.MeanTime) / 
            math.Sqrt((new.StdDev*new.StdDev)/float64(new.ExecutionCount) + 
                     (old.StdDev*old.StdDev)/float64(old.ExecutionCount))
       
       // Calculate degrees of freedom
       df := rd.welchSatterthwaite(old, new)
       
       // Compare with critical value
       criticalValue := rd.tDistributionCriticalValue(rd.thresholds.StatisticalConfidence, df)
       
       return math.Abs(t) > criticalValue
   }
   ```

4. **Plan Analysis Engine**
   ```go
   // processors/planattributeextractor/plan_analyzer.go
   
   type PlanAnalyzer struct {
       nodeAnalyzers map[string]NodeAnalyzer
       costModel     *CostModel
   }
   
   type NodeAnalyzer interface {
       Analyze(node map[string]interface{}) NodeAnalysis
   }
   
   type NodeAnalysis struct {
       NodeType        string
       EstimatedCost   float64
       ActualTime      float64
       RowsEstimated   int64
       RowsActual      int64
       Issues          []PlanIssue
       Recommendations []string
   }
   
   type PlanIssue struct {
       Severity    string // "critical", "warning", "info"
       Type        string // "estimation_error", "missing_index", "inefficient_join"
       Description string
       Impact      float64
   }
   
   func (pa *PlanAnalyzer) AnalyzePlan(planJSON string) (*PlanAnalysis, error) {
       var plan map[string]interface{}
       if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
           return nil, err
       }
       
       analysis := &PlanAnalysis{
           TotalCost:       pa.extractTotalCost(plan),
           ExecutionTime:   pa.extractExecutionTime(plan),
           NodeAnalyses:    make([]NodeAnalysis, 0),
           Issues:          make([]PlanIssue, 0),
           Recommendations: make([]string, 0),
       }
       
       // Recursively analyze plan nodes
       pa.analyzeNode(plan["Plan"].(map[string]interface{}), analysis)
       
       // Generate recommendations based on issues
       analysis.Recommendations = pa.generateRecommendations(analysis.Issues)
       
       return analysis, nil
   }
   
   func (pa *PlanAnalyzer) analyzeNode(node map[string]interface{}, analysis *PlanAnalysis) {
       nodeType := node["Node Type"].(string)
       
       // Use appropriate analyzer
       if analyzer, exists := pa.nodeAnalyzers[nodeType]; exists {
           nodeAnalysis := analyzer.Analyze(node)
           analysis.NodeAnalyses = append(analysis.NodeAnalyses, nodeAnalysis)
           analysis.Issues = append(analysis.Issues, nodeAnalysis.Issues...)
       }
       
       // Analyze child nodes
       if plans, ok := node["Plans"].([]interface{}); ok {
           for _, childPlan := range plans {
               pa.analyzeNode(childPlan.(map[string]interface{}), analysis)
           }
       }
   }
   ```

#### 2.2 pg_querylens Integration (Optional Advanced Feature)

**Objective**: Enable zero-overhead telemetry collection via shared memory

**Implementation Tasks**:

1. **Shared Memory Reader**
   ```go
   // receivers/pgquerylens/shared_memory_reader.go
   
   type SharedMemoryReader struct {
       shmPath     string
       shmSize     int
       buffer      []byte
       lastOffset  int64
   }
   
   func (smr *SharedMemoryReader) ReadTelemetry() ([]*TelemetryEvent, error) {
       // Map shared memory
       file, err := os.OpenFile(smr.shmPath, os.O_RDONLY, 0)
       if err != nil {
           return nil, err
       }
       defer file.Close()
       
       // Read circular buffer header
       header := &CircularBufferHeader{}
       if err := binary.Read(file, binary.LittleEndian, header); err != nil {
           return nil, err
       }
       
       // Read new events since last offset
       events := make([]*TelemetryEvent, 0)
       currentOffset := smr.lastOffset
       
       for currentOffset != header.WriteOffset {
           event := &TelemetryEvent{}
           if err := smr.readEvent(file, currentOffset, event); err != nil {
               return nil, err
           }
           
           events = append(events, event)
           currentOffset = (currentOffset + event.Size) % smr.shmSize
       }
       
       smr.lastOffset = currentOffset
       return events, nil
   }
   ```

2. **Event Stream Processor**
   ```go
   // receivers/pgquerylens/event_processor.go
   
   type EventProcessor struct {
       reader       *SharedMemoryReader
       metrics      *MetricsBuilder
       planStore    *PlanStore
       eventBuffer  chan *TelemetryEvent
   }
   
   func (ep *EventProcessor) Start(ctx context.Context) error {
       ticker := time.NewTicker(100 * time.Millisecond) // High frequency polling
       defer ticker.Stop()
       
       for {
           select {
           case <-ctx.Done():
               return ctx.Err()
           case <-ticker.C:
               events, err := ep.reader.ReadTelemetry()
               if err != nil {
                   ep.logger.Error("Failed to read telemetry", zap.Error(err))
                   continue
               }
               
               for _, event := range events {
                   ep.processEvent(event)
               }
           }
       }
   }
   
   func (ep *EventProcessor) processEvent(event *TelemetryEvent) {
       switch event.Type {
       case EventTypeQuery:
           ep.processQueryEvent(event)
       case EventTypePlan:
           ep.processPlanEvent(event)
       case EventTypeWait:
           ep.processWaitEvent(event)
       }
   }
   ```

### Phase 3: Active Session History Implementation (Weeks 7-8)

#### 3.1 Enhanced ASH Collection

**Objective**: Implement comprehensive session history tracking

**Implementation Tasks**:

1. **ASH Collector Design**
   ```go
   // receivers/ash/collector.go
   
   type ASHCollector struct {
       db           *sql.DB
       interval     time.Duration
       retention    time.Duration
       storage      *ASHStorage
       sampler      *AdaptiveSampler
   }
   
   type SessionSnapshot struct {
       Timestamp       time.Time
       PID             int
       Username        string
       ApplicationName string
       ClientAddr      string
       BackendStart    time.Time
       QueryStart      *time.Time
       State           string
       WaitEventType   *string
       WaitEvent       *string
       QueryID         *string
       Query           string
       BlockingPID     *int
       LockType        *string
   }
   
   func (ac *ASHCollector) CollectSnapshot(ctx context.Context) ([]*SessionSnapshot, error) {
       query := `
           WITH active_sessions AS (
               SELECT 
                   a.pid,
                   a.usename,
                   a.application_name,
                   a.client_addr::text,
                   a.backend_start,
                   a.query_start,
                   a.state,
                   a.wait_event_type,
                   a.wait_event,
                   a.query,
                   s.queryid
               FROM pg_stat_activity a
               LEFT JOIN pg_stat_statements s 
                   ON s.query = a.query AND s.userid = a.usesysid
               WHERE a.state != 'idle' 
                   AND a.backend_type = 'client backend'
           ),
           blocking_info AS (
               SELECT 
                   blocked.pid AS blocked_pid,
                   blocking.pid AS blocking_pid,
                   blocked_locks.locktype
               FROM pg_locks blocked_locks
               JOIN pg_locks blocking_locks 
                   ON blocking_locks.locktype = blocked_locks.locktype
                   AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
                   AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
                   AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
                   AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
                   AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
                   AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
                   AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
                   AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
                   AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
                   AND blocking_locks.granted
               JOIN pg_stat_activity blocked 
                   ON blocked.pid = blocked_locks.pid
               JOIN pg_stat_activity blocking 
                   ON blocking.pid = blocking_locks.pid
               WHERE NOT blocked_locks.granted
           )
           SELECT 
               current_timestamp as snapshot_time,
               a.*,
               b.blocking_pid,
               b.locktype
           FROM active_sessions a
           LEFT JOIN blocking_info b ON a.pid = b.blocked_pid
       `
       
       rows, err := ac.db.QueryContext(ctx, query)
       if err != nil {
           return nil, err
       }
       defer rows.Close()
       
       snapshots := make([]*SessionSnapshot, 0)
       for rows.Next() {
           snapshot := &SessionSnapshot{}
           // Scan all fields...
           snapshots = append(snapshots, snapshot)
       }
       
       return ac.sampler.Sample(snapshots), nil
   }
   ```

2. **ASH Storage Engine**
   ```go
   // receivers/ash/storage.go
   
   type ASHStorage struct {
       buffer      *CircularBuffer
       aggregator  *TimeWindowAggregator
       compressor  *SnapshotCompressor
   }
   
   type CircularBuffer struct {
       snapshots  []*SessionSnapshot
       capacity   int
       head       int
       tail       int
       mu         sync.RWMutex
   }
   
   func (cb *CircularBuffer) Add(snapshot *SessionSnapshot) {
       cb.mu.Lock()
       defer cb.mu.Unlock()
       
       cb.snapshots[cb.head] = snapshot
       cb.head = (cb.head + 1) % cb.capacity
       
       if cb.head == cb.tail {
           cb.tail = (cb.tail + 1) % cb.capacity
       }
   }
   
   type TimeWindowAggregator struct {
       windows map[time.Duration]*AggregatedWindow
   }
   
   type AggregatedWindow struct {
       StartTime    time.Time
       EndTime      time.Time
       SessionCount map[string]int // By state
       WaitEvents   map[string]int // By wait event
       TopQueries   []QuerySummary
       TopWaits     []WaitSummary
   }
   
   func (twa *TimeWindowAggregator) Aggregate(snapshots []*SessionSnapshot, window time.Duration) *AggregatedWindow {
       agg := &AggregatedWindow{
           SessionCount: make(map[string]int),
           WaitEvents:   make(map[string]int),
       }
       
       queryStats := make(map[string]*QuerySummary)
       waitStats := make(map[string]*WaitSummary)
       
       for _, snapshot := range snapshots {
           // Count by state
           agg.SessionCount[snapshot.State]++
           
           // Count wait events
           if snapshot.WaitEvent != nil {
               agg.WaitEvents[*snapshot.WaitEvent]++
               
               // Track wait statistics
               if stats, exists := waitStats[*snapshot.WaitEvent]; exists {
                   stats.Count++
                   stats.TotalTime += time.Since(*snapshot.QueryStart)
               } else {
                   waitStats[*snapshot.WaitEvent] = &WaitSummary{
                       WaitEvent: *snapshot.WaitEvent,
                       Count:     1,
                       TotalTime: time.Since(*snapshot.QueryStart),
                   }
               }
           }
           
           // Track query statistics
           if snapshot.QueryID != nil {
               if stats, exists := queryStats[*snapshot.QueryID]; exists {
                   stats.Count++
               } else {
                   queryStats[*snapshot.QueryID] = &QuerySummary{
                       QueryID: *snapshot.QueryID,
                       Count:   1,
                   }
               }
           }
       }
       
       // Convert maps to sorted slices
       agg.TopQueries = twa.getTopQueries(queryStats, 10)
       agg.TopWaits = twa.getTopWaits(waitStats, 10)
       
       return agg
   }
   ```

3. **Wait Event Analysis**
   ```go
   // processors/waitanalysis/processor.go
   
   type WaitAnalysisProcessor struct {
       waitPatterns map[string]WaitPattern
       alertRules   []WaitAlertRule
   }
   
   type WaitPattern struct {
       Name        string
       EventTypes  []string
       Events      []string
       Category    string // "Lock", "IO", "CPU", "Network", "IPC"
       Severity    string // "info", "warning", "critical"
       Description string
   }
   
   type WaitAlertRule struct {
       Name       string
       Condition  string // e.g., "wait_time > 5s AND event = 'Lock:relation'"
       Threshold  float64
       Window     time.Duration
       Action     string
   }
   
   func (wap *WaitAnalysisProcessor) AnalyzeWaitEvents(snapshots []*SessionSnapshot) *WaitAnalysis {
       analysis := &WaitAnalysis{
           Timestamp:    time.Now(),
           TotalSessions: len(snapshots),
           WaitSummary:  make(map[string]*WaitCategorySummary),
       }
       
       // Categorize wait events
       for _, snapshot := range snapshots {
           if snapshot.WaitEvent == nil {
               continue
           }
           
           pattern := wap.identifyPattern(*snapshot.WaitEvent)
           if pattern == nil {
               continue
           }
           
           if summary, exists := analysis.WaitSummary[pattern.Category]; exists {
               summary.Count++
               summary.Sessions = append(summary.Sessions, snapshot.PID)
           } else {
               analysis.WaitSummary[pattern.Category] = &WaitCategorySummary{
                   Category: pattern.Category,
                   Count:    1,
                   Sessions: []int{snapshot.PID},
               }
           }
       }
       
       // Check alert rules
       analysis.Alerts = wap.checkAlertRules(snapshots)
       
       return analysis
   }
   ```

### Phase 4: Testing and Performance Optimization (Weeks 9-10)

#### 4.1 Comprehensive Testing Suite

**Objective**: Ensure reliability and performance under various conditions

**Implementation Tasks**:

1. **Load Testing Framework**
   ```go
   // tests/load/framework.go
   
   type LoadTestFramework struct {
       collector     *Collector
       dbSimulator   *DatabaseSimulator
       metrics       *TestMetrics
   }
   
   type DatabaseSimulator struct {
       queryPatterns []QueryPattern
       workloadProfiles []WorkloadProfile
   }
   
   type QueryPattern struct {
       Name         string
       Template     string
       Parameters   []interface{}
       Frequency    int // queries per second
       Distribution string // "uniform", "poisson", "burst"
   }
   
   type WorkloadProfile struct {
       Name        string
       Duration    time.Duration
       Patterns    []QueryPattern
       Concurrency int
   }
   
   func (ltf *LoadTestFramework) RunScenario(scenario string) (*TestReport, error) {
       profile := ltf.getWorkloadProfile(scenario)
       
       // Start collector
       ctx, cancel := context.WithTimeout(context.Background(), profile.Duration)
       defer cancel()
       
       go ltf.collector.Start(ctx)
       
       // Generate load
       var wg sync.WaitGroup
       errors := make(chan error, 1000)
       
       for i := 0; i < profile.Concurrency; i++ {
           wg.Add(1)
           go func() {
               defer wg.Done()
               ltf.generateLoad(ctx, profile, errors)
           }()
       }
       
       // Collect metrics
       go ltf.collectMetrics(ctx)
       
       wg.Wait()
       
       return ltf.generateReport(), nil
   }
   ```

2. **Cardinality Testing**
   ```go
   // tests/cardinality/high_cardinality_test.go
   
   func TestHighCardinalityQueries(t *testing.T) {
       // Generate database with high cardinality
       db := setupTestDatabase(t)
       defer db.Close()
       
       // Create tables with many unique queries
       createHighCardinalitySchema(db, 1000) // 1000 tables
       
       // Generate unique queries
       queries := generateUniqueQueries(10000) // 10k unique patterns
       
       // Configure collector with adaptive sampling
       config := &Config{
           AdaptiveSampler: AdaptiveSamplerConfig{
               Enabled:        true,
               MaxCardinality: 1000,
               SampleRates: map[string]float64{
                   "slow":   1.0,  // Keep all slow queries
                   "normal": 0.1,  // Sample 10% of normal queries
                   "fast":   0.01, // Sample 1% of fast queries
               },
           },
       }
       
       collector := NewCollector(config)
       
       // Run test
       ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
       defer cancel()
       
       // Execute queries
       for _, query := range queries {
           go executeQuery(db, query)
       }
       
       // Monitor memory usage
       memBefore := runtime.MemStats{}
       runtime.ReadMemStats(&memBefore)
       
       collector.Start(ctx)
       
       memAfter := runtime.MemStats{}
       runtime.ReadMemStats(&memAfter)
       
       // Verify memory usage is bounded
       memIncrease := memAfter.Alloc - memBefore.Alloc
       assert.Less(t, memIncrease, uint64(100*1024*1024), "Memory usage should be less than 100MB")
       
       // Verify cardinality limits
       metrics := collector.GetMetrics()
       uniqueQueries := countUniqueQueries(metrics)
       assert.LessOrEqual(t, uniqueQueries, 1000, "Should limit to 1000 unique queries")
   }
   ```

3. **Regression Testing**
   ```go
   // tests/regression/compatibility_test.go
   
   func TestBackwardCompatibility(t *testing.T) {
       // Load legacy OHI output
       legacyOutput := loadLegacyOHIOutput(t, "testdata/ohi_output.json")
       
       // Configure new collector to match legacy
       config := &Config{
           CompatibilityMode: true,
           MetricMapping: map[string]string{
               "db.postgresql.query.duration": "avg_elapsed_time_ms",
               "db.postgresql.query.calls":    "execution_count",
               // ... more mappings
           },
       }
       
       // Run collector against same database state
       newOutput := runCollector(t, config)
       
       // Compare outputs
       for _, legacyEvent := range legacyOutput.Events {
           newEvent := findMatchingEvent(newOutput, legacyEvent)
           require.NotNil(t, newEvent, "Should find matching event for %s", legacyEvent.ID)
           
           // Compare all fields
           for field, legacyValue := range legacyEvent.Fields {
               newValue := newEvent.Fields[field]
               assert.Equal(t, legacyValue, newValue, 
                   "Field %s should match (query: %s)", field, legacyEvent.Query)
           }
       }
   }
   ```

#### 4.2 Performance Optimization

**Objective**: Minimize collector overhead and database impact

**Implementation Tasks**:

1. **Query Optimization**
   ```sql
   -- Optimized slow query collection
   CREATE OR REPLACE FUNCTION get_slow_queries(
       threshold_ms FLOAT,
       limit_count INT,
       sample_rate FLOAT DEFAULT 1.0
   ) RETURNS TABLE (
       query_id TEXT,
       query_text TEXT,
       calls BIGINT,
       total_time FLOAT,
       mean_time FLOAT,
       -- ... other fields
   ) AS $$
   BEGIN
       -- Use sampling for high-volume databases
       IF sample_rate < 1.0 THEN
           RETURN QUERY
           SELECT * FROM (
               SELECT 
                   queryid::text,
                   query,
                   calls,
                   total_exec_time,
                   mean_exec_time,
                   -- ... other fields
               FROM pg_stat_statements
               WHERE mean_exec_time > threshold_ms
                   AND random() < sample_rate
           ) sampled
           ORDER BY mean_exec_time DESC
           LIMIT limit_count;
       ELSE
           -- Full collection for normal load
           RETURN QUERY
           SELECT 
               queryid::text,
               query,
               calls,
               total_exec_time,
               mean_exec_time,
               -- ... other fields
           FROM pg_stat_statements
           WHERE mean_exec_time > threshold_ms
           ORDER BY mean_exec_time DESC
           LIMIT limit_count;
       END IF;
   END;
   $$ LANGUAGE plpgsql;
   ```

2. **Batching and Caching**
   ```go
   // processors/cache/query_cache.go
   
   type QueryCache struct {
       cache    *lru.Cache
       ttl      time.Duration
       hitRate  *RateMeter
   }
   
   type CachedQuery struct {
       QueryID      string
       QueryText    string
       Fingerprint  string
       AnonymizedText string
       CachedAt     time.Time
   }
   
   func (qc *QueryCache) GetOrCompute(
       queryID string, 
       computeFn func() (*CachedQuery, error),
   ) (*CachedQuery, error) {
       // Check cache
       if cached, ok := qc.cache.Get(queryID); ok {
           cachedQuery := cached.(*CachedQuery)
           if time.Since(cachedQuery.CachedAt) < qc.ttl {
               qc.hitRate.Mark(true)
               return cachedQuery, nil
           }
       }
       
       qc.hitRate.Mark(false)
       
       // Compute and cache
       result, err := computeFn()
       if err != nil {
           return nil, err
       }
       
       result.CachedAt = time.Now()
       qc.cache.Add(queryID, result)
       
       return result, nil
   }
   ```

### Phase 5: Deployment and Integration (Weeks 11-12)

#### 5.1 Packaging and Distribution

**Objective**: Make deployment seamless across environments

**Implementation Tasks**:

1. **Helm Chart Creation**
   ```yaml
   # deployments/helm/db-intelligence/values.yaml
   
   replicaCount: 1
   
   image:
     repository: newrelic/otel-db-collector
     pullPolicy: IfNotPresent
     tag: "2.0.0"
   
   config:
     # Feature flags
     features:
       ashEnabled: true
       planTrackingEnabled: true
       adaptiveSamplingEnabled: true
       circuitBreakerEnabled: true
     
     # Database connections
     databases:
       - name: primary
         type: postgresql
         host: "{{ .Values.database.host }}"
         port: 5432
         username: "{{ .Values.database.username }}"
         password: "{{ .Values.database.password }}"
         
     # Extension detection
     extensionDetection:
       enabled: true
       checkInterval: 5m
       requiredExtensions: []
       optionalExtensions:
         - pg_stat_statements
         - pg_wait_sampling
         - auto_explain
         
     # Resource limits
     resources:
       limits:
         cpu: 2000m
         memory: 2Gi
       requests:
         cpu: 500m
         memory: 512Mi
         
     # New Relic integration
     newrelic:
       endpoint: "otlp.nr-data.net:4317"
       apiKey: "{{ .Values.newrelic.apiKey }}"
       
   monitoring:
     prometheus:
       enabled: true
       port: 8888
     
   healthcheck:
     enabled: true
     port: 13133
   ```

2. **Docker Multi-Stage Build**
   ```dockerfile
   # Dockerfile
   
   # Build stage
   FROM golang:1.23-alpine AS builder
   
   RUN apk add --no-cache git make
   
   WORKDIR /app
   
   # Copy go mod files
   COPY go.mod go.sum ./
   RUN go mod download
   
   # Copy source
   COPY . .
   
   # Build with optimizations
   RUN CGO_ENABLED=0 GOOS=linux go build \
       -ldflags="-w -s -X main.version=${VERSION}" \
       -o otelcol-custom \
       ./otelcol-custom
   
   # Runtime stage
   FROM alpine:3.19
   
   RUN apk add --no-cache ca-certificates
   
   # Create non-root user
   RUN addgroup -g 10001 -S otel && \
       adduser -u 10001 -S otel -G otel
   
   WORKDIR /
   
   # Copy binary
   COPY --from=builder /app/otelcol-custom /otelcol-custom
   
   # Copy configs
   COPY configs /configs
   
   # Set ownership
   RUN chown -R otel:otel /otelcol-custom /configs
   
   USER otel
   
   EXPOSE 4317 4318 8888 13133
   
   ENTRYPOINT ["/otelcol-custom"]
   CMD ["--config", "/configs/collector.yaml"]
   ```

3. **Installation Script**
   ```bash
   #!/bin/bash
   # install.sh
   
   set -e
   
   INSTALL_DIR="/opt/newrelic/otel-db-collector"
   CONFIG_DIR="/etc/newrelic/otel-db-collector"
   SERVICE_NAME="nr-otel-db-collector"
   
   # Detect OS
   if [[ -f /etc/os-release ]]; then
       . /etc/os-release
       OS=$ID
       VER=$VERSION_ID
   fi
   
   # Install dependencies
   case $OS in
       ubuntu|debian)
           apt-get update
           apt-get install -y curl jq
           ;;
       centos|rhel|fedora)
           yum install -y curl jq
           ;;
       *)
           echo "Unsupported OS: $OS"
           exit 1
           ;;
   esac
   
   # Download collector
   echo "Downloading OTEL DB Collector..."
   LATEST_VERSION=$(curl -s https://api.github.com/repos/newrelic/otel-db-collector/releases/latest | jq -r .tag_name)
   curl -L "https://github.com/newrelic/otel-db-collector/releases/download/${LATEST_VERSION}/otel-db-collector_${LATEST_VERSION}_linux_amd64.tar.gz" | tar -xz -C /tmp
   
   # Install binary
   mkdir -p "$INSTALL_DIR"
   cp /tmp/otel-db-collector "$INSTALL_DIR/"
   chmod +x "$INSTALL_DIR/otel-db-collector"
   
   # Setup configuration
   mkdir -p "$CONFIG_DIR"
   cp /tmp/configs/* "$CONFIG_DIR/"
   
   # Create systemd service
   cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
   [Unit]
   Description=New Relic OpenTelemetry Database Collector
   After=network.target
   
   [Service]
   Type=simple
   User=newrelic
   Group=newrelic
   ExecStart=${INSTALL_DIR}/otel-db-collector --config ${CONFIG_DIR}/collector.yaml
   Restart=on-failure
   RestartSec=10
   
   # Security
   NoNewPrivileges=true
   PrivateTmp=true
   ProtectSystem=strict
   ProtectHome=true
   ReadWritePaths=/var/log/newrelic
   
   [Install]
   WantedBy=multi-user.target
   EOF
   
   # Create user
   useradd -r -s /bin/false newrelic || true
   
   # Set permissions
   chown -R newrelic:newrelic "$INSTALL_DIR" "$CONFIG_DIR"
   
   # Enable service
   systemctl daemon-reload
   systemctl enable "$SERVICE_NAME"
   
   echo "Installation complete!"
   echo "Configure your database connection in ${CONFIG_DIR}/collector.yaml"
   echo "Start the service with: systemctl start ${SERVICE_NAME}"
   ```

#### 5.2 Migration Tools and Procedures

**Objective**: Enable smooth transition from legacy integration

**Implementation Tasks**:

1. **Migration Validator**
   ```go
   // tools/migration/validator.go
   
   type MigrationValidator struct {
       legacyCollector *LegacyCollector
       newCollector    *OTELCollector
       tolerance       float64
   }
   
   func (mv *MigrationValidator) ValidateMetrics(duration time.Duration) (*ValidationReport, error) {
       // Run both collectors in parallel
       ctx, cancel := context.WithTimeout(context.Background(), duration)
       defer cancel()
       
       var wg sync.WaitGroup
       var legacyMetrics, newMetrics []Metric
       var legacyErr, newErr error
       
       wg.Add(2)
       
       go func() {
           defer wg.Done()
           legacyMetrics, legacyErr = mv.legacyCollector.Collect(ctx)
       }()
       
       go func() {
           defer wg.Done()
           newMetrics, newErr = mv.newCollector.Collect(ctx)
       }()
       
       wg.Wait()
       
       if legacyErr != nil || newErr != nil {
           return nil, fmt.Errorf("collection errors: legacy=%v, new=%v", legacyErr, newErr)
       }
       
       // Compare metrics
       report := &ValidationReport{
           StartTime: time.Now().Add(-duration),
           EndTime:   time.Now(),
           Results:   make([]ComparisonResult, 0),
       }
       
       for _, legacyMetric := range legacyMetrics {
           newMetric := findCorrespondingMetric(newMetrics, legacyMetric)
           if newMetric == nil {
               report.Results = append(report.Results, ComparisonResult{
                   MetricName: legacyMetric.Name,
                   Status:     "missing",
                   Message:    "Metric not found in new collector",
               })
               continue
           }
           
           // Compare values
           diff := math.Abs(legacyMetric.Value - newMetric.Value) / legacyMetric.Value
           if diff > mv.tolerance {
               report.Results = append(report.Results, ComparisonResult{
                   MetricName:  legacyMetric.Name,
                   Status:      "mismatch",
                   LegacyValue: legacyMetric.Value,
                   NewValue:    newMetric.Value,
                   Difference:  diff,
               })
           } else {
               report.Results = append(report.Results, ComparisonResult{
                   MetricName: legacyMetric.Name,
                   Status:     "match",
               })
           }
       }
       
       report.Summary = mv.generateSummary(report.Results)
       return report, nil
   }
   ```

2. **Phased Rollout Controller**
   ```go
   // tools/rollout/controller.go
   
   type RolloutController struct {
       inventory    *DatabaseInventory
       deployments  map[string]*Deployment
       metrics      *RolloutMetrics
   }
   
   type RolloutPhase struct {
       Name       string
       Percentage float64
       Criteria   []SuccessCriteria
       Duration   time.Duration
   }
   
   var DefaultRolloutPhases = []RolloutPhase{
       {Name: "validation", Percentage: 0, Duration: 24 * time.Hour},
       {Name: "canary", Percentage: 5, Duration: 48 * time.Hour},
       {Name: "early", Percentage: 25, Duration: 72 * time.Hour},
       {Name: "broad", Percentage: 50, Duration: 72 * time.Hour},
       {Name: "complete", Percentage: 100, Duration: 0},
   }
   
   func (rc *RolloutController) ExecuteRollout(ctx context.Context) error {
       for _, phase := range DefaultRolloutPhases {
           if err := rc.executePhase(ctx, phase); err != nil {
               return rc.rollback(phase, err)
           }
       }
       
       return nil
   }
   
   func (rc *RolloutController) executePhase(ctx context.Context, phase RolloutPhase) error {
       // Select databases for this phase
       databases := rc.selectDatabases(phase.Percentage)
       
       // Deploy to selected databases
       for _, db := range databases {
           if err := rc.deployToDatabase(ctx, db); err != nil {
               return fmt.Errorf("deployment failed for %s: %w", db.Name, err)
           }
       }
       
       // Monitor for phase duration
       if phase.Duration > 0 {
           if err := rc.monitorPhase(ctx, phase); err != nil {
               return err
           }
       }
       
       // Validate success criteria
       for _, criteria := range phase.Criteria {
           if !criteria.Evaluate(rc.metrics) {
               return fmt.Errorf("criteria %s not met", criteria.Name)
           }
       }
       
       return nil
   }
   ```

## Success Metrics and Monitoring

### Key Performance Indicators

1. **Collection Overhead**
   - Target: < 1% CPU overhead on database
   - Query execution time: < 100ms for all collection queries
   - Memory usage: < 500MB per collector instance

2. **Data Quality**
   - PII detection rate: 100% of known patterns
   - Query anonymization accuracy: 99.9%
   - Metric accuracy vs legacy: ±1% tolerance

3. **Operational Metrics**
   - Deployment success rate: > 99%
   - Mean time to detect issues: < 5 minutes
   - Plan regression detection accuracy: > 95%

### Monitoring Dashboard

```yaml
# monitoring/dashboards/collector-health.json

{
  "dashboard": {
    "title": "OTEL DB Collector Health",
    "panels": [
      {
        "title": "Collection Rate",
        "targets": [{
          "expr": "rate(otelcol_receiver_accepted_metric_points[5m])"
        }]
      },
      {
        "title": "Error Rate",
        "targets": [{
          "expr": "rate(otelcol_processor_dropped_metric_points[5m])"
        }]
      },
      {
        "title": "Memory Usage",
        "targets": [{
          "expr": "go_memstats_heap_alloc_bytes{job=\"otel-collector\"}"
        }]
      },
      {
        "title": "PII Detections",
        "targets": [{
          "expr": "increase(otelcol_processor_pii_detected_total[1h])"
        }]
      },
      {
        "title": "Plan Regressions Detected",
        "targets": [{
          "expr": "increase(db_postgresql_plan_regressions_total[1h])"
        }]
      }
    ]
  }
}
```

## Risk Mitigation

### Technical Risks

1. **Performance Impact**
   - Mitigation: Adaptive sampling, circuit breakers, query timeouts
   - Monitoring: Database CPU/IO metrics, query execution times

2. **Data Loss**
   - Mitigation: Persistent queues, retry logic, failover exporters
   - Monitoring: Export success rates, queue depths

3. **Security Vulnerabilities**
   - Mitigation: Comprehensive PII scrubbing, encrypted transport, RBAC
   - Monitoring: PII detection alerts, access logs

### Operational Risks

1. **Migration Failures**
   - Mitigation: Phased rollout, rollback procedures, validation tools
   - Monitoring: Deployment success metrics, data comparison reports

2. **Extension Dependencies**
   - Mitigation: Feature detection, graceful degradation, fallback queries
   - Monitoring: Extension availability metrics, feature usage stats

## Conclusion

This ultra-detailed implementation plan provides a comprehensive roadmap for evolving the OpenTelemetry Database Intelligence Collector from MVP to production-grade solution. The plan addresses all critical gaps while maintaining backward compatibility and adding significant new capabilities for database observability.

The phased approach ensures that security and robustness concerns are addressed first, followed by intelligence features and operational polish. With proper execution of this plan, the result will be a best-in-class database monitoring solution that leverages OpenTelemetry standards while providing enterprise-grade features for New Relic customers.

## Appendices

### A. Configuration Templates
[Detailed configuration examples for various deployment scenarios]

### B. Query Library
[Complete set of optimized queries for all database types and feature sets]

### C. Testing Scenarios
[Comprehensive test cases covering all edge cases and failure modes]

### D. Troubleshooting Guide
[Common issues and resolution procedures]