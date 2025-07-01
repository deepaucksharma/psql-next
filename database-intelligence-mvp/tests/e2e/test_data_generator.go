package e2e

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDataGenerator generates test data for E2E tests
type TestDataGenerator struct {
	db     *sql.DB
	random *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator(db *sql.DB) *TestDataGenerator {
	return &TestDataGenerator{
		db:     db,
		random: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateUsers creates test users with PII data
func (g *TestDataGenerator) GenerateUsers(t *testing.T, count int) {
	t.Logf("Generating %d test users", count)
	
	tx, err := g.db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO users (email, phone, ssn, credit_card, created_at) 
		VALUES ($1, $2, $3, $4, $5)
	`)
	require.NoError(t, err)
	defer stmt.Close()

	for i := 0; i < count; i++ {
		email := g.generateEmail()
		phone := g.generatePhone()
		ssn := g.generateSSN()
		creditCard := g.generateCreditCard()
		createdAt := g.generateTimestamp()

		_, err = stmt.Exec(email, phone, ssn, creditCard, createdAt)
		require.NoError(t, err)

		if i%1000 == 0 {
			t.Logf("Generated %d users", i)
		}
	}

	err = tx.Commit()
	require.NoError(t, err)
}

// GenerateOrders creates test orders
func (g *TestDataGenerator) GenerateOrders(t *testing.T, count int, maxUserID int) {
	t.Logf("Generating %d test orders", count)
	
	tx, err := g.db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO orders (user_id, total, status, created_at) 
		VALUES ($1, $2, $3, $4)
	`)
	require.NoError(t, err)
	defer stmt.Close()

	statuses := []string{"pending", "processing", "completed", "cancelled"}

	for i := 0; i < count; i++ {
		userID := g.random.Intn(maxUserID) + 1
		total := g.random.Float64() * 1000
		status := statuses[g.random.Intn(len(statuses))]
		createdAt := g.generateTimestamp()

		_, err = stmt.Exec(userID, total, status, createdAt)
		if err != nil {
			// Skip foreign key violations
			continue
		}
	}

	err = tx.Commit()
	require.NoError(t, err)
}

// GenerateQueryPatterns creates various query patterns for testing
func (g *TestDataGenerator) GenerateQueryPatterns() []string {
	patterns := []string{
		// Simple selects
		"SELECT * FROM users WHERE id = %d",
		"SELECT * FROM users WHERE email = '%s'",
		"SELECT COUNT(*) FROM users",
		
		// Joins
		"SELECT u.*, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id WHERE u.id = %d GROUP BY u.id",
		"SELECT u.email, SUM(o.total) FROM users u JOIN orders o ON u.id = o.user_id GROUP BY u.email",
		
		// Subqueries
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE total > %f)",
		"SELECT * FROM users u WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id AND o.status = '%s')",
		
		// Complex queries
		"WITH user_stats AS (SELECT user_id, COUNT(*) as order_count, SUM(total) as total_spent FROM orders GROUP BY user_id) SELECT u.*, us.* FROM users u JOIN user_stats us ON u.id = us.user_id WHERE us.order_count > %d",
		
		// Updates
		"UPDATE users SET last_login = NOW() WHERE id = %d",
		"UPDATE orders SET status = '%s' WHERE id = %d",
		
		// Deletes
		"DELETE FROM orders WHERE created_at < NOW() - INTERVAL '%d days'",
	}

	// Generate actual queries with random parameters
	queries := make([]string, 0, len(patterns)*3)
	for _, pattern := range patterns {
		for i := 0; i < 3; i++ {
			query := g.fillQueryPattern(pattern)
			queries = append(queries, query)
		}
	}

	return queries
}

// fillQueryPattern fills a query pattern with random values
func (g *TestDataGenerator) fillQueryPattern(pattern string) string {
	// Count format specifiers
	specifierCount := strings.Count(pattern, "%")
	
	args := make([]interface{}, specifierCount)
	for i := 0; i < specifierCount; i++ {
		// Determine type based on format specifier
		idx := strings.Index(pattern[strings.Index(pattern, "%"):], "%")
		if idx+1 < len(pattern) {
			switch pattern[idx+1] {
			case 'd':
				args[i] = g.random.Intn(1000) + 1
			case 'f':
				args[i] = g.random.Float64() * 1000
			case 's':
				args[i] = g.generateRandomValue()
			}
		}
	}

	return fmt.Sprintf(pattern, args...)
}

// generateEmail generates a realistic email address
func (g *TestDataGenerator) generateEmail() string {
	domains := []string{"example.com", "test.com", "email.com", "mail.com"}
	firstName := g.randomFirstName()
	lastName := g.randomLastName()
	domain := domains[g.random.Intn(len(domains))]
	
	formats := []string{
		"%s.%s@%s",
		"%s%s@%s",
		"%s_%s@%s",
		"%s%d@%s",
	}
	
	format := formats[g.random.Intn(len(formats))]
	if strings.Contains(format, "%d") {
		return fmt.Sprintf(format, strings.ToLower(firstName), g.random.Intn(1000), domain)
	}
	return fmt.Sprintf(format, strings.ToLower(firstName), strings.ToLower(lastName), domain)
}

// generatePhone generates a realistic phone number
func (g *TestDataGenerator) generatePhone() string {
	formats := []string{
		"(%03d) %03d-%04d",
		"%03d-%03d-%04d",
		"+1%03d%03d%04d",
		"%03d.%03d.%04d",
	}
	
	format := formats[g.random.Intn(len(formats))]
	return fmt.Sprintf(format, 
		g.random.Intn(900)+100,
		g.random.Intn(900)+100,
		g.random.Intn(9000)+1000)
}

// generateSSN generates a realistic SSN (for testing only!)
func (g *TestDataGenerator) generateSSN() string {
	return fmt.Sprintf("%03d-%02d-%04d",
		g.random.Intn(900)+100,
		g.random.Intn(99)+1,
		g.random.Intn(9000)+1000)
}

// generateCreditCard generates a realistic credit card number (for testing only!)
func (g *TestDataGenerator) generateCreditCard() string {
	// Generate a valid-looking credit card number (Luhn algorithm not implemented)
	prefixes := []string{"4", "5", "37", "6011"} // Visa, Mastercard, Amex, Discover
	prefix := prefixes[g.random.Intn(len(prefixes))]
	
	// Fill remaining digits
	remaining := 16 - len(prefix)
	if prefix == "37" {
		remaining = 15 - len(prefix) // Amex has 15 digits
	}
	
	number := prefix
	for i := 0; i < remaining; i++ {
		number += fmt.Sprintf("%d", g.random.Intn(10))
	}
	
	// Format with spaces
	formatted := ""
	for i, digit := range number {
		if i > 0 && i%4 == 0 {
			formatted += " "
		}
		formatted += string(digit)
	}
	
	return formatted
}

// generateTimestamp generates a random timestamp within the last year
func (g *TestDataGenerator) generateTimestamp() time.Time {
	now := time.Now()
	daysAgo := g.random.Intn(365)
	return now.AddDate(0, 0, -daysAgo)
}

// generateRandomValue generates random string values for queries
func (g *TestDataGenerator) generateRandomValue() string {
	values := []string{
		"pending", "completed", "cancelled", "processing",
		"active", "inactive", "suspended",
		"high", "medium", "low",
	}
	return values[g.random.Intn(len(values))]
}

// randomFirstName returns a random first name
func (g *TestDataGenerator) randomFirstName() string {
	names := []string{
		"John", "Jane", "Robert", "Mary", "Michael", "Patricia",
		"David", "Jennifer", "James", "Linda", "William", "Elizabeth",
		"Richard", "Barbara", "Joseph", "Susan", "Thomas", "Jessica",
		"Christopher", "Sarah", "Charles", "Karen", "Daniel", "Lisa",
	}
	return names[g.random.Intn(len(names))]
}

// randomLastName returns a random last name
func (g *TestDataGenerator) randomLastName() string {
	names := []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia",
		"Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez",
		"Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore",
		"Jackson", "Martin", "Lee", "Perez", "Thompson", "White",
	}
	return names[g.random.Intn(len(names))]
}

