package e2e

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// TestMultipleMySQLInstances tests collector with multiple MySQL databases
func TestMultipleMySQLInstances(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	
	if licenseKey == "" || apiKey == "" || accountID == "" {
		t.Skip("Required credentials not set")
	}
	
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
	}

	runID := fmt.Sprintf("multi_mysql_%d", time.Now().Unix())
	t.Logf("Starting multiple MySQL instances test with run ID: %s", runID)

	// Instance configurations
	instances := []struct {
		name       string
		port       string
		role       string
		version    string
		config     string
	}{
		{
			name:    "mysql-main",
			port:    "3306",
			role:    "primary",
			version: "8.0",
			config:  "innodb_buffer_pool_size=256M",
		},
		{
			name:    "mysql-replica",
			port:    "3307",
			role:    "replica",
			version: "8.0",
			config:  "read_only=1",
		},
		{
			name:    "mysql-legacy",
			port:    "3308",
			role:    "legacy",
			version: "5.7",
			config:  "max_connections=50",
		},
	}

	// Start all MySQL instances
	t.Log("Starting multiple MySQL instances...")
	for _, inst := range instances {
		startMySQLInstance(t, inst.name, inst.port, inst.version, inst.config)
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		for _, inst := range instances {
			exec.Command("docker", "stop", inst.name).Run()
			exec.Command("docker", "rm", inst.name).Run()
		}
		exec.Command("docker", "stop", "multi-mysql-collector").Run()
		exec.Command("docker", "rm", "multi-mysql-collector").Run()
	}()

	// Wait for all instances to be ready
	t.Log("Waiting for MySQL instances to be ready...")
	time.Sleep(40 * time.Second) // MySQL takes longer to start

	// Create different schemas on each instance
	var wg sync.WaitGroup
	for _, inst := range instances {
		wg.Add(1)
		go func(instance struct {
			name       string
			port       string
			role       string
			version    string
			config     string
		}) {
			defer wg.Done()
			setupMySQLInstanceSchema(t, instance.port, instance.role)
		}(inst)
	}
	wg.Wait()

	// Create collector config for multiple instances
	config := createMultiMySQLConfig(instances, runID, otlpEndpoint, licenseKey)
	
	// Write config
	configPath := "multi-mysql-config.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector for multiple MySQL instances...")
	exec.Command("docker", "rm", "-f", "multi-mysql-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "multi-mysql-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-p", "38888:8888",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Generate workload on each instance
	t.Log("Generating workload on each MySQL instance...")
	generateMultiMySQLWorkload(t, instances)

	// Wait for metrics collection
	t.Log("Waiting for metrics collection (60 seconds)...")
	time.Sleep(60 * time.Second)

	// Run verification tests
	t.Run("VerifyAllMySQLInstances", func(t *testing.T) {
		verifyMultiMySQLMetrics(t, runID, instances, accountID, apiKey)
	})

	t.Run("MySQLVersionComparison", func(t *testing.T) {
		compareMySQLVersionMetrics(t, runID, accountID, apiKey)
	})

	t.Run("ReplicationMetrics", func(t *testing.T) {
		verifyReplicationMetrics(t, runID, accountID, apiKey)
	})
}

func startMySQLInstance(t *testing.T, name, port, version, config string) {
	t.Logf("Starting MySQL %s instance %s on port %s...", version, name, port)
	
	// Remove existing container if any
	exec.Command("docker", "rm", "-f", name).Run()
	
	// Build docker command with version-specific image
	image := fmt.Sprintf("mysql:%s", version)
	mysqlCmd := exec.Command("docker", "run",
		"--name", name,
		"-e", "MYSQL_ROOT_PASSWORD=mysql",
		"-e", "MYSQL_DATABASE=testdb",
		"-p", port+":3306",
		"--network", "bridge",
		"-d", image)

	// Add custom MySQL config if provided
	if config != "" {
		mysqlCmd.Args = append(mysqlCmd.Args[:len(mysqlCmd.Args)-1],
			"--"+config,
			image)
	}

	output, err := mysqlCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start MySQL %s: %v\n%s", name, err, output)
	}
}

