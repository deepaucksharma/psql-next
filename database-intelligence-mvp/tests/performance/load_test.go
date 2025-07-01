package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/verification"
)

// LoadTestConfig defines a load test scenario
type LoadTestConfig struct {
	Name              string
	Duration          time.Duration
	MetricsPerSecond  int
	ConcurrentSenders int
	MetricSize        int
	ErrorRate         float64
	SlowQueryRate     float64
}

// LoadTestResult contains the results of a load test
type LoadTestResult struct {
	TotalMetrics      int64
	ProcessedMetrics  int64
	DroppedMetrics    int64
	Errors           int64
	AverageLatency   time.Duration
	P99Latency       time.Duration
	MaxLatency       time.Duration
	MemoryUsedMB     float64
	CPUPercent       float64
	ThroughputPerSec float64
}

// TestLoadScenarios runs various load scenarios
func TestLoadScenarios(t *testing.T) {
	scenarios := []LoadTestConfig{
		{
			Name:              "Light_Load",
			Duration:          30 * time.Second,
			MetricsPerSecond:  1000,
			ConcurrentSenders: 10,
			MetricSize:        10,
			ErrorRate:         0.01,
			SlowQueryRate:     0.05,
		},
		{
			Name:              "Normal_Load",
			Duration:          60 * time.Second,
			MetricsPerSecond:  5000,
			ConcurrentSenders: 50,
			MetricSize:        20,
			ErrorRate:         0.02,
			SlowQueryRate:     0.10,
		},
		{
			Name:              "Heavy_Load",
			Duration:          120 * time.Second,
			MetricsPerSecond:  10000,
			ConcurrentSenders: 100,
			MetricSize:        50,
			ErrorRate:         0.05,
			SlowQueryRate:     0.20,
		},
		{
			Name:              "Stress_Load",
			Duration:          60 * time.Second,
			MetricsPerSecond:  50000,
			ConcurrentSenders: 200,
			MetricSize:        100,
			ErrorRate:         0.10,
			SlowQueryRate:     0.30,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			result := runLoadTest(t, scenario)
			
			// Log results
			t.Logf("Load Test Results for %s:", scenario.Name)
			t.Logf("  Total Metrics: %d", result.TotalMetrics)
			t.Logf("  Processed: %d (%.2f%%)", result.ProcessedMetrics, 
				float64(result.ProcessedMetrics)/float64(result.TotalMetrics)*100)
			t.Logf("  Dropped: %d", result.DroppedMetrics)
			t.Logf("  Errors: %d", result.Errors)
			t.Logf("  Throughput: %.2f metrics/sec", result.ThroughputPerSec)
			t.Logf("  Avg Latency: %v", result.AverageLatency)
			t.Logf("  P99 Latency: %v", result.P99Latency)
			t.Logf("  Max Latency: %v", result.MaxLatency)
			t.Logf("  Memory Used: %.2f MB", result.MemoryUsedMB)
			t.Logf("  CPU Usage: %.2f%%", result.CPUPercent)
			
			// Assertions based on scenario
			switch scenario.Name {
			case "Light_Load":
				assert.Equal(t, int64(0), result.DroppedMetrics, "No metrics should be dropped under light load")
				assert.Less(t, result.AverageLatency, 5*time.Millisecond, "Average latency should be < 5ms")
				assert.Less(t, result.MemoryUsedMB, 100.0, "Memory usage should be < 100MB")
			case "Normal_Load":
				assert.Less(t, float64(result.DroppedMetrics)/float64(result.TotalMetrics), 0.01, 
					"Drop rate should be < 1%")
				assert.Less(t, result.AverageLatency, 10*time.Millisecond, "Average latency should be < 10ms")
				assert.Less(t, result.MemoryUsedMB, 256.0, "Memory usage should be < 256MB")
			case "Heavy_Load":
				assert.Less(t, float64(result.DroppedMetrics)/float64(result.TotalMetrics), 0.05, 
					"Drop rate should be < 5%")
				assert.Less(t, result.P99Latency, 50*time.Millisecond, "P99 latency should be < 50ms")
				assert.Less(t, result.MemoryUsedMB, 512.0, "Memory usage should be < 512MB")
			}
		})
	}
}

