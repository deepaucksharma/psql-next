package framework

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// TestDataGenerator generates test data for databases
type TestDataGenerator struct {
	pgDB    *sql.DB
	mysqlDB *sql.DB
	random  *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator(pgDB, mysqlDB *sql.DB) *TestDataGenerator {
	return &TestDataGenerator{
		pgDB:    pgDB,
		mysqlDB: mysqlDB,
		random:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GeneratePostgreSQLSchema creates a comprehensive test schema for PostgreSQL
func (g *TestDataGenerator) GeneratePostgreSQLSchema(ctx context.Context) error {
	schema := []string{
		// Enable extensions
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		`CREATE EXTENSION IF NOT EXISTS "pg_stat_statements"`,
		`CREATE EXTENSION IF NOT EXISTS "pg_trgm"`,
		
		// Users table
		`CREATE TABLE IF NOT EXISTS test_users (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			username VARCHAR(50) UNIQUE NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			full_name VARCHAR(200),
			phone VARCHAR(20),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			is_active BOOLEAN DEFAULT true,
			metadata JSONB
		)`,
		
		// Products table
		`CREATE TABLE IF NOT EXISTS test_products (
			id SERIAL PRIMARY KEY,
			sku VARCHAR(50) UNIQUE NOT NULL,
			name VARCHAR(200) NOT NULL,
			description TEXT,
			price NUMERIC(10,2) NOT NULL CHECK (price >= 0),
			category VARCHAR(50),
			stock_quantity INTEGER DEFAULT 0 CHECK (stock_quantity >= 0),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			attributes JSONB
		)`,
		
		// Orders table
		`CREATE TABLE IF NOT EXISTS test_orders (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES test_users(id) ON DELETE CASCADE,
			order_number VARCHAR(50) UNIQUE NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			total_amount NUMERIC(10,2) NOT NULL CHECK (total_amount >= 0),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			shipping_address JSONB,
			payment_info JSONB
		)`,
		
		// Order items table
		`CREATE TABLE IF NOT EXISTS test_order_items (
			id SERIAL PRIMARY KEY,
			order_id UUID NOT NULL REFERENCES test_orders(id) ON DELETE CASCADE,
			product_id INTEGER NOT NULL REFERENCES test_products(id),
			quantity INTEGER NOT NULL CHECK (quantity > 0),
			unit_price NUMERIC(10,2) NOT NULL CHECK (unit_price >= 0),
			discount_amount NUMERIC(10,2) DEFAULT 0 CHECK (discount_amount >= 0),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		
		// Activity logs table
		`CREATE TABLE IF NOT EXISTS test_activity_logs (
			id BIGSERIAL PRIMARY KEY,
			user_id UUID REFERENCES test_users(id) ON DELETE SET NULL,
			action VARCHAR(100) NOT NULL,
			entity_type VARCHAR(50),
			entity_id VARCHAR(100),
			ip_address INET,
			user_agent TEXT,
			request_data JSONB,
			response_data JSONB,
			duration_ms INTEGER,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		
		// Metrics table
		`CREATE TABLE IF NOT EXISTS test_metrics (
			id BIGSERIAL PRIMARY KEY,
			metric_name VARCHAR(100) NOT NULL,
			metric_value NUMERIC NOT NULL,
			tags JSONB,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_users_email ON test_users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON test_users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_users_created_at ON test_users(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_products_sku ON test_products(sku)`,
		`CREATE INDEX IF NOT EXISTS idx_products_category ON test_products(category)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON test_orders(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON test_orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created_at ON test_orders(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON test_order_items(order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON test_order_items(product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_user_id ON test_activity_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at ON test_activity_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_name_timestamp ON test_metrics(metric_name, timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_tags ON test_metrics USING GIN(tags)`,
		
		// Create materialized view
		`CREATE MATERIALIZED VIEW IF NOT EXISTS test_order_summary AS
			SELECT 
				o.id,
				o.order_number,
				o.status,
				o.total_amount,
				o.created_at,
				u.username,
				u.email,
				COUNT(oi.id) as item_count,
				SUM(oi.quantity) as total_quantity
			FROM test_orders o
			JOIN test_users u ON o.user_id = u.id
			LEFT JOIN test_order_items oi ON o.id = oi.order_id
			GROUP BY o.id, o.order_number, o.status, o.total_amount, o.created_at, u.username, u.email`,
		
		// Create function for triggers
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql'`,
		
		// Create triggers
		`CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON test_users
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
		`CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON test_products
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
		`CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON test_orders
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}
	
	for _, stmt := range schema {
		if _, err := g.pgDB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute schema statement: %w", err)
		}
	}
	
	return nil
}

// GenerateMySQLSchema creates a test schema for MySQL
func (g *TestDataGenerator) GenerateMySQLSchema(ctx context.Context) error {
	if g.mysqlDB == nil {
		return nil
	}
	
	schema := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS test_users (
			id CHAR(36) PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			full_name VARCHAR(200),
			phone VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			is_active BOOLEAN DEFAULT true,
			metadata JSON
		)`,
		
		// Products table
		`CREATE TABLE IF NOT EXISTS test_products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			sku VARCHAR(50) UNIQUE NOT NULL,
			name VARCHAR(200) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			category VARCHAR(50),
			stock_quantity INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			attributes JSON
		)`,
		
		// Orders table
		`CREATE TABLE IF NOT EXISTS test_orders (
			id CHAR(36) PRIMARY KEY,
			user_id CHAR(36) NOT NULL,
			order_number VARCHAR(50) UNIQUE NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			total_amount DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			shipping_address JSON,
			payment_info JSON,
			FOREIGN KEY (user_id) REFERENCES test_users(id) ON DELETE CASCADE
		)`,
		
		// Create indexes
		`CREATE INDEX idx_users_email ON test_users(email)`,
		`CREATE INDEX idx_users_created_at ON test_users(created_at)`,
		`CREATE INDEX idx_products_category ON test_products(category)`,
		`CREATE INDEX idx_orders_user_id ON test_orders(user_id)`,
		`CREATE INDEX idx_orders_status ON test_orders(status)`,
	}
	
	for _, stmt := range schema {
		if _, err := g.mysqlDB.ExecContext(ctx, stmt); err != nil {
			// Ignore "already exists" errors
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("failed to execute MySQL schema statement: %w", err)
			}
		}
	}
	
	return nil
}

// PopulateTestData generates test data for all tables
func (g *TestDataGenerator) PopulateTestData(ctx context.Context, scale int) error {
	// Generate users
	userIDs, err := g.generateUsers(ctx, scale)
	if err != nil {
		return fmt.Errorf("failed to generate users: %w", err)
	}
	
	// Generate products
	productIDs, err := g.generateProducts(ctx, scale)
	if err != nil {
		return fmt.Errorf("failed to generate products: %w", err)
	}
	
	// Generate orders
	orderIDs, err := g.generateOrders(ctx, userIDs, scale)
	if err != nil {
		return fmt.Errorf("failed to generate orders: %w", err)
	}
	
	// Generate order items
	if err := g.generateOrderItems(ctx, orderIDs, productIDs); err != nil {
		return fmt.Errorf("failed to generate order items: %w", err)
	}
	
	// Generate activity logs
	if err := g.generateActivityLogs(ctx, userIDs, scale); err != nil {
		return fmt.Errorf("failed to generate activity logs: %w", err)
	}
	
	// Generate metrics
	if err := g.generateMetrics(ctx, scale); err != nil {
		return fmt.Errorf("failed to generate metrics: %w", err)
	}
	
	// Refresh materialized view
	if _, err := g.pgDB.ExecContext(ctx, "REFRESH MATERIALIZED VIEW test_order_summary"); err != nil {
		return fmt.Errorf("failed to refresh materialized view: %w", err)
	}
	
	return nil
}

// CleanupTestData removes all test data
func (g *TestDataGenerator) CleanupTestData(ctx context.Context) error {
	// PostgreSQL cleanup
	pgCleanup := []string{
		"DROP MATERIALIZED VIEW IF EXISTS test_order_summary",
		"DROP TABLE IF EXISTS test_metrics CASCADE",
		"DROP TABLE IF EXISTS test_activity_logs CASCADE",
		"DROP TABLE IF EXISTS test_order_items CASCADE",
		"DROP TABLE IF EXISTS test_orders CASCADE",
		"DROP TABLE IF EXISTS test_products CASCADE",
		"DROP TABLE IF EXISTS test_users CASCADE",
		"DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE",
	}
	
	for _, stmt := range pgCleanup {
		if _, err := g.pgDB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to cleanup PostgreSQL: %w", err)
		}
	}
	
	// MySQL cleanup
	if g.mysqlDB != nil {
		mysqlCleanup := []string{
			"DROP TABLE IF EXISTS test_orders",
			"DROP TABLE IF EXISTS test_products",
			"DROP TABLE IF EXISTS test_users",
		}
		
		for _, stmt := range mysqlCleanup {
			if _, err := g.mysqlDB.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("failed to cleanup MySQL: %w", err)
			}
		}
	}
	
	return nil
}

// GenerateWorkload simulates database workload
func (g *TestDataGenerator) GenerateWorkload(ctx context.Context, duration time.Duration, qps int) error {
	endTime := time.Now().Add(duration)
	ticker := time.NewTicker(time.Second / time.Duration(qps))
	defer ticker.Stop()
	
	queryTypes := []func(context.Context) error{
		g.workloadSelectUsers,
		g.workloadSelectProducts,
		g.workloadSelectOrders,
		g.workloadUpdateProduct,
		g.workloadInsertMetric,
		g.workloadComplexQuery,
	}
	
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Random query type
			queryFunc := queryTypes[g.random.Intn(len(queryTypes))]
			go queryFunc(ctx) // Async execution
		}
	}
	
	return nil
}

