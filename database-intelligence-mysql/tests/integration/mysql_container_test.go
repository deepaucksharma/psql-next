package integration

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// MySQLContainerTestSuite tests MySQL monitoring with containerized MySQL
type MySQLContainerTestSuite struct {
	suite.Suite
	logger         *zap.Logger
	mysqlContainer *mysql.MySQLContainer
	db             *sql.DB
	otelContainer  testcontainers.Container
}

func TestMySQLContainer(t *testing.T) {
	suite.Run(t, new(MySQLContainerTestSuite))
}

func (s *MySQLContainerTestSuite) SetupSuite() {
	s.logger, _ = zap.NewDevelopment()
	ctx := context.Background()
	
	// Start MySQL container
	s.startMySQLContainer(ctx)
	
	// Start OpenTelemetry Collector container
	s.startOTelCollectorContainer(ctx)
	
	// Wait for services to be ready
	time.Sleep(10 * time.Second)
}

func (s *MySQLContainerTestSuite) TearDownSuite() {
	ctx := context.Background()
	
	if s.db != nil {
		s.db.Close()
	}
	
	if s.mysqlContainer != nil {
		s.mysqlContainer.Terminate(ctx)
	}
	
	if s.otelContainer != nil {
		s.otelContainer.Terminate(ctx)
	}
}