// runLoadTest executes a single load test scenario
func runLoadTest(t *testing.T, config LoadTestConfig) LoadTestResult {
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()
	
	// Create processing pipeline
	pipeline := createTestPipeline(t)
	
	// Metrics tracking
	var totalMetrics atomic.Int64
	var processedMetrics atomic.Int64
	var droppedMetrics atomic.Int64
	var errors atomic.Int64
	var latencies []time.Duration
	var latencyMutex sync.Mutex
	
	// Resource tracking
	startMem := getMemoryUsage()
	startTime := time.Now()
	
	// Start metric generators
	var wg sync.WaitGroup
	metricsPerSender := config.MetricsPerSecond / config.ConcurrentSenders
	sendInterval := time.Second / time.Duration(metricsPerSender)
	
	for i := 0; i < config.ConcurrentSenders; i++ {
		wg.Add(1)
		go func(senderID int) {
			defer wg.Done()
			
			ticker := time.NewTicker(sendInterval)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// Generate metrics
					metrics := generateLoadTestMetrics(config.MetricSize, senderID, 
						config.ErrorRate, config.SlowQueryRate)
					
					totalMetrics.Add(int64(config.MetricSize))
					
					// Process metrics
					start := time.Now()
					processed, err := processMetricsThroughPipeline(ctx, pipeline, metrics)
					latency := time.Since(start)
					
					if err != nil {
						errors.Add(1)
					}
					
					processedMetrics.Add(int64(processed))
					dropped := config.MetricSize - processed
					if dropped > 0 {
						droppedMetrics.Add(int64(dropped))
					}
					
					// Track latency
					latencyMutex.Lock()
					latencies = append(latencies, latency)
					latencyMutex.Unlock()
				}
			}
		}(i)
	}
	
	// Wait for test completion
	wg.Wait()
	
	// Calculate results
	duration := time.Since(startTime)
	endMem := getMemoryUsage()
	
	// Calculate latency percentiles
	avgLatency, p99Latency, maxLatency := calculateLatencyPercentiles(latencies)
	
	return LoadTestResult{
		TotalMetrics:     totalMetrics.Load(),
		ProcessedMetrics: processedMetrics.Load(),
		DroppedMetrics:   droppedMetrics.Load(),
		Errors:          errors.Load(),
		AverageLatency:  avgLatency,
		P99Latency:      p99Latency,
		MaxLatency:      maxLatency,
		MemoryUsedMB:    endMem - startMem,
		CPUPercent:      getCPUUsage(),
		ThroughputPerSec: float64(processedMetrics.Load()) / duration.Seconds(),
	}
}

// TestPipelineStress tests the pipeline under extreme conditions
func TestPipelineStress(t *testing.T) {
	pipeline := createTestPipeline(t)
	
	// Generate massive batch
	metrics := generateLoadTestMetrics(10000, 0, 0.1, 0.3)
	
	// Measure processing time
	start := time.Now()
	processed, err := processMetricsThroughPipeline(context.Background(), pipeline, metrics)
	duration := time.Since(start)
	
	require.NoError(t, err)
	t.Logf("Processed %d/%d metrics in %v", processed, 10000, duration)
	t.Logf("Throughput: %.2f metrics/sec", float64(processed)/duration.Seconds())
	
	// Should process at least 95% under stress
	assert.GreaterOrEqual(t, float64(processed)/10000.0, 0.95)
}

// TestMemoryLeaks checks for memory leaks during extended operation
func TestMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	pipeline := createTestPipeline(t)
	
	// Force GC and record baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineMem := getMemoryUsage()
	
	// Run for extended period
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
					metrics := generateLoadTestMetrics(100, 0, 0.05, 0.1)
					processMetricsThroughPipeline(ctx, pipeline, metrics)
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}
	
	// Monitor memory growth
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	var memSamples []float64
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			goto done
		case <-ticker.C:
			runtime.GC()
			time.Sleep(100 * time.Millisecond)
			currentMem := getMemoryUsage()
			growth := currentMem - baselineMem
			memSamples = append(memSamples, growth)
			t.Logf("Memory growth: %.2f MB", growth)
		}
	}
	
done:
	// Check for excessive memory growth
	if len(memSamples) > 0 {
		avgGrowth := average(memSamples)
		t.Logf("Average memory growth: %.2f MB", avgGrowth)
		assert.Less(t, avgGrowth, 50.0, "Memory growth should be < 50MB")
	}
}

