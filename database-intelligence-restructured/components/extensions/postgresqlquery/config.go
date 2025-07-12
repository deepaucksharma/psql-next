package postgresqlquery

// Config represents the receiver configuration
type Config struct {
	// Database connection string
	Datasource string `mapstructure:"datasource"`
}