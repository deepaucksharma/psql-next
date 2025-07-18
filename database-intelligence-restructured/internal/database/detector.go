package database

import (
	"context"
	"fmt"
	"strings"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypePostgreSQL    DatabaseType = "postgresql"
	DatabaseTypeMySQL         DatabaseType = "mysql"
	DatabaseTypeMongoDB       DatabaseType = "mongodb"
	DatabaseTypeRedis         DatabaseType = "redis"
	DatabaseTypeOracle        DatabaseType = "oracle"
	DatabaseTypeSQLServer     DatabaseType = "sqlserver"
	DatabaseTypeCassandra     DatabaseType = "cassandra"
	DatabaseTypeElasticsearch DatabaseType = "elasticsearch"
	DatabaseTypeUnknown       DatabaseType = "unknown"
)

// Detector provides database type detection capabilities
type Detector struct {
	client Client
}

// NewDetector creates a new database detector
func NewDetector(client Client) *Detector {
	return &Detector{client: client}
}

// DetectType detects the type of database from a client
func (d *Detector) DetectType(ctx context.Context) (DatabaseType, error) {
	// First try the client's built-in type
	if d.client != nil {
		clientType := d.client.DatabaseType()
		if dbType := parseClientType(clientType); dbType != DatabaseTypeUnknown {
			return dbType, nil
		}
	}
	
	// Try version-based detection
	dbType, err := d.detectByVersion(ctx)
	if err == nil && dbType != DatabaseTypeUnknown {
		return dbType, nil
	}
	
	// Try feature-based detection
	dbType, err = d.detectByFeatures(ctx)
	if err == nil && dbType != DatabaseTypeUnknown {
		return dbType, nil
	}
	
	return DatabaseTypeUnknown, fmt.Errorf("unable to detect database type")
}

// detectByVersion tries to detect database by version query
func (d *Detector) detectByVersion(ctx context.Context) (DatabaseType, error) {
	versionQueries := map[string]DatabaseType{
		"SELECT version()":                    DatabaseTypePostgreSQL,
		"SELECT @@version":                    DatabaseTypeMySQL,
		"SELECT * FROM v$version":             DatabaseTypeOracle,
		"SELECT @@VERSION":                    DatabaseTypeSQLServer,
		"db.version()":                        DatabaseTypeMongoDB,
		"INFO server":                         DatabaseTypeRedis,
	}
	
	for query, dbType := range versionQueries {
		rows, err := d.client.Query(ctx, query)
		if err == nil {
			if rows != nil {
				rows.Close()
			}
			return dbType, nil
		}
	}
	
	return DatabaseTypeUnknown, fmt.Errorf("version detection failed")
}

// detectByFeatures tries to detect database by feature queries
func (d *Detector) detectByFeatures(ctx context.Context) (DatabaseType, error) {
	featureQueries := []struct {
		query  string
		dbType DatabaseType
	}{
		{"SELECT 1 FROM pg_catalog.pg_database LIMIT 1", DatabaseTypePostgreSQL},
		{"SELECT 1 FROM information_schema.ENGINES WHERE ENGINE='InnoDB'", DatabaseTypeMySQL},
		{"SELECT 1 FROM v$database", DatabaseTypeOracle},
		{"SELECT 1 FROM sys.databases", DatabaseTypeSQLServer},
	}
	
	for _, fq := range featureQueries {
		rows, err := d.client.Query(ctx, fq.query)
		if err == nil {
			rows.Close()
			return fq.dbType, nil
		}
	}
	
	return DatabaseTypeUnknown, fmt.Errorf("feature detection failed")
}

// parseClientType converts client type string to DatabaseType
func parseClientType(clientType string) DatabaseType {
	normalized := strings.ToLower(strings.TrimSpace(clientType))
	
	switch {
	case strings.Contains(normalized, "postgres") || strings.Contains(normalized, "pgx"):
		return DatabaseTypePostgreSQL
	case strings.Contains(normalized, "mysql") || strings.Contains(normalized, "mariadb"):
		return DatabaseTypeMySQL
	case strings.Contains(normalized, "mongo"):
		return DatabaseTypeMongoDB
	case strings.Contains(normalized, "redis"):
		return DatabaseTypeRedis
	case strings.Contains(normalized, "oracle"):
		return DatabaseTypeOracle
	case strings.Contains(normalized, "sqlserver") || strings.Contains(normalized, "mssql"):
		return DatabaseTypeSQLServer
	case strings.Contains(normalized, "cassandra"):
		return DatabaseTypeCassandra
	case strings.Contains(normalized, "elasticsearch") || strings.Contains(normalized, "elastic"):
		return DatabaseTypeElasticsearch
	default:
		return DatabaseTypeUnknown
	}
}

