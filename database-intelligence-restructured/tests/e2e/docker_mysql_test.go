package e2e

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// TestDockerMySQLCollection runs collector with Docker-based MySQL
func TestDockerMySQLCollection(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	
	if licenseKey == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}
	
	// Default OTLP endpoint if not set
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
		t.Logf("Using default NEW_RELIC_OTLP_ENDPOINT: %s", otlpEndpoint)
	}

	runID := fmt.Sprintf("docker_mysql_%d", time.Now().Unix())
	t.Logf("Starting Docker MySQL test with run ID: %s", runID)

	// Start MySQL using Docker
	t.Log("Starting MySQL container...")
	mysqlCmd := exec.Command("docker", "run",
		"--name", "e2e-mysql-test",
		"-e", "MYSQL_ROOT_PASSWORD=mysql",
		"-e", "MYSQL_DATABASE=testdb",
		"-p", "3306:3306",
		"--network", "bridge",
		"-d", "mysql:8.0")

	output, err := mysqlCmd.CombinedOutput()
	if err != nil {
		// Check if container already exists
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start MySQL: %v\n%s", err, output)
		}
		// Container exists, remove and recreate
		t.Log("Removing existing container...")
		exec.Command("docker", "rm", "-f", "e2e-mysql-test").Run()
		
		// Create new command for retry
		mysqlCmd = exec.Command("docker", "run",
			"--name", "e2e-mysql-test",
			"-e", "MYSQL_ROOT_PASSWORD=mysql",
			"-e", "MYSQL_DATABASE=testdb",
			"-p", "3306:3306",
			"--network", "bridge",
			"-d", "mysql:8.0")
		
		output, err = mysqlCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to start MySQL after cleanup: %v\n%s", err, output)
		}
	}

	// Cleanup function
	defer func() {
		t.Log("Cleaning up containers...")
		exec.Command("docker", "stop", "e2e-mysql-test").Run()
		exec.Command("docker", "rm", "e2e-mysql-test").Run()
		exec.Command("docker", "stop", "e2e-mysql-otel-collector").Run()
		exec.Command("docker", "rm", "e2e-mysql-otel-collector").Run()
	}()

	// Wait for MySQL to be ready
	t.Log("Waiting for MySQL to be ready...")
	time.Sleep(30 * time.Second) // MySQL takes longer to start than PostgreSQL

	// Connect to MySQL
	db, err := sql.Open("mysql", "root:mysql@tcp(localhost:3306)/testdb")
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	// Wait for connection to be ready
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Create test data
	t.Log("Creating test database and data...")
	if err := createMySQLTestData(db); err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Create collector config
	collectorConfig := fmt.Sprintf(`
receivers:
  mysql:
    endpoint: host.docker.internal:3306
    username: root
    password: mysql
    database: testdb
    collection_interval: 10s
    tls:
      insecure: true

processors:
  batch:
    timeout: 5s
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: environment
        value: e2e-docker-test
        action: insert
      - key: test.type
        value: mysql
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 5s
  
  logging:
    loglevel: info

service:
  pipelines:
    metrics:
      receivers: [mysql]
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
`, runID, otlpEndpoint, licenseKey)

	// Write config file
	configPath := "docker-mysql-collector-config.yaml"
	err = os.WriteFile(configPath, []byte(collectorConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path for mounting
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Remove any existing collector container
	exec.Command("docker", "rm", "-f", "e2e-mysql-otel-collector").Run()

	// Start OpenTelemetry Collector using official contrib image
	t.Log("Starting OpenTelemetry Collector...")
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-mysql-otel-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"-p", "14317:4317",
		"-p", "14318:4318",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}
	containerID := strings.TrimSpace(string(output))
	t.Logf("Collector container started: %s", containerID[:12])

	// Wait for collector to initialize
	t.Log("Waiting for collector to initialize...")
	time.Sleep(10 * time.Second)

	// Check collector logs
	logsCmd := exec.Command("docker", "logs", "e2e-mysql-otel-collector")
	logs, _ := logsCmd.CombinedOutput()
	t.Logf("Collector logs:\n%s", logs)

	// Wait for collection cycles
	t.Log("Waiting for metric collection (30 seconds)...")
	time.Sleep(30 * time.Second)

	// Generate some database activity
	t.Log("Generating database activity...")
	generateMySQLActivity(t, db)

	// Wait more for data to be exported
	t.Log("Waiting for data export to New Relic (30 seconds)...")
	time.Sleep(30 * time.Second)

	// Check final collector logs
	logsCmd = exec.Command("docker", "logs", "--tail", "50", "e2e-mysql-otel-collector")
	logs, err = logsCmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: Failed to get collector logs: %v", err)
	} else {
		t.Logf("Final collector logs:\n%s", logs)
	}

	t.Log("Test completed successfully!")
	t.Logf("Check New Relic UI for MySQL metrics with test.run.id = %s", runID)

	// Next step
	t.Logf("Next: Query NRDB for MySQL metrics with test.run.id = %s", runID)
}

