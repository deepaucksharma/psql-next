package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/database-intelligence-mvp/tests/e2e/framework"
	"github.com/database-intelligence-mvp/tests/e2e/suites"
)

// TestOrchestrator manages the execution of all E2E test suites
type TestOrchestrator struct {
	config          *framework.TestConfig
	environment     framework.EnvironmentManager
	suites          []framework.TestSuite
	resultCollector *framework.ResultCollector
	reporter        framework.Reporter
	ctx             context.Context
	cancel          context.CancelFunc
	mutex           sync.RWMutex
	executionID     string
}

// OrchestratorConfig contains configuration for the test orchestrator
type OrchestratorConfig struct {
	ConfigFile      string
	Environment     string
	SuitesFilter    []string
	ParallelMode    bool
	MaxConcurrency  int
	OutputDir       string
	Verbose         bool
	DryRun          bool
	ContinueOnError bool
	Timeout         time.Duration
}

func main() {
	config := parseFlags()
	
	orchestrator, err := NewTestOrchestrator(config)
	if err != nil {
		log.Fatalf("Failed to create test orchestrator: %v", err)
	}
	defer orchestrator.Shutdown()

	if config.DryRun {
		if err := orchestrator.DryRun(); err != nil {
			log.Fatalf("Dry run failed: %v", err)
		}
		return
	}

	result := orchestrator.Execute()
	
	// Generate reports
	if err := orchestrator.GenerateReports(); err != nil {
		log.Printf("Failed to generate reports: %v", err)
	}
	
	// Exit with appropriate code
	if result.HasFailures() {
		os.Exit(1)
	}
}

func parseFlags() *OrchestratorConfig {
	config := &OrchestratorConfig{}
	
	flag.StringVar(&config.ConfigFile, "config", "test_config.yaml", "Test configuration file path")
	flag.StringVar(&config.Environment, "env", "local", "Test environment (local, kubernetes, ci)")
	flag.Var((*StringSlice)(&config.SuitesFilter), "suite", "Test suites to run (can be specified multiple times)")
	flag.BoolVar(&config.ParallelMode, "parallel", true, "Enable parallel test execution")
	flag.IntVar(&config.MaxConcurrency, "max-concurrency", 4, "Maximum concurrent test suites")
	flag.StringVar(&config.OutputDir, "output", "test-results", "Output directory for test results")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be executed without running tests")
	flag.BoolVar(&config.ContinueOnError, "continue-on-error", false, "Continue executing tests after failures")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Minute, "Global timeout for test execution")
	
	flag.Parse()
	
	return config
}

// StringSlice implements flag.Value for string slices
type StringSlice []string

func (s *StringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// NewTestOrchestrator creates a new test orchestrator
func NewTestOrchestrator(config *OrchestratorConfig) (*TestOrchestrator, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	
	executionID := fmt.Sprintf("e2e_%s_%d", config.Environment, time.Now().Unix())
	
	// Load test configuration
	testConfig, err := framework.LoadTestConfig(config.ConfigFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load test config: %w", err)
	}
	
	// Override config with command line flags
	if config.ParallelMode {
		testConfig.Framework.ParallelExecution = true
	}
	if config.MaxConcurrency > 0 {
		testConfig.Framework.MaxConcurrentSuites = config.MaxConcurrency
	}
	
	// Create environment manager
	envManager, err := framework.NewEnvironmentManager(config.Environment, testConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create environment manager: %w", err)
	}
	
	// Create output directory
	outputDir := filepath.Join(config.OutputDir, executionID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	
	// Initialize result collector
	resultCollector := framework.NewResultCollector(outputDir, executionID)
	
	// Initialize reporter
	reporter := framework.NewReporter(outputDir, testConfig.Reporting)
	
	orchestrator := &TestOrchestrator{
		config:          testConfig,
		environment:     envManager,
		resultCollector: resultCollector,
		reporter:        reporter,
		ctx:             ctx,
		cancel:          cancel,
		executionID:     executionID,
	}
	
	// Initialize test suites
	if err := orchestrator.initializeTestSuites(config.SuitesFilter); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize test suites: %w", err)
	}
	
	return orchestrator, nil
}

// initializeTestSuites creates and configures all test suites
func (o *TestOrchestrator) initializeTestSuites(filter []string) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	
	// Get all available test suites
	allSuites := suites.GetAvailableSuites()
	
	// Apply filter if specified
	var enabledSuites []framework.TestSuite
	if len(filter) > 0 {
		filterMap := make(map[string]bool)
		for _, name := range filter {
			filterMap[name] = true
		}
		
		for _, suite := range allSuites {
			if filterMap[suite.Name()] {
				enabledSuites = append(enabledSuites, suite)
			}
		}
	} else {
		// Use configuration to determine enabled suites
		for _, suite := range allSuites {
			if o.config.IsSuiteEnabled(suite.Name()) {
				enabledSuites = append(enabledSuites, suite)
			}
		}
	}
	
	if len(enabledSuites) == 0 {
		return fmt.Errorf("no test suites enabled or match filter criteria")
	}
	
	// Sort suites by priority
	sort.Slice(enabledSuites, func(i, j int) bool {
		return enabledSuites[i].GetMetadata().Priority < enabledSuites[j].GetMetadata().Priority
	})
	
	o.suites = enabledSuites
	return nil
}

