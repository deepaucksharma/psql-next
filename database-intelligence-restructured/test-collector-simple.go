// Simple test collector using only standard OTEL components
package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
)

func main() {
	factories := otelcol.Factories{
		Receivers: map[component.Type]receiver.Factory{
			"postgresql": postgresqlreceiver.NewFactory(),
		},
		Processors: map[component.Type]processor.Factory{
			"batch": batchprocessor.NewFactory(),
		},
		Exporters: map[component.Type]exporter.Factory{
			"debug": debugexporter.NewFactory(),
		},
	}

	info := component.BuildInfo{
		Command:     "test-collector",
		Description: "Simple Test Collector",
		Version:     "1.0.0",
	}

	settings := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: func() (otelcol.Factories, error) {
			return factories, nil
		},
	}

	if err := otelcol.NewCommand(settings).Execute(); err != nil {
		log.Fatal(err)
	}
}