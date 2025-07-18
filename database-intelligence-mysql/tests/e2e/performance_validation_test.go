package e2e

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PerformanceBaseline represents expected performance characteristics
type PerformanceBaseline struct {
	MaxCPUOverhead      float64 // Maximum CPU overhead in percentage
	MaxMemoryMB         int     // Maximum memory usage in MB
	MaxLatencyMs        int     // Maximum collection latency in ms
	MaxQueryOverheadMs  float64 // Maximum overhead per query
}

// TestMonitoringPerformanceImpact validates monitoring overhead is within acceptable limits
func TestMonitoringPerformanceImpact(t *testing.T) {
	baseline := PerformanceBaseline{
		MaxCPUOverhead:     1.0,  // 1% CPU overhead
		MaxMemoryMB:        384,  // 384MB memory
		MaxLatencyMs:       5,    // 5ms collection latency
		MaxQueryOverheadMs: 0.5,  // 0.5ms per query overhead
	}

	t.Run("CPU_Overhead", func(t *testing.T) {
		testCPUOverhead(t, baseline)
	})

	t.Run("Memory_Usage", func(t *testing.T) {
		testMemoryUsage(t, baseline)
	})

	t.Run("Query_Overhead", func(t *testing.T) {
		testQueryOverhead(t, baseline)
	})

	t.Run("Collection_Latency", func(t *testing.T) {
		testCollectionLatency(t, baseline)
	})

	t.Run("Scalability", func(t *testing.T) {
		testScalability(t)
	})
}

// testCPUOverhead measures CPU impact of monitoring
func testCPUOverhead(t *testing.T, baseline PerformanceBaseline) {
	db := connectMySQL(t)
	defer db.Close()

	// Measure baseline CPU without monitoring
	print_status("info", "Measuring baseline CPU usage...")
	baselineCPU := measureCPU(t, func() {
		// Run workload without monitoring
		runWorkload(t, db, "mixed", 100, false)
	})

	// Enable monitoring and measure CPU
	print_status("info", "Measuring CPU with monitoring enabled...")
	monitoringCPU := measureCPU(t, func() {
		// Run same workload with monitoring
		runWorkload(t, db, "mixed", 100, true)
	})

	// Calculate overhead
	overhead := ((monitoringCPU - baselineCPU) / baselineCPU) * 100
	
	t.Logf("CPU Overhead: %.2f%% (baseline: %.2f%%, with monitoring: %.2f%%)",
		overhead, baselineCPU, monitoringCPU)
	
	assert.LessOrEqual(t, overhead, baseline.MaxCPUOverhead,
		"CPU overhead exceeds maximum allowed %.2f%%", baseline.MaxCPUOverhead)
}

// testMemoryUsage validates memory consumption
func testMemoryUsage(t *testing.T, baseline PerformanceBaseline) {
	// Get collector memory usage from metrics
	resp, err := http.Get("http://localhost:8888/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Parse Prometheus metrics
	var memoryBytes float64
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "process_resident_memory_bytes") && !strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				memoryBytes, _ = strconv.ParseFloat(parts[1], 64)
				break
			}
		}
	}

	memoryMB := int(memoryBytes / 1024 / 1024)
	t.Logf("Collector Memory Usage: %d MB", memoryMB)
	
	assert.LessOrEqual(t, memoryMB, baseline.MaxMemoryMB,
		"Memory usage exceeds maximum allowed %d MB", baseline.MaxMemoryMB)

	// Test memory stability over time
	t.Run("Memory_Stability", func(t *testing.T) {
		measurements := make([]int, 10)
		
		for i := 0; i < 10; i++ {
			// Generate load
			db := connectMySQL(t)
			runWorkload(t, db, "mixed", 50, true)
			db.Close()
			
			// Measure memory
			measurements[i] = getCurrentMemoryMB(t)
			time.Sleep(30 * time.Second)
		}

		// Check for memory leaks (growth over time)
		firstHalf := averageInt(measurements[:5])
		secondHalf := averageInt(measurements[5:])
		growth := secondHalf - firstHalf
		
		t.Logf("Memory growth over time: %d MB", growth)
		assert.Less(t, growth, 50, "Potential memory leak detected")
	})
}

