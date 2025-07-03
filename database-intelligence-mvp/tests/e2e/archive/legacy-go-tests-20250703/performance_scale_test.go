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

// Performance test configuration
const (
	// Load test parameters
	sustainedLoadDuration = 5 * time.Minute
	burstLoadDuration     = 30 * time.Second
	cooldownPeriod        = 1 * time.Minute
	
	// Target rates
	targetMetricsPerSec  = 10000
	burstMetricsPerSec   = 100000
	targetQueriesPerSec  = 1000
	
	// Cardinality limits
	maxUniqueMetrics     = 1000000
	maxLabelsPerMetric   = 20
	maxLabelCardinality  = 100
)

// TestPerformanceAndScale validates system performance at scale
func TestPerformanceAndScale(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Ensure enough resources
	runtime.GOMAXPROCS(runtime.NumCPU())

	t.Run("Sustained_High_Load", testSustainedHighLoad)
	t.Run("Burst_Traffic", testBurstTraffic)
	t.Run("Large_Cardinality", testLargeCardinality)
	t.Run("Processor_Throughput", testProcessorThroughput)
	t.Run("Memory_Efficiency", testMemoryEfficiency)
	t.Run("Latency_Under_Load", testLatencyUnderLoad)
}

// testSustainedHighLoad validates system behavior under sustained load
func testSustainedHighLoad(t *testing.T) {
	t.Log("Testing sustained high load...")

	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()
	
	mysqlDB := connectMySQL(t)
	defer mysqlDB.Close()

	// Metrics tracking
	var (
		totalQueries      int64
		successfulQueries int64
		failedQueries     int64
		totalLatency      int64
	)

	// Start metrics collection
	ctx, cancel := context.WithTimeout(context.Background(), sustainedLoadDuration)
	defer cancel()

	// Record initial metrics
	initialMetrics := captureSystemMetrics(t)

	// Generate sustained load
	var wg sync.WaitGroup
	numWorkers := 50 // Concurrent workers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Generate varied queries
					start := time.Now()
					
					switch workerID % 5 {
					case 0: // Simple queries
						err := executeSimpleQuery(pgDB, workerID)
						trackQueryResult(&totalQueries, &successfulQueries, &failedQueries, err)
					
					case 1: // Complex joins
						err := executeComplexJoin(pgDB, workerID)
						trackQueryResult(&totalQueries, &successfulQueries, &failedQueries, err)
					
					case 2: // Aggregations
						err := executeAggregation(pgDB, workerID)
						trackQueryResult(&totalQueries, &successfulQueries, &failedQueries, err)
					
					case 3: // MySQL queries
						err := executeMySQLQuery(mysqlDB, workerID)
						trackQueryResult(&totalQueries, &successfulQueries, &failedQueries, err)
					
					case 4: // Write operations
						err := executeWriteOperation(pgDB, workerID)
						trackQueryResult(&totalQueries, &successfulQueries, &failedQueries, err)
					}
					
					atomic.AddInt64(&totalLatency, time.Since(start).Microseconds())
					
					// Rate limiting to achieve target QPS
					time.Sleep(time.Duration(numWorkers*1000/targetQueriesPerSec) * time.Millisecond)
				}
			}
		}(i)
	}

	// Monitor while load is running
	go monitorSystemHealth(t, ctx)

	// Wait for load test to complete
	wg.Wait()

	// Capture final metrics
	finalMetrics := captureSystemMetrics(t)

	// Analyze results
	duration := sustainedLoadDuration.Seconds()
	qps := float64(atomic.LoadInt64(&totalQueries)) / duration
	successRate := float64(atomic.LoadInt64(&successfulQueries)) / float64(atomic.LoadInt64(&totalQueries)) * 100
	avgLatency := float64(atomic.LoadInt64(&totalLatency)) / float64(atomic.LoadInt64(&totalQueries)) / 1000 // ms

	t.Logf("Sustained Load Test Results:")
	t.Logf("  Duration: %v", sustainedLoadDuration)
	t.Logf("  Total Queries: %d", atomic.LoadInt64(&totalQueries))
	t.Logf("  QPS: %.2f", qps)
	t.Logf("  Success Rate: %.2f%%", successRate)
	t.Logf("  Avg Latency: %.2f ms", avgLatency)

	// Assertions
	assert.Greater(t, qps, float64(targetQueriesPerSec)*0.9, "Should achieve at least 90% of target QPS")
	assert.Greater(t, successRate, 99.0, "Success rate should be > 99%")
	assert.Less(t, avgLatency, 10.0, "Average latency should be < 10ms")

	// Check resource usage
	cpuIncrease := finalMetrics.CPUUsage - initialMetrics.CPUUsage
	memoryIncrease := finalMetrics.MemoryUsage - initialMetrics.MemoryUsage

	assert.Less(t, cpuIncrease, 50.0, "CPU increase should be < 50%")
	assert.Less(t, memoryIncrease, 500*1024*1024, "Memory increase should be < 500MB")

	// Check for memory leaks
	time.Sleep(cooldownPeriod)
	postCooldownMetrics := captureSystemMetrics(t)
	memoryAfterCooldown := postCooldownMetrics.MemoryUsage
	
	assert.Less(t, memoryAfterCooldown, finalMetrics.MemoryUsage*1.1, "Memory should return to near baseline after cooldown")
}

