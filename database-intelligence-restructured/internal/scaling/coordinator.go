package scaling

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Coordinator manages distributed coordination for horizontal scaling
type Coordinator struct {
	mu               sync.RWMutex
	nodeID           string
	logger           *zap.Logger
	storage          Storage
	heartbeatTicker  *time.Ticker
	shutdownCh       chan struct{}
	wg               sync.WaitGroup
	
	// Current state
	nodes            map[string]*NodeInfo
	assignments      map[string]string // resource -> nodeID
	lastRebalance    time.Time
	
	// Configuration
	config           *CoordinatorConfig
	
	// Metrics
	metricsCollector *MetricsCollector
}

// CoordinatorConfig configures the coordinator
type CoordinatorConfig struct {
	NodeID                string
	HeartbeatInterval     time.Duration
	NodeTimeout           time.Duration
	RebalanceInterval     time.Duration
	MinRebalanceInterval  time.Duration
}

// NodeInfo represents information about a collector node
type NodeInfo struct {
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	StartTime    time.Time         `json:"start_time"`
	LastHeartbeat time.Time        `json:"last_heartbeat"`
	Capabilities []string          `json:"capabilities"`
	Resources    []string          `json:"resources"`
	Load         float64           `json:"load"`
	Metadata     map[string]string `json:"metadata"`
}

// Storage interface for distributed state storage
type Storage interface {
	// Node operations
	RegisterNode(ctx context.Context, node *NodeInfo) error
	UpdateNodeHeartbeat(ctx context.Context, nodeID string) error
	GetActiveNodes(ctx context.Context) ([]*NodeInfo, error)
	RemoveNode(ctx context.Context, nodeID string) error
	
	// Assignment operations
	GetAssignments(ctx context.Context) (map[string]string, error)
	UpdateAssignments(ctx context.Context, assignments map[string]string) error
	
	// Leader election
	AcquireLeadership(ctx context.Context, nodeID string) (bool, error)
	RenewLeadership(ctx context.Context, nodeID string) error
	GetLeader(ctx context.Context) (string, error)
}

// NewCoordinator creates a new coordinator
func NewCoordinator(config *CoordinatorConfig, storage Storage, logger *zap.Logger) *Coordinator {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}
	if config.NodeTimeout == 0 {
		config.NodeTimeout = 3 * config.HeartbeatInterval
	}
	if config.RebalanceInterval == 0 {
		config.RebalanceInterval = 5 * time.Minute
	}
	if config.MinRebalanceInterval == 0 {
		config.MinRebalanceInterval = 1 * time.Minute
	}
	
	c := &Coordinator{
		nodeID:      config.NodeID,
		logger:      logger,
		storage:     storage,
		shutdownCh:  make(chan struct{}),
		nodes:       make(map[string]*NodeInfo),
		assignments: make(map[string]string),
		config:      config,
	}
	
	// Create metrics collector
	c.metricsCollector = NewMetricsCollector(c, logger)
	
	return c
}

// Start begins coordination
func (c *Coordinator) Start(ctx context.Context) error {
	// Register this node
	node := &NodeInfo{
		ID:           c.nodeID,
		Hostname:     getHostname(),
		StartTime:    time.Now(),
		LastHeartbeat: time.Now(),
		Capabilities: c.getCapabilities(),
		Metadata:     c.getMetadata(),
	}
	
	if err := c.storage.RegisterNode(ctx, node); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}
	
	// Start heartbeat
	c.heartbeatTicker = time.NewTicker(c.config.HeartbeatInterval)
	c.wg.Add(1)
	go c.heartbeatLoop()
	
	// Start coordination loop
	c.wg.Add(1)
	go c.coordinationLoop()
	
	c.logger.Info("Coordinator started",
		zap.String("node_id", c.nodeID),
		zap.Duration("heartbeat_interval", c.config.HeartbeatInterval))
	
	return nil
}

// Stop stops the coordinator
func (c *Coordinator) Stop(ctx context.Context) error {
	close(c.shutdownCh)
	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}
	c.wg.Wait()
	
	// Remove node registration
	if err := c.storage.RemoveNode(ctx, c.nodeID); err != nil {
		c.logger.Warn("Failed to remove node registration", zap.Error(err))
	}
	
	return nil
}

// GetAssignment returns the node assignment for a resource
func (c *Coordinator) GetAssignment(resource string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	nodeID, exists := c.assignments[resource]
	return nodeID, exists
}

