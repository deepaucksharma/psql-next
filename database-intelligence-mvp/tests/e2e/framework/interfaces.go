package framework

import (
	"context"
	"time"
)

// TestSuite represents a collection of related test cases
type TestSuite interface {
	// Name returns the unique name of the test suite
	Name() string
	
	// Setup prepares the test suite for execution
	Setup(env TestEnvironment) error
	
	// Execute runs all test cases in the suite
	Execute(ctx context.Context, env TestEnvironment) (*TestResult, error)
	
	// Cleanup performs cleanup operations after test execution
	Cleanup() error
	
	// GetMetadata returns metadata about the test suite
	GetMetadata() *SuiteMetadata
}

// TestEnvironment represents the test execution environment
type TestEnvironment interface {
	// GetInfo returns information about the environment
	GetInfo() *EnvironmentInfo
	
	// GetConnectionInfo returns database connection information
	GetConnectionInfo() *ConnectionInfo
	
	// GetCollectorEndpoint returns the collector endpoint URL
	GetCollectorEndpoint() string
	
	// GetMetricsEndpoint returns the metrics endpoint URL
	GetMetricsEndpoint() string
	
	// IsHealthy checks if the environment is healthy
	IsHealthy() bool
	
	// GetTempDir returns a temporary directory for test artifacts
	GetTempDir() string
}

// EnvironmentManager manages test environment lifecycle
type EnvironmentManager interface {
	// Name returns the environment manager name
	Name() string
	
	// Provision creates and configures the test environment
	Provision(ctx context.Context) (TestEnvironment, error)
	
	// WaitForReady waits for the environment to be ready
	WaitForReady(ctx context.Context, timeout time.Duration) error
	
	// Cleanup destroys the test environment
	Cleanup() error
	
	// HealthCheck verifies environment health
	HealthCheck() error
}

// DataGenerator generates test data for various scenarios
type DataGenerator interface {
	// GenerateWorkload creates a database workload
	GenerateWorkload(config *WorkloadConfig) (*TestData, error)
	
	// GeneratePIIData creates PII test data
	GeneratePIIData(categories []PIICategory) (*PIITestData, error)
	
	// GenerateLoadPattern creates load testing data
	GenerateLoadPattern(pattern LoadPattern) (*LoadData, error)
	
	// GenerateFailureScenario creates failure scenario data
	GenerateFailureScenario(scenario FailureScenario) (*FailureData, error)
}

// Validator validates test results and system behavior
type Validator interface {
	// Validate performs validation and returns results
	Validate(ctx context.Context, data interface{}) (*ValidationResult, error)
	
	// GetName returns the validator name
	GetName() string
	
	// GetDescription returns validator description
	GetDescription() string
}

// MetricsCollector collects and validates metrics
type MetricsCollector interface {
	// CollectMetrics gathers metrics from various sources
	CollectMetrics(ctx context.Context, sources []string) (*MetricsData, error)
	
	// ValidateMetrics checks if metrics meet expectations
	ValidateMetrics(expected, actual *MetricsData) (*MetricsValidation, error)
	
	// GetMetricValue retrieves a specific metric value
	GetMetricValue(name string) (interface{}, error)
}

// Reporter generates test reports in various formats
type Reporter interface {
	// GenerateReports creates all configured reports
	GenerateReports(result *ExecutionResult) error
	
	// GenerateExecutiveSummary creates high-level summary
	GenerateExecutiveSummary(result *ExecutionResult) error
	
	// GenerateTechnicalReport creates detailed technical report
	GenerateTechnicalReport(result *ExecutionResult) error
	
	// GenerateDashboard creates interactive dashboard
	GenerateDashboard(result *ExecutionResult) error
}

// ResultCollector stores and retrieves test results
type ResultCollector interface {
	// Store saves test execution results
	Store(result *ExecutionResult) error
	
	// GetExecutionResult retrieves stored execution result
	GetExecutionResult() (*ExecutionResult, error)
	
	// GetSuiteResult retrieves specific suite result
	GetSuiteResult(suiteName string) (*TestResult, error)
	
	// ListExecutions returns list of all executions
	ListExecutions() ([]string, error)
}

