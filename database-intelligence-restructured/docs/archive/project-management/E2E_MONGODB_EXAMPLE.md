# MongoDB E2E Test Implementation Example

This document provides a concrete example of how to implement E2E tests for MongoDB, which can serve as a template for other databases.

## Directory Structure

```
tests/e2e/databases/mongodb/
├── mongodb_test.go          # Main test file
├── workload.go             # Workload generator
├── verifier.go             # Metric verification
├── test_data.go            # Test data definitions
└── config_test.yaml        # Test configuration
```

## 1. MongoDB Test Configuration

### config_test.yaml
```yaml
receivers:
  mongodb:
    endpoint: ${env:MONGODB_ENDPOINT}
    collection_interval: 5s
    username: ${env:MONGODB_USERNAME}
    password: ${env:MONGODB_PASSWORD}
    databases:
      - admin
      - testdb
    tls:
      insecure_skip_verify: true
    metrics:
      mongodb.database.size: true
      mongodb.collection.size: true
      mongodb.collection.count: true
      mongodb.operation.latency: true
      mongodb.operation.count: true
      mongodb.connections.current: true
      mongodb.replication.lag: true
      mongodb.cache.operations: true

processors:
  batch:
    timeout: 5s
    send_batch_size: 100
  
  resource:
    attributes:
      - key: service.name
        value: mongodb-test
        action: insert
      - key: db.system
        value: mongodb
        action: insert

exporters:
  file:
    path: /tmp/mongodb-metrics.json
  
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: mongodb_test

service:
  pipelines:
    metrics:
      receivers: [mongodb]
      processors: [batch, resource]
      exporters: [file, prometheus]
```

## 2. Main Test File

### mongodb_test.go
```go
package mongodb

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.mongodb.org/mongo-driver/mongo"
    
    "github.com/deepaksharma/db-otel/tests/e2e/framework"
)

func TestMongoDBE2E(t *testing.T) {
    // Skip if not in E2E mode
    framework.SkipIfNotE2E(t)
    
    // Setup test environment
    ctx := context.Background()
    env := framework.NewTestEnvironment(t)
    defer env.Cleanup()
    
    // Start MongoDB container
    mongoContainer := env.StartMongoDB(framework.MongoDBConfig{
        Version:     "7.0",
        ReplicaSet:  true,
        InitScript:  "init-mongodb-e2e.js",
    })
    
    // Start collector
    collector := env.StartCollector(framework.CollectorConfig{
        ConfigPath: "config_test.yaml",
        Env: map[string]string{
            "MONGODB_ENDPOINT": mongoContainer.Endpoint(),
            "MONGODB_USERNAME": "monitoring",
            "MONGODB_PASSWORD": "monitoring_password",
        },
    })
    
    // Wait for services to be ready
    require.NoError(t, mongoContainer.WaitReady(ctx, 30*time.Second))
    require.NoError(t, collector.WaitReady(ctx, 30*time.Second))
    
    // Create workload generator
    workload := NewWorkloadGenerator(mongoContainer.Client())
    
    // Run test scenarios
    t.Run("BasicMetrics", func(t *testing.T) {
        testBasicMetrics(t, env, workload)
    })
    
    t.Run("ReplicationMetrics", func(t *testing.T) {
        testReplicationMetrics(t, env, workload)
    })
    
    t.Run("PerformanceMetrics", func(t *testing.T) {
        testPerformanceMetrics(t, env, workload)
    })
    
    t.Run("SlowQueryDetection", func(t *testing.T) {
        testSlowQueryDetection(t, env, workload)
    })
}

func testBasicMetrics(t *testing.T, env *framework.TestEnvironment, workload *WorkloadGenerator) {
    // Generate basic workload
    ctx := context.Background()
    workload.GenerateBasicOperations(ctx, 100)
    
    // Wait for metrics collection
    time.Sleep(10 * time.Second)
    
    // Verify metrics
    verifier := NewMongoDBVerifier(env.PrometheusClient())
    
    metrics, err := verifier.GetMetrics("mongodb_database_size")
    require.NoError(t, err)
    assert.NotEmpty(t, metrics)
    
    // Verify database size metric
    for _, metric := range metrics {
        assert.Contains(t, metric.Labels, "database")
        assert.Greater(t, metric.Value, float64(0))
    }
    
    // Verify connection metrics
    connMetrics, err := verifier.GetMetrics("mongodb_connections_current")
    require.NoError(t, err)
    assert.NotEmpty(t, connMetrics)
}

func testReplicationMetrics(t *testing.T, env *framework.TestEnvironment, workload *WorkloadGenerator) {
    // Generate replication workload
    ctx := context.Background()
    workload.GenerateReplicationLoad(ctx)
    
    // Wait for metrics
    time.Sleep(15 * time.Second)
    
    // Verify replication lag
    verifier := NewMongoDBVerifier(env.PrometheusClient())
    lagMetrics, err := verifier.GetMetrics("mongodb_replication_lag_seconds")
    require.NoError(t, err)
    
    // Should have metrics for each secondary
    assert.GreaterOrEqual(t, len(lagMetrics), 2)
    
    // Lag should be reasonable
    for _, metric := range lagMetrics {
        assert.Less(t, metric.Value, float64(5)) // Less than 5 seconds
    }
}

func testPerformanceMetrics(t *testing.T, env *framework.TestEnvironment, workload *WorkloadGenerator) {
    // Baseline performance
    verifier := NewMongoDBVerifier(env.PrometheusClient())
    baseline, err := verifier.GetOperationLatencies()
    require.NoError(t, err)
    
    // Generate heavy workload
    ctx := context.Background()
    workload.GenerateHeavyLoad(ctx, 1000)
    
    // Wait and collect metrics
    time.Sleep(20 * time.Second)
    
    // Verify latency metrics exist
    latencies, err := verifier.GetOperationLatencies()
    require.NoError(t, err)
    
    // Check latency percentiles
    assert.Contains(t, latencies, "p50")
    assert.Contains(t, latencies, "p95")
    assert.Contains(t, latencies, "p99")
    
    // Verify reasonable latencies
    assert.Less(t, latencies["p50"], float64(10)) // p50 < 10ms
    assert.Less(t, latencies["p95"], float64(50)) // p95 < 50ms
}
```

