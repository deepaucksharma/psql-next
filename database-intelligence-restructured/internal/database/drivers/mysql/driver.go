package mysql

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/database-intelligence/db-intel/internal/database"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"go.uber.org/zap"
)

// Driver implements the database.Driver interface for MySQL
type Driver struct {
	logger *zap.Logger
}

// NewDriver creates a new MySQL driver
func NewDriver(logger *zap.Logger) *Driver {
	return &Driver{
		logger: logger,
	}
}

// Name returns the database type name
func (d *Driver) Name() string {
	return "mysql"
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
	client, err := database.NewPooledClient("mysql", dataSource, poolConfig, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create MySQL connection: %w", err)
	}
	
	// Test connection
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}
	
	if d.logger != nil {
		d.logger.Info("Connected to MySQL",
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
	// MySQL DSN format: username:password@protocol(address)/dbname?param=value
	// or URL format: mysql://username:password@host:port/database?param=value
	
	config := &database.Config{
		Options: make(map[string]interface{}),
	}
	
	// Try URL format first
	if strings.HasPrefix(dsn, "mysql://") {
		return d.parseURLFormat(dsn)
	}
	
	// Parse standard MySQL DSN format
	return d.parseMySQLFormat(dsn)
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
		config.Port = 3306 // Default MySQL port
	}
	
	if config.Username == "" {
		return fmt.Errorf("username is required")
	}
	
	if config.Database == "" {
		return fmt.Errorf("database name is required")
	}
	
	return nil
}

// buildConnectionString builds a MySQL connection string
func (d *Driver) buildConnectionString(config *database.Config) string {
	// Format: username:password@tcp(host:port)/database?params
	var dsn strings.Builder
	
	// Username and password
	dsn.WriteString(config.Username)
	if config.Password != "" {
		dsn.WriteString(":")
		dsn.WriteString(config.Password)
	}
	dsn.WriteString("@")
	
	// Protocol and address
	dsn.WriteString("tcp(")
	dsn.WriteString(config.Host)
	dsn.WriteString(":")
	dsn.WriteString(strconv.Itoa(config.Port))
	dsn.WriteString(")/")
	
	// Database name
	dsn.WriteString(config.Database)
	
	// Parameters
	params := make([]string, 0, 10)
	params = append(params, "parseTime=true") // Always parse time values
	
	// TLS configuration
	if config.TLSEnabled {
		params = append(params, "tls=true")
		
		if config.TLSConfig != nil && config.TLSConfig.InsecureSkipVerify {
			params = append(params, "tls=skip-verify")
		}
	}
	
	// Add custom options
	for key, value := range config.Options {
		params = append(params, fmt.Sprintf("%s=%v", key, value))
	}
	
	if len(params) > 0 {
		dsn.WriteString("?")
		dsn.WriteString(strings.Join(params, "&"))
	}
	
	return dsn.String()
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
		config.Port = 3306
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
			case "tls":
				config.TLSEnabled = values[0] == "true" || values[0] == "1"
			default:
				config.Options[key] = values[0]
			}
		}
	}
	
	return config, nil
}

// parseMySQLFormat parses a standard MySQL DSN format
func (d *Driver) parseMySQLFormat(dsn string) (*database.Config, error) {
	config := &database.Config{
		Options: make(map[string]interface{}),
	}
	
	// Split into main part and query parameters
	parts := strings.SplitN(dsn, "?", 2)
	mainPart := parts[0]
	
	// Parse main part: username:password@protocol(address)/dbname
	atIndex := strings.LastIndex(mainPart, "@")
	if atIndex == -1 {
		return nil, fmt.Errorf("invalid MySQL DSN: missing @ character")
	}
	
	// Parse username and password
	userPass := mainPart[:atIndex]
	if colonIndex := strings.Index(userPass, ":"); colonIndex != -1 {
		config.Username = userPass[:colonIndex]
		config.Password = userPass[colonIndex+1:]
	} else {
		config.Username = userPass
	}
	
	// Parse protocol and address
	remaining := mainPart[atIndex+1:]
	if !strings.HasPrefix(remaining, "tcp(") {
		return nil, fmt.Errorf("unsupported protocol: only tcp is supported")
	}
	
	endParen := strings.Index(remaining, ")")
	if endParen == -1 {
		return nil, fmt.Errorf("invalid MySQL DSN: missing closing parenthesis")
	}
	
	address := remaining[4:endParen]
	if colonIndex := strings.LastIndex(address, ":"); colonIndex != -1 {
		config.Host = address[:colonIndex]
		port, err := strconv.Atoi(address[colonIndex+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		config.Port = port
	} else {
		config.Host = address
		config.Port = 3306
	}
	
	// Parse database name
	dbPart := remaining[endParen+1:]
	if strings.HasPrefix(dbPart, "/") {
		config.Database = dbPart[1:]
	}
	
	// Parse query parameters
	if len(parts) > 1 {
		params := strings.Split(parts[1], "&")
		for _, param := range params {
			kv := strings.SplitN(param, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				value := kv[1]
				
				switch key {
				case "tls":
					config.TLSEnabled = value == "true" || value == "1"
				default:
					config.Options[key] = value
				}
			}
		}
	}
	
	return config, nil
}