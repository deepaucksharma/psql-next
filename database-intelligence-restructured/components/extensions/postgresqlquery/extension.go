package postgresqlquery

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// postgresqlQueryExtension is the implementation of the PostgreSQL query extension
type postgresqlQueryExtension struct {
	config *Config
	logger *zap.Logger
}

// Start starts the extension
func (e *postgresqlQueryExtension) Start(ctx context.Context, host component.Host) error {
	e.logger.Info("Starting PostgreSQL Query extension")
	return nil
}

// Shutdown stops the extension
func (e *postgresqlQueryExtension) Shutdown(ctx context.Context) error {
	e.logger.Info("Shutting down PostgreSQL Query extension")
	return nil
}