package ohitransform

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "ohitransform"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the OHI transform processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TransformRules: []TransformRule{
			// PostgreSQL transformations
			{
				SourceMetric: "db.ash.active_sessions",
				TargetEvent:  "PostgresSlowQueries",
				Mappings: map[string]string{
					"db.querylens.queryid":          "query_id",
					"db.query.execution_time_mean":  "avg_elapsed_time_ms",
					"db.query.calls":                "execution_count",
					"db.name":                       "database_name",
					"db.query.text":                 "query_text",
					"db.schema":                     "schema_name",
					"db.query.disk_reads":           "avg_disk_reads",
					"db.query.disk_writes":          "avg_disk_writes",
				},
			},
			{
				SourceMetric: "db.ash.wait_events",
				TargetEvent:  "PostgresWaitEvents",
				Mappings: map[string]string{
					"wait_event_name":    "wait_event_name",
					"wait_event_type":    "wait_event_type",
					"wait_time_ms":       "total_wait_time_ms",
					"db.name":            "database_name",
				},
			},
			{
				SourceMetric: "db.ash.blocked_sessions",
				TargetEvent:  "PostgresBlockingSessions",
				Mappings: map[string]string{
					"blocked_pid":        "blocked_pid",
					"blocking_pid":       "blocking_pid",
					"blocked_query":      "blocked_query",
					"blocking_query":     "blocking_query",
					"db.name":            "database_name",
				},
			},
			// MySQL transformations
			{
				SourceMetric: "mysql.queries.slow",
				TargetEvent:  "MySQLSlowQueries",
				Mappings: map[string]string{
					"query_id":           "query_id",
					"query_time_ms":      "query_time_ms",
					"rows_examined":      "rows_examined",
					"rows_sent":          "rows_sent",
					"db.name":            "database_name",
				},
			},
		},
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	
	if err := oCfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	otp := &ohiTransformProcessor{
		config: oCfg,
		logger: set.Logger,
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		otp.processMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}