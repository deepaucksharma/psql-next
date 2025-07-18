package scaling

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements Storage interface using Redis
type RedisStorage struct {
	client     *redis.Client
	keyPrefix  string
	leaderKey  string
	leaderTTL  time.Duration
}

// RedisConfig configures Redis storage
type RedisConfig struct {
	Address   string
	Password  string
	DB        int
	KeyPrefix string
	LeaderTTL time.Duration
}

// NewRedisStorage creates a new Redis storage
func NewRedisStorage(config *RedisConfig) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Address,
		Password: config.Password,
		DB:       config.DB,
	})
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	
	keyPrefix := config.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = "dbintel:scaling:"
	}
	
	leaderTTL := config.LeaderTTL
	if leaderTTL == 0 {
		leaderTTL = 30 * time.Second
	}
	
	return &RedisStorage{
		client:    client,
		keyPrefix: keyPrefix,
		leaderKey: keyPrefix + "leader",
		leaderTTL: leaderTTL,
	}, nil
}

// RegisterNode registers a node
func (rs *RedisStorage) RegisterNode(ctx context.Context, node *NodeInfo) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}
	
	key := rs.nodeKey(node.ID)
	ttl := 3 * time.Minute // Nodes must heartbeat within 3 minutes
	
	return rs.client.Set(ctx, key, data, ttl).Err()
}

// UpdateNodeHeartbeat updates a node's heartbeat
func (rs *RedisStorage) UpdateNodeHeartbeat(ctx context.Context, nodeID string) error {
	key := rs.nodeKey(nodeID)
	
	// Get current node data
	data, err := rs.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("node %s not found", nodeID)
		}
		return err
	}
	
	var node NodeInfo
	if err := json.Unmarshal(data, &node); err != nil {
		return fmt.Errorf("failed to unmarshal node info: %w", err)
	}
	
	// Update heartbeat
	node.LastHeartbeat = time.Now()
	
	// Save back
	updatedData, err := json.Marshal(&node)
	if err != nil {
		return fmt.Errorf("failed to marshal updated node info: %w", err)
	}
	
	ttl := 3 * time.Minute
	return rs.client.Set(ctx, key, updatedData, ttl).Err()
}

// GetActiveNodes returns all active nodes
func (rs *RedisStorage) GetActiveNodes(ctx context.Context) ([]*NodeInfo, error) {
	pattern := rs.keyPrefix + "nodes:*"
	
	// Scan for all node keys
	var cursor uint64
	var keys []string
	
	for {
		var batch []string
		var err error
		batch, cursor, err = rs.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		
		keys = append(keys, batch...)
		
		if cursor == 0 {
			break
		}
	}
	
	// Get all node data
	nodes := make([]*NodeInfo, 0, len(keys))
	for _, key := range keys {
		data, err := rs.client.Get(ctx, key).Bytes()
		if err != nil {
			if err == redis.Nil {
				continue // Node expired
			}
			return nil, err
		}
		
		var node NodeInfo
		if err := json.Unmarshal(data, &node); err != nil {
			continue // Skip invalid data
		}
		
		nodes = append(nodes, &node)
	}
	
	return nodes, nil
}

// RemoveNode removes a node
func (rs *RedisStorage) RemoveNode(ctx context.Context, nodeID string) error {
	key := rs.nodeKey(nodeID)
	return rs.client.Del(ctx, key).Err()
}

// GetAssignments returns current resource assignments
func (rs *RedisStorage) GetAssignments(ctx context.Context) (map[string]string, error) {
	key := rs.keyPrefix + "assignments"
	
	result, err := rs.client.HGetAll(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return make(map[string]string), nil
		}
		return nil, err
	}
	
	return result, nil
}

// UpdateAssignments updates resource assignments
func (rs *RedisStorage) UpdateAssignments(ctx context.Context, assignments map[string]string) error {
	key := rs.keyPrefix + "assignments"
	
	// Use transaction to update atomically
	pipe := rs.client.TxPipeline()
	
	// Clear existing assignments
	pipe.Del(ctx, key)
	
	// Set new assignments
	if len(assignments) > 0 {
		pipe.HSet(ctx, key, assignments)
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

// AcquireLeadership attempts to acquire leadership
func (rs *RedisStorage) AcquireLeadership(ctx context.Context, nodeID string) (bool, error) {
	// Try to set leader key with NX (only if not exists)
	ok, err := rs.client.SetNX(ctx, rs.leaderKey, nodeID, rs.leaderTTL).Result()
	if err != nil {
		return false, err
	}
	
	if ok {
		return true, nil
	}
	
	// Check if we already have leadership
	current, err := rs.client.Get(ctx, rs.leaderKey).Result()
	if err != nil {
		if err == redis.Nil {
			// Key expired between SetNX and Get, try again
			return rs.AcquireLeadership(ctx, nodeID)
		}
		return false, err
	}
	
	if current == nodeID {
		// Renew leadership
		return true, rs.RenewLeadership(ctx, nodeID)
	}
	
	return false, nil
}

// RenewLeadership renews leadership lease
func (rs *RedisStorage) RenewLeadership(ctx context.Context, nodeID string) error {
	// Use Lua script for atomic check-and-extend
	script := redis.NewScript(`
		local key = KEYS[1]
		local nodeID = ARGV[1]
		local ttl = ARGV[2]
		
		local current = redis.call('GET', key)
		if current == nodeID then
			redis.call('EXPIRE', key, ttl)
			return 1
		else
			return 0
		end
	`)
	
	result, err := script.Run(ctx, rs.client, []string{rs.leaderKey}, nodeID, int(rs.leaderTTL.Seconds())).Int()
	if err != nil {
		return err
	}
	
	if result == 0 {
		return fmt.Errorf("node %s is not the leader", nodeID)
	}
	
	return nil
}

// GetLeader returns the current leader
func (rs *RedisStorage) GetLeader(ctx context.Context) (string, error) {
	leader, err := rs.client.Get(ctx, rs.leaderKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	
	return leader, nil
}

// Close closes the Redis connection
func (rs *RedisStorage) Close() error {
	return rs.client.Close()
}

// nodeKey returns the Redis key for a node
func (rs *RedisStorage) nodeKey(nodeID string) string {
	return rs.keyPrefix + "nodes:" + nodeID
}