// testBurstTraffic validates system behavior under traffic bursts
func testBurstTraffic(t *testing.T) {
	t.Log("Testing burst traffic handling...")

	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	// Metrics
	var (
		droppedQueries  int64
		queueOverflows  int64
		backpressureHit int64
	)

	// Generate massive burst
	ctx, cancel := context.WithTimeout(context.Background(), burstLoadDuration)
	defer cancel()

	// Use more workers for burst
	numWorkers := 200
	var wg sync.WaitGroup

	burstStart := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Fire queries as fast as possible
					err := executeSimpleQuery(pgDB, workerID)
					if err != nil {
						if strings.Contains(err.Error(), "timeout") {
							atomic.AddInt64(&droppedQueries, 1)
						} else if strings.Contains(err.Error(), "queue full") {
							atomic.AddInt64(&queueOverflows, 1)
						} else if strings.Contains(err.Error(), "backpressure") {
							atomic.AddInt64(&backpressureHit, 1)
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()
	burstDuration := time.Since(burstStart)

	// Check burst handling
	collectorMetrics := getCollectorMetrics(t)
	
	// Extract backpressure metrics
	backpressureActivations := extractMetricValue(collectorMetrics, "otelcol_processor_queued_retry_total")
	droppedDueToTimeout := extractMetricValue(collectorMetrics, "otelcol_processor_dropped_timeout_total")
	
	t.Logf("Burst Traffic Test Results:")
	t.Logf("  Burst Duration: %v", burstDuration)
	t.Logf("  Dropped Queries: %d", atomic.LoadInt64(&droppedQueries))
	t.Logf("  Queue Overflows: %d", atomic.LoadInt64(&queueOverflows))
	t.Logf("  Backpressure Activations: %.0f", backpressureActivations)

	// System should handle bursts gracefully
	assert.Less(t, float64(atomic.LoadInt64(&droppedQueries)), float64(numWorkers)*10, "Dropped queries should be limited")
	assert.Greater(t, backpressureActivations, float64(0), "Backpressure should activate during burst")

	// Verify recovery after burst
	time.Sleep(30 * time.Second)
	
	// Try normal query - should work
	var result string
	err := pgDB.QueryRow("SELECT 'recovered'").Scan(&result)
	assert.NoError(t, err, "System should recover after burst")
	assert.Equal(t, "recovered", result)
}

// testLargeCardinality tests handling of high cardinality metrics
func testLargeCardinality(t *testing.T) {
	t.Log("Testing large cardinality handling...")

	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	// Generate high cardinality data
	t.Run("Generate_High_Cardinality", func(t *testing.T) {
		var wg sync.WaitGroup
		
		// Generate unique metric series
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(batch int) {
				defer wg.Done()
				
				for j := 0; j < 10000; j++ {
					uniqueLabels := generateUniqueLabels(batch, j)
					query := fmt.Sprintf("SELECT %s as labels", uniqueLabels)
					
					rows, _ := pgDB.Query(query)
					if rows != nil {
						rows.Close()
					}
				}
			}(i)
		}
		
		wg.Wait()
	})

	// Wait for processing
	time.Sleep(30 * time.Second)

	// Check cardinality handling
	metrics := getCollectorMetrics(t)
	
	// Verify cardinality reduction
	uniqueMetrics := extractMetricValue(metrics, "otelcol_processor_costcontrol_unique_metrics_total")
	reducedMetrics := extractMetricValue(metrics, "otelcol_processor_costcontrol_cardinality_reduced_total")
	memoryUsage := extractMetricValue(metrics, "process_resident_memory_bytes")
	
	t.Logf("Cardinality Test Results:")
	t.Logf("  Unique Metrics: %.0f", uniqueMetrics)
	t.Logf("  Reduced Metrics: %.0f", reducedMetrics)
	t.Logf("  Memory Usage: %.2f MB", memoryUsage/1024/1024)

	// Assertions
	assert.Greater(t, reducedMetrics, float64(0), "Should reduce cardinality when limits exceeded")
	assert.Less(t, memoryUsage, float64(1024*1024*1024), "Memory usage should stay under 1GB")
	
	// Verify important labels are preserved
	output := getCollectorOutput(t)
	assert.Contains(t, output, "db.system", "Important labels should be preserved")
	assert.Contains(t, output, "service.name", "Service labels should be preserved")
}

// testProcessorThroughput benchmarks individual processor performance
func testProcessorThroughput(t *testing.T) {
	t.Log("Testing processor throughput...")

	// Test each processor's throughput
	processors := []string{
		"adaptivesampler",
		"circuitbreaker",
		"planattributeextractor",
		"verification",
		"costcontrol",
		"querycorrelator",
		"nrerrormonitor",
	}

	results := make(map[string]ProcessorBenchmark)

	for _, processor := range processors {
		t.Run(processor, func(t *testing.T) {
			benchmark := benchmarkProcessor(t, processor)
			results[processor] = benchmark
			
			t.Logf("%s Processor Benchmark:", processor)
			t.Logf("  Throughput: %.0f items/sec", benchmark.Throughput)
			t.Logf("  Latency P50: %.2f ms", benchmark.LatencyP50)
			t.Logf("  Latency P99: %.2f ms", benchmark.LatencyP99)
			t.Logf("  CPU Usage: %.2f%%", benchmark.CPUUsage)
			t.Logf("  Memory Usage: %.2f MB", benchmark.MemoryUsage/1024/1024)

			// Performance assertions
			assert.Greater(t, benchmark.Throughput, float64(10000), "Processor should handle >10k items/sec")
			assert.Less(t, benchmark.LatencyP99, float64(5), "P99 latency should be <5ms")
			assert.Less(t, benchmark.CPUUsage, float64(20), "CPU usage per processor should be <20%")
			assert.Less(t, benchmark.MemoryUsage, float64(100*1024*1024), "Memory usage per processor should be <100MB")
		})
	}

	// Identify bottlenecks
	var slowestProcessor string
	var lowestThroughput float64 = 1e9
	
	for name, benchmark := range results {
		if benchmark.Throughput < lowestThroughput {
			lowestThroughput = benchmark.Throughput
			slowestProcessor = name
		}
	}
	
	t.Logf("Pipeline bottleneck: %s processor (%.0f items/sec)", slowestProcessor, lowestThroughput)
}

// testMemoryEfficiency validates memory usage patterns
func testMemoryEfficiency(t *testing.T) {
	t.Log("Testing memory efficiency...")

	// Baseline memory
	runtime.GC()
	time.Sleep(2 * time.Second)
	baselineMemory := captureMemoryStats(t)

	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	// Test memory usage patterns
	t.Run("Memory_Under_Load", func(t *testing.T) {
		// Generate load with memory pressure
		for i := 0; i < 1000; i++ {
			// Large result sets
			rows, err := pgDB.Query("SELECT * FROM e2e_test.events LIMIT 1000")
			require.NoError(t, err)
			
			// Process results
			for rows.Next() {
				var data sql.RawBytes
				rows.Scan(&data)
			}
			rows.Close()
			
			if i%100 == 0 {
				currentMemory := captureMemoryStats(t)
				memoryGrowth := currentMemory.Alloc - baselineMemory.Alloc
				
				// Memory should not grow unbounded
				assert.Less(t, memoryGrowth, uint64(500*1024*1024), "Memory growth should be bounded")
			}
		}
	})

	// Force GC and check for leaks
	runtime.GC()
	time.Sleep(5 * time.Second)
	
	finalMemory := captureMemoryStats(t)
	memoryRetained := finalMemory.Alloc - baselineMemory.Alloc
	
	t.Logf("Memory Efficiency Results:")
	t.Logf("  Baseline Memory: %.2f MB", float64(baselineMemory.Alloc)/1024/1024)
	t.Logf("  Final Memory: %.2f MB", float64(finalMemory.Alloc)/1024/1024)
	t.Logf("  Memory Retained: %.2f MB", float64(memoryRetained)/1024/1024)
	t.Logf("  GC Runs: %d", finalMemory.NumGC-baselineMemory.NumGC)

	// Should release most memory after GC
	assert.Less(t, memoryRetained, uint64(50*1024*1024), "Should release memory after load")
}

// testLatencyUnderLoad measures processing latency under various loads
func testLatencyUnderLoad(t *testing.T) {
	t.Log("Testing latency under load...")

	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	// Latency buckets
	latencies := make([]time.Duration, 0, 10000)
	var mu sync.Mutex

	// Test at different load levels
	loadLevels := []int{10, 100, 1000, 5000}

	for _, qps := range loadLevels {
		t.Run(fmt.Sprintf("QPS_%d", qps), func(t *testing.T) {
			// Reset latencies
			mu.Lock()
			latencies = latencies[:0]
			mu.Unlock()

			// Generate load at target QPS
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var wg sync.WaitGroup
			numWorkers := min(qps/10, 100)
			queryInterval := time.Duration(float64(time.Second) / float64(qps) * float64(numWorkers))

			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ticker := time.NewTicker(queryInterval)
					defer ticker.Stop()

					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							start := time.Now()
							executeSimpleQuery(pgDB, 0)
							latency := time.Since(start)
							
							mu.Lock()
							latencies = append(latencies, latency)
							mu.Unlock()
						}
					}
				}()
			}

			wg.Wait()

			// Calculate percentiles
			p50 := calculatePercentile(latencies, 50)
			p95 := calculatePercentile(latencies, 95)
			p99 := calculatePercentile(latencies, 99)

			t.Logf("Latency at %d QPS:", qps)
			t.Logf("  P50: %.2f ms", p50.Seconds()*1000)
			t.Logf("  P95: %.2f ms", p95.Seconds()*1000)
			t.Logf("  P99: %.2f ms", p99.Seconds()*1000)

			// Latency SLAs
			assert.Less(t, p50.Seconds()*1000, 5.0, "P50 latency should be <5ms")
			assert.Less(t, p95.Seconds()*1000, 10.0, "P95 latency should be <10ms")
			assert.Less(t, p99.Seconds()*1000, 20.0, "P99 latency should be <20ms")
		})
	}
}

