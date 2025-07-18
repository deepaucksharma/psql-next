//go:build e2e
// +build e2e

package mongodb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/database-intelligence/db-intel/tests/e2e/testutils"
)

func TestMongoDBE2E(t *testing.T) {
	// Skip if not in E2E mode
	testutils.SkipIfNotE2E(t)
	
	// Create test context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	
	// Setup test environment
	env := testutils.NewTestEnvironment(t)
	defer env.Cleanup()
	
	// Start MongoDB container
	mongoContainer, err := env.StartMongoDB(testutils.MongoDBConfig{
		Version:    "7.0",
		ReplicaSet: true,
		InitScript: "../../../../deployments/docker/init-scripts/mongodb-init.js",
	})
	require.NoError(t, err)
	defer mongoContainer.Stop()
	
	// Wait for MongoDB to be ready
	require.NoError(t, mongoContainer.WaitReady(ctx, 60*time.Second))
	
	// Get MongoDB endpoint
	endpoint := mongoContainer.Endpoint()
	t.Logf("MongoDB endpoint: %s", endpoint)
	
	// Create workload generator
	workload, err := NewWorkloadGenerator(endpoint, "monitoring", "monitoring_password")
	require.NoError(t, err)
	defer workload.Close()
	
	// Start collector with MongoDB configuration
	collectorConfig := fmt.Sprintf(`
receivers:
  mongodb:
    hosts:
      - endpoint: %s
    username: monitoring
    password: monitoring_password
    collection_interval: 5s
    databases:
      - admin
      - testdb
    metrics:
      mongodb.database.size: true
      mongodb.collection.size: true
      mongodb.collection.count: true
      mongodb.operation.count: true
      mongodb.connection.count: true

processors:
  batch:
    timeout: 5s
    send_batch_size: 100

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: mongodb_test
  
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [mongodb]
      processors: [batch]
      exporters: [prometheus, logging]
`, endpoint)
	
	collector, err := env.StartCollectorWithConfig(collectorConfig)
	require.NoError(t, err)
	defer collector.Stop()
	
	// Wait for collector to be ready
	require.NoError(t, collector.WaitReady(ctx, 30*time.Second))
	
	// Create verifier
	verifier := NewVerifier(collector.PrometheusEndpoint())
	
	// Run test scenarios
	t.Run("BasicMetrics", func(t *testing.T) {
		testBasicMetrics(t, ctx, workload, verifier)
	})
	
	t.Run("CollectionMetrics", func(t *testing.T) {
		testCollectionMetrics(t, ctx, workload, verifier)
	})
	
	t.Run("OperationMetrics", func(t *testing.T) {
		testOperationMetrics(t, ctx, workload, verifier)
	})
	
	t.Run("ReplicationMetrics", func(t *testing.T) {
		if mongoContainer.IsReplicaSet() {
			testReplicationMetrics(t, ctx, workload, verifier)
		} else {
			t.Skip("Replica set not enabled")
		}
	})
	
	t.Run("PerformanceMetrics", func(t *testing.T) {
		testPerformanceMetrics(t, ctx, workload, verifier)
	})
}

func testBasicMetrics(t *testing.T, ctx context.Context, workload *WorkloadGenerator, verifier *Verifier) {
	// Generate basic workload
	err := workload.GenerateBasicOperations(ctx, 100)
	require.NoError(t, err)
	
	// Wait for metrics collection
	time.Sleep(10 * time.Second)
	
	// Verify database size metrics
	metrics, err := verifier.GetMetrics("mongodb_test_database_size")
	require.NoError(t, err)
	assert.NotEmpty(t, metrics)
	
	// Check we have metrics for expected databases
	databases := make(map[string]bool)
	for _, metric := range metrics {
		if db, ok := metric.Labels["database"]; ok {
			databases[db] = true
		}
	}
	
	assert.True(t, databases["admin"], "Missing metrics for admin database")
	assert.True(t, databases["testdb"], "Missing metrics for testdb database")
	
	// Verify connection metrics
	connMetrics, err := verifier.GetMetrics("mongodb_test_connection_count")
	require.NoError(t, err)
	assert.NotEmpty(t, connMetrics)
	
	// Connection count should be reasonable
	for _, metric := range connMetrics {
		assert.Greater(t, metric.Value, float64(0), "Connection count should be positive")
		assert.Less(t, metric.Value, float64(100), "Connection count seems too high")
	}
}