## 3. Workload Generator

### workload.go
```go
package mongodb

import (
    "context"
    "fmt"
    "math/rand"
    "time"
    
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
)

type WorkloadGenerator struct {
    client *mongo.Client
    db     *mongo.Database
}

func NewWorkloadGenerator(client *mongo.Client) *WorkloadGenerator {
    return &WorkloadGenerator{
        client: client,
        db:     client.Database("testdb"),
    }
}

func (w *WorkloadGenerator) GenerateBasicOperations(ctx context.Context, count int) error {
    collection := w.db.Collection("test_collection")
    
    // Insert operations
    for i := 0; i < count; i++ {
        doc := bson.M{
            "user_id":    fmt.Sprintf("user_%d", i),
            "name":       fmt.Sprintf("Test User %d", i),
            "email":      fmt.Sprintf("user%d@test.com", i),
            "age":        rand.Intn(80) + 18,
            "created_at": time.Now(),
            "tags":       []string{"test", "e2e", fmt.Sprintf("batch_%d", i/10)},
        }
        
        _, err := collection.InsertOne(ctx, doc)
        if err != nil {
            return err
        }
    }
    
    // Update operations
    for i := 0; i < count/2; i++ {
        filter := bson.M{"user_id": fmt.Sprintf("user_%d", i)}
        update := bson.M{"$set": bson.M{"last_login": time.Now()}}
        
        _, err := collection.UpdateOne(ctx, filter, update)
        if err != nil {
            return err
        }
    }
    
    // Find operations
    for i := 0; i < count; i++ {
        filter := bson.M{"age": bson.M{"$gte": rand.Intn(50) + 18}}
        cursor, err := collection.Find(ctx, filter)
        if err != nil {
            return err
        }
        cursor.Close(ctx)
    }
    
    // Aggregation operations
    pipeline := []bson.M{
        {"$match": bson.M{"age": bson.M{"$gte": 25}}},
        {"$group": bson.M{
            "_id":   "$tags",
            "count": bson.M{"$sum": 1},
            "avg_age": bson.M{"$avg": "$age"},
        }},
        {"$sort": bson.M{"count": -1}},
        {"$limit": 10},
    }
    
    cursor, err := collection.Aggregate(ctx, pipeline)
    if err != nil {
        return err
    }
    cursor.Close(ctx)
    
    return nil
}

func (w *WorkloadGenerator) GenerateSlowQueries(ctx context.Context) error {
    collection := w.db.Collection("slow_query_test")
    
    // Create large collection
    var docs []interface{}
    for i := 0; i < 10000; i++ {
        docs = append(docs, bson.M{
            "id":    i,
            "data":  fmt.Sprintf("%064d", i), // Large string
            "field1": rand.Intn(1000),
            "field2": rand.Intn(1000),
            "field3": rand.Intn(1000),
        })
    }
    
    _, err := collection.InsertMany(ctx, docs)
    if err != nil {
        return err
    }
    
    // Slow query without index
    filter := bson.M{
        "$and": []bson.M{
            {"field1": bson.M{"$gte": 500}},
            {"field2": bson.M{"$lte": 500}},
            {"field3": bson.M{"$mod": []int{7, 0}}},
        },
    }
    
    cursor, err := collection.Find(ctx, filter)
    if err != nil {
        return err
    }
    cursor.Close(ctx)
    
    // Complex aggregation
    pipeline := []bson.M{
        {"$match": filter},
        {"$lookup": bson.M{
            "from":         "test_collection",
            "localField":   "id",
            "foreignField": "user_id",
            "as":          "user_data",
        }},
        {"$unwind": "$user_data"},
        {"$group": bson.M{
            "_id":     "$field1",
            "count":   bson.M{"$sum": 1},
            "avg_f2":  bson.M{"$avg": "$field2"},
            "max_f3":  bson.M{"$max": "$field3"},
        }},
    }
    
    cursor, err = collection.Aggregate(ctx, pipeline)
    if err != nil {
        return err
    }
    cursor.Close(ctx)
    
    return nil
}
```