// DetectTypeFromDSN attempts to detect database type from connection string
func DetectTypeFromDSN(dsn string) DatabaseType {
	lowerDSN := strings.ToLower(dsn)
	
	// Check for URI scheme
	switch {
	case strings.HasPrefix(lowerDSN, "postgres://") || strings.HasPrefix(lowerDSN, "postgresql://"):
		return DatabaseTypePostgreSQL
	case strings.HasPrefix(lowerDSN, "mysql://"):
		return DatabaseTypeMySQL
	case strings.HasPrefix(lowerDSN, "mongodb://") || strings.HasPrefix(lowerDSN, "mongodb+srv://"):
		return DatabaseTypeMongoDB
	case strings.HasPrefix(lowerDSN, "redis://") || strings.HasPrefix(lowerDSN, "rediss://"):
		return DatabaseTypeRedis
	case strings.Contains(lowerDSN, "oracle") || strings.Contains(lowerDSN, "@//"):
		return DatabaseTypeOracle
	case strings.Contains(lowerDSN, "sqlserver://") || strings.Contains(lowerDSN, "server="):
		return DatabaseTypeSQLServer
	}
	
	// Check for common patterns
	switch {
	case strings.Contains(lowerDSN, "host=") && strings.Contains(lowerDSN, "sslmode="):
		return DatabaseTypePostgreSQL
	case strings.Contains(lowerDSN, "@tcp(") || strings.Contains(lowerDSN, "@unix("):
		return DatabaseTypeMySQL
	case strings.Contains(lowerDSN, "replicaSet="):
		return DatabaseTypeMongoDB
	default:
		return DatabaseTypeUnknown
	}
}

// GetDatabaseInfo returns detailed information about the database
type DatabaseInfo struct {
	Type         DatabaseType
	Version      string
	Features     []Feature
	Replication  *ReplicationInfo
	Clustering   *ClusteringInfo
}

// ReplicationInfo contains replication details
type ReplicationInfo struct {
	Enabled      bool
	Role         string // "primary", "replica", "standby"
	Replicas     int
	Lag          string
}

// ClusteringInfo contains clustering details
type ClusteringInfo struct {
	Enabled      bool
	NodeCount    int
	NodeRole     string
	ClusterName  string
}

// GetDatabaseInfo retrieves comprehensive database information
func (d *Detector) GetDatabaseInfo(ctx context.Context) (*DatabaseInfo, error) {
	dbType, err := d.DetectType(ctx)
	if err != nil {
		return nil, err
	}
	
	info := &DatabaseInfo{
		Type:     dbType,
		Features: []Feature{},
	}
	
	// Get version information
	version, _ := d.getVersion(ctx, dbType)
	info.Version = version
	
	// Get features based on database type
	info.Features = d.getFeatures(dbType)
	
	// Get replication info
	info.Replication, _ = d.getReplicationInfo(ctx, dbType)
	
	// Get clustering info
	info.Clustering, _ = d.getClusteringInfo(ctx, dbType)
	
	return info, nil
}