// Private helper methods

func (g *TestDataGenerator) generateUsers(ctx context.Context, count int) ([]string, error) {
	userIDs := make([]string, 0, count)
	
	for i := 0; i < count; i++ {
		userID := generateUUID()
		username := fmt.Sprintf("user_%d_%d", i, g.random.Intn(10000))
		email := fmt.Sprintf("%s@example.com", username)
		fullName := fmt.Sprintf("Test User %d", i)
		phone := fmt.Sprintf("555-%04d", g.random.Intn(10000))
		
		metadata := fmt.Sprintf(`{"source": "test", "batch": %d, "preferences": {"theme": "%s"}}`,
			i/100, []string{"light", "dark"}[g.random.Intn(2)])
		
		_, err := g.pgDB.ExecContext(ctx, `
			INSERT INTO test_users (id, username, email, full_name, phone, metadata)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, userID, username, email, fullName, phone, metadata)
		
		if err != nil {
			return nil, err
		}
		
		userIDs = append(userIDs, userID)
	}
	
	return userIDs, nil
}

func (g *TestDataGenerator) generateProducts(ctx context.Context, count int) ([]int, error) {
	productIDs := make([]int, 0, count)
	categories := []string{"Electronics", "Clothing", "Books", "Home", "Sports", "Toys"}
	
	for i := 0; i < count; i++ {
		sku := fmt.Sprintf("SKU-%06d", i)
		name := fmt.Sprintf("Product %d - %s", i, categories[g.random.Intn(len(categories))])
		description := fmt.Sprintf("Description for product %d", i)
		price := float64(g.random.Intn(10000)) / 100.0
		category := categories[g.random.Intn(len(categories))]
		stock := g.random.Intn(1000)
		
		attributes := fmt.Sprintf(`{"color": "%s", "size": "%s", "weight": %d}`,
			[]string{"Red", "Blue", "Green", "Black", "White"}[g.random.Intn(5)],
			[]string{"S", "M", "L", "XL"}[g.random.Intn(4)],
			g.random.Intn(1000))
		
		var productID int
		err := g.pgDB.QueryRowContext(ctx, `
			INSERT INTO test_products (sku, name, description, price, category, stock_quantity, attributes)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`, sku, name, description, price, category, stock, attributes).Scan(&productID)
		
		if err != nil {
			return nil, err
		}
		
		productIDs = append(productIDs, productID)
	}
	
	return productIDs, nil
}

func (g *TestDataGenerator) generateOrders(ctx context.Context, userIDs []string, count int) ([]string, error) {
	orderIDs := make([]string, 0, count)
	statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
	
	for i := 0; i < count; i++ {
		orderID := generateUUID()
		userID := userIDs[g.random.Intn(len(userIDs))]
		orderNumber := fmt.Sprintf("ORD-%08d", i)
		status := statuses[g.random.Intn(len(statuses))]
		totalAmount := float64(g.random.Intn(100000)) / 100.0
		
		shippingAddress := fmt.Sprintf(`{
			"street": "%d Main St",
			"city": "Test City",
			"state": "TS",
			"zip": "%05d",
			"country": "US"
		}`, g.random.Intn(1000), g.random.Intn(100000))
		
		paymentInfo := `{"method": "credit_card", "last4": "1234", "status": "authorized"}`
		
		_, err := g.pgDB.ExecContext(ctx, `
			INSERT INTO test_orders (id, user_id, order_number, status, total_amount, shipping_address, payment_info)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, orderID, userID, orderNumber, status, totalAmount, shippingAddress, paymentInfo)
		
		if err != nil {
			return nil, err
		}
		
		orderIDs = append(orderIDs, orderID)
	}
	
	return orderIDs, nil
}

