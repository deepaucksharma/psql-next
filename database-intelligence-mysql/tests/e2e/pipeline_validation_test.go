package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/database-intelligence/mysql-monitoring/tests/e2e/framework"
	"go.uber.org/zap"
)

// PipelineValidationTestSuite validates the complete data pipeline
type PipelineValidationTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	validator *framework.MetricValidator
	logger    *zap.Logger
}

func TestPipelineValidation(t *testing.T) {
	suite.Run(t, new(PipelineValidationTestSuite))
}

func (s *PipelineValidationTestSuite) SetupSuite() {
	s.logger, _ = zap.NewDevelopment()
	
	// Setup test environment
	s.env = framework.NewTestEnvironment()
	err := s.env.Setup()
	s.Require().NoError(err)
	
	// Initialize validator
	s.validator = framework.NewMetricValidator(s.logger, "http://localhost:9091/metrics")
	
	// Wait for pipeline to be ready
	s.waitForPipeline()
}

func (s *PipelineValidationTestSuite) TearDownSuite() {
	s.env.Teardown()
}

func (s *PipelineValidationTestSuite) TestEdgeCollectorHealth() {
	s.Run("Edge collector should be healthy", func() {
		// Check health endpoint
		resp, err := http.Get("http://localhost:13133/")
		s.NoError(err)
		defer resp.Body.Close()
		
		s.Equal(http.StatusOK, resp.StatusCode, "Edge collector health check should return OK")
		
		// Check metrics endpoint
		resp, err = http.Get("http://localhost:8888/metrics")
		s.NoError(err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		s.NoError(err)
		
		// Verify collector metrics
		metrics := string(body)
		s.Contains(metrics, "otelcol_process_uptime", "Collector uptime metric should be present")
		s.Contains(metrics, "otelcol_receiver_accepted_metric_points", "Receiver metrics should be present")
		s.Contains(metrics, "otelcol_exporter_sent_metric_points", "Exporter metrics should be present")
	})
}

func (s *PipelineValidationTestSuite) TestGatewayHealth() {
	s.Run("Gateway should be healthy", func() {
		// Check health endpoint
		resp, err := http.Get("http://localhost:13134/")
		s.NoError(err)
		defer resp.Body.Close()
		
		s.Equal(http.StatusOK, resp.StatusCode, "Gateway health check should return OK")
		
		// Check zpages
		resp, err = http.Get("http://localhost:55679/debug/tracez")
		s.NoError(err)
		defer resp.Body.Close()
		
		s.Equal(http.StatusOK, resp.StatusCode, "Gateway zpages should be accessible")
	})
}

func (s *PipelineValidationTestSuite) TestMetricFlow() {
	s.Run("Metrics should flow through the pipeline", func() {
		// Generate test workload
		s.generateTestWorkload()
		
		// Wait for metrics to propagate
		time.Sleep(30 * time.Second)
		
		// Check edge collector received metrics
		edgeMetrics := s.getCollectorMetrics("http://localhost:8888/metrics")
		receivedPoints := s.extractMetricValue(edgeMetrics, "otelcol_receiver_accepted_metric_points")
		s.Greater(receivedPoints, float64(0), "Edge collector should have received metrics")
		
		// Check gateway received metrics
		gatewayMetrics := s.getCollectorMetrics("http://localhost:8889/metrics")
		gatewayReceived := s.extractMetricValue(gatewayMetrics, "otelcol_receiver_accepted_metric_points")
		s.Greater(gatewayReceived, float64(0), "Gateway should have received metrics")
		
		// Check gateway processed metrics
		gatewaySent := s.extractMetricValue(gatewayMetrics, "otelcol_exporter_sent_metric_points")
		s.Greater(gatewaySent, float64(0), "Gateway should have sent metrics")
	})
}

func (s *PipelineValidationTestSuite) TestWaitMetricsInPrometheus() {
	s.Run("Wait metrics should be available in Prometheus", func() {
		// Generate wait-inducing workload
		s.generateWaitWorkload()
		time.Sleep(20 * time.Second)
		
		// Query Prometheus endpoint
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		// Verify wait metrics are present
		waitMetrics := []string{
			"mysql_query_wait_profile",
			"mysql_query_execution_stats",
			"mysql_gateway_mysql_query_wait_profile",
		}
		
		for _, metric := range waitMetrics {
			s.Contains(metrics, metric, fmt.Sprintf("Metric %s should be present", metric))
		}
		
		// Verify attributes are present
		requiredAttributes := []string{
			"query_hash",
			"wait_category",
			"wait_severity",
			"advisor_type",
			"service_name",
		}
		
		for _, attr := range requiredAttributes {
			s.Contains(metrics, attr+"=", fmt.Sprintf("Attribute %s should be present", attr))
		}
	})
}

func (s *PipelineValidationTestSuite) TestAdvisoryProcessing() {
	s.Run("Advisories should be processed by gateway", func() {
		// Generate queries that trigger advisories
		s.generateAdvisoryWorkload()
		time.Sleep(30 * time.Second)
		
		// Check gateway metrics for advisory processing
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		// Verify advisories are present
		s.Contains(metrics, "advisor_priority=", "Advisory priority should be set")
		s.Contains(metrics, "advisor_recommendation=", "Advisory recommendations should be present")
		
		// Check composite advisories
		s.Contains(metrics, "advisor_composite=", "Composite advisories should be generated")
	})
}

func (s *PipelineValidationTestSuite) TestNewRelicExport() {
	s.Run("Metrics should be exported to New Relic", func() {
		// Skip if no New Relic key configured
		if os.Getenv("NEW_RELIC_LICENSE_KEY") == "" {
			s.T().Skip("NEW_RELIC_LICENSE_KEY not configured")
		}
		
		// Check gateway logs for successful export
		logs := s.getGatewayLogs()
		
		// Look for successful export indicators
		s.Contains(logs, "Metrics sent successfully", "Gateway should log successful exports")
		s.NotContains(logs, "Failed to send metrics", "Gateway should not have export failures")
	})
}

func (s *PipelineValidationTestSuite) TestMemoryLimits() {
	s.Run("Collectors should respect memory limits", func() {
		// Generate heavy load
		for i := 0; i < 10; i++ {
			go s.generateHeavyWorkload()
		}
		
		time.Sleep(30 * time.Second)
		
		// Check memory usage from metrics
		edgeMetrics := s.getCollectorMetrics("http://localhost:8888/metrics")
		edgeMemory := s.extractMetricValue(edgeMetrics, "otelcol_process_memory_rss")
		
		// Edge collector should be under 384MB (with some buffer)
		s.Less(edgeMemory, float64(450*1024*1024), "Edge collector memory should be under limit")
		
		// Check gateway memory
		gatewayMetrics := s.getCollectorMetrics("http://localhost:8889/metrics")
		gatewayMemory := s.extractMetricValue(gatewayMetrics, "otelcol_process_memory_rss")
		
		// Gateway should be under 1GB (with some buffer)
		s.Less(gatewayMemory, float64(1200*1024*1024), "Gateway memory should be under limit")
	})
}

func (s *PipelineValidationTestSuite) TestBatchProcessing() {
	s.Run("Metrics should be batched appropriately", func() {
		// Generate burst of metrics
		for i := 0; i < 100; i++ {
			s.executeQuery(fmt.Sprintf("SELECT %d", i))
		}
		
		time.Sleep(15 * time.Second)
		
		// Check batch processor metrics
		gatewayMetrics := s.getCollectorMetrics("http://localhost:8889/metrics")
		
		// Look for batch processor metrics
		s.Contains(gatewayMetrics, "otelcol_processor_batch_batch_send_size", 
			"Batch processor should report batch sizes")
		s.Contains(gatewayMetrics, "otelcol_processor_batch_timeout_trigger_send", 
			"Batch processor should report timeout triggers")
	})
}

func (s *PipelineValidationTestSuite) TestCardinalityControl() {
	s.Run("Cardinality should be controlled", func() {
		// Generate high cardinality data
		for i := 0; i < 1000; i++ {
			query := fmt.Sprintf("SELECT * FROM orders WHERE order_id = %d", i)
			s.executeQuery(query)
		}
		
		time.Sleep(30 * time.Second)
		
		// Check that metrics are filtered
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		// Count unique query hashes (should be limited by sampling)
		queryHashes := s.countUniqueValues(metrics, "query_hash")
		s.Less(queryHashes, 500, "Query hash cardinality should be controlled")
	})
}

// Helper methods

func (s *PipelineValidationTestSuite) waitForPipeline() {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		// Check if edge collector is ready
		resp, err := http.Get("http://localhost:13133/")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			
			// Check if gateway is ready
			resp2, err2 := http.Get("http://localhost:13134/")
			if err2 == nil && resp2.StatusCode == http.StatusOK {
				resp2.Body.Close()
				s.logger.Info("Pipeline is ready")
				return
			}
		}
		
		s.logger.Info("Waiting for pipeline to be ready", zap.Int("attempt", i+1))
		time.Sleep(2 * time.Second)
	}
	
	s.Fail("Pipeline did not become ready in time")
}