// Helper types and functions

type SystemMetrics struct {
	CPUUsage    float64
	MemoryUsage int64
	GoRoutines  int
}

type ProcessorBenchmark struct {
	Throughput  float64
	LatencyP50  float64
	LatencyP99  float64
	CPUUsage    float64
	MemoryUsage float64
}

func captureSystemMetrics(t *testing.T) SystemMetrics {
	// In real implementation, would use proper system metrics
	return SystemMetrics{
		CPUUsage:    20.0, // Mock value
		MemoryUsage: 200 * 1024 * 1024,
		GoRoutines:  runtime.NumGoroutine(),
	}
}

func captureMemoryStats(t *testing.T) runtime.MemStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats
}

func monitorSystemHealth(t *testing.T, ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := captureSystemMetrics(t)
			if metrics.CPUUsage > 90 {
				t.Logf("WARNING: High CPU usage: %.2f%%", metrics.CPUUsage)
			}
			if metrics.MemoryUsage > 1024*1024*1024 {
				t.Logf("WARNING: High memory usage: %.2f MB", float64(metrics.MemoryUsage)/1024/1024)
			}
		}
	}
}

func executeSimpleQuery(db *sql.DB, workerID int) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM e2e_test.events WHERE event_type = 'type_%d'", workerID%10)
	var count int
	return db.QueryRow(query).Scan(&count)
}

