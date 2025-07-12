package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

type LoadGenerator struct {
	db      *sql.DB
	pattern string
	qps     int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func main() {
	lg := &LoadGenerator{
		pattern: getEnv("LOAD_PATTERN", "mixed"),
		qps:     getEnvInt("QUERIES_PER_SECOND", 10),
	}

	// Connect to PostgreSQL
	pgDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_PORT", "5432"),
		getEnv("POSTGRES_USER", "postgres"),
		getEnv("POSTGRES_PASSWORD", "postgres"),
		getEnv("POSTGRES_DB", "testdb"),
	)
	
	var err error
	lg.db, err = sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer lg.db.Close()

	// Configure connection pool
	lg.db.SetMaxOpenConns(20)
	lg.db.SetMaxIdleConns(10)
	lg.db.SetConnMaxLifetime(5 * time.Minute)

	// Create context for graceful shutdown
	lg.ctx, lg.cancel = context.WithCancel(context.Background())

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("PostgreSQL load generator started: pattern=%s, qps=%d", lg.pattern, lg.qps)
	
	// Create test tables
	if err := lg.createTables(); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	
	// Start load generation
	lg.generateLoad()

	// Wait for interrupt
	<-sigChan
	log.Println("Shutting down...")
	lg.cancel()
	lg.wg.Wait()
	log.Println("Load generator stopped")
}