// IsAssignedToMe checks if a resource is assigned to this node
func (c *Coordinator) IsAssignedToMe(resource string) bool {
	nodeID, exists := c.GetAssignment(resource)
	return exists && nodeID == c.nodeID
}

// RegisterResource registers a resource for assignment
func (c *Coordinator) RegisterResource(resource string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if already assigned
	if _, exists := c.assignments[resource]; exists {
		return nil
	}
	
	// Trigger rebalance
	go c.triggerRebalance()
	
	return nil
}

// heartbeatLoop sends periodic heartbeats
func (c *Coordinator) heartbeatLoop() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.heartbeatTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := c.storage.UpdateNodeHeartbeat(ctx, c.nodeID); err != nil {
				c.logger.Error("Failed to update heartbeat", zap.Error(err))
				c.metricsCollector.IncrementHeartbeatError()
			}
			cancel()
			
		case <-c.shutdownCh:
			return
		}
	}
}

// coordinationLoop manages distributed coordination
func (c *Coordinator) coordinationLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(c.config.RebalanceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.performCoordination()
			
		case <-c.shutdownCh:
			return
		}
	}
}

// performCoordination performs coordination tasks
func (c *Coordinator) performCoordination() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Try to acquire leadership
	isLeader, err := c.storage.AcquireLeadership(ctx, c.nodeID)
	if err != nil {
		c.logger.Error("Failed to acquire leadership", zap.Error(err))
		c.metricsCollector.IncrementCoordinationError()
		return
	}
	
	if !isLeader {
		// Not the leader, just update local state
		c.updateLocalState(ctx)
		c.metricsCollector.UpdateState(int64(len(c.nodes)), int64(len(c.assignments)), false)
		return
	}
	
	// We are the leader, perform coordination tasks
	c.logger.Debug("Performing coordination as leader")
	
	// Get active nodes
	nodes, err := c.storage.GetActiveNodes(ctx)
	if err != nil {
		c.logger.Error("Failed to get active nodes", zap.Error(err))
		return
	}
	
	// Remove stale nodes
	activeNodes := c.removeStaleNodes(nodes)
	
	// Get current assignments
	assignments, err := c.storage.GetAssignments(ctx)
	if err != nil {
		c.logger.Error("Failed to get assignments", zap.Error(err))
		return
	}
	
	// Check if rebalance is needed
	if c.shouldRebalance(activeNodes, assignments) {
		newAssignments := c.rebalance(activeNodes, assignments)
		
		// Update assignments
		if err := c.storage.UpdateAssignments(ctx, newAssignments); err != nil {
			c.logger.Error("Failed to update assignments", zap.Error(err))
			return
		}
		
		c.lastRebalance = time.Now()
		c.metricsCollector.IncrementRebalance()
		c.logger.Info("Rebalanced assignments",
			zap.Int("nodes", len(activeNodes)),
			zap.Int("resources", len(newAssignments)))
	}
	
	// Update local state
	c.updateLocalStateFromNodes(activeNodes, assignments)
	
	// Update metrics
	c.metricsCollector.UpdateState(int64(len(activeNodes)), int64(len(assignments)), true)
}

// updateLocalState updates local state from storage
func (c *Coordinator) updateLocalState(ctx context.Context) {
	nodes, err := c.storage.GetActiveNodes(ctx)
	if err != nil {
		c.logger.Error("Failed to get active nodes", zap.Error(err))
		return
	}
	
	assignments, err := c.storage.GetAssignments(ctx)
	if err != nil {
		c.logger.Error("Failed to get assignments", zap.Error(err))
		return
	}
	
	c.updateLocalStateFromNodes(nodes, assignments)
}

// updateLocalStateFromNodes updates local state
func (c *Coordinator) updateLocalStateFromNodes(nodes []*NodeInfo, assignments map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Update nodes
	c.nodes = make(map[string]*NodeInfo)
	for _, node := range nodes {
		c.nodes[node.ID] = node
	}
	
	// Update assignments
	c.assignments = assignments
}

