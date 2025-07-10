package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	
	// Core exporters
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	
	// Core processors
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	
	// Core receivers
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

var components = otelcol.Factories{
	Receivers: map[component.Type]receiver.Factory{
		component.MustNewType("otlp"):        otlpreceiver.NewFactory(),
	},
	Processors: map[component.Type]processor.Factory{
		component.MustNewType("batch"):        batchprocessor.NewFactory(),
		component.MustNewType("memory_limiter"): memorylimiterprocessor.NewFactory(),
	},
	Exporters: map[component.Type]exporter.Factory{
		component.MustNewType("debug"):       debugexporter.NewFactory(),
		component.MustNewType("otlp"):        otlpexporter.NewFactory(),
	},
	Extensions: map[component.Type]extension.Factory{},
	Connectors: map[component.Type]connector.Factory{},
}