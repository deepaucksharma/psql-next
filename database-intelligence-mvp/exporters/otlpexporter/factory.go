package otlpexporter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// typeStr is the type string for this exporter
	typeStr = "otlp/postgresql"

	// stability is the stability level of this exporter
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new OTLP exporter factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

// createMetricsExporter creates a metrics exporter
func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oCfg := cfg.(*Config)
	
	if err := oCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	exp, err := newPostgresOTLPExporter(oCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exp.ConsumeMetrics,
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
		exporterhelper.WithTimeout(oCfg.TimeoutSettings),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithRetry(convertRetryConfig(oCfg.RetryConfig)),
		exporterhelper.WithCapabilities(exp.Capabilities()),
	)
}

// createLogsExporter creates a logs exporter
func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	oCfg := cfg.(*Config)
	
	if err := oCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	exp, err := newPostgresOTLPExporter(oCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exp.ConsumeLogs,
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
		exporterhelper.WithTimeout(oCfg.TimeoutSettings),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithRetry(convertRetryConfig(oCfg.RetryConfig)),
		exporterhelper.WithCapabilities(exp.Capabilities()),
	)
}

// convertRetryConfig converts our retry config to exporter helper retry settings
func convertRetryConfig(cfg *RetryConfig) exporterhelper.RetrySettings {
	if cfg == nil || !cfg.Enabled {
		return exporterhelper.RetrySettings{
			Enabled: false,
		}
	}

	return exporterhelper.RetrySettings{
		Enabled:         true,
		InitialInterval: cfg.InitialInterval,
		MaxInterval:     cfg.MaxInterval,
		MaxElapsedTime:  cfg.MaxElapsedTime,
	}
}