func (s *MySQLContainerTestSuite) TestMySQLMetricsCollection() {
	ctx := context.Background()
	
	// Create test database and tables
	s.Run("Setup test data", func() {
		_, err := s.db.Exec(`
			CREATE TABLE IF NOT EXISTS test_table (
				id INT PRIMARY KEY AUTO_INCREMENT,
				name VARCHAR(100),
				value DECIMAL(10,2),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
		s.NoError(err)
		
		// Insert test data
		for i := 0; i < 100; i++ {
			_, err := s.db.Exec(
				"INSERT INTO test_table (name, value) VALUES (?, ?)",
				fmt.Sprintf("test_%d", i),
				float64(i)*10.5,
			)
			s.NoError(err)
		}
	})
	
	// Generate workload
	s.Run("Generate database workload", func() {
		workloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		
		go s.generateWorkload(workloadCtx)
		
		// Let workload run
		time.Sleep(10 * time.Second)
	})
	
	// Verify metrics are collected
	s.Run("Verify metrics collection", func() {
		// Get metrics from OTel collector
		metricsEndpoint, err := s.otelContainer.Endpoint(ctx, "metrics")
		s.NoError(err)
		
		// Check that metrics endpoint is accessible
		// In a real test, you would parse and validate the metrics
		s.logger.Info("Metrics endpoint", zap.String("endpoint", metricsEndpoint))
		
		// Verify database metrics
		var connections int
		err = s.db.QueryRow("SELECT COUNT(*) FROM information_schema.processlist").Scan(&connections)
		s.NoError(err)
		s.Greater(connections, 0, "Should have active connections")
		
		// Verify Performance Schema is enabled
		var perfSchemaEnabled string
		err = s.db.QueryRow("SELECT @@performance_schema").Scan(&perfSchemaEnabled)
		s.NoError(err)
		s.Equal("1", perfSchemaEnabled, "Performance Schema should be enabled")
	})
}

func (s *MySQLContainerTestSuite) TestSlowQueryDetection() {
	s.Run("Slow queries should be detected", func() {
		// Execute a slow query
		_, err := s.db.Exec("SELECT SLEEP(2)")
		s.NoError(err)
		
		// Check slow query log
		var slowQueries int
		err = s.db.QueryRow(`
			SELECT COUNT(*) 
			FROM mysql.slow_log 
			WHERE sql_text LIKE '%SLEEP%'
		`).Scan(&slowQueries)
		
		// Note: This might fail if slow_log table is not available
		// In that case, check performance_schema.events_statements_summary_by_digest
		if err != nil {
			err = s.db.QueryRow(`
				SELECT COUNT(*) 
				FROM performance_schema.events_statements_summary_by_digest 
				WHERE DIGEST_TEXT LIKE '%SLEEP%' AND AVG_TIMER_WAIT > 1000000000
			`).Scan(&slowQueries)
			s.NoError(err)
		}
		
		s.Greater(slowQueries, 0, "Slow query should be recorded")
	})
}

func (s *MySQLContainerTestSuite) startMySQLContainer(ctx context.Context) {
	// Read init scripts
	initScriptPath := filepath.Join("..", "..", "mysql", "init")
	
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysql.WithDatabase("testdb"),
		mysql.WithUsername("testuser"),
		mysql.WithPassword("testpass"),
		testcontainers.WithEnv(map[string]string{
			"MYSQL_ROOT_PASSWORD": "rootpass",
		}),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Mounts: testcontainers.Mounts(
					testcontainers.BindMount(initScriptPath, "/docker-entrypoint-initdb.d"),
				),
				WaitingFor: wait.ForLog("ready for connections").
					WithOccurrence(2).
					WithStartupTimeout(60 * time.Second),
			},
		}),
	)
	
	s.Require().NoError(err, "Failed to start MySQL container")
	s.mysqlContainer = mysqlContainer
	
	// Get connection string
	connectionString, err := mysqlContainer.ConnectionString(ctx)
	s.Require().NoError(err)
	
	// Connect to database
	s.db, err = sql.Open("mysql", connectionString)
	s.Require().NoError(err)
	
	// Verify connection
	err = s.db.Ping()
	s.Require().NoError(err)
	
	s.logger.Info("MySQL container started", zap.String("connection", connectionString))
}

func (s *MySQLContainerTestSuite) startOTelCollectorContainer(ctx context.Context) {
	// Get MySQL container host and port
	mysqlHost, err := s.mysqlContainer.Host(ctx)
	s.Require().NoError(err)
	
	mysqlPort, err := s.mysqlContainer.MappedPort(ctx, "3306")
	s.Require().NoError(err)
	
	configPath := filepath.Join("..", "..", "config", "otel-collector-config.yaml")
	
	req := testcontainers.ContainerRequest{
		Image: "otel/opentelemetry-collector-contrib:latest",
		ExposedPorts: []string{
			"4317/tcp",  // OTLP gRPC
			"4318/tcp",  // OTLP HTTP
			"9090/tcp",  // Prometheus metrics
			"8888/tcp",  // Collector metrics
		},
		Env: map[string]string{
			"MYSQL_HOST":     mysqlHost,
			"MYSQL_PORT":     mysqlPort.Port(),
			"MYSQL_USER":     "testuser",
			"MYSQL_PASSWORD": "testpass",
			"MYSQL_DATABASE": "testdb",
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(configPath, "/etc/otelcol/config.yaml"),
		),
		Cmd: []string{"--config=/etc/otelcol/config.yaml"},
		WaitingFor: wait.ForLog("Everything is ready").
			WithStartupTimeout(30 * time.Second),
	}
	
	otelContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	
	s.Require().NoError(err, "Failed to start OTel collector container")
	s.otelContainer = otelContainer
	
	endpoint, err := otelContainer.Endpoint(ctx, "8888")
	s.Require().NoError(err)
	
	s.logger.Info("OTel Collector container started", zap.String("endpoint", endpoint))
}

func (s *MySQLContainerTestSuite) generateWorkload(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	queries := []string{
		"SELECT COUNT(*) FROM test_table",
		"SELECT AVG(value) FROM test_table",
		"SELECT * FROM test_table ORDER BY RAND() LIMIT 10",
		"UPDATE test_table SET value = value + 1 WHERE id = ?",
		"INSERT INTO test_table (name, value) VALUES (?, ?)",
	}
	
	queryCount := 0
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Workload generation stopped", zap.Int("queries", queryCount))
			return
		case <-ticker.C:
			// Execute random query
			query := queries[queryCount%len(queries)]
			
			if query == queries[3] || query == queries[4] {
				// Queries with parameters
				_, err := s.db.Exec(query, queryCount%100, fmt.Sprintf("load_%d", queryCount), float64(queryCount))
				if err != nil {
					s.logger.Warn("Query failed", zap.Error(err))
				}
			} else {
				// Queries without parameters
				_, err := s.db.Query(query)
				if err != nil {
					s.logger.Warn("Query failed", zap.Error(err))
				}
			}
			
			queryCount++
		}
	}
}