// CreateComplexScenario creates a complex database scenario
func (g *TestDataGenerator) CreateComplexScenario(t *testing.T) {
	// Create users with various patterns
	g.GenerateUsers(t, 1000)
	
	// Get max user ID
	var maxUserID int
	err := g.db.QueryRow("SELECT MAX(id) FROM users").Scan(&maxUserID)
	require.NoError(t, err)
	
	// Create orders
	g.GenerateOrders(t, 5000, maxUserID)
	
	// Create indexes for testing plan changes
	g.db.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)")
	g.db.Exec("CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)")
	g.db.Exec("CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)")
	
	// Analyze tables for statistics
	g.db.Exec("ANALYZE users")
	g.db.Exec("ANALYZE orders")
	
	t.Log("Complex scenario created successfully")
}

// SimulateProductionWorkload simulates a production-like workload
func (g *TestDataGenerator) SimulateProductionWorkload(t *testing.T, duration time.Duration) {
	t.Logf("Simulating production workload for %v", duration)
	
	endTime := time.Now().Add(duration)
	queries := g.GenerateQueryPatterns()
	
	// Mix of read and write operations
	for time.Now().Before(endTime) {
		// Pick a random query
		query := queries[g.random.Intn(len(queries))]
		
		// Execute with some probability of slow queries
		if g.random.Float64() < 0.1 {
			// 10% chance of slow query
			query = "SELECT pg_sleep(0.5); " + query
		}
		
		_, err := g.db.Exec(query)
		if err != nil {
			// Log but don't fail - some queries might have syntax issues
			t.Logf("Query error (expected for some): %v", err)
		}
		
		// Variable think time
		thinkTime := time.Duration(g.random.Intn(100)) * time.Millisecond
		time.Sleep(thinkTime)
	}
}