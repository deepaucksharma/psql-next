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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	
	// Import standard components we need
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	
	// Import database receivers
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"
	
	// Import additional processors
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	
	// Import custom processors - Database specific
	"github.com/database-intelligence/processors/adaptivesampler"
	"github.com/database-intelligence/processors/circuitbreaker"
	"github.com/database-intelligence/processors/planattributeextractor"
	"github.com/database-intelligence/processors/verification"
	
	// Import enterprise processors - New for 2025 patterns
	"github.com/database-intelligence/processors/nrerrormonitor"
	"github.com/database-intelligence/processors/costcontrol"
	"github.com/database-intelligence/processors/querycorrelator"
	
	// Import extensions
	"github.com/database-intelligence/extensions/healthcheck"
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

	// Create configuration provider settings
	set := otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: []string{"file:config.yaml"},
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(),
				envprovider.NewFactory(),
				yamlprovider.NewFactory(),
			},
		},
	}

	// Create and run the collector
	params := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: func() (otelcol.Factories, error) {
			return factories, nil
		},
		ConfigProviderSettings: set,
	}

	if err := otelcol.NewCommand(params).Execute(); err != nil {
		log.Fatalf("collector failed: %v", err)
	}
}

// Components returns the set of components for the collector
// Enhanced with enterprise-grade processors for production deployments
func Components() (otelcol.Factories, error) {
	factories := otelcol.Factories{}

	// Initialize factory maps
	factories.Extensions = make(map[component.Type]extension.Factory)
	factories.Receivers = make(map[component.Type]receiver.Factory)
	factories.Processors = make(map[component.Type]processor.Factory)
	factories.Exporters = make(map[component.Type]exporter.Factory)
	factories.Connectors = make(map[component.Type]connector.Factory)
	
	// Add extensions
	factories.Extensions[healthcheck.NewFactory().Type()] = healthcheck.NewFactory()

	// Add standard receivers
	factories.Receivers[otlpreceiver.NewFactory().Type()] = otlpreceiver.NewFactory()
	factories.Receivers[postgresqlreceiver.NewFactory().Type()] = postgresqlreceiver.NewFactory()
	factories.Receivers[mysqlreceiver.NewFactory().Type()] = mysqlreceiver.NewFactory()
	factories.Receivers[sqlqueryreceiver.NewFactory().Type()] = sqlqueryreceiver.NewFactory()
	
	// Add standard exporters
	factories.Exporters[debugexporter.NewFactory().Type()] = debugexporter.NewFactory()
	factories.Exporters[otlpexporter.NewFactory().Type()] = otlpexporter.NewFactory()
	factories.Exporters[fileexporter.NewFactory().Type()] = fileexporter.NewFactory()
	factories.Exporters[prometheusexporter.NewFactory().Type()] = prometheusexporter.NewFactory()
	
	// Add standard processors
	factories.Processors[batchprocessor.NewFactory().Type()] = batchprocessor.NewFactory()
	factories.Processors[memorylimiterprocessor.NewFactory().Type()] = memorylimiterprocessor.NewFactory()
	factories.Processors[transformprocessor.NewFactory().Type()] = transformprocessor.NewFactory()
	factories.Processors[resourceprocessor.NewFactory().Type()] = resourceprocessor.NewFactory()

	// Add our custom processors
	// Database-specific processors
	// These fill gaps in standard OTEL for database monitoring
	factories.Processors[planattributeextractor.GetType()] = planattributeextractor.NewFactory()
	factories.Processors[adaptivesampler.GetType()] = adaptivesampler.NewFactory()
	factories.Processors[circuitbreaker.GetType()] = circuitbreaker.NewFactory()
	factories.Processors[verification.GetType()] = verification.NewFactory()
	
	// Enterprise processors for production deployments
	// These implement the 2025 patterns for cost control and monitoring
	factories.Processors[component.MustNewType(nrerrormonitor.TypeStr)] = nrerrormonitor.NewFactory()
	factories.Processors[component.MustNewType(costcontrol.TypeStr)] = costcontrol.NewFactory()
	
	// OHI migration processor for query correlation  
	factories.Processors[component.MustNewType("querycorrelator")] = querycorrelator.NewFactory()

	return factories, nil
}