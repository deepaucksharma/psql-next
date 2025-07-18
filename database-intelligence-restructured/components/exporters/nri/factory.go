package nri

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// typeStr is the type of the exporter
	typeStr = "nri"
	// stability is the stability level of the exporter
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new NRI exporter factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

// createDefaultConfig creates the default configuration for the exporter
func createDefaultConfig() component.Config {
	return DefaultConfig()
}

// createMetricsExporter creates a metrics exporter
func createMetricsExporter(
	ctx context.Context,
	settings exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	nriCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}
	
	exp, err := newMetricsExporter(nriCfg, settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	
	return exporterhelper.NewMetricsExporter(
		ctx,
		settings,
		cfg,
		exp.exportMetrics,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: nriCfg.Timeout}),
	)
}

// createLogsExporter creates a logs exporter
func createLogsExporter(
	ctx context.Context,
	settings exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	nriCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}
	
	exp, err := newLogsExporter(nriCfg, settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	
	return exporterhelper.NewLogsExporter(
		ctx,
		settings,
		cfg,
		exp.exportLogs,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: nriCfg.Timeout}),
	)
}