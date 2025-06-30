// Database Intelligence Collector - OTEL-First Implementation
// This collector uses standard OpenTelemetry components with minimal custom processors
package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	
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
		Factories: factories,
	}

	if err := otelcol.NewCommand(params).Execute(); err != nil {
		log.Fatalf("collector failed: %v", err)
	}
}

// components returns the set of components for the collector
// This is a minimal set - mostly standard OTEL components
func Components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	// Get default OTEL components
	factories.Extensions, err = otelcol.DefaultExtensions()
	if err != nil {
		return factories, err
	}

	factories.Receivers, err = otelcol.DefaultReceivers()
	if err != nil {
		return factories, err
	}

	factories.Exporters, err = otelcol.DefaultExporters()
	if err != nil {
		return factories, err
	}

	// Start with default processors
	factories.Processors, err = otelcol.DefaultProcessors()
	if err != nil {
		return factories, err
	}

	// Add our custom processors for gaps
	// Only including processors that are actually built in ocb-config.yaml
	factories.Processors[planattributeextractor.TypeStr] = planattributeextractor.NewFactory()

	return factories, nil
}