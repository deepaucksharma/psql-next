package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/database-intelligence-mvp/internal/database"
	_ "github.com/lib/pq"
	"github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"go.uber.org/zap"
)

// OHIMetric represents a metric from OHI
type OHIMetric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Database  string
	Host      string
}

// OTELMetric represents a metric from OTEL
type OTELMetric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Database  string
	Host      string
}

// ValidationResult represents the comparison result
type ValidationResult struct {
	MetricName    string
	OHIValue      float64
	OTELValue     float64
	Difference    float64
	PercentDiff   float64
	Status        string
	FailureReason string
}

const (
	StatusMatch = "MATCH"
)

// Validator performs side-by-side validation
type Validator struct {
	db       *sql.DB
	nrClient *newrelic.NewRelic
	config   Config
	results  []ValidationResult
	logger   *log.Logger
}

// Config holds validation configuration
type Config struct {
	PostgresURL      string
	NewRelicAPIKey   string
	NewRelicAccount  int
	ValidationPeriod time.Duration
	Tolerance        float64 // Percentage tolerance for differences
}

// MetricMapping defines OHI to OTEL metric mappings
var MetricMappings = map[string]string{
	// PostgreSQL mappings
	"db.bgwriter.checkpointsScheduledPerSecond":             "postgresql.bgwriter.checkpoint.count",
	"db.bgwriter.checkpointWriteTimeInMillisecondsPerSecond": "postgresql.bgwriter.duration",
	"db.bgwriter.buffersWrittenByBackgroundWriterPerSecond":  "postgresql.bgwriter.buffers.writes",
	"db.commitsPerSecond":                                    "postgresql.commits",
	"db.rollbacksPerSecond":                                  "postgresql.rollbacks",
	"db.reads.blocksPerSecond":                               "postgresql.blocks_read",
	"db.writes.blocksPerSecond":                              "postgresql.blocks_written",
	"db.connections.active":                                  "postgresql.database.backends",
	
	// Query performance mappings
	"db.query.count":         "db.sql.count",
	"db.query.mean_duration": "db.sql.mean_duration",
	"db.query.duration":      "db.sql.duration",
	"db.io.disk_reads":       "db.sql.io.disk_reads",
	"db.io.disk_writes":      "db.sql.io.disk_writes",
}

// NewValidator creates a new validator instance
func NewValidator(config Config) (*Validator, error) {
	// Create logger for validation
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	
	// Use validation-specific connection pool configuration
	poolConfig := database.ValidationConnectionPoolConfig()
	
	// Connect to PostgreSQL with secure connection pooling
	db, err := database.OpenWithSecurePool("postgres", config.PostgresURL, poolConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to establish secure database connection: %w", err)
	}

	// Create New Relic client
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create New Relic client: %w", err)
	}

	return &Validator{
		db:       db,
		nrClient: nrClient,
		config:   config,
		results:  []ValidationResult{},
		logger:   log.New(os.Stdout, "[validator] ", log.LstdFlags),
	}, nil
}

// ValidateAll runs all validation checks
func (v *Validator) ValidateAll() error {
	log.Println("Starting OHI vs OTEL validation...")

	// Validate core database metrics
	if err := v.validateDatabaseMetrics(); err != nil {
		log.Printf("Warning: Database metrics validation failed: %v", err)
	}

	// Validate query performance metrics
	if err := v.validateQueryMetrics(); err != nil {
		log.Printf("Warning: Query metrics validation failed: %v", err)
	}

	// Validate connection metrics
	if err := v.validateConnectionMetrics(); err != nil {
		log.Printf("Warning: Connection metrics validation failed: %v", err)
	}

	// Generate report
	v.generateReport()

	return nil
}

// validateDatabaseMetrics compares core database metrics
func (v *Validator) validateDatabaseMetrics() error {
	metrics := []string{
		"db.commitsPerSecond",
		"db.rollbacksPerSecond",
		"db.reads.blocksPerSecond",
		"db.writes.blocksPerSecond",
	}

	for _, metric := range metrics {
		ohiValue, err := v.getOHIMetric(metric)
		if err != nil {
			v.addResult(metric, 0, 0, "FAILED", fmt.Sprintf("Failed to get OHI metric: %v", err))
			continue
		}

		otelMetric := MetricMappings[metric]
		otelValue, err := v.getOTELMetric(otelMetric)
		if err != nil {
			v.addResult(metric, ohiValue, 0, "FAILED", fmt.Sprintf("Failed to get OTEL metric: %v", err))
			continue
		}

		v.compareMetrics(metric, ohiValue, otelValue)
	}

	return nil
}

// validateQueryMetrics compares query performance metrics
func (v *Validator) validateQueryMetrics() error {
	// Get top queries from both systems
	ohiQueries, err := v.getOHITopQueries()
	if err != nil {
		return fmt.Errorf("failed to get OHI queries: %w", err)
	}

	otelQueries, err := v.getOTELTopQueries()
	if err != nil {
		return fmt.Errorf("failed to get OTEL queries: %w", err)
	}

	// Compare query counts and performance
	v.compareQuerySets(ohiQueries, otelQueries)

	return nil
}

