package database

import (
	"github.com/database-intelligence/db-intel/internal/database/drivers/mysql"
	"github.com/database-intelligence/db-intel/internal/database/drivers/postgresql"
	"go.uber.org/zap"
)

func init() {
	// Register built-in drivers
	logger := zap.NewNop() // Use no-op logger for driver registration
	
	// Register PostgreSQL driver
	RegisterDriver("postgresql", postgresql.NewDriver(logger))
	RegisterDriver("postgres", postgresql.NewDriver(logger))  // Alias
	
	// Register MySQL driver
	RegisterDriver("mysql", mysql.NewDriver(logger))
}

// InitializeDrivers initializes drivers with a specific logger
func InitializeDrivers(logger *zap.Logger) {
	// Re-register drivers with proper logger
	RegisterDriver("postgresql", postgresql.NewDriver(logger))
	RegisterDriver("postgres", postgresql.NewDriver(logger))
	RegisterDriver("mysql", mysql.NewDriver(logger))
}