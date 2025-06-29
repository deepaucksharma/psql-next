// Database Intelligence Collector - OTEL-First Implementation
// This collector uses standard OpenTelemetry components with minimal custom processors
package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
	
	// Import custom processors
	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/verification"
)

func main() {
	// Create factories map
	factories, err := components()
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
func components() (otelcol.Factories, error) {
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
	factories.Processors[adaptivesampler.TypeStr] = adaptivesampler.NewFactory()
	factories.Processors[circuitbreaker.TypeStr] = circuitbreaker.NewFactory()
	factories.Processors[planattributeextractor.TypeStr] = planattributeextractor.NewFactory()
	factories.Processors[verification.TypeStr] = verification.NewFactory()

	return factories, nil
}