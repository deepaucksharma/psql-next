package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DataGenerator creates realistic workload patterns for testing
type DataGenerator struct {
	db              *sql.DB
	ctx             context.Context
	wg              sync.WaitGroup
	stopCh          chan struct{}
	metricsCollector *MetricsCollector
}

// MetricsCollector tracks generated workload metrics
type MetricsCollector struct {
	mu               sync.Mutex
	queriesExecuted  int64
	waitsGenerated   int64
	locksGenerated   int64
	slowQueries      int64
	advisoriesTriggered int64
}

// NewDataGenerator creates a new data generator instance
func NewDataGenerator(dsn string) (*DataGenerator, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for load generation
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &DataGenerator{
		db:              db,
		ctx:             context.Background(),
		stopCh:          make(chan struct{}),
		metricsCollector: &MetricsCollector{},
	}, nil
}

// SetupTestSchema creates all necessary tables and procedures
func (dg *DataGenerator) SetupTestSchema() error {
	schemas := []string{
		// Main transactional tables
		`CREATE DATABASE IF NOT EXISTS wait_test`,
		
		`USE wait_test`,
		
		`CREATE TABLE IF NOT EXISTS customers (
			id INT PRIMARY KEY AUTO_INCREMENT,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP NULL,
			status ENUM('active', 'inactive', 'suspended') DEFAULT 'active',
			credit_score INT DEFAULT 0,
			INDEX idx_email (email),
			INDEX idx_status_created (status, created_at)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS products (
			id INT PRIMARY KEY AUTO_INCREMENT,
			sku VARCHAR(100) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			stock_quantity INT DEFAULT 0,
			category_id INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_sku (sku),
			INDEX idx_category_price (category_id, price),
			INDEX idx_stock (stock_quantity)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS orders (
			id INT PRIMARY KEY AUTO_INCREMENT,
			order_number VARCHAR(50) UNIQUE NOT NULL,
			customer_id INT NOT NULL,
			order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') DEFAULT 'pending',
			total_amount DECIMAL(12,2) NOT NULL,
			shipping_address TEXT,
			payment_method VARCHAR(50),
			notes TEXT,
			INDEX idx_customer_date (customer_id, order_date),
			INDEX idx_status (status),
			INDEX idx_order_number (order_number),
			FOREIGN KEY (customer_id) REFERENCES customers(id)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS order_items (
			id INT PRIMARY KEY AUTO_INCREMENT,
			order_id INT NOT NULL,
			product_id INT NOT NULL,
			quantity INT NOT NULL,
			unit_price DECIMAL(10,2) NOT NULL,
			discount_amount DECIMAL(10,2) DEFAULT 0,
			tax_amount DECIMAL(10,2) DEFAULT 0,
			INDEX idx_order (order_id),
			INDEX idx_product (product_id),
			FOREIGN KEY (order_id) REFERENCES orders(id),
			FOREIGN KEY (product_id) REFERENCES products(id)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS inventory_transactions (
			id INT PRIMARY KEY AUTO_INCREMENT,
			product_id INT NOT NULL,
			transaction_type ENUM('in', 'out', 'adjustment') NOT NULL,
			quantity INT NOT NULL,
			reference_type VARCHAR(50),
			reference_id INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_by INT,
			notes TEXT,
			INDEX idx_product_date (product_id, created_at),
			INDEX idx_type (transaction_type),
			FOREIGN KEY (product_id) REFERENCES products(id)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS user_sessions (
			id VARCHAR(128) PRIMARY KEY,
			customer_id INT NOT NULL,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			expired BOOLEAN DEFAULT FALSE,
			INDEX idx_customer (customer_id),
			INDEX idx_activity (last_activity),
			FOREIGN KEY (customer_id) REFERENCES customers(id)
		) ENGINE=InnoDB`,

		`CREATE TABLE IF NOT EXISTS audit_log (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			entity_type VARCHAR(50) NOT NULL,
			entity_id INT NOT NULL,
			action VARCHAR(50) NOT NULL,
			old_value JSON,
			new_value JSON,
			user_id INT,
			ip_address VARCHAR(45),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_entity (entity_type, entity_id),
			INDEX idx_created (created_at),
			INDEX idx_user (user_id)
		) ENGINE=InnoDB`,

		// Table without indexes for testing missing index advisories
		`CREATE TABLE IF NOT EXISTS search_logs (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			search_term VARCHAR(500),
			customer_id INT,
			result_count INT,
			clicked_position INT,
			search_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB`,

		// Wide table for I/O testing
		`CREATE TABLE IF NOT EXISTS analytics_events (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			event_id VARCHAR(128),
			event_type VARCHAR(50),
			customer_id INT,
			session_id VARCHAR(128),
			page_url VARCHAR(2000),
			referrer_url VARCHAR(2000),
			utm_source VARCHAR(255),
			utm_medium VARCHAR(255),
			utm_campaign VARCHAR(255),
			device_type VARCHAR(50),
			browser VARCHAR(100),
			os VARCHAR(100),
			ip_address VARCHAR(45),
			country VARCHAR(2),
			region VARCHAR(100),
			city VARCHAR(100),
			event_data JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_event_type_date (event_type, created_at),
			INDEX idx_customer (customer_id)
		) ENGINE=InnoDB`,
	}

	for _, schema := range schemas {
		if _, err := dg.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to execute schema: %v", err)
		}
	}

	// Create stored procedures for complex operations
	procedures := []string{
		`DROP PROCEDURE IF EXISTS process_order`,
		`CREATE PROCEDURE process_order(
			IN p_order_id INT,
			IN p_new_status VARCHAR(20)
		)
		BEGIN
			DECLARE v_customer_id INT;
			DECLARE v_old_status VARCHAR(20);
			
			START TRANSACTION;
			
			-- Lock the order row
			SELECT customer_id, status INTO v_customer_id, v_old_status
			FROM orders 
			WHERE id = p_order_id 
			FOR UPDATE;
			
			-- Simulate processing time
			DO SLEEP(0.1);
			
			-- Update order status
			UPDATE orders 
			SET status = p_new_status 
			WHERE id = p_order_id;
			
			-- Log the change
			INSERT INTO audit_log (entity_type, entity_id, action, old_value, new_value)
			VALUES ('order', p_order_id, 'status_change', 
				JSON_OBJECT('status', v_old_status), 
				JSON_OBJECT('status', p_new_status));
			
			-- Update inventory if shipped
			IF p_new_status = 'shipped' THEN
				INSERT INTO inventory_transactions (product_id, transaction_type, quantity, reference_type, reference_id)
				SELECT product_id, 'out', quantity, 'order', order_id
				FROM order_items
				WHERE order_id = p_order_id;
				
				UPDATE products p
				JOIN order_items oi ON p.id = oi.product_id
				SET p.stock_quantity = p.stock_quantity - oi.quantity
				WHERE oi.order_id = p_order_id;
			END IF;
			
			COMMIT;
		END`,

		`DROP PROCEDURE IF EXISTS calculate_customer_metrics`,
		`CREATE PROCEDURE calculate_customer_metrics(
			IN p_customer_id INT
		)
		BEGIN
			-- Complex aggregation query
			SELECT 
				c.id,
				c.name,
				COUNT(DISTINCT o.id) as total_orders,
				COALESCE(SUM(o.total_amount), 0) as lifetime_value,
				COALESCE(AVG(o.total_amount), 0) as avg_order_value,
				MAX(o.order_date) as last_order_date,
				COUNT(DISTINCT oi.product_id) as unique_products_ordered,
				COUNT(DISTINCT DATE(o.order_date)) as active_days
			FROM customers c
			LEFT JOIN orders o ON c.id = o.customer_id
			LEFT JOIN order_items oi ON o.id = oi.order_id
			WHERE c.id = p_customer_id
			GROUP BY c.id, c.name;
		END`,
	}

	for _, proc := range procedures {
		if _, err := dg.db.Exec(proc); err != nil {
			return fmt.Errorf("failed to create procedure: %v", err)
		}
	}

	return nil
}