// DryRun shows what would be executed without running tests
func (o *TestOrchestrator) DryRun() error {
	fmt.Printf("Dry Run - Test Execution Plan\n")
	fmt.Printf("=============================\n")
	fmt.Printf("Execution ID: %s\n", o.executionID)
	fmt.Printf("Environment: %s\n", o.environment.Name())
	fmt.Printf("Parallel Mode: %t\n", o.config.Framework.ParallelExecution)
	fmt.Printf("Max Concurrency: %d\n", o.config.Framework.MaxConcurrentSuites)
	fmt.Printf("Total Timeout: %s\n", o.config.Framework.DefaultTimeout)
	fmt.Printf("\nTest Suites to Execute:\n")
	
	for i, suite := range o.suites {
		metadata := suite.GetMetadata()
		fmt.Printf("  %d. %s\n", i+1, suite.Name())
		fmt.Printf("     Description: %s\n", metadata.Description)
		fmt.Printf("     Priority: %d\n", metadata.Priority)
		fmt.Printf("     Estimated Duration: %s\n", metadata.EstimatedDuration)
		fmt.Printf("     Tags: %v\n", metadata.Tags)
		fmt.Printf("\n")
	}
	
	return nil
}

// Execute runs all configured test suites
func (o *TestOrchestrator) Execute() *framework.ExecutionResult {
	log.Printf("Starting E2E test execution (ID: %s)", o.executionID)
	
	startTime := time.Now()
	
	// Provision test environment
	log.Printf("Provisioning test environment...")
	testEnv, err := o.environment.Provision(o.ctx)
	if err != nil {
		return &framework.ExecutionResult{
			Status:    framework.StatusFailed,
			Error:     fmt.Errorf("environment provisioning failed: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
		}
	}
	
	defer func() {
		log.Printf("Cleaning up test environment...")
		if cleanupErr := o.environment.Cleanup(); cleanupErr != nil {
			log.Printf("Environment cleanup failed: %v", cleanupErr)
		}
	}()
	
	// Wait for environment to be ready
	log.Printf("Waiting for environment to be ready...")
	if err := o.environment.WaitForReady(o.ctx, 5*time.Minute); err != nil {
		return &framework.ExecutionResult{
			Status:    framework.StatusFailed,
			Error:     fmt.Errorf("environment not ready: %w", err),
			StartTime: startTime,
			EndTime:   time.Now(),
		}
	}
	
	// Execute test suites
	var results []*framework.TestResult
	if o.config.Framework.ParallelExecution {
		results = o.executeParallel(testEnv)
	} else {
		results = o.executeSequential(testEnv)
	}
	
	// Collect overall execution result
	executionResult := &framework.ExecutionResult{
		ExecutionID: o.executionID,
		StartTime:   startTime,
		EndTime:     time.Now(),
		Environment: testEnv.GetInfo(),
		Results:     results,
	}
	
	// Determine overall status
	executionResult.Status = framework.StatusPassed
	for _, result := range results {
		if result.Status == framework.StatusFailed {
			executionResult.Status = framework.StatusFailed
			break
		}
	}
	
	// Store results
	if err := o.resultCollector.Store(executionResult); err != nil {
		log.Printf("Failed to store execution result: %v", err)
	}
	
	log.Printf("E2E test execution completed in %s", executionResult.Duration())
	
	return executionResult
}

// executeSequential runs test suites one by one
func (o *TestOrchestrator) executeSequential(env framework.TestEnvironment) []*framework.TestResult {
	var results []*framework.TestResult
	
	for _, suite := range o.suites {
		select {
		case <-o.ctx.Done():
			log.Printf("Test execution cancelled")
			return results
		default:
		}
		
		log.Printf("Executing test suite: %s", suite.Name())
		result := o.executeSuite(suite, env)
		results = append(results, result)
		
		// Stop execution on failure if not continuing on error
		if result.Status == framework.StatusFailed && !o.config.Framework.ContinueOnError {
			log.Printf("Stopping execution due to test failure")
			break
		}
	}
	
	return results
}

// executeParallel runs test suites in parallel with concurrency control
func (o *TestOrchestrator) executeParallel(env framework.TestEnvironment) []*framework.TestResult {
	semaphore := make(chan struct{}, o.config.Framework.MaxConcurrentSuites)
	resultsChan := make(chan *framework.TestResult, len(o.suites))
	var wg sync.WaitGroup
	
	// Start all test suites
	for _, suite := range o.suites {
		wg.Add(1)
		go func(s framework.TestSuite) {
			defer wg.Done()
			
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
			case <-o.ctx.Done():
				return
			}
			defer func() { <-semaphore }()
			
			log.Printf("Executing test suite: %s", s.Name())
			result := o.executeSuite(s, env)
			resultsChan <- result
		}(suite)
	}
	
	// Wait for all to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	var results []*framework.TestResult
	for result := range resultsChan {
		results = append(results, result)
	}
	
	// Sort results by suite name for consistent output
	sort.Slice(results, func(i, j int) bool {
		return results[i].SuiteName < results[j].SuiteName
	})
	
	return results
}

