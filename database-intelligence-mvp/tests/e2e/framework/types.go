package framework

import (
	"encoding/json"
	"time"
)

// TestStatus represents the status of a test
type TestStatus string

const (
	StatusPending  TestStatus = "pending"
	StatusRunning  TestStatus = "running"
	StatusPassed   TestStatus = "passed"
	StatusFailed   TestStatus = "failed"
	StatusSkipped  TestStatus = "skipped"
	StatusCanceled TestStatus = "canceled"
)

// TestConfig represents the overall test configuration
type TestConfig struct {
	Framework FrameworkConfig            `yaml:"framework" json:"framework"`
	Environments map[string]EnvironmentConfig `yaml:"environments" json:"environments"`
	TestSuites map[string]SuiteConfig    `yaml:"test_suites" json:"test_suites"`
	Reporting  ReportingConfig           `yaml:"reporting" json:"reporting"`
	Security   SecurityConfig            `yaml:"security" json:"security"`
}

// FrameworkConfig contains framework-level configuration
type FrameworkConfig struct {
	Version              string        `yaml:"version" json:"version"`
	ParallelExecution    bool          `yaml:"parallel_execution" json:"parallel_execution"`
	MaxConcurrentSuites  int           `yaml:"max_concurrent_suites" json:"max_concurrent_suites"`
	DefaultTimeout       string        `yaml:"default_timeout" json:"default_timeout"`
	ContinueOnError      bool          `yaml:"continue_on_error" json:"continue_on_error"`
	ArtifactRetention    string        `yaml:"artifact_retention" json:"artifact_retention"`
}

// EnvironmentConfig contains environment-specific configuration
type EnvironmentConfig struct {
	Type             string                 `yaml:"type" json:"type"`
	DockerCompose    string                 `yaml:"docker_compose,omitempty" json:"docker_compose,omitempty"`
	KubernetesConfig string                 `yaml:"kubernetes_config,omitempty" json:"kubernetes_config,omitempty"`
	Databases        DatabasesConfig        `yaml:"databases" json:"databases"`
	Resources        ResourcesConfig        `yaml:"resources" json:"resources"`
	NetworkConfig    NetworkConfig          `yaml:"network" json:"network"`
	Environment      map[string]string      `yaml:"environment" json:"environment"`
}

// DatabasesConfig contains database connection configuration
type DatabasesConfig struct {
	PostgreSQL DatabaseConfig `yaml:"postgresql" json:"postgresql"`
	MySQL      DatabaseConfig `yaml:"mysql" json:"mysql"`
}