func (s *PipelineValidationTestSuite) generateTestWorkload() {
	queries := []string{
		"SELECT COUNT(*) FROM orders",
		"SELECT * FROM inventory WHERE product_id = 1",
		"UPDATE inventory SET quantity_available = quantity_available - 1 WHERE product_id = 2",
		"SELECT o.*, oi.* FROM orders o JOIN order_items oi ON o.order_id = oi.order_id WHERE o.customer_id = 1",
	}
	
	for _, query := range queries {
		for i := 0; i < 10; i++ {
			s.executeQuery(query)
		}
	}
}

func (s *PipelineValidationTestSuite) generateWaitWorkload() {
	// Ensure we're using the test database
	_, err := s.env.GetDB().Exec("USE wait_analysis_test")
	s.NoError(err)
	
	// Call stored procedures
	procedures := []string{
		"CALL generate_io_waits()",
		"CALL generate_lock_waits()",
		"CALL generate_temp_table_waits()",
	}
	
	for _, proc := range procedures {
		_, err := s.env.GetDB().Exec(proc)
		if err != nil {
			s.logger.Warn("Failed to execute procedure", 
				zap.String("procedure", proc), 
				zap.Error(err))
		}
	}
}

func (s *PipelineValidationTestSuite) generateAdvisoryWorkload() {
	// Missing index query
	s.executeQuery(`
		SELECT * FROM order_items 
		WHERE unit_price > 50 
		ORDER BY quantity DESC
	`)
	
	// Full table scan
	s.executeQuery(`
		SELECT COUNT(*) FROM audit_log 
		WHERE details LIKE '%error%'
	`)
	
	// Inefficient join
	s.executeQuery(`
		SELECT * FROM orders o
		CROSS JOIN inventory i
		WHERE o.total_amount > i.quantity_available
	`)
}

