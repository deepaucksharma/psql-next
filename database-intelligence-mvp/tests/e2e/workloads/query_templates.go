// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package workloads

import (
	"fmt"
	"math/rand"
	"time"
)

// initializeQueryTemplates initializes query templates for different database types and workload types
func (wg *WorkloadGenerator) initializeQueryTemplates() {
	switch wg.dbType {
	case "postgresql":
		wg.initializePostgreSQLTemplates()
	case "mysql":
		wg.initializeMySQLTemplates()
	}
}

// initializePostgreSQLTemplates initializes PostgreSQL-specific query templates
func (wg *WorkloadGenerator) initializePostgreSQLTemplates() {
	// SELECT queries
	selectTemplates := []QueryTemplate{
		{
			Name:      "user_by_id",
			QueryType: "select",
			Template:  "SELECT * FROM users WHERE id = $1",
			Parameters: []ParameterDef{
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "user_by_email",
			QueryType: "select",
			Template:  "SELECT id, username, email, created_at FROM users WHERE email = $1",
			Parameters: []ParameterDef{
				{Name: "email", Type: "string", Generator: "email"},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "orders_by_user",
			QueryType: "select",
			Template:  "SELECT o.*, u.username FROM orders o JOIN users u ON o.user_id = u.id WHERE o.user_id = $1 ORDER BY o.created_at DESC LIMIT 10",
			Parameters: []ParameterDef{
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
			},
			Complexity:   "medium",
			ExpectedRows: 10,
			PIIRisk:      "medium",
		},
		{
			Name:      "product_search",
			QueryType: "select",
			Template:  "SELECT * FROM products WHERE name ILIKE $1 OR description ILIKE $1 ORDER BY price",
			Parameters: []ParameterDef{
				{Name: "search_term", Type: "string", Generator: "product_search"},
			},
			Complexity:   "medium",
			ExpectedRows: 5,
			PIIRisk:      "low",
		},
		{
			Name:      "order_analytics",
			QueryType: "select",
			Template: `
				SELECT 
					DATE_TRUNC('day', created_at) as order_date,
					COUNT(*) as order_count,
					SUM(total_amount) as total_revenue,
					AVG(total_amount) as avg_order_value,
					COUNT(DISTINCT user_id) as unique_customers
				FROM orders 
				WHERE created_at >= $1 AND created_at < $2
				GROUP BY DATE_TRUNC('day', created_at)
				ORDER BY order_date
			`,
			Parameters: []ParameterDef{
				{Name: "start_date", Type: "timestamp", Generator: "date_range"},
				{Name: "end_date", Type: "timestamp", Generator: "date_range"},
			},
			Complexity:   "complex",
			ExpectedRows: 30,
			PIIRisk:      "low",
		},
		{
			Name:      "slow_analytical_query",
			QueryType: "select",
			Template: `
				WITH monthly_stats AS (
					SELECT 
						DATE_TRUNC('month', o.created_at) as month,
						u.id as user_id,
						COUNT(o.id) as order_count,
						SUM(o.total_amount) as total_spent,
						AVG(oi.quantity * p.price) as avg_item_value
					FROM orders o
					JOIN users u ON o.user_id = u.id
					JOIN order_items oi ON o.id = oi.order_id
					JOIN products p ON oi.product_id = p.id
					WHERE o.created_at >= NOW() - INTERVAL '6 months'
					GROUP BY DATE_TRUNC('month', o.created_at), u.id
				),
				user_segments AS (
					SELECT 
						user_id,
						SUM(total_spent) as total_lifetime_value,
						AVG(order_count) as avg_monthly_orders,
						CASE 
							WHEN SUM(total_spent) > 1000 THEN 'high_value'
							WHEN SUM(total_spent) > 500 THEN 'medium_value'
							ELSE 'low_value'
						END as segment
					FROM monthly_stats
					GROUP BY user_id
				)
				SELECT 
					segment,
					COUNT(*) as user_count,
					AVG(total_lifetime_value) as avg_ltv,
					AVG(avg_monthly_orders) as avg_monthly_orders
				FROM user_segments
				GROUP BY segment
				ORDER BY avg_ltv DESC
			`,
			Parameters: []ParameterDef{},
			Complexity:   "complex",
			ExpectedRows: 3,
			PIIRisk:      "low",
		},
	}
	
	// INSERT queries
	insertTemplates := []QueryTemplate{
		{
			Name:      "insert_user",
			QueryType: "insert",
			Template:  "INSERT INTO users (username, email, first_name, last_name, phone, created_at) VALUES ($1, $2, $3, $4, $5, NOW()) RETURNING id",
			Parameters: []ParameterDef{
				{Name: "username", Type: "string", Generator: "username"},
				{Name: "email", Type: "string", Generator: "email"},
				{Name: "first_name", Type: "string", Generator: "first_name"},
				{Name: "last_name", Type: "string", Generator: "last_name"},
				{Name: "phone", Type: "string", Generator: "phone"},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "insert_order",
			QueryType: "insert",
			Template:  "INSERT INTO orders (user_id, total_amount, status, created_at) VALUES ($1, $2, $3, NOW()) RETURNING id",
			Parameters: []ParameterDef{
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
				{Name: "total_amount", Type: "decimal", Generator: "amount", Range: &RangeSpec{Min: 10.0, Max: 1000.0}},
				{Name: "status", Type: "string", Generator: "order_status", Values: []string{"pending", "processing", "shipped", "delivered", "cancelled"}},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "low",
		},
		{
			Name:      "insert_product",
			QueryType: "insert",
			Template:  "INSERT INTO products (name, description, price, category, sku, created_at) VALUES ($1, $2, $3, $4, $5, NOW()) RETURNING id",
			Parameters: []ParameterDef{
				{Name: "name", Type: "string", Generator: "product"},
				{Name: "description", Type: "string", Generator: "product_description"},
				{Name: "price", Type: "decimal", Generator: "price", Range: &RangeSpec{Min: 5.0, Max: 500.0}},
				{Name: "category", Type: "string", Generator: "category", Values: []string{"electronics", "clothing", "books", "home", "sports"}},
				{Name: "sku", Type: "string", Generator: "sku"},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "low",
		},
		{
			Name:      "insert_with_pii",
			QueryType: "insert",
			Template:  "INSERT INTO sensitive_data (user_id, ssn, credit_card, notes) VALUES ($1, $2, $3, $4) RETURNING id",
			Parameters: []ParameterDef{
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
				{Name: "ssn", Type: "string", Generator: "ssn"},
				{Name: "credit_card", Type: "string", Generator: "credit_card"},
				{Name: "notes", Type: "string", Generator: "pii_notes"},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "critical",
		},
	}
	
	// UPDATE queries
	updateTemplates := []QueryTemplate{
		{
			Name:      "update_user_email",
			QueryType: "update",
			Template:  "UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2",
			Parameters: []ParameterDef{
				{Name: "email", Type: "string", Generator: "email"},
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "update_order_status",
			QueryType: "update",
			Template:  "UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2",
			Parameters: []ParameterDef{
				{Name: "status", Type: "string", Generator: "order_status", Values: []string{"processing", "shipped", "delivered", "cancelled"}},
				{Name: "order_id", Type: "int", Generator: "order_id", Range: &RangeSpec{Min: 1, Max: 5000}},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "low",
		},
		{
			Name:      "update_product_price",
			QueryType: "update",
			Template:  "UPDATE products SET price = $1, updated_at = NOW() WHERE category = $2",
			Parameters: []ParameterDef{
				{Name: "price_multiplier", Type: "decimal", Generator: "multiplier", Range: &RangeSpec{Min: 0.9, Max: 1.1}},
				{Name: "category", Type: "string", Generator: "category", Values: []string{"electronics", "clothing", "books", "home", "sports"}},
			},
			Complexity:   "medium",
			ExpectedRows: 10,
			PIIRisk:      "low",
		},
	}
	
	// DELETE queries
	deleteTemplates := []QueryTemplate{
		{
			Name:      "delete_old_logs",
			QueryType: "delete",
			Template:  "DELETE FROM access_logs WHERE created_at < $1",
			Parameters: []ParameterDef{
				{Name: "cutoff_date", Type: "timestamp", Generator: "old_date"},
			},
			Complexity:   "simple",
			ExpectedRows: 100,
			PIIRisk:      "medium",
		},
		{
			Name:      "delete_cancelled_orders",
			QueryType: "delete",
			Template:  "DELETE FROM orders WHERE status = 'cancelled' AND created_at < $1",
			Parameters: []ParameterDef{
				{Name: "cutoff_date", Type: "timestamp", Generator: "old_date"},
			},
			Complexity:   "simple",
			ExpectedRows: 10,
			PIIRisk:      "low",
		},
	}
	
	// Analytical queries
	analyticalTemplates := []QueryTemplate{
		{
			Name:      "revenue_by_category",
			QueryType: "analytical",
			Template: `
				SELECT 
					p.category,
					COUNT(DISTINCT o.id) as order_count,
					SUM(oi.quantity * p.price) as total_revenue,
					AVG(oi.quantity * p.price) as avg_revenue_per_order
				FROM orders o
				JOIN order_items oi ON o.id = oi.order_id
				JOIN products p ON oi.product_id = p.id
				WHERE o.created_at >= $1
				GROUP BY p.category
				ORDER BY total_revenue DESC
			`,
			Parameters: []ParameterDef{
				{Name: "start_date", Type: "timestamp", Generator: "recent_date"},
			},
			Complexity:   "complex",
			ExpectedRows: 5,
			PIIRisk:      "low",
		},
		{
			Name:      "customer_cohort_analysis",
			QueryType: "analytical",
			Template: `
				WITH first_purchase AS (
					SELECT user_id, MIN(created_at) as first_purchase_date
					FROM orders
					GROUP BY user_id
				),
				cohort_data AS (
					SELECT 
						fp.user_id,
						DATE_TRUNC('month', fp.first_purchase_date) as cohort_month,
						DATE_TRUNC('month', o.created_at) as purchase_month,
						o.total_amount
					FROM first_purchase fp
					JOIN orders o ON fp.user_id = o.user_id
					WHERE fp.first_purchase_date >= $1
				)
				SELECT 
					cohort_month,
					purchase_month,
					COUNT(DISTINCT user_id) as customers,
					SUM(total_amount) as revenue
				FROM cohort_data
				GROUP BY cohort_month, purchase_month
				ORDER BY cohort_month, purchase_month
			`,
			Parameters: []ParameterDef{
				{Name: "start_date", Type: "timestamp", Generator: "recent_date"},
			},
			Complexity:   "complex",
			ExpectedRows: 50,
			PIIRisk:      "low",
		},
	}
	
	// Store templates by type
	wg.queryTemplates["select"] = selectTemplates
	wg.queryTemplates["insert"] = insertTemplates
	wg.queryTemplates["update"] = updateTemplates
	wg.queryTemplates["delete"] = deleteTemplates
	wg.queryTemplates["analytical"] = analyticalTemplates
}

// initializeMySQLTemplates initializes MySQL-specific query templates
func (wg *WorkloadGenerator) initializeMySQLTemplates() {
	// SELECT queries for MySQL
	selectTemplates := []QueryTemplate{
		{
			Name:      "user_by_id",
			QueryType: "select",
			Template:  "SELECT * FROM users WHERE id = ?",
			Parameters: []ParameterDef{
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "orders_by_date_range",
			QueryType: "select",
			Template:  "SELECT o.*, u.username FROM orders o INNER JOIN users u ON o.user_id = u.id WHERE o.created_at BETWEEN ? AND ? ORDER BY o.created_at DESC",
			Parameters: []ParameterDef{
				{Name: "start_date", Type: "timestamp", Generator: "date_range"},
				{Name: "end_date", Type: "timestamp", Generator: "date_range"},
			},
			Complexity:   "medium",
			ExpectedRows: 20,
			PIIRisk:      "medium",
		},
		{
			Name:      "product_inventory",
			QueryType: "select",
			Template:  "SELECT p.*, COALESCE(i.quantity, 0) as stock_level FROM products p LEFT JOIN inventory i ON p.id = i.product_id WHERE p.category = ?",
			Parameters: []ParameterDef{
				{Name: "category", Type: "string", Generator: "category", Values: []string{"electronics", "clothing", "books", "home", "sports"}},
			},
			Complexity:   "medium",
			ExpectedRows: 10,
			PIIRisk:      "low",
		},
		{
			Name:      "slow_join_query",
			QueryType: "select",
			Template: `
				SELECT 
					u.username,
					u.email,
					COUNT(o.id) as order_count,
					SUM(o.total_amount) as total_spent,
					MAX(o.created_at) as last_order_date,
					GROUP_CONCAT(DISTINCT p.category) as purchased_categories
				FROM users u
				LEFT JOIN orders o ON u.id = o.user_id
				LEFT JOIN order_items oi ON o.id = oi.order_id
				LEFT JOIN products p ON oi.product_id = p.id
				WHERE u.created_at >= ?
				GROUP BY u.id, u.username, u.email
				HAVING order_count > ?
				ORDER BY total_spent DESC
				LIMIT 100
			`,
			Parameters: []ParameterDef{
				{Name: "start_date", Type: "timestamp", Generator: "recent_date"},
				{Name: "min_orders", Type: "int", Generator: "threshold", Range: &RangeSpec{Min: 1, Max: 5}},
			},
			Complexity:   "complex",
			ExpectedRows: 50,
			PIIRisk:      "high",
		},
	}
	
	// INSERT queries for MySQL
	insertTemplates := []QueryTemplate{
		{
			Name:      "insert_user",
			QueryType: "insert",
			Template:  "INSERT INTO users (username, email, first_name, last_name, phone, created_at) VALUES (?, ?, ?, ?, ?, NOW())",
			Parameters: []ParameterDef{
				{Name: "username", Type: "string", Generator: "username"},
				{Name: "email", Type: "string", Generator: "email"},
				{Name: "first_name", Type: "string", Generator: "first_name"},
				{Name: "last_name", Type: "string", Generator: "last_name"},
				{Name: "phone", Type: "string", Generator: "phone"},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "bulk_insert_orders",
			QueryType: "insert",
			Template:  "INSERT INTO orders (user_id, total_amount, status, created_at) VALUES (?, ?, ?, NOW()), (?, ?, ?, NOW()), (?, ?, ?, NOW())",
			Parameters: []ParameterDef{
				{Name: "user_id_1", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
				{Name: "amount_1", Type: "decimal", Generator: "amount", Range: &RangeSpec{Min: 10.0, Max: 500.0}},
				{Name: "status_1", Type: "string", Generator: "order_status", Values: []string{"pending", "processing"}},
				{Name: "user_id_2", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
				{Name: "amount_2", Type: "decimal", Generator: "amount", Range: &RangeSpec{Min: 10.0, Max: 500.0}},
				{Name: "status_2", Type: "string", Generator: "order_status", Values: []string{"pending", "processing"}},
				{Name: "user_id_3", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
				{Name: "amount_3", Type: "decimal", Generator: "amount", Range: &RangeSpec{Min: 10.0, Max: 500.0}},
				{Name: "status_3", Type: "string", Generator: "order_status", Values: []string{"pending", "processing"}},
			},
			Complexity:   "medium",
			ExpectedRows: 3,
			PIIRisk:      "low",
		},
	}
	
	// UPDATE queries for MySQL
	updateTemplates := []QueryTemplate{
		{
			Name:      "update_user_profile",
			QueryType: "update",
			Template:  "UPDATE users SET email = ?, phone = ?, updated_at = NOW() WHERE id = ?",
			Parameters: []ParameterDef{
				{Name: "email", Type: "string", Generator: "email"},
				{Name: "phone", Type: "string", Generator: "phone"},
				{Name: "user_id", Type: "int", Generator: "user_id", Range: &RangeSpec{Min: 1, Max: 1000}},
			},
			Complexity:   "simple",
			ExpectedRows: 1,
			PIIRisk:      "high",
		},
		{
			Name:      "bulk_update_prices",
			QueryType: "update",
			Template:  "UPDATE products SET price = price * ?, updated_at = NOW() WHERE category = ? AND price < ?",
			Parameters: []ParameterDef{
				{Name: "multiplier", Type: "decimal", Generator: "multiplier", Range: &RangeSpec{Min: 0.9, Max: 1.2}},
				{Name: "category", Type: "string", Generator: "category", Values: []string{"electronics", "clothing", "books"}},
				{Name: "max_price", Type: "decimal", Generator: "price", Range: &RangeSpec{Min: 100.0, Max: 500.0}},
			},
			Complexity:   "medium",
			ExpectedRows: 25,
			PIIRisk:      "low",
		},
	}
	
	// DELETE queries for MySQL
	deleteTemplates := []QueryTemplate{
		{
			Name:      "delete_old_sessions",
			QueryType: "delete",
			Template:  "DELETE FROM user_sessions WHERE last_activity < DATE_SUB(NOW(), INTERVAL ? DAY)",
			Parameters: []ParameterDef{
				{Name: "days_old", Type: "int", Generator: "days", Range: &RangeSpec{Min: 30, Max: 90}},
			},
			Complexity:   "simple",
			ExpectedRows: 50,
			PIIRisk:      "medium",
		},
	}
	
	// Analytical queries for MySQL
	analyticalTemplates := []QueryTemplate{
		{
			Name:      "daily_sales_report",
			QueryType: "analytical",
			Template: `
				SELECT 
					DATE(o.created_at) as sale_date,
					COUNT(DISTINCT o.id) as orders,
					COUNT(DISTINCT o.user_id) as customers,
					SUM(o.total_amount) as revenue,
					AVG(o.total_amount) as avg_order_value
				FROM orders o
				WHERE o.created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
				GROUP BY DATE(o.created_at)
				ORDER BY sale_date DESC
			`,
			Parameters: []ParameterDef{
				{Name: "days_back", Type: "int", Generator: "days", Range: &RangeSpec{Min: 7, Max: 30}},
			},
			Complexity:   "complex",
			ExpectedRows: 30,
			PIIRisk:      "low",
		},
		{
			Name:      "performance_schema_query",
			QueryType: "analytical",
			Template: `
				SELECT 
					DIGEST_TEXT as query_pattern,
					COUNT_STAR as execution_count,
					SUM_TIMER_WAIT/1000000000 as total_time_seconds,
					AVG_TIMER_WAIT/1000000000 as avg_time_seconds,
					SUM_ROWS_EXAMINED as total_rows_examined,
					SUM_ROWS_SENT as total_rows_sent
				FROM performance_schema.events_statements_summary_by_digest
				WHERE DIGEST_TEXT IS NOT NULL
				  AND DIGEST_TEXT NOT LIKE '%performance_schema%'
				  AND DIGEST_TEXT NOT LIKE '%information_schema%'
				ORDER BY SUM_TIMER_WAIT DESC
				LIMIT ?
			`,
			Parameters: []ParameterDef{
				{Name: "limit", Type: "int", Generator: "limit", Range: &RangeSpec{Min: 10, Max: 50}},
			},
			Complexity:   "complex",
			ExpectedRows: 20,
			PIIRisk:      "low",
		},
	}
	
	// Store templates by type
	wg.queryTemplates["select"] = selectTemplates
	wg.queryTemplates["insert"] = insertTemplates
	wg.queryTemplates["update"] = updateTemplates
	wg.queryTemplates["delete"] = deleteTemplates
	wg.queryTemplates["analytical"] = analyticalTemplates
}

// initializeDataGenerators initializes data generators for parameter values
func (wg *WorkloadGenerator) initializeDataGenerators() {
	generators := map[string]*DataGenerator{
		"default": wg.createDataGenerator(),
	}
	
	wg.dataGenerators = generators
}

// createDataGenerator creates a new data generator with sample data
func (wg *WorkloadGenerator) createDataGenerator() *DataGenerator {
	seed := time.Now().UnixNano()
	random := rand.New(rand.NewSource(seed))
	
	return &DataGenerator{
		random: random,
		usernames: []string{
			"john_doe", "jane_smith", "mike_wilson", "sarah_johnson", "david_brown",
			"lisa_davis", "tom_miller", "amy_garcia", "chris_martinez", "jessica_lopez",
			"kevin_anderson", "maria_taylor", "robert_thomas", "linda_jackson", "paul_white",
			"michelle_harris", "james_clark", "nancy_lewis", "daniel_robinson", "karen_walker",
		},
		emails: []string{
			"john.doe@example.com", "jane.smith@company.org", "mike.wilson@test.net",
			"sarah.johnson@email.com", "david.brown@sample.org", "lisa.davis@demo.net",
			"tom.miller@example.org", "amy.garcia@test.com", "chris.martinez@company.net",
			"jessica.lopez@email.org", "kevin.anderson@sample.com", "maria.taylor@demo.org",
			"robert.thomas@example.net", "linda.jackson@company.com", "paul.white@test.org",
			"michelle.harris@email.net", "james.clark@sample.net", "nancy.lewis@demo.com",
			"daniel.robinson@example.org", "karen.walker@company.net",
		},
		products: []string{
			"Wireless Headphones", "Smartphone Case", "Laptop Stand", "Gaming Mouse",
			"Bluetooth Speaker", "Tablet Screen Protector", "USB Cable", "Power Bank",
			"Wireless Charger", "Keyboard", "Monitor", "Webcam", "Microphone",
			"External Hard Drive", "Memory Card", "Router", "Smart Watch", "Fitness Tracker",
			"Coffee Maker", "Blender", "Air Fryer", "Vacuum Cleaner", "Desk Lamp",
		},
		companies: []string{
			"TechCorp Inc", "Global Solutions LLC", "Innovation Labs", "Digital Dynamics",
			"Future Systems", "Smart Technologies", "Advanced Analytics", "Cloud Computing Co",
			"Data Insights Ltd", "AI Solutions Group", "Quantum Technologies", "Cyber Security Inc",
			"Mobile Apps Corp", "Web Development LLC", "Software Engineering Ltd",
		},
		addresses: []string{
			"123 Main St, Anytown, ST 12345", "456 Oak Ave, Somewhere, ST 67890",
			"789 Pine Rd, Elsewhere, ST 11111", "321 Elm Dr, Nowhere, ST 22222",
			"654 Maple Ln, Anywhere, ST 33333", "987 Cedar Blvd, Everywhere, ST 44444",
		},
		phoneNumbers: []string{
			"555-123-4567", "555-234-5678", "555-345-6789", "555-456-7890",
			"555-567-8901", "555-678-9012", "555-789-0123", "555-890-1234",
		},
		creditCards: []string{
			"4111-1111-1111-1111", "4222-2222-2222-2222", "4333-3333-3333-3333",
			"4444-4444-4444-4444", "4555-5555-5555-5555", "4666-6666-6666-6666",
		},
		ssns: []string{
			"123-45-6789", "234-56-7890", "345-67-8901", "456-78-9012",
			"567-89-0123", "678-90-1234", "789-01-2345", "890-12-3456",
		},
	}
}

// Additional helper methods for generating specific types of data
func (wg *WorkloadGenerator) generateProductSearch() string {
	searchTerms := []string{
		"wireless", "bluetooth", "smart", "portable", "premium", "professional",
		"gaming", "office", "home", "outdoor", "fitness", "tech", "digital",
	}
	
	generator := wg.dataGenerators["default"]
	return "%" + searchTerms[generator.random.Intn(len(searchTerms))] + "%"
}

func (wg *WorkloadGenerator) generatePIINotes() string {
	piiNotes := []string{
		"Customer John Doe (SSN: 123-45-6789) requested account update",
		"Credit card 4111-1111-1111-1111 was declined for user",
		"Phone verification for 555-123-4567 completed successfully",
		"Email jane.doe@example.com needs to be updated in system",
		"Social security number verification required for account",
		"Payment method with card ending in 1111 was processed",
	}
	
	generator := wg.dataGenerators["default"]
	return piiNotes[generator.random.Intn(len(piiNotes))]
}

func (wg *WorkloadGenerator) generateSKU() string {
	generator := wg.dataGenerators["default"]
	return fmt.Sprintf("SKU-%06d", generator.random.Intn(999999))
}

func (wg *WorkloadGenerator) generateProductDescription() string {
	descriptions := []string{
		"High-quality product with excellent features and durability",
		"Professional-grade item suitable for business and personal use",
		"Innovative design with cutting-edge technology integration",
		"Premium materials and construction for long-lasting performance",
		"User-friendly interface with advanced functionality",
		"Compact and portable solution for modern lifestyle needs",
		"Energy-efficient design with environmental considerations",
		"Versatile product suitable for multiple applications",
	}
	
	generator := wg.dataGenerators["default"]
	return descriptions[generator.random.Intn(len(descriptions))]
}