// DatabaseConfig contains individual database configuration
type DatabaseConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Database string `yaml:"database" json:"database"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	SSL      bool   `yaml:"ssl" json:"ssl"`
}

// ResourcesConfig contains resource allocation configuration
type ResourcesConfig struct {
	CPU    string `yaml:"cpu" json:"cpu"`
	Memory string `yaml:"memory" json:"memory"`
	Disk   string `yaml:"disk" json:"disk"`
}

// NetworkConfig contains network configuration
type NetworkConfig struct {
	CollectorPort int    `yaml:"collector_port" json:"collector_port"`
	MetricsPort   int    `yaml:"metrics_port" json:"metrics_port"`
	NetworkName   string `yaml:"network_name" json:"network_name"`
}

// SuiteConfig contains test suite configuration
type SuiteConfig struct {
	Enabled          bool                   `yaml:"enabled" json:"enabled"`
	Timeout          string                 `yaml:"timeout" json:"timeout"`
	Parameters       map[string]interface{} `yaml:"parameters" json:"parameters"`
	Dependencies     []string               `yaml:"dependencies" json:"dependencies"`
	Tags             []string               `yaml:"tags" json:"tags"`
}

// ReportingConfig contains reporting configuration
type ReportingConfig struct {
	Formats           []string `yaml:"formats" json:"formats"`
	OutputDir         string   `yaml:"output_dir" json:"output_dir"`
	MetricsCollection bool     `yaml:"metrics_collection" json:"metrics_collection"`
	DashboardGeneration bool   `yaml:"dashboard_generation" json:"dashboard_generation"`
	Notifications     NotificationConfig `yaml:"notifications" json:"notifications"`
}

// NotificationConfig contains notification configuration
type NotificationConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	Channels []string `yaml:"channels" json:"channels"`
	Webhook  string   `yaml:"webhook,omitempty" json:"webhook,omitempty"`
	Email    string   `yaml:"email,omitempty" json:"email,omitempty"`
}

// SecurityConfig contains security testing configuration
type SecurityConfig struct {
	PIIDetection      PIIDetectionConfig    `yaml:"pii_detection" json:"pii_detection"`
	ComplianceStandards []ComplianceStandard `yaml:"compliance_standards" json:"compliance_standards"`
	VulnerabilityScanning bool              `yaml:"vulnerability_scanning" json:"vulnerability_scanning"`
}

// PIIDetectionConfig contains PII detection configuration
type PIIDetectionConfig struct {
	Enabled    bool           `yaml:"enabled" json:"enabled"`
	Categories []PIICategory  `yaml:"categories" json:"categories"`
	Patterns   []PIIPattern   `yaml:"patterns" json:"patterns"`
}

// SuiteMetadata contains metadata about a test suite
type SuiteMetadata struct {
	Description       string        `json:"description"`
	Priority          int           `json:"priority"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	Tags              []string      `json:"tags"`
	Dependencies      []string      `json:"dependencies"`
	Author            string        `json:"author"`
	Version           string        `json:"version"`
}

