package suites

import (
	"os"
	"testing"
)

// TestMain provides setup and teardown for all e2e tests
func TestMain(m *testing.M) {
	// Setup code here if needed
	
	// Run tests
	code := m.Run()
	
	// Cleanup code here if needed
	
	os.Exit(code)
}

// TestAll runs all test suites when not in short mode
func TestAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping all e2e tests in short mode")
	}
	
	// Run each test suite
	t.Run("Comprehensive", func(t *testing.T) {
		TestComprehensiveSuite(t)
	})
	
	t.Run("CustomProcessors", func(t *testing.T) {
		TestCustomProcessorsSuite(t)
	})
	
	t.Run("ModeComparison", func(t *testing.T) {
		TestModeComparisonSuite(t)
	})
	
	t.Run("ASHPlanAnalysis", func(t *testing.T) {
		TestASHPlanAnalysisSuite(t)
	})
	
	t.Run("PerformanceScale", func(t *testing.T) {
		TestPerformanceScaleSuite(t)
	})
	
	t.Run("NewRelicValidation", func(t *testing.T) {
		// Only run if New Relic credentials are available
		if os.Getenv("NEW_RELIC_LICENSE_KEY") != "" {
			TestNewRelicValidationSuite(t)
		} else {
			t.Skip("Skipping New Relic validation: credentials not set")
		}
	})
}