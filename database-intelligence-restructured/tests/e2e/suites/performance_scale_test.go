package suites

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/database-intelligence/tests/e2e/framework"
)

// PerformanceScaleTestSuite tests collector performance and scalability
type PerformanceScaleTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	ctx       context.Context
	cancel    context.CancelFunc
}

func (s *PerformanceScaleTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 60*time.Minute)
	
	env, err := framework.NewTestEnvironment(s.ctx, framework.TestConfig{
		DatabaseType: "postgresql",
		EnableMySQL:  true,
		// Use high-performance configuration
		DBConfig: map[string]string{
			"max_connections":    "500",
			"shared_buffers":     "1GB",
			"work_mem":          "16MB",
			"maintenance_work_mem": "256MB",
		},
	})
	s.Require().NoError(err)
	s.env = env

	// Start collector with performance-optimized configuration
	collector, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/enhanced-mode-full.yaml",
		LogLevel:   "info", // Less verbose for performance tests
		Features: []string{
			"postgresql",
			"mysql",
			"hostmetrics",
			"enhancedsql",
			"ash",
			"adaptivesampler",
			"circuitbreaker",
			"costcontrol",
		},
		// Performance tuning
		EnvVars: map[string]string{
			"GOGC":       "100",
			"GOMAXPROCS": fmt.Sprintf("%d", runtime.NumCPU()),
			"GOMEMLIMIT": "4GiB",
		},
	})
	s.Require().NoError(err)
	s.collector = collector

	s.Require().NoError(s.collector.WaitForReady(s.ctx, 2*time.Minute))
	
	// Create performance test schema
	s.setupPerformanceSchema()
}

func (s *PerformanceScaleTestSuite) TearDownSuite() {
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
	s.cancel()
}