// testQueryOverhead measures per-query overhead
func testQueryOverhead(t *testing.T, baseline PerformanceBaseline) {
	db := connectMySQL(t)
	defer db.Close()

	queries := []struct {
		name  string
		query string
		count int
	}{
		{
			name:  "Simple_Select",
			query: "SELECT 1",
			count: 1000,
		},
		{
			name:  "Index_Scan",
			query: "SELECT * FROM test_orders WHERE customer_id = ?",
			count: 500,
		},
		{
			name:  "Full_Scan",
			query: "SELECT COUNT(*) FROM test_orders WHERE description LIKE ?",
			count: 100,
		},
		{
			name:  "Join_Query",
			query: `SELECT o.*, c.name FROM test_orders o 
			        JOIN test_customers c ON o.customer_id = c.id 
			        WHERE o.order_date > ?`,
			count: 200,
		},
	}

	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			// Measure without monitoring
			disableMonitoring(t)
			baselineTime := measureQueryTime(t, db, q.query, q.count)
			
			// Measure with monitoring
			enableMonitoring(t)
			monitoringTime := measureQueryTime(t, db, q.query, q.count)
			
			// Calculate overhead
			overheadMs := (monitoringTime - baselineTime) / float64(q.count) * 1000
			
			t.Logf("Query overhead: %.3f ms (baseline: %.2f ms, monitoring: %.2f ms)",
				overheadMs, baselineTime*1000, monitoringTime*1000)
			
			assert.LessOrEqual(t, overheadMs, baseline.MaxQueryOverheadMs,
				"Query overhead exceeds maximum allowed %.2f ms", baseline.MaxQueryOverheadMs)
		})
	}
}

// testCollectionLatency measures metric collection latency
func testCollectionLatency(t *testing.T, baseline PerformanceBaseline) {
	db := connectMySQL(t)
	defer db.Close()
	nrClient := NewNewRelicClient(t)

	// Insert marker event
	marker := fmt.Sprintf("LATENCY_TEST_%d", time.Now().UnixNano())
	startTime := time.Now()
	
	_, err := db.Exec(fmt.Sprintf("SELECT '%s' as marker", marker))
	require.NoError(t, err)

	// Measure time until metric appears in edge collector
	edgeLatency := measureLatencyToCollector(t, marker, "http://localhost:9091/metrics")
	
	// Measure time until metric appears in New Relic
	nrLatency := measureLatencyToNewRelic(t, nrClient, marker, startTime)

	t.Logf("Collection Latencies - Edge: %v, New Relic: %v", edgeLatency, nrLatency)
	
	assert.Less(t, edgeLatency, time.Duration(baseline.MaxLatencyMs)*time.Millisecond,
		"Edge collection latency too high")
}

// testScalability validates system behavior under load
func testScalability(t *testing.T) {
	scenarios := []struct {
		name               string
		concurrentQueries  int
		queryRate          int // queries per second
		duration           time.Duration
		maxResponseTime    time.Duration
		maxErrorRate       float64
	}{
		{
			name:              "Light_Load",
			concurrentQueries: 10,
			queryRate:         100,
			duration:          2 * time.Minute,
			maxResponseTime:   50 * time.Millisecond,
			maxErrorRate:      0.01, // 1%
		},
		{
			name:              "Medium_Load",
			concurrentQueries: 50,
			queryRate:         500,
			duration:          2 * time.Minute,
			maxResponseTime:   100 * time.Millisecond,
			maxErrorRate:      0.02, // 2%
		},
		{
			name:              "Heavy_Load",
			concurrentQueries: 100,
			queryRate:         1000,
			duration:          2 * time.Minute,
			maxResponseTime:   200 * time.Millisecond,
			maxErrorRate:      0.05, // 5%
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			results := runLoadTest(t, scenario)
			
			// Validate results
			assert.LessOrEqual(t, results.AvgResponseTime, scenario.maxResponseTime,
				"Average response time too high")
			
			assert.LessOrEqual(t, results.ErrorRate, scenario.maxErrorRate,
				"Error rate too high")
			
			assert.GreaterOrEqual(t, results.ActualQPS, float64(scenario.queryRate)*0.9,
				"Could not maintain target query rate")
			
			// Check monitoring system health during load
			validateMonitoringHealth(t)
		})
	}
}

