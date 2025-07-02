package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestEnvironment manages the test infrastructure
type TestEnvironment struct {
	PostgresContainer testcontainers.Container
	PostgresDB        *sql.DB
	PostgresDSN       string
	CollectorProcess  *CollectorProcess
	MetricsStore      *MetricsStore
	LogsStore         *LogsStore
	NRDBExporter      *MockNRDBExporter
	t                 *testing.T
	cleanupFuncs      []func()
}

// CollectorProcess represents a running collector instance
type CollectorProcess struct {
	configPath string
	process    *os.Process
	healthURL  string
	metricsURL string
}

// MetricsStore stores collected metrics for verification
type MetricsStore struct {
	mu      sync.RWMutex
	metrics []pmetric.Metrics
}

// LogsStore stores collected logs for verification
type LogsStore struct {
	mu   sync.RWMutex
	logs []plog.Logs
}

// MockNRDBExporter simulates New Relic exporter
type MockNRDBExporter struct {
	mu       sync.RWMutex
	payloads []NRDBPayload
	server   *http.Server
}

// NRDBPayload represents a New Relic metrics payload
type NRDBPayload struct {
	CommonAttributes map[string]interface{} `json:"common"`
	Metrics          []NRDBMetric          `json:"metrics"`
}

// NRDBMetric represents a single metric in NRDB format
type NRDBMetric struct {
	Name             string                 `json:"name"`
	Type             string                 `json:"type"`
	Value            interface{}            `json:"value"`
	Timestamp        int64                  `json:"timestamp"`
	Attributes       map[string]interface{} `json:"attributes"`
	CommonAttributes map[string]interface{} `json:"common,omitempty"`
}

// setupTestEnvironment creates a complete test environment
func setupTestEnvironment(t *testing.T) *TestEnvironment {
	env := &TestEnvironment{
		t:            t,
		MetricsStore: &MetricsStore{},
		LogsStore:    &LogsStore{},
		cleanupFuncs: []func(){},
	}

	// Start PostgreSQL container
	env.startPostgreSQL(t)

	// Setup mock NRDB exporter
	env.setupNRDBExporter(t)

	// Create test log directory
	os.MkdirAll("/tmp/test-logs", 0755)
	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		os.RemoveAll("/tmp/test-logs")
	})

	return env
}

// startPostgreSQL starts a PostgreSQL container for testing
func (env *TestEnvironment) startPostgreSQL(t *testing.T) {
	ctx := context.Background()

	// PostgreSQL container configuration
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test_user",
			"POSTGRES_PASSWORD": "test_password",
			"POSTGRES_DB":       "test_db",
		},
		WaitingFor: wait.ForSQL("5432/tcp", "postgres", func(port nat.Port) string {
			return fmt.Sprintf("postgres://test_user:test_password@localhost:%s/test_db?sslmode=disable", port.Port())
		}).WithStartupTimeout(60 * time.Second),
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount("/tmp/test-logs", "/var/log/postgresql"),
		),
		Cmd: []string{
			"postgres",
			"-c", "shared_preload_libraries=auto_explain,pg_stat_statements",
			"-c", "auto_explain.log_min_duration=10",
			"-c", "auto_explain.log_analyze=true",
			"-c", "auto_explain.log_buffers=true",
			"-c", "auto_explain.log_format=json",
			"-c", "log_destination=csvlog",
			"-c", "logging_collector=on",
			"-c", "log_directory=/var/log/postgresql",
			"-c", "log_filename=postgresql.log",
			"-c", "log_rotation_age=0",
			"-c", "log_rotation_size=0",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	env.PostgresContainer = container
	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		container.Terminate(ctx)
	})

	// Get connection details
	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	env.PostgresDSN = fmt.Sprintf("postgres://test_user:test_password@%s:%s/test_db?sslmode=disable", host, port.Port())

	// Connect to database
	db, err := sql.Open("postgres", env.PostgresDSN)
	require.NoError(t, err)

	env.PostgresDB = db
	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		db.Close()
	})

	// Wait for database to be ready
	require.Eventually(t, func() bool {
		err := db.Ping()
		return err == nil
	}, 30*time.Second, 1*time.Second)

	// Enable required extensions
	env.setupPostgreSQLExtensions(t, db)
}

// setupPostgreSQLExtensions enables required PostgreSQL extensions
func (env *TestEnvironment) setupPostgreSQLExtensions(t *testing.T, db *sql.DB) {
	extensions := []string{
		"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
		"CREATE EXTENSION IF NOT EXISTS pgcrypto",
	}

	for _, ext := range extensions {
		_, err := db.Exec(ext)
		if err != nil {
			t.Logf("Warning: Failed to create extension: %v", err)
		}
	}
}

