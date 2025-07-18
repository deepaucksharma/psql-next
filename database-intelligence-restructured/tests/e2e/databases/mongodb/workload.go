package mongodb

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// WorkloadGenerator generates MongoDB workload for testing
type WorkloadGenerator struct {
	client   *mongo.Client
	db       *mongo.Database
	endpoint string
}

// WorkloadConfig configures workload generation
type WorkloadConfig struct {
	Duration          time.Duration
	ConcurrentClients int
	OperationsPerSec  int
}

// NewWorkloadGenerator creates a new MongoDB workload generator
func NewWorkloadGenerator(endpoint, username, password string) (*WorkloadGenerator, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Create connection URI
	uri := fmt.Sprintf("mongodb://%s:%s@%s/admin", username, password, endpoint)
	
	// Connect to MongoDB
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	
	// Ping to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	
	return &WorkloadGenerator{
		client:   client,
		db:       client.Database("testdb"),
		endpoint: endpoint,
	}, nil
}

// Close closes the MongoDB connection
func (w *WorkloadGenerator) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.client.Disconnect(ctx)
}

// GenerateBasicOperations generates basic CRUD operations
func (w *WorkloadGenerator) GenerateBasicOperations(ctx context.Context, count int) error {
	collection := w.db.Collection("basic_operations")
	
	// Insert operations
	for i := 0; i < count; i++ {
		doc := bson.M{
			"user_id":    fmt.Sprintf("user_%d", i),
			"name":       fmt.Sprintf("Test User %d", i),
			"email":      fmt.Sprintf("user%d@test.com", i),
			"age":        rand.Intn(60) + 18,
			"created_at": time.Now(),
			"active":     rand.Float32() > 0.2,
			"tags":       generateTags(),
			"metadata": bson.M{
				"source":    "e2e_test",
				"batch":     i / 100,
				"timestamp": time.Now().Unix(),
			},
		}
		
		if _, err := collection.InsertOne(ctx, doc); err != nil {
			return fmt.Errorf("insert failed: %w", err)
		}
	}
	
	// Update operations
	for i := 0; i < count/2; i++ {
		filter := bson.M{"user_id": fmt.Sprintf("user_%d", i)}
		update := bson.M{
			"$set": bson.M{
				"last_login":   time.Now(),
				"login_count":  rand.Intn(100),
				"updated_at":   time.Now(),
			},
			"$inc": bson.M{
				"version": 1,
			},
		}
		
		if _, err := collection.UpdateOne(ctx, filter, update); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
	}
	
	// Find operations
	for i := 0; i < count; i++ {
		var filter bson.M
		
		// Vary query patterns
		switch i % 5 {
		case 0:
			filter = bson.M{"age": bson.M{"$gte": 25, "$lte": 45}}
		case 1:
			filter = bson.M{"active": true}
		case 2:
			filter = bson.M{"tags": bson.M{"$in": []string{"test", "e2e"}}}
		case 3:
			filter = bson.M{"user_id": fmt.Sprintf("user_%d", rand.Intn(count))}
		default:
			filter = bson.M{"name": bson.M{"$regex": "Test User"}}
		}
		
		cursor, err := collection.Find(ctx, filter)
		if err != nil {
			return fmt.Errorf("find failed: %w", err)
		}
		cursor.Close(ctx)
	}
	
	// Aggregation operations
	pipeline := []bson.M{
		{"$match": bson.M{"active": true}},
		{"$group": bson.M{
			"_id":      "$metadata.batch",
			"count":    bson.M{"$sum": 1},
			"avg_age":  bson.M{"$avg": "$age"},
			"max_age":  bson.M{"$max": "$age"},
			"min_age":  bson.M{"$min": "$age"},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 10},
	}
	
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("aggregation failed: %w", err)
	}
	cursor.Close(ctx)
	
	// Delete operations
	for i := 0; i < count/10; i++ {
		filter := bson.M{"user_id": fmt.Sprintf("user_%d", rand.Intn(count))}
		if _, err := collection.DeleteOne(ctx, filter); err != nil {
			return fmt.Errorf("delete failed: %w", err)
		}
	}
	
	return nil
}