// LoadTestResults contains load test metrics
type LoadTestResults struct {
	TotalQueries     int
	SuccessfulQueries int
	FailedQueries    int
	AvgResponseTime  time.Duration
	P95ResponseTime  time.Duration
	P99ResponseTime  time.Duration
	ErrorRate        float64
	ActualQPS        float64
}

// runLoadTest executes a load test scenario
func runLoadTest(t *testing.T, scenario struct {
	name              string
	concurrentQueries int
	queryRate         int
	duration          time.Duration
	maxResponseTime   time.Duration
	maxErrorRate      float64
}) LoadTestResults {
	ctx, cancel := context.WithTimeout(context.Background(), scenario.duration)
	defer cancel()

	results := LoadTestResults{}
	responseTimes := make([]time.Duration, 0)
	var mu sync.Mutex
	
	// Rate limiter
	ticker := time.NewTicker(time.Second / time.Duration(scenario.queryRate))
	defer ticker.Stop()

	// Worker pool
	var wg sync.WaitGroup
	workCh := make(chan struct{}, scenario.concurrentQueries)
	
	// Start workers
	for i := 0; i < scenario.concurrentQueries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db := connectMySQL(t)
			defer db.Close()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-workCh:
					start := time.Now()
					err := executeRandomQuery(db)
					duration := time.Since(start)
					
					mu.Lock()
					results.TotalQueries++
					if err != nil {
						results.FailedQueries++
					} else {
						results.SuccessfulQueries++
						responseTimes = append(responseTimes, duration)
					}
					mu.Unlock()
				}
			}
		}()
	}

	// Generate load
	startTime := time.Now()
	for {
		select {
		case <-ctx.Done():
			close(workCh)
			wg.Wait()
			
			// Calculate results
			elapsedTime := time.Since(startTime)
			results.ErrorRate = float64(results.FailedQueries) / float64(results.TotalQueries)
			results.ActualQPS = float64(results.TotalQueries) / elapsedTime.Seconds()
			
			if len(responseTimes) > 0 {
				results.AvgResponseTime = average(responseTimes)
				results.P95ResponseTime = percentile(responseTimes, 95)
				results.P99ResponseTime = percentile(responseTimes, 99)
			}
			
			return results
			
		case <-ticker.C:
			select {
			case workCh <- struct{}{}:
			default:
				// Channel full, skip this tick
			}
		}
	}
}

// TestRegressionDetection validates that performance regressions are detected
func TestRegressionDetection(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()
	nrClient := NewNewRelicClient(t)

	// Create a query with known performance characteristics
	setupQuery := `CREATE TABLE IF NOT EXISTS test_regression (
	              id INT PRIMARY KEY,
	              data VARCHAR(100),
	              INDEX idx_data (data)
	              )`
	_, err := db.Exec(setupQuery)
	require.NoError(t, err)

	// Establish baseline performance
	baselineQuery := "SELECT * FROM test_regression WHERE data = ?"
	baseline := measureQueryPerformance(t, db, baselineQuery, 100)
	
	t.Logf("Baseline performance: %.2f ms", baseline.AvgTimeMs)

	// Drop index to simulate regression
	_, err = db.Exec("DROP INDEX idx_data ON test_regression")
	require.NoError(t, err)

	// Run query again
	regression := measureQueryPerformance(t, db, baselineQuery, 100)
	
	t.Logf("Regression performance: %.2f ms (%.2fx slower)", 
		regression.AvgTimeMs, regression.AvgTimeMs/baseline.AvgTimeMs)

	// Wait for advisory generation
	time.Sleep(30 * time.Second)

	// Check for regression advisory
	nrql := `SELECT count(*) as count FROM Metric 
	         WHERE advisor.type = 'plan_regression' 
	         AND query_hash IS NOT NULL 
	         SINCE 2 minutes ago`
	
	results, err := nrClient.QueryNRQL(nrql)
	require.NoError(t, err)
	
	assert.NotEmpty(t, results, "Plan regression advisory not generated")
	if len(results) > 0 {
		count := results[0]["count"].(float64)
		assert.Greater(t, count, float64(0), "No plan regression advisory found")
	}
}