func setupMySQLInstanceSchema(t *testing.T, port, role string) {
	// Connect to instance
	connStr := fmt.Sprintf("root:mysql@tcp(localhost:%s)/testdb", port)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL on port %s: %v", port, err)
	}
	defer db.Close()

	// Wait for connection
	for i := 0; i < 60; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	t.Logf("Setting up schema for %s MySQL instance (port %s)...", role, port)

	// Create schema based on role
	switch role {
	case "primary":
		createMySQLPrimarySchema(t, db)
	case "replica":
		createMySQLReplicaSchema(t, db)
	case "legacy":
		createMySQLLegacySchema(t, db)
	}
}

func createMySQLPrimarySchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE DATABASE IF NOT EXISTS ecommerce`,
		`USE ecommerce`,
		`CREATE TABLE IF NOT EXISTS products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(200),
			price DECIMAL(10,2),
			stock INT,
			category VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_category (category),
			INDEX idx_price (price)
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id INT AUTO_INCREMENT PRIMARY KEY,
			customer_id INT,
			total DECIMAL(10,2),
			status ENUM('pending', 'processing', 'shipped', 'delivered'),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_customer (customer_id),
			INDEX idx_status (status),
			INDEX idx_created (created_at)
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_log (
			id INT AUTO_INCREMENT PRIMARY KEY,
			product_id INT,
			quantity_change INT,
			reason VARCHAR(100),
			logged_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_product (product_id),
			INDEX idx_logged (logged_at)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to execute query: %v", err)
		}
	}

	// Insert sample data
	db.Exec("USE ecommerce")
	for i := 0; i < 500; i++ {
		db.Exec(`INSERT INTO products (name, price, stock, category) VALUES (?, ?, ?, ?)`,
			fmt.Sprintf("Product_%d", i), float64(i%100)+9.99, 100-i%50, fmt.Sprintf("Category_%d", i%10))
	}
}

func createMySQLReplicaSchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE DATABASE IF NOT EXISTS reporting`,
		`USE reporting`,
		`CREATE TABLE IF NOT EXISTS sales_summary (
			id INT AUTO_INCREMENT PRIMARY KEY,
			report_date DATE,
			total_sales DECIMAL(12,2),
			order_count INT,
			avg_order_value DECIMAL(10,2),
			INDEX idx_date (report_date)
		)`,
		`CREATE TABLE IF NOT EXISTS customer_analytics (
			id INT AUTO_INCREMENT PRIMARY KEY,
			customer_id INT,
			lifetime_value DECIMAL(12,2),
			order_frequency DECIMAL(5,2),
			last_order_date DATE,
			INDEX idx_customer (customer_id),
			INDEX idx_value (lifetime_value)
		)`,
		`CREATE VIEW top_customers AS
			SELECT customer_id, lifetime_value 
			FROM customer_analytics 
			ORDER BY lifetime_value DESC 
			LIMIT 100`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil && !strings.Contains(err.Error(), "read only") {
			t.Logf("Failed to execute query: %v", err)
		}
	}
}

func createMySQLLegacySchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE DATABASE IF NOT EXISTS legacy_app`,
		`USE legacy_app`,
		`CREATE TABLE IF NOT EXISTS users (
			user_id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(50) UNIQUE,
			email VARCHAR(100),
			created DATETIME,
			INDEX idx_username (username),
			INDEX idx_email (email)
		) ENGINE=MyISAM`, // Legacy storage engine
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id VARCHAR(32) PRIMARY KEY,
			user_id INT,
			ip_address VARCHAR(15),
			last_access DATETIME,
			INDEX idx_user (user_id)
		) ENGINE=MyISAM`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			log_id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			action VARCHAR(100),
			timestamp DATETIME,
			details TEXT
		) ENGINE=MyISAM`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to execute query: %v", err)
		}
	}
}

