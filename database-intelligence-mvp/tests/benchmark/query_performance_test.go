package benchmark

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
)

// BenchmarkConfig holds benchmark configuration
type BenchmarkConfig struct {
	DatabaseType     string
	ConnectionString string
	NumWorkers       int
	TestDuration     time.Duration
	QueryTypes       []QueryType
}

// QueryType represents different query patterns
type QueryType struct {
	Name     string
	Query    string
	Weight   int // Relative frequency
	ReadOnly bool
}

// BenchmarkResult holds benchmark results
type BenchmarkResult struct {
	TotalQueries      int64
	TotalDuration     time.Duration
	QueriesPerSecond  float64
	AvgLatency        time.Duration
	P50Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	ErrorCount        int64
	QueryTypeMetrics  map[string]*QueryMetrics
}

// QueryMetrics holds metrics for a specific query type
type QueryMetrics struct {
	Count      int64
	TotalTime  time.Duration
	AvgLatency time.Duration
	Errors     int64
}

// PostgreSQL benchmark queries
var PostgreSQLQueries = []QueryType{
	{
		Name:     "simple_select",
		Query:    "SELECT 1",
		Weight:   30,
		ReadOnly: true,
	},
	{
		Name:     "table_scan",
		Query:    "SELECT COUNT(*) FROM pg_stat_user_tables",
		Weight:   20,
		ReadOnly: true,
	},
	{
		Name:     "complex_join",
		Query:    `SELECT t.schemaname, t.tablename, i.indexname 
				   FROM pg_stat_user_tables t 
				   LEFT JOIN pg_stat_user_indexes i ON t.tablename = i.tablename 
				   WHERE t.schemaname = 'public' LIMIT 10`,
		Weight:   15,
		ReadOnly: true,
	},
	{
		Name:     "aggregation",
		Query:    `SELECT schemaname, COUNT(*) as table_count, SUM(n_live_tup) as total_rows 
				   FROM pg_stat_user_tables 
				   GROUP BY schemaname`,
		Weight:   15,
		ReadOnly: true,
	},
	{
		Name:     "system_catalog",
		Query:    "SELECT * FROM pg_stat_activity WHERE state = 'active'",
		Weight:   10,
		ReadOnly: true,
	},
	{
		Name:     "pg_stat_statements",
		Query:    `SELECT query, calls, mean_exec_time 
				   FROM pg_stat_statements 
				   WHERE calls > 100 
				   ORDER BY mean_exec_time DESC 
				   LIMIT 10`,
		Weight:   10,
		ReadOnly: true,
	},
}

// MySQL benchmark queries
var MySQLQueries = []QueryType{
	{
		Name:     "simple_select",
		Query:    "SELECT 1",
		Weight:   30,
		ReadOnly: true,
	},
	{
		Name:     "information_schema",
		Query:    "SELECT COUNT(*) FROM information_schema.tables",
		Weight:   20,
		ReadOnly: true,
	},
	{
		Name:     "performance_schema",
		Query:    `SELECT EVENT_NAME, COUNT_STAR, AVG_TIMER_WAIT 
				   FROM performance_schema.events_statements_summary_global_by_event_name 
				   WHERE COUNT_STAR > 0 
				   ORDER BY AVG_TIMER_WAIT DESC 
				   LIMIT 10`,
		Weight:   15,
		ReadOnly: true,
	},
	{
		Name:     "innodb_status",
		Query:    "SHOW ENGINE INNODB STATUS",
		Weight:   10,
		ReadOnly: true,
	},
	{
		Name:     "processlist",
		Query:    "SHOW PROCESSLIST",
		Weight:   15,
		ReadOnly: true,
	},
	{
		Name:     "table_status",
		Query:    "SHOW TABLE STATUS",
		Weight:   10,
		ReadOnly: true,
	},
}

// BenchmarkPostgreSQLQueries benchmarks PostgreSQL query performance
func BenchmarkPostgreSQLQueries(b *testing.B) {
	config := BenchmarkConfig{
		DatabaseType:     "postgres",
		ConnectionString: "postgres://user:password@localhost/testdb?sslmode=disable",
		NumWorkers:       10,
		TestDuration:     30 * time.Second,
		QueryTypes:       PostgreSQLQueries,
	}

	result := runBenchmark(b, config)
	printResults(result)
}

// BenchmarkMySQLQueries benchmarks MySQL query performance
func BenchmarkMySQLQueries(b *testing.B) {
	config := BenchmarkConfig{
		DatabaseType:     "mysql",
		ConnectionString: "user:password@tcp(localhost:3306)/testdb",
		NumWorkers:       10,
		TestDuration:     30 * time.Second,
		QueryTypes:       MySQLQueries,
	}

	result := runBenchmark(b, config)
	printResults(result)
}

// runBenchmark executes the benchmark
func runBenchmark(b *testing.B, config BenchmarkConfig) *BenchmarkResult {
	b.Helper()

	// Open database connection pool
	db, err := sql.Open(config.DatabaseType, config.ConnectionString)
	if err != nil {
		b.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(config.NumWorkers * 2)
	db.SetMaxIdleConns(config.NumWorkers)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		b.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize result tracking
	result := &BenchmarkResult{
		QueryTypeMetrics: make(map[string]*QueryMetrics),
	}
	for _, qt := range config.QueryTypes {
		result.QueryTypeMetrics[qt.Name] = &QueryMetrics{}
	}

	// Latency tracking
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// Start workers
	var wg sync.WaitGroup
	ctx, cancel = context.WithTimeout(context.Background(), config.TestDuration)
	defer cancel()

	startTime := time.Now()

	for i := 0; i < config.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorker(ctx, db, config, result, &latencies, &latencyMutex)
		}(i)
	}

	// Wait for completion
	wg.Wait()
	result.TotalDuration = time.Since(startTime)

	// Calculate statistics
	calculateStatistics(result, latencies)

	return result
}

