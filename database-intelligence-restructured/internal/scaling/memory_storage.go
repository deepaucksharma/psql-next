package scaling

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryStorage implements Storage interface using in-memory storage
// This is suitable for single-node deployments or testing
type MemoryStorage struct {
	mu           sync.RWMutex
	nodes        map[string]*NodeInfo
	assignments  map[string]string
	leader       string
	leaderExpiry time.Time
}

// NewMemoryStorage creates a new memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		nodes:       make(map[string]*NodeInfo),
		assignments: make(map[string]string),
	}
}

// RegisterNode registers a node
func (ms *MemoryStorage) RegisterNode(ctx context.Context, node *NodeInfo) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	ms.nodes[node.ID] = node
	return nil
}

// UpdateNodeHeartbeat updates a node's heartbeat
func (ms *MemoryStorage) UpdateNodeHeartbeat(ctx context.Context, nodeID string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	node, exists := ms.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}
	
	node.LastHeartbeat = time.Now()
	return nil
}

// GetActiveNodes returns all active nodes
func (ms *MemoryStorage) GetActiveNodes(ctx context.Context) ([]*NodeInfo, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	nodes := make([]*NodeInfo, 0, len(ms.nodes))
	for _, node := range ms.nodes {
		// Create a copy to avoid race conditions
		nodeCopy := *node
		nodes = append(nodes, &nodeCopy)
	}
	
	return nodes, nil
}

// RemoveNode removes a node
func (ms *MemoryStorage) RemoveNode(ctx context.Context, nodeID string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	delete(ms.nodes, nodeID)
	
	// If this was the leader, clear leadership
	if ms.leader == nodeID {
		ms.leader = ""
		ms.leaderExpiry = time.Time{}
	}
	
	return nil
}

// GetAssignments returns current resource assignments
func (ms *MemoryStorage) GetAssignments(ctx context.Context) (map[string]string, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	assignments := make(map[string]string)
	for k, v := range ms.assignments {
		assignments[k] = v
	}
	
	return assignments, nil
}

// UpdateAssignments updates resource assignments
func (ms *MemoryStorage) UpdateAssignments(ctx context.Context, assignments map[string]string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	ms.assignments = make(map[string]string)
	for k, v := range assignments {
		ms.assignments[k] = v
	}
	
	return nil
}

// AcquireLeadership attempts to acquire leadership
func (ms *MemoryStorage) AcquireLeadership(ctx context.Context, nodeID string) (bool, error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	now := time.Now()
	
	// Check if leadership is available
	if ms.leader == "" || ms.leaderExpiry.Before(now) {
		ms.leader = nodeID
		ms.leaderExpiry = now.Add(30 * time.Second)
		return true, nil
	}
	
	// Check if we already have leadership
	if ms.leader == nodeID {
		ms.leaderExpiry = now.Add(30 * time.Second)
		return true, nil
	}
	
	return false, nil
}

// RenewLeadership renews leadership lease
func (ms *MemoryStorage) RenewLeadership(ctx context.Context, nodeID string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	if ms.leader != nodeID {
		return fmt.Errorf("node %s is not the leader", nodeID)
	}
	
	ms.leaderExpiry = time.Now().Add(30 * time.Second)
	return nil
}

// GetLeader returns the current leader
func (ms *MemoryStorage) GetLeader(ctx context.Context) (string, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	if ms.leaderExpiry.Before(time.Now()) {
		return "", nil
	}
	
	return ms.leader, nil
}