func createMultiMySQLConfig(instances []struct {
	name       string
	port       string
	role       string
	version    string
	config     string
}, runID, otlpEndpoint, licenseKey string) string {
	// Build receivers section
	receiversConfig := "receivers:\n"
	for _, inst := range instances {
		receiversConfig += fmt.Sprintf(`  mysql/%s:
    endpoint: host.docker.internal:%s
    username: root
    password: mysql
    database: testdb
    collection_interval: 10s
    tls:
      insecure: true
    resource_attributes:
      mysql.instance.name: %s
      mysql.instance.role: %s
      mysql.instance.version: %s

`, inst.name, inst.port, inst.name, inst.role, inst.version)
	}

	// Build service pipeline
	receiverList := []string{}
	for _, inst := range instances {
		receiverList = append(receiverList, fmt.Sprintf("mysql/%s", inst.name))
	}

	return fmt.Sprintf(`%s
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1024
    spike_limit_mib: 256
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: multi_mysql_instance
        action: insert
      - key: environment
        value: e2e-test
        action: insert
  
  resource:
    attributes:
      - key: service.name
        value: multi-mysql-collector
        action: insert
      - key: deployment.type
        value: multi-instance
        action: insert
  
  # Group metrics by instance for better organization
  groupbyattrs:
    keys:
      - mysql.instance.name
      - mysql.instance.role
  
  batch:
    timeout: 10s
    send_batch_size: 500
    send_batch_max_size: 1000

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
  
  logging:
    verbosity: normal
    sampling_initial: 10
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      receivers: [%s]
      processors: [memory_limiter, attributes, resource, groupbyattrs, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`, receiversConfig, runID, otlpEndpoint, licenseKey, strings.Join(receiverList, ", "))
}

func generateMultiMySQLWorkload(t *testing.T, instances []struct {
	name       string
	port       string
	role       string
	version    string
	config     string
}) {
	var wg sync.WaitGroup
	
	for _, inst := range instances {
		wg.Add(1)
		go func(instance struct {
			name       string
			port       string
			role       string
			version    string
			config     string
		}) {
			defer wg.Done()
			
			connStr := fmt.Sprintf("root:mysql@tcp(localhost:%s)/testdb", instance.port)
			db, err := sql.Open("mysql", connStr)
			if err != nil {
				t.Logf("Failed to connect for workload generation: %v", err)
				return
			}
			defer db.Close()
			
			t.Logf("Generating workload on MySQL %s (%s)...", instance.name, instance.role)
			
			// Generate role-specific workload
			for i := 0; i < 30; i++ {
				switch instance.role {
				case "primary":
					// Write-heavy workload
					db.Exec("USE ecommerce")
					db.Exec(`INSERT INTO orders (customer_id, total, status) VALUES (?, ?, ?)`,
						i%100, float64(i)*25.5, "pending")
					db.Exec(`UPDATE products SET stock = stock - 1 WHERE id = ?`, i%500)
					
				case "replica":
					// Read-heavy analytical queries
					db.Exec("USE reporting")
					db.Query(`SELECT * FROM sales_summary WHERE report_date > DATE_SUB(NOW(), INTERVAL 30 DAY)`)
					db.Query(`SELECT AVG(lifetime_value) FROM customer_analytics`)
					
				case "legacy":
					// Legacy application patterns
					db.Exec("USE legacy_app")
					db.Query(`SELECT * FROM users WHERE username LIKE ?`, fmt.Sprintf("user_%d%%", i%10))
					db.Exec(`INSERT INTO audit_log (user_id, action, timestamp) VALUES (?, ?, NOW())`,
						i%100, fmt.Sprintf("action_%d", i))
				}
				time.Sleep(2 * time.Second)
			}
		}(inst)
	}
	
	wg.Wait()
}