// executeSuite runs a single test suite
func (o *TestOrchestrator) executeSuite(suite framework.TestSuite, env framework.TestEnvironment) *framework.TestResult {
	startTime := time.Now()
	
	result := &framework.TestResult{
		SuiteName:   suite.Name(),
		StartTime:   startTime,
		Environment: env.GetInfo(),
		Metadata:    suite.GetMetadata(),
	}
	
	// Setup suite
	if err := suite.Setup(env); err != nil {
		result.Status = framework.StatusFailed
		result.Error = fmt.Errorf("suite setup failed: %w", err)
		result.EndTime = time.Now()
		return result
	}
	
	// Ensure cleanup runs
	defer func() {
		if cleanupErr := suite.Cleanup(); cleanupErr != nil {
			log.Printf("Suite cleanup failed for %s: %v", suite.Name(), cleanupErr)
		}
	}()
	
	// Execute suite
	suiteResult, err := suite.Execute(o.ctx, env)
	if err != nil {
		result.Status = framework.StatusFailed
		result.Error = err
	} else {
		result.Status = suiteResult.Status
		result.TestCases = suiteResult.TestCases
		result.Metrics = suiteResult.Metrics
		result.Artifacts = suiteResult.Artifacts
	}
	
	result.EndTime = time.Now()
	
	// Log result
	status := "PASSED"
	if result.Status == framework.StatusFailed {
		status = "FAILED"
	}
	log.Printf("Test suite %s: %s (Duration: %s)", suite.Name(), status, result.Duration())
	
	return result
}

// GenerateReports creates all configured reports
func (o *TestOrchestrator) GenerateReports() error {
	log.Printf("Generating test reports...")
	
	executionResult, err := o.resultCollector.GetExecutionResult()
	if err != nil {
		return fmt.Errorf("failed to get execution result: %w", err)
	}
	
	return o.reporter.GenerateReports(executionResult)
}

// Shutdown performs cleanup operations
func (o *TestOrchestrator) Shutdown() {
	if o.cancel != nil {
		o.cancel()
	}
	
	if o.environment != nil {
		if err := o.environment.Cleanup(); err != nil {
			log.Printf("Environment cleanup error: %v", err)
		}
	}
}

// Helper function to check if context is cancelled
func (o *TestOrchestrator) isCancelled() bool {
	select {
	case <-o.ctx.Done():
		return true
	default:
		return false
	}
}