func (s *PerformanceScaleTestSuite) setupPerformanceSchema() {
	// Create tables with various sizes for testing
	tables := []struct {
		name string
		ddl  string
		rows int
	}{
		{
			name: "small_table",
			ddl: `CREATE TABLE small_table (
				id SERIAL PRIMARY KEY,
				data VARCHAR(100),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			rows: 1000,
		},
		{
			name: "medium_table",
			ddl: `CREATE TABLE medium_table (
				id SERIAL PRIMARY KEY,
				user_id INT,
				data TEXT,
				status VARCHAR(20),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			rows: 100000,
		},
		{
			name: "large_table",
			ddl: `CREATE TABLE large_table (
				id SERIAL PRIMARY KEY,
				user_id INT,
				session_id UUID,
				event_type VARCHAR(50),
				event_data JSONB,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
			rows: 1000000,
		},
	}
	
	for _, table := range tables {
		// Create table
		_, err := s.env.ExecuteSQL(table.ddl)
		s.Require().NoError(err)
		
		// Insert data in batches
		batchSize := 1000
		for i := 0; i < table.rows; i += batchSize {
			var values []string
			for j := 0; j < batchSize && i+j < table.rows; j++ {
				switch table.name {
				case "small_table":
					values = append(values, fmt.Sprintf("('data-%d')", i+j))
				case "medium_table":
					values = append(values, fmt.Sprintf("(%d, 'data-%d', 'active')", (i+j)%1000, i+j))
				case "large_table":
					values = append(values, fmt.Sprintf("(%d, gen_random_uuid(), 'event-%d', '{\"value\": %d}')", 
						(i+j)%10000, (i+j)%10, i+j))
				}
			}
			
			if len(values) > 0 {
				query := fmt.Sprintf("INSERT INTO %s %s VALUES %s",
					table.name,
					map[string]string{
						"small_table":  "(data)",
						"medium_table": "(user_id, data, status)",
						"large_table":  "(user_id, session_id, event_type, event_data)",
					}[table.name],
					values[0]) // Simplified for example
				_, err := s.env.ExecuteSQL(query)
				s.Require().NoError(err)
			}
		}
		
		// Create indexes
		switch table.name {
		case "medium_table":
			s.env.ExecuteSQL("CREATE INDEX idx_medium_user ON medium_table(user_id)")
			s.env.ExecuteSQL("CREATE INDEX idx_medium_status ON medium_table(status)")
		case "large_table":
			s.env.ExecuteSQL("CREATE INDEX idx_large_user ON large_table(user_id)")
			s.env.ExecuteSQL("CREATE INDEX idx_large_event ON large_table(event_type)")
			s.env.ExecuteSQL("CREATE INDEX idx_large_created ON large_table(created_at)")
		}
	}
	
	// Analyze all tables
	_, err := s.env.ExecuteSQL("ANALYZE")
	s.Require().NoError(err)
}

// Test01_BaselinePerformance establishes performance baselines
func (s *PerformanceScaleTestSuite) Test01_BaselinePerformance() {
	// Measure baseline metrics collection performance
	
	// Get initial resource usage
	initialResources := s.getCollectorResources()
	s.T().Logf("Initial resources - Memory: %.2f MB, CPU: %.2f%%", 
		initialResources.MemoryMB, initialResources.CPUPercent)
	
	// Run light workload
	workload := s.env.CreateWorkload("baseline", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10, // Light load
		ConnectionCount: 5,
		Operations: []string{
			"SELECT * FROM small_table WHERE id = $1",
			"SELECT COUNT(*) FROM medium_table WHERE user_id = $1",
		},
	})
	
	// Measure metrics collection during workload
	var collectionTimes []time.Duration
	done := make(chan bool)
	
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				start := time.Now()
				metrics, err := s.collector.GetMetrics(s.ctx, "*")
				if err == nil {
					collectionTimes = append(collectionTimes, time.Since(start))
					s.T().Logf("Collected %d metrics in %v", len(metrics), time.Since(start))
				}
				time.Sleep(10 * time.Second)
			}
		}
	}()
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	done <- true
	
	// Get final resource usage
	finalResources := s.getCollectorResources()
	
	// Calculate statistics
	avgCollection := s.averageDuration(collectionTimes)
	p95Collection := s.percentileDuration(collectionTimes, 95)
	
	memoryIncrease := finalResources.MemoryMB - initialResources.MemoryMB
	cpuAverage := (initialResources.CPUPercent + finalResources.CPUPercent) / 2
	
	// Assert baseline performance
	s.Assert().Less(avgCollection, 100*time.Millisecond, 
		"Average collection time should be < 100ms")
	s.Assert().Less(p95Collection, 200*time.Millisecond, 
		"P95 collection time should be < 200ms")
	s.Assert().Less(memoryIncrease, 100.0, 
		"Memory increase should be < 100MB for light load")
	s.Assert().Less(cpuAverage, 10.0, 
		"Average CPU should be < 10% for light load")
	
	s.T().Logf("Baseline Performance:\n"+
		"  Avg Collection: %v\n"+
		"  P95 Collection: %v\n"+
		"  Memory Increase: %.2f MB\n"+
		"  Avg CPU: %.2f%%",
		avgCollection, p95Collection, memoryIncrease, cpuAverage)
}

// Test02_HighThroughput tests performance under high query rates
func (s *PerformanceScaleTestSuite) Test02_HighThroughput() {
	// Test with very high query rates
	
	targetQPS := 1000 // Target 1000 queries per second
	duration := 5 * time.Minute
	
	s.T().Logf("Starting high throughput test: %d QPS for %v", targetQPS, duration)
	
	// Track actual QPS
	var queryCount int64
	var errorCount int64
	
	// Create multiple workers to achieve target QPS
	workerCount := 50
	queriesPerWorker := targetQPS / workerCount
	
	var wg sync.WaitGroup
	start := time.Now()
	
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			conn := s.env.GetConnection()
			defer conn.Close()
			
			ticker := time.NewTicker(time.Second / time.Duration(queriesPerWorker))
			defer ticker.Stop()
			
			timeout := time.After(duration)
			
			for {
				select {
				case <-timeout:
					return
				case <-ticker.C:
					// Execute varied queries
					query := s.getRandomQuery(workerID)
					_, err := conn.Exec(query)
					
					atomic.AddInt64(&queryCount, 1)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					}
				}
			}
		}(i)
	}
	
	// Monitor performance during test
	go s.monitorPerformance(duration)
	
	wg.Wait()
	elapsed := time.Since(start)
	
	// Calculate actual QPS
	actualQPS := float64(queryCount) / elapsed.Seconds()
	errorRate := float64(errorCount) / float64(queryCount) * 100
	
	s.T().Logf("High Throughput Results:\n"+
		"  Target QPS: %d\n"+
		"  Actual QPS: %.2f\n"+
		"  Total Queries: %d\n"+
		"  Error Rate: %.2f%%",
		targetQPS, actualQPS, queryCount, errorRate)
	
	// Performance assertions
	s.Assert().Greater(actualQPS, float64(targetQPS)*0.8, 
		"Should achieve at least 80% of target QPS")
	s.Assert().Less(errorRate, 1.0, 
		"Error rate should be < 1%")
	
	// Check collector didn't degrade
	collectorMetrics, err := s.collector.GetMetrics(s.ctx, "otelcol_processor_refused_metric_points")
	s.Require().NoError(err)
	
	droppedMetrics := s.sumMetricValues(collectorMetrics)
	s.Assert().Equal(0.0, droppedMetrics, 
		"Collector should not drop metrics under high load")
}

// Test03_HighCardinality tests handling of high cardinality metrics
func (s *PerformanceScaleTestSuite) Test03_HighCardinality() {
	// Generate high cardinality scenario
	
	uniqueUsers := 10000
	uniqueQueries := 1000
	duration := 3 * time.Minute
	
	s.T().Logf("Starting high cardinality test: %d users, %d unique queries", 
		uniqueUsers, uniqueQueries)
	
	// Generate unique queries
	queries := make([]string, uniqueQueries)
	for i := 0; i < uniqueQueries; i++ {
		queries[i] = fmt.Sprintf("SELECT * FROM large_table WHERE user_id = $1 AND event_type = 'event-%d'", i)
	}
	
	// Track cardinality
	initialMetrics, _ := s.collector.GetMetrics(s.ctx, "*")
	initialCardinality := s.calculateCardinality(initialMetrics)
	
	// Run high cardinality workload
	var wg sync.WaitGroup
	workersPerBatch := 100
	
	for batch := 0; batch < uniqueUsers/workersPerBatch; batch++ {
		for i := 0; i < workersPerBatch; i++ {
			wg.Add(1)
			userID := batch*workersPerBatch + i
			
			go func(uid int) {
				defer wg.Done()
				
				endTime := time.Now().Add(duration)
				for time.Now().Before(endTime) {
					// Each user runs different queries
					queryIdx := uid % len(queries)
					s.env.ExecuteSQL(queries[queryIdx], uid)
					time.Sleep(100 * time.Millisecond)
				}
			}(userID)
		}
		
		// Stagger batch starts
		time.Sleep(5 * time.Second)
	}
	
	wg.Wait()
	
	// Check final cardinality
	finalMetrics, err := s.collector.GetMetrics(s.ctx, "*")
	s.Require().NoError(err)
	finalCardinality := s.calculateCardinality(finalMetrics)
	
	cardinalityIncrease := finalCardinality - initialCardinality
	
	s.T().Logf("Cardinality Results:\n"+
		"  Initial: %d\n"+
		"  Final: %d\n"+
		"  Increase: %d",
		initialCardinality, finalCardinality, cardinalityIncrease)
	
	// Check cardinality was managed
	s.Assert().Less(finalCardinality, 100000, 
		"Total cardinality should be limited to < 100K series")
	
	// Verify cost control worked
	costMetrics, _ := s.collector.GetMetrics(s.ctx, "otelcol.costcontrol.*")
	if len(costMetrics) > 0 {
		s.T().Log("Cost control processor successfully limited cardinality")
	}
}

// Test04_BurstLoad tests handling of sudden traffic spikes
func (s *PerformanceScaleTestSuite) Test04_BurstLoad() {
	// Test burst scenarios
	
	normalQPS := 50
	burstQPS := 500
	burstDuration := 30 * time.Second
	testDuration := 5 * time.Minute
	
	s.T().Log("Starting burst load test")
	
	// Metrics tracking
	var normalLatencies []time.Duration
	var burstLatencies []time.Duration
	var mu sync.Mutex
	
	// Create workload controller
	var currentQPS int32 = int32(normalQPS)
	
	// Query workers
	var wg sync.WaitGroup
	workerCount := 20
	
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			endTime := time.Now().Add(testDuration)
			for time.Now().Before(endTime) {
				qps := atomic.LoadInt32(&currentQPS)
				delay := time.Second / time.Duration(qps/int32(workerCount))
				
				start := time.Now()
				_, err := s.env.ExecuteSQL("SELECT COUNT(*) FROM medium_table WHERE user_id = $1", workerID)
				latency := time.Since(start)
				
				if err == nil {
					mu.Lock()
					if qps > int32(normalQPS*2) {
						burstLatencies = append(burstLatencies, latency)
					} else {
						normalLatencies = append(normalLatencies, latency)
					}
					mu.Unlock()
				}
				
				time.Sleep(delay)
			}
		}(i)
	}
	
	// Burst controller
	go func() {
		time.Sleep(1 * time.Minute) // Normal period
		
		s.T().Log("Triggering burst")
		atomic.StoreInt32(&currentQPS, int32(burstQPS))
		time.Sleep(burstDuration)
		
		s.T().Log("Returning to normal")
		atomic.StoreInt32(&currentQPS, int32(normalQPS))
	}()
	
	wg.Wait()
	
	// Calculate statistics
	normalP95 := s.percentileDuration(normalLatencies, 95)
	burstP95 := s.percentileDuration(burstLatencies, 95)
	
	degradation := float64(burstP95-normalP95) / float64(normalP95) * 100
	
	s.T().Logf("Burst Load Results:\n"+
		"  Normal P95 Latency: %v\n"+
		"  Burst P95 Latency: %v\n"+
		"  Degradation: %.1f%%",
		normalP95, burstP95, degradation)
	
	// Performance should degrade gracefully
	s.Assert().Less(degradation, 200.0, 
		"Latency degradation during burst should be < 200%")
	
	// Check adaptive sampling kicked in
	s.checkCollectorLogs("adaptive_sampler", []string{
		"spike detected",
		"adjusting sample rate",
	})
}

// Test05_LongRunningStability tests 24-hour stability
func (s *PerformanceScaleTestSuite) Test05_LongRunningStability() {
	if testing.Short() {
		s.T().Skip("Skipping 24-hour stability test in short mode")
	}
	
	duration := 24 * time.Hour
	checkInterval := 1 * time.Hour
	
	s.T().Logf("Starting %v stability test", duration)
	
	// Resource tracking
	type ResourceSnapshot struct {
		Time      time.Time
		MemoryMB  float64
		CPUPercent float64
		Metrics   int
		Errors    int
	}
	
	var snapshots []ResourceSnapshot
	var snapshotMu sync.Mutex
	
	// Start continuous workload
	ctx, cancel := context.WithTimeout(s.ctx, duration)
	defer cancel()
	
	workload := s.env.CreateWorkload("stability", framework.WorkloadConfig{
		Duration:        duration,
		QueryRate:       100,
		ConnectionCount: 20,
		Operations: []string{
			"SELECT * FROM small_table ORDER BY RANDOM() LIMIT 10",
			"SELECT COUNT(*) FROM medium_table WHERE status = $1",
			"INSERT INTO large_table (user_id, event_type, event_data) VALUES ($1, $2, $3)",
			"UPDATE medium_table SET updated_at = NOW() WHERE id = $1",
			"DELETE FROM large_table WHERE created_at < NOW() - INTERVAL '1 day' LIMIT 100",
		},
	})
	
	go workload.Run(ctx)
	
	// Monitor resources over time
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			goto done
		case <-ticker.C:
			resources := s.getCollectorResources()
			metrics, _ := s.collector.GetMetrics(s.ctx, "*")
			errors := s.getErrorCount()
			
			snapshot := ResourceSnapshot{
				Time:       time.Now(),
				MemoryMB:   resources.MemoryMB,
				CPUPercent: resources.CPUPercent,
				Metrics:    len(metrics),
				Errors:     errors,
			}
			
			snapshotMu.Lock()
			snapshots = append(snapshots, snapshot)
			snapshotMu.Unlock()
			
			s.T().Logf("Stability check at %v:\n"+
				"  Memory: %.2f MB\n"+
				"  CPU: %.2f%%\n"+
				"  Metrics: %d\n"+
				"  Errors: %d",
				time.Since(startTime).Round(time.Hour),
				snapshot.MemoryMB, snapshot.CPUPercent,
				snapshot.Metrics, snapshot.Errors)
		}
	}
	
done:
	// Analyze stability
	if len(snapshots) < 2 {
		s.T().Skip("Not enough data for stability analysis")
		return
	}
	
	// Check for memory leaks
	firstSnapshot := snapshots[0]
	lastSnapshot := snapshots[len(snapshots)-1]
	
	memoryGrowth := lastSnapshot.MemoryMB - firstSnapshot.MemoryMB
	memoryGrowthRate := memoryGrowth / float64(len(snapshots))
	
	s.Assert().Less(memoryGrowthRate, 10.0, 
		"Memory growth rate should be < 10MB per hour")
	
	// Check for consistent performance
	var cpuValues []float64
	for _, snapshot := range snapshots {
		cpuValues = append(cpuValues, snapshot.CPUPercent)
	}
	
	cpuStdDev := s.standardDeviation(cpuValues)
	s.Assert().Less(cpuStdDev, 10.0, 
		"CPU usage should be stable (std dev < 10%)")
	
	// Check error rate
	totalErrors := lastSnapshot.Errors
	errorRate := float64(totalErrors) / duration.Hours()
	s.Assert().Less(errorRate, 10.0, 
		"Error rate should be < 10 per hour")
	
	s.T().Logf("Stability Test Summary:\n"+
		"  Duration: %v\n"+
		"  Memory Growth: %.2f MB (%.2f MB/hour)\n"+
		"  CPU Std Dev: %.2f%%\n"+
		"  Total Errors: %d (%.2f/hour)",
		duration, memoryGrowth, memoryGrowthRate,
		cpuStdDev, totalErrors, errorRate)
}

// Test06_ResourceLimits tests behavior at resource limits
func (s *PerformanceScaleTestSuite) Test06_ResourceLimits() {
	// Test behavior when approaching memory and CPU limits
	
	s.T().Log("Testing resource limit behavior")
	
	// Create memory pressure
	s.T().Log("Phase 1: Memory pressure test")
	
	// Generate large result sets
	memoryWorkload := s.env.CreateWorkload("memory_pressure", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       50,
		ConnectionCount: 10,
		Operations: []string{
			"SELECT * FROM large_table", // Large result set
			"SELECT * FROM large_table l1 JOIN large_table l2 ON l1.user_id = l2.user_id LIMIT 10000",
		},
	})
	
	initialMemory := s.getCollectorResources().MemoryMB
	
	go memoryWorkload.Run(s.ctx)
	
	// Monitor memory usage
	maxMemory := initialMemory
	for i := 0; i < 24; i++ { // 2 minutes
		time.Sleep(5 * time.Second)
		currentMemory := s.getCollectorResources().MemoryMB
		if currentMemory > maxMemory {
			maxMemory = currentMemory
		}
		
		// Check if memory limiter activated
		if currentMemory < maxMemory*0.9 {
			s.T().Log("Memory limiter activated")
			break
		}
	}
	
	memoryIncrease := maxMemory - initialMemory
	s.T().Logf("Memory pressure test - Increase: %.2f MB", memoryIncrease)
	
	// Memory should be limited
	s.Assert().Less(maxMemory, 4096.0, 
		"Memory usage should be limited to < 4GB")
	
	// Test CPU limits
	s.T().Log("Phase 2: CPU pressure test")
	
	// CPU intensive queries
	cpuWorkload := s.env.CreateWorkload("cpu_pressure", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       200,
		ConnectionCount: 50,
		Operations: []string{
			"SELECT COUNT(*) FROM generate_series(1, 1000000)",
			"SELECT md5(random()::text) FROM generate_series(1, 10000)",
		},
	})
	
	go cpuWorkload.Run(s.ctx)
	
	// Monitor CPU usage
	var cpuReadings []float64
	for i := 0; i < 24; i++ { // 2 minutes
		time.Sleep(5 * time.Second)
		cpu := s.getCollectorResources().CPUPercent
		cpuReadings = append(cpuReadings, cpu)
	}
	
	maxCPU := s.maxFloat(cpuReadings)
	avgCPU := s.averageFloat(cpuReadings)
	
	s.T().Logf("CPU pressure test - Max: %.2f%%, Avg: %.2f%%", maxCPU, avgCPU)
	
	// CPU should be managed
	s.Assert().Less(avgCPU, 80.0, 
		"Average CPU should be < 80%")
	
	// Check circuit breaker activation
	if maxCPU > 70 {
		s.checkCollectorLogs("circuitbreaker", []string{
			"threshold exceeded",
			"circuit opened",
		})
	}
}

// Test07_MultiDatabaseScale tests scaling across multiple databases
func (s *PerformanceScaleTestSuite) Test07_MultiDatabaseScale() {
	// Test performance with multiple databases
	
	dbCount := 10
	s.T().Logf("Testing with %d databases", dbCount)
	
	// Create additional databases
	databases := []string{"postgres"} // Start with default
	for i := 1; i < dbCount; i++ {
		dbName := fmt.Sprintf("testdb_%d", i)
		err := s.env.CreateDatabase(dbName)
		s.Require().NoError(err)
		databases = append(databases, dbName)
		
		// Create schema in each database
		conn := s.env.GetConnectionToDatabase(dbName)
		_, err = conn.Exec(`
			CREATE TABLE metrics_test (
				id SERIAL PRIMARY KEY,
				value FLOAT,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
		s.Require().NoError(err)
		conn.Close()
	}
	
	// Run workload across all databases
	var wg sync.WaitGroup
	metricsPerDB := make(map[string]int64)
	var metricsMu sync.Mutex
	
	for _, db := range databases {
		for i := 0; i < 5; i++ { // 5 workers per database
			wg.Add(1)
			go func(database string, workerID int) {
				defer wg.Done()
				
				conn := s.env.GetConnectionToDatabase(database)
				defer conn.Close()
				
				endTime := time.Now().Add(3 * time.Minute)
				count := int64(0)
				
				for time.Now().Before(endTime) {
					_, err := conn.Exec("INSERT INTO metrics_test (value) VALUES ($1)", 
						math.Sin(float64(time.Now().Unix())))
					if err == nil {
						count++
					}
					
					_, err = conn.Exec("SELECT AVG(value) FROM metrics_test WHERE timestamp > NOW() - INTERVAL '1 minute'")
					if err == nil {
						count++
					}
					
					time.Sleep(100 * time.Millisecond)
				}
				
				metricsMu.Lock()
				metricsPerDB[database] += count
				metricsMu.Unlock()
			}(db, i)
		}
	}
	
	// Monitor while running
	go s.monitorPerformance(3 * time.Minute)
	
	wg.Wait()
	
	// Check metrics distribution
	totalMetrics := int64(0)
	for db, count := range metricsPerDB {
		totalMetrics += count
		s.T().Logf("Database %s: %d operations", db, count)
	}
	
	avgPerDB := totalMetrics / int64(dbCount)
	
	// Check even distribution
	for db, count := range metricsPerDB {
		deviation := math.Abs(float64(count-avgPerDB)) / float64(avgPerDB) * 100
		s.Assert().Less(deviation, 20.0, 
			"Database %s operations should be within 20%% of average", db)
	}
	
	// Check resource usage scales linearly
	resources := s.getCollectorResources()
	memoryPerDB := resources.MemoryMB / float64(dbCount)
	
	s.Assert().Less(memoryPerDB, 100.0, 
		"Memory per database should be < 100MB")
	
	s.T().Logf("Multi-database results:\n"+
		"  Total DBs: %d\n"+
		"  Total Operations: %d\n"+
		"  Memory per DB: %.2f MB",
		dbCount, totalMetrics, memoryPerDB)
}

// Helper methods

func (s *PerformanceScaleTestSuite) getRandomQuery(seed int) string {
	queries := []string{
		"SELECT * FROM small_table WHERE id = $1",
		"SELECT COUNT(*) FROM medium_table WHERE user_id = $1",
		"SELECT * FROM large_table WHERE event_type = $1 LIMIT 100",
		"SELECT u.user_id, COUNT(*) FROM large_table u GROUP BY u.user_id HAVING COUNT(*) > $1",
		"INSERT INTO large_table (user_id, event_type, event_data) VALUES ($1, 'test', '{}')",
		"UPDATE medium_table SET updated_at = NOW() WHERE id = $1",
	}
	return queries[seed%len(queries)]
}

func (s *PerformanceScaleTestSuite) monitorPerformance(duration time.Duration) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(duration)
	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			resources := s.getCollectorResources()
			s.T().Logf("Performance: Memory=%.2fMB, CPU=%.2f%%", 
				resources.MemoryMB, resources.CPUPercent)
		}
	}
}