// setupNRDBExporter creates a mock NRDB exporter endpoint
func (env *TestEnvironment) setupNRDBExporter(t *testing.T) {
	exporter := &MockNRDBExporter{
		payloads: []NRDBPayload{},
	}

	// Create mock OTLP endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Parse OTLP data and convert to NRDB format
		payload := env.convertOTLPToNRDB(body)
		
		exporter.mu.Lock()
		exporter.payloads = append(exporter.payloads, payload)
		exporter.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	server := &http.Server{
		Addr:    ":4317",
		Handler: mux,
	}

	exporter.server = server
	env.NRDBExporter = exporter

	go server.ListenAndServe()

	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		server.Shutdown(context.Background())
	})
}

// StartCollector starts the OpenTelemetry collector with given config
func (env *TestEnvironment) StartCollector(t *testing.T, configPath string) *CollectorProcess {
	// Write config to temp file
	configContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		// Use embedded test config
		configContent = []byte(env.getTestConfig(configPath))
	}

	tmpConfig, err := ioutil.TempFile("", "otel-config-*.yaml")
	require.NoError(t, err)
	defer tmpConfig.Close()

	_, err = tmpConfig.Write(configContent)
	require.NoError(t, err)

	// Start collector process
	cmd := exec.Command("otelcol", "--config", tmpConfig.Name())
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("POSTGRES_HOST=%s", env.getPostgresHost()),
		fmt.Sprintf("POSTGRES_PORT=%s", env.getPostgresPort()),
		"POSTGRES_USER=test_user",
		"POSTGRES_PASSWORD=test_password",
		"POSTGRES_DB=test_db",
		fmt.Sprintf("POSTGRES_LOG_PATH=/tmp/test-logs/postgresql.log"),
		"NEW_RELIC_LICENSE_KEY=test-key",
		"NEW_RELIC_OTLP_ENDPOINT=localhost:4317",
	)

	err = cmd.Start()
	require.NoError(t, err)

	collector := &CollectorProcess{
		configPath: tmpConfig.Name(),
		process:    cmd.Process,
		healthURL:  "http://localhost:13133/health",
		metricsURL: "http://localhost:8888/metrics",
	}

	env.CollectorProcess = collector
	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		cmd.Process.Kill()
		os.Remove(tmpConfig.Name())
	})

	return collector
}

// Cleanup cleans up all test resources
func (env *TestEnvironment) Cleanup() {
	for i := len(env.cleanupFuncs) - 1; i >= 0; i-- {
		env.cleanupFuncs[i]()
	}
}

// GetCollectedMetrics returns all collected metrics
func (env *TestEnvironment) GetCollectedMetrics() []pmetric.Metrics {
	env.MetricsStore.mu.RLock()
	defer env.MetricsStore.mu.RUnlock()
	return env.MetricsStore.metrics
}

// GetCollectedLogs returns all collected logs
func (env *TestEnvironment) GetCollectedLogs() []plog.Logs {
	env.LogsStore.mu.RLock()
	defer env.LogsStore.mu.RUnlock()
	return env.LogsStore.logs
}

// GetNRDBPayload returns the latest NRDB payload
func (env *TestEnvironment) GetNRDBPayload() *NRDBPayload {
	env.NRDBExporter.mu.RLock()
	defer env.NRDBExporter.mu.RUnlock()
	
	if len(env.NRDBExporter.payloads) == 0 {
		return nil
	}
	
	return &env.NRDBExporter.payloads[len(env.NRDBExporter.payloads)-1]
}

// SimulateAutoExplainError simulates auto_explain not being loaded
func (env *TestEnvironment) SimulateAutoExplainError() {
	// Temporarily rename log file to simulate error
	os.Rename("/tmp/test-logs/postgresql.log", "/tmp/test-logs/postgresql.log.bak")
}

// RestoreAutoExplain restores auto_explain functionality
func (env *TestEnvironment) RestoreAutoExplain() {
	os.Rename("/tmp/test-logs/postgresql.log.bak", "/tmp/test-logs/postgresql.log")
}

// SimulateDatabaseOutage simulates a database outage
func (env *TestEnvironment) SimulateDatabaseOutage(duration time.Duration) {
	go func() {
		env.PostgresContainer.Stop(context.Background())
		time.Sleep(duration)
		env.PostgresContainer.Start(context.Background())
	}()
}

