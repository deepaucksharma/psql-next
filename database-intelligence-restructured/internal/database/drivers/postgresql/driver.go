package postgresql

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/database-intelligence/db-intel/internal/database"
	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
)

// Driver implements the database.Driver interface for PostgreSQL
type Driver struct {
	logger *zap.Logger
}

// NewDriver creates a new PostgreSQL driver
func NewDriver(logger *zap.Logger) *Driver {
	return &Driver{
		logger: logger,
	}
}

// Name returns the database type name
func (d *Driver) Name() string {
	return "postgresql"
}

// Connect creates a new database connection with pooling
func (d *Driver) Connect(ctx context.Context, config *database.Config) (database.Client, error) {
	// Build connection string
	dataSource := d.buildConnectionString(config)
	
	// Create pool configuration
	poolConfig := database.ConnectionPoolConfig{
		MaxOpenConnections: config.MaxOpenConns,
		MaxIdleConnections: config.MaxIdleConns,
		ConnMaxLifetime:    config.ConnMaxLifetime,
		ConnMaxIdleTime:    config.ConnMaxIdleTime,
	}
	
	// Use default pool config if not specified
	if poolConfig.MaxOpenConnections == 0 {
		poolConfig = database.DefaultConnectionPoolConfig()
	}
	
	// Create pooled client
	client, err := database.NewPooledClient("postgres", dataSource, poolConfig, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL connection: %w", err)
	}
	
	// Test connection
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}
	
	if d.logger != nil {
		d.logger.Info("Connected to PostgreSQL",
			zap.String("host", config.Host),
			zap.Int("port", config.Port),
			zap.String("database", config.Database),
			zap.Int("max_open_conns", poolConfig.MaxOpenConnections),
			zap.Int("max_idle_conns", poolConfig.MaxIdleConnections))
	}
	
	return client, nil
}

// ParseDSN parses a connection string into a Config
func (d *Driver) ParseDSN(dsn string) (*database.Config, error) {
	// PostgreSQL connection strings can be in two formats:
	// 1. URL format: postgresql://user:pass@host:port/dbname?option=value
	// 2. Key-value format: host=localhost port=5432 user=postgres ...
	
	config := &database.Config{
		Options: make(map[string]interface{}),
	}
	
	// Try URL format first
	if strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "postgres://") {
		return d.parseURLFormat(dsn)
	}
	
	// Parse key-value format
	return d.parseKeyValueFormat(dsn)
}

// SupportedFeatures returns the list of features this driver supports
func (d *Driver) SupportedFeatures() []database.Feature {
	return []database.Feature{
		database.FeatureTransactions,
		database.FeaturePreparedStmts,
		database.FeatureStoredProcs,
		database.FeatureReplication,
		database.FeaturePartitioning,
		database.FeatureQueryPlans,
		database.FeatureMetrics,
		database.FeatureStreaming,
	}
}

// ValidateConfig checks if the configuration is valid for this driver
func (d *Driver) ValidateConfig(config *database.Config) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}
	
	if config.Host == "" {
		return fmt.Errorf("host is required")
	}
	
	if config.Port == 0 {
		config.Port = 5432 // Default PostgreSQL port
	}
	
	if config.Username == "" {
		return fmt.Errorf("username is required")
	}
	
	if config.Database == "" {
		return fmt.Errorf("database name is required")
	}
	
	return nil
}