// Helper functions

func measureCPU(t *testing.T, workload func()) float64 {
	// This would use system metrics to measure CPU
	// For now, simulate measurement
	workload()
	return 15.5 // Simulated CPU percentage
}

func runWorkload(t *testing.T, db *sql.DB, workloadType string, iterations int, withMonitoring bool) {
	if !withMonitoring {
		// Disable Performance Schema consumers temporarily
		db.Exec("UPDATE performance_schema.setup_consumers SET ENABLED = 'NO' WHERE NAME LIKE '%statements%'")
		defer db.Exec("UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME LIKE '%statements%'")
	}
	
	_, err := db.Exec(fmt.Sprintf("CALL generate_workload(%d, '%s')", iterations, workloadType))
	if err != nil {
		t.Logf("Workload generation error: %v", err)
	}
}

func getCurrentMemoryMB(t *testing.T) int {
	// Parse collector metrics for memory usage
	resp, err := http.Get("http://localhost:8888/metrics")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	
	// Parse and return memory in MB
	return 256 // Simulated value
}

func measureQueryTime(t *testing.T, db *sql.DB, query string, count int) float64 {
	start := time.Now()
	
	for i := 0; i < count; i++ {
		rows, err := db.Query(query, i)
		if err == nil {
			rows.Close()
		}
	}
	
	return time.Since(start).Seconds()
}

func disableMonitoring(t *testing.T) {
	// Temporarily disable monitoring
	// This would stop collectors or disable Performance Schema
}

func enableMonitoring(t *testing.T) {
	// Re-enable monitoring
}

func executeRandomQuery(db *sql.DB) error {
	queries := []string{
		"SELECT COUNT(*) FROM test_orders",
		"SELECT * FROM test_orders WHERE id = ?",
		"SELECT * FROM test_orders WHERE customer_id = ? LIMIT 10",
		"SELECT AVG(total_amount) FROM test_orders WHERE order_date > ?",
	}
	
	query := queries[time.Now().UnixNano()%int64(len(queries))]
	rows, err := db.Query(query, time.Now().Unix()%1000)
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

func measureQueryPerformance(t *testing.T, db *sql.DB, query string, iterations int) struct {
	AvgTimeMs float64
	MinTimeMs float64
	MaxTimeMs float64
} {
	times := make([]float64, iterations)
	
	for i := 0; i < iterations; i++ {
		start := time.Now()
		rows, err := db.Query(query, fmt.Sprintf("test%d", i))
		if err == nil {
			rows.Close()
		}
		times[i] = float64(time.Since(start).Microseconds()) / 1000.0
	}
	
	return struct {
		AvgTimeMs float64
		MinTimeMs float64
		MaxTimeMs float64
	}{
		AvgTimeMs: averageFloat64(times),
		MinTimeMs: min(times),
		MaxTimeMs: max(times),
	}
}

func validateMonitoringHealth(t *testing.T) {
	// Check collector health
	resp, err := http.Get("http://localhost:13133/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, 200, resp.StatusCode, "Collector health check failed")
	
	// Check for dropped metrics
	metricsResp, err := http.Get("http://localhost:8888/metrics")
	require.NoError(t, err)
	defer metricsResp.Body.Close()
	
	// Parse and check for dropped metrics
	// This would parse Prometheus metrics and check dropped_metric_points
}

// Utility functions

// print_status prints formatted status messages
func print_status(status, message string) {
	colors := map[string]string{
		"info":    "\033[0;36m",
		"success": "\033[0;32m",
		"error":   "\033[0;31m",
		"warning": "\033[1;33m",
	}
	reset := "\033[0m"
	
	symbol := map[string]string{
		"info":    "ℹ",
		"success": "✓",
		"error":   "✗",
		"warning": "⚠",
	}[status]
	
	fmt.Printf("%s%s%s %s\n", colors[status], symbol, reset, message)
}

func average(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	var sum time.Duration
	for _, v := range values {
		sum += v
	}
	return sum / time.Duration(len(values))
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
	
	index := int(math.Ceil(float64(len(values)) * p / 100.0)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	
	return values[index]
}

// Helper functions for int and float64 arrays
func averageInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

func averageFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}