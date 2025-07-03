//go:build benchmark
// +build benchmark

package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Benchmarks for critical paths

func BenchmarkPlanCollection(b *testing.B) {
	// Convert benchmark to test.T for setup
	t := &testing.T{}
	
	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupTestSchema(t, db)

	collector := testEnv.StartCollector(t, "testdata/config-plan-intelligence.yaml")
	defer collector.Shutdown()

	// Wait for collector to be ready
	time.Sleep(5 * time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn := getNewConnection(t, testEnv)
		defer conn.Close()

		for pb.Next() {
			// Execute query that triggers auto_explain
			_, _ = conn.Exec(`
				SELECT u.*, COUNT(o.id) as order_count
				FROM users u
				LEFT JOIN orders o ON u.id = o.user_id
				GROUP BY u.id
				HAVING COUNT(o.id) > 5
				LIMIT 100`)
		}
	})

	b.StopTimer()
	
	// Report metrics
	metrics := testEnv.GetCollectedMetrics()
	planMetrics := findMetricsByName(metrics, "db.postgresql.query.exec_time")
	b.Logf("Plan metrics collected: %d", len(planMetrics))
}

func BenchmarkASHSampling(b *testing.B) {
	testEnv := setupTestEnvironment(b)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupASHTestSchema(b, db)

	collector := testEnv.StartCollector(b, "testdata/config-ash.yaml")
	defer collector.Shutdown()

	// Create persistent sessions
	sessions := make([]*sql.DB, 50)
	for i := range sessions {
		sessions[i] = getNewConnection(b, testEnv)
		defer sessions[i].Close()
	}

	b.ResetTimer()
	
	// Run for N seconds (ASH samples every second)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(b.N)*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i, conn := range sessions {
		wg.Add(1)
		go func(id int, c *sql.DB) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Various session states
					switch id % 4 {
					case 0:
						c.Exec("SELECT pg_sleep(0.1)")
					case 1:
						c.Exec("SELECT COUNT(*) FROM ash_test_table")
					case 2:
						c.Exec("UPDATE ash_test_table SET value = $1 WHERE id = $2", id, id)
					case 3:
						time.Sleep(100 * time.Millisecond)
					}
				}
			}
		}(i, conn)
	}

	wg.Wait()
	b.StopTimer()

	// Report sampling efficiency
	metrics := testEnv.GetCollectedMetrics()
	ashMetrics := findMetricsByName(metrics, "postgresql.ash.sessions.count")
	b.Logf("ASH samples collected: %d (expected ~%d)", len(ashMetrics), b.N)
}

func BenchmarkPlanAnonymization(b *testing.B) {
	testEnv := setupTestEnvironment(b)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupTestSchema(b, db)

	collector := testEnv.StartCollector(b, "testdata/config-plan-intelligence.yaml")
	defer collector.Shutdown()

	// Prepare queries with PII
	queries := []string{
		"SELECT * FROM users WHERE email = 'user1@example.com'",
		"SELECT * FROM users WHERE ssn = '123-45-6789'",
		"SELECT * FROM users WHERE credit_card = '4111111111111111'",
		"SELECT * FROM users WHERE phone = '+1-555-123-4567'",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn := getNewConnection(b, testEnv)
		defer conn.Close()

		i := 0
		for pb.Next() {
			query := queries[i%len(queries)]
			_, _ = conn.Exec(query)
			i++
		}
	})

	b.StopTimer()
	
	// Verify anonymization
	logs := testEnv.GetCollectedLogs()
	piiFound := 0
	for _, log := range logs {
		body := log.Body().AsString()
		if strings.Contains(body, "@example.com") || 
		   strings.Contains(body, "123-45-6789") ||
		   strings.Contains(body, "4111111111111111") {
			piiFound++
		}
	}
	
	if piiFound > 0 {
		b.Errorf("Found %d instances of non-anonymized PII", piiFound)
	}
}

func BenchmarkHighCardinality(b *testing.B) {
	testEnv := setupTestEnvironment(b)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupPerformanceTestSchema(b, db)

	collector := testEnv.StartCollector(b, "testdata/config-performance.yaml")
	defer collector.Shutdown()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		conn := getNewConnection(b, testEnv)
		defer conn.Close()

		i := 0
		for pb.Next() {
			// Generate unique query
			query := fmt.Sprintf(`
				SELECT * FROM performance_test 
				WHERE id = %d AND status = 'bench_%d'
				ORDER BY created_at DESC
				LIMIT %d`, i, i%1000, i%100+1)
			
			_, _ = conn.Exec(query)
			i++
		}
	})

	b.StopTimer()
	
	// Check cardinality management
	metrics := testEnv.GetCollectedMetrics()
	uniqueQueries := make(map[string]bool)
	for _, metric := range metrics {
		if metric.Name() == "db.postgresql.query.exec_time" {
			attrs := getMetricAttributes(metric)
			if qid, ok := attrs["query_id"]; ok {
				uniqueQueries[qid] = true
			}
		}
	}
	
	b.Logf("Unique queries tracked: %d (total executed: %d)", len(uniqueQueries), b.N)
}