// validateConnectionMetrics compares connection-related metrics
func (v *Validator) validateConnectionMetrics() error {
	// Direct database query for current connections
	var actualConnections int
	err := v.db.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE state != 'idle'").Scan(&actualConnections)
	if err != nil {
		return fmt.Errorf("failed to query actual connections: %w", err)
	}

	// Get OHI metric
	ohiConnections, err := v.getOHIMetric("db.connections.active")
	if err != nil {
		return fmt.Errorf("failed to get OHI connections: %w", err)
	}

	// Get OTEL metric
	otelConnections, err := v.getOTELMetric("postgresql.database.backends")
	if err != nil {
		return fmt.Errorf("failed to get OTEL connections: %w", err)
	}

	// Compare all three
	v.addResult("connections_vs_actual", ohiConnections, float64(actualConnections), 
		v.getStatus(ohiConnections, float64(actualConnections)), "OHI vs Actual DB")
	v.addResult("otel_vs_actual", otelConnections, float64(actualConnections), 
		v.getStatus(otelConnections, float64(actualConnections)), "OTEL vs Actual DB")

	return nil
}

// getOHIMetric queries OHI metric from New Relic
func (v *Validator) getOHIMetric(metricName string) (float64, error) {
	query := fmt.Sprintf(`
		SELECT average(%s) 
		FROM PostgreSQLSample 
		WHERE hostname = '%s' 
		SINCE 5 minutes ago
	`, metricName, v.getHostname())

	result, err := v.nrClient.Nrdb.Query(v.config.NewRelicAccount, nrdb.NRQL(query))
	if err != nil {
		return 0, err
	}

	// Parse result
	if len(result.Results) > 0 && len(result.Results[0]) > 0 {
		if val, ok := result.Results[0]["average"].(float64); ok {
			return val, nil
		}
	}

	return 0, fmt.Errorf("no data found for metric %s", metricName)
}

// getOTELMetric queries OTEL metric from New Relic
func (v *Validator) getOTELMetric(metricName string) (float64, error) {
	query := fmt.Sprintf(`
		SELECT average(%s) 
		FROM Metric 
		WHERE host.name = '%s' 
		AND telemetry.source = 'otel'
		SINCE 5 minutes ago
	`, metricName, v.getHostname())

	result, err := v.nrClient.Nrdb.Query(v.config.NewRelicAccount, nrdb.NRQL(query))
	if err != nil {
		return 0, err
	}

	// Parse result
	if len(result.Results) > 0 && len(result.Results[0]) > 0 {
		if val, ok := result.Results[0]["average"].(float64); ok {
			return val, nil
		}
	}

	return 0, fmt.Errorf("no data found for metric %s", metricName)
}

// getOHITopQueries gets top queries from OHI
func (v *Validator) getOHITopQueries() ([]map[string]interface{}, error) {
	query := `
		SELECT 
			statement_type,
			average(avg_elapsed_time_ms) as avg_duration,
			sum(execution_count) as total_executions
		FROM PostgresSlowQueries
		WHERE hostname = '` + v.getHostname() + `'
		SINCE 1 hour ago
		FACET statement_type
		LIMIT 10
	`

	result, err := v.nrClient.Nrdb.Query(v.config.NewRelicAccount, nrdb.NRQL(query))
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, len(result.Results))
	for i, r := range result.Results {
		results[i] = r
	}

	return results, nil
}

// getOTELTopQueries gets top queries from OTEL
func (v *Validator) getOTELTopQueries() ([]map[string]interface{}, error) {
	query := `
		SELECT 
			statement_type,
			average(db.query.mean_duration) as avg_duration,
			sum(db.query.count) as total_executions
		FROM Metric
		WHERE host.name = '` + v.getHostname() + `'
		AND telemetry.source = 'otel'
		SINCE 1 hour ago
		FACET statement_type
		LIMIT 10
	`

	result, err := v.nrClient.Nrdb.Query(v.config.NewRelicAccount, nrdb.NRQL(query))
	if err != nil {
		return nil, err
	}

	results := make([]map[string]interface{}, len(result.Results))
	for i, r := range result.Results {
		results[i] = r
	}

	return results, nil
}

// compareMetrics compares two metric values
func (v *Validator) compareMetrics(name string, ohiValue, otelValue float64) {
	diff := otelValue - ohiValue
	percentDiff := 0.0
	if ohiValue != 0 {
		percentDiff = (diff / ohiValue) * 100
	}

	status := v.getStatus(ohiValue, otelValue)
	
	// Log significant differences
	if status != StatusMatch && percentDiff > v.config.Tolerance {
		v.logger.Printf("WARNING: Metric %s differs by %.2f%%", name, percentDiff)
	}
	
	v.addResult(name, ohiValue, otelValue, status, "")
}

// getStatus determines if values are within tolerance
func (v *Validator) getStatus(value1, value2 float64) string {
	if value1 == 0 && value2 == 0 {
		return "MATCH"
	}
	if value1 == 0 || value2 == 0 {
		return "MISSING"
	}

	diff := value2 - value1
	percentDiff := (diff / value1) * 100
	if percentDiff < 0 {
		percentDiff = -percentDiff
	}

	if percentDiff <= v.config.Tolerance {
		return "PASS"
	}
	return "FAIL"
}

