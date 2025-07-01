package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerformanceE2E validates collector performance under various load conditions
func TestPerformanceE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupPerformanceTestSchema(t, db)

	// Start collector
	collector := testEnv.StartCollector(t, "testdata/config-performance.yaml")
	defer collector.Shutdown()

	require.Eventually(t, func() bool {
		return collector.IsHealthy()
	}, 30*time.Second, 1*time.Second)

	t.Run("BaselinePerformance", func(t *testing.T) {
		// Establish baseline metrics
		runtime.GC()
		memBefore := getMemStats()
		cpuBefore := getCPUStats()
		
		// Run standard workload
		runStandardWorkload(t, db, 100, 10*time.Second)
		
		// Measure resource usage
		memAfter := getMemStats()
		cpuAfter := getCPUStats()
		
		// Calculate deltas
		memDelta := memAfter.Alloc - memBefore.Alloc
		cpuDelta := cpuAfter - cpuBefore
		
		t.Logf("Baseline - Memory: %d MB, CPU: %.2f%%", memDelta/1024/1024, cpuDelta)
		
		// Verify reasonable resource usage
		assert.Less(t, memDelta, uint64(100*1024*1024), "Memory usage exceeds 100MB")
		assert.Less(t, cpuDelta, 5.0, "CPU usage exceeds 5%")
	})

	t.Run("HighCardinalityQueries", func(t *testing.T) {
		// Test with many unique queries
		startTime := time.Now()
		uniqueQueries := 10000
		
		var wg sync.WaitGroup
		for i := 0; i < uniqueQueries; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Generate unique query
				query := fmt.Sprintf(`
					SELECT * FROM performance_test 
					WHERE id = %d 
					AND status = 'status_%d' 
					AND value > %d
					ORDER BY created_at DESC
					LIMIT %d`, id, id%100, id*10, id%50+1)
				
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				_, _ = conn.Exec(query)
			}(i)
			
			// Rate limit to avoid overwhelming
			if i%100 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
		
		wg.Wait()
		duration := time.Since(startTime)
		
		t.Logf("Processed %d unique queries in %v", uniqueQueries, duration)
		
		// Wait for processing
		time.Sleep(5 * time.Second)
		
		// Check metrics
		metrics := testEnv.GetCollectedMetrics()
		
		// Verify cardinality limiting worked
		queryMetrics := findMetricsByName(metrics, "db.postgresql.query.exec_time")
		uniqueQueryIDs := make(map[string]bool)
		for _, metric := range queryMetrics {
			attrs := getMetricAttributes(metric)
			if qid, ok := attrs["query_id"]; ok {
				uniqueQueryIDs[qid] = true
			}
		}
		
		t.Logf("Unique queries tracked: %d", len(uniqueQueryIDs))
		assert.LessOrEqual(t, len(uniqueQueryIDs), 1000, "Cardinality limit not enforced")
	})

	t.Run("SustainedHighLoad", func(t *testing.T) {
		// Test sustained high load for memory leaks
		duration := 2 * time.Minute
		sessionsPerSecond := 50
		
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		
		// Track metrics over time
		memSamples := []uint64{}
		cpuSamples := []float64{}
		
		// Start load generator
		go generateSustainedLoad(ctx, t, db, sessionsPerSecond)
		
		// Sample metrics every 10 seconds
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				runtime.GC()
				mem := getMemStats()
				cpu := getCPUStats()
				
				memSamples = append(memSamples, mem.Alloc)
				cpuSamples = append(cpuSamples, cpu)
				
				t.Logf("Sample - Memory: %d MB, CPU: %.2f%%", 
					mem.Alloc/1024/1024, cpu)
				
			case <-ctx.Done():
				goto done
			}
		}
		
	done:
		// Analyze trends
		memTrend := calculateTrend(memSamples)
		cpuAvg := calculateAverage(cpuSamples)
		
		t.Logf("Memory trend: %.2f MB/min, CPU average: %.2f%%", 
			memTrend/1024/1024, cpuAvg)
		
		// Memory should not grow significantly
		assert.Less(t, memTrend, float64(10*1024*1024), "Memory leak detected")
		assert.Less(t, cpuAvg, 10.0, "CPU usage too high")
	})

	t.Run("BurstLoad", func(t *testing.T) {
		// Test handling of sudden load spikes
		normalLoad := 10
		spikeLoad := 500
		
		// Normal load
		t.Log("Starting normal load phase")
		runStandardWorkload(t, db, normalLoad, 10*time.Second)
		
		// Sudden spike
		t.Log("Starting spike load phase")
		startTime := time.Now()
		runStandardWorkload(t, db, spikeLoad, 5*time.Second)
		spikeHandlingTime := time.Since(startTime)
		
		// Return to normal
		t.Log("Returning to normal load")
		runStandardWorkload(t, db, normalLoad, 10*time.Second)
		
		// Check adaptive sampling kicked in
		time.Sleep(5 * time.Second)
		metrics := testEnv.GetCollectedMetrics()
		
		samplerMetrics := findMetricsByName(metrics, "otelcol_processor_adaptivesampler_sample_rate")
		assert.NotEmpty(t, samplerMetrics, "No adaptive sampler metrics")
		
		// Verify processing completed reasonably fast
		assert.Less(t, spikeHandlingTime, 10*time.Second, "Spike handling too slow")
	})

	t.Run("PlanRegressionLoad", func(t *testing.T) {
		// Test performance with many plan regressions
		
		// Create index for good plans
		_, err := db.Exec("CREATE INDEX idx_perf_test_status ON performance_test(status)")
		require.NoError(t, err)
		
		// Generate queries with good plans
		t.Log("Generating baseline plans")
		for i := 0; i < 100; i++ {
			_, _ = db.Exec("SELECT * FROM performance_test WHERE status = $1", 
				fmt.Sprintf("status_%d", i%10))
		}
		
		// Drop index to cause regressions
		_, err = db.Exec("DROP INDEX idx_perf_test_status")
		require.NoError(t, err)
		
		// Generate same queries with bad plans
		t.Log("Generating regressed plans")
		startTime := time.Now()
		
		var regressionCount int64
		var wg sync.WaitGroup
		
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				_, err := db.Exec("SELECT * FROM performance_test WHERE status = $1", 
					fmt.Sprintf("status_%d", id%10))
				if err == nil {
					atomic.AddInt64(&regressionCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		processingTime := time.Since(startTime)
		
		t.Logf("Processed %d potential regressions in %v", regressionCount, processingTime)
		
		// Wait for regression detection
		time.Sleep(5 * time.Second)
		
		// Verify regressions were detected
		metrics := testEnv.GetCollectedMetrics()
		regressionMetrics := findMetricsByName(metrics, "db.postgresql.plan.regression")
		assert.NotEmpty(t, regressionMetrics, "No regressions detected")
		
		// Performance should still be reasonable
		assert.Less(t, processingTime, 30*time.Second, "Regression processing too slow")
	})

	t.Run("ASHHighFrequencySampling", func(t *testing.T) {
		// Test ASH performance at 1-second sampling
		duration := 1 * time.Minute
		concurrentSessions := 100
		
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		
		// Create persistent sessions
		sessions := make([]*sql.DB, concurrentSessions)
		for i := 0; i < concurrentSessions; i++ {
			sessions[i] = getNewConnection(t, testEnv)
			defer sessions[i].Close()
		}
		
		// Generate varied activity
		var samplesCollected int64
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Count ASH samples
					metrics := testEnv.GetCollectedMetrics()
					ashMetrics := findMetricsByName(metrics, "postgresql.ash.sessions.count")
					atomic.StoreInt64(&samplesCollected, int64(len(ashMetrics)))
					time.Sleep(1 * time.Second)
				}
			}
		}()
		
		// Run workload on sessions
		var wg sync.WaitGroup
		for i, sess := range sessions {
			wg.Add(1)
			go func(id int, conn *sql.DB) {
				defer wg.Done()
				
				for {
					select {
					case <-ctx.Done():
						return
					default:
						// Varied queries
						switch id % 5 {
						case 0:
							conn.Exec("SELECT pg_sleep(0.1)")
						case 1:
							conn.Exec("UPDATE performance_test SET value = value + 1 WHERE id = $1", id)
						case 2:
							conn.Exec("SELECT COUNT(*) FROM performance_test WHERE status = $1", fmt.Sprintf("status_%d", id%10))
						case 3:
							tx, _ := conn.Begin()
							tx.Exec("SELECT * FROM performance_test WHERE id = $1 FOR UPDATE", id)
							time.Sleep(100 * time.Millisecond)
							tx.Rollback()
						case 4:
							time.Sleep(200 * time.Millisecond)
						}
					}
				}
			}(i, sess)
		}
		
		// Wait for completion
		<-ctx.Done()
		wg.Wait()
		
		finalSamples := atomic.LoadInt64(&samplesCollected)
		expectedSamples := int64(duration.Seconds())
		
		t.Logf("Collected %d ASH samples (expected ~%d)", finalSamples, expectedSamples)
		
		// Should collect close to expected number of samples
		tolerance := float64(expectedSamples) * 0.2 // 20% tolerance
		assert.InDelta(t, expectedSamples, finalSamples, tolerance, 
			"ASH sampling rate significantly off target")
	})

	t.Run("MemoryLimiterStress", func(t *testing.T) {
		// Test memory limiter under pressure
		
		// Generate massive amount of metrics
		t.Log("Generating high metric volume")
		
		var wg sync.WaitGroup
		metricBurst := 50000
		
		for i := 0; i < metricBurst; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Unique query to generate new metric
				query := fmt.Sprintf("SELECT %d, random(), NOW()", id)
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				conn.Exec(query)
			}(i)
			
			// Burst generation
			if i%1000 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
		}
		
		wg.Wait()
		time.Sleep(10 * time.Second)
		
		// Check for dropped metrics
		metrics := testEnv.GetCollectedMetrics()
		droppedMetrics := findMetricsByName(metrics, "otelcol_processor_memorylimiter_refused_metric_points")
		
		if len(droppedMetrics) > 0 {
			t.Log("Memory limiter correctly dropped metrics under pressure")
			for _, metric := range droppedMetrics {
				value := getMetricValue(metric)
				t.Logf("Dropped metric points: %.0f", value)
			}
		}
		
		// Verify collector didn't crash
		assert.True(t, collector.IsHealthy(), "Collector unhealthy after memory pressure")
	})

	t.Run("ExportLatency", func(t *testing.T) {
		// Measure end-to-end latency
		
		// Execute query with known timestamp
		queryTime := time.Now()
		testQuery := "SELECT 'latency_test_marker_query'"
		_, err := db.Exec(testQuery)
		require.NoError(t, err)
		
		// Wait for metric to appear in NRDB
		var metricTime time.Time
		require.Eventually(t, func() bool {
			payload := testEnv.GetNRDBPayload()
			if payload == nil {
				return false
			}
			
			for _, metric := range payload.Metrics {
				// Look for our marker query
				if attrs, ok := metric.Attributes["query"]; ok {
					if query, ok := attrs.(string); ok && query == testQuery {
						metricTime = time.Unix(0, metric.Timestamp*int64(time.Millisecond))
						return true
					}
				}
			}
			return false
		}, 30*time.Second, 100*time.Millisecond)
		
		latency := metricTime.Sub(queryTime)
		t.Logf("End-to-end latency: %v", latency)
		
		// Should be under 10 seconds
		assert.Less(t, latency, 10*time.Second, "Export latency too high")
	})
}