func (g *TestDataGenerator) generateOrderItems(ctx context.Context, orderIDs []string, productIDs []int) error {
	for _, orderID := range orderIDs {
		// Random number of items per order (1-5)
		itemCount := g.random.Intn(5) + 1
		
		for i := 0; i < itemCount; i++ {
			productID := productIDs[g.random.Intn(len(productIDs))]
			quantity := g.random.Intn(10) + 1
			unitPrice := float64(g.random.Intn(10000)) / 100.0
			discount := float64(g.random.Intn(1000)) / 100.0
			
			_, err := g.pgDB.ExecContext(ctx, `
				INSERT INTO test_order_items (order_id, product_id, quantity, unit_price, discount_amount)
				VALUES ($1, $2, $3, $4, $5)
			`, orderID, productID, quantity, unitPrice, discount)
			
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

func (g *TestDataGenerator) generateActivityLogs(ctx context.Context, userIDs []string, count int) error {
	actions := []string{"login", "logout", "view_product", "add_to_cart", "checkout", "update_profile"}
	entityTypes := []string{"user", "product", "order", "cart"}
	
	for i := 0; i < count*10; i++ { // More activity logs
		var userID *string
		if g.random.Float32() < 0.9 { // 90% have user_id
			id := userIDs[g.random.Intn(len(userIDs))]
			userID = &id
		}
		
		action := actions[g.random.Intn(len(actions))]
		entityType := entityTypes[g.random.Intn(len(entityTypes))]
		entityID := fmt.Sprintf("%d", g.random.Intn(1000))
		ipAddress := fmt.Sprintf("192.168.%d.%d", g.random.Intn(256), g.random.Intn(256))
		userAgent := "Mozilla/5.0 Test Browser"
		duration := g.random.Intn(5000)
		
		requestData := fmt.Sprintf(`{"page": "%s", "referrer": "%s"}`,
			[]string{"/home", "/products", "/cart", "/checkout"}[g.random.Intn(4)],
			[]string{"google.com", "facebook.com", "direct"}[g.random.Intn(3)])
		
		responseData := `{"status": "success", "code": 200}`
		
		_, err := g.pgDB.ExecContext(ctx, `
			INSERT INTO test_activity_logs 
			(user_id, action, entity_type, entity_id, ip_address, user_agent, request_data, response_data, duration_ms)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, userID, action, entityType, entityID, ipAddress, userAgent, requestData, responseData, duration)
		
		if err != nil {
			return err
		}
	}
	
	return nil
}

func (g *TestDataGenerator) generateMetrics(ctx context.Context, count int) error {
	metricNames := []string{
		"api.request.count",
		"api.request.duration",
		"db.query.count",
		"db.query.duration",
		"cache.hit.ratio",
		"error.rate",
		"user.active.count",
	}
	
	// Generate metrics for the last hour
	now := time.Now()
	for i := 0; i < count*20; i++ {
		metricName := metricNames[g.random.Intn(len(metricNames))]
		metricValue := g.random.Float64() * 1000
		timestamp := now.Add(-time.Duration(g.random.Intn(3600)) * time.Second)
		
		tags := fmt.Sprintf(`{
			"environment": "%s",
			"region": "%s",
			"service": "test-service"
		}`,
			[]string{"dev", "staging", "prod"}[g.random.Intn(3)],
			[]string{"us-east-1", "us-west-2", "eu-central-1"}[g.random.Intn(3)])
		
		_, err := g.pgDB.ExecContext(ctx, `
			INSERT INTO test_metrics (metric_name, metric_value, tags, timestamp)
			VALUES ($1, $2, $3, $4)
		`, metricName, metricValue, tags, timestamp)
		
		if err != nil {
			return err
		}
	}
	
	return nil
}

// Workload generation methods

func (g *TestDataGenerator) workloadSelectUsers(ctx context.Context) error {
	query := `
		SELECT id, username, email, full_name, created_at 
		FROM test_users 
		WHERE is_active = true 
		ORDER BY created_at DESC 
		LIMIT 10
	`
	rows, err := g.pgDB.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	// Consume results
	for rows.Next() {
		var id, username, email, fullName string
		var createdAt time.Time
		rows.Scan(&id, &username, &email, &fullName, &createdAt)
	}
	
	return nil
}

func (g *TestDataGenerator) workloadSelectProducts(ctx context.Context) error {
	category := []string{"Electronics", "Clothing", "Books", "Home", "Sports", "Toys"}[g.random.Intn(6)]
	
	query := `
		SELECT id, sku, name, price, stock_quantity 
		FROM test_products 
		WHERE category = $1 AND stock_quantity > 0
		ORDER BY price DESC
		LIMIT 20
	`
	rows, err := g.pgDB.QueryContext(ctx, query, category)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var id int
		var sku, name string
		var price float64
		var stock int
		rows.Scan(&id, &sku, &name, &price, &stock)
	}
	
	return nil
}

func (g *TestDataGenerator) workloadSelectOrders(ctx context.Context) error {
	status := []string{"pending", "processing", "shipped", "delivered", "cancelled"}[g.random.Intn(5)]
	
	query := `
		SELECT o.id, o.order_number, o.total_amount, u.username, u.email
		FROM test_orders o
		JOIN test_users u ON o.user_id = u.id
		WHERE o.status = $1
		ORDER BY o.created_at DESC
		LIMIT 50
	`
	rows, err := g.pgDB.QueryContext(ctx, query, status)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var orderID, orderNumber, username, email string
		var totalAmount float64
		rows.Scan(&orderID, &orderNumber, &totalAmount, &username, &email)
	}
	
	return nil
}

func (g *TestDataGenerator) workloadUpdateProduct(ctx context.Context) error {
	// Random product stock update
	productID := g.random.Intn(100) + 1
	stockChange := g.random.Intn(20) - 10 // -10 to +10
	
	_, err := g.pgDB.ExecContext(ctx, `
		UPDATE test_products 
		SET stock_quantity = stock_quantity + $1
		WHERE id = $2 AND stock_quantity + $1 >= 0
	`, stockChange, productID)
	
	return err
}

func (g *TestDataGenerator) workloadInsertMetric(ctx context.Context) error {
	metricName := []string{
		"api.request.count",
		"api.request.duration",
		"db.query.count",
		"db.query.duration",
	}[g.random.Intn(4)]
	
	metricValue := g.random.Float64() * 100
	tags := fmt.Sprintf(`{"endpoint": "/api/v1/%s", "method": "%s"}`,
		[]string{"users", "products", "orders"}[g.random.Intn(3)],
		[]string{"GET", "POST", "PUT", "DELETE"}[g.random.Intn(4)])
	
	_, err := g.pgDB.ExecContext(ctx, `
		INSERT INTO test_metrics (metric_name, metric_value, tags)
		VALUES ($1, $2, $3)
	`, metricName, metricValue, tags)
	
	return err
}

func (g *TestDataGenerator) workloadComplexQuery(ctx context.Context) error {
	// Complex analytical query
	query := `
		WITH order_stats AS (
			SELECT 
				u.id as user_id,
				u.username,
				COUNT(DISTINCT o.id) as order_count,
				SUM(o.total_amount) as total_spent,
				AVG(o.total_amount) as avg_order_value
			FROM test_users u
			LEFT JOIN test_orders o ON u.id = o.user_id
			WHERE u.created_at > NOW() - INTERVAL '30 days'
			GROUP BY u.id, u.username
		)
		SELECT 
			username,
			order_count,
			total_spent,
			avg_order_value,
			CASE 
				WHEN total_spent > 1000 THEN 'VIP'
				WHEN total_spent > 500 THEN 'Regular'
				ELSE 'New'
			END as customer_tier
		FROM order_stats
		ORDER BY total_spent DESC
		LIMIT 100
	`
	
	rows, err := g.pgDB.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var username, tier string
		var orderCount int
		var totalSpent, avgValue float64
		rows.Scan(&username, &orderCount, &totalSpent, &avgValue, &tier)
	}
	
	return nil
}

// Helper function to generate UUID
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}