func createMySQLTestData(db *sql.DB) error {
	// Create test tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS customers (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id INT AUTO_INCREMENT PRIMARY KEY,
			customer_id INT,
			total DECIMAL(10,2),
			status VARCHAR(20),
			order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_customer (customer_id),
			INDEX idx_status (status),
			INDEX idx_date (order_date)
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(10,2),
			stock INT,
			category VARCHAR(50),
			INDEX idx_category (category)
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			order_id INT,
			product_id INT,
			quantity INT,
			price DECIMAL(10,2),
			INDEX idx_order (order_id),
			INDEX idx_product (product_id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Insert test data
	// Customers
	for i := 0; i < 100; i++ {
		_, err := db.Exec(`
			INSERT INTO customers (name, email) 
			VALUES (?, ?)`,
			fmt.Sprintf("Customer_%d", i),
			fmt.Sprintf("customer%d@example.com", i))
		if err != nil {
			return fmt.Errorf("failed to insert customer: %w", err)
		}
	}

	// Products
	categories := []string{"Electronics", "Clothing", "Books", "Food", "Toys"}
	for i := 0; i < 50; i++ {
		_, err := db.Exec(`
			INSERT INTO products (name, price, stock, category) 
			VALUES (?, ?, ?, ?)`,
			fmt.Sprintf("Product_%d", i),
			float64(i)*9.99+10,
			100-i*2,
			categories[i%len(categories)])
		if err != nil {
			return fmt.Errorf("failed to insert product: %w", err)
		}
	}

	// Orders
	statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
	for i := 0; i < 200; i++ {
		result, err := db.Exec(`
			INSERT INTO orders (customer_id, total, status) 
			VALUES (?, ?, ?)`,
			(i%100)+1,
			float64(i)*15.5+25,
			statuses[i%len(statuses)])
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}

		orderID, _ := result.LastInsertId()

		// Add order items
		for j := 0; j < 3; j++ {
			_, err = db.Exec(`
				INSERT INTO order_items (order_id, product_id, quantity, price) 
				VALUES (?, ?, ?, ?)`,
				orderID,
				(j+i)%50+1,
				j+1,
				float64(j)*5.99+10)
			if err != nil {
				return fmt.Errorf("failed to insert order item: %w", err)
			}
		}
	}

	return nil
}

func generateMySQLActivity(t *testing.T, db *sql.DB) {
	queries := []string{
		"SELECT COUNT(*) FROM orders",
		"SELECT AVG(total) FROM orders WHERE status = 'delivered'",
		"SELECT c.name, COUNT(o.id) as order_count FROM customers c JOIN orders o ON c.id = o.customer_id GROUP BY c.id LIMIT 10",
		"SELECT p.category, SUM(oi.quantity) as total_sold FROM products p JOIN order_items oi ON p.id = oi.product_id GROUP BY p.category",
		"SELECT * FROM orders WHERE order_date > DATE_SUB(NOW(), INTERVAL 1 DAY)",
		"SELECT p.name, p.stock FROM products WHERE stock < 20 ORDER BY stock ASC",
		"UPDATE orders SET status = 'processing' WHERE status = 'pending' AND id % 7 = 0",
		"SELECT COUNT(DISTINCT customer_id) FROM orders WHERE status IN ('shipped', 'delivered')",
	}

	for i := 0; i < 20; i++ {
		query := queries[i%len(queries)]
		
		if strings.HasPrefix(query, "UPDATE") {
			_, err := db.Exec(query)
			if err != nil {
				t.Logf("Update query failed: %v", err)
			}
		} else {
			rows, err := db.Query(query)
			if err != nil {
				t.Logf("Query failed: %v", err)
				continue
			}
			rows.Close()
		}
		
		time.Sleep(1 * time.Second)
	}
}