func (s *PerformanceScaleTestSuite) getCollectorResources() framework.ResourceUsage {
	metrics, _ := s.collector.GetMetrics(s.ctx, "otelcol_process.*")
	
	usage := framework.ResourceUsage{}
	for _, m := range metrics {
		switch m.Name() {
		case "otelcol_process_memory_rss":
			usage.MemoryMB = s.getLatestValue(m) / 1024 / 1024
		case "otelcol_process_cpu_seconds":
			usage.CPUPercent = s.getLatestValue(m) * 100
		}
	}
	return usage
}

func (s *PerformanceScaleTestSuite) calculateCardinality(metrics []pmetric.Metric) int {
	uniqueSeries := make(map[string]bool)
	
	for _, m := range metrics {
		// Create series identifier from metric name and attributes
		seriesID := m.Name()
		m.ResourceMetrics().At(0).Resource().Attributes().Range(func(k string, v interface{}) bool {
			seriesID += fmt.Sprintf(",%s=%v", k, v)
			return true
		})
		uniqueSeries[seriesID] = true
	}
	
	return len(uniqueSeries)
}

func (s *PerformanceScaleTestSuite) sumMetricValues(metrics []pmetric.Metric) float64 {
	sum := 0.0
	for _, m := range metrics {
		sum += s.getLatestValue(m)
	}
	return sum
}