// runWorker executes queries in a loop
func runWorker(ctx context.Context, db *sql.DB, config BenchmarkConfig, result *BenchmarkResult, latencies *[]time.Duration, latencyMutex *sync.Mutex) {
	// Build weighted query selection
	totalWeight := 0
	for _, qt := range config.QueryTypes {
		totalWeight += qt.Weight
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Select query based on weight
			queryType := selectQueryType(config.QueryTypes, totalWeight)
			
			// Execute query
			startTime := time.Now()
			err := executeQuery(ctx, db, queryType)
			latency := time.Since(startTime)

			// Update metrics
			latencyMutex.Lock()
			*latencies = append(*latencies, latency)
			
			metrics := result.QueryTypeMetrics[queryType.Name]
			metrics.Count++
			metrics.TotalTime += latency
			
			if err != nil {
				metrics.Errors++
				result.ErrorCount++
			}
			
			result.TotalQueries++
			latencyMutex.Unlock()
		}
	}
}

// selectQueryType selects a query based on weights
func selectQueryType(queryTypes []QueryType, totalWeight int) QueryType {
	r := rand.Intn(totalWeight)
	
	for _, qt := range queryTypes {
		r -= qt.Weight
		if r < 0 {
			return qt
		}
	}
	
	return queryTypes[0] // Fallback
}

// executeQuery executes a single query
func executeQuery(ctx context.Context, db *sql.DB, queryType QueryType) error {
	rows, err := db.QueryContext(ctx, queryType.Query)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Consume all rows
	for rows.Next() {
		// Just iterate through results
	}

	return rows.Err()
}

// calculateStatistics calculates benchmark statistics
func calculateStatistics(result *BenchmarkResult, latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculation
	sortLatencies(latencies)

	// Calculate basic stats
	result.QueriesPerSecond = float64(result.TotalQueries) / result.TotalDuration.Seconds()
	
	totalLatency := time.Duration(0)
	for _, l := range latencies {
		totalLatency += l
	}
	result.AvgLatency = totalLatency / time.Duration(len(latencies))

	// Calculate percentiles
	result.P50Latency = latencies[len(latencies)*50/100]
	result.P95Latency = latencies[len(latencies)*95/100]
	result.P99Latency = latencies[len(latencies)*99/100]

	// Calculate per-query-type averages
	for _, metrics := range result.QueryTypeMetrics {
		if metrics.Count > 0 {
			metrics.AvgLatency = metrics.TotalTime / time.Duration(metrics.Count)
		}
	}
}

// sortLatencies sorts latencies for percentile calculation
func sortLatencies(latencies []time.Duration) {
	// Simple bubble sort for demonstration (use sort.Slice in production)
	n := len(latencies)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if latencies[j] > latencies[j+1] {
				latencies[j], latencies[j+1] = latencies[j+1], latencies[j]
			}
		}
	}
}

// printResults prints benchmark results
func printResults(result *BenchmarkResult) {
	fmt.Printf("\n=== Benchmark Results ===\n")
	fmt.Printf("Total Duration: %v\n", result.TotalDuration)
	fmt.Printf("Total Queries: %d\n", result.TotalQueries)
	fmt.Printf("Queries/Second: %.2f\n", result.QueriesPerSecond)
	fmt.Printf("Error Rate: %.2f%%\n", float64(result.ErrorCount)/float64(result.TotalQueries)*100)
	fmt.Printf("\nLatency Statistics:\n")
	fmt.Printf("  Average: %v\n", result.AvgLatency)
	fmt.Printf("  P50: %v\n", result.P50Latency)
	fmt.Printf("  P95: %v\n", result.P95Latency)
	fmt.Printf("  P99: %v\n", result.P99Latency)
	fmt.Printf("\nQuery Type Breakdown:\n")
	
	for name, metrics := range result.QueryTypeMetrics {
		if metrics.Count > 0 {
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    Count: %d (%.1f%%)\n", metrics.Count, float64(metrics.Count)/float64(result.TotalQueries)*100)
			fmt.Printf("    Avg Latency: %v\n", metrics.AvgLatency)
			fmt.Printf("    Error Rate: %.2f%%\n", float64(metrics.Errors)/float64(metrics.Count)*100)
		}
	}
}

// BenchmarkCompareOHIvsOTEL compares monitoring overhead
func BenchmarkCompareOHIvsOTEL(b *testing.B) {
	// This would compare the overhead of OHI vs OTEL monitoring
	// by running the same workload with each monitoring solution enabled
	
	b.Run("Baseline", func(b *testing.B) {
		// Run without any monitoring
		config := BenchmarkConfig{
			DatabaseType:     "postgres",
			ConnectionString: "postgres://user:password@localhost/testdb?sslmode=disable",
			NumWorkers:       5,
			TestDuration:     10 * time.Second,
			QueryTypes:       PostgreSQLQueries[:3], // Subset for faster test
		}
		runBenchmark(b, config)
	})

	b.Run("WithOTEL", func(b *testing.B) {
		// Run with OTEL monitoring enabled
		// This would have OTEL collector running and collecting metrics
		config := BenchmarkConfig{
			DatabaseType:     "postgres",
			ConnectionString: "postgres://user:password@localhost/testdb?sslmode=disable",
			NumWorkers:       5,
			TestDuration:     10 * time.Second,
			QueryTypes:       PostgreSQLQueries[:3],
		}
		runBenchmark(b, config)
	})
}