// PopulateBaseData creates initial dataset
func (dg *DataGenerator) PopulateBaseData() error {
	// Insert customers
	for i := 0; i < 10000; i++ {
		email := fmt.Sprintf("customer%d@example.com", i)
		name := fmt.Sprintf("Customer %d", i)
		status := []string{"active", "inactive", "suspended"}[rand.Intn(3)]
		creditScore := rand.Intn(850)
		
		_, err := dg.db.Exec(`
			INSERT INTO customers (email, name, status, credit_score)
			VALUES (?, ?, ?, ?)
		`, email, name, status, creditScore)
		
		if err != nil && !strings.Contains(err.Error(), "Duplicate") {
			return err
		}
	}

	// Insert products
	categories := []string{"Electronics", "Clothing", "Books", "Home", "Sports", "Toys", "Food", "Beauty"}
	for i := 0; i < 5000; i++ {
		sku := fmt.Sprintf("SKU-%05d", i)
		name := fmt.Sprintf("Product %d", i)
		description := fmt.Sprintf("Description for product %d with various features and specifications", i)
		price := float64(rand.Intn(100000)) / 100.0
		stock := rand.Intn(1000)
		category := rand.Intn(len(categories))
		
		_, err := dg.db.Exec(`
			INSERT INTO products (sku, name, description, price, stock_quantity, category_id)
			VALUES (?, ?, ?, ?, ?, ?)
		`, sku, name, description, price, stock, category)
		
		if err != nil && !strings.Contains(err.Error(), "Duplicate") {
			return err
		}
	}

	// Insert historical orders
	for i := 0; i < 50000; i++ {
		orderNum := fmt.Sprintf("ORD-%010d", i)
		customerID := rand.Intn(10000) + 1
		orderDate := time.Now().Add(-time.Duration(rand.Intn(365)) * 24 * time.Hour)
		status := []string{"pending", "processing", "shipped", "delivered", "cancelled"}[rand.Intn(5)]
		total := float64(rand.Intn(500000)) / 100.0
		
		result, err := dg.db.Exec(`
			INSERT INTO orders (order_number, customer_id, order_date, status, total_amount, shipping_address, payment_method)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, orderNum, customerID, orderDate, status, total, "123 Main St, City, State", "credit_card")
		
		if err != nil && !strings.Contains(err.Error(), "Duplicate") {
			return err
		}

		orderID, _ := result.LastInsertId()
		
		// Add order items
		itemCount := rand.Intn(5) + 1
		for j := 0; j < itemCount; j++ {
			productID := rand.Intn(5000) + 1
			quantity := rand.Intn(5) + 1
			unitPrice := float64(rand.Intn(10000)) / 100.0
			
			_, err = dg.db.Exec(`
				INSERT INTO order_items (order_id, product_id, quantity, unit_price)
				VALUES (?, ?, ?, ?)
			`, orderID, productID, quantity, unitPrice)
			
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GenerateIOIntensiveWorkload creates queries that cause I/O waits
func (dg *DataGenerator) GenerateIOIntensiveWorkload(duration time.Duration, concurrency int) {
	dg.wg.Add(concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer dg.wg.Done()
			
			endTime := time.Now().Add(duration)
			queries := []string{
				// Full table scan on large table
				`SELECT COUNT(*) FROM analytics_events WHERE event_data LIKE '%search%'`,
				
				// Join without proper indexes
				`SELECT s.search_term, COUNT(*) as search_count, AVG(s.result_count) as avg_results
				 FROM search_logs s
				 JOIN customers c ON s.customer_id = c.id
				 WHERE s.search_term LIKE ? 
				 GROUP BY s.search_term
				 ORDER BY search_count DESC`,
				
				// Large range scan
				`SELECT * FROM orders 
				 WHERE order_date BETWEEN ? AND ? 
				 AND total_amount > ?
				 ORDER BY total_amount DESC`,
				
				// Subquery causing temp tables
				`SELECT c.name, 
					(SELECT COUNT(*) FROM orders WHERE customer_id = c.id) as order_count,
					(SELECT SUM(total_amount) FROM orders WHERE customer_id = c.id) as total_spent,
					(SELECT MAX(order_date) FROM orders WHERE customer_id = c.id) as last_order
				 FROM customers c
				 WHERE c.status = 'active'
				 ORDER BY total_spent DESC
				 LIMIT 100`,
			}
			
			for time.Now().Before(endTime) {
				select {
				case <-dg.stopCh:
					return
				default:
					query := queries[rand.Intn(len(queries))]
					
					switch rand.Intn(4) {
					case 0:
						dg.db.Exec(query)
					case 1:
						searchTerm := fmt.Sprintf("%%%s%%", []string{"laptop", "phone", "tablet", "camera"}[rand.Intn(4)])
						dg.db.Exec(query, searchTerm)
					case 2:
						startDate := time.Now().Add(-time.Duration(rand.Intn(30)) * 24 * time.Hour)
						endDate := time.Now()
						amount := rand.Float64() * 1000
						dg.db.Exec(query, startDate, endDate, amount)
					case 3:
						dg.db.Exec(query)
					}
					
					dg.metricsCollector.recordQuery()
					time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
				}
			}
		}(i)
	}
}

// GenerateLockIntensiveWorkload creates queries that cause lock waits
func (dg *DataGenerator) GenerateLockIntensiveWorkload(duration time.Duration, concurrency int) {
	dg.wg.Add(concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer dg.wg.Done()
			
			endTime := time.Now().Add(duration)
			
			for time.Now().Before(endTime) {
				select {
				case <-dg.stopCh:
					return
				default:
					// Randomly choose lock scenario
					switch rand.Intn(4) {
					case 0:
						// Conflicting order updates
						dg.generateOrderLockConflict()
					case 1:
						// Inventory update conflicts
						dg.generateInventoryLockConflict()
					case 2:
						// Hot row updates
						dg.generateHotRowLock()
					case 3:
						// Deadlock scenario
						dg.generateDeadlockScenario(workerID)
					}
					
					dg.metricsCollector.recordLock()
					time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
				}
			}
		}(i)
	}
}

// generateOrderLockConflict creates order processing conflicts
func (dg *DataGenerator) generateOrderLockConflict() {
	orderID := rand.Intn(1000) + 1
	
	tx, err := dg.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	
	// Lock order for processing
	_, err = tx.Exec(`
		SELECT * FROM orders WHERE id = ? FOR UPDATE
	`, orderID)
	
	if err != nil {
		return
	}
	
	// Simulate processing time
	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
	
	// Update order
	newStatus := []string{"processing", "shipped", "delivered"}[rand.Intn(3)]
	_, err = tx.Exec(`
		UPDATE orders SET status = ?, updated_at = NOW() WHERE id = ?
	`, newStatus, orderID)
	
	// Random chance to commit or rollback
	if rand.Float64() > 0.1 {
		tx.Commit()
	}
}

// generateInventoryLockConflict creates inventory update conflicts
func (dg *DataGenerator) generateInventoryLockConflict() {
	productID := rand.Intn(100) + 1 // Focus on hot products
	
	tx, err := dg.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	
	// Lock product for inventory update
	var currentStock int
	err = tx.QueryRow(`
		SELECT stock_quantity FROM products WHERE id = ? FOR UPDATE
	`, productID).Scan(&currentStock)
	
	if err != nil {
		return
	}
	
	// Simulate decision time
	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
	
	// Update stock
	change := rand.Intn(20) - 10
	newStock := currentStock + change
	if newStock < 0 {
		newStock = 0
	}
	
	_, err = tx.Exec(`
		UPDATE products SET stock_quantity = ? WHERE id = ?
	`, newStock, productID)
	
	if err == nil {
		// Record transaction
		_, _ = tx.Exec(`
			INSERT INTO inventory_transactions (product_id, transaction_type, quantity)
			VALUES (?, ?, ?)
		`, productID, "adjustment", change)
		
		tx.Commit()
	}
}

// generateHotRowLock creates contention on frequently accessed rows
func (dg *DataGenerator) generateHotRowLock() {
	// Popular customers (hot rows)
	customerID := []int{1, 2, 3, 4, 5}[rand.Intn(5)]
	
	tx, err := dg.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	
	// Update customer credit score (common operation)
	_, err = tx.Exec(`
		UPDATE customers 
		SET credit_score = credit_score + ?, 
		    last_login = NOW()
		WHERE id = ?
	`, rand.Intn(10)-5, customerID)
	
	if err != nil {
		return
	}
	
	// Simulate additional processing
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
	
	// Update session
	sessionID := fmt.Sprintf("session_%d_%d", customerID, time.Now().Unix())
	_, _ = tx.Exec(`
		INSERT INTO user_sessions (id, customer_id, ip_address)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE last_activity = NOW()
	`, sessionID, customerID, "192.168.1.1")
	
	tx.Commit()
}

// generateDeadlockScenario creates potential deadlocks
func (dg *DataGenerator) generateDeadlockScenario(workerID int) {
	// Two resources that will be locked in different order
	resource1 := rand.Intn(10) + 1
	resource2 := rand.Intn(10) + 11
	
	if workerID%2 == 0 {
		// Even workers: lock order -> product
		tx, _ := dg.db.Begin()
		defer tx.Rollback()
		
		tx.Exec("SELECT * FROM orders WHERE id = ? FOR UPDATE", resource1)
		time.Sleep(50 * time.Millisecond)
		tx.Exec("SELECT * FROM products WHERE id = ? FOR UPDATE", resource2)
		
		tx.Commit()
	} else {
		// Odd workers: lock product -> order
		tx, _ := dg.db.Begin()
		defer tx.Rollback()
		
		tx.Exec("SELECT * FROM products WHERE id = ? FOR UPDATE", resource2)
		time.Sleep(50 * time.Millisecond)
		tx.Exec("SELECT * FROM orders WHERE id = ? FOR UPDATE", resource1)
		
		tx.Commit()
	}
}

// GenerateCPUIntensiveWorkload creates queries that cause CPU waits
func (dg *DataGenerator) GenerateCPUIntensiveWorkload(duration time.Duration, concurrency int) {
	dg.wg.Add(concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func() {
			defer dg.wg.Done()
			
			endTime := time.Now().Add(duration)
			
			for time.Now().Before(endTime) {
				select {
				case <-dg.stopCh:
					return
				default:
					// Complex analytical queries
					queries := []string{
						// Heavy aggregation
						`SELECT 
							DATE(o.order_date) as order_day,
							COUNT(DISTINCT o.customer_id) as unique_customers,
							COUNT(o.id) as total_orders,
							SUM(o.total_amount) as revenue,
							AVG(o.total_amount) as avg_order_value,
							STDDEV(o.total_amount) as stddev_order_value,
							MAX(o.total_amount) as max_order,
							MIN(o.total_amount) as min_order
						FROM orders o
						WHERE o.order_date >= DATE_SUB(NOW(), INTERVAL 30 DAY)
						GROUP BY DATE(o.order_date)
						WITH ROLLUP`,
						
						// Complex joins with calculations
						`SELECT 
							c.id,
							c.name,
							COUNT(DISTINCT o.id) as order_count,
							COUNT(DISTINCT oi.product_id) as unique_products,
							SUM(oi.quantity * oi.unit_price) as total_revenue,
							AVG(oi.quantity * oi.unit_price) as avg_item_value,
							RANK() OVER (ORDER BY SUM(oi.quantity * oi.unit_price) DESC) as revenue_rank,
							DENSE_RANK() OVER (ORDER BY COUNT(DISTINCT o.id) DESC) as order_count_rank
						FROM customers c
						JOIN orders o ON c.id = o.customer_id
						JOIN order_items oi ON o.id = oi.order_id
						WHERE o.status != 'cancelled'
						GROUP BY c.id, c.name
						ORDER BY total_revenue DESC
						LIMIT 1000`,
						
						// Recursive CTE (if supported)
						`WITH RECURSIVE category_tree AS (
							SELECT id, name, parent_id, 0 as level
							FROM categories
							WHERE parent_id IS NULL
							UNION ALL
							SELECT c.id, c.name, c.parent_id, ct.level + 1
							FROM categories c
							JOIN category_tree ct ON c.parent_id = ct.id
						)
						SELECT * FROM category_tree`,
						
						// String manipulation and pattern matching
						`SELECT 
							customer_id,
							COUNT(*) as search_count,
							GROUP_CONCAT(DISTINCT search_term ORDER BY search_term) as all_searches,
							SUM(CASE WHEN search_term REGEXP '[0-9]+' THEN 1 ELSE 0 END) as numeric_searches,
							SUM(CASE WHEN LENGTH(search_term) > 20 THEN 1 ELSE 0 END) as long_searches
						FROM search_logs
						WHERE search_timestamp >= DATE_SUB(NOW(), INTERVAL 7 DAY)
						GROUP BY customer_id
						HAVING search_count > 10`,
					}
					
					query := queries[rand.Intn(len(queries))]
					dg.db.Exec(query)
					
					dg.metricsCollector.recordQuery()
					time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				}
			}
		}()
	}
}

// GenerateSlowQueries creates queries that will appear in slow query log
func (dg *DataGenerator) GenerateSlowQueries(duration time.Duration, concurrency int) {
	dg.wg.Add(concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func() {
			defer dg.wg.Done()
			
			endTime := time.Now().Add(duration)
			
			for time.Now().Before(endTime) {
				select {
				case <-dg.stopCh:
					return
				default:
					// Queries designed to be slow
					queries := []string{
						// Cartesian product
						`SELECT COUNT(*) 
						FROM customers c1, customers c2 
						WHERE c1.credit_score > c2.credit_score`,
						
						// Multiple subqueries
						`SELECT 
							p.name,
							(SELECT COUNT(*) FROM order_items WHERE product_id = p.id) as times_ordered,
							(SELECT AVG(quantity) FROM order_items WHERE product_id = p.id) as avg_quantity,
							(SELECT MAX(oi.unit_price) FROM order_items oi WHERE oi.product_id = p.id) as max_price,
							(SELECT MIN(oi.unit_price) FROM order_items oi WHERE oi.product_id = p.id) as min_price
						FROM products p
						WHERE p.stock_quantity < 100`,
						
						// Sleep function
						`SELECT *, SLEEP(1) FROM orders WHERE id = ?`,
					}
					
					query := queries[rand.Intn(len(queries))]
					if strings.Contains(query, "?") {
						dg.db.Exec(query, rand.Intn(100))
					} else {
						dg.db.Exec(query)
					}
					
					dg.metricsCollector.recordSlowQuery()
					time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
				}
			}
		}()
	}
}

// GenerateAdvisoryTriggers creates scenarios that should trigger advisories
func (dg *DataGenerator) GenerateAdvisoryTriggers() error {
	scenarios := []struct {
		name        string
		setup       string
		query       string
		expectedAdvisory string
	}{
		{
			name: "Missing Index",
			setup: `CREATE TABLE IF NOT EXISTS missing_index_test (
				id INT PRIMARY KEY AUTO_INCREMENT,
				user_id INT,
				action VARCHAR(50),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				data JSON
			)`,
			query: `SELECT * FROM missing_index_test 
			        WHERE user_id = ? AND action = ? 
			        ORDER BY created_at DESC LIMIT 10`,
			expectedAdvisory: "missing_index",
		},
		{
			name: "Temp Table to Disk",
			setup: ``,
			query: `SELECT 
				customer_id,
				GROUP_CONCAT(DISTINCT order_number ORDER BY order_date) as all_orders,
				COUNT(*) as order_count
			FROM orders
			GROUP BY customer_id
			HAVING order_count > 5`,
			expectedAdvisory: "temp_table_disk",
		},
		{
			name: "Full Table Scan",
			setup: `CREATE TABLE IF NOT EXISTS full_scan_test (
				id INT PRIMARY KEY AUTO_INCREMENT,
				data TEXT,
				status VARCHAR(20)
			)`,
			query: `SELECT * FROM full_scan_test WHERE data LIKE '%pattern%'`,
			expectedAdvisory: "full_scan",
		},
	}
	
	for _, scenario := range scenarios {
		// Setup
		if scenario.setup != "" {
			if _, err := dg.db.Exec(scenario.setup); err != nil {
				return err
			}
		}
		
		// Execute query multiple times to ensure it's captured
		for i := 0; i < 100; i++ {
			if strings.Contains(scenario.query, "?") {
				dg.db.Exec(scenario.query, rand.Intn(1000), "action_type")
			} else {
				dg.db.Exec(scenario.query)
			}
		}
		
		dg.metricsCollector.recordAdvisory()
	}
	
	return nil
}

// GenerateMixedWorkload combines all workload types
func (dg *DataGenerator) GenerateMixedWorkload(duration time.Duration, concurrency int) {
	// Start different workload types concurrently
	go dg.GenerateIOIntensiveWorkload(duration, concurrency/4)
	go dg.GenerateLockIntensiveWorkload(duration, concurrency/4)
	go dg.GenerateCPUIntensiveWorkload(duration, concurrency/4)
	go dg.GenerateSlowQueries(duration, concurrency/4)
	
	// Also generate normal traffic
	dg.wg.Add(concurrency / 2)
	for i := 0; i < concurrency/2; i++ {
		go func() {
			defer dg.wg.Done()
			dg.generateNormalTraffic(duration)
		}()
	}
}

// generateNormalTraffic simulates typical application queries
func (dg *DataGenerator) generateNormalTraffic(duration time.Duration) {
	endTime := time.Now().Add(duration)
	
	for time.Now().Before(endTime) {
		select {
		case <-dg.stopCh:
			return
		default:
			operations := []func(){
				// Customer login
				func() {
					customerID := rand.Intn(10000) + 1
					dg.db.Exec(`UPDATE customers SET last_login = NOW() WHERE id = ?`, customerID)
				},
				// View product
				func() {
					productID := rand.Intn(5000) + 1
					dg.db.Exec(`SELECT * FROM products WHERE id = ?`, productID)
				},
				// Search products
				func() {
					category := rand.Intn(8) + 1
					dg.db.Exec(`
						SELECT * FROM products 
						WHERE category_id = ? AND stock_quantity > 0 
						ORDER BY price DESC LIMIT 20
					`, category)
				},
				// Check order status
				func() {
					orderNum := fmt.Sprintf("ORD-%010d", rand.Intn(50000))
					dg.db.Exec(`
						SELECT o.*, oi.* 
						FROM orders o 
						JOIN order_items oi ON o.id = oi.order_id 
						WHERE o.order_number = ?
					`, orderNum)
				},
				// Add to cart (session update)
				func() {
					sessionID := fmt.Sprintf("session_%d", rand.Intn(1000))
					dg.db.Exec(`
						INSERT INTO user_sessions (id, customer_id, ip_address)
						VALUES (?, ?, ?)
						ON DUPLICATE KEY UPDATE last_activity = NOW()
					`, sessionID, rand.Intn(10000)+1, "192.168.1.1")
				},
			}
			
			// Execute random operation
			operations[rand.Intn(len(operations))]()
			
			dg.metricsCollector.recordQuery()
			
			// Variable think time
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		}
	}
}

// Stop gracefully stops all workload generation
func (dg *DataGenerator) Stop() {
	close(dg.stopCh)
	dg.wg.Wait()
}

// GetMetrics returns collected metrics
func (dg *DataGenerator) GetMetrics() map[string]int64 {
	dg.metricsCollector.mu.Lock()
	defer dg.metricsCollector.mu.Unlock()
	
	return map[string]int64{
		"queries_executed":     dg.metricsCollector.queriesExecuted,
		"waits_generated":      dg.metricsCollector.waitsGenerated,
		"locks_generated":      dg.metricsCollector.locksGenerated,
		"slow_queries":         dg.metricsCollector.slowQueries,
		"advisories_triggered": dg.metricsCollector.advisoriesTriggered,
	}
}

// Helper methods for MetricsCollector
func (mc *MetricsCollector) recordQuery() {
	mc.mu.Lock()
	mc.queriesExecuted++
	mc.mu.Unlock()
}

func (mc *MetricsCollector) recordWait() {
	mc.mu.Lock()
	mc.waitsGenerated++
	mc.mu.Unlock()
}

func (mc *MetricsCollector) recordLock() {
	mc.mu.Lock()
	mc.locksGenerated++
	mc.mu.Unlock()
}

func (mc *MetricsCollector) recordSlowQuery() {
	mc.mu.Lock()
	mc.slowQueries++
	mc.mu.Unlock()
}

func (mc *MetricsCollector) recordAdvisory() {
	mc.mu.Lock()
	mc.advisoriesTriggered++
	mc.mu.Unlock()
}