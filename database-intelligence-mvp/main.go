// Database Intelligence Collector - Enterprise OTEL Implementation
// Enhanced with enterprise-grade processors for cost control, monitoring, and security
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
	
	// Import custom processors - Database specific
	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/verification"
	
	// Import enterprise processors - New for 2025 patterns
	"github.com/database-intelligence-mvp/processors/nrerrormonitor"
	"github.com/database-intelligence-mvp/processors/costcontrol"
	"github.com/database-intelligence-mvp/processors/querycorrelator"
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
		Description: "Database Intelligence Collector - Enterprise Edition with Advanced Cost Control and Monitoring",
		Version:     "2.0.0", // Bumped for enterprise features
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

// Components returns the set of components for the collector
// Enhanced with enterprise-grade processors for production deployments
func Components() (otelcol.Factories, error) {
	factories := otelcol.Factories{}

	// Initialize empty factory maps
	factories.Extensions = make(map[component.Type]extension.Factory)
	factories.Receivers = make(map[component.Type]receiver.Factory)
	factories.Processors = make(map[component.Type]processor.Factory)
	factories.Exporters = make(map[component.Type]exporter.Factory)
	factories.Connectors = make(map[component.Type]connector.Factory)

	// Database-specific processors
	// These fill gaps in standard OTEL for database monitoring
	factories.Processors[planattributeextractor.GetType()] = planattributeextractor.NewFactory()
	factories.Processors[adaptivesampler.GetType()] = adaptivesampler.NewFactory()
	factories.Processors[circuitbreaker.GetType()] = circuitbreaker.NewFactory()
	factories.Processors[verification.GetType()] = verification.NewFactory()
	
	// Enterprise processors for production deployments
	// These implement the 2025 patterns for cost control and monitoring
	factories.Processors[nrerrormonitor.TypeStr] = nrerrormonitor.NewFactory()
	factories.Processors[costcontrol.TypeStr] = costcontrol.NewFactory()
	
	// OHI migration processor for query correlation
	factories.Processors[querycorrelator.TypeStr] = querycorrelator.NewFactory()

	return factories, nil
}