package scaling

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// MetricsCollector collects metrics for the scaling system
type MetricsCollector struct {
	mu          sync.RWMutex
	coordinator *Coordinator
	logger      *zap.Logger
	
	// Metrics data
	nodesTotal         int64
	assignmentsTotal   int64
	isLeader           bool
	rebalanceCount     int64
	heartbeatErrors    int64
	coordinationErrors int64
	lastCollection     time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(coordinator *Coordinator, logger *zap.Logger) *MetricsCollector {
	return &MetricsCollector{
		coordinator:    coordinator,
		logger:        logger,
		lastCollection: time.Now(),
	}
}

// CollectMetrics collects scaling metrics
func (mc *MetricsCollector) CollectMetrics(ctx context.Context) pmetric.Metrics {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Create metrics
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := resourceMetrics.Resource()
	resource.Attributes().PutStr("service.name", "database-intelligence")
	resource.Attributes().PutStr("scaling.node_id", mc.coordinator.nodeID)
	
	// Create scope metrics
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("github.com/database-intelligence/db-intel/scaling")
	scopeMetrics.Scope().SetVersion("1.0.0")
	
	// Add metrics
	mc.addNodesMetric(scopeMetrics)
	mc.addAssignmentsMetric(scopeMetrics)
	mc.addLeaderMetric(scopeMetrics)
	mc.addRebalanceMetric(scopeMetrics)
	mc.addErrorMetrics(scopeMetrics)
	mc.addCoordinatorStateMetrics(scopeMetrics)
	
	mc.lastCollection = time.Now()
	
	return metrics
}

// UpdateState updates the metrics state
func (mc *MetricsCollector) UpdateState(nodes int64, assignments int64, isLeader bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.nodesTotal = nodes
	mc.assignmentsTotal = assignments
	mc.isLeader = isLeader
}

// IncrementRebalance increments the rebalance counter
func (mc *MetricsCollector) IncrementRebalance() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.rebalanceCount++
}

// IncrementHeartbeatError increments heartbeat error counter
func (mc *MetricsCollector) IncrementHeartbeatError() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.heartbeatErrors++
}

// IncrementCoordinationError increments coordination error counter
func (mc *MetricsCollector) IncrementCoordinationError() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.coordinationErrors++
}

// addNodesMetric adds the active nodes metric
func (mc *MetricsCollector) addNodesMetric(sm pmetric.ScopeMetrics) {
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dbintel.scaling.nodes.total")
	metric.SetDescription("Total number of active collector nodes")
	metric.SetUnit("1")
	
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(mc.nodesTotal)
}

// addAssignmentsMetric adds the assignments metric
func (mc *MetricsCollector) addAssignmentsMetric(sm pmetric.ScopeMetrics) {
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dbintel.scaling.assignments.total")
	metric.SetDescription("Total number of resource assignments")
	metric.SetUnit("1")
	
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(mc.assignmentsTotal)
}

// addLeaderMetric adds the leader status metric
func (mc *MetricsCollector) addLeaderMetric(sm pmetric.ScopeMetrics) {
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dbintel.scaling.leader")
	metric.SetDescription("Whether this node is the leader (1) or not (0)")
	metric.SetUnit("1")
	
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	if mc.isLeader {
		dp.SetIntValue(1)
	} else {
		dp.SetIntValue(0)
	}
}

// addRebalanceMetric adds the rebalance counter metric
func (mc *MetricsCollector) addRebalanceMetric(sm pmetric.ScopeMetrics) {
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dbintel.scaling.rebalance.total")
	metric.SetDescription("Total number of rebalance operations")
	metric.SetUnit("1")
	
	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(mc.rebalanceCount)
}

