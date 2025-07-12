package postgresqlquery

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	typeStr   = "postgresqlquery"
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for PostgreSQL query extension
func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		createExtension,
		stability,
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	return &postgresqlQueryExtension{
		config: config,
		logger: set.Logger,
	}, nil
}