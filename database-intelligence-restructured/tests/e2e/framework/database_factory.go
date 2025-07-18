package framework

import (
	"context"
	"fmt"
	"time"
	
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
	DatabaseTypeRedis      DatabaseType = "redis"
	DatabaseTypeOracle     DatabaseType = "oracle"
	DatabaseTypeSQLServer  DatabaseType = "sqlserver"
)

// DatabaseContainer represents a running database container
type DatabaseContainer struct {
	Type      DatabaseType
	Container testcontainers.Container
	Host      string
	Port      int
	Username  string
	Password  string
	Database  string
}

// DatabaseFactory creates database containers for testing
type DatabaseFactory struct {
	ctx context.Context
}

// NewDatabaseFactory creates a new database factory
func NewDatabaseFactory(ctx context.Context) *DatabaseFactory {
	return &DatabaseFactory{ctx: ctx}
}

// CreateDatabase creates a database container of the specified type
func (f *DatabaseFactory) CreateDatabase(dbType DatabaseType, config DatabaseConfig) (*DatabaseContainer, error) {
	switch dbType {
	case DatabaseTypePostgreSQL:
		return f.createPostgreSQL(config)
	case DatabaseTypeMySQL:
		return f.createMySQL(config)
	case DatabaseTypeMongoDB:
		return f.createMongoDB(config)
	case DatabaseTypeRedis:
		return f.createRedis(config)
	case DatabaseTypeOracle:
		return f.createOracle(config)
	case DatabaseTypeSQLServer:
		return f.createSQLServer(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// DatabaseConfig configures database creation
type DatabaseConfig struct {
	Version    string
	InitScript string
	Username   string
	Password   string
	Database   string
	// Database-specific options
	Options map[string]interface{}
}

func (f *DatabaseFactory) createPostgreSQL(config DatabaseConfig) (*DatabaseContainer, error) {
	if config.Version == "" {
		config.Version = "15-alpine"
	}
	if config.Username == "" {
		config.Username = "postgres"
	}
	if config.Password == "" {
		config.Password = "postgres"
	}
	if config.Database == "" {
		config.Database = "postgres"
	}
	
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("postgres:%s", config.Version),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     config.Username,
			"POSTGRES_PASSWORD": config.Password,
			"POSTGRES_DB":       config.Database,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections"),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(60 * time.Second),
	}
	
	if config.InitScript != "" {
		req.Files = []testcontainers.ContainerFile{
			{
				HostFilePath:      config.InitScript,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
			},
		}
	}
	
	container, err := testcontainers.GenericContainer(f.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL container: %w", err)
	}
	
	host, err := container.Host(f.ctx)
	if err != nil {
		return nil, err
	}
	
	port, err := container.MappedPort(f.ctx, "5432")
	if err != nil {
		return nil, err
	}
	
	return &DatabaseContainer{
		Type:      DatabaseTypePostgreSQL,
		Container: container,
		Host:      host,
		Port:      port.Int(),
		Username:  config.Username,
		Password:  config.Password,
		Database:  config.Database,
	}, nil
}

func (f *DatabaseFactory) createMySQL(config DatabaseConfig) (*DatabaseContainer, error) {
	if config.Version == "" {
		config.Version = "8.0"
	}
	if config.Username == "" {
		config.Username = "root"
	}
	if config.Password == "" {
		config.Password = "rootpassword"
	}
	if config.Database == "" {
		config.Database = "mysql"
	}
	
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("mysql:%s", config.Version),
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": config.Password,
			"MYSQL_DATABASE":      config.Database,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("ready for connections"),
			wait.ForListeningPort("3306/tcp"),
		).WithDeadline(60 * time.Second),
	}
	
	if config.InitScript != "" {
		req.Files = []testcontainers.ContainerFile{
			{
				HostFilePath:      config.InitScript,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
			},
		}
	}
	
	container, err := testcontainers.GenericContainer(f.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MySQL container: %w", err)
	}
	
	host, err := container.Host(f.ctx)
	if err != nil {
		return nil, err
	}
	
	port, err := container.MappedPort(f.ctx, "3306")
	if err != nil {
		return nil, err
	}
	
	return &DatabaseContainer{
		Type:      DatabaseTypeMySQL,
		Container: container,
		Host:      host,
		Port:      port.Int(),
		Username:  config.Username,
		Password:  config.Password,
		Database:  config.Database,
	}, nil
}