func testCollectionMetrics(t *testing.T, ctx context.Context, workload *WorkloadGenerator, verifier *Verifier) {
	// Create collections with known data
	collections := []string{"users", "orders", "products"}
	for _, coll := range collections {
		err := workload.CreateCollection(ctx, coll, 1000)
		require.NoError(t, err)
	}
	
	// Wait for metrics
	time.Sleep(10 * time.Second)
	
	// Verify collection count metrics
	countMetrics, err := verifier.GetMetrics("mongodb_test_collection_count")
	require.NoError(t, err)
	assert.NotEmpty(t, countMetrics)
	
	// Check each collection has metrics
	for _, coll := range collections {
		found := false
		for _, metric := range countMetrics {
			if metric.Labels["collection"] == coll {
				found = true
				assert.Equal(t, float64(1000), metric.Value, "Collection %s should have 1000 documents", coll)
				break
			}
		}
		assert.True(t, found, "Missing metrics for collection %s", coll)
	}
	
	// Verify collection size metrics
	sizeMetrics, err := verifier.GetMetrics("mongodb_test_collection_size")
	require.NoError(t, err)
	assert.NotEmpty(t, sizeMetrics)
	
	// Each collection should have some size
	for _, metric := range sizeMetrics {
		assert.Greater(t, metric.Value, float64(0), "Collection size should be positive")
	}
}

func testOperationMetrics(t *testing.T, ctx context.Context, workload *WorkloadGenerator, verifier *Verifier) {
	// Record baseline operation counts
	baselineOps, err := verifier.GetOperationCounts()
	require.NoError(t, err)
	
	// Generate specific operations
	operations := map[string]int{
		"insert": 500,
		"find":   1000,
		"update": 300,
		"delete": 100,
	}
	
	for opType, count := range operations {
		err := workload.GenerateOperations(ctx, opType, count)
		require.NoError(t, err)
	}
	
	// Wait for metrics
	time.Sleep(10 * time.Second)
	
	// Get new operation counts
	newOps, err := verifier.GetOperationCounts()
	require.NoError(t, err)
	
	// Verify operation counts increased appropriately
	for opType, expectedCount := range operations {
		baseline := baselineOps[opType]
		current := newOps[opType]
		increase := current - baseline
		
		// Allow some variance due to timing
		assert.Greater(t, increase, float64(expectedCount)*0.8, 
			"Operation %s count didn't increase enough", opType)
		assert.Less(t, increase, float64(expectedCount)*1.2,
			"Operation %s count increased too much", opType)
	}
}

func testReplicationMetrics(t *testing.T, ctx context.Context, workload *WorkloadGenerator, verifier *Verifier) {
	// Generate replication workload
	err := workload.GenerateReplicationLoad(ctx, 1000)
	require.NoError(t, err)
	
	// Wait for replication
	time.Sleep(15 * time.Second)
	
	// Verify replication lag metrics
	lagMetrics, err := verifier.GetMetrics("mongodb_test_replication_lag_seconds")
	require.NoError(t, err)
	assert.NotEmpty(t, lagMetrics)
	
	// Lag should be reasonable
	for _, metric := range lagMetrics {
		assert.Less(t, metric.Value, float64(5), "Replication lag should be less than 5 seconds")
		assert.GreaterOrEqual(t, metric.Value, float64(0), "Replication lag should be non-negative")
	}
	
	// Verify replica set member metrics
	memberMetrics, err := verifier.GetMetrics("mongodb_test_replset_member_state")
	require.NoError(t, err)
	assert.NotEmpty(t, memberMetrics)
	
	// Should have at least one primary
	hasPrimary := false
	for _, metric := range memberMetrics {
		if metric.Labels["state"] == "PRIMARY" {
			hasPrimary = true
			assert.Equal(t, float64(1), metric.Value, "Primary state should be 1")
		}
	}
	assert.True(t, hasPrimary, "No primary found in replica set")
}

func testPerformanceMetrics(t *testing.T, ctx context.Context, workload *WorkloadGenerator, verifier *Verifier) {
	// Generate heavy workload
	err := workload.GenerateHeavyLoad(ctx, WorkloadConfig{
		Duration:          30 * time.Second,
		ConcurrentClients: 10,
		OperationsPerSec:  100,
	})
	require.NoError(t, err)
	
	// Wait for metrics
	time.Sleep(10 * time.Second)
	
	// Verify operation latency metrics
	latencyMetrics, err := verifier.GetLatencyPercentiles()
	require.NoError(t, err)
	
	// Check latency percentiles
	assert.Contains(t, latencyMetrics, "p50")
	assert.Contains(t, latencyMetrics, "p95")
	assert.Contains(t, latencyMetrics, "p99")
	
	// Latencies should be reasonable
	assert.Less(t, latencyMetrics["p50"], 10.0, "p50 latency should be less than 10ms")
	assert.Less(t, latencyMetrics["p95"], 50.0, "p95 latency should be less than 50ms")
	assert.Less(t, latencyMetrics["p99"], 100.0, "p99 latency should be less than 100ms")
	
	// Verify throughput metrics
	throughput, err := verifier.GetThroughput()
	require.NoError(t, err)
	
	// Should have reasonable throughput
	assert.Greater(t, throughput, 50.0, "Throughput should be at least 50 ops/sec")
	assert.Less(t, throughput, 1000.0, "Throughput seems unrealistically high")
}