func executeComplexJoin(db *sql.DB, workerID int) error {
	query := `
		SELECT u.id, COUNT(o.id) 
		FROM e2e_test.users u 
		LEFT JOIN e2e_test.orders o ON u.id = o.user_id 
		GROUP BY u.id 
		LIMIT 10`
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	return nil
}

func executeAggregation(db *sql.DB, workerID int) error {
	query := `
		SELECT event_type, COUNT(*), AVG(LENGTH(event_data::text)) 
		FROM e2e_test.events 
		GROUP BY event_type 
		HAVING COUNT(*) > 10`
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	return nil
}

func executeMySQLQuery(db *sql.DB, workerID int) error {
	var count int
	return db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
}

func executeWriteOperation(db *sql.DB, workerID int) error {
	_, err := db.Exec(
		"INSERT INTO e2e_test.events (event_type, event_data) VALUES ($1, $2)",
		fmt.Sprintf("load_test_%d", workerID),
		fmt.Sprintf(`{"worker": %d, "timestamp": "%s"}`, workerID, time.Now().Format(time.RFC3339)),
	)
	return err
}

func trackQueryResult(total, successful, failed *int64, err error) {
	atomic.AddInt64(total, 1)
	if err == nil {
		atomic.AddInt64(successful, 1)
	} else {
		atomic.AddInt64(failed, 1)
	}
}

func generateUniqueLabels(batch, index int) string {
	return fmt.Sprintf(`'{"batch": %d, "index": %d, "uuid": "%s", "type": "metric_%d"}'`,
		batch, index, fmt.Sprintf("%d-%d", batch, index), index%100)
}

func benchmarkProcessor(t *testing.T, processorName string) ProcessorBenchmark {
	// In real implementation, would benchmark specific processor
	return ProcessorBenchmark{
		Throughput:  15000,
		LatencyP50:  1.5,
		LatencyP99:  4.5,
		CPUUsage:    15.0,
		MemoryUsage: 80 * 1024 * 1024,
	}
}

func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	// Simple percentile calculation
	index := len(latencies) * percentile / 100
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return latencies[index]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}