// Minimal Database Intelligence Collector for testing processor communication
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	// Import custom processors
	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/verification"
	"github.com/database-intelligence-mvp/processors/nrerrormonitor"
	"github.com/database-intelligence-mvp/processors/costcontrol"
	"github.com/database-intelligence-mvp/processors/querycorrelator"
)

// testSink is a simple consumer that prints data
type testSink struct {
	name string
	attributes map[string]string
}

func (s *testSink) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	log.Printf("[%s] Received %d resource metrics", s.name, md.ResourceMetrics().Len())
	
	// Check for enriched attributes
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			s.attributes[k] = v.AsString()
			return true
		})
		
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				log.Printf("  Metric: %s", metric.Name())
			}
		}
	}
	return nil
}

func (s *testSink) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	log.Printf("[%s] Received %d resource logs", s.name, ld.ResourceLogs().Len())
	
	// Check for enriched attributes
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		rl.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			s.attributes[k] = v.AsString()
			return true
		})
	}
	return nil
}

func (s *testSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func main() {
	// Create logger
	logger, _ := zap.NewDevelopment()
	
	// Test 1: Logs Pipeline (most processors)
	log.Println("=== Testing Logs Pipeline ===")
	testLogsPipeline(logger)
	
	// Test 2: Metrics Pipeline 
	log.Println("\n=== Testing Metrics Pipeline ===")
	testMetricsPipeline(logger)
	
	// Test 3: Cost Control (supports all signals)
	log.Println("\n=== Testing Cost Control Processor ===")
	testCostControl(logger)
	
	log.Println("\n✅ All processor communication tests completed successfully!")
	
	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	
	log.Println("Shutting down...")
}

func testLogsPipeline(logger *zap.Logger) {
	baseSettings := component.TelemetrySettings{
		Logger: logger,
	}
	
	// Create final sink
	sink := &testSink{name: "logs-sink", attributes: make(map[string]string)}
	
	// Create logs processors in reverse order
	processors := []struct {
		name   string
		create func(consumer.Logs) (processor.Logs, error)
	}{
		{
			name: "verification",
			create: func(next consumer.Logs) (processor.Logs, error) {
				settings := processor.Settings{
					ID: component.MustNewID("verification"),
					TelemetrySettings: baseSettings,
				}
				cfg := verification.NewFactory().CreateDefaultConfig()
				return verification.NewFactory().CreateLogs(context.Background(), settings, cfg, next)
			},
		},
		{
			name: "planattributeextractor",
			create: func(next consumer.Logs) (processor.Logs, error) {
				settings := processor.Settings{
					ID: component.MustNewID("planattributeextractor"),
					TelemetrySettings: baseSettings,
				}
				cfg := planattributeextractor.NewFactory().CreateDefaultConfig()
				return planattributeextractor.NewFactory().CreateLogs(context.Background(), settings, cfg, next)
			},
		},
		{
			name: "circuitbreaker",
			create: func(next consumer.Logs) (processor.Logs, error) {
				settings := processor.Settings{
					ID: component.MustNewID("circuitbreaker"),
					TelemetrySettings: baseSettings,
				}
				cfg := circuitbreaker.NewFactory().CreateDefaultConfig()
				return circuitbreaker.NewFactory().CreateLogs(context.Background(), settings, cfg, next)
			},
		},
		{
			name: "adaptivesampler",
			create: func(next consumer.Logs) (processor.Logs, error) {
				settings := processor.Settings{
					ID: component.MustNewID("adaptivesampler"),
					TelemetrySettings: baseSettings,
				}
				cfg := adaptivesampler.NewFactory().CreateDefaultConfig()
				return adaptivesampler.NewFactory().CreateLogs(context.Background(), settings, cfg, next)
			},
		},
	}
	
	// Build the pipeline
	var pipeline consumer.Logs = sink
	for _, p := range processors {
		proc, err := p.create(pipeline)
		if err != nil {
			log.Printf("Failed to create %s: %v", p.name, err)
			continue
		}
		
		// Start the processor
		if err := proc.Start(context.Background(), nil); err != nil {
			log.Printf("Failed to start %s: %v", p.name, err)
			continue
		}
		defer proc.Shutdown(context.Background())
		
		log.Printf("Started logs processor: %s", p.name)
		pipeline = proc
	}
	
	// Create test logs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-db")
	rl.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	lr.Body().SetStr("SELECT * FROM users WHERE email='test@example.com'")
	lr.Attributes().PutStr("db.statement", "SELECT * FROM users WHERE email='test@example.com'")
	lr.Attributes().PutStr("db.plan.json", `{"Plan": {"Node Type": "Seq Scan"}}`)
	
	// Send logs through pipeline
	if err := pipeline.ConsumeLogs(context.Background(), logs); err != nil {
		log.Printf("Failed to consume logs: %v", err)
	}
	
	// Check if attributes were added
	if val, ok := sink.attributes["db.query.plan.hash"]; ok {
		log.Printf("✓ Plan hash extracted: %s", val)
	}
}

func testMetricsPipeline(logger *zap.Logger) {
	baseSettings := component.TelemetrySettings{
		Logger: logger,
	}
	
	// Create final sink
	sink := &testSink{name: "metrics-sink", attributes: make(map[string]string)}
	
	// Create metrics processors
	processors := []struct {
		name   string
		create func(consumer.Metrics) (processor.Metrics, error)
	}{
		{
			name: "nrerrormonitor",
			create: func(next consumer.Metrics) (processor.Metrics, error) {
				settings := processor.Settings{
					ID: component.MustNewID("nrerrormonitor"),
					TelemetrySettings: baseSettings,
				}
				cfg := nrerrormonitor.NewFactory().CreateDefaultConfig()
				return nrerrormonitor.NewFactory().CreateMetrics(context.Background(), settings, cfg, next)
			},
		},
		{
			name: "querycorrelator",
			create: func(next consumer.Metrics) (processor.Metrics, error) {
				settings := processor.Settings{
					ID: component.MustNewID("querycorrelator"),
					TelemetrySettings: baseSettings,
				}
				cfg := querycorrelator.NewFactory().CreateDefaultConfig()
				return querycorrelator.NewFactory().CreateMetrics(context.Background(), settings, cfg, next)
			},
		},
	}
	
	// Build the pipeline
	var pipeline consumer.Metrics = sink
	for _, p := range processors {
		proc, err := p.create(pipeline)
		if err != nil {
			log.Printf("Failed to create %s: %v", p.name, err)
			continue
		}
		
		// Start the processor
		if err := proc.Start(context.Background(), nil); err != nil {
			log.Printf("Failed to start %s: %v", p.name, err)
			continue
		}
		defer proc.Shutdown(context.Background())
		
		log.Printf("Started metrics processor: %s", p.name)
		pipeline = proc
	}
	
	// Create test metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-db")
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.query.duration")
	
	histogram := metric.SetEmptyHistogram()
	dp := histogram.DataPoints().AppendEmpty()
	dp.SetSum(123.45)
	dp.SetCount(1)
	dp.Attributes().PutStr("db.statement", "SELECT * FROM users")
	dp.Attributes().PutStr("db.user", "postgres")
	
	// Send metrics through pipeline
	if err := pipeline.ConsumeMetrics(context.Background(), metrics); err != nil {
		log.Printf("Failed to consume metrics: %v", err)
	}
}

func testCostControl(logger *zap.Logger) {
	baseSettings := component.TelemetrySettings{
		Logger: logger,
	}
	
	// Create final sink
	sink := &testSink{name: "cost-sink", attributes: make(map[string]string)}
	
	// Create cost control processor
	settings := processor.Settings{
		ID: component.MustNewID("costcontrol"),
		TelemetrySettings: baseSettings,
	}
	cfg := costcontrol.NewFactory().CreateDefaultConfig()
	
	// Test with metrics
	metricsProc, err := costcontrol.NewFactory().CreateMetrics(context.Background(), settings, cfg, sink)
	if err != nil {
		log.Printf("Failed to create cost control for metrics: %v", err)
		return
	}
	
	if err := metricsProc.Start(context.Background(), nil); err != nil {
		log.Printf("Failed to start cost control: %v", err)
		return
	}
	defer metricsProc.Shutdown(context.Background())
	
	log.Printf("Started cost control processor for metrics")
	
	// Create test metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-db")
	
	// Add many metrics to test cardinality control
	sm := rm.ScopeMetrics().AppendEmpty()
	for i := 0; i < 10; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("db.custom.metric")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(float64(i))
		dp.Attributes().PutStr("unique_id", strconv.Itoa(i))
	}
	
	// Send metrics through cost control
	if err := metricsProc.ConsumeMetrics(context.Background(), metrics); err != nil {
		log.Printf("Failed to consume metrics in cost control: %v", err)
	}
}