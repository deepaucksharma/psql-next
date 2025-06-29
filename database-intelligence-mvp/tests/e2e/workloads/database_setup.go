// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package workloads

import (
	"fmt"
)

// preparePostgreSQLTestData creates test schema and sample data for PostgreSQL
func (wg *WorkloadGenerator) preparePostgreSQLTestData() error {
	wg.logger.Info("Preparing PostgreSQL test data")
	
	// Create extensions first
	if err := wg.createPostgreSQLExtensions(); err != nil {
		return fmt.Errorf("failed to create extensions: %w", err)
	}
	
	// Create schema
	if err := wg.createPostgreSQLSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	
	// Insert sample data
	if err := wg.insertPostgreSQLSampleData(); err != nil {
		return fmt.Errorf("failed to insert sample data: %w", err)
	}
	
	return nil
}

// createPostgreSQLExtensions creates necessary PostgreSQL extensions
func (wg *WorkloadGenerator) createPostgreSQLExtensions() error {
	extensions := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
		"CREATE EXTENSION IF NOT EXISTS pg_trgm",
		"CREATE EXTENSION IF NOT EXISTS btree_gin",
		"SELECT pg_stat_statements_reset()",
	}
	
	for _, ext := range extensions {
		if _, err := wg.db.Exec(ext); err != nil {
			wg.logger.Error("Failed to create extension", 
				zap.String("extension", ext), 
				zap.Error(err))
			// Don't fail on extension errors as they might already exist
		}
	}
	
	return nil
}