// removeStaleNodes removes nodes that haven't sent heartbeats
func (c *Coordinator) removeStaleNodes(nodes []*NodeInfo) []*NodeInfo {
	active := make([]*NodeInfo, 0, len(nodes))
	staleThreshold := time.Now().Add(-c.config.NodeTimeout)
	
	for _, node := range nodes {
		if node.LastHeartbeat.After(staleThreshold) {
			active = append(active, node)
		} else {
			c.logger.Warn("Removing stale node",
				zap.String("node_id", node.ID),
				zap.Time("last_heartbeat", node.LastHeartbeat))
			
			// Remove from storage
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			c.storage.RemoveNode(ctx, node.ID)
			cancel()
		}
	}
	
	return active
}

// shouldRebalance determines if rebalancing is needed
func (c *Coordinator) shouldRebalance(nodes []*NodeInfo, assignments map[string]string) bool {
	// Don't rebalance too frequently
	if time.Since(c.lastRebalance) < c.config.MinRebalanceInterval {
		return false
	}
	
	// Check for unassigned resources
	for _, node := range nodes {
		for _, resource := range node.Resources {
			if _, assigned := assignments[resource]; !assigned {
				return true
			}
		}
	}
	
	// Check for assignments to dead nodes
	nodeMap := make(map[string]bool)
	for _, node := range nodes {
		nodeMap[node.ID] = true
	}
	
	for _, nodeID := range assignments {
		if !nodeMap[nodeID] {
			return true
		}
	}
	
	// Check for load imbalance
	if c.isLoadImbalanced(nodes, assignments) {
		return true
	}
	
	return false
}

// isLoadImbalanced checks if load is imbalanced across nodes
func (c *Coordinator) isLoadImbalanced(nodes []*NodeInfo, assignments map[string]string) bool {
	if len(nodes) < 2 {
		return false
	}
	
	// Count assignments per node
	counts := make(map[string]int)
	for _, node := range nodes {
		counts[node.ID] = 0
	}
	
	for _, nodeID := range assignments {
		counts[nodeID]++
	}
	
	// Find min and max
	min, max := len(assignments), 0
	for _, count := range counts {
		if count < min {
			min = count
		}
		if count > max {
			max = count
		}
	}
	
	// Consider imbalanced if difference is more than 1
	return max-min > 1
}

// rebalance performs resource rebalancing
func (c *Coordinator) rebalance(nodes []*NodeInfo, currentAssignments map[string]string) map[string]string {
	if len(nodes) == 0 {
		return make(map[string]string)
	}
	
	// Collect all resources
	allResources := make(map[string]bool)
	for _, node := range nodes {
		for _, resource := range node.Resources {
			allResources[resource] = true
		}
	}
	
	// Add currently assigned resources
	for resource := range currentAssignments {
		allResources[resource] = true
	}
	
	// Convert to slice for consistent ordering
	resources := make([]string, 0, len(allResources))
	for resource := range allResources {
		resources = append(resources, resource)
	}
	
	// Use consistent hashing for stable assignment
	assignments := make(map[string]string)
	for _, resource := range resources {
		nodeID := c.selectNode(resource, nodes)
		assignments[resource] = nodeID
	}
	
	return assignments
}

// selectNode selects a node for a resource using consistent hashing
func (c *Coordinator) selectNode(resource string, nodes []*NodeInfo) string {
	if len(nodes) == 0 {
		return ""
	}
	
	// Create hash of resource
	h := sha256.New()
	h.Write([]byte(resource))
	hash := h.Sum(nil)
	
	// Convert to number
	hashNum := uint64(0)
	for i := 0; i < 8; i++ {
		hashNum = (hashNum << 8) | uint64(hash[i])
	}
	
	// Select node based on hash
	index := hashNum % uint64(len(nodes))
	return nodes[index].ID
}

// triggerRebalance triggers an immediate rebalance
func (c *Coordinator) triggerRebalance() {
	// Perform coordination immediately
	c.performCoordination()
}

// getCapabilities returns node capabilities
func (c *Coordinator) getCapabilities() []string {
	return []string{
		"metrics",
		"logs",
		"traces",
		"postgresql",
		"mysql",
		"mongodb",
		"redis",
	}
}

// getMetadata returns node metadata
func (c *Coordinator) getMetadata() map[string]string {
	return map[string]string{
		"version": "1.0.0",
		"type":    "database-intelligence",
	}
}

// getHostname returns the hostname
func getHostname() string {
	// This would get the actual hostname
	return "localhost"
}

// GetMetricsCollector returns the metrics collector
func (c *Coordinator) GetMetricsCollector() *MetricsCollector {
	return c.metricsCollector
}