// CreateCollection creates a collection with specified number of documents
func (w *WorkloadGenerator) CreateCollection(ctx context.Context, name string, docCount int) error {
	collection := w.db.Collection(name)
	
	// Drop existing collection
	_ = collection.Drop(ctx)
	
	// Batch insert for efficiency
	batchSize := 1000
	for i := 0; i < docCount; i += batchSize {
		var docs []interface{}
		
		end := i + batchSize
		if end > docCount {
			end = docCount
		}
		
		for j := i; j < end; j++ {
			doc := generateDocument(name, j)
			docs = append(docs, doc)
		}
		
		if _, err := collection.InsertMany(ctx, docs); err != nil {
			return fmt.Errorf("batch insert failed: %w", err)
		}
	}
	
	// Create indexes
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{"id", 1}}},
		{Keys: bson.D{{"created_at", -1}}},
	}
	
	if name == "users" {
		indexes = append(indexes, mongo.IndexModel{Keys: bson.D{{"email", 1}}, Options: options.Index().SetUnique(true)})
	}
	
	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("index creation failed: %w", err)
	}
	
	return nil
}

// GenerateOperations generates specific operation types
func (w *WorkloadGenerator) GenerateOperations(ctx context.Context, opType string, count int) error {
	collection := w.db.Collection("operations_test")
	
	switch opType {
	case "insert":
		for i := 0; i < count; i++ {
			doc := generateDocument("operations", i)
			if _, err := collection.InsertOne(ctx, doc); err != nil {
				return err
			}
		}
		
	case "find":
		// Ensure we have data to find
		w.ensureTestData(ctx, collection, 1000)
		
		for i := 0; i < count; i++ {
			filter := bson.M{"id": rand.Intn(1000)}
			var result bson.M
			if err := collection.FindOne(ctx, filter).Decode(&result); err != nil && err != mongo.ErrNoDocuments {
				return err
			}
		}
		
	case "update":
		// Ensure we have data to update
		w.ensureTestData(ctx, collection, 1000)
		
		for i := 0; i < count; i++ {
			filter := bson.M{"id": rand.Intn(1000)}
			update := bson.M{"$set": bson.M{"updated_at": time.Now(), "counter": i}}
			if _, err := collection.UpdateOne(ctx, filter, update); err != nil {
				return err
			}
		}
		
	case "delete":
		// Ensure we have data to delete
		w.ensureTestData(ctx, collection, count*2)
		
		for i := 0; i < count; i++ {
			filter := bson.M{"id": i}
			if _, err := collection.DeleteOne(ctx, filter); err != nil {
				return err
			}
		}
		
	default:
		return fmt.Errorf("unknown operation type: %s", opType)
	}
	
	return nil
}

// GenerateReplicationLoad generates load suitable for testing replication
func (w *WorkloadGenerator) GenerateReplicationLoad(ctx context.Context, docCount int) error {
	collection := w.db.Collection("replication_test")
	
	// Rapid inserts to test replication
	docs := make([]interface{}, 0, 100)
	for i := 0; i < docCount; i++ {
		doc := bson.M{
			"id":          i,
			"timestamp":   time.Now(),
			"data":        generateRandomData(1024), // 1KB of data
			"replication": true,
		}
		docs = append(docs, doc)
		
		// Batch insert every 100 documents
		if len(docs) >= 100 || i == docCount-1 {
			if _, err := collection.InsertMany(ctx, docs); err != nil {
				return err
			}
			docs = docs[:0]
		}
	}
	
	return nil
}