// ConfigManager handles test configuration
type ConfigManager interface {
	// LoadConfig loads configuration from file
	LoadConfig(path string) (*TestConfig, error)
	
	// SaveConfig saves configuration to file
	SaveConfig(config *TestConfig, path string) error
	
	// ValidateConfig validates configuration
	ValidateConfig(config *TestConfig) error
	
	// MergeConfigs merges multiple configurations
	MergeConfigs(base, override *TestConfig) (*TestConfig, error)
}

// Notifier sends notifications about test results
type Notifier interface {
	// NotifyStart sends notification when tests start
	NotifyStart(execution *ExecutionResult) error
	
	// NotifyComplete sends notification when tests complete
	NotifyComplete(execution *ExecutionResult) error
	
	// NotifyFailure sends notification on test failure
	NotifyFailure(execution *ExecutionResult, failure *TestFailure) error
}

// ArtifactManager handles test artifacts
type ArtifactManager interface {
	// StoreArtifact saves a test artifact
	StoreArtifact(name string, data []byte) (string, error)
	
	// GetArtifact retrieves a test artifact
	GetArtifact(name string) ([]byte, error)
	
	// ListArtifacts returns list of all artifacts
	ListArtifacts() ([]string, error)
	
	// CleanupArtifacts removes old artifacts
	CleanupArtifacts(olderThan time.Duration) error
}

// SecurityScanner performs security testing
type SecurityScanner interface {
	// ScanForPII scans data for personally identifiable information
	ScanForPII(data []byte) (*PIIScanResult, error)
	
	// ScanForVulnerabilities checks for security vulnerabilities
	ScanForVulnerabilities(target string) (*VulnerabilityScanResult, error)
	
	// ValidateCompliance checks compliance with standards
	ValidateCompliance(standard ComplianceStandard, data interface{}) (*ComplianceResult, error)
}

// PerformanceProfiler profiles system performance
type PerformanceProfiler interface {
	// StartProfiling begins performance profiling
	StartProfiling() error
	
	// StopProfiling ends profiling and returns results
	StopProfiling() (*PerformanceProfile, error)
	
	// GetCPUProfile returns CPU profiling data
	GetCPUProfile() (*CPUProfile, error)
	
	// GetMemoryProfile returns memory profiling data
	GetMemoryProfile() (*MemoryProfile, error)
	
	// GetNetworkProfile returns network profiling data
	GetNetworkProfile() (*NetworkProfile, error)
}

// LoadGenerator generates various types of load
type LoadGenerator interface {
	// StartLoad begins load generation
	StartLoad(config *LoadConfig) error
	
	// StopLoad stops load generation
	StopLoad() error
	
	// GetLoadMetrics returns current load metrics
	GetLoadMetrics() (*LoadMetrics, error)
	
	// AdjustLoad modifies load parameters
	AdjustLoad(config *LoadConfig) error
}

// FailureInjector simulates various failure scenarios
type FailureInjector interface {
	// InjectNetworkPartition simulates network partition
	InjectNetworkPartition(config *NetworkPartitionConfig) error
	
	// InjectDiskFailure simulates disk failure
	InjectDiskFailure(config *DiskFailureConfig) error
	
	// InjectMemoryPressure simulates memory pressure
	InjectMemoryPressure(config *MemoryPressureConfig) error
	
	// InjectCPUStress simulates CPU stress
	InjectCPUStress(config *CPUStressConfig) error
	
	// RecoverFromFailure removes injected failures
	RecoverFromFailure() error
}

// ComplianceValidator validates regulatory compliance
type ComplianceValidator interface {
	// ValidateGDPR checks GDPR compliance
	ValidateGDPR(data interface{}) (*GDPRValidation, error)
	
	// ValidateHIPAA checks HIPAA compliance
	ValidateHIPAA(data interface{}) (*HIPAAValidation, error)
	
	// ValidatePCIDSS checks PCI DSS compliance
	ValidatePCIDSS(data interface{}) (*PCIDSSValidation, error)
	
	// ValidateSOC2 checks SOC 2 compliance
	ValidateSOC2(data interface{}) (*SOC2Validation, error)
}