// getVersion retrieves the database version
func (d *Detector) getVersion(ctx context.Context, dbType DatabaseType) (string, error) {
	var query string
	
	switch dbType {
	case DatabaseTypePostgreSQL:
		query = "SELECT version()"
	case DatabaseTypeMySQL:
		query = "SELECT VERSION()"
	case DatabaseTypeOracle:
		query = "SELECT banner FROM v$version WHERE ROWNUM = 1"
	case DatabaseTypeSQLServer:
		query = "SELECT @@VERSION"
	default:
		return "", fmt.Errorf("version query not supported for %s", dbType)
	}
	
	rows, err := d.client.Query(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	
	var version string
	if rows.Next() {
		err = rows.Scan(&version)
		if err != nil {
			return "", err
		}
	}
	
	return version, nil
}

// getFeatures returns supported features for the database type
func (d *Detector) getFeatures(dbType DatabaseType) []Feature {
	featureMap := map[DatabaseType][]Feature{
		DatabaseTypePostgreSQL: {
			FeatureTransactions,
			FeaturePreparedStmts,
			FeatureStoredProcs,
			FeatureReplication,
			FeaturePartitioning,
			FeatureQueryPlans,
			FeatureMetrics,
			FeatureStreaming,
		},
		DatabaseTypeMySQL: {
			FeatureTransactions,
			FeaturePreparedStmts,
			FeatureStoredProcs,
			FeatureReplication,
			FeaturePartitioning,
			FeatureQueryPlans,
			FeatureMetrics,
		},
		DatabaseTypeMongoDB: {
			FeatureReplication,
			FeatureClustering,
			FeatureMetrics,
			FeatureStreaming,
		},
		DatabaseTypeRedis: {
			FeatureReplication,
			FeatureClustering,
			FeatureMetrics,
			FeatureStreaming,
		},
		DatabaseTypeOracle: {
			FeatureTransactions,
			FeaturePreparedStmts,
			FeatureStoredProcs,
			FeatureReplication,
			FeaturePartitioning,
			FeatureQueryPlans,
			FeatureMetrics,
			FeatureClustering,
		},
		DatabaseTypeSQLServer: {
			FeatureTransactions,
			FeaturePreparedStmts,
			FeatureStoredProcs,
			FeatureReplication,
			FeaturePartitioning,
			FeatureQueryPlans,
			FeatureMetrics,
			FeatureClustering,
		},
	}
	
	if features, ok := featureMap[dbType]; ok {
		return features
	}
	
	return []Feature{}
}

// getReplicationInfo retrieves replication information
func (d *Detector) getReplicationInfo(ctx context.Context, dbType DatabaseType) (*ReplicationInfo, error) {
	switch dbType {
	case DatabaseTypePostgreSQL:
		return d.getPostgreSQLReplication(ctx)
	case DatabaseTypeMySQL:
		return d.getMySQLReplication(ctx)
	default:
		return nil, fmt.Errorf("replication info not implemented for %s", dbType)
	}
}

func (d *Detector) getPostgreSQLReplication(ctx context.Context) (*ReplicationInfo, error) {
	info := &ReplicationInfo{}
	
	// Check if in recovery mode (replica)
	var inRecovery bool
	err := d.queryValue(ctx, "SELECT pg_is_in_recovery()", &inRecovery)
	if err != nil {
		return nil, err
	}
	
	if inRecovery {
		info.Role = "replica"
		
		// Get replication lag
		var lag string
		_ = d.queryValue(ctx, `
			SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))::text || ' seconds'
		`, &lag)
		info.Lag = lag
	} else {
		info.Role = "primary"
		
		// Count replicas
		var replicas int
		_ = d.queryValue(ctx, "SELECT COUNT(*) FROM pg_stat_replication", &replicas)
		info.Replicas = replicas
	}
	
	info.Enabled = info.Role != "" || info.Replicas > 0
	
	return info, nil
}

func (d *Detector) getMySQLReplication(ctx context.Context) (*ReplicationInfo, error) {
	info := &ReplicationInfo{}
	
	// Check slave status
	rows, err := d.client.Query(ctx, "SHOW SLAVE STATUS")
	if err == nil && rows.Next() {
		info.Role = "replica"
		info.Enabled = true
		rows.Close()
	} else {
		// Check for replicas
		var replicas int
		_ = d.queryValue(ctx, "SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE COMMAND = 'Binlog Dump'", &replicas)
		if replicas > 0 {
			info.Role = "primary"
			info.Replicas = replicas
			info.Enabled = true
		}
	}
	
	return info, nil
}

// getClusteringInfo retrieves clustering information
func (d *Detector) getClusteringInfo(ctx context.Context, dbType DatabaseType) (*ClusteringInfo, error) {
	// This would be implemented based on database-specific clustering queries
	return nil, fmt.Errorf("clustering info not implemented for %s", dbType)
}

// queryValue is a helper to query a single value
func (d *Detector) queryValue(ctx context.Context, query string, dest interface{}) error {
	rows, err := d.client.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	if rows.Next() {
		return rows.Scan(dest)
	}
	
	return fmt.Errorf("no rows returned")
}