// GenerateHeavyLoad generates sustained heavy workload
func (w *WorkloadGenerator) GenerateHeavyLoad(ctx context.Context, config WorkloadConfig) error {
	// Create worker context with timeout
	workCtx, cancel := context.WithTimeout(ctx, config.Duration)
	defer cancel()
	
	// Calculate operations per worker
	opsPerWorker := config.OperationsPerSec / config.ConcurrentClients
	if opsPerWorker < 1 {
		opsPerWorker = 1
	}
	
	// Start workers
	var wg sync.WaitGroup
	errChan := make(chan error, config.ConcurrentClients)
	
	for i := 0; i < config.ConcurrentClients; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			collection := w.db.Collection(fmt.Sprintf("heavy_load_%d", workerID))
			ticker := time.NewTicker(time.Second / time.Duration(opsPerWorker))
			defer ticker.Stop()
			
			opCount := 0
			for {
				select {
				case <-workCtx.Done():
					return
				case <-ticker.C:
					// Vary operation types
					switch opCount % 4 {
					case 0: // Insert
						doc := generateDocument("heavy", opCount)
						if _, err := collection.InsertOne(workCtx, doc); err != nil {
							errChan <- err
							return
						}
					case 1: // Find
						filter := bson.M{"id": rand.Intn(1000)}
						cursor, err := collection.Find(workCtx, filter)
						if err != nil {
							errChan <- err
							return
						}
						cursor.Close(workCtx)
					case 2: // Update
						filter := bson.M{"id": rand.Intn(1000)}
						update := bson.M{"$inc": bson.M{"counter": 1}}
						if _, err := collection.UpdateOne(workCtx, filter, update); err != nil {
							errChan <- err
							return
						}
					case 3: // Aggregation
						pipeline := []bson.M{
							{"$sample": bson.M{"size": 10}},
							{"$group": bson.M{
								"_id":   nil,
								"count": bson.M{"$sum": 1},
								"avg":   bson.M{"$avg": "$value"},
							}},
						}
						cursor, err := collection.Aggregate(workCtx, pipeline)
						if err != nil {
							errChan <- err
							return
						}
						cursor.Close(workCtx)
					}
					opCount++
				}
			}
		}(i)
	}
	
	// Wait for completion
	wg.Wait()
	close(errChan)
	
	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	
	return nil
}

// Helper functions

func (w *WorkloadGenerator) ensureTestData(ctx context.Context, collection *mongo.Collection, minDocs int) error {
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	
	if count < int64(minDocs) {
		// Insert missing documents
		var docs []interface{}
		for i := int(count); i < minDocs; i++ {
			docs = append(docs, generateDocument("test", i))
		}
		if _, err := collection.InsertMany(ctx, docs); err != nil {
			return err
		}
	}
	
	return nil
}

func generateDocument(collectionType string, id int) bson.M {
	base := bson.M{
		"id":         id,
		"created_at": time.Now(),
		"type":       collectionType,
		"value":      rand.Float64() * 1000,
		"active":     rand.Float32() > 0.1,
	}
	
	switch collectionType {
	case "users":
		base["email"] = fmt.Sprintf("user%d@test.com", id)
		base["name"] = fmt.Sprintf("User %d", id)
		base["age"] = rand.Intn(60) + 18
		
	case "orders":
		base["user_id"] = rand.Intn(1000)
		base["total"] = rand.Float64() * 500
		base["status"] = []string{"pending", "processing", "shipped", "delivered"}[rand.Intn(4)]
		
	case "products":
		base["name"] = fmt.Sprintf("Product %d", id)
		base["price"] = rand.Float64() * 100
		base["stock"] = rand.Intn(1000)
		base["category"] = []string{"electronics", "clothing", "food", "books"}[rand.Intn(4)]
	}
	
	return base
}

func generateTags() []string {
	allTags := []string{"test", "e2e", "mongodb", "performance", "load", "stress", "basic", "advanced"}
	
	// Select random number of tags
	numTags := rand.Intn(4) + 1
	tags := make([]string, 0, numTags)
	
	for i := 0; i < numTags; i++ {
		tag := allTags[rand.Intn(len(allTags))]
		// Avoid duplicates
		found := false
		for _, t := range tags {
			if t == tag {
				found = true
				break
			}
		}
		if !found {
			tags = append(tags, tag)
		}
	}
	
	return tags
}

func generateRandomData(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	data := make([]byte, size)
	for i := range data {
		data[i] = charset[rand.Intn(len(charset))]
	}
	return string(data)
}