func (lg *LoadGenerator) createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(100) UNIQUE,
			email VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			data JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255),
			price DECIMAL(10,2),
			stock INTEGER,
			category VARCHAR(100),
			description TEXT,
			attributes JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			total DECIMAL(10,2),
			status VARCHAR(50),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			items JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS analytics (
			id SERIAL PRIMARY KEY,
			event_type VARCHAR(100),
			user_id INTEGER,
			data JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id SERIAL PRIMARY KEY,
			session_id UUID DEFAULT gen_random_uuid(),
			user_id INTEGER,
			started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			data TEXT
		)`,
		// Indexes to exercise postgresql.index.scans
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_products_category ON products(category)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_event ON analytics(event_type, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		// Partial index
		`CREATE INDEX IF NOT EXISTS idx_orders_pending ON orders(id) WHERE status = 'pending'`,
		// Expression index
		`CREATE INDEX IF NOT EXISTS idx_products_lower_name ON products(LOWER(name))`,
	}

	for _, query := range tables {
		if _, err := lg.db.ExecContext(lg.ctx, query); err != nil {
			return fmt.Errorf("failed to execute: %s - %v", query, err)
		}
	}

	// Insert initial data
	log.Println("Inserting initial test data...")
	
	// Insert users
	for i := 0; i < 100; i++ {
		_, err := lg.db.ExecContext(lg.ctx,
			"INSERT INTO users (username, email, data) VALUES ($1, $2, $3) ON CONFLICT (username) DO NOTHING",
			fmt.Sprintf("user_%d", i),
			fmt.Sprintf("user_%d@example.com", i),
			fmt.Sprintf(`{"role": "user", "level": %d}`, rand.Intn(10)),
		)
		if err != nil {
			log.Printf("Failed to insert user: %v", err)
		}
	}

	// Insert products
	categories := []string{"electronics", "books", "clothing", "food", "toys"}
	for i := 0; i < 500; i++ {
		_, err := lg.db.ExecContext(lg.ctx,
			"INSERT INTO products (name, price, stock, category, description) VALUES ($1, $2, $3, $4, $5)",
			fmt.Sprintf("Product %d", i),
			rand.Float64()*1000,
			rand.Intn(100),
			categories[rand.Intn(len(categories))],
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		)
		if err != nil {
			log.Printf("Failed to insert product: %v", err)
		}
	}

	return nil
}

func (lg *LoadGenerator) generateLoad() {
	patterns := map[string]func(){
		"simple":     lg.simpleQueries,
		"complex":    lg.complexQueries,
		"analytical": lg.analyticalQueries,
		"blocking":   lg.blockingQueries,
		"mixed":      lg.mixedQueries,
		"stress":     lg.stressTest,
	}

	pattern, exists := patterns[lg.pattern]
	if !exists {
		log.Printf("Unknown pattern %s, using mixed", lg.pattern)
		pattern = patterns["mixed"]
	}

	// Start the selected pattern
	lg.wg.Add(1)
	go func() {
		defer lg.wg.Done()
		pattern()
	}()

	// Start background activities
	lg.wg.Add(3)
	go lg.vacuumWorker()
	go lg.checkpointWorker()
	go lg.connectionChurnWorker()
}

func (lg *LoadGenerator) simpleQueries() {
	ticker := time.NewTicker(time.Second / time.Duration(lg.qps))
	defer ticker.Stop()

	queries := []func(){
		lg.selectByPrimaryKey,
		lg.selectByIndex,
		lg.insertData,
		lg.updateData,
		lg.deleteData,
	}

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			go queries[rand.Intn(len(queries))]()
		}
	}
}

func (lg *LoadGenerator) complexQueries() {
	ticker := time.NewTicker(time.Second / time.Duration(lg.qps/2))
	defer ticker.Stop()

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			go lg.complexJoin()
			go lg.aggregateQuery()
		}
	}
}

func (lg *LoadGenerator) analyticalQueries() {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			go lg.analyticalQuery()
			go lg.windowFunction()
		}
	}
}

func (lg *LoadGenerator) blockingQueries() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			go lg.lockingTransaction()
			if rand.Float32() < 0.1 { // 10% chance
				go lg.createDeadlock()
			}
		}
	}
}

func (lg *LoadGenerator) mixedQueries() {
	// Run all patterns concurrently
	lg.wg.Add(4)
	go func() {
		defer lg.wg.Done()
		lg.simpleQueries()
	}()
	go func() {
		defer lg.wg.Done()
		lg.complexQueries()
	}()
	go func() {
		defer lg.wg.Done()
		lg.analyticalQueries()
	}()
	go func() {
		defer lg.wg.Done()
		lg.blockingQueries()
	}()
}

func (lg *LoadGenerator) stressTest() {
	// High load pattern
	for i := 0; i < lg.qps*2; i++ {
		lg.wg.Add(1)
		go func() {
			defer lg.wg.Done()
			ticker := time.NewTicker(time.Second / time.Duration(lg.qps))
			defer ticker.Stop()
			
			for {
				select {
				case <-lg.ctx.Done():
					return
				case <-ticker.C:
					lg.selectByPrimaryKey()
					lg.insertData()
				}
			}
		}()
	}
}

// Query implementations

func (lg *LoadGenerator) selectByPrimaryKey() {
	var id int
	var username string
	err := lg.db.QueryRowContext(lg.ctx, 
		"SELECT id, username FROM users WHERE id = $1", 
		rand.Intn(100)+1,
	).Scan(&id, &username)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Select by PK error: %v", err)
	}
}

func (lg *LoadGenerator) selectByIndex() {
	rows, err := lg.db.QueryContext(lg.ctx,
		"SELECT id, name, price FROM products WHERE category = $1 LIMIT 10",
		[]string{"electronics", "books", "clothing", "food", "toys"}[rand.Intn(5)],
	)
	if err != nil {
		log.Printf("Select by index error: %v", err)
		return
	}
	defer rows.Close()
	
	// Consume results to exercise postgresql.rows metric
	for rows.Next() {
		var id int
		var name string
		var price float64
		rows.Scan(&id, &name, &price)
	}
}

func (lg *LoadGenerator) insertData() {
	_, err := lg.db.ExecContext(lg.ctx,
		"INSERT INTO analytics (event_type, user_id, data) VALUES ($1, $2, $3)",
		[]string{"page_view", "click", "purchase", "search"}[rand.Intn(4)],
		rand.Intn(100)+1,
		fmt.Sprintf(`{"timestamp": "%s", "value": %d}`, time.Now().Format(time.RFC3339), rand.Intn(100)),
	)
	if err != nil {
		log.Printf("Insert error: %v", err)
	}
}

func (lg *LoadGenerator) updateData() {
	_, err := lg.db.ExecContext(lg.ctx,
		"UPDATE products SET stock = stock - 1 WHERE id = $1 AND stock > 0",
		rand.Intn(500)+1,
	)
	if err != nil {
		log.Printf("Update error: %v", err)
	}
}

func (lg *LoadGenerator) deleteData() {
	_, err := lg.db.ExecContext(lg.ctx,
		"DELETE FROM analytics WHERE created_at < NOW() - INTERVAL '7 days' AND event_type = $1",
		[]string{"page_view", "click"}[rand.Intn(2)],
	)
	if err != nil {
		log.Printf("Delete error: %v", err)
	}
}

func (lg *LoadGenerator) complexJoin() {
	rows, err := lg.db.QueryContext(lg.ctx, `
		SELECT u.username, COUNT(o.id) as order_count, SUM(o.total) as total_spent
		FROM users u
		LEFT JOIN orders o ON u.id = o.user_id
		WHERE u.created_at > NOW() - INTERVAL '30 days'
		GROUP BY u.username
		HAVING COUNT(o.id) > 0
		ORDER BY total_spent DESC
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Complex join error: %v", err)
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var username string
		var orderCount int
		var totalSpent float64
		rows.Scan(&username, &orderCount, &totalSpent)
	}
}

func (lg *LoadGenerator) aggregateQuery() {
	var count int
	err := lg.db.QueryRowContext(lg.ctx, `
		SELECT COUNT(DISTINCT user_id) 
		FROM analytics 
		WHERE event_type = $1 
		AND created_at > NOW() - INTERVAL '1 hour'
	`, "page_view").Scan(&count)
	if err != nil {
		log.Printf("Aggregate query error: %v", err)
	}
}

