package database

import (
	"context"
	"time"
)

// Driver represents a database driver that can create connections
type Driver interface {
	// Name returns the database type name (e.g., "postgresql", "mysql", "mongodb")
	Name() string
	
	// Connect creates a new database connection
	Connect(ctx context.Context, config *Config) (Client, error)
	
	// ParseDSN parses a connection string into a Config
	ParseDSN(dsn string) (*Config, error)
	
	// SupportedFeatures returns the list of features this driver supports
	SupportedFeatures() []Feature
	
	// ValidateConfig checks if the configuration is valid for this driver
	ValidateConfig(config *Config) error
}

// Client represents a database connection
type Client interface {
	// Query executes a query that returns rows
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	
	// Exec executes a query that doesn't return rows
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
	
	// Close closes the database connection
	Close() error
	
	// Ping verifies the connection is alive
	Ping(ctx context.Context) error
	
	// Stats returns connection statistics
	Stats() Statistics
	
	// DatabaseType returns the type of database
	DatabaseType() string
}

// Config represents database connection configuration
type Config struct {
	// Common fields
	Host     string
	Port     int
	Username string
	Password string
	Database string
	
	// TLS configuration
	TLSEnabled bool
	TLSConfig  *TLSConfig
	
	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	
	// Database-specific options
	Options map[string]interface{}
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	CAFile             string
	CertFile           string
	KeyFile            string
	InsecureSkipVerify bool
}

// Feature represents a database feature
type Feature string

const (
	FeatureTransactions   Feature = "transactions"
	FeaturePreparedStmts  Feature = "prepared_statements"
	FeatureStoredProcs    Feature = "stored_procedures"
	FeatureReplication    Feature = "replication"
	FeatureClustering     Feature = "clustering"
	FeaturePartitioning   Feature = "partitioning"
	FeatureQueryPlans     Feature = "query_plans"
	FeatureMetrics        Feature = "metrics"
	FeatureStreaming      Feature = "streaming"
)

// Statistics represents database connection statistics
type Statistics struct {
	OpenConnections      int
	InUse               int
	Idle                int
	WaitCount           int64
	WaitDuration        time.Duration
	MaxIdleClosed       int64
	MaxIdleTimeClosed   int64
	MaxLifetimeClosed   int64
}

// Result represents the result of an Exec query
type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// Rows represents the result of a Query
type Rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
	Columns() ([]string, error)
	Err() error
}

// QueryAnalyzer provides query analysis capabilities
type QueryAnalyzer interface {
	// ExtractPlan extracts the execution plan for a query
	ExtractPlan(ctx context.Context, query string) (Plan, error)
	
	// IdentifySlowQueries returns queries exceeding the threshold
	IdentifySlowQueries(ctx context.Context, threshold time.Duration) ([]SlowQuery, error)
	
	// GetQueryPattern normalizes a query to its pattern
	GetQueryPattern(query string) string
	
	// AnalyzeQuery provides detailed analysis of a query
	AnalyzeQuery(ctx context.Context, query string) (*QueryAnalysis, error)
}

// Plan represents a query execution plan
type Plan interface {
	// String returns a string representation of the plan
	String() string
	
	// Cost returns the estimated cost of the plan
	Cost() float64
	
	// Operations returns the list of operations in the plan
	Operations() []PlanOperation
	
	// Format returns the plan in the specified format
	Format(format PlanFormat) string
}

// PlanOperation represents a single operation in a query plan
type PlanOperation struct {
	Type         string
	Table        string
	Index        string
	Rows         int64
	Cost         float64
	ActualRows   int64
	ActualTime   float64
	Filters      []string
	Children     []PlanOperation
}

// PlanFormat represents the format for plan output
type PlanFormat string

const (
	PlanFormatText PlanFormat = "text"
	PlanFormatJSON PlanFormat = "json"
	PlanFormatXML  PlanFormat = "xml"
)

// SlowQuery represents a slow query
type SlowQuery struct {
	Query          string
	Duration       time.Duration
	StartTime      time.Time
	User           string
	Database       string
	RowsExamined   int64
	RowsSent       int64
	LockTime       time.Duration
}

// QueryAnalysis represents detailed query analysis
type QueryAnalysis struct {
	Query          string
	Pattern        string
	Tables         []string
	Indexes        []string
	EstimatedCost  float64
	Warnings       []string
	Suggestions    []string
	Statistics     QueryStatistics
}

// QueryStatistics represents query execution statistics
type QueryStatistics struct {
	ExecutionCount   int64
	TotalTime        time.Duration
	MinTime          time.Duration
	MaxTime          time.Duration
	MeanTime         time.Duration
	StdDevTime       time.Duration
	RowsReturned     int64
	RowsExamined     int64
}

// MetricsCollector provides database metrics collection
type MetricsCollector interface {
	// CollectMetrics collects current database metrics
	CollectMetrics(ctx context.Context) (*Metrics, error)
	
	// CollectTableMetrics collects metrics for specific tables
	CollectTableMetrics(ctx context.Context, tables []string) (map[string]*TableMetrics, error)
	
	// CollectQueryMetrics collects query performance metrics
	CollectQueryMetrics(ctx context.Context) ([]*QueryMetrics, error)
}

// Metrics represents database-level metrics
type Metrics struct {
	Timestamp     time.Time
	Connections   ConnectionMetrics
	Transactions  TransactionMetrics
	Performance   PerformanceMetrics
	Replication   ReplicationMetrics
	Storage       StorageMetrics
	Custom        map[string]interface{}
}

// ConnectionMetrics represents connection-related metrics
type ConnectionMetrics struct {
	Active         int
	Idle           int
	MaxConnections int
	Waiting        int
	Rejected       int64
	Timeouts       int64
}

// TransactionMetrics represents transaction-related metrics
type TransactionMetrics struct {
	Active         int
	Committed      int64
	RolledBack     int64
	Deadlocks      int64
	LockWaits      int64
	AvgDuration    time.Duration
}

// PerformanceMetrics represents performance-related metrics
type PerformanceMetrics struct {
	QPS            float64 // Queries per second
	TPS            float64 // Transactions per second
	ResponseTime   time.Duration
	CacheHitRatio  float64
	BufferHitRatio float64
}

// ReplicationMetrics represents replication-related metrics
type ReplicationMetrics struct {
	Enabled        bool
	Role           string // "primary" or "replica"
	Lag            time.Duration
	SlaveCount     int
	SyncState      string
}

// StorageMetrics represents storage-related metrics
type StorageMetrics struct {
	DatabaseSize   int64
	TableCount     int
	IndexCount     int
	TotalRows      int64
	DiskUsage      float64
}

// TableMetrics represents table-level metrics
type TableMetrics struct {
	TableName      string
	RowCount       int64
	DataSize       int64
	IndexSize      int64
	TotalSize      int64
	AutoVacuum     *time.Time
	LastAnalyze    *time.Time
	DeadTuples     int64
}

// QueryMetrics represents query-level metrics
type QueryMetrics struct {
	QueryID        string
	Query          string
	CallCount      int64
	TotalTime      time.Duration
	MeanTime       time.Duration
	MinTime        time.Duration
	MaxTime        time.Duration
	RowsReturned   int64
	SharedBlocks   BlockMetrics
	TempBlocks     BlockMetrics
}

// BlockMetrics represents block I/O metrics
type BlockMetrics struct {
	Hit      int64
	Read     int64
	Dirtied  int64
	Written  int64
}