func BenchmarkBlockingDetection(b *testing.B) {
	testEnv := setupTestEnvironment(b)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupASHTestSchema(b, db)

	collector := testEnv.StartCollector(b, "testdata/config-ash.yaml")
	defer collector.Shutdown()

	// Create table with known contention points
	db.Exec(`
		CREATE TABLE IF NOT EXISTS contention_test (
			id INT PRIMARY KEY,
			value INT
		)`)
	db.Exec(`
		INSERT INTO contention_test (id, value)
		SELECT i, 0 FROM generate_series(1, 100) i`)

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Create blocking scenario
		var wg sync.WaitGroup
		
		// Blocker
		blockingConn := getNewConnection(b, testEnv)
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, _ := blockingConn.Begin()
			tx.Exec("UPDATE contention_test SET value = value + 1 WHERE id = 1")
			time.Sleep(100 * time.Millisecond)
			tx.Rollback()
			blockingConn.Close()
		}()

		// Give blocker time to acquire lock
		time.Sleep(10 * time.Millisecond)

		// Blocked sessions
		for j := 0; j < 5; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn := getNewConnection(b, testEnv)
				defer conn.Close()
				
				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				defer cancel()
				
				conn.ExecContext(ctx, "UPDATE contention_test SET value = value + 1 WHERE id = 1")
			}()
		}

		wg.Wait()
		
		// Let ASH capture the blocking
		time.Sleep(1 * time.Second)
	}

	b.StopTimer()
	
	// Verify blocking was detected
	metrics := testEnv.GetCollectedMetrics()
	blockingMetrics := findMetricsByName(metrics, "postgresql.ash.blocking_sessions.count")
	b.Logf("Blocking scenarios detected: %d", len(blockingMetrics))
}

func BenchmarkNRDBExport(b *testing.B) {
	testEnv := setupTestEnvironment(b)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupFullTestSchema(b, db)

	collector := testEnv.StartCollector(b, "testdata/config-full-integration.yaml")
	defer collector.Shutdown()

	// Generate initial activity
	generateFullStackActivity(b, db)
	time.Sleep(5 * time.Second)

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Generate metrics
		conn := getNewConnection(b, testEnv)
		conn.Exec("SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '1 hour'")
		conn.Close()
		
		// Wait for export
		time.Sleep(100 * time.Millisecond)
		
		// Verify export
		payload := testEnv.GetNRDBPayload()
		if payload == nil || len(payload.Metrics) == 0 {
			b.Error("No metrics exported to NRDB")
		}
	}

	b.StopTimer()
	
	// Report export statistics
	finalPayload := testEnv.GetNRDBPayload()
	if finalPayload != nil {
		b.Logf("Final NRDB payload size: %d metrics", len(finalPayload.Metrics))
	}
}

// Benchmark helpers

func BenchmarkLogParsing(b *testing.B) {
	// Benchmark just the log parsing performance
	sampleLogs := []string{
		`{"timestamp":"2024-01-01 12:00:00.123","message":"duration: 123.456 ms  plan: {\"Plan\":{\"Node Type\":\"Seq Scan\",\"Relation Name\":\"users\",\"Total Cost\":100.00}}"}`,
		`{"timestamp":"2024-01-01 12:00:01.123","message":"duration: 234.567 ms  plan: {\"Plan\":{\"Node Type\":\"Index Scan\",\"Index Name\":\"users_pkey\",\"Total Cost\":50.00}}"}`,
		`{"timestamp":"2024-01-01 12:00:02.123","message":"duration: 345.678 ms  plan: {\"Plan\":{\"Node Type\":\"Nested Loop\",\"Total Cost\":200.00,\"Plans\":[{\"Node Type\":\"Seq Scan\"},{\"Node Type\":\"Index Scan\"}]}}"}`,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		log := sampleLogs[i%len(sampleLogs)]
		
		// Simulate parsing
		var parsed map[string]interface{}
		json.Unmarshal([]byte(log), &parsed)
		
		// Extract plan
		if msg, ok := parsed["message"].(string); ok {
			planStart := strings.Index(msg, "plan: ")
			if planStart >= 0 {
				planJSON := msg[planStart+6:]
				var plan map[string]interface{}
				json.Unmarshal([]byte(planJSON), &plan)
			}
		}
	}
}

func BenchmarkMetricCreation(b *testing.B) {
	// Benchmark metric creation overhead
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Simulate metric creation
		metric := pmetric.NewMetric()
		metric.SetName("db.postgresql.query.exec_time")
		metric.SetDescription("Query execution time")
		metric.SetUnit("ms")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(123.45)
		dp.Attributes().PutStr("query_id", fmt.Sprintf("query_%d", i))
		dp.Attributes().PutStr("database", "test_db")
	}
}

// Stress test scenarios

func BenchmarkStressTest(b *testing.B) {
	scenarios := []struct {
		name string
		fn   func(*testing.B)
	}{
		{"LowLoad", benchmarkLowLoad},
		{"MediumLoad", benchmarkMediumLoad},
		{"HighLoad", benchmarkHighLoad},
		{"SpikeLoad", benchmarkSpikeLoad},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, scenario.fn)
	}
}

func benchmarkLowLoad(b *testing.B) {
	runLoadBenchmark(b, 10, 100*time.Millisecond)
}

func benchmarkMediumLoad(b *testing.B) {
	runLoadBenchmark(b, 50, 20*time.Millisecond)
}

func benchmarkHighLoad(b *testing.B) {
	runLoadBenchmark(b, 200, 5*time.Millisecond)
}

func benchmarkSpikeLoad(b *testing.B) {
	// Alternating between low and high load
	for i := 0; i < b.N; i++ {
		if i%10 < 2 {
			// Spike
			runLoadBenchmark(b, 500, 1*time.Millisecond)
		} else {
			// Normal
			runLoadBenchmark(b, 20, 50*time.Millisecond)
		}
	}
}

func runLoadBenchmark(b *testing.B, concurrency int, delay time.Duration) {
	testEnv := setupTestEnvironment(b)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupPerformanceTestSchema(b, db)

	collector := testEnv.StartCollector(b, "testdata/config-performance.yaml")
	defer collector.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			conn := getNewConnection(b, testEnv)
			defer conn.Close()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					conn.Exec("SELECT * FROM performance_test WHERE id = $1", id)
					time.Sleep(delay)
				}
			}
		}(i)
	}

	wg.Wait()
}