func (lg *LoadGenerator) analyticalQuery() {
	// Force sequential scan on purpose to exercise postgresql.sequential_scans
	rows, err := lg.db.QueryContext(lg.ctx, `
		WITH monthly_sales AS (
			SELECT 
				DATE_TRUNC('month', created_at) as month,
				COUNT(*) as order_count,
				SUM(total) as revenue,
				AVG(total) as avg_order_value
			FROM orders
			WHERE total > 10 -- No index on total, forces seq scan
			GROUP BY DATE_TRUNC('month', created_at)
		)
		SELECT * FROM monthly_sales 
		ORDER BY month DESC
		LIMIT 12
	`)
	if err != nil {
		log.Printf("Analytical query error: %v", err)
		return
	}
	defer rows.Close()
	
	for rows.Next() {
		var month time.Time
		var orderCount int
		var revenue, avgValue float64
		rows.Scan(&month, &orderCount, &revenue, &avgValue)
	}
}

func (lg *LoadGenerator) windowFunction() {
	// Query with temp file generation
	rows, err := lg.db.QueryContext(lg.ctx, `
		SELECT 
			user_id,
			event_type,
			created_at,
			ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) as rn,
			LAG(created_at) OVER (PARTITION BY user_id ORDER BY created_at) as prev_event_time
		FROM analytics
		WHERE created_at > NOW() - INTERVAL '24 hours'
		ORDER BY user_id, created_at DESC
	`)
	if err != nil {
		log.Printf("Window function error: %v", err)
		return
	}
	defer rows.Close()
	
	// Consume all rows
	for rows.Next() {
		var userID int
		var eventType string
		var createdAt, prevEventTime sql.NullTime
		var rn int
		rows.Scan(&userID, &eventType, &createdAt, &rn, &prevEventTime)
	}
}

func (lg *LoadGenerator) lockingTransaction() {
	tx, err := lg.db.BeginTx(lg.ctx, nil)
	if err != nil {
		log.Printf("Begin transaction error: %v", err)
		return
	}
	defer tx.Rollback()

	// Lock a row
	var total float64
	err = tx.QueryRow(
		"SELECT total FROM orders WHERE id = $1 FOR UPDATE",
		rand.Intn(100)+1,
	).Scan(&total)
	
	if err != nil && err != sql.ErrNoRows {
		return
	}

	// Simulate some work
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

	// Update the locked row
	_, err = tx.Exec(
		"UPDATE orders SET status = $1, total = $2 WHERE id = $3",
		[]string{"pending", "processing", "completed"}[rand.Intn(3)],
		total * 1.1,
		rand.Intn(100)+1,
	)

	// Randomly commit or rollback to exercise both metrics
	if rand.Float32() < 0.9 {
		tx.Commit()
	} else {
		tx.Rollback()
	}
}

func (lg *LoadGenerator) createDeadlock() {
	// Try to create a deadlock situation
	orderID1 := rand.Intn(50) + 1
	orderID2 := rand.Intn(50) + 51

	// First transaction
	go func() {
		tx, _ := lg.db.BeginTx(lg.ctx, nil)
		defer tx.Rollback()
		
		tx.Exec("UPDATE orders SET status = 'lock1' WHERE id = $1", orderID1)
		time.Sleep(100 * time.Millisecond)
		tx.Exec("UPDATE orders SET status = 'lock1' WHERE id = $1", orderID2)
	}()

	// Second transaction (reverse order)
	go func() {
		tx, _ := lg.db.BeginTx(lg.ctx, nil)
		defer tx.Rollback()
		
		tx.Exec("UPDATE orders SET status = 'lock2' WHERE id = $1", orderID2)
		time.Sleep(100 * time.Millisecond)
		tx.Exec("UPDATE orders SET status = 'lock2' WHERE id = $1", orderID1)
	}()
}

// Background workers

func (lg *LoadGenerator) vacuumWorker() {
	defer lg.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			// Run VACUUM to exercise postgresql.table.vacuum.count
			tables := []string{"analytics", "sessions", "orders"}
			for _, table := range tables {
				lg.db.ExecContext(lg.ctx, fmt.Sprintf("VACUUM %s", table))
			}
		}
	}
}

func (lg *LoadGenerator) checkpointWorker() {
	defer lg.wg.Done()
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			// Force checkpoint to exercise postgresql.bgwriter metrics
			lg.db.ExecContext(lg.ctx, "CHECKPOINT")
		}
	}
}

func (lg *LoadGenerator) connectionChurnWorker() {
	defer lg.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lg.ctx.Done():
			return
		case <-ticker.C:
			// Create new connections to exercise postgresql.backends
			for i := 0; i < 5; i++ {
				go func() {
					conn, err := lg.db.Conn(lg.ctx)
					if err != nil {
						return
					}
					
					// Hold connection for a bit
					time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
					conn.Close()
				}()
			}
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}