package performance

import (
	"context"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryEfficiency validates memory usage patterns
func TestMemoryEfficiency(t *testing.T) {
	tests := []struct {
		name           string
		metricCount    int
		expectedMaxMB  float64
		processorCount int
	}{
		{
			name:           "Small_Batch",
			metricCount:    100,
			expectedMaxMB:  10,
			processorCount: 4,
		},
		{
			name:           "Medium_Batch",
			metricCount:    1000,
			expectedMaxMB:  50,
			processorCount: 4,
		},
		{
			name:           "Large_Batch",
			metricCount:    10000,
			expectedMaxMB:  200,
			processorCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Force GC before test
			runtime.GC()
			time.Sleep(100 * time.Millisecond)
			
			// Record baseline memory
			var baselineStats runtime.MemStats
			runtime.ReadMemStats(&baselineStats)
			
			// Create processors
			pipeline := createTestPipeline(t)
			
			// Process metrics
			metrics := generateTestMetrics(tt.metricCount)
			ctx := context.Background()
			
			// Run multiple iterations to detect leaks
			for i := 0; i < 10; i++ {
				_, err := processMetricsThroughPipeline(ctx, pipeline, metrics)
				require.NoError(t, err)
			}
			
			// Force GC and measure
			runtime.GC()
			time.Sleep(100 * time.Millisecond)
			
			var endStats runtime.MemStats
			runtime.ReadMemStats(&endStats)
			
			// Calculate memory growth
			memGrowthMB := float64(endStats.Alloc-baselineStats.Alloc) / 1024 / 1024
			
			t.Logf("Memory growth: %.2f MB (limit: %.2f MB)", memGrowthMB, tt.expectedMaxMB)
			assert.Less(t, memGrowthMB, tt.expectedMaxMB, 
				"Memory usage exceeded expected maximum")
		})
	}
}

// TestCPUEfficiency validates CPU usage patterns
func TestCPUEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU efficiency test in short mode")
	}

	// Start CPU profiling
	cpuFile, err := os.Create("cpu_profile.prof")
	require.NoError(t, err)
	defer cpuFile.Close()

	err = pprof.StartCPUProfile(cpuFile)
	require.NoError(t, err)
	defer pprof.StopCPUProfile()

	// Create pipeline
	pipeline := createTestPipeline(t)
	ctx := context.Background()

	// Measure processing time
	start := time.Now()
	totalProcessed := 0

	// Process for 30 seconds
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-timeout:
			goto done
		default:
			metrics := generateTestMetrics(100)
			processed, err := processMetricsThroughPipeline(ctx, pipeline, metrics)
			require.NoError(t, err)
			totalProcessed += processed
		}
	}

done:
	duration := time.Since(start)
	throughput := float64(totalProcessed) / duration.Seconds()

	t.Logf("Processed %d metrics in %v", totalProcessed, duration)
	t.Logf("Throughput: %.2f metrics/sec", throughput)

	// Verify minimum throughput
	assert.Greater(t, throughput, 5000.0, "Throughput should exceed 5000 metrics/sec")
}

// TestGoroutineLeaks checks for goroutine leaks
func TestGoroutineLeaks(t *testing.T) {
	// Record baseline goroutines
	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()

	// Run test workload
	t.Run("Workload", func(t *testing.T) {
		pipeline := createTestPipeline(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						metrics := generateTestMetrics(100)
						processMetricsThroughPipeline(ctx, pipeline, metrics)
						time.Sleep(10 * time.Millisecond)
					}
				}
			}()
		}

		wg.Wait()
	})

	// Allow goroutines to clean up
	time.Sleep(2 * time.Second)
	runtime.GC()

	// Check for leaks
	endGoroutines := runtime.NumGoroutine()
	leaked := endGoroutines - baselineGoroutines

	t.Logf("Baseline goroutines: %d", baselineGoroutines)
	t.Logf("End goroutines: %d", endGoroutines)
	t.Logf("Leaked goroutines: %d", leaked)

	assert.LessOrEqual(t, leaked, 5, "Should not leak more than 5 goroutines")
}

// TestResourceLimits validates behavior at resource limits
func TestResourceLimits(t *testing.T) {
	tests := []struct {
		name          string
		memoryLimitMB int
		testFunc      func(t *testing.T)
	}{
		{
			name:          "Low_Memory",
			memoryLimitMB: 64,
			testFunc:      testLowMemoryScenario,
		},
		{
			name:          "High_Cardinality",
			memoryLimitMB: 128,
			testFunc:      testHighCardinalityScenario,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: In production, you'd use cgroups or container limits
			// This is a simplified test
			tt.testFunc(t)
		})
	}
}

func testLowMemoryScenario(t *testing.T) {
	// Skip this test as it uses outdated APIs
	t.Skip("Low memory scenario test needs to be rewritten to use OTEL factory pattern")
}

func testHighCardinalityScenario(t *testing.T) {
	// Skip this test as it uses outdated APIs
	t.Skip("High cardinality scenario test needs to be rewritten to use OTEL factory pattern")
}