// ExecutionResult represents the overall test execution result
type ExecutionResult struct {
	ExecutionID string             `json:"execution_id"`
	Status      TestStatus         `json:"status"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	Environment *EnvironmentInfo   `json:"environment"`
	Results     []*TestResult      `json:"results"`
	Summary     *ExecutionSummary  `json:"summary"`
	Error       error              `json:"error,omitempty"`
}

// Duration returns the execution duration
func (er *ExecutionResult) Duration() time.Duration {
	return er.EndTime.Sub(er.StartTime)
}

// HasFailures returns true if any test failed
func (er *ExecutionResult) HasFailures() bool {
	return er.Status == StatusFailed
}

// ExecutionSummary contains summary statistics
type ExecutionSummary struct {
	TotalSuites   int           `json:"total_suites"`
	PassedSuites  int           `json:"passed_suites"`
	FailedSuites  int           `json:"failed_suites"`
	SkippedSuites int           `json:"skipped_suites"`
	TotalDuration time.Duration `json:"total_duration"`
	PassRate      float64       `json:"pass_rate"`
}

// TestResult represents the result of a single test suite
type TestResult struct {
	SuiteName   string              `json:"suite_name"`
	Status      TestStatus          `json:"status"`
	StartTime   time.Time           `json:"start_time"`
	EndTime     time.Time           `json:"end_time"`
	TestCases   []*TestCaseResult   `json:"test_cases"`
	Metrics     map[string]interface{} `json:"metrics"`
	Artifacts   []string            `json:"artifacts"`
	Environment *EnvironmentInfo    `json:"environment"`
	Metadata    *SuiteMetadata      `json:"metadata"`
	Error       error               `json:"error,omitempty"`
}

// Duration returns the test duration
func (tr *TestResult) Duration() time.Duration {
	return tr.EndTime.Sub(tr.StartTime)
}

// TestCaseResult represents the result of a single test case
type TestCaseResult struct {
	Name        string                 `json:"name"`
	Status      TestStatus             `json:"status"`
	Duration    time.Duration          `json:"duration"`
	Assertions  []*AssertionResult     `json:"assertions"`
	Metrics     map[string]interface{} `json:"metrics"`
	Artifacts   []string               `json:"artifacts"`
	Error       error                  `json:"error,omitempty"`
	Description string                 `json:"description"`
}

// AssertionResult represents the result of a single assertion
type AssertionResult struct {
	Name        string      `json:"name"`
	Status      TestStatus  `json:"status"`
	Expected    interface{} `json:"expected"`
	Actual      interface{} `json:"actual"`
	Message     string      `json:"message"`
	Error       error       `json:"error,omitempty"`
}

// EnvironmentInfo contains information about the test environment
type EnvironmentInfo struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	Version         string                 `json:"version"`
	Configuration   map[string]interface{} `json:"configuration"`
	Resources       *ResourcesInfo         `json:"resources"`
	Network         *NetworkInfo           `json:"network"`
	CreatedAt       time.Time              `json:"created_at"`
	HealthStatus    string                 `json:"health_status"`
}

// ResourcesInfo contains resource information
type ResourcesInfo struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

// NetworkInfo contains network information
type NetworkInfo struct {
	CollectorEndpoint string `json:"collector_endpoint"`
	MetricsEndpoint   string `json:"metrics_endpoint"`
	NetworkName       string `json:"network_name"`
}

// ConnectionInfo contains database connection information
type ConnectionInfo struct {
	PostgreSQL *DatabaseConnectionInfo `json:"postgresql"`
	MySQL      *DatabaseConnectionInfo `json:"mysql"`
}

// DatabaseConnectionInfo contains individual database connection info
type DatabaseConnectionInfo struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	SSL      bool   `json:"ssl"`
}

// TestData represents generated test data
type TestData struct {
	Tables    []*TableData           `json:"tables"`
	Queries   []*QueryData           `json:"queries"`
	Workload  *WorkloadData          `json:"workload"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// TableData represents test table data
type TableData struct {
	Name    string                 `json:"name"`
	Schema  string                 `json:"schema"`
	Rows    []map[string]interface{} `json:"rows"`
	Indexes []string               `json:"indexes"`
}

// QueryData represents test query data
type QueryData struct {
	ID          string                 `json:"id"`
	SQL         string                 `json:"sql"`
	Parameters  map[string]interface{} `json:"parameters"`
	Expected    *QueryExpectedResult   `json:"expected"`
	Category    string                 `json:"category"`
}

// QueryExpectedResult represents expected query results
type QueryExpectedResult struct {
	RowCount      int                    `json:"row_count"`
	Columns       []string               `json:"columns"`
	Duration      time.Duration          `json:"duration"`
	PlanHash      string                 `json:"plan_hash"`
	Metrics       map[string]interface{} `json:"metrics"`
}

// WorkloadData represents workload configuration
type WorkloadData struct {
	Pattern    LoadPattern   `json:"pattern"`
	Duration   time.Duration `json:"duration"`
	QPS        int           `json:"qps"`
	Concurrent int           `json:"concurrent"`
}

// PIITestData represents PII test data
type PIITestData struct {
	Categories []PIICategory          `json:"categories"`
	Samples    []*PIISample           `json:"samples"`
	Patterns   []PIIPattern           `json:"patterns"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// PIISample represents a PII sample
type PIISample struct {
	Category PIICategory `json:"category"`
	Value    string      `json:"value"`
	Pattern  string      `json:"pattern"`
	Masked   string      `json:"masked"`
}

// LoadData represents load testing data
type LoadData struct {
	Pattern   LoadPattern   `json:"pattern"`
	Duration  time.Duration `json:"duration"`
	Requests  []*LoadRequest `json:"requests"`
	Metrics   *LoadMetrics  `json:"metrics"`
}

// LoadRequest represents a single load request
type LoadRequest struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Target    string                 `json:"target"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
}

// LoadMetrics represents load testing metrics
type LoadMetrics struct {
	TotalRequests  int           `json:"total_requests"`
	SuccessfulRequests int       `json:"successful_requests"`
	FailedRequests int           `json:"failed_requests"`
	AverageLatency time.Duration `json:"average_latency"`
	ThroughputQPS  float64       `json:"throughput_qps"`
}

// FailureData represents failure scenario data
type FailureData struct {
	Scenario FailureScenario        `json:"scenario"`
	Config   map[string]interface{} `json:"config"`
	Duration time.Duration          `json:"duration"`
	Impact   *FailureImpact         `json:"impact"`
}

// FailureImpact represents the impact of a failure
type FailureImpact struct {
	Services   []string               `json:"services"`
	Metrics    map[string]interface{} `json:"metrics"`
	Recovery   time.Duration          `json:"recovery"`
}

// ValidationResult represents validation results
type ValidationResult struct {
	Valid       bool                   `json:"valid"`
	Score       float64                `json:"score"`
	Issues      []*ValidationIssue     `json:"issues"`
	Metrics     map[string]interface{} `json:"metrics"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ValidationIssue represents a validation issue
type ValidationIssue struct {
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Field       string `json:"field"`
	Expected    interface{} `json:"expected"`
	Actual      interface{} `json:"actual"`
	Suggestion  string `json:"suggestion"`
}

// MetricsData represents collected metrics
type MetricsData struct {
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Metrics   map[string]interface{} `json:"metrics"`
	Labels    map[string]string      `json:"labels"`
}

// MetricsValidation represents metrics validation results
type MetricsValidation struct {
	Valid      bool                   `json:"valid"`
	Matches    int                    `json:"matches"`
	Mismatches []*MetricMismatch      `json:"mismatches"`
	Missing    []string               `json:"missing"`
	Extra      []string               `json:"extra"`
}

// MetricMismatch represents a metric validation mismatch
type MetricMismatch struct {
	Name     string      `json:"name"`
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Tolerance float64    `json:"tolerance"`
}

// Enums and constants

// PIICategory represents categories of PII
type PIICategory string

const (
	PIICategoryEmail      PIICategory = "email"
	PIICategoryPhone      PIICategory = "phone"
	PIICategorySSN        PIICategory = "ssn"
	PIICategoryCreditCard PIICategory = "credit_card"
	PIICategoryIPAddress  PIICategory = "ip_address"
	PIICategoryAPIKey     PIICategory = "api_key"
	PIICategoryCustom     PIICategory = "custom"
)

// PIIPattern represents a PII detection pattern
type PIIPattern struct {
	Name     string      `json:"name"`
	Category PIICategory `json:"category"`
	Regex    string      `json:"regex"`
	Examples []string    `json:"examples"`
}

// LoadPattern represents load testing patterns
type LoadPattern string

const (
	LoadPatternSteady   LoadPattern = "steady"
	LoadPatternBurst    LoadPattern = "burst"
	LoadPatternRampUp   LoadPattern = "ramp_up"
	LoadPatternRampDown LoadPattern = "ramp_down"
	LoadPatternSpike    LoadPattern = "spike"
	LoadPatternChaos    LoadPattern = "chaos"
)

// FailureScenario represents failure scenarios
type FailureScenario string

const (
	FailureScenarioNetworkPartition FailureScenario = "network_partition"
	FailureScenarioDiskFailure      FailureScenario = "disk_failure"
	FailureScenarioMemoryPressure   FailureScenario = "memory_pressure"
	FailureScenarioCPUStress        FailureScenario = "cpu_stress"
	FailureScenarioProcessCrash     FailureScenario = "process_crash"
)

// ComplianceStandard represents compliance standards
type ComplianceStandard string

const (
	ComplianceStandardGDPR   ComplianceStandard = "GDPR"
	ComplianceStandardHIPAA  ComplianceStandard = "HIPAA"
	ComplianceStandardPCIDSS ComplianceStandard = "PCI_DSS"
	ComplianceStandardSOC2   ComplianceStandard = "SOC2"
)

// TestFailure represents a test failure
type TestFailure struct {
	SuiteName   string    `json:"suite_name"`
	TestCase    string    `json:"test_case"`
	Message     string    `json:"message"`
	Error       error     `json:"error"`
	Timestamp   time.Time `json:"timestamp"`
	StackTrace  string    `json:"stack_trace"`
	Artifacts   []string  `json:"artifacts"`
	Category    string    `json:"category"`
	Severity    string    `json:"severity"`
}

// WorkloadConfig represents workload generation configuration
type WorkloadConfig struct {
	Pattern     LoadPattern   `json:"pattern"`
	Duration    time.Duration `json:"duration"`
	QPS         int           `json:"qps"`
	Concurrency int           `json:"concurrency"`
	DataSize    string        `json:"data_size"`
	Tables      []string      `json:"tables"`
}

// LoadConfig represents load generation configuration
type LoadConfig struct {
	Pattern      LoadPattern   `json:"pattern"`
	Duration     time.Duration `json:"duration"`
	RequestsPerSecond int      `json:"requests_per_second"`
	Concurrency  int           `json:"concurrency"`
	RampUpTime   time.Duration `json:"ramp_up_time"`
	RampDownTime time.Duration `json:"ramp_down_time"`
}

// Helper method to check if a suite is enabled
func (tc *TestConfig) IsSuiteEnabled(suiteName string) bool {
	if config, exists := tc.TestSuites[suiteName]; exists {
		return config.Enabled
	}
	return false
}

// Additional types for interfaces.go compatibility

// Security scan results
type PIIScanResult struct {
	Found      bool
	Patterns   []string
	Locations  []string
	Severity   string
}

type VulnerabilityScanResult struct {
	Vulnerabilities []Vulnerability
	Risk            string
}

type Vulnerability struct {
	Type        string
	Severity    string
	Description string
	Location    string
}

type ComplianceResult struct {
	Standard   ComplianceStandard
	Compliant  bool
	Violations []string
	Warnings   []string
}

// Performance profiles
type PerformanceProfile struct {
	Duration    time.Duration
	CPUProfile  *CPUProfile
	MemProfile  *MemoryProfile
	NetProfile  *NetworkProfile
}

type CPUProfile struct {
	AvgUsage   float64
	MaxUsage   float64
	Cores      int
	Samples    []float64
}

type MemoryProfile struct {
	AvgUsage   uint64
	MaxUsage   uint64
	Allocated  uint64
	GCPauses   []time.Duration
}

type NetworkProfile struct {
	BytesSent     uint64
	BytesReceived uint64
	PacketsSent   uint64
	PacketsRecv   uint64
	Errors        uint64
}

// Failure injection configs
type NetworkPartitionConfig struct {
	Duration    time.Duration
	Nodes       []string
	Direction   string // "inbound", "outbound", "both"
	PacketLoss  float64
}

type DiskFailureConfig struct {
	Duration   time.Duration
	Nodes      []string
	FailureType string // "read", "write", "both"
	ErrorRate   float64
}

type MemoryPressureConfig struct {
	Duration       time.Duration
	Nodes          []string
	TargetUsage    float64
	GrowthRate     float64
}

type CPUStressConfig struct {
	Duration    time.Duration
	Nodes       []string
	TargetUsage float64
	Cores       int
}

// Validation results
type GDPRValidation struct {
	Compliant        bool
	DataMinimization bool
	ConsentTracking  bool
	RightToErasure   bool
	Violations       []string
}

type HIPAAValidation struct {
	Compliant         bool
	EncryptionAtRest  bool
	EncryptionInTransit bool
	AccessControls    bool
	AuditLogging      bool
	Violations        []string
}

type PCIDSSValidation struct {
	Compliant       bool
	NoCardStorage   bool
	Encryption      bool
	AccessControl   bool
	Monitoring      bool
	Violations      []string
}

type SOC2Validation struct {
	Compliant         bool
	SecurityPrinciple bool
	AvailabilityPrinciple bool
	ProcessingIntegrity bool
	Confidentiality   bool
	Privacy           bool
	Violations        []string
}