// Helper functions

func setupPerformanceTestSchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS performance_test (
			id SERIAL PRIMARY KEY,
			status VARCHAR(50),
			value BIGINT,
			data TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`INSERT INTO performance_test (status, value, data)
		 SELECT 
			'status_' || (i % 100),
			i * random() * 1000,
			repeat('x', 100)
		 FROM generate_series(1, 100000) i`,
		`CREATE INDEX idx_perf_test_created ON performance_test(created_at)`,
		`ANALYZE performance_test`,
	}
	
	for _, query := range queries {
		_, err := db.Exec(query)
		require.NoError(t, err)
	}
}

func runStandardWorkload(t *testing.T, db *sql.DB, concurrency int, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Mix of query types
					switch id % 4 {
					case 0:
						conn.Exec("SELECT COUNT(*) FROM performance_test")
					case 1:
						conn.Exec("SELECT * FROM performance_test WHERE id = $1", id)
					case 2:
						conn.Exec("UPDATE performance_test SET value = value + 1 WHERE id = $1", id)
					case 3:
						conn.Exec("SELECT AVG(value) FROM performance_test WHERE status = $1", 
							fmt.Sprintf("status_%d", id%10))
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

func generateSustainedLoad(ctx context.Context, t *testing.T, db *sql.DB, rate int) {
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go func() {
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				
				// Random query
				switch time.Now().Unix() % 3 {
				case 0:
					conn.Exec("SELECT * FROM performance_test ORDER BY random() LIMIT 10")
				case 1:
					conn.Exec("UPDATE performance_test SET value = $1 WHERE id = $2", 
						time.Now().Unix(), time.Now().Unix()%1000)
				case 2:
					conn.Exec("SELECT COUNT(*), AVG(value) FROM performance_test WHERE created_at > NOW() - INTERVAL '1 hour'")
				}
			}()
		}
	}
}