// buildConnectionString builds a PostgreSQL connection string
func (d *Driver) buildConnectionString(config *database.Config) string {
	params := make([]string, 0, 10)
	
	params = append(params, fmt.Sprintf("host=%s", config.Host))
	params = append(params, fmt.Sprintf("port=%d", config.Port))
	params = append(params, fmt.Sprintf("user=%s", config.Username))
	
	if config.Password != "" {
		params = append(params, fmt.Sprintf("password=%s", config.Password))
	}
	
	params = append(params, fmt.Sprintf("dbname=%s", config.Database))
	
	// TLS configuration
	if config.TLSEnabled {
		params = append(params, "sslmode=require")
		
		if config.TLSConfig != nil {
			if config.TLSConfig.CAFile != "" {
				params = append(params, fmt.Sprintf("sslrootcert=%s", config.TLSConfig.CAFile))
			}
			if config.TLSConfig.CertFile != "" {
				params = append(params, fmt.Sprintf("sslcert=%s", config.TLSConfig.CertFile))
			}
			if config.TLSConfig.KeyFile != "" {
				params = append(params, fmt.Sprintf("sslkey=%s", config.TLSConfig.KeyFile))
			}
		}
	} else {
		params = append(params, "sslmode=disable")
	}
	
	// Add custom options
	for key, value := range config.Options {
		params = append(params, fmt.Sprintf("%s=%v", key, value))
	}
	
	return strings.Join(params, " ")
}

// parseURLFormat parses a URL-format connection string
func (d *Driver) parseURLFormat(dsn string) (*database.Config, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	
	config := &database.Config{
		Options: make(map[string]interface{}),
	}
	
	// Extract host and port
	if u.Hostname() != "" {
		config.Host = u.Hostname()
	}
	
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		config.Port = port
	} else {
		config.Port = 5432
	}
	
	// Extract username and password
	if u.User != nil {
		config.Username = u.User.Username()
		if password, ok := u.User.Password(); ok {
			config.Password = password
		}
	}
	
	// Extract database name
	if u.Path != "" {
		config.Database = strings.TrimPrefix(u.Path, "/")
	}
	
	// Parse query parameters
	for key, values := range u.Query() {
		if len(values) > 0 {
			switch key {
			case "sslmode":
				config.TLSEnabled = values[0] != "disable"
			case "sslrootcert":
				if config.TLSConfig == nil {
					config.TLSConfig = &database.TLSConfig{}
				}
				config.TLSConfig.CAFile = values[0]
			case "sslcert":
				if config.TLSConfig == nil {
					config.TLSConfig = &database.TLSConfig{}
				}
				config.TLSConfig.CertFile = values[0]
			case "sslkey":
				if config.TLSConfig == nil {
					config.TLSConfig = &database.TLSConfig{}
				}
				config.TLSConfig.KeyFile = values[0]
			default:
				config.Options[key] = values[0]
			}
		}
	}
	
	return config, nil
}

// parseKeyValueFormat parses a key-value format connection string
func (d *Driver) parseKeyValueFormat(dsn string) (*database.Config, error) {
	config := &database.Config{
		Options: make(map[string]interface{}),
	}
	
	// Split by spaces, handling quoted values
	parts := splitKeyValue(dsn)
	
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		
		switch key {
		case "host":
			config.Host = value
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %w", err)
			}
			config.Port = port
		case "user":
			config.Username = value
		case "password":
			config.Password = value
		case "dbname":
			config.Database = value
		case "sslmode":
			config.TLSEnabled = value != "disable"
		case "sslrootcert":
			if config.TLSConfig == nil {
				config.TLSConfig = &database.TLSConfig{}
			}
			config.TLSConfig.CAFile = value
		case "sslcert":
			if config.TLSConfig == nil {
				config.TLSConfig = &database.TLSConfig{}
			}
			config.TLSConfig.CertFile = value
		case "sslkey":
			if config.TLSConfig == nil {
				config.TLSConfig = &database.TLSConfig{}
			}
			config.TLSConfig.KeyFile = value
		default:
			config.Options[key] = value
		}
	}
	
	if config.Port == 0 {
		config.Port = 5432
	}
	
	return config, nil
}

// splitKeyValue splits a key-value string handling quoted values
func splitKeyValue(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	
	for i, r := range s {
		switch r {
		case '\'':
			inQuote = !inQuote
		case ' ':
			if !inQuote && current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
		
		// Handle last character
		if i == len(s)-1 && current.Len() > 0 {
			parts = append(parts, current.String())
		}
	}
	
	return parts
}