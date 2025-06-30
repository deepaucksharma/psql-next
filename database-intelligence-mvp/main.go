// Database Intelligence Collector - OTEL-First Implementation
// This collector uses standard OpenTelemetry components with minimal custom processors
package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/connector"
	
	// Import custom processors
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
)

func main() {
	// Create factories map
		factories, err := Components()
	if err != nil {
		log.Fatalf("failed to build components: %v", err)
	}

	// Build collector info
	info := component.BuildInfo{
		Command:     "database-intelligence-collector",
		Description: "Database Intelligence Collector - OTEL-First Implementation",
		Version:     "1.0.0",
	}

	// Create and run the collector
	params := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: func() (otelcol.Factories, error) {
			return factories, nil
		},
	}

	if err := otelcol.NewCommand(params).Execute(); err != nil {
		log.Fatalf("collector failed: %v", err)
	}
}

// components returns the set of components for the collector
// This is a minimal set - mostly standard OTEL components
func Components() (otelcol.Factories, error) {
	factories := otelcol.Factories{}

	// Initialize empty factory maps
	factories.Extensions = make(map[component.Type]extension.Factory)
	factories.Receivers = make(map[component.Type]receiver.Factory)
	factories.Processors = make(map[component.Type]processor.Factory)
	factories.Exporters = make(map[component.Type]exporter.Factory)
	factories.Connectors = make(map[component.Type]connector.Factory)

	// Add our custom processors for gaps
	// Only including processors that are actually built in ocb-config.yaml
	factories.Processors[planattributeextractor.GetType()] = planattributeextractor.NewFactory()

	return factories, nil
}