// compareQuerySets compares query performance between OHI and OTEL
func (v *Validator) compareQuerySets(ohiQueries, otelQueries []map[string]interface{}) {
	// Build maps for easier comparison
	ohiMap := make(map[string]map[string]interface{})
	for _, q := range ohiQueries {
		if stmt, ok := q["statement_type"].(string); ok {
			ohiMap[stmt] = q
		}
	}

	otelMap := make(map[string]map[string]interface{})
	for _, q := range otelQueries {
		if stmt, ok := q["statement_type"].(string); ok {
			otelMap[stmt] = q
		}
	}

	// Compare each statement type
	for stmt, ohiData := range ohiMap {
		otelData, exists := otelMap[stmt]
		if !exists {
			v.addResult(fmt.Sprintf("query_%s_exists", stmt), 1, 0, "MISSING", "Query type missing in OTEL")
			continue
		}

		// Compare average duration
		ohiDuration, _ := ohiData["avg_duration"].(float64)
		otelDuration, _ := otelData["avg_duration"].(float64)
		v.compareMetrics(fmt.Sprintf("query_%s_duration", stmt), ohiDuration, otelDuration)

		// Compare execution count
		ohiCount, _ := ohiData["total_executions"].(float64)
		otelCount, _ := otelData["total_executions"].(float64)
		v.compareMetrics(fmt.Sprintf("query_%s_count", stmt), ohiCount, otelCount)
	}
}

// addResult adds a validation result
func (v *Validator) addResult(metric string, ohiValue, otelValue float64, status, reason string) {
	diff := otelValue - ohiValue
	percentDiff := 0.0
	if ohiValue != 0 {
		percentDiff = (diff / ohiValue) * 100
	}

	v.results = append(v.results, ValidationResult{
		MetricName:    metric,
		OHIValue:      ohiValue,
		OTELValue:     otelValue,
		Difference:    diff,
		PercentDiff:   percentDiff,
		Status:        status,
		FailureReason: reason,
	})
}

// generateReport creates a validation report
func (v *Validator) generateReport() {
	// Summary statistics
	total := len(v.results)
	passed := 0
	failed := 0
	missing := 0

	for _, r := range v.results {
		switch r.Status {
		case "PASS", "MATCH":
			passed++
		case "FAIL":
			failed++
		case "MISSING":
			missing++
		}
	}

	// Generate JSON report
	report := map[string]interface{}{
		"timestamp": time.Now(),
		"summary": map[string]interface{}{
			"total":   total,
			"passed":  passed,
			"failed":  failed,
			"missing": missing,
			"success_rate": float64(passed) / float64(total) * 100,
		},
		"tolerance": v.config.Tolerance,
		"results":   v.results,
	}

	// Write to file
	data, _ := json.MarshalIndent(report, "", "  ")
	filename := fmt.Sprintf("validation_report_%s.json", time.Now().Format("20060102_150405"))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("Failed to write report: %v", err)
	}

	// Print summary
	fmt.Printf("\n=== OHI vs OTEL Validation Summary ===\n")
	fmt.Printf("Total Metrics Checked: %d\n", total)
	fmt.Printf("Passed: %d (%.1f%%)\n", passed, float64(passed)/float64(total)*100)
	fmt.Printf("Failed: %d (%.1f%%)\n", failed, float64(failed)/float64(total)*100)
	fmt.Printf("Missing: %d (%.1f%%)\n", missing, float64(missing)/float64(total)*100)
	fmt.Printf("Report saved to: %s\n", filename)

	// Print failures
	if failed > 0 || missing > 0 {
		fmt.Printf("\n=== Failed/Missing Metrics ===\n")
		for _, r := range v.results {
			if r.Status == "FAIL" || r.Status == "MISSING" {
				fmt.Printf("- %s: OHI=%.2f, OTEL=%.2f, Diff=%.1f%% [%s]\n", 
					r.MetricName, r.OHIValue, r.OTELValue, r.PercentDiff, r.Status)
				if r.FailureReason != "" {
					fmt.Printf("  Reason: %s\n", r.FailureReason)
				}
			}
		}
	}
}

// getHostname returns the current hostname
func (v *Validator) getHostname() string {
	hostname, _ := os.Hostname()
	return hostname
}

// Close cleans up resources
func (v *Validator) Close() {
	if v.db != nil {
		v.db.Close()
	}
}

func main() {
	config := Config{
		PostgresURL:      os.Getenv("POSTGRES_URL"),
		NewRelicAPIKey:   os.Getenv("NEW_RELIC_API_KEY"),
		NewRelicAccount:  123456, // Replace with actual account ID
		ValidationPeriod: 5 * time.Minute,
		Tolerance:        5.0, // 5% tolerance
	}

	validator, err := NewValidator(config)
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}
	defer validator.Close()

	if err := validator.ValidateAll(); err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
}