// TestConcurrentProcessing validates thread safety and concurrent performance
func TestConcurrentProcessing(t *testing.T) {
	pipeline := createTestPipeline(t)
	ctx := context.Background()

	// Number of concurrent processors
	concurrency := runtime.NumCPU() * 2
	metricsPerProcessor := 1000

	// Synchronization
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan int, concurrency)

	// Start concurrent processors
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Wait for start signal
			<-start

			// Process metrics
			metrics := generateTestMetrics(metricsPerProcessor)
			processed, err := processMetricsThroughPipeline(ctx, pipeline, metrics)
			if err != nil {
				t.Errorf("Processor %d error: %v", id, err)
				return
			}

			results <- processed
		}(i)
	}

	// Start all processors simultaneously
	startTime := time.Now()
	close(start)

	// Wait for completion
	wg.Wait()
	close(results)

	duration := time.Since(startTime)

	// Collect results
	totalProcessed := 0
	for processed := range results {
		totalProcessed += processed
	}

	throughput := float64(totalProcessed) / duration.Seconds()
	t.Logf("Concurrent processing:")
	t.Logf("  Concurrency: %d", concurrency)
	t.Logf("  Total processed: %d", totalProcessed)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Throughput: %.2f metrics/sec", throughput)

	// Verify all metrics were processed
	expectedTotal := concurrency * metricsPerProcessor
	assert.GreaterOrEqual(t, totalProcessed, int(float64(expectedTotal)*0.95),
		"Should process at least 95% of metrics")
}

// BenchmarkResourceMemoryAllocations profiles memory allocations for resource usage
func BenchmarkResourceMemoryAllocations(b *testing.B) {
	// Skip this benchmark as it uses outdated APIs
	b.Skip("Resource memory allocations benchmark needs to be rewritten to use OTEL factory pattern")
}


// TestResourceMonitoring validates resource monitoring accuracy
func TestResourceMonitoring(t *testing.T) {
	// Create a resource monitor
	monitor := &ResourceMonitor{
		interval: 100 * time.Millisecond,
		samples:  make([]ResourceSample, 0),
	}

	// Start monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	monitor.Start(ctx)

	// Run workload
	pipeline := createTestPipeline(t)
	for i := 0; i < 50; i++ {
		metrics := generateTestMetrics(1000)
		processMetricsThroughPipeline(ctx, pipeline, metrics)
		time.Sleep(50 * time.Millisecond)
	}

	// Stop monitoring
	monitor.Stop()

	// Analyze results
	stats := monitor.GetStats()
	t.Logf("Resource usage statistics:")
	t.Logf("  Samples: %d", stats.SampleCount)
	t.Logf("  Avg Memory: %.2f MB", stats.AvgMemoryMB)
	t.Logf("  Max Memory: %.2f MB", stats.MaxMemoryMB)
	t.Logf("  Avg Goroutines: %d", stats.AvgGoroutines)
	t.Logf("  Max Goroutines: %d", stats.MaxGoroutines)

	// Verify monitoring worked
	assert.Greater(t, stats.SampleCount, 10, "Should have collected samples")
	assert.Greater(t, stats.AvgMemoryMB, 0.0, "Should have memory measurements")
}

// ResourceMonitor tracks resource usage over time
type ResourceMonitor struct {
	interval time.Duration
	samples  []ResourceSample
	mu       sync.Mutex
	stop     chan struct{}
}

type ResourceSample struct {
	Timestamp  time.Time
	MemoryMB   float64
	Goroutines int
}

type ResourceStats struct {
	SampleCount   int
	AvgMemoryMB   float64
	MaxMemoryMB   float64
	AvgGoroutines int
	MaxGoroutines int
}

func (m *ResourceMonitor) Start(ctx context.Context) {
	m.stop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stop:
				return
			case <-ticker.C:
				sample := ResourceSample{
					Timestamp:  time.Now(),
					MemoryMB:   getMemoryUsage(),
					Goroutines: runtime.NumGoroutine(),
				}

				m.mu.Lock()
				m.samples = append(m.samples, sample)
				m.mu.Unlock()
			}
		}
	}()
}

func (m *ResourceMonitor) Stop() {
	close(m.stop)
}

func (m *ResourceMonitor) GetStats() ResourceStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.samples) == 0 {
		return ResourceStats{}
	}

	stats := ResourceStats{
		SampleCount: len(m.samples),
	}

	var totalMem float64
	var totalGoroutines int

	for _, s := range m.samples {
		totalMem += s.MemoryMB
		totalGoroutines += s.Goroutines

		if s.MemoryMB > stats.MaxMemoryMB {
			stats.MaxMemoryMB = s.MemoryMB
		}
		if s.Goroutines > stats.MaxGoroutines {
			stats.MaxGoroutines = s.Goroutines
		}
	}

	stats.AvgMemoryMB = totalMem / float64(len(m.samples))
	stats.AvgGoroutines = totalGoroutines / len(m.samples)

	return stats
}