func verifyMultiMySQLMetrics(t *testing.T, runID string, instances []struct {
	name       string
	port       string
	role       string
	version    string
	config     string
}, accountID, apiKey string) {
	// Wait for metrics
	time.Sleep(10 * time.Second)

	// Query metrics for all instances
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET mysql.instance.name", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	// Verify we have metrics from all instances
	instanceCounts := make(map[string]int)
	for _, res := range result.Data.Actor.Account.NRQL.Results {
		if instanceName, ok := res["mysql.instance.name"].(string); ok {
			if count, ok := res["count"].(float64); ok {
				instanceCounts[instanceName] = int(count)
			}
		}
	}

	t.Log("MySQL metrics by instance:")
	allInstancesFound := true
	for _, inst := range instances {
		if count, found := instanceCounts[inst.name]; found {
			t.Logf("  ✓ %s (MySQL %s): %d metrics collected", inst.name, inst.version, count)
		} else {
			t.Errorf("  ✗ %s: No metrics found", inst.name)
			allInstancesFound = false
		}
	}

	if allInstancesFound && len(instanceCounts) == len(instances) {
		t.Log("✓ All MySQL instances are being monitored")
	}

	// Check buffer pool metrics for each instance
	nrql = fmt.Sprintf("SELECT average(mysql.buffer_pool.pages.total) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET mysql.instance.name", runID)
	
	result, err = queryNRDB(accountID, apiKey, nrql)
	if err == nil {
		t.Log("Buffer pool sizes by instance:")
		for _, res := range result.Data.Actor.Account.NRQL.Results {
			if instanceName, ok := res["mysql.instance.name"].(string); ok {
				if avgPages, ok := res["average.mysql.buffer_pool.pages.total"].(float64); ok {
					t.Logf("  %s: %.0f pages", instanceName, avgPages)
				}
			}
		}
	}
}

func compareMySQLVersionMetrics(t *testing.T, runID string, accountID, apiKey string) {
	// Compare metric availability between MySQL versions
	nrql := fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET mysql.instance.version", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query version metrics: %v", err)
		return
	}

	t.Log("Metric types by MySQL version:")
	versionMetrics := make(map[string][]string)
	for _, res := range result.Data.Actor.Account.NRQL.Results {
		if version, ok := res["mysql.instance.version"].(string); ok {
			if metricNames, ok := res["uniques.metricName"].([]interface{}); ok {
				metrics := []string{}
				for _, m := range metricNames {
					if mStr, ok := m.(string); ok {
						metrics = append(metrics, mStr)
					}
				}
				versionMetrics[version] = metrics
				t.Logf("  MySQL %s: %d unique metric types", version, len(metrics))
			}
		}
	}

	// Check for version-specific metrics
	if mysql8Metrics, ok := versionMetrics["8.0"]; ok {
		hasNewMetrics := false
		for _, metric := range mysql8Metrics {
			// MySQL 8.0 introduced new performance schema metrics
			if strings.Contains(metric, "performance_schema") {
				hasNewMetrics = true
				break
			}
		}
		if hasNewMetrics {
			t.Log("✓ MySQL 8.0 specific metrics detected")
		}
	}
}

func verifyReplicationMetrics(t *testing.T, runID string, accountID, apiKey string) {
	// Check for replication-related attributes
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND mysql.instance.role = 'replica' SINCE 5 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query replica metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok && count > 0 {
			t.Logf("✓ Replica instance metrics collected: %.0f data points", count)
		}
	}

	// Check for primary vs replica metric differences
	nrql = fmt.Sprintf("SELECT average(mysql.queries) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET mysql.instance.role", runID)
	
	result, err = queryNRDB(accountID, apiKey, nrql)
	if err == nil {
		t.Log("Query rates by instance role:")
		for _, res := range result.Data.Actor.Account.NRQL.Results {
			if role, ok := res["mysql.instance.role"].(string); ok {
				if avgQueries, ok := res["average.mysql.queries"].(float64); ok {
					t.Logf("  %s: %.2f queries/sec", role, avgQueries)
				}
			}
		}
	}
}