func (s *PipelineValidationTestSuite) generateHeavyWorkload() {
	for i := 0; i < 100; i++ {
		s.executeQuery(fmt.Sprintf("SELECT COUNT(*) FROM orders WHERE customer_id = %d", i%100))
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *PipelineValidationTestSuite) executeQuery(query string) {
	_, err := s.env.GetDB().Query(query)
	if err != nil {
		s.logger.Debug("Query execution", zap.String("query", query), zap.Error(err))
	}
}

func (s *PipelineValidationTestSuite) getCollectorMetrics(endpoint string) string {
	resp, err := http.Get(endpoint)
	s.NoError(err)
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	s.NoError(err)
	
	return string(body)
}

func (s *PipelineValidationTestSuite) extractMetricValue(metrics string, metricName string) float64 {
	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var value float64
				fmt.Sscanf(parts[len(parts)-1], "%f", &value)
				return value
			}
		}
	}
	return 0
}

func (s *PipelineValidationTestSuite) getGatewayLogs() string {
	// In a real test, this would tail the actual log file
	// For now, return a placeholder
	return "Gateway logs would be checked here"
}

func (s *PipelineValidationTestSuite) countUniqueValues(metrics string, attribute string) int {
	uniqueValues := make(map[string]bool)
	lines := strings.Split(metrics, "\n")
	
	searchPattern := attribute + "=\""
	for _, line := range lines {
		if idx := strings.Index(line, searchPattern); idx != -1 {
			start := idx + len(searchPattern)
			end := strings.Index(line[start:], "\"")
			if end != -1 {
				value := line[start : start+end]
				uniqueValues[value] = true
			}
		}
	}
	
	return len(uniqueValues)
}