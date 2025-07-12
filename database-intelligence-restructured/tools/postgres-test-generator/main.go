package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

type Config struct {
	Host               string
	Port               int
	User               string
	Password           string
	Database           string
	MaxConnections     int
	WorkersPerPattern  int
	QueryInterval      time.Duration
	EnableDeadlocks    bool
	EnableTempFiles    bool
	EnableReplication  bool
}

type TestGenerator struct {
	config *Config
	db     *sql.DB
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func main() {
	config := parseFlags()
	
	generator, err := NewTestGenerator(config)
	if err != nil {
		log.Fatalf("Failed to create test generator: %v", err)
	}
	defer generator.Close()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("Starting PostgreSQL test data generator...")
	log.Printf("Generating load to exercise all %d+ metrics", 35)

	// Start all test patterns
	generator.Start()

	// Wait for interrupt
	<-sigChan
	log.Println("Shutting down...")
	generator.Stop()
}

func parseFlags() *Config {
	config := &Config{}
	
	flag.StringVar(&config.Host, "host", getEnv("POSTGRES_HOST", "localhost"), "PostgreSQL host")
	flag.IntVar(&config.Port, "port", getEnvInt("POSTGRES_PORT", 5432), "PostgreSQL port")
	flag.StringVar(&config.User, "user", getEnv("POSTGRES_USER", "postgres"), "PostgreSQL user")
	flag.StringVar(&config.Password, "password", getEnv("POSTGRES_PASSWORD", "postgres"), "PostgreSQL password")
	flag.StringVar(&config.Database, "database", getEnv("POSTGRES_DB", "testdb"), "PostgreSQL database")
	flag.IntVar(&config.MaxConnections, "max-connections", 50, "Maximum number of connections")
	flag.IntVar(&config.WorkersPerPattern, "workers", 5, "Workers per test pattern")
	flag.DurationVar(&config.QueryInterval, "interval", 100*time.Millisecond, "Query interval")
	flag.BoolVar(&config.EnableDeadlocks, "deadlocks", true, "Enable deadlock generation")
	flag.BoolVar(&config.EnableTempFiles, "temp-files", true, "Enable temp file generation")
	flag.BoolVar(&config.EnableReplication, "replication", false, "Enable replication testing")
	
	flag.Parse()
	return config
}

func NewTestGenerator(config *Config) (*TestGenerator, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Database)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxConnections / 2)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	generator := &TestGenerator{
		config: config,
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}
	
	// Initialize test schema
	if err := generator.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	
	return generator, nil
}