// createPostgreSQLSchema creates the test database schema
func (wg *WorkloadGenerator) createPostgreSQLSchema() error {
	schemaSQL := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			first_name VARCHAR(255),
			last_name VARCHAR(255),
			phone VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Products table
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			category VARCHAR(100),
			sku VARCHAR(100) UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Orders table
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			total_amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Order items table
		`CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INTEGER REFERENCES orders(id),
			product_id INTEGER REFERENCES products(id),
			quantity INTEGER NOT NULL,
			unit_price DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Inventory table
		`CREATE TABLE IF NOT EXISTS inventory (
			id SERIAL PRIMARY KEY,
			product_id INTEGER REFERENCES products(id),
			quantity INTEGER NOT NULL DEFAULT 0,
			reserved_quantity INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Access logs table
		`CREATE TABLE IF NOT EXISTS access_logs (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			action VARCHAR(255),
			ip_address INET,
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// User sessions table
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			session_token VARCHAR(255) UNIQUE,
			last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Sensitive data table (for PII testing)
		`CREATE TABLE IF NOT EXISTS sensitive_data (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			ssn VARCHAR(11),
			credit_card VARCHAR(19),
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		// Performance testing table
		`CREATE TABLE IF NOT EXISTS performance_test_data (
			id SERIAL PRIMARY KEY,
			user_id INTEGER,
			large_text TEXT,
			json_data JSONB,
			numeric_value DECIMAL(15,5),
			timestamp_value TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	
	// Create indexes for performance
	indexSQL := []string{
		"CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)",
		"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)",
		"CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)",
		"CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id)",
		"CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(product_id)",
		"CREATE INDEX IF NOT EXISTS idx_products_category ON products(category)",
		"CREATE INDEX IF NOT EXISTS idx_products_sku ON products(sku)",
		"CREATE INDEX IF NOT EXISTS idx_access_logs_user_id ON access_logs(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_access_logs_created_at ON access_logs(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(session_token)",
		"CREATE INDEX IF NOT EXISTS idx_performance_test_user_id ON performance_test_data(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_performance_test_timestamp ON performance_test_data(timestamp_value)",
		
		// GIN indexes for full-text search
		"CREATE INDEX IF NOT EXISTS idx_products_name_gin ON products USING gin(name gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_products_description_gin ON products USING gin(description gin_trgm_ops)",
		
		// JSONB indexes
		"CREATE INDEX IF NOT EXISTS idx_performance_test_json ON performance_test_data USING gin(json_data)",
	}
	
	// Execute schema creation
	for _, sql := range schemaSQL {
		if _, err := wg.db.Exec(sql); err != nil {
			return fmt.Errorf("failed to execute schema SQL: %w", err)
		}
	}
	
	// Execute index creation
	for _, sql := range indexSQL {
		if _, err := wg.db.Exec(sql); err != nil {
			wg.logger.Warn("Failed to create index", 
				zap.String("sql", sql), 
				zap.Error(err))
			// Don't fail on index errors as they might already exist
		}
	}
	
	return nil
}

// insertPostgreSQLSampleData inserts sample data for testing
func (wg *WorkloadGenerator) insertPostgreSQLSampleData() error {
	generator := wg.dataGenerators["default"]
	
	// Insert users
	userCount := wg.config.DataSetSize.Users
	for i := 0; i < userCount; i++ {
		username := fmt.Sprintf("user_%d", i+1)
		email := fmt.Sprintf("user%d@example.com", i+1)
		firstName := generator.usernames[i%len(generator.usernames)]
		lastName := "User"
		phone := generator.phoneNumbers[i%len(generator.phoneNumbers)]
		
		_, err := wg.db.Exec(`
			INSERT INTO users (username, email, first_name, last_name, phone) 
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (username) DO NOTHING
		`, username, email, firstName, lastName, phone)
		
		if err != nil {
			return fmt.Errorf("failed to insert user: %w", err)
		}
	}
	
	// Insert products
	productCount := wg.config.DataSetSize.Products
	categories := []string{"electronics", "clothing", "books", "home", "sports"}
	
	for i := 0; i < productCount; i++ {
		name := fmt.Sprintf("Product %d", i+1)
		description := fmt.Sprintf("Description for product %d", i+1)
		price := 10.0 + float64(i%490) // Prices from $10 to $500
		category := categories[i%len(categories)]
		sku := fmt.Sprintf("SKU-%06d", i+1)
		
		_, err := wg.db.Exec(`
			INSERT INTO products (name, description, price, category, sku) 
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (sku) DO NOTHING
		`, name, description, price, category, sku)
		
		if err != nil {
			return fmt.Errorf("failed to insert product: %w", err)
		}
	}
	
	// Insert inventory
	_, err := wg.db.Exec(`
		INSERT INTO inventory (product_id, quantity)
		SELECT id, (RANDOM() * 100)::INTEGER + 10
		FROM products
		ON CONFLICT DO NOTHING
	`)
	
	if err != nil {
		return fmt.Errorf("failed to insert inventory: %w", err)
	}
	
	// Insert orders
	orderCount := wg.config.DataSetSize.Orders
	statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
	
	for i := 0; i < orderCount; i++ {
		userID := (i % userCount) + 1
		totalAmount := 50.0 + float64(i%950) // Orders from $50 to $1000
		status := statuses[i%len(statuses)]
		
		var orderID int
		err := wg.db.QueryRow(`
			INSERT INTO orders (user_id, total_amount, status) 
			VALUES ($1, $2, $3) RETURNING id
		`, userID, totalAmount, status).Scan(&orderID)
		
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}
		
		// Insert order items (1-3 items per order)
		itemCount := (i % 3) + 1
		for j := 0; j < itemCount; j++ {
			productID := (generator.random.Intn(productCount)) + 1
			quantity := generator.random.Intn(5) + 1
			unitPrice := 10.0 + float64(generator.random.Intn(190)) // $10-$200
			
			_, err := wg.db.Exec(`
				INSERT INTO order_items (order_id, product_id, quantity, unit_price) 
				VALUES ($1, $2, $3, $4)
			`, orderID, productID, quantity, unitPrice)
			
			if err != nil {
				return fmt.Errorf("failed to insert order item: %w", err)
			}
		}
	}
	
	// Insert access logs
	logCount := wg.config.DataSetSize.LogEntries
	actions := []string{"login", "logout", "view_product", "add_to_cart", "checkout", "search"}
	
	for i := 0; i < logCount; i++ {
		userID := (i % userCount) + 1
		action := actions[i%len(actions)]
		ipAddress := fmt.Sprintf("192.168.1.%d", (i%254)+1)
		userAgent := "Mozilla/5.0 (Test Browser)"
		
		_, err := wg.db.Exec(`
			INSERT INTO access_logs (user_id, action, ip_address, user_agent) 
			VALUES ($1, $2, $3, $4)
		`, userID, action, ipAddress, userAgent)
		
		if err != nil {
			return fmt.Errorf("failed to insert access log: %w", err)
		}
	}
	
	// Insert performance test data
	for i := 0; i < 1000; i++ {
		userID := (i % userCount) + 1
		largeText := fmt.Sprintf("Large text data for performance testing - record %d. %s", 
			i, strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 20))
		jsonData := fmt.Sprintf(`{"id": %d, "type": "test", "data": "%s", "timestamp": "%s"}`, 
			i, fmt.Sprintf("data_%d", i), time.Now().Format(time.RFC3339))
		numericValue := float64(i) * 3.14159
		timestampValue := time.Now().Add(-time.Duration(i) * time.Hour)
		
		_, err := wg.db.Exec(`
			INSERT INTO performance_test_data (user_id, large_text, json_data, numeric_value, timestamp_value) 
			VALUES ($1, $2, $3, $4, $5)
		`, userID, largeText, jsonData, numericValue, timestampValue)
		
		if err != nil {
			return fmt.Errorf("failed to insert performance test data: %w", err)
		}
	}
	
	wg.logger.Info("PostgreSQL test data preparation completed",
		zap.Int("users", userCount),
		zap.Int("products", productCount),
		zap.Int("orders", orderCount),
		zap.Int("logs", logCount))
	
	return nil
}

// prepareMySQLTestData creates test schema and sample data for MySQL
func (wg *WorkloadGenerator) prepareMySQLTestData() error {
	wg.logger.Info("Preparing MySQL test data")
	
	// Enable performance schema and slow query log
	if err := wg.configureMySQLSettings(); err != nil {
		return fmt.Errorf("failed to configure MySQL settings: %w", err)
	}
	
	// Create schema
	if err := wg.createMySQLSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	
	// Insert sample data
	if err := wg.insertMySQLSampleData(); err != nil {
		return fmt.Errorf("failed to insert sample data: %w", err)
	}
	
	return nil
}

// configureMySQLSettings configures MySQL for testing
func (wg *WorkloadGenerator) configureMySQLSettings() error {
	settings := []string{
		"SET GLOBAL performance_schema = ON",
		"SET GLOBAL slow_query_log = ON",
		"SET GLOBAL long_query_time = 1",
		"SET GLOBAL log_queries_not_using_indexes = ON",
		"UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'statement/%'",
		"UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME LIKE 'events_statements_%'",
	}
	
	for _, setting := range settings {
		if _, err := wg.db.Exec(setting); err != nil {
			wg.logger.Warn("Failed to apply MySQL setting", 
				zap.String("setting", setting), 
				zap.Error(err))
			// Don't fail on setting errors as some might require privileges
		}
	}
	
	return nil
}

// createMySQLSchema creates the test database schema for MySQL
func (wg *WorkloadGenerator) createMySQLSchema() error {
	schemaSQL := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			first_name VARCHAR(255),
			last_name VARCHAR(255),
			phone VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_username (username),
			INDEX idx_email (email)
		) ENGINE=InnoDB`,
		
		// Products table
		`CREATE TABLE IF NOT EXISTS products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			category VARCHAR(100),
			sku VARCHAR(100) UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_category (category),
			INDEX idx_sku (sku),
			FULLTEXT idx_name_description (name, description)
		) ENGINE=InnoDB`,
		
		// Orders table
		`CREATE TABLE IF NOT EXISTS orders (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			total_amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_created_at (created_at),
			INDEX idx_status (status),
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB`,
		
		// Order items table
		`CREATE TABLE IF NOT EXISTS order_items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			order_id INT,
			product_id INT,
			quantity INT NOT NULL,
			unit_price DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_order_id (order_id),
			INDEX idx_product_id (product_id),
			FOREIGN KEY (order_id) REFERENCES orders(id),
			FOREIGN KEY (product_id) REFERENCES products(id)
		) ENGINE=InnoDB`,
		
		// Inventory table
		`CREATE TABLE IF NOT EXISTS inventory (
			id INT AUTO_INCREMENT PRIMARY KEY,
			product_id INT,
			quantity INT NOT NULL DEFAULT 0,
			reserved_quantity INT NOT NULL DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_product_id (product_id),
			FOREIGN KEY (product_id) REFERENCES products(id)
		) ENGINE=InnoDB`,
		
		// Access logs table
		`CREATE TABLE IF NOT EXISTS access_logs (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			action VARCHAR(255),
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_created_at (created_at),
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB`,
		
		// User sessions table
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			session_token VARCHAR(255) UNIQUE,
			last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_session_token (session_token),
			INDEX idx_last_activity (last_activity),
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB`,
		
		// Sensitive data table (for PII testing)
		`CREATE TABLE IF NOT EXISTS sensitive_data (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			ssn VARCHAR(11),
			credit_card VARCHAR(19),
			notes TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		) ENGINE=InnoDB`,
		
		// Performance testing table
		`CREATE TABLE IF NOT EXISTS performance_test_data (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			large_text TEXT,
			json_data JSON,
			numeric_value DECIMAL(15,5),
			timestamp_value TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_timestamp_value (timestamp_value)
		) ENGINE=InnoDB`,
	}
	
	// Execute schema creation
	for _, sql := range schemaSQL {
		if _, err := wg.db.Exec(sql); err != nil {
			return fmt.Errorf("failed to execute schema SQL: %w", err)
		}
	}
	
	return nil
}

// insertMySQLSampleData inserts sample data for MySQL testing
func (wg *WorkloadGenerator) insertMySQLSampleData() error {
	generator := wg.dataGenerators["default"]
	
	// Insert users
	userCount := wg.config.DataSetSize.Users
	for i := 0; i < userCount; i++ {
		username := fmt.Sprintf("user_%d", i+1)
		email := fmt.Sprintf("user%d@example.com", i+1)
		firstName := generator.usernames[i%len(generator.usernames)]
		lastName := "User"
		phone := generator.phoneNumbers[i%len(generator.phoneNumbers)]
		
		_, err := wg.db.Exec(`
			INSERT IGNORE INTO users (username, email, first_name, last_name, phone) 
			VALUES (?, ?, ?, ?, ?)
		`, username, email, firstName, lastName, phone)
		
		if err != nil {
			return fmt.Errorf("failed to insert user: %w", err)
		}
	}
	
	// Insert products
	productCount := wg.config.DataSetSize.Products
	categories := []string{"electronics", "clothing", "books", "home", "sports"}
	
	for i := 0; i < productCount; i++ {
		name := fmt.Sprintf("Product %d", i+1)
		description := fmt.Sprintf("Description for product %d with various keywords for search testing", i+1)
		price := 10.0 + float64(i%490) // Prices from $10 to $500
		category := categories[i%len(categories)]
		sku := fmt.Sprintf("SKU-%06d", i+1)
		
		_, err := wg.db.Exec(`
			INSERT IGNORE INTO products (name, description, price, category, sku) 
			VALUES (?, ?, ?, ?, ?)
		`, name, description, price, category, sku)
		
		if err != nil {
			return fmt.Errorf("failed to insert product: %w", err)
		}
	}
	
	// Insert inventory
	_, err := wg.db.Exec(`
		INSERT IGNORE INTO inventory (product_id, quantity)
		SELECT id, FLOOR(RAND() * 100) + 10
		FROM products
	`)
	
	if err != nil {
		return fmt.Errorf("failed to insert inventory: %w", err)
	}
	
	// Insert orders and order items (similar to PostgreSQL but using MySQL syntax)
	orderCount := wg.config.DataSetSize.Orders
	statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
	
	for i := 0; i < orderCount; i++ {
		userID := (i % userCount) + 1
		totalAmount := 50.0 + float64(i%950)
		status := statuses[i%len(statuses)]
		
		result, err := wg.db.Exec(`
			INSERT INTO orders (user_id, total_amount, status) 
			VALUES (?, ?, ?)
		`, userID, totalAmount, status)
		
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}
		
		orderID, _ := result.LastInsertId()
		
		// Insert order items
		itemCount := (i % 3) + 1
		for j := 0; j < itemCount; j++ {
			productID := (generator.random.Intn(productCount)) + 1
			quantity := generator.random.Intn(5) + 1
			unitPrice := 10.0 + float64(generator.random.Intn(190))
			
			_, err := wg.db.Exec(`
				INSERT INTO order_items (order_id, product_id, quantity, unit_price) 
				VALUES (?, ?, ?, ?)
			`, orderID, productID, quantity, unitPrice)
			
			if err != nil {
				return fmt.Errorf("failed to insert order item: %w", err)
			}
		}
	}
	
	// Insert access logs
	logCount := wg.config.DataSetSize.LogEntries
	actions := []string{"login", "logout", "view_product", "add_to_cart", "checkout", "search"}
	
	for i := 0; i < logCount; i++ {
		userID := (i % userCount) + 1
		action := actions[i%len(actions)]
		ipAddress := fmt.Sprintf("192.168.1.%d", (i%254)+1)
		userAgent := "Mozilla/5.0 (Test Browser)"
		
		_, err := wg.db.Exec(`
			INSERT INTO access_logs (user_id, action, ip_address, user_agent) 
			VALUES (?, ?, ?, ?)
		`, userID, action, ipAddress, userAgent)
		
		if err != nil {
			return fmt.Errorf("failed to insert access log: %w", err)
		}
	}
	
	wg.logger.Info("MySQL test data preparation completed",
		zap.Int("users", userCount),
		zap.Int("products", productCount),
		zap.Int("orders", orderCount),
		zap.Int("logs", logCount))
	
	return nil
}

// initializePostgreSQLDatabase performs PostgreSQL-specific initialization
func (suite *E2EMetricsFlowTestSuite) initializePostgreSQLDatabase() {
	// Enable required extensions and configure for testing
	queries := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
		"SELECT pg_stat_statements_reset()",
		"ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements'",
		"ALTER SYSTEM SET pg_stat_statements.track = 'all'",
		"ALTER SYSTEM SET log_statement = 'all'",
		"ALTER SYSTEM SET log_min_duration_statement = 0",
	}
	
	for _, query := range queries {
		if _, err := suite.pgDB.Exec(query); err != nil {
			suite.logger.Warn("Failed to execute PostgreSQL initialization query",
				zap.String("query", query),
				zap.Error(err))
		}
	}
}

// initializeMySQLDatabase performs MySQL-specific initialization
func (suite *E2EMetricsFlowTestSuite) initializeMySQLDatabase() {
	// Configure MySQL for testing
	queries := []string{
		"SET GLOBAL performance_schema = ON",
		"SET GLOBAL slow_query_log = ON",
		"SET GLOBAL long_query_time = 0",
		"SET GLOBAL log_queries_not_using_indexes = ON",
		"SET GLOBAL general_log = ON",
	}
	
	for _, query := range queries {
		if _, err := suite.mysqlDB.Exec(query); err != nil {
			suite.logger.Warn("Failed to execute MySQL initialization query",
				zap.String("query", query),
				zap.Error(err))
		}
	}
}