// SimulateLogFileError simulates log file access error
func (env *TestEnvironment) SimulateLogFileError() {
	os.Chmod("/tmp/test-logs/postgresql.log", 0000)
	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		os.Chmod("/tmp/test-logs/postgresql.log", 0644)
	})
}

// DisableExtension disables a PostgreSQL extension
func (env *TestEnvironment) DisableExtension(name string) {
	_, _ = env.PostgresDB.Exec(fmt.Sprintf("DROP EXTENSION IF EXISTS %s CASCADE", name))
}

// EnableExtension enables a PostgreSQL extension
func (env *TestEnvironment) EnableExtension(name string) {
	_, _ = env.PostgresDB.Exec(fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s", name))
}

// IsHealthy checks if collector is healthy
func (c *CollectorProcess) IsHealthy() bool {
	resp, err := http.Get(c.healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Shutdown gracefully shuts down the collector
func (c *CollectorProcess) Shutdown() {
	if c.process != nil {
		c.process.Signal(os.Interrupt)
		time.Sleep(2 * time.Second)
	}
}

// Helper functions for getting PostgreSQL connection details
func (env *TestEnvironment) getPostgresHost() string {
	ctx := context.Background()
	host, _ := env.PostgresContainer.Host(ctx)
	return host
}

func (env *TestEnvironment) getPostgresPort() string {
	ctx := context.Background()
	port, _ := env.PostgresContainer.MappedPort(ctx, "5432")
	return port.Port()
}

// getTestConfig returns embedded test configuration
func (env *TestEnvironment) getTestConfig(name string) string {
	switch filepath.Base(name) {
	case "config-plan-intelligence.yaml":
		return testPlanIntelligenceConfig
	case "config-ash.yaml":
		return testASHConfig
	case "config-full-integration.yaml":
		return testFullIntegrationConfig
	default:
		return testFullIntegrationConfig
	}
}

// convertOTLPToNRDB converts OTLP metrics to NRDB format (simplified)
func (env *TestEnvironment) convertOTLPToNRDB(otlpData []byte) NRDBPayload {
	// This is a simplified conversion - real implementation would parse OTLP protobuf
	return NRDBPayload{
		CommonAttributes: map[string]interface{}{
			"service.name":          "postgresql-test",
			"deployment.environment": "test",
			"db.system":             "postgresql",
		},
		Metrics: []NRDBMetric{
			{
				Name:      "db.postgresql.query.exec_time",
				Type:      "gauge",
				Value:     123.45,
				Timestamp: time.Now().Unix(),
				Attributes: map[string]interface{}{
					"query_id": "test_query_123",
					"database": "test_db",
				},
			},
		},
	}
}

// Metric helper functions

func findMetricsByName(metrics []pmetric.Metrics, name string) []pmetric.Metric {
	var found []pmetric.Metric
	
	for _, md := range metrics {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			rm := rms.At(i)
			sms := rm.ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				sm := sms.At(j)
				metrics := sm.Metrics()
				for k := 0; k < metrics.Len(); k++ {
					metric := metrics.At(k)
					if metric.Name() == name {
						found = append(found, metric)
					}
				}
			}
		}
	}
	
	return found
}

func getMetricAttributes(metric pmetric.Metric) map[string]string {
	attrs := make(map[string]string)
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			dp := metric.Gauge().DataPoints().At(0)
			dp.Attributes().Range(func(k string, v pcommon.Value) bool {
				attrs[k] = v.AsString()
				return true
			})
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			dp := metric.Sum().DataPoints().At(0)
			dp.Attributes().Range(func(k string, v pcommon.Value) bool {
				attrs[k] = v.AsString()
				return true
			})
		}
	}
	
	return attrs
}

func getMetricValue(metric pmetric.Metric) float64 {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			dp := metric.Gauge().DataPoints().At(0)
			return dp.DoubleValue()
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			dp := metric.Sum().DataPoints().At(0)
			return dp.DoubleValue()
		}
	}
	
	return 0
}

func filterNRDBMetrics(metrics []NRDBMetric, name string) []NRDBMetric {
	var filtered []NRDBMetric
	for _, m := range metrics {
		if m.Name == name {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func filterNRDBMetricsByPrefix(metrics []NRDBMetric, prefix string) []NRDBMetric {
	var filtered []NRDBMetric
	for _, m := range metrics {
		if strings.HasPrefix(m.Name, prefix) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}