func (f *DatabaseFactory) createMongoDB(config DatabaseConfig) (*DatabaseContainer, error) {
	if config.Version == "" {
		config.Version = "7.0"
	}
	if config.Username == "" {
		config.Username = "root"
	}
	if config.Password == "" {
		config.Password = "rootpassword"
	}
	if config.Database == "" {
		config.Database = "admin"
	}
	
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("mongo:%s", config.Version),
		ExposedPorts: []string{"27017/tcp"},
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": config.Username,
			"MONGO_INITDB_ROOT_PASSWORD": config.Password,
			"MONGO_INITDB_DATABASE":      config.Database,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Waiting for connections"),
			wait.ForListeningPort("27017/tcp"),
		).WithDeadline(60 * time.Second),
	}
	
	// MongoDB replica set mode
	if replicaSet, ok := config.Options["replicaSet"].(bool); ok && replicaSet {
		req.Cmd = []string{"--replSet", "rs0"}
		req.WaitingFor = wait.ForAll(
			wait.ForLog("Waiting for connections"),
			wait.ForListeningPort("27017/tcp"),
			wait.ForExec([]string{
				"mongosh", "--eval", "rs.initiate()",
			}).WithExitCodeMatcher(func(exitCode int) bool {
				return exitCode == 0
			}),
		).WithDeadline(90 * time.Second)
	}
	
	if config.InitScript != "" {
		req.Files = []testcontainers.ContainerFile{
			{
				HostFilePath:      config.InitScript,
				ContainerFilePath: "/docker-entrypoint-initdb.d/init.js",
			},
		}
	}
	
	container, err := testcontainers.GenericContainer(f.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MongoDB container: %w", err)
	}
	
	host, err := container.Host(f.ctx)
	if err != nil {
		return nil, err
	}
	
	port, err := container.MappedPort(f.ctx, "27017")
	if err != nil {
		return nil, err
	}
	
	return &DatabaseContainer{
		Type:      DatabaseTypeMongoDB,
		Container: container,
		Host:      host,
		Port:      port.Int(),
		Username:  config.Username,
		Password:  config.Password,
		Database:  config.Database,
	}, nil
}

func (f *DatabaseFactory) createRedis(config DatabaseConfig) (*DatabaseContainer, error) {
	if config.Version == "" {
		config.Version = "7-alpine"
	}
	
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("redis:%s", config.Version),
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Ready to accept connections"),
			wait.ForListeningPort("6379/tcp"),
		).WithDeadline(30 * time.Second),
	}
	
	// Redis with password
	if config.Password != "" {
		req.Cmd = []string{"redis-server", "--requirepass", config.Password}
	}
	
	// Redis cluster mode
	if cluster, ok := config.Options["cluster"].(bool); ok && cluster {
		req.Cmd = append(req.Cmd, "--cluster-enabled", "yes")
	}
	
	container, err := testcontainers.GenericContainer(f.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis container: %w", err)
	}
	
	host, err := container.Host(f.ctx)
	if err != nil {
		return nil, err
	}
	
	port, err := container.MappedPort(f.ctx, "6379")
	if err != nil {
		return nil, err
	}
	
	return &DatabaseContainer{
		Type:      DatabaseTypeRedis,
		Container: container,
		Host:      host,
		Port:      port.Int(),
		Password:  config.Password,
	}, nil
}

func (f *DatabaseFactory) createOracle(config DatabaseConfig) (*DatabaseContainer, error) {
	// Oracle XE implementation
	return nil, fmt.Errorf("Oracle support not yet implemented")
}

func (f *DatabaseFactory) createSQLServer(config DatabaseConfig) (*DatabaseContainer, error) {
	// SQL Server implementation
	return nil, fmt.Errorf("SQL Server support not yet implemented")
}

// GetConnectionString returns the connection string for the database
func (dc *DatabaseContainer) GetConnectionString() string {
	switch dc.Type {
	case DatabaseTypePostgreSQL:
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			dc.Host, dc.Port, dc.Username, dc.Password, dc.Database)
			
	case DatabaseTypeMySQL:
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
			
	case DatabaseTypeMongoDB:
		return fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
			dc.Username, dc.Password, dc.Host, dc.Port, dc.Database)
			
	case DatabaseTypeRedis:
		if dc.Password != "" {
			return fmt.Sprintf("redis://:%s@%s:%d", dc.Password, dc.Host, dc.Port)
		}
		return fmt.Sprintf("redis://%s:%d", dc.Host, dc.Port)
		
	default:
		return ""
	}
}

// Stop stops the database container
func (dc *DatabaseContainer) Stop() error {
	if dc.Container != nil {
		return dc.Container.Terminate(context.Background())
	}
	return nil
}

// WaitReady waits for the database to be ready
func (dc *DatabaseContainer) WaitReady(ctx context.Context, timeout time.Duration) error {
	// Database-specific readiness checks can be added here
	return nil
}

// GetEndpoint returns the database endpoint
func (dc *DatabaseContainer) GetEndpoint() string {
	return fmt.Sprintf("%s:%d", dc.Host, dc.Port)
}