## 4. Metric Verifier

### verifier.go
```go
package mongodb

import (
    "fmt"
    "time"
    
    "github.com/prometheus/client_golang/api"
    v1 "github.com/prometheus/client_golang/api/prometheus/v1"
    "github.com/prometheus/common/model"
)

type MongoDBVerifier struct {
    promAPI v1.API
}

func NewMongoDBVerifier(client api.Client) *MongoDBVerifier {
    return &MongoDBVerifier{
        promAPI: v1.NewAPI(client),
    }
}

func (v *MongoDBVerifier) GetMetrics(metricName string) ([]MetricPoint, error) {
    query := fmt.Sprintf(`%s{service_name="mongodb-test"}`, metricName)
    
    result, _, err := v.promAPI.Query(context.Background(), query, time.Now())
    if err != nil {
        return nil, err
    }
    
    vector, ok := result.(model.Vector)
    if !ok {
        return nil, fmt.Errorf("unexpected result type")
    }
    
    var metrics []MetricPoint
    for _, sample := range vector {
        metrics = append(metrics, MetricPoint{
            Labels: sample.Metric,
            Value:  float64(sample.Value),
            Time:   sample.Timestamp.Time(),
        })
    }
    
    return metrics, nil
}

func (v *MongoDBVerifier) VerifyDatabaseMetrics() error {
    // Check database size
    dbSizeMetrics, err := v.GetMetrics("mongodb_database_size_bytes")
    if err != nil {
        return err
    }
    
    if len(dbSizeMetrics) == 0 {
        return fmt.Errorf("no database size metrics found")
    }
    
    // Verify we have metrics for expected databases
    expectedDbs := map[string]bool{"admin": false, "testdb": false}
    for _, metric := range dbSizeMetrics {
        if db, ok := metric.Labels["database"]; ok {
            expectedDbs[string(db)] = true
        }
    }
    
    for db, found := range expectedDbs {
        if !found {
            return fmt.Errorf("missing metrics for database: %s", db)
        }
    }
    
    return nil
}

func (v *MongoDBVerifier) GetOperationLatencies() (map[string]float64, error) {
    latencies := make(map[string]float64)
    
    percentiles := []string{"0.5", "0.95", "0.99"}
    for _, p := range percentiles {
        query := fmt.Sprintf(
            `histogram_quantile(%s, sum(rate(mongodb_operation_latency_bucket[5m])) by (le))`,
            p,
        )
        
        result, _, err := v.promAPI.Query(context.Background(), query, time.Now())
        if err != nil {
            return nil, err
        }
        
        if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
            latencies[fmt.Sprintf("p%d", int(float64(p)*100))] = float64(vector[0].Value)
        }
    }
    
    return latencies, nil
}
```