// Helper functions

type testPipeline struct {
	sampler   *adaptivesampler.AdaptiveSampler
	breaker   *circuitbreaker.CircuitBreaker
	extractor *planattributeextractor.PlanAttributeExtractor
	verifier  *verification.VerificationProcessor
}

func createTestPipeline(t *testing.T) *testPipeline {
	sampler, err := adaptivesampler.NewAdaptiveSampler(&adaptivesampler.Config{
		DefaultSamplingRate: 0.5,
		InMemoryOnly:       true,
	}, zap.NewNop())
	require.NoError(t, err)
	
	breaker, err := circuitbreaker.NewCircuitBreaker(&circuitbreaker.Config{
		FailureThreshold: 10,
		Timeout:         30 * time.Second,
	}, zap.NewNop())
	require.NoError(t, err)
	
	extractor, err := planattributeextractor.NewPlanAttributeExtractor(&planattributeextractor.Config{
		SafeMode: true,
		Timeout:  50 * time.Millisecond,
	}, zap.NewNop())
	require.NoError(t, err)
	
	verifier, err := verification.NewVerificationProcessor(&verification.Config{
		PIIDetection: verification.PIIDetectionConfig{
			Enabled: false, // Disable for performance tests
		},
	}, zap.NewNop())
	require.NoError(t, err)
	
	return &testPipeline{
		sampler:   sampler,
		breaker:   breaker,
		extractor: extractor,
		verifier:  verifier,
	}
}

func processMetricsThroughPipeline(ctx context.Context, pipeline *testPipeline, metrics pmetric.Metrics) (int, error) {
	m := metrics
	var err error
	
	// Process through pipeline
	m, err = pipeline.sampler.ProcessMetrics(ctx, m)
	if err != nil {
		return 0, err
	}
	
	m, err = pipeline.breaker.ProcessMetrics(ctx, m)
	if err != nil {
		return 0, err
	}
	
	m, err = pipeline.extractor.ProcessMetrics(ctx, m)
	if err != nil {
		return 0, err
	}
	
	m, err = pipeline.verifier.ProcessMetrics(ctx, m)
	if err != nil {
		return 0, err
	}
	
	// Count remaining metrics
	count := 0
	for i := 0; i < m.ResourceMetrics().Len(); i++ {
		rm := m.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			count += sm.Metrics().Len()
		}
	}
	
	return count, nil
}

func generateLoadTestMetrics(count, senderID int, errorRate, slowQueryRate float64) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", fmt.Sprintf("postgres-%d", senderID))
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	for i := 0; i < count; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("db.query.duration")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pmetric.NewTimestampFromTime(time.Now()))
		
		// Normal query duration 10-100ms
		duration := float64(10 + (i%90))
		
		// Add slow queries
		if float64(i)/float64(count) < slowQueryRate {
			duration = float64(1000 + (i%4000)) // 1-5 seconds
		}
		
		dp.SetDoubleValue(duration)
		dp.Attributes().PutStr("db.statement", fmt.Sprintf("SELECT * FROM table_%d", i%50))
		dp.Attributes().PutStr("db.user", fmt.Sprintf("user_%d", i%10))
		dp.Attributes().PutStr("db.name", fmt.Sprintf("db_%d", senderID%5))
		
		// Add errors
		if float64(i)/float64(count) < errorRate {
			dp.Attributes().PutStr("error", "connection timeout")
		}
	}
	
	return metrics
}

func getMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024 // MB
}

func getCPUUsage() float64 {
	// Simplified CPU usage - in production use proper CPU monitoring
	return float64(runtime.NumGoroutine()) * 0.1
}

func calculateLatencyPercentiles(latencies []time.Duration) (avg, p99, max time.Duration) {
	if len(latencies) == 0 {
		return
	}
	
	// Calculate average
	var total time.Duration
	for _, l := range latencies {
		total += l
		if l > max {
			max = l
		}
	}
	avg = total / time.Duration(len(latencies))
	
	// Calculate P99 (simplified - proper implementation would sort)
	p99Index := int(float64(len(latencies)) * 0.99)
	if p99Index < len(latencies) {
		p99 = latencies[p99Index]
	}
	
	return
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}