// addErrorMetrics adds error metrics
func (mc *MetricsCollector) addErrorMetrics(sm pmetric.ScopeMetrics) {
	// Heartbeat errors
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("dbintel.scaling.heartbeat.errors.total")
	metric.SetDescription("Total number of heartbeat errors")
	metric.SetUnit("1")
	
	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(mc.heartbeatErrors)
	
	// Coordination errors
	metric2 := sm.Metrics().AppendEmpty()
	metric2.SetName("dbintel.scaling.coordination.errors.total")
	metric2.SetDescription("Total number of coordination errors")
	metric2.SetUnit("1")
	
	sum2 := metric2.SetEmptySum()
	sum2.SetIsMonotonic(true)
	sum2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp2 := sum2.DataPoints().AppendEmpty()
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp2.SetIntValue(mc.coordinationErrors)
}

// addCoordinatorStateMetrics adds detailed coordinator state metrics
func (mc *MetricsCollector) addCoordinatorStateMetrics(sm pmetric.ScopeMetrics) {
	// Get current state
	mc.coordinator.mu.RLock()
	nodes := mc.coordinator.nodes
	assignments := mc.coordinator.assignments
	lastRebalance := mc.coordinator.lastRebalance
	mc.coordinator.mu.RUnlock()
	
	// Node load distribution
	if len(nodes) > 0 {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("dbintel.scaling.node.load")
		metric.SetDescription("Load distribution across nodes")
		metric.SetUnit("1")
		
		gauge := metric.SetEmptyGauge()
		
		// Count assignments per node
		assignmentCounts := make(map[string]int)
		for _, nodeID := range assignments {
			assignmentCounts[nodeID]++
		}
		
		// Create data points for each node
		for nodeID, node := range nodes {
			dp := gauge.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			dp.SetIntValue(int64(assignmentCounts[nodeID]))
			dp.Attributes().PutStr("node_id", nodeID)
			dp.Attributes().PutStr("hostname", node.Hostname)
		}
	}
	
	// Time since last rebalance
	if !lastRebalance.IsZero() {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("dbintel.scaling.rebalance.age.seconds")
		metric.SetDescription("Time since last rebalance in seconds")
		metric.SetUnit("s")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(time.Since(lastRebalance).Seconds())
	}
	
	// Assignment to node ratio
	if len(nodes) > 0 {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("dbintel.scaling.assignment.ratio")
		metric.SetDescription("Average number of assignments per node")
		metric.SetUnit("1")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(float64(len(assignments)) / float64(len(nodes)))
	}
}

// CollectorMetrics represents metrics from a metrics collector
type CollectorMetrics struct {
	Timestamp          time.Time
	NodesTotal         int64
	AssignmentsTotal   int64
	IsLeader           bool
	RebalanceCount     int64
	HeartbeatErrors    int64
	CoordinationErrors int64
	NodeLoadMap        map[string]int64
	TimeSinceRebalance float64
	AssignmentRatio    float64
}

// GetSnapshot returns a snapshot of current metrics
func (mc *MetricsCollector) GetSnapshot() CollectorMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	mc.coordinator.mu.RLock()
	nodes := mc.coordinator.nodes
	assignments := mc.coordinator.assignments
	lastRebalance := mc.coordinator.lastRebalance
	mc.coordinator.mu.RUnlock()
	
	// Calculate node load
	nodeLoad := make(map[string]int64)
	for _, nodeID := range assignments {
		nodeLoad[nodeID]++
	}
	
	// Calculate metrics
	snapshot := CollectorMetrics{
		Timestamp:          time.Now(),
		NodesTotal:         mc.nodesTotal,
		AssignmentsTotal:   mc.assignmentsTotal,
		IsLeader:           mc.isLeader,
		RebalanceCount:     mc.rebalanceCount,
		HeartbeatErrors:    mc.heartbeatErrors,
		CoordinationErrors: mc.coordinationErrors,
		NodeLoadMap:        nodeLoad,
	}
	
	if !lastRebalance.IsZero() {
		snapshot.TimeSinceRebalance = time.Since(lastRebalance).Seconds()
	}
	
	if len(nodes) > 0 {
		snapshot.AssignmentRatio = float64(len(assignments)) / float64(len(nodes))
	}
	
	return snapshot
}