## 5. Test Data Initialization

### deployments/docker/init-scripts/mongodb-init.js
```javascript
// MongoDB initialization script for E2E testing

// Switch to admin database
db = db.getSiblingDB('admin');

// Create monitoring user
db.createUser({
  user: 'monitoring',
  pwd: 'monitoring_password',
  roles: [
    { role: 'clusterMonitor', db: 'admin' },
    { role: 'read', db: 'local' },
    { role: 'readAnyDatabase', db: 'admin' }
  ]
});

// Create test database
db = db.getSiblingDB('testdb');

// Create collections with different patterns
db.createCollection('test_collection');
db.createCollection('slow_query_test');
db.createCollection('time_series_test', {
  timeseries: {
    timeField: 'timestamp',
    metaField: 'metadata',
    granularity: 'seconds'
  }
});

// Create indexes
db.test_collection.createIndex({ user_id: 1 });
db.test_collection.createIndex({ email: 1 }, { unique: true });
db.test_collection.createIndex({ age: 1, created_at: -1 });
db.test_collection.createIndex({ tags: 1 });

// Insert sample data
var sampleUsers = [];
for (var i = 0; i < 1000; i++) {
  sampleUsers.push({
    user_id: 'init_user_' + i,
    name: 'Initial User ' + i,
    email: 'init' + i + '@test.com',
    age: Math.floor(Math.random() * 60) + 18,
    created_at: new Date(),
    tags: ['initial', 'test', 'user_group_' + Math.floor(i / 100)]
  });
}
db.test_collection.insertMany(sampleUsers);

// Create a function for slow query testing
db.system.js.save({
  _id: 'slowFunction',
  value: function(n) {
    var sum = 0;
    for (var i = 0; i < n; i++) {
      sum += Math.sqrt(i);
    }
    return sum;
  }
});

// Enable profiling for slow query detection
db.setProfilingLevel(1, { slowms: 100 });

print('MongoDB E2E test initialization complete');
```

## 6. Docker Compose Addition

### deployments/docker/compose.yaml (MongoDB section)
```yaml
  mongodb:
    image: mongo:7.0
    container_name: db-intel-mongodb
    environment:
      - MONGO_INITDB_ROOT_USERNAME=root
      - MONGO_INITDB_ROOT_PASSWORD=rootpassword
      - MONGO_INITDB_DATABASE=admin
    volumes:
      - ./init-scripts/mongodb-init.js:/docker-entrypoint-initdb.d/init.js:ro
      - mongodb-data:/data/db
    ports:
      - "27017:27017"
    command: mongod --replSet rs0
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 30s
    networks:
      - db-intel-network

  # MongoDB replica set initialization
  mongodb-init:
    image: mongo:7.0
    depends_on:
      mongodb:
        condition: service_healthy
    networks:
      - db-intel-network
    command: >
      mongosh --host mongodb:27017 -u root -p rootpassword --eval 
      "rs.initiate({
        _id: 'rs0',
        members: [{ _id: 0, host: 'mongodb:27017' }]
      })"
```

## Usage

1. **Run MongoDB E2E Tests**:
```bash
cd tests/e2e/databases/mongodb
go test -v -tags=e2e
```

2. **Run with specific test**:
```bash
go test -v -tags=e2e -run TestMongoDBE2E/BasicMetrics
```

3. **Debug mode**:
```bash
E2E_DEBUG=true go test -v -tags=e2e
```

This example provides a complete template for implementing E2E tests for any database type. The same pattern can be applied to Redis, Oracle, SQL Server, and other databases by adapting the specific metrics, workload patterns, and verification logic.