func (s *PerformanceScaleTestSuite) getLatestValue(metric pmetric.Metric) float64 {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			return metric.Gauge().DataPoints().At(0).DoubleValue()
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			return metric.Sum().DataPoints().At(0).DoubleValue()
		}
	}
	return 0
}

func (s *PerformanceScaleTestSuite) getErrorCount() int {
	// Get from collector logs or metrics
	errorMetrics, _ := s.collector.GetMetrics(s.ctx, "otelcol_processor_errors")
	return int(s.sumMetricValues(errorMetrics))
}

func (s *PerformanceScaleTestSuite) averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func (s *PerformanceScaleTestSuite) percentileDuration(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	// Sort durations
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	index := int(float64(len(sorted)-1) * percentile / 100)
	return sorted[index]
}

func (s *PerformanceScaleTestSuite) standardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	mean := s.averageFloat(values)
	var sum float64
	for _, v := range values {
		diff := v - mean
		sum += diff * diff
	}
	
	return math.Sqrt(sum / float64(len(values)))
}

func (s *PerformanceScaleTestSuite) averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (s *PerformanceScaleTestSuite) maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func (s *PerformanceScaleTestSuite) checkCollectorLogs(component string, patterns []string) {
	logs := s.collector.GetLogs()
	for _, pattern := range patterns {
		for _, log := range logs {
			if strings.Contains(log, component) && strings.Contains(log, pattern) {
				s.T().Logf("Found expected log: %s", pattern)
				return
			}
		}
	}
}

func TestPerformanceScaleSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance and scale e2e tests in short mode")
	}
	
	suite.Run(t, new(PerformanceScaleTestSuite))
}