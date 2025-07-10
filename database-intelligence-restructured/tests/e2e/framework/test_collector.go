package framework

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TestCollector represents a test instance of the OpenTelemetry collector
type TestCollector struct {
	env        *TestEnvironment
	configPath string
	cmd        *exec.Cmd
	logFile    *os.File
}

// NewTestCollector creates a new test collector instance
func NewTestCollector(env *TestEnvironment) *TestCollector {
	return &TestCollector{
		env: env,
	}
}

// Start starts the collector with the given configuration
func (tc *TestCollector) Start(config string) error {
	// Write config to temp file
	configPath := filepath.Join(tc.env.TempDir, "collector-config.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	tc.configPath = configPath
	
	// Create log file
	logPath := filepath.Join(tc.env.TempDir, "collector.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	tc.logFile = logFile
	
	// Build collector command
	collectorBinary := os.Getenv("COLLECTOR_BINARY")
	if collectorBinary == "" {
		// Try to find the collector binary
		possiblePaths := []string{
			"../../working-collector/collector",
			"../../core/cmd/collector/collector",
			"./collector",
			"otelcol-custom",
		}
		
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				collectorBinary = path
				break
			}
		}
		
		if collectorBinary == "" {
			return fmt.Errorf("collector binary not found")
		}
	}
	
	// Start collector
	tc.cmd = exec.Command(collectorBinary, "--config", configPath)
	tc.cmd.Stdout = tc.logFile
	tc.cmd.Stderr = tc.logFile
	
	// Set environment variables
	tc.cmd.Env = append(os.Environ(),
		fmt.Sprintf("POSTGRES_HOST=%s", tc.env.PostgresHost),
		fmt.Sprintf("POSTGRES_PORT=%d", tc.env.PostgresPort),
		fmt.Sprintf("POSTGRES_USER=%s", tc.env.PostgresUser),
		fmt.Sprintf("POSTGRES_PASSWORD=%s", tc.env.PostgresPassword),
		fmt.Sprintf("POSTGRES_DB=%s", tc.env.PostgresDatabase),
		fmt.Sprintf("MYSQL_HOST=%s", tc.env.MySQLHost),
		fmt.Sprintf("MYSQL_PORT=%d", tc.env.MySQLPort),
		fmt.Sprintf("MYSQL_USER=%s", tc.env.MySQLUser),
		fmt.Sprintf("MYSQL_PASSWORD=%s", tc.env.MySQLPassword),
		fmt.Sprintf("MYSQL_DB=%s", tc.env.MySQLDatabase),
		fmt.Sprintf("NEW_RELIC_LICENSE_KEY=%s", tc.env.NewRelicLicenseKey),
		fmt.Sprintf("NEW_RELIC_OTLP_ENDPOINT=%s", tc.env.NewRelicEndpoint),
	)
	
	if err := tc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}
	
	// Wait for collector to be ready
	if err := tc.env.WaitForCollector(30 * time.Second); err != nil {
		tc.Stop()
		return fmt.Errorf("collector failed to start: %w", err)
	}
	
	return nil
}

// Stop stops the collector
func (tc *TestCollector) Stop() error {
	if tc.cmd != nil && tc.cmd.Process != nil {
		// Send interrupt signal
		tc.cmd.Process.Signal(os.Interrupt)
		
		// Wait for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- tc.cmd.Wait()
		}()
		
		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(10 * time.Second):
			// Force kill after timeout
			tc.cmd.Process.Kill()
		}
	}
	
	if tc.logFile != nil {
		tc.logFile.Close()
	}
	
	return nil
}

// GetLogs returns the collector logs
func (tc *TestCollector) GetLogs() (string, error) {
	if tc.logFile == nil {
		return "", fmt.Errorf("no log file available")
	}
	
	logPath := tc.logFile.Name()
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}
	
	return string(content), nil
}

// Restart restarts the collector with the same configuration
func (tc *TestCollector) Restart() error {
	if err := tc.Stop(); err != nil {
		return fmt.Errorf("failed to stop collector: %w", err)
	}
	
	// Small delay to ensure clean shutdown
	time.Sleep(2 * time.Second)
	
	config, err := os.ReadFile(tc.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	
	return tc.Start(string(config))
}

// UpdateConfig updates the collector configuration and restarts
func (tc *TestCollector) UpdateConfig(newConfig string) error {
	if err := os.WriteFile(tc.configPath, []byte(newConfig), 0644); err != nil {
		return fmt.Errorf("failed to write new config: %w", err)
	}
	
	return tc.Restart()
}

// SendMetricWithAttributes sends a metric through the collector (simulated)
func (tc *TestCollector) SendMetricWithAttributes(metricName string, value interface{}, attributes map[string]interface{}) error {
	// This would typically send metrics through OTLP
	// For now, we'll rely on the collector's own receivers to collect metrics
	// This is a placeholder for future implementation
	return nil
}

// WaitForMetricCollection waits for a collection cycle to complete
func (tc *TestCollector) WaitForMetricCollection(interval time.Duration) {
	// Wait for collection interval plus processing time
	time.Sleep(interval + 5*time.Second)
}

// VerifyProcessorEnabled verifies a processor is enabled in the pipeline
func (tc *TestCollector) VerifyProcessorEnabled(processorName string) error {
	logs, err := tc.GetLogs()
	if err != nil {
		return err
	}
	
	// Check for processor initialization in logs
	expectedLog := fmt.Sprintf("processor %s started", processorName)
	if !contains(logs, expectedLog) {
		// Try alternative log format
		expectedLog = fmt.Sprintf("Starting processor %s", processorName)
		if !contains(logs, expectedLog) {
			return fmt.Errorf("processor %s not found in logs", processorName)
		}
	}
	
	return nil
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr ||
		len(s) > len(substr) && containsHelper(s[1:], substr)
}

func containsHelper(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[0:len(substr)] == substr {
		return true
	}
	return containsHelper(s[1:], substr)
}