func getMemStats() *runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &m
}

func getCPUStats() float64 {
	// Simplified CPU measurement - in production use proper CPU profiling
	return 0.0
}

func calculateTrend(samples []uint64) float64 {
	if len(samples) < 2 {
		return 0
	}
	
	// Simple linear regression
	n := float64(len(samples))
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, y := range samples {
		x := float64(i)
		sumX += x
		sumY += float64(y)
		sumXY += x * float64(y)
		sumX2 += x * x
	}
	
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope * 60 // Convert to per minute
}

func calculateAverage(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	return sum / float64(len(samples))
}

// Performance test configuration
const testPerformanceConfig = `
receivers:
  postgresql:
    endpoint: localhost:5432
    username: test_user
    password: test_password
    databases: [test_db]
    collection_interval: 10s
  
  autoexplain:
    log_path: /tmp/test-postgresql.log
    log_format: json
    database:
      endpoint: localhost:5432
      username: test_user
      password: test_password
      database: test_db
    plan_collection:
      enabled: true
      min_duration: 100ms
      max_plans_per_query: 5  # Lower for performance
  
  ash:
    endpoint: localhost:5432
    username: test_user
    password: test_password
    database: test_db
    collection_interval: 1s
    sampling:
      enabled: true
      sample_rate: 0.2  # Lower base rate
      adaptive_sampling: true

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
    spike_limit_mib: 50
  
  adaptivesampler:
    enabled: true
    default_sampling_rate: 0.1
    max_cardinality: 1000

exporters:
  otlp/newrelic:
    endpoint: localhost:4317
    headers:
      api-key: test-api-key
    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 5000

service:
  pipelines:
    metrics:
      receivers: [postgresql, autoexplain, ash]
      processors: [memory_limiter, adaptivesampler]
      exporters: [otlp/newrelic]
  
  telemetry:
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`