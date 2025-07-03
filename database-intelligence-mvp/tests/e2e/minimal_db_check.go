package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	// Test PostgreSQL connection
	fmt.Println("Testing PostgreSQL connection...")
	
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	
	fmt.Println("✓ Successfully connected to PostgreSQL")

	// Run some basic queries
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		log.Fatalf("Failed to query version: %v", err)
	}
	fmt.Printf("PostgreSQL version: %s\n", version)

	// Check basic stats
	var dbSize int64
	err = db.QueryRow("SELECT pg_database_size(current_database())").Scan(&dbSize)
	if err != nil {
		log.Fatalf("Failed to get database size: %v", err)
	}
	fmt.Printf("Database size: %d bytes\n", dbSize)

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test_minimal (
			id SERIAL PRIMARY KEY,
			test_name VARCHAR(100),
			test_value FLOAT,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	fmt.Println("✓ Created test table")

	// Insert test data
	testName := fmt.Sprintf("test_%d", time.Now().Unix())
	testValue := 123.45
	
	result, err := db.Exec(`
		INSERT INTO e2e_test_minimal (test_name, test_value) 
		VALUES ($1, $2)
	`, testName, testValue)
	if err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("✓ Inserted %d row(s)\n", rowsAffected)

	// Query the data back
	var retrievedName string
	var retrievedValue float64
	var createdAt time.Time
	
	err = db.QueryRow(`
		SELECT test_name, test_value, created_at 
		FROM e2e_test_minimal 
		ORDER BY id DESC 
		LIMIT 1
	`).Scan(&retrievedName, &retrievedValue, &createdAt)
	if err != nil {
		log.Fatalf("Failed to query data: %v", err)
	}
	
	fmt.Printf("✓ Retrieved data: name=%s, value=%.2f, created=%v\n", 
		retrievedName, retrievedValue, createdAt)

	// Check pg_stat tables
	var tableCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM pg_stat_user_tables 
		WHERE schemaname = 'public'
	`).Scan(&tableCount)
	if err != nil {
		log.Fatalf("Failed to query pg_stat_user_tables: %v", err)
	}
	fmt.Printf("✓ Found %d user tables in pg_stat_user_tables\n", tableCount)

	// Cleanup
	_, err = db.Exec("DROP TABLE IF EXISTS e2e_test_minimal")
	if err != nil {
		log.Fatalf("Failed to drop table: %v", err)
	}
	fmt.Println("✓ Cleaned up test table")

	fmt.Println("\n✅ All tests passed!")
	os.Exit(0)
}