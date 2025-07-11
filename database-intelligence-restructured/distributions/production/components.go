// Package main provides component factories for the Database Intelligence Collector
package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	
	// Custom receivers
	"github.com/database-intelligence/receivers/ash"
	"github.com/database-intelligence/receivers/kernelmetrics"
	
	// Custom processors
	"github.com/database-intelligence/processors/adaptivesampler"
	"github.com/database-intelligence/processors/circuitbreaker"
	"github.com/database-intelligence/processors/costcontrol"
	"github.com/database-intelligence/processors/nrerrormonitor"
	"github.com/database-intelligence/processors/planattributeextractor"
	"github.com/database-intelligence/processors/querycorrelator"
	"github.com/database-intelligence/processors/verification"
	
	// Custom exporters
	"github.com/database-intelligence/exporters/nri"
)

// componentsComplete provides all available components
var componentsComplete = otelcol.Factories{
	Receivers: map[component.Type]receiver.Factory{
		// Core receivers
		component.MustNewType("otlp"): otlpreceiver.NewFactory(),
		
		// Custom receivers
		component.MustNewType("ash"):           ash.NewFactory(),
		component.MustNewType("kernelmetrics"): kernelmetrics.NewFactory(),
	},
	Processors: map[component.Type]processor.Factory{
		// Core processors
		component.MustNewType("batch"):          batchprocessor.NewFactory(),
		component.MustNewType("memory_limiter"): memorylimiterprocessor.NewFactory(),
		
		// Custom processors
		component.MustNewType("adaptivesampler"):         adaptivesampler.NewFactory(),
		component.MustNewType("circuit_breaker"):         circuitbreaker.NewFactory(),
		component.MustNewType("costcontrol"):            costcontrol.NewFactory(),
		component.MustNewType("nrerrormonitor"):         nrerrormonitor.NewFactory(),
		component.MustNewType("planattributeextractor"): planattributeextractor.NewFactory(),
		component.MustNewType("querycorrelator"):        querycorrelator.NewFactory(),
		component.MustNewType("verification"):           verification.NewFactory(),
	},
	Exporters: map[component.Type]exporter.Factory{
		// Core exporters
		component.MustNewType("debug"): debugexporter.NewFactory(),
		component.MustNewType("otlp"):  otlpexporter.NewFactory(),
		
		// Custom exporters
		component.MustNewType("nri"): nri.NewFactory(),
	},
	Extensions: map[component.Type]extension.Factory{},
	Connectors: map[component.Type]connector.Factory{},
}