func (g *TestGenerator) initSchema() error {
	queries := []string{
		// Create test tables to exercise table/index metrics
		`CREATE TABLE IF NOT EXISTS test_metrics (
			id SERIAL PRIMARY KEY,
			data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			category VARCHAR(50),
			value NUMERIC(10,2)
		)`,
		
		`CREATE TABLE IF NOT EXISTS test_transactions (
			id SERIAL PRIMARY KEY,
			account_id INT NOT NULL,
			amount NUMERIC(10,2),
			type VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS test_locks (
			id SERIAL PRIMARY KEY,
			resource_id INT NOT NULL,
			lock_type VARCHAR(20),
			acquired_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Create indexes to exercise index metrics
		`CREATE INDEX IF NOT EXISTS idx_metrics_category ON test_metrics(category)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_created ON test_metrics(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_account ON test_transactions(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_locks_resource ON test_locks(resource_id)`,
		
		// Create a large table for sequential scan testing
		`CREATE TABLE IF NOT EXISTS test_large (
			id SERIAL PRIMARY KEY,
			data TEXT,
			random_value INT
		)`,
	}
	
	for _, query := range queries {
		if _, err := g.db.ExecContext(g.ctx, query); err != nil {
			return err
		}
	}
	
	// Insert initial data for large table
	log.Println("Inserting initial test data...")
	tx, err := g.db.BeginTx(g.ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare("INSERT INTO test_large (data, random_value) VALUES ($1, $2)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for i := 0; i < 10000; i++ {
		_, err := stmt.Exec(generateRandomString(100), rand.Intn(1000))
		if err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (g *TestGenerator) Start() {
	patterns := []struct {
		name    string
		workers int
		fn      func()
	}{
		{"Connection Churning", 3, g.connectionChurnPattern},
		{"Transaction Mix", g.config.WorkersPerPattern, g.transactionPattern},
		{"Query Load", g.config.WorkersPerPattern, g.queryLoadPattern},
		{"Index Operations", 2, g.indexOperationsPattern},
		{"Sequential Scans", 2, g.sequentialScanPattern},
		{"Temp File Generation", 2, g.tempFilePattern},
		{"WAL Activity", 2, g.walActivityPattern},
		{"Vacuum Activity", 1, g.vacuumPattern},
		{"Lock Contention", 3, g.lockContentionPattern},
	}
	
	if g.config.EnableDeadlocks {
		patterns = append(patterns, struct {
			name    string
			workers int
			fn      func()
		}{"Deadlock Generation", 2, g.deadlockPattern})
	}
	
	for _, pattern := range patterns {
		for i := 0; i < pattern.workers; i++ {
			g.wg.Add(1)
			go func(name string, id int, fn func()) {
				defer g.wg.Done()
				log.Printf("Starting %s worker %d", name, id)
				fn()
			}(pattern.name, i, pattern.fn)
		}
	}
}

func (g *TestGenerator) Stop() {
	g.cancel()
	g.wg.Wait()
}

func (g *TestGenerator) Close() {
	g.db.Close()
}

// Pattern implementations to exercise different metrics

func (g *TestGenerator) connectionChurnPattern() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Create and close connections to exercise postgresql.backends metric
			conn, err := g.db.Conn(g.ctx)
			if err != nil {
				log.Printf("Connection churn error: %v", err)
				continue
			}
			
			// Hold connection briefly
			time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
			conn.Close()
		}
	}
}

func (g *TestGenerator) transactionPattern() {
	ticker := time.NewTicker(g.config.QueryInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Randomly choose commit or rollback to exercise both metrics
			shouldCommit := rand.Float32() > 0.1 // 90% commit, 10% rollback
			
			tx, err := g.db.BeginTx(g.ctx, nil)
			if err != nil {
				continue
			}
			
			// Perform some operations
			accountID := rand.Intn(1000)
			amount := rand.Float64() * 1000
			
			_, err = tx.Exec(
				"INSERT INTO test_transactions (account_id, amount, type) VALUES ($1, $2, $3)",
				accountID, amount, "TEST",
			)
			
			if err != nil || !shouldCommit {
				tx.Rollback() // Exercise postgresql.rollbacks
			} else {
				tx.Commit() // Exercise postgresql.commits
			}
		}
	}
}

func (g *TestGenerator) queryLoadPattern() {
	ticker := time.NewTicker(g.config.QueryInterval)
	defer ticker.Stop()
	
	queries := []string{
		// Simple queries to exercise postgresql.rows
		"SELECT * FROM test_metrics WHERE category = $1 LIMIT 10",
		"SELECT COUNT(*) FROM test_metrics",
		"SELECT category, AVG(value) FROM test_metrics GROUP BY category",
		
		// Updates/Inserts/Deletes to exercise DML metrics
		"INSERT INTO test_metrics (data, category, value) VALUES ($1, $2, $3)",
		"UPDATE test_metrics SET value = value + 1 WHERE category = $1",
		"DELETE FROM test_metrics WHERE created_at < NOW() - INTERVAL '1 hour' AND category = $1",
	}
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			query := queries[rand.Intn(len(queries))]
			
			switch query {
			case queries[0], queries[2], queries[4], queries[5]:
				g.db.ExecContext(g.ctx, query, fmt.Sprintf("cat_%d", rand.Intn(10)))
			case queries[1]:
				g.db.QueryRowContext(g.ctx, query).Scan(new(int))
			case queries[3]:
				g.db.ExecContext(g.ctx, query, 
					generateRandomString(50), 
					fmt.Sprintf("cat_%d", rand.Intn(10)),
					rand.Float64()*100,
				)
			}
		}
	}
}

func (g *TestGenerator) indexOperationsPattern() {
	ticker := time.NewTicker(g.config.QueryInterval * 2)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Queries that use indexes to exercise postgresql.index.scans
			category := fmt.Sprintf("cat_%d", rand.Intn(10))
			
			rows, err := g.db.QueryContext(g.ctx,
				"SELECT * FROM test_metrics WHERE category = $1", category)
			if err == nil {
				rows.Close()
			}
			
			// Query by date range (uses index)
			rows, err = g.db.QueryContext(g.ctx,
				"SELECT * FROM test_metrics WHERE created_at > NOW() - INTERVAL '1 hour'")
			if err == nil {
				rows.Close()
			}
		}
	}
}

func (g *TestGenerator) sequentialScanPattern() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Force sequential scan on large table (no index on random_value)
			// This exercises postgresql.sequential_scans
			rows, err := g.db.QueryContext(g.ctx,
				"SELECT * FROM test_large WHERE random_value = $1", rand.Intn(1000))
			if err == nil {
				rows.Close()
			}
		}
	}
}

func (g *TestGenerator) tempFilePattern() {
	if !g.config.EnableTempFiles {
		return
	}
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Large sort operation to generate temp files
			// This exercises postgresql.temp_files
			rows, err := g.db.QueryContext(g.ctx, `
				SELECT t1.*, t2.data 
				FROM test_large t1 
				JOIN test_large t2 ON t1.random_value = t2.random_value 
				ORDER BY t1.data, t2.data
			`)
			if err == nil {
				rows.Close()
			}
		}
	}
}

func (g *TestGenerator) walActivityPattern() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Bulk insert to generate WAL activity
			// This exercises postgresql.wal.* metrics
			tx, err := g.db.BeginTx(g.ctx, nil)
			if err != nil {
				continue
			}
			
			stmt, err := tx.Prepare("INSERT INTO test_metrics (data, category, value) VALUES ($1, $2, $3)")
			if err != nil {
				tx.Rollback()
				continue
			}
			
			for i := 0; i < 100; i++ {
				stmt.Exec(
					generateRandomString(100),
					fmt.Sprintf("bulk_%d", rand.Intn(5)),
					rand.Float64()*1000,
				)
			}
			stmt.Close()
			tx.Commit()
		}
	}
}

func (g *TestGenerator) vacuumPattern() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Run VACUUM to exercise postgresql.table.vacuum.count
			tables := []string{"test_metrics", "test_transactions", "test_locks"}
			table := tables[rand.Intn(len(tables))]
			
			g.db.ExecContext(g.ctx, fmt.Sprintf("VACUUM %s", table))
		}
	}
}

func (g *TestGenerator) lockContentionPattern() {
	ticker := time.NewTicker(g.config.QueryInterval * 3)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			resourceID := rand.Intn(10) // Limited resources to increase contention
			
			tx, err := g.db.BeginTx(g.ctx, nil)
			if err != nil {
				continue
			}
			
			// Try to acquire lock on resource
			// This exercises postgresql.locks and potentially db.ash.blocked_sessions
			_, err = tx.Exec(`
				INSERT INTO test_locks (resource_id, lock_type) 
				VALUES ($1, 'exclusive')
				ON CONFLICT (resource_id) DO UPDATE 
				SET acquired_at = CURRENT_TIMESTAMP
			`, resourceID)
			
			if err == nil {
				// Hold lock briefly to create contention
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			}
			
			tx.Rollback()
		}
	}
}

func (g *TestGenerator) deadlockPattern() {
	if !g.config.EnableDeadlocks {
		return
	}
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Create potential deadlock situation
			// This exercises postgresql.deadlocks
			go g.deadlockWorker(1, 2)
			go g.deadlockWorker(2, 1)
		}
	}
}

func (g *TestGenerator) deadlockWorker(first, second int) {
	tx, err := g.db.BeginTx(g.ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	
	// Lock first resource
	tx.Exec("UPDATE test_locks SET lock_type = 'deadlock_test' WHERE resource_id = $1", first)
	
	// Small delay
	time.Sleep(100 * time.Millisecond)
	
	// Try to lock second resource (potential deadlock)
	tx.Exec("UPDATE test_locks SET lock_type = 'deadlock_